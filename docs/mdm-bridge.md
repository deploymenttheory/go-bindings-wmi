# The MDM bridge (`root\cimv2\mdm\dmmap`)

The MDM bridge namespace exposes Windows' Configuration Service Providers
(CSPs) as WMI classes — `MDM_Policy_Config01_*`, `MDM_WindowsLicensing`,
`MDM_DevDetail_*`, and hundreds more. It is the WMI face of the same
policy surface that Intune and other MDM authorities drive, which makes it
directly relevant to device-management tooling.

It is captured and generated exactly like any other namespace — one extra
package, `bindings/cim/dmmap` (467 classes, ~400 `MDM_*`) — with one
operational difference: **acquisition and querying both require the SYSTEM
account.** The committed snapshot was captured this way; refreshing it, and
running live queries against the bridge, need the same context.

## Why SYSTEM, not just elevation

The MDM Bridge WMI provider answers only the local SYSTEM account. A normal
or even an elevated-administrator capture fails at `ConnectServer` with
`WBEM_E_ACCESS_DENIED` (0x80041003). The capture tool detects this and prints
a pointer to the helper below rather than a bare HRESULT.

## Capturing it

From an **elevated** PowerShell at the repo root (Go on PATH):

```powershell
.\scripts\Capture-MdmBridge.ps1
go run ./cmd/generate
```

`Capture-MdmBridge.ps1` builds the capture tool, runs it as SYSTEM via a
transient one-shot scheduled task, surfaces its output, and cleans up — the
snapshot (`metadata/cim/root.cimv2.mdm.dmmap.json`) lands like any other.
`go run ./cmd/generate` then produces `bindings/cim/dmmap`.

The script also captures other SYSTEM-gated namespaces:

```powershell
.\scripts\Capture-MdmBridge.ps1 -Namespace root\cimv2\mdm\dmmap
```

## If it still denies as SYSTEM

Some hosts gate the bridge behind actual MDM enrollment — an unenrolled
device may expose an empty or unavailable `dmmap`. That is a host state, not
a tooling gap; capture on an enrolled device (or a managed CI image) to get
the full class set. Because the snapshot is host-specific by design (its
provenance records the build), capturing the bridge is the same deliberate,
reviewed act as any other namespace.

## Using it

Once generated, the bridge classes are ordinary typed bindings — query CSP
state, read `MDM_*` policy instances, and (where the class exposes methods)
invoke them through the generated wrappers, all on the same `wmi.Service`.
Note bridge instances are typically singletons or key-addressed; use the
generated `Get<Class>` lookups where a class has key properties.
