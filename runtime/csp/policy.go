// Package csp is the runtime surface for the DDF-sourced MDM policy
// bindings. It defines the typed descriptors the generated bindings/csp
// packages reference — the OMA-DM URI, value format, allowed values,
// applicability, and deprecation of every configuration service provider
// node, parsed from Microsoft's versioned DDF v2 files.
//
// These descriptors are pure metadata (no Windows dependency). Executing a
// policy — reading or writing it on a device — is done through the MDM WMI
// bridge (root\cimv2\mdm\dmmap) via runtime/wmi; the descriptor's URI and
// format tell you how.
package csp

// Policy describes one CSP node: a device-management setting addressed by
// its OMA-DM URI.
type Policy struct {
	// Name is the node's leaf name, e.g. "AllowActionCenterNotifications".
	Name string
	// URI is the full OMA-DM path, e.g.
	// ./Device/Vendor/MSFT/Policy/Config/AboveLock/AllowActionCenterNotifications.
	URI string
	// Format is the value format: int, bool, chr, xml, b64, bin, float,
	// date, time (grouping "node" descriptors are not emitted as policies).
	Format string
	// Access are the permitted OMA-DM verbs (Get, Add, Replace, Delete,
	// Exec), sorted.
	Access []string
	// Default is the default value, if the DDF declares one.
	Default string
	// DeprecatedOSBuild is the build a deprecated node stopped being
	// recommended in; empty when the node is current.
	DeprecatedOSBuild string
	// MinOSBuild is the first OS build the node applies to.
	MinOSBuild string
	// CSPVersion is the lowest CSP version the node released to.
	CSPVersion string
	// Editions is the raw EditionAllowList (empty = all editions).
	Editions string
	// RequiresAAD reports whether the node requires an Entra-joined device.
	RequiresAAD bool
	// Allowed describes the value constraints, if the DDF declares any.
	Allowed *Allowed
	// Bridge maps the policy onto the MDM WMI bridge for execution
	// (read/set/delete). Non-nil only for policies the bridge exposes —
	// currently the native Policy areas. A nil Bridge means the schema is
	// known but the policy is not (yet) drivable through this runtime.
	Bridge *Bridge
}

// Deprecated reports whether the node is marked deprecated.
func (p Policy) Deprecated() bool { return p.DeprecatedOSBuild != "" }

// Executable reports whether the policy can be read/set/deleted through the
// MDM WMI bridge (see runtime/csp exec).
func (p Policy) Executable() bool { return p.Bridge != nil }

// Bridge is a policy's projection onto the MDM WMI bridge
// (root\cimv2\mdm\dmmap): the WMI classes and instance keys that read and
// write it. The bridge exposes a Policy area as a singleton instance whose
// properties are the area's individual settings.
type Bridge struct {
	// ConfigClass is the bridge class for the desired value / writes, e.g.
	// MDM_Policy_Config01_Browser02.
	ConfigClass string
	// ResultClass is the bridge class for the effective (applied) value,
	// e.g. MDM_Policy_Result01_Browser02.
	ResultClass string
	// InstanceID keys the area instance (the Policy area name, e.g. Browser).
	InstanceID string
	// ParentID keys the area instance (the OMA-DM parent path, e.g.
	// ./Device/Vendor/MSFT/Policy/Config).
	ParentID string
	// Property is the setting's property name on the area instance (the leaf
	// node name, e.g. AllowCookies).
	Property string
}

// Allowed is a node's value constraint.
type Allowed struct {
	// Type is ENUM, Range, RegEx, ADMX, XSD, JSON, Flag, SDDL, or None.
	Type string
	// Enum holds the enumeration entries (ENUM/Flag).
	Enum []EnumValue
	// ADMX identifies the backing ADMX policy (ADMX type).
	ADMX *ADMXBacking
}

// EnumValue is one allowed enumeration entry.
type EnumValue struct {
	Value       string
	Description string
}

// ADMXBacking identifies the ADMX policy behind an ADMX-backed node.
type ADMXBacking struct {
	Area string
	Name string
	File string
}
