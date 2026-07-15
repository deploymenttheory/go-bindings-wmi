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
`Query<Class>` helper that runs the WQL and unmarshals each instance — and
one typed wrapper per CIM method:

```go
owner, _ := cimv2.Win32ProcessGetOwner(svc, path)   // instance method via __PATH
res, _ := cimv2.Win32ProcessCreate(svc, "notepad.exe", "", nil) // static method
```

The runtime also provides the full instance lifecycle
(`CreateInstance`/`UpdateInstance`/`DeleteInstance`/`GetInstance`, plus
generated `Get<Class>` key lookups), association traversal
(`Associators`/`References`), event subscriptions (`SubscribeEvents`),
streaming (`QuerySeq`) and cancellable (`QueryContext`) queries, remote
connections (`ConnectWith`), and DMTF datetime parsing (`ParseDMTF`).

## How it's built

Unlike the winmd sisters, the metadata source is the **live CIM repository**,
so acquisition is a capture step rather than a NuGet download:

```sh
go run ./cmd/capture                    # introspect the live repository → metadata/cim/<ns>.json (committed)
go run ./cmd/generate                   # snapshot → bindings/cim/<ns> (validated, self-cleaning, byte-deterministic)
go run ./cmd/generate validate          # snapshot structural invariants
go run ./cmd/generate diff old new      # semantic schema diff (markdown)
```

The committed snapshot (`metadata/cim/root.cimv2.json`) is the winmd
equivalent: codegen is fully offline and deterministic from it, and CI
regenerates and diffs to enforce that. Capturing a fresh snapshot is a
deliberate, reviewed act (schema varies by host OS build — recorded in the
snapshot's provenance, auto-filled from the capture host). A weekly
`schema-update` workflow captures on the CI runner and opens a PR — body: the
semantic diff — only when the schema actually changed.

## Runtime

`runtime/wmi` is hand-written on go-bindings-win32: it connects through
`IWbemLocator`/`IWbemServices`, runs WQL via `ExecQuery`, walks
`IEnumWbemClassObject` (using the `(HRESULT, error)` informational-success
shape to detect end-of-enumeration), and decodes result `VARIANT`s into Go
values. This is also the seed of a general OLE-automation ergonomics layer
(VARIANT/BSTR/SAFEARRAY handling).

## Coverage

Four namespaces are captured, each its own generated package
(`go run ./cmd/capture` enumerates a whole namespace by default; `-classes a,b`
narrows):

- **`root\cimv2`** (~1,300 classes) → `bindings/cim/cimv2` — the classic
  inventory surface (Win32_*, CIM_*).
- **`root\StandardCimv2`** (372) → `bindings/cim/standardcimv2` — modern
  networking (MSFT_NetAdapter, MSFT_NetIPAddress, MSFT_NetRoute, …).
- **`root\SecurityCenter2`** (69) → `bindings/cim/securitycenter2` —
  registered AV / firewall / anti-spyware products (client SKUs only).
- **`root\cimv2\mdm\dmmap`** (467, incl. ~400 `MDM_*`) → `bindings/cim/dmmap`
  — the MDM bridge: Windows' CSP policy surface (AppLocker, ActiveSync,
  BitLocker, …) as WMI classes. Capturing **and querying** it require the
  **SYSTEM** account; see [docs/mdm-bridge.md](docs/mdm-bridge.md) and
  `scripts/Capture-MdmBridge.ps1`, which automates the capture.

Any other namespace (`root\Microsoft\Windows\*`, …) is just
`go run ./cmd/capture -namespace <ns>` then regenerate.

### CSP policy catalog (DDF v2)

A second pipeline generates a typed catalog of the entire Windows **MDM
policy / CSP surface** (~5,100 settings across ~313 areas) from Microsoft's
canonical, versioned **DDF v2** files — the winmd-NuGet analogue for MDM.
Acquisition is a pinned download with provenance; codegen is offline and the
bindings are **pure Go** (no Windows dependency):

```sh
go run ./cmd/fetchddf     # download the pinned DDF v2 zip, verify sha256 → metadata/csp/ (committed)
go run ./cmd/gencsp       # snapshots → bindings/csp/<area> (typed policy descriptors + enum constants)
```

```go
import "github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybitlocker"
p := policybitlocker.EncryptionMethod   // URI, format, applicability, allowed values, …
```

See [docs/csp-and-ddf.md](docs/csp-and-ddf.md). The DDF gives the canonical
schema; the MDM WMI bridge is the local runtime for driving those policies.

## Examples & docs

- [`examples`](examples) — runnable programs, one per surface: typed-query
  inventory, instance CRUD, association walking, method invocation, and
  live event watching.
- [Getting started](docs/getting-started.md)
- [Capture and generate](docs/capture-and-generate.md) — the metadata pipeline
- [WQL and VARIANTs](docs/wql-and-variants.md) — types, coercion, decoding
- [Methods, events, and query shapes](docs/methods-and-events.md) — ExecMethod,
  subscriptions, streaming, remote
- [Instances and associations](docs/instances-and-associations.md) — CRUD,
  key lookups, ASSOCIATORS OF
- [CSP policy bindings (DDF v2)](docs/csp-and-ddf.md) — the DDF pipeline:
  fetch, parse, the typed policy catalog

- [`CLAUDE.md`](CLAUDE.md) — the capture doctrine and why the CLI is minimal

## Related projects

Part of the deploymenttheory Windows bindings family:

- [go-winmd](https://github.com/deploymenttheory/go-winmd) — the shared ECMA-335 `.winmd` metadata reader
- [go-bindings-win32](https://github.com/deploymenttheory/go-bindings-win32) — the Win32 API surface — functions, structs, enums, COM
- [go-bindings-wdk](https://github.com/deploymenttheory/go-bindings-wdk) — the Windows Driver Kit / user-mode Native API surface
- **go-bindings-wmi** — typed WMI/CIM classes *(this repo)*
- [go-bindings-winrt](https://github.com/deploymenttheory/go-bindings-winrt) — WinRT bindings (in progress)

## License

[MIT](LICENSE).
