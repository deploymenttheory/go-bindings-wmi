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

## DDF vs the MDM WMI bridge

The DDF gives the **canonical, versioned schema** — what policies exist,
their types, allowed values, and applicability — independent of any device.
The MDM WMI bridge (`root\cimv2\mdm\dmmap`) is the **local runtime** for
reading and writing those policies on a device. A policy descriptor's `URI`
and `Format` tell you how to drive it through the bridge. The two are
complementary: DDF for the schema, the bridge for execution.
