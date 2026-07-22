# Instances and associations

The full instance lifecycle — the equivalent of PowerShell's
`Get-/New-/Set-/Remove-CimInstance` and WMIC's GET/CREATE/SET/DELETE/ASSOC
verbs.

## Read

Queries are the usual bulk read. For point reads:

```go
// Generated key lookup — the parameters are the class's [key] properties,
// captured into the snapshot. Misses return wmi.ErrNotFound.
spooler, err := cimv2.GetWin32Service(svc, "Spooler")

// Runtime read by object path (a WMIPath from a typed query, or a
// key-qualified relative path — wmi.ObjectPath builds one with correct
// escaping).
row, err := svc.GetInstance(wmi.ObjectPath("Win32_Service", map[string]any{"Name": "Spooler"}))

// A Row from GetInstance/Associators/events decodes into the typed struct:
spooler2 := cimv2.Win32ServiceFromRow(row)
```

Key values are rendered as WQL literals by `wmi.WQLValue`/`wmi.QuoteWQL`
(quotes and backslashes escaped), so paths and names with special
characters are safe; `wmi.ParsePath` goes the other way, splitting a
`__PATH` into server, namespace, class, and unescaped key values.

## Create / Update / Delete

```go
path, err := svc.CreateInstance("Win32_Environment", map[string]any{
    "Name": "MY_VAR", "UserName": `DOMAIN\user`, "VariableValue": "v",
})
err = svc.UpdateInstance(path, map[string]any{"VariableValue": "v2"}) // nil sets NULL
err = svc.DeleteInstance(path)  // missing instance → wmi.ErrNotFound
```

`CreateInstance` returns the provider-assigned object path ("" when the
provider doesn't report one). Values encode exactly like method
in-parameters — including embedded objects via `wmi.Instance` and all
slice widths.

Note most WMI classes are read-only or method-driven: you *call*
`Win32_Process.Create`, not create its rows. Instance CUD applies to the
writable subset — environment variables, `root\subscription` event
registrations, share/registry-style providers, the MDM bridge.

## Associations

CIM links instances through association classes; `Associators` returns the
instances on the other end, `References` the association instances
themselves:

```go
partitions, err := svc.Associators(diskPath, wmi.AssociatorFilter{
    ResultClass: "Win32_DiskPartition",
})
links, err := svc.References(diskPath, "Win32_LogicalDiskToPartition")
```

`AssociatorFilter` maps one-to-one onto the WQL `ASSOCIATORS OF` clauses
(`AssocClass`, `ResultClass`, `ResultRole`, `Role`); the zero filter
returns every associated instance. The returned Rows decode into typed
structs via the generated `<Class>FromRow` helpers. See
`examples/associators` for a disk → partition → drive walk.
