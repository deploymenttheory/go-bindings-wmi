# WQL and VARIANTs

## WQL

Queries use WQL (a SQL subset). The generated `Query<Class>(svc, where)` helpers
build `SELECT * FROM <Class>` and append your `WHERE` clause:

```go
cimv2.QueryWin32Service(svc, "State = 'Running' AND StartMode = 'Auto'")
```

For anything WQL supports beyond a single class projection, use the runtime
directly: `svc.Query("SELECT Name FROM Win32_Process WHERE WorkingSetSize > 100000000")`.

## CIM → Go types

The generator maps CIM property types to Go field types:

| CIM type | Go |
|---|---|
| string, datetime | `string` |
| boolean | `bool` |
| sint8/16/32/64 | `int8`/`int16`/`int32`/`int64` |
| uint8/16/32/64 | `uint8`/`uint16`/`uint32`/`uint64` |
| real32/64 | `float32`/`float64` |
| reference (REF) | `string` (the object path) |
| object (embedded) | `wmi.Row` |
| (array of the above) | `[]T` |
| unknown | `any` |

## Enumeration constants

Properties carrying the CIM `Values`/`ValueMap` qualifiers (captured into the
snapshot) generate named constants into `<ns>_constants.go`, one block per
property: `cimv2.Win32LogicalDiskDriveTypeLocalDisk` (`uint32 = 3`),
`cimv2.Win32ServiceStartModeAuto` (`"Auto"`), and so on. Integer enums take
their value from the `ValueMap`; a negative map entry on an unsigned property
is reinterpreted as its two's-complement bit pattern (matching how that value
decodes from a VARIANT — `-1` on a `uint32` becomes `4294967295`). Range
entries (`128..255`) and free-form text have no single value and are skipped.

`datetime` is surfaced as its raw DMTF string (e.g.
`20260714120000.000000+060`). The runtime provides parsers:
`wmi.ParseDMTF` → `time.Time` (offset preserved as a fixed zone) and
`wmi.ParseDMTFInterval` → `time.Duration` for interval values
(`ddddddddHHMMSS.mmmmmm:000`).

## VARIANT decoding

WMI returns values as COM `VARIANT`s. The runtime decodes every scalar type
WMI produces into widened Go values (`string`, `int64`, `uint64`, `bool`,
`float64`, or `nil` for `VT_EMPTY`/`VT_NULL`), decodes SAFEARRAY values
into `[]any` of those widened elements, and decodes embedded CIM objects
(`VT_UNKNOWN`) into nested `wmi.Row` maps carrying their `__CLASS`. The typed `Query<Class>` helpers then
*coerce* each property into its declared struct field (`wmi.AsUint32`,
`wmi.AsStringSlice`, …) rather than type-asserting — necessary because WMI
does not return values in the CIM-declared width: most integers arrive as
`VT_I4` and 64-bit values (disk sizes, memory) arrive as BSTR strings. A
value that cannot be coerced is left zero rather than erroring.

## End-of-enumeration

Iterating a result set uses go-bindings-win32's informational-success shape:
`IEnumWbemClassObject.Next` returns `(win32.HRESULT, error)`, and the runtime
treats `WBEM_S_FALSE` (a *success* HRESULT) as end-of-data rather than an error
— the reason that `(HRESULT, error)` shape exists.
