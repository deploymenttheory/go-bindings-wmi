// Package cimschema is the committed CIM schema snapshot format — the
// winmd-equivalent for the WMI bindings. A snapshot is captured from a live
// CIM repository (cmd/capture) and drives deterministic codegen
// (cmd/generate).
package cimschema

// Snapshot is one WMI namespace's captured class schema.
type Snapshot struct {
	// Namespace is the CIM namespace, e.g. `root\cimv2`.
	Namespace string `json:"namespace"`
	// Provenance records the capture host and time (informational; a capture
	// is a deliberate, reviewed act, like fetch-metadata).
	Provenance Provenance `json:"provenance"`
	// Classes are the captured classes, sorted by name for determinism.
	Classes []Class `json:"classes"`
}

// Provenance records where and when a snapshot was captured.
type Provenance struct {
	OSBuild  string `json:"osBuild"`
	Captured string `json:"captured"`
}

// Class is one CIM class: its name and properties (sorted by name).
type Class struct {
	Name       string     `json:"name"`
	Properties []Property `json:"properties"`
}

// Property is one CIM property with its type and key CIM qualifiers.
type Property struct {
	Name string `json:"name"`
	// CIMType is the CIMTYPE_ENUMERATION value (e.g. 8 = string, 3 = sint32).
	CIMType int32 `json:"cimType"`
	// Array reports whether the property is CIM_FLAG_ARRAY.
	Array bool `json:"array,omitempty"`
	// Key marks a key property (the [key] qualifier).
	Key bool `json:"key,omitempty"`
}

// CIM types (CIMTYPE_ENUMERATION from wbemcli.h).
const (
	CIMSint8    int32 = 16
	CIMUint8    int32 = 17
	CIMSint16   int32 = 2
	CIMUint16   int32 = 18
	CIMSint32   int32 = 3
	CIMUint32   int32 = 19
	CIMSint64   int32 = 20
	CIMUint64   int32 = 21
	CIMReal32   int32 = 4
	CIMReal64   int32 = 5
	CIMBoolean  int32 = 11
	CIMString   int32 = 8
	CIMDatetime int32 = 101
	CIMChar16   int32 = 103
	CIMFlagArray int32 = 0x2000
)

// GoType maps a CIM property to its Go type (arrays become slices).
func GoType(p Property) string {
	base := scalarGoType(p.CIMType &^ CIMFlagArray)
	if p.Array || p.CIMType&CIMFlagArray != 0 {
		return "[]" + base
	}
	return base
}

func scalarGoType(cimType int32) string {
	switch cimType {
	case CIMSint8:
		return "int8"
	case CIMUint8:
		return "uint8"
	case CIMSint16:
		return "int16"
	case CIMUint16, CIMChar16:
		return "uint16"
	case CIMSint32:
		return "int32"
	case CIMUint32:
		return "uint32"
	case CIMSint64:
		return "int64"
	case CIMUint64:
		return "uint64"
	case CIMReal32:
		return "float32"
	case CIMReal64:
		return "float64"
	case CIMBoolean:
		return "bool"
	case CIMString, CIMDatetime:
		return "string"
	}
	return "any"
}
