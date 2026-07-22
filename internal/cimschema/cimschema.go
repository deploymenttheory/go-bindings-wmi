// Package cimschema is the committed CIM schema snapshot format — the
// winmd-equivalent for the WMI bindings. A snapshot is captured from a live
// CIM repository (cmd/capture) and drives deterministic codegen
// (cmd/generate).
package cimschema

import "slices"

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

// Class is one CIM class: its name, properties (sorted by name), and
// methods (sorted by name).
type Class struct {
	Name       string     `json:"name"`
	Properties []Property `json:"properties"`
	Methods    []Method   `json:"methods,omitempty"`
}

// Method is one CIM method. Parameters keep their declaration ([ID]) order —
// it defines the generated Go signature; Out includes ReturnValue last.
type Method struct {
	Name   string  `json:"name"`
	Static bool    `json:"static,omitempty"`
	In     []Param `json:"in,omitempty"`
	Out    []Param `json:"out,omitempty"`
}

// Param is one method parameter.
type Param struct {
	Name    string `json:"name"`
	CIMType int32  `json:"cimType"`
	Array   bool   `json:"array,omitempty"`
	// Values/ValueMap and BitValues/BitMap mirror Property's enumeration and
	// bitmask qualifiers. Older snapshots (captured before parameter
	// qualifiers were recorded) carry none; the generator then emits the
	// parameter untyped.
	Values    []string `json:"values,omitempty"`
	ValueMap  []string `json:"valueMap,omitempty"`
	BitValues []string `json:"bitValues,omitempty"`
	BitMap    []string `json:"bitMap,omitempty"`
}

// Equal reports semantic parameter equality (used by the snapshot diff;
// Param is not comparable once it carries qualifier slices).
func (p Param) Equal(other Param) bool {
	return p.Name == other.Name && p.CIMType == other.CIMType && p.Array == other.Array &&
		slices.Equal(p.Values, other.Values) && slices.Equal(p.ValueMap, other.ValueMap) &&
		slices.Equal(p.BitValues, other.BitValues) && slices.Equal(p.BitMap, other.BitMap)
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
	// Values and ValueMap are the CIM enumeration qualifiers: ValueMap holds
	// the stored values (numeric strings for integer properties), Values the
	// display names. Either may be present alone — without a ValueMap the
	// Values entries map to consecutive indices 0..n-1 (integer properties)
	// or are the stored strings themselves (string properties).
	Values   []string `json:"values,omitempty"`
	ValueMap []string `json:"valueMap,omitempty"`
	// BitValues and BitMap are the CIM bitmask qualifiers: BitMap holds bit
	// positions (decimal strings), BitValues the flag display names. Without
	// a BitMap the BitValues entries map to consecutive positions 0..n-1.
	BitValues []string `json:"bitValues,omitempty"`
	BitMap    []string `json:"bitMap,omitempty"`
}

// Equal reports semantic property equality (used by the snapshot diff;
// Property is not comparable once it carries qualifier slices).
func (p Property) Equal(other Property) bool {
	return p.Name == other.Name && p.CIMType == other.CIMType &&
		p.Array == other.Array && p.Key == other.Key &&
		slices.Equal(p.Values, other.Values) && slices.Equal(p.ValueMap, other.ValueMap) &&
		slices.Equal(p.BitValues, other.BitValues) && slices.Equal(p.BitMap, other.BitMap)
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
	// CIMObject is an embedded CIM object (decoded to a wmi.Row).
	CIMObject int32 = 13
	// CIMReference is a REF — an object path string.
	CIMReference int32 = 102
	CIMFlagArray int32 = 0x2000
)

// GoType maps a CIM property to its Go type (arrays become slices).
func GoType(p Property) string {
	return GoTypeFor(p.CIMType, p.Array)
}

// ParamGoType maps a CIM method parameter to its Go type.
func ParamGoType(p Param) string {
	return GoTypeFor(p.CIMType, p.Array)
}

// GoTypeFor maps a CIM type (with or without the array flag folded in) to a
// Go type.
func GoTypeFor(cimType int32, array bool) string {
	base := scalarGoType(cimType &^ CIMFlagArray)
	if array || cimType&CIMFlagArray != 0 {
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
	case CIMString, CIMDatetime, CIMReference:
		return "string"
	case CIMObject:
		return "wmi.Row"
	}
	return "any"
}
