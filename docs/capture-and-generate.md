# Capture and generate

WMI metadata is the schema of the **live CIM repository**, not a downloadable
file. So this repo's pipeline has an acquisition (capture) step whose output —
a committed JSON snapshot — is the winmd-equivalent.

```
live CIM repository → committed snapshot → deterministic codegen → bindings
   cmd/capture       metadata/cim/*.json    cmd/generate           bindings/cim
```

## Capture

```sh
go run ./cmd/capture
```

By default this introspects **every class** in `root\cimv2` (~1,300) and writes
`metadata/cim/root.cimv2.json`: each class with its properties, CIM types,
array flags, and key qualifiers, sorted for determinism, under a provenance
header. Provenance auto-fills from the capture host (OS build from
`Win32_OperatingSystem`, date from the clock); `-osbuild`/`-captured` override.
Other flags:

- `-namespace root\StandardCimv2` — a different CIM namespace (its own package).
- `-classes Win32_OperatingSystem,Win32_Service` — narrow to specific classes.

Some namespaces gate schema reads: the MDM bridge (`root\cimv2\mdm\dmmap`)
returns `WBEM_E_ACCESS_DENIED` unless captured from the **SYSTEM** account
(elevation alone is not enough). Capture surfaces a hint pointing at
`scripts/Capture-MdmBridge.ps1`, which runs the capture as SYSTEM — see
[the MDM bridge doc](mdm-bridge.md). Namespace leaves must be unique across
snapshots — the generator fails fast on package-name collisions.

The snapshot is **committed**. Capturing a new one is a deliberate, reviewed act
— the schema varies by host OS build, which is why provenance records the build.
The weekly `schema-update.yml` workflow automates the capture but keeps the
review: it opens a PR whose body is the semantic diff, and skips when only
provenance changed.

## Generate

```sh
go run ./cmd/generate
```

Reads `metadata/cim/*.json`, validates each snapshot, and writes
`bindings/cim/<leaf>/` split by construct, mirroring the winmd sisters'
file layout:

- `doc.go` — the package doc
- `<leaf>_structs.go` — one struct per class (`WMIPath` first: the instance's
  `__PATH`, ready for the method wrappers)
- `<leaf>_enums.go` — named enumeration types for `Values`/`ValueMap`
  qualifiers (with `String()` on integer enums) and bitmask types for
  `BitValues`/`BitMap`
- `<leaf>_constants.go` — flat fallback constants (array enum properties, and
  enums whose type name a class already claimed)
- `<leaf>_rows.go` — `<Class>FromRow` decoders (`wmi.Row` → typed struct;
  shared by queries, `GetInstance`, `Associators`, events, `ParseObjectText`)
- `<leaf>_queries.go` — the `Query<Class>`, `QueryOne<Class>`, and
  `Get<Class>` helpers
- `<leaf>_methods.go` — method wrappers and their result structs; async
  `(ReturnValue, Job)` methods get `result.Wait(ctx, svc)`, plain
  `ReturnValue` methods get `result.Err()`

Empty files are not written. Self-cleaning (stale files pruned) and
**byte-deterministic** — running it twice produces no diff, which CI enforces
with `git diff --exit-code`. Codegen is fully offline from the committed
snapshot; it never touches the live system.

## Validate and diff

```sh
go run ./cmd/generate validate                      # structural invariants
go run ./cmd/generate diff old.json new.json        # semantic diff (markdown)
```

`validate` checks what the pipeline relies on: sorted/unique classes and
properties, normalized CIM types, provenance present. `diff` reports classes
and properties added/removed/changed between two captures of the same
namespace — the review artifact for a snapshot refresh. When only provenance
differs it prints exactly "No schema changes.", which the scheduled workflow
keys off to skip churn PRs.

## Robustness at scale

Capturing the whole namespace exercises the long tail of CIM: classes whose
property names collide as Go identifiers (deduplicated, first wins), classes
with zero own properties, and exotic CIM types (mapped to `any`). The generator
handles all of these rather than failing the run.
