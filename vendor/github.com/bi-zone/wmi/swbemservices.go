// +build windows

package wmi

import (
	"fmt"
	"sync"

	"github.com/bi-zone/go-ole"
	"github.com/bi-zone/go-ole/oleutil"
	"github.com/hashicorp/go-multierror"
	"github.com/scjalliance/comshim"
)

// SWbemServices is used to access wmi on a different machines or namespaces
// (with different `SWbemServices ConnectServer` args) using the single object.
//
// If you need to query the single namespace on a single server prefer using
// SWbemServicesConnection instead.
type SWbemServices struct {
	sync.Mutex
	Decoder

	sWbemLocator *ole.IDispatch
}

// NewSWbemServices creates SWbemServices instance.
func NewSWbemServices() (s *SWbemServices, err error) {
	//  Be aware of reflections and COM usage.
	defer func() {
		if r := recover(); r != nil {
			err = multierror.Append(err, fmt.Errorf("runtime panic; %v", r))
		}
	}()

	comshim.Add(1)
	defer func() {
		if err != nil {
			comshim.Done()
		}
	}()

	locatorIUnknown, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, fmt.Errorf("CreateObject SWbemLocator error; %v", err)
	} else if locatorIUnknown == nil {
		return nil, ErrNilCreateObject
	}
	defer locatorIUnknown.Release()

	sWbemLocator, err := locatorIUnknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("SWbemLocator QueryInterface error; %v", err)
	}

	res := SWbemServices{
		sWbemLocator: sWbemLocator,
	}
	return &res, nil
}

// InitializeSWbemServices will return a new SWbemServices object that can be used to query WMI.
//
// Deprecated: Use NewSWbemServices instead.
func InitializeSWbemServices(c *Client, connectServerArgs ...interface{}) (*SWbemServices, error) {
	s, err := NewSWbemServices()
	if err != nil {
		return nil, err
	}
	s.Decoder = c.Decoder
	return s, nil
}

// Close will clear and release all of the SWbemServices resources.
func (s *SWbemServices) Close() error {
	s.Lock()
	defer s.Unlock()
	if s.sWbemLocator == nil {
		return fmt.Errorf("SWbemServices is not Initialized")
	}
	s.sWbemLocator.Release()
	s.sWbemLocator = nil
	comshim.Done()
	return nil
}

// Query runs the WQL query using a SWbemServicesConnection instance and appends
// the values to dst.
//
// More info about result unmarshalling is available in `Decoder.Unmarshal` doc.
//
// By default, the local machine and default namespace are used. These can be
// changed using connectServerArgs. See Ref. for more info.
//
// Ref: https://docs.microsoft.com/en-us/windows/desktop/wmisdk/swbemlocator-connectserver
func (s *SWbemServices) Query(query string, dst interface{}, connectServerArgs ...interface{}) (err error) {
	s.Lock()
	if s.sWbemLocator == nil {
		s.Unlock()
		return fmt.Errorf("SWbemServices has been closed")
	}
	s.Unlock()

	connection, err := s.ConnectServer(connectServerArgs...)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := connection.Close(); closeErr != nil {
			err = multierror.Append(err, closeErr)
		}
	}()
	return connection.Query(query, dst)
}
