# Methods, events, and query shapes

## Invoking CIM methods

Method schemas are captured into the snapshot alongside properties
(including parameter enumeration qualifiers), so the generator emits one
typed wrapper per method plus a result struct holding the out-parameters
(always including `ReturnValue`, enum-typed when the schema declares its
values).

**Static methods** target the class:

```go
res, err := cimv2.Win32ProcessCreate(svc, wmi.Ptr("notepad.exe"), nil, nil)
if err := res.Err(); err != nil { /* non-zero ReturnValue as *wmi.JobError */ }
// res.ProcessId
```

**Instance methods** take the instance's object path — every generated
struct carries it as `WMIPath`:

```go
spooler, _ := cimv2.GetWin32Service(svc, "Spooler")
res, _ := cimv2.Win32ServiceStopService(svc, spooler.WMIPath)
```

**In-parameter semantics:** scalar in-parameters are pointers. `nil` omits
the parameter (NULL — the provider applies its default); a non-nil value is
always sent, **including zeros**. `wmi.Ptr` builds pointers inline:

```go
res, err := cimv2.Win32ProcessSetPriority(svc, path, wmi.Ptr(cimv2.Win32ProcessSetPriorityPriorityIdle))
```

Enum-qualified parameters get named types
(`<Class><Method><Param>`) with generated constants, so valid values are
discoverable and comparisons are typed. Slices, embedded objects
(`wmi.Row`), and `any` parameters keep their natural nil state and are
passed directly.

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
res, _ := cimv2.Win32ProcessCreate(svc, wmi.Ptr("cmd.exe /c exit 0"), nil, startup)
```

A `wmi.Row` returned by a query also works — it already carries `__CLASS`.
Arrays of embedded objects (`[]wmi.Row`) encode too, as a SAFEARRAY of
instances.

For a cancellable call, `ExecMethodContext(ctx, path, method, in)` runs the
method semisynchronously and checks `ctx` between short completion polls.
WMI cannot abort a provider mid-call, so cancellation abandons the call —
the provider may finish it anyway.

Abstract base classes (`CIM_*`) declare methods their providers often don't
implement; the wrappers are generated faithfully and the provider's error
surfaces at call time.

## Async methods and jobs

Providers like Hyper-V return the CIM async pair: `ReturnValue` 0 means
done, 4096 means a `CIM_ConcreteJob` was started (its REF is in the `Job`
out-parameter). Results with that shape get a generated `Wait` that
resolves the whole contract — immediate success, job polling to a terminal
state, and failure as a `*wmi.JobError` carrying the job's state, error
code, and description:

```go
res, err := v2.MsvmComputerSystemRequestStateChange(svc, vm.WMIPath,
    wmi.Ptr(v2.MsvmComputerSystemRequestStateChangeRequestedStateEnabled), nil)
if err != nil { /* transport error */ }
if err := res.Wait(ctx, svc); err != nil { /* rejected, or the job failed */ }
```

Under the hood that is `svc.WaitJob(ctx, what, returnValue, jobPath)` —
usable directly with the raw `ExecMethod` API, with `WaitJobEvery` for a
custom poll interval. Cancelling `ctx` abandons the wait, not the job.
Methods with a plain `uint32 ReturnValue` and no job get `Err()` instead:
`nil` on 0, a `*wmi.JobError` otherwise (typed `ReturnValue`s print their
schema display name).

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

The zero `ConnectOptions` is exactly `Connect`. When credentials are given,
they are applied both to `ConnectServer` and to the DCOM proxy blanket (via
a `COAUTHIDENTITY`) — without the latter, remote calls run as the caller's
token and fail access-denied. `User` may be `DOMAIN\user`; the domain is
split out for the blanket. WMI rejects explicit credentials on local
connections. All Service operations remain bound to the connecting
goroutine (see the thread-affinity note on `Connect`).
