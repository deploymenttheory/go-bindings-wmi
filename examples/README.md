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

The WMI examples above require Windows. The CSP examples below use the
DDF-sourced policy catalog:

- **[`csp-catalog`](csp-catalog)** — explore the policy catalog: list an
  area's policies, read a policy's schema (type, applicability, allowed
  values), typed enum constants, and which policies are executable via the
  bridge. **Pure metadata — runs on any OS.**
- **[`csp-policy`](csp-policy)** — drive a policy through the MDM bridge:
  read its value, and (with `-write`) set and restore it — the R/U/D of
  policy values. Requires the **SYSTEM** account; read-only unless `-write`
  is passed, and self-restoring.
- **[`csp-lcrud`](csp-lcrud)** — one subcommand per LCRUD operation
  (`list`/`read`/`set`/`delete`/`cycle`) on `Browser/AllowCookies`. Because
  the bridge needs **SYSTEM** (an elevated prompt is not SYSTEM), run the
  bridge verbs via the helper, which elevates to SYSTEM for you:

  ```powershell
  # from an ELEVATED PowerShell or cmd prompt, at the repo root:
  powershell -ExecutionPolicy Bypass -File scripts\Invoke-CspLcrud.ps1 read
  powershell -ExecutionPolicy Bypass -File scripts\Invoke-CspLcrud.ps1 cycle   # full LCRUD, self-restoring
  powershell -ExecutionPolicy Bypass -File scripts\Invoke-CspLcrud.ps1 set 2
  ```

  `list` needs no device access: `go run ./examples/csp-lcrud list`.
