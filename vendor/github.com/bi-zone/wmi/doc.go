// +build windows

/*
Package wmi provides a WMI Query Language (WQL) interface for
Windows Management Instrumentation (WMI) on Windows.

This package uses COM API for WMI therefore it's only usable on the Windows machines.

This package has many .Query calls, the main rule of thumb for choosing the right
one is "prefer SWbemServicesConnection if you bother about performance and just do
wmi.Query if not". More detailed benchmarks are available in the repo:
https://github.com/bi-zone/wmi#benchmarks

More reference about WMI is available in Microsoft Docs:
https://docs.microsoft.com/en-us/windows/win32/wmisdk/wmi-reference)
*/
package wmi
