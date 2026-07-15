# Examples

Runnable programs on the generated CIM bindings. Part of the root module — run
directly on Windows:

```sh
go run ./examples/inventory
```

- **[`inventory`](inventory)** — host inventory via typed queries: OS,
  processors, fixed disks, and a count of auto-start running services. Shows the
  generated `Query<Class>(svc, where)` helpers and WQL `WHERE` clauses.
- **[`instances`](instances)** — the full instance CRUD cycle on
  `Win32_Environment`: `CreateInstance`, the generated `Get<Class>` key
  lookup, `UpdateInstance`, `GetInstance` by path, `DeleteInstance`, and the
  `wmi.ErrNotFound` contract. Self-cleaning.
- **[`associators`](associators)** — association traversal: each fixed
  logical disk → its partitions → the physical drive, via
  `Associators`/`AssociatorFilter`, plus `References` for the association
  instances themselves.
- **[`methods`](methods)** — typed CIM method invocation: an instance method
  through a queried `__PATH` (`Win32ProcessGetOwner`) and a static method
  with an embedded-object parameter (`Win32ProcessCreate` with a
  hidden-window `Win32_ProcessStartup` built by `wmi.Instance`). Spawns one
  immediately-exiting hidden `cmd.exe`.
- **[`events`](events)** — a 30-second live subscription to
  process-creation events (`SubscribeEvents`), decoding each event's
  embedded `TargetInstance`.

WMI is a Windows service, so the examples require Windows to run.
