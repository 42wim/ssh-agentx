// +build windows

package wmi

import (
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/bi-zone/go-ole"
	"github.com/bi-zone/go-ole/oleutil"
	"github.com/hashicorp/go-multierror"
	"github.com/scjalliance/comshim"
)

var (
	// ErrConnectionClosed is returned for methods called on the closed
	// SWbemServicesConnection.
	ErrConnectionClosed = errors.New("SWbemServicesConnection has been closed")
)

// SWbemServicesConnection is used to access SWbemServices methods of the
// single server.
//
// Ref: https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemservices
type SWbemServicesConnection struct {
	sync.Mutex
	Decoder

	sWbemServices *ole.IDispatch
}

// ConnectSWbemServices creates SWbemServices connection to the server defined
// by @connectServerArgs. Actually it just creates `SWbemLocator` and invokes
// `SWbemServices ConnectServer` method. Args are passed to the method as it.
//
// Ref: https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemlocator-connectserver
func ConnectSWbemServices(connectServerArgs ...interface{}) (conn *SWbemServicesConnection, err error) {
	services, err := NewSWbemServices()
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := services.Close(); closeErr != nil {
			err = multierror.Append(err, closeErr)
		}
	}()
	return services.ConnectServer(connectServerArgs...)
}

// ConnectSWbemServices creates SWbemServices connection to the server defined
// by @args.
//
// Ref: https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemlocator-connectserver
func (s *SWbemServices) ConnectServer(args ...interface{}) (c *SWbemServicesConnection, err error) {
	//  Be aware of reflections and COM usage.
	defer func() {
		if r := recover(); r != nil {
			err = multierror.Append(err, fmt.Errorf("runtime panic; %v", r))
		}
	}()

	// Notify that we are going to use COM. We will care about at least one
	// reference for connection.
	comshim.Add(1)
	defer func() {
		if err != nil {
			comshim.Done()
		}
	}()

	serviceRaw, err := oleutil.CallMethod(s.sWbemLocator, "ConnectServer", args...)
	if err != nil {
		return nil, fmt.Errorf("SWbemServices ConnectServer error; %v", err)
	}
	service := serviceRaw.ToIDispatch()
	if service == nil {
		return nil, errors.New("SWbemServices IDispatch returned nil")
	}

	// Resulting IDispatch uses the same mem as a variant, and a variant will not clear anything
	// (ref: https://docs.microsoft.com/en-us/windows/desktop/api/oleauto/nf-oleauto-variantclear)
	// so we have no need to care about of serviceRaw and moreover call clear on it.

	conn := &SWbemServicesConnection{
		Decoder:       s.Decoder,
		sWbemServices: service,
	}
	conn.Decoder.Dereferencer = conn
	return conn, nil
}

// Close will clear and release all of the SWbemServicesConnection resources.
func (s *SWbemServicesConnection) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.sWbemServices == nil {
		return nil // Already stopped.
	}
	s.sWbemServices.Release()
	s.sWbemServices = nil
	comshim.Done()
	return nil
}

// Query runs the WQL query using a SWbemServicesConnection instance and appends
// the values to dst.
//
// More info about result unmarshalling is available in `Decoder.Unmarshal` doc.
//
// Query is performed using `SWbemServices.ExecQuery` method.
//
// Ref: https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemservices-execquery
func (s *SWbemServicesConnection) Query(query string, dst interface{}) error {
	s.Lock()
	if s.sWbemServices == nil {
		s.Unlock()
		return ErrConnectionClosed
	}
	s.Unlock()

	sliceRefl := reflect.ValueOf(dst) // TODO: Double argument check?
	if sliceRefl.Kind() != reflect.Ptr || sliceRefl.IsNil() {
		return ErrInvalidEntityType
	}
	sliceRefl = sliceRefl.Elem() // "Dereference" pointer.

	argType, elemType := checkMultiArg(sliceRefl)
	if argType == multiArgTypeInvalid {
		return ErrInvalidEntityType
	}

	return s.query(query, &queryDst{
		dst:         sliceRefl,
		dsArgType:   argType,
		dstElemType: elemType,
	})
}

// Get retrieves a single instance of a managed resource (or class definition)
// based on an object @path. The result is unmarshalled into @dst. @dst should
// be a pointer to the structure type.
//
// More info about result unmarshalling is available in `Decoder.Unmarshal` doc.
//
// Get method reference:
// https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemservices-get
func (s *SWbemServicesConnection) Get(path string, dst interface{}) (err error) {
	s.Lock()
	if s.sWbemServices == nil {
		s.Unlock()
		return ErrConnectionClosed
	}
	s.Unlock()

	//  Be aware of reflections and COM usage.
	defer func() {
		if r := recover(); r != nil {
			err = multierror.Append(err, fmt.Errorf("runtime panic; %v", r))
		}
	}()

	dstRef := reflect.ValueOf(dst)
	if dstRef.Kind() != reflect.Ptr && dstRef.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("dst should be a pointer to struct")
	}

	resultRaw, err := s.dereference(path)
	if err != nil {
		return err
	}
	result := resultRaw.ToIDispatch()
	defer func() {
		if clErr := resultRaw.Clear(); clErr != nil {
			err = multierror.Append(err, clErr)
		}
	}()

	return s.Unmarshal(result, dst)
}

// Dereference performs `SWbemServices.Get` on the given path, but returns the
// low level result itself not performing unmarshalling.
func (s *SWbemServicesConnection) Dereference(referencePath string) (v *ole.VARIANT, err error) {
	s.Lock()
	if s.sWbemServices == nil {
		s.Unlock()
		return nil, ErrConnectionClosed
	}
	s.Unlock()

	//  Be aware of reflections and COM usage.
	defer func() {
		if r := recover(); r != nil {
			err = multierror.Append(err, fmt.Errorf("runtime panic; %v", r))
		}
	}()

	return s.dereference(referencePath)
}

func (s *SWbemServicesConnection) dereference(referencePath string) (v *ole.VARIANT, err error) {
	return oleutil.CallMethod(s.sWbemServices, "Get", referencePath)
}

type queryDst struct {
	dst         reflect.Value
	dsArgType   multiArgType
	dstElemType reflect.Type
}

func (s *SWbemServicesConnection) query(query string, dst *queryDst) (err error) {
	//  Be aware of reflections and COM usage.
	defer func() {
		if r := recover(); r != nil {
			err = multierror.Append(err, fmt.Errorf("runtime panic; %v", r))
		}
	}()

	// result is a SWBemObjectSet
	resultRaw, err := oleutil.CallMethod(s.sWbemServices, "ExecQuery", query)
	if err != nil {
		return err
	}
	result := resultRaw.ToIDispatch()
	defer func() {
		if clErr := resultRaw.Clear(); clErr != nil {
			err = multierror.Append(err, clErr)
		}
	}()

	count, err := oleInt64(result, "Count")
	if err != nil {
		return err
	}

	enumProperty, err := result.GetProperty("_NewEnum")
	if err != nil {
		return err
	}
	defer func() {
		if clErr := enumProperty.Clear(); clErr != nil {
			err = multierror.Append(err, clErr)
		}
	}()

	enum, err := enumProperty.ToIUnknown().IEnumVARIANT(ole.IID_IEnumVariant)
	if err != nil {
		return err
	}
	if enum == nil {
		return fmt.Errorf("can't get IEnumVARIANT, enum is nil")
	}
	defer enum.Release()

	// Initialize a slice with Count capacity
	dst.dst.Set(reflect.MakeSlice(dst.dst.Type(), 0, int(count)))

	var errFieldMismatch error
	for itemRaw, length, err := enum.Next(1); length > 0; itemRaw, length, err = enum.Next(1) {
		if err != nil {
			return err
		}

		// Closure for defer in the loop.
		err := func() error {
			item := itemRaw.ToIDispatch()
			defer item.Release()

			ev := reflect.New(dst.dstElemType)
			if err = s.Unmarshal(item, ev.Interface()); err != nil {
				if _, ok := err.(ErrFieldMismatch); ok {
					// We continue loading entities even in the face of field mismatch errors.
					// If we encounter any other error, that other error is returned. Otherwise,
					// an ErrFieldMismatch is returned.
					//
					// Note that we are unmarshalling into the slice, so every element of the
					// result will have the same error thus we can save the only error occurred.
					errFieldMismatch = err
				} else {
					return err
				}
			}

			if dst.dsArgType != multiArgTypeStructPtr {
				ev = ev.Elem()
			}
			dst.dst.Set(reflect.Append(dst.dst, ev))

			return nil
		}()
		if err != nil {
			return err
		}
	}
	return errFieldMismatch
}

type multiArgType int

const (
	multiArgTypeInvalid multiArgType = iota
	multiArgTypeStruct
	multiArgTypeStructPtr
)

// checkMultiArg checks that v has type []S, []*S for some struct type S.
//
// It returns what category the slice's elements are, and the reflect.Type
// that represents S.
func checkMultiArg(v reflect.Value) (m multiArgType, elemType reflect.Type) {
	if v.Kind() != reflect.Slice {
		return multiArgTypeInvalid, nil
	}
	elemType = v.Type().Elem()
	switch elemType.Kind() {
	case reflect.Struct:
		return multiArgTypeStruct, elemType
	case reflect.Ptr:
		elemType = elemType.Elem()
		if elemType.Kind() == reflect.Struct {
			return multiArgTypeStructPtr, elemType
		}
	}
	return multiArgTypeInvalid, nil
}

func oleInt64(item *ole.IDispatch, prop string) (val int64, err error) {
	v, err := oleutil.GetProperty(item, prop)
	if err != nil {
		return 0, err
	}
	defer func() {
		if clErr := v.Clear(); clErr != nil {
			err = multierror.Append(err, clErr)
		}
	}()

	i := int64(v.Val)
	return i, nil
}
