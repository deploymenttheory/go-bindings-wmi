# Examples

Runnable programs on the generated CIM bindings. Part of the root module — run
directly on Windows:

```sh
go run ./examples/inventory
```

- **[`inventory`](inventory)** — host inventory via typed queries: OS,
  processors, fixed disks, and a count of auto-start running services. Shows the
  generated `Query<Class>(svc, where)` helpers and WQL `WHERE` clauses.

WMI is a Windows service, so the examples require Windows to run.
