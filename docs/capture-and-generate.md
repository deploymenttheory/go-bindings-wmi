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
go run ./cmd/capture -osbuild 26200 -captured 2026-07-14
```

By default this introspects **every class** in `root\cimv2` (~1,300) and writes
`metadata/cim/root.cimv2.json`: each class with its properties, CIM types, and
array flags, sorted for determinism, under a provenance header (OS build +
date). Flags:

- `-namespace root\StandardCimv2` — a different CIM namespace (its own package).
- `-classes Win32_OperatingSystem,Win32_Service` — narrow to specific classes.

The snapshot is **committed**. Capturing a new one is a deliberate, reviewed act
— the schema varies by host OS build, which is why provenance records the build.

## Generate

```sh
go run ./cmd/generate
```

Reads `metadata/cim/*.json` and writes `bindings/cim/<leaf>/<leaf>_classes.go`:
one struct per class plus a `Query<Class>` helper. Self-cleaning (stale files
pruned) and **byte-deterministic** — running it twice produces no diff, which CI
enforces with `git diff --exit-code`. Codegen is fully offline from the
committed snapshot; it never touches the live system.

## Robustness at scale

Capturing the whole namespace exercises the long tail of CIM: classes whose
property names collide as Go identifiers (deduplicated, first wins), classes
with zero own properties, and exotic CIM types (mapped to `any`). The generator
handles all of these rather than failing the run.
