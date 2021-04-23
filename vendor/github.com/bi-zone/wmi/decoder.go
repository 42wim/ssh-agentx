// +build windows

package wmi

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/bi-zone/go-ole"
	"github.com/bi-zone/go-ole/oleutil"
	"github.com/hashicorp/go-multierror"
)

// Unmarshaler is the interface implemented by types that can unmarshal COM
// object of themselves.
//
// N.B. Unmarshaler currently can't be implemented to non structure types!
type Unmarshaler interface {
	UnmarshalOLE(d Decoder, src *ole.IDispatch) error
}

// Dereferencer is anything that can fetch WMI objects using its object path.
// Used to retrieve object from CIM reference strings, e.g. from
// `Win32_LoggedOnUser`.
//
// Ref:
// 	https://docs.microsoft.com/en-us/openspecs/windows_protocols/ms-wmio/58e803a6-25f6-4ba6-abdc-b39e1daa66fc
//	https://docs.microsoft.com/en-us/windows/desktop/cimwin32prov/win32-loggedonuser
type Dereferencer interface {
	Dereference(referencePath string) (*ole.VARIANT, error)
}

// Decoder handles "decoding" of `ole.IDispatch` objects into the given
// structure. See `Decoder.Unmarshal` for more info.
type Decoder struct {
	// NonePtrZero specifies if nil values for fields which aren't pointers
	// should be returned as the field types zero value.
	//
	// Setting this to true allows structs without pointer fields to be used
	// without the risk failure should a nil value returned from WMI.
	NonePtrZero bool

	// PtrNil specifies if nil values for pointer fields should be returned
	// as nil.
	//
	// Setting this to true will set pointer fields to nil where WMI
	// returned nil, otherwise the types zero value will be returned.
	PtrNil bool

	// AllowMissingFields specifies that struct fields not present in the
	// query result should not result in an error.
	//
	// Setting this to true allows custom queries to be used with full
	// struct definitions instead of having to define multiple structs.
	AllowMissingFields bool

	// Dereferencer specifies an interface to resolve reference fields.
	// Dereferencer will be invoked on the fields tagged with ",ref" tag, e.g.
	//     Field Type `wmi:"FieldName,ref"
	//
	// Such fields will be resolved to the string that will be passed to
	// `Dereference` call. Resulting object will be used to fill the actual
	// field value.
	//
	// Dereferencer is automatically set by all query calls. Setting it to nil
	// will cause all fields tagged as references to return resolution error.
	Dereferencer Dereferencer
}

// ErrFieldMismatch is returned when a field is to be loaded into a different
// type than the one it was stored from, or when a field is missing or
// unexported in the destination struct.
// FieldType is the type of the struct pointed to by the destination argument.
type ErrFieldMismatch struct {
	FieldType reflect.Type
	FieldName string
	Reason    string
}

func (e ErrFieldMismatch) Error() string {
	return fmt.Sprintf("wmi: cannot load field %q into a %q: %s",
		e.FieldName, e.FieldType, e.Reason)
}

var timeType = reflect.TypeOf(time.Time{})

// Unmarshal loads `ole.IDispatch` into a struct pointer.
// N.B. Unmarshal supports only limited subset of structure field
// types:
//   - all signed and unsigned integers
//   - uintptr
//   - time.Time
//   - string
//   - bool
//   - float32
//   - a pointer to one of types above
//   - a slice of one of thus types
//   - structure types.
//
// To unmarshal more complex struct consider implementing `wmi.Unmarshaler`.
// For such types Unmarshal just calls `.UnmarshalOLE` on the @src object .
//
// To unmarshal COM-object into a struct, Unmarshal tries to fetch COM-object
// properties for each public struct field using as a property name either
// field name itself or the name specified in "wmi" field tag.
//
// By default any field missed in the COM-object leads to the error. To allow
// skipping such fields set `.AllowMissingFields` to `true`.
//
// Unmarshal does some "smart" type conversions between integer types (including
// unsigned ones), so you could receive e.g. `uint32` into `uint` if you don't
// care about the size.
//
// Unmarshal allows to specify special COM-object property name or skip a field
// using structure field tags, e.g.
//   // Will be filled from property `Frequency_Object`
//   FrequencyObject int wmi:"Frequency_Object"`
//
//   // Will be skipped during unmarshalling.
//   MyHelperField   int wmi:"-"`
//
//   // Will be unmarshalled by CIM reference.
//   // See `Dereferencer` for more info.
//	 Field  Type `wmi:"FieldName,ref"
//	 Field2 Type `wmi:",ref"
//
// Unmarshal prefers tag value over the field name, but ignores any name collisions.
// So for example all the following fields will be resolved to the same value.
//   Field  int
//   Field1 int `wmi:"Field"`
//   Field2 int `wmi:"Field"`
func (d Decoder) Unmarshal(src *ole.IDispatch, dst interface{}) (err error) {
	defer func() {
		// We use lots of reflection, so always be alert!
		if r := recover(); r != nil {
			err = fmt.Errorf("runtime panic: %v", r)
		}
	}()

	// Checks whether the type can handle unmarshalling of himself.
	if u, ok := dst.(Unmarshaler); ok {
		return u.UnmarshalOLE(d, src)
	}

	v := reflect.ValueOf(dst).Elem()
	vType := v.Type()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		fType := vType.Field(i)
		if err = d.unmarshalField(src, f, fType); err != nil {
			return ErrFieldMismatch{
				FieldType: fType.Type,
				FieldName: fType.Name,
				Reason:    err.Error(),
			}
		}
	}

	return nil
}

func (d Decoder) unmarshalField(src *ole.IDispatch, f reflect.Value, fType reflect.StructField) (err error) {
	fieldName, options := getFieldName(fType)
	if !f.CanSet() || fieldName == "-" {
		return nil
	}

	clearVariant := func(p *ole.VARIANT) {
		if clErr := p.Clear(); clErr != nil {
			err = multierror.Append(err, clErr)
		}
	}

	// Fetch property from the COM object.
	prop, err := oleutil.GetProperty(src, fieldName)
	if err != nil {
		if d.AllowMissingFields {
			return nil
		}
		return fmt.Errorf("no result field %q", fieldName)
	}
	defer clearVariant(prop)

	if prop.VT == ole.VT_NULL {
		return nil
	}

	// If it's a reference field and we have Dereferencer - resolve it.
	if options == "ref" {
		if d.Dereferencer == nil {
			return errors.New("failed to dereference ref field; no Decoder.Dereferencer set")
		}
		refPath := prop.ToString()
		prop, err = d.Dereferencer.Dereference(refPath)
		if err != nil {
			return err
		}
		defer clearVariant(prop)
	}

	return d.unmarshalValue(f, prop)
}

func (d Decoder) unmarshalValue(dst reflect.Value, prop *ole.VARIANT) error {
	isPtr := dst.Kind() == reflect.Ptr
	fieldDstOrig := dst
	if isPtr { // Create empty object for pointer receiver.
		ptr := reflect.New(dst.Type().Elem())
		dst.Set(ptr)
		dst = dst.Elem()
	}

	// First of all try to unmarshal it as a simple type.
	err := unmarshalSimpleValue(dst, prop.Value())
	if err != errSimpleVariantsExceeded {
		return err // Either nil and value set or unexpected error.
	}

	// Or we faced not so simple type. Do our best.
	switch dst.Kind() {
	case reflect.Slice:
		safeArray := prop.ToArray()
		if safeArray == nil {
			return fmt.Errorf("can't unmarshal %s into slice", prop.VT)
		}
		return unmarshalSlice(dst, safeArray)
	case reflect.Struct:
		dispatch := prop.ToIDispatch()
		if dispatch == nil {
			return fmt.Errorf("can't unmarshal %s into struct", prop.VT)
		}
		fieldPointer := dst.Addr().Interface()
		return d.Unmarshal(dispatch, fieldPointer)
	default:
		// If we got nil value - handle it with magic config fields.
		gotNilProp := reflect.TypeOf(prop.Value()) == nil
		if gotNilProp && (isPtr || d.NonePtrZero) {
			ptrNeedZero := isPtr && d.PtrNil
			nonPtrAllowNil := !isPtr && d.NonePtrZero
			if ptrNeedZero || nonPtrAllowNil {
				fieldDstOrig.Set(reflect.Zero(fieldDstOrig.Type()))
			}
			return nil
		}
		return fmt.Errorf("unsupported type (%T)", prop.Value())
	}
}

var (
	errSimpleVariantsExceeded = errors.New("unknown simple type")
)

// Here goes some kind of "smart" (too smart) field unmarshalling.
// It checks a type of a property value returned from COM object and then
// tries to fit it inside a given structure field with some possible
// conversions (e.g. possible integer conversions, string to int parsing
// and others).
//
// This function handles all oleutil.VARIANT types except VT_UNKNOWN and
// VT_DISPATCH.
func unmarshalSimpleValue(dst reflect.Value, value interface{}) error {
	switch val := value.(type) {
	case int8, int16, int32, int64, int:
		v := reflect.ValueOf(val).Int()
		switch dst.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			dst.SetInt(v)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			dst.SetUint(uint64(v))
		default:
			return errors.New("not an integer class")
		}
	case uint8, uint16, uint32, uint64:
		v := reflect.ValueOf(val).Uint()
		switch dst.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			dst.SetInt(int64(v))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			dst.SetUint(v)
		default:
			return errors.New("not an integer class")
		}
	case bool:
		switch dst.Kind() {
		case reflect.Bool:
			dst.SetBool(val)
		default:
			return errors.New("not a bool")
		}
	case float32:
		switch dst.Kind() {
		case reflect.Float32:
			dst.SetFloat(float64(val))
		default:
			return errors.New("not a float32")
		}
	case time.Time:
		switch dst.Type() {
		case timeType:
			dst.Set(reflect.ValueOf(val))
		default:
			return errors.New("not a time")
		}
	case uintptr:
		switch dst.Kind() {
		case reflect.Uintptr:
			dst.Set(reflect.ValueOf(val))
		default:
			return errors.New("not an uintptr")
		}
	case string:
		return smartUnmarshalString(dst, val)
	default:
		return errSimpleVariantsExceeded
	}
	return nil
}

func unmarshalSlice(fieldDst reflect.Value, safeArray *ole.SafeArrayConversion) error {
	arr := safeArray.ToValueArray()
	resultArr := reflect.MakeSlice(fieldDst.Type(), len(arr), len(arr))
	for i, v := range arr {
		s := resultArr.Index(i)
		err := unmarshalSimpleValue(s, v)
		if err != nil {
			return fmt.Errorf("can't put %T into []%s", v, fieldDst.Type().Elem().Kind())
		}
	}
	fieldDst.Set(resultArr)
	return nil
}

func smartUnmarshalString(fieldDst reflect.Value, val string) error {
	switch fieldDst.Kind() {
	case reflect.String:
		fieldDst.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		iv, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		fieldDst.SetInt(iv)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uv, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		fieldDst.SetUint(uv)
	case reflect.Struct:
		switch t := fieldDst.Type(); t {
		case timeType:
			return unmarshalTime(fieldDst, val)
		default:
			return fmt.Errorf("can't deserialize string into struct %T", fieldDst.Interface())
		}
	default:
		return fmt.Errorf("can't deserealize string into %s", fieldDst.Kind())
	}
	return nil
}

// parses CIM_DATETIME from string format "yyyymmddHHMMSS.mmmmmmsUUU"
// where
//		"mmmmmm"	Six-digit number of microseconds in the second.
//		"s"			Plus sign (+) or minus sign (-) to indicate a positive or
//					negative offset from UTC.
// 		"UUU" 	 	Three-digit offset indicating the number of minutes that the
// 					originating time zone deviates from UTC.
// 		(other are obvious)
// ref: https://docs.microsoft.com/en-us/windows/desktop/wmisdk/cim-datetime
func unmarshalTime(fieldDst reflect.Value, val string) error {
	const signPos = 21
	if sign := val[signPos]; sign == '+' || sign == '-' {
		// golang can't understand such timezone offset, so transform minute
		// offset to the "HHMM" tz offset.
		timeZonePart := val[signPos+1:]
		minOffset, err := strconv.Atoi(timeZonePart)
		if err != nil {
			return err
		}
		isoTzOffset := fmt.Sprintf("%02d%02d", minOffset/60, minOffset%60)
		val = val[:signPos+1] + isoTzOffset
	}
	// Parsing format: "yyyymmddHHMMSS.mmmmmmsHHMM"
	t, err := time.Parse("20060102150405.000000-0700", val)
	if err != nil {
		return err
	}
	fieldDst.Set(reflect.ValueOf(t))
	return nil
}

func getFieldName(fType reflect.StructField) (name, options string) {
	tag := fType.Tag.Get("wmi")
	if idx := strings.Index(tag, ","); idx != -1 {
		name = tag[:idx]
		options = tag[idx+1:]
	} else {
		name = tag
	}
	if name == "" {
		name = fType.Name
	}
	return
}
