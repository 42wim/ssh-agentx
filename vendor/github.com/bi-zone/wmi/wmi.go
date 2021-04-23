// +build windows

package wmi

import (
	"bytes"
	"errors"
	"reflect"
	"strings"

	"github.com/hashicorp/go-multierror"
)

var (
	// ErrInvalidEntityType is returned in case of unsupported destination type
	// given to the `Query` call.
	ErrInvalidEntityType = errors.New("wmi: invalid entity type")

	// ErrNilCreateObject is the error returned if CreateObject returns nil even
	// if the error was nil.
	ErrNilCreateObject = errors.New("wmi: create object returned nil")
)

// QueryNamespace invokes Query with the given namespace on the local machine.
func QueryNamespace(query string, dst interface{}, namespace string) error {
	return Query(query, dst, nil, namespace)
}

// Query runs the WQL query and appends the values to dst.
//
// More info about result unmarshalling is available in `Decoder.Unmarshal` doc.
//
// By default, the local machine and default namespace are used. These can be
// changed using connectServerArgs. See a reference below for details.
//
//   https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemlocator-connectserver
//
// Query is a wrapper around DefaultClient.Query.
func Query(query string, dst interface{}, connectServerArgs ...interface{}) error {
	return DefaultClient.Query(query, dst, connectServerArgs...)
}

// CreateQuery returns a WQL query string that queries all columns of @src.
//
// @src could be T, *T, []T, or *[]T;
//
// @where is an optional string that is appended to the query, to be used with
// WHERE clauses. In such a case, the "WHERE" string should appear at
// the beginning.
//
//   type Win32_Product struct {
//   	Name            string
//   	InstallLocation string
//   }
//   var dst []Win32_Product
//   query := wmi.CreateQuery(&dst, "WHERE InstallLocation != null")
func CreateQuery(src interface{}, where string) string {
	s := reflect.Indirect(reflect.ValueOf(src))
	t := s.Type()
	if s.Kind() == reflect.Slice {
		t = t.Elem()
	}
	return CreateQueryFrom(src, t.Name(), where)
}

// CreateQuery returns a WQL query string that queries all columns of @src from
// class @from with condition @where (optional).
//
// N.B. The call is the same as `CreateQuery` but uses @from instead of structure
// name as a class name.
func CreateQueryFrom(src interface{}, from, where string) string {
	s := reflect.Indirect(reflect.ValueOf(src))
	t := s.Type()
	if s.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}

	var b bytes.Buffer
	b.WriteString("SELECT ")
	var fields []string
	for i := 0; i < t.NumField(); i++ {
		name, _ := getFieldName(t.Field(i))
		if name == "-" {
			continue
		}
		fields = append(fields, name)
	}
	b.WriteString(strings.Join(fields, ", "))
	b.WriteString(" FROM ")
	b.WriteString(from)
	b.WriteString(" " + where)
	return b.String()
}

// A Client is an WMI query client.
//
// Its zero value (`DefaultClient`) is a usable client.
//
// Client provides an ability to modify result decoding params by modifying
// embedded `.Decoder` properties.
//
// Important: Using zero-value Client does not speed up your queries comparing
// to using `wmi.Query` method. Refer to benchmarks in repo README.md for more
// info about the speed.
type Client struct {
	// Embedded Decoder for backward-compatibility.
	Decoder

	// SWbemServiceClient is an optional SWbemServices object that can be
	// initialized and then reused across multiple queries. If it is null
	// then the method will initialize a new temporary client each time.
	SWbemServicesClient *SWbemServices
}

// DefaultClient is the default Client and is used by Query, QueryNamespace
var DefaultClient = &Client{}

// Query runs the WQL query and appends the values to dst.
//
// More info about result unmarshalling is available in `Decoder.Unmarshal` doc.
//
// By default, the local machine and default namespace are used. These can be
// changed using connectServerArgs. See a reference below for details.
//
//   https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemlocator-connectserver
func (c *Client) Query(query string, dst interface{}, connectServerArgs ...interface{}) (err error) {
	client := c.SWbemServicesClient
	if client == nil {
		client, err = NewSWbemServices()
		if err != nil {
			return err
		}
		defer func() {
			if clErr := client.Close(); clErr != nil {
				err = multierror.Append(err, clErr)
			}
		}()
	}
	client.Decoder = c.Decoder // Patch decoder to use set decoder flags inside `Query`.
	return client.Query(query, dst, connectServerArgs...)
}
