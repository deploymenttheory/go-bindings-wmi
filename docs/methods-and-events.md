# Methods, events, and query shapes

## Invoking CIM methods

Method schemas are captured into the snapshot alongside properties, so the
generator emits one typed wrapper per method plus a result struct holding
the out-parameters (always including `ReturnValue`).

**Static methods** target the class:

```go
res, err := cimv2.Win32ProcessCreate(svc, `notepad.exe`, "", nil)
// res.ProcessId, res.ReturnValue
```

**Instance methods** take the instance's object path — the `__PATH` system
property every queried row carries:

```go
rows, _ := svc.Query(`SELECT * FROM Win32_Service WHERE Name = 'Spooler'`)
path := wmi.AsString(rows[0]["__PATH"])
res, _ := cimv2.Win32ServiceStopService(svc, path)
```

**Zero-value semantics:** generated wrappers omit zero-valued in-parameters
(`""`, `0`, `false`, `nil`), leaving them NULL so the provider applies its
defaults. When an explicit zero matters, drop to the runtime map API, which
sends exactly what you give it and skips only `nil`:

```go
out, err := svc.ExecMethod(path, "SetPriority", map[string]any{"Priority": int32(0)})
```

Values encode per WMI's conventions: integers as `VT_I4` when they fit 32
bits, as decimal strings otherwise (WMI's own 64-bit shape), strings as
BSTR, and slices (string, bool, all numeric widths) as SAFEARRAYs of the
matching element type.

**Embedded-object parameters** (typed `wmi.Row` in the wrappers) are built
with `wmi.Instance` — the runtime spawns the class instance, puts the
properties (recursively, so objects nest), and passes it as `VT_UNKNOWN`:

```go
startup := wmi.Instance("Win32_ProcessStartup", map[string]any{
    "ShowWindow": uint16(0), // SW_HIDE
})
res, _ := cimv2.Win32ProcessCreate(svc, "cmd.exe /c exit 0", "", startup)
```

A `wmi.Row` returned by a query also works — it already carries `__CLASS`.

Abstract base classes (`CIM_*`) declare methods their providers often don't
implement; the wrappers are generated faithfully and the provider's error
surfaces at call time.

## Event subscriptions

`SubscribeEvents` runs a WQL event query semisynchronously; poll with
`Next`, which returns `wmi.ErrEventTimeout` when the wait elapses (just poll
again) and `io.EOF` if the subscription ends:

```go
sub, err := svc.SubscribeEvents(
    "SELECT * FROM __InstanceCreationEvent WITHIN 2 WHERE TargetInstance ISA 'Win32_Process'")
if err != nil { /* ... */ }
defer sub.Close()

for {
    event, err := sub.Next(5 * time.Second)
    if errors.Is(err, wmi.ErrEventTimeout) {
        continue
    }
    if err != nil {
        break
    }
    // intrinsic events embed the instance in TargetInstance, decoded to a
    // nested wmi.Row; extrinsic events carry their own properties.
    instance := wmi.AsRow(event["TargetInstance"])
    fmt.Println(instance["__CLASS"], instance["Name"])
}
```

## Streaming and cancellation

`Query` materializes every row. For large result sets, stream:

```go
for row, err := range svc.QuerySeq("SELECT * FROM Win32_NTLogEvent") {
    if err != nil { /* ... */ }
    // breaking early releases the enumerator
}
```

`QueryContext` adds cancellation by polling the enumerator in short waits:

```go
rows, err := svc.QueryContext(ctx, "SELECT * FROM Win32_Product") // honors ctx
```

Note WMI has no server-side cancel for semisynchronous queries —
cancellation abandons the enumerator; the provider may keep working
briefly.

## Remote connections

```go
svc, err := wmi.ConnectWith(`root\cimv2`, wmi.ConnectOptions{
    Host: "server01",
    User: `DOMAIN\admin`, Password: "...",
})
```

The zero `ConnectOptions` is exactly `Connect`. WMI rejects explicit
credentials on local connections. All Service operations remain bound to
the connecting goroutine (see the thread-affinity note on `Connect`).
