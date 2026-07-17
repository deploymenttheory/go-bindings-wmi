# CLAUDE.md

Guidance for Claude Code (claude.ai/code) working in this repository.

## What this is

`go-bindings-wmi` provides typed Go bindings for **WMI / CIM classes**,
generated from a committed schema snapshot and running on `go-bindings-win32`'s
generated WMI COM interfaces. It is a member of the deploymenttheory Windows
bindings family, applying the same generate-from-committed-metadata doctrine to
a **different metadata source: the live CIM repository**.

## The capture doctrine

Unlike the winmd sisters (whose metadata is a NuGet download), WMI's metadata
lives in the running system's CIM repository. So acquisition is a **capture**
step, and the committed snapshot is the winmd-equivalent:

```
live CIM repository → committed snapshot → deterministic codegen → typed query
   (cmd/capture)     (metadata/cim/*.json)  (cmd/generate)         (runtime/wmi)
```

- **`cmd/capture`** (Windows) introspects the live repository via the generated
  COM interfaces and writes a deterministic, sorted schema snapshot
  (`metadata/cim/root.cimv2.json`: class → properties, CIM types, array/key
  flags) with a provenance header (OS build + date, auto-filled from the
  capture host). By default it captures **every class** in the namespace
  (~1,300 for `root\cimv2`); `-classes a,b,c` narrows.
- **`cmd/generate`** turns the snapshot into `bindings/cim/<ns>`, split by
  construct like the winmd sisters (`doc.go`, `<ns>_structs.go`,
  `<ns>_queries.go`, `<ns>_methods.go`; empty files not written): one struct
  per class (CIM types → Go types, arrays → slices), `Query<Class>`/
  `Get<Class>` helpers that decode via the runtime's coercers, and typed
  method wrappers. Self-cleaning, byte-deterministic —
  CI regenerates and diffs. It validates every snapshot before generating, and
  has two snapshot subcommands:
  - `go run ./cmd/generate validate [dir]` — structural invariants (sorted,
    unique, normalized types, provenance present)
  - `go run ./cmd/generate diff <old> <new>` — semantic schema diff as
    markdown (backs the scheduled update PR body; prints the stable sentence
    "No schema changes." when only provenance differs)
- **`runtime/wmi`** (hand-written, on go-bindings-win32) connects, runs WQL,
  walks the enumerator, and decodes VARIANTs (scalars and SAFEARRAYs).

Capturing a fresh snapshot is a **deliberate, reviewed act** (schema varies by
host OS build — recorded in the snapshot's provenance), analogous to
`fetch-metadata` in the winmd repos. The weekly `schema-update.yml` workflow
automates the capture but keeps the review: it opens a PR whose body is the
semantic diff, and skips entirely when the schema is unchanged. Codegen itself
is fully offline from the committed snapshot.

## Why the CLI is simpler than win32/wdk

The winmd generators have more subcommands (`fetch-metadata`/`ingest`/
`bindings`/`abitest`/`list`), a `cmd/inspect`, and a diagnostics-baseline
ratchet. WMI adopted the family's `validate` and `diff` verbs (they are what
makes "capture is a reviewed act" enforceable) but deliberately not the rest:

- No NuGet `fetch-metadata` — the metadata is captured, not downloaded.
- No `ingest`/cross-namespace resolution — CIM classes are flat; there is no
  cross-assembly type graph to project.
- No `abitest` — there are no C struct ABIs to assert. The analogue is the
  acceptance sweep (`acceptance/sweep_test.go`): the snapshot is host-specific
  by design, so the sweep asserts the introspection/decode pipeline works
  against whatever the host repository holds, never that the host matches the
  snapshot.
- No diagnostics ratchet — the generator degrades unknown CIM types to `any`
  rather than skipping, so there is no "skipped construct" set to track;
  degradations are visible in the snapshot diff instead.

Keep it this way unless the pipeline genuinely grows a stage that needs
tracking.

## Packages

- **`runtime/wmi`** — `Connect`/`ConnectWith` (remote host + credentials,
  applied to the DCOM proxy blanket) → `Service`; `Query`/`QuerySeq`/
  `QueryContext` → rows; instance CRUD (`GetInstance`/`CreateInstance`/
  `UpdateInstance`/`DeleteInstance`, `ErrNotFound`) and association traversal
  (`Associators`/`References`); `ExecMethod`/`ExecMethodContext` (typed by the
  generated wrappers); `SubscribeEvents`; `ClassProperties`/`ClassNames`/`ClassMethods` for schema
  introspection (used by capture); VARIANT decode (scalars, SAFEARRAYs, and
  embedded objects → nested `Row`) and encode (method in-parameters,
  including slices and embedded instances via `wmi.Instance`); DMTF datetime
  parsing. Uses
  go-bindings-win32's `IWbemLocator`/`IWbemServices`/`IEnumWbemClassObject`
  (the `(HRESULT, error)` shape detects end-of-enum/timeouts), `VARIANT`,
  `BSTR`, SAFEARRAY, and COM init. This is also the seed of a general
  OLE-automation ergonomics layer.
- **`internal/cimschema`** — the snapshot format + the CIM→Go type mapping.
- **`bindings/cim/<ns>`** — generated typed classes + `Query<Class>` helpers.
  Never hand-edited.

## CSP policies (DDF v2) live elsewhere

The DDF v2 pipeline (fetchddf → metadata/csp → gencsp → bindings/csp) that
used to live here moved to its own project:
[go-sdk-windowscsp](https://github.com/deploymenttheory/go-sdk-windowscsp)
generates a full LCRUD SDK from the DDF v2 schema. This repo keeps the WMI
side: `bindings/cim/dmmap` is the MDM bridge namespace — the local WMI face
of the same CSP surface (SYSTEM-gated; see docs/mdm-bridge.md) — and the
natural place to build go-sdk-windowscsp's `client.Client` transport for
on-device execution.

## Growing coverage

To capture another namespace: `go run ./cmd/capture -namespace root\StandardCimv2`
then `go run ./cmd/generate`. Each namespace becomes its own package
(`bindings/cim/<leaf>`). The generator handles arbitrary classes; duplicate Go
field/class names and zero-property classes are handled.

The MDM bridge (`root\cimv2\mdm\dmmap`, CSP policy classes) needs the SYSTEM
account, not just elevation — capture surfaces a hint and
`scripts/Capture-MdmBridge.ps1` runs it as SYSTEM via a transient scheduled
task (see [docs/mdm-bridge.md](docs/mdm-bridge.md)).
