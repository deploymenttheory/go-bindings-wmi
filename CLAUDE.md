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
  (`metadata/cim/root.cimv2.json`: class → properties, CIM types, array flags)
  with a provenance header (OS build + date). By default it captures **every
  class** in the namespace (~1,300 for `root\cimv2`); `-classes a,b,c` narrows.
- **`cmd/generate`** turns the snapshot into `bindings/cim/<ns>`: one struct per
  class (CIM types → Go types, arrays → slices) plus a `Query<Class>` helper.
  Self-cleaning, byte-deterministic — CI regenerates and diffs.
- **`runtime/wmi`** (hand-written, on go-bindings-win32) connects, runs WQL,
  walks the enumerator, and decodes VARIANTs.

Capturing a fresh snapshot is a **deliberate, reviewed act** (schema varies by
host OS build — recorded in the snapshot's provenance), analogous to
`fetch-metadata` in the winmd repos. Codegen itself is fully offline from the
committed snapshot.

## Why the CLI is intentionally simpler than win32/wdk

The winmd generators have subcommands (`fetch-metadata`/`ingest`/`bindings`/
`abitest`/`validate`/`diff`/`list`), a `cmd/inspect`, and a
diagnostics-baseline ratchet. WMI has **none of these, by design**:

- No NuGet `fetch-metadata` — the metadata is captured, not downloaded.
- No `ingest`/`validate`/cross-namespace resolution — CIM classes are flat;
  there is no cross-assembly type graph to project or validate.
- No `abitest` — there are no C struct ABIs to assert (values come back as
  VARIANTs, decoded at runtime, not as memory-mapped structs).
- No diagnostics ratchet — the generator degrades unknown CIM types to `any`
  rather than skipping, so there is no "skipped construct" set to track.

So `cmd/generate` is a single command (optional positional metadata dir), and
`cmd/capture` is the acquisition tool. Keep it this way unless the pipeline
genuinely grows a stage that needs tracking.

## Packages

- **`runtime/wmi`** — `Connect(namespace)` → `Service`; `Query(wql)` → `[]Row`;
  `ClassProperties`/`ClassNames` for schema introspection (used by capture);
  VARIANT decoding. Uses go-bindings-win32's `IWbemLocator`/`IWbemServices`/
  `IEnumWbemClassObject` (the `(HRESULT, error)` shape detects end-of-enum),
  `VARIANT`, `BSTR`, and COM init. This is also the seed of a general
  OLE-automation ergonomics layer.
- **`internal/cimschema`** — the snapshot format + the CIM→Go type mapping.
- **`bindings/cim/<ns>`** — generated typed classes + `Query<Class>` helpers.
  Never hand-edited.

## Growing coverage

To capture another namespace: `go run ./cmd/capture -namespace root\StandardCimv2`
then `go run ./cmd/generate`. Each namespace becomes its own package
(`bindings/cim/<leaf>`). The generator handles arbitrary classes; duplicate Go
field/class names and zero-property classes are handled.
