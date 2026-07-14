# Getting started

`go-bindings-wmi` gives you typed access to WMI/CIM classes.

```sh
go get github.com/deploymenttheory/go-bindings-wmi
```

**Requirements:** Go 1.25+; runs on Windows (WMI is a Windows service). Depends
on `go-bindings-win32` (COM interfaces + runtime) and, transitively, `go-winmd`.

## Query a class

```go
//go:build windows

import (
	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

svc, err := wmi.Connect(`root\cimv2`)
if err != nil { /* ... */ }
defer svc.Close()

os, err := cimv2.QueryWin32OperatingSystem(svc, "") // typed []Win32OperatingSystem
fmt.Println(os[0].Caption, os[0].BuildNumber)

disks, _ := cimv2.QueryWin32LogicalDisk(svc, "DriveType = 3") // WHERE clause
for _, d := range disks {
	fmt.Printf("%s %d/%d bytes free\n", d.DeviceID, d.FreeSpace, d.Size)
}
```

Every captured class has a generated struct (typed fields) and a
`Query<Class>(svc, where)` helper. The second argument is the WQL `WHERE`
clause, or `""` for all instances.

## Untyped queries

For ad-hoc WQL, the runtime returns property maps:

```go
rows, _ := svc.Query("SELECT Name, ProcessId FROM Win32_Process")
for _, row := range rows {
	fmt.Println(row["Name"], row["ProcessId"]) // any: string, int64, uint64, bool, float64, nil
}
```

## Threading

`Connect` calls `CoInitializeEx` and locks the OS thread; `Close` reverses it.
Keep a `Service` on one goroutine, or connect per goroutine.

## Documentation

- [Capture and generate](capture-and-generate.md) — the metadata pipeline
- [WQL and VARIANTs](wql-and-variants.md) — types and decoding
- [`CLAUDE.md`](../CLAUDE.md) — the capture doctrine and why the CLI is minimal
