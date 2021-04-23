# wmi
[![GoDoc](https://godoc.org/github.com/bi-zone/wmi?status.svg)](https://godoc.org/github.com/bi-zone/wmi/)
[![Go Report Card](https://goreportcard.com/badge/github.com/bi-zone/wmi)](https://goreportcard.com/report/github.com/bi-zone/wmi)

Package wmi provides a WMI Query Language (WQL) interface for
Windows Management Instrumentation (WMI) on Windows.

This package uses [COM API for WMI](https://docs.microsoft.com/en-us/windows/win32/wmisdk/com-api-for-wmi)
therefore it's only usable on the Windows machines.

Package reference is available at https://godoc.org/github.com/bi-zone/wmi

## Fork Features
**Fork is fully compatibly with the original repo.** If not - please open an issue.

New features introduced in fork:
- Go 1.11 modules support :)
- Improved decoder:
    + support all basic types: all integer types, `float32`, 
    `string`, `bool`, `uintptr` and `time.Time`
    + support slices and pointers to all basic types
    + support decoding of structure fields (see [events example](./examples/events/main.go))
    + support structure tags
    + support JSON-like interface for custom decoding
    + suitable for decoding properties of any [go-ole](https://github.com/go-ole/go-ole) 
    `IDispatch` object
- Ability to perform multiple queries in a single connection
- `SWbemServices.Get` + auto dereference of REF fields
- `SWbemServices.ExecNotificationQuery` support
- More other improvements described in [releases page](https://github.com/bi-zone/wmi/releases)

## Example
 Print names of the currently running processes
 ```golang
package main

import (
	"fmt"
	"log"

	"github.com/bi-zone/wmi"
)

type win32Process struct {
	PID       uint32 `wmi:"ProcessId"`
	Name      string
	UserField int `wmi:"-"`
}

func main() {
	var dst []win32Process

	q := wmi.CreateQueryFrom(&dst, "Win32_Process", "")
	fmt.Println(q)

	if err := wmi.Query(q, &dst); err != nil {
		log.Fatal(err)
	}
	for _, v := range dst {
		fmt.Printf("%6d\t%s\n", v.PID, v.Name)
	}
}
 ```
 
 A more sophisticated examples are located at in [`examples`](./examples) folder.

## Benchmarks
Using `DefaultClient`, `SWbemServices` or `SWbemServicesConnection` differ in a number
of setup calls doing to perform each query (from the most to the least).

Estimated overhead is shown below:
```
BenchmarkQuery_DefaultClient   5000  33529798 ns/op
BenchmarkQuery_SWbemServices   5000  32031199 ns/op
BenchmarkQuery_SWbemConnection 5000  30099403 ns/op
```

You could reproduce the results on your machine running:
```bash
go test -run=NONE -bench=Query -benchtime=120s
```

## Versioning

Project uses [semantic versioning](http://semver.org) for version numbers, which
is similar to the version contract of the Go language. Which means that the major
version will always maintain backwards compatibility with minor versions. Minor 
versions will only add new additions and changes. Fixes will always be in patch. 

This contract should allow you to upgrade to new minor and patch versions without
breakage or modifications to your existing code. Leave a ticket, if there is breakage,
so that it could be fixed.
