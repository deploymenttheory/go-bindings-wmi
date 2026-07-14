# go-bindings-wmi

[![Go Reference](https://pkg.go.dev/badge/github.com/deploymenttheory/go-bindings-wmi.svg)](https://pkg.go.dev/github.com/deploymenttheory/go-bindings-wmi)
[![CI](https://github.com/deploymenttheory/go-bindings-wmi/actions/workflows/ci.yml/badge.svg)](https://github.com/deploymenttheory/go-bindings-wmi/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Typed Go bindings for **WMI / CIM classes**, generated from a committed
schema snapshot and running on
[go-bindings-win32](https://github.com/deploymenttheory/go-bindings-win32)'s
generated WMI COM interfaces. A member of the deploymenttheory Windows
bindings family — same doctrine as its winmd-based siblings, applied to a
different metadata source: **capture → committed snapshot → deterministic
codegen → live query.**

```go
import (
	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

svc, err := wmi.Connect(`root\cimv2`)
if err != nil { /* ... */ }
defer svc.Close()

os, err := cimv2.QueryWin32OperatingSystem(svc, "")   // typed []Win32OperatingSystem
fmt.Println(os[0].Caption, os[0].BuildNumber)

disks, _ := cimv2.QueryWin32LogicalDisk(svc, "DriveType = 3")
for _, d := range disks {
	fmt.Printf("%s %d/%d bytes free\n", d.DeviceID, d.FreeSpace, d.Size)
}
```

No stringly-typed WQL result maps: one Go struct per CIM class with typed
fields (CIM types → Go types, arrays → slices), plus a generated
`Query<Class>` helper that runs the WQL and unmarshals each instance.

## How it's built

Unlike the winmd sisters, the metadata source is the **live CIM repository**,
so acquisition is a capture step rather than a NuGet download:

```sh
go run ./cmd/capture     # introspect the live repository → metadata/cim/<ns>.json (committed)
go run ./cmd/generate    # snapshot → bindings/cim/<ns> (self-cleaning, byte-deterministic)
```

The committed snapshot (`metadata/cim/root.cimv2.json`) is the winmd
equivalent: codegen is fully offline and deterministic from it, and CI
regenerates and diffs to enforce that. Capturing a fresh snapshot is a
deliberate, reviewed act (schema varies by host OS build — recorded in the
snapshot's provenance).

## Runtime

`runtime/wmi` is hand-written on go-bindings-win32: it connects through
`IWbemLocator`/`IWbemServices`, runs WQL via `ExecQuery`, walks
`IEnumWbemClassObject` (using the `(HRESULT, error)` informational-success
shape to detect end-of-enumeration), and decodes result `VARIANT`s into Go
values. This is also the seed of a general OLE-automation ergonomics layer
(VARIANT/BSTR/SAFEARRAY handling).

## Scope (v0)

A curated set of core `root\cimv2` inventory classes (OS, ComputerSystem,
BIOS, Processor, LogicalDisk, PhysicalMemory, NetworkAdapterConfiguration,
Service). Grow coverage by extending the curated list in `cmd/capture` and
re-capturing; other namespaces (`root\StandardCimv2`, MDM bridge classes) are
additive.

## License

[MIT](LICENSE).
