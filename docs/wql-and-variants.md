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
| (array of the above) | `[]T` |
| unknown / object refs | `any` |

`datetime` is currently surfaced as its raw DMTF string (e.g.
`20260714120000.000000+000`); parse at the call site if you need a `time.Time`.

## VARIANT decoding

WMI returns values as COM `VARIANT`s. The runtime decodes the common scalar
types into Go values (`string`, `int64`, `uint64`, `bool`, `float64`, or `nil`
for `VT_EMPTY`/`VT_NULL`); unsupported or array VARIANTs decode to `nil`. The
typed `Query<Class>` helpers then type-assert each property into its struct
field, so a value that doesn't match its declared type is simply left zero
rather than erroring.

## End-of-enumeration

Iterating a result set uses go-bindings-win32's informational-success shape:
`IEnumWbemClassObject.Next` returns `(win32.HRESULT, error)`, and the runtime
treats `WBEM_S_FALSE` (a *success* HRESULT) as end-of-data rather than an error
— the reason that `(HRESULT, error)` shape exists.
