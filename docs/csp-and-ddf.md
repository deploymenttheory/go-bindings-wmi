# CSP policy bindings (DDF v2)

Alongside the WMI/CIM bindings, the repo generates a typed catalog of the
entire Windows **MDM policy / CSP surface** from Microsoft's canonical
**Device Description Framework (DDF) v2** files — ~5,100 configuration
service provider settings across ~313 areas, versioned to a specific DDF
release.

This is the same doctrine as the winmd sisters, applied to a different
artifact: **the DDF v2 zip is the winmd-NuGet of the MDM surface** — a
pinned, downloadable, machine-readable metadata release. Acquisition is a
pipeline stage; codegen is offline and deterministic from the committed
snapshots.

```
DDF v2 zip (pinned, versioned)  →  committed snapshots  →  typed bindings
   cmd/fetchddf                     metadata/csp/*.json     cmd/gencsp → bindings/csp/<area>
```

## Fetch

```sh
go run ./cmd/fetchddf                    # download the pinned release, verify sha256, parse
go run ./cmd/fetchddf -zip local.zip     # parse an already-downloaded zip (offline)
```

`fetchddf` downloads the pinned DDF v2 zip, verifies its SHA-256, parses
every CSP/policy-area DDF into `metadata/csp/<area>.json`, and records
`metadata/csp/PROVENANCE.json` (release, source URL, digest, date). Adopting
a new Microsoft drop is a deliberate, reviewed act: bump the pinned
`defaultRelease`/`defaultURL` in `cmd/fetchddf` and re-run. The weekly
`ddf-update.yml` workflow re-fetches the pin and opens a PR when the surface
changes.

## Generate

```sh
go run ./cmd/gencsp
```

Reads `metadata/csp/*.json` and writes `bindings/csp/<area>/`: one
`csp.Policy` descriptor per leaf node plus typed constants for enumerated
values. Self-cleaning and byte-deterministic (CI regenerates and diffs).
The CSP bindings are **pure Go** — no Windows dependency — so they build and
test on any OS.

## Using the catalog

Each policy is a typed descriptor carrying its OMA-DM URI, value format,
access verbs, default, applicability (min OS build, CSP version, edition
allow-list, Entra-join requirement), deprecation, and allowed values:

```go
import "github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybitlocker"

p := policybitlocker.EncryptionMethod
fmt.Println(p.URI)         // ./Device/Vendor/MSFT/Policy/Config/Bitlocker/EncryptionMethod
fmt.Println(p.MinOSBuild)  // 10.0.14393
for _, all := range policybitlocker.All { /* enumerate the area */ }
```

Enumerated policies also get typed constants:

```go
import "github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policyabovelock"

_ = policyabovelock.AllowActionCenterNotificationsAllowed // int64 = 1
```

Package naming: policy areas are prefixed `policy` (`Bitlocker_AreaDDF` →
`policybitlocker`) so they never collide with a same-named standalone CSP
(`BitLocker` → `bitlocker`).

## Executing policies (LCRUD via the bridge)

The DDF gives the schema; the MDM WMI bridge (`root\cimv2\mdm\dmmap`) is the
local runtime. `cmd/gencsp` joins the two, cross-checking each DDF node
against the bridge classes captured from a live device, and attaches a
`csp.Bridge` mapping to every policy the bridge exposes (~1,760 across ~1,500
native Policy settings and ~260 non-Policy CSP settings):

- **Native Policy areas** (`metadata/csp/bridge-policy-classes.json`) use the
  regular `MDM_Policy_Config01_<Area>02` / `Result01` convention.
- **Non-Policy CSPs** (`metadata/csp/bridge-csp-classes.json`) have irregular
  class names, so the join matches each instance-node to its captured class
  by normalized name, validated by the property — covering the flat,
  statically-keyable CSPs (DevDetail, DeviceStatus, WindowsLicensing,
  BitLocker, AssignedAccess, …).

The key convention is uniform and verified against device truth: `InstanceID`
is the instance-node's name, `ParentID` the scope-relative path of its parent
(`./Vendor/MSFT/Policy/Config` for a Policy area, `./Vendor/MSFT` for
WindowsLicensing, `./DevDetail` for DevDetail/Ext). **Dynamic-instance nodes
(`{GUID}`) carry no static mapping** — they need a runtime instance id;
construct a `csp.Bridge` by hand for those. Bridge-backed policies report
`Executable() == true` and can be read, set, and deleted through
`runtime/csp`:

```go
import "github.com/deploymenttheory/go-bindings-wmi/runtime/csp"

svc, err := csp.Connect()          // opens root\cimv2\mdm\dmmap — needs SYSTEM
defer svc.Close()

v, err := csp.Read(svc, policybrowser.AllowCookies)          // R (effective value)
err = csp.Set(svc, policybrowser.AllowCookies, int64(2))     // C/U (Add/Replace)
err = csp.Delete(svc, policybrowser.AllowCookies)            // D (unmanage the area)
```

The mapping is grounded in real captured bridge classes, so the projection
(`MDM_Policy_Config01_<Area>02`, keyed `ParentID`+`InstanceID`, the leaf as
the property) is exact, not guessed. Non-Policy CSPs and ADMX-ingested
policies carry no `Bridge` yet (`Executable() == false`) — their schema is
known but they are not drivable through this runtime.

**Constraints:** the bridge answers only the **SYSTEM** account, and writes
**mutate real device configuration**. A policy area's Config instance is a
singleton, so `Delete` unmanages the whole area — prefer `Set`-to-default to
clear one setting. See `examples/csp-policy` for a read-only-by-default,
self-restoring cycle.

## DDF vs the MDM WMI bridge

The DDF gives the **canonical, versioned schema** — what policies exist,
their types, allowed values, and applicability — independent of any device.
The MDM WMI bridge is the **local runtime** for reading and writing those
policies. Complementary: DDF for the schema, the bridge for execution.
