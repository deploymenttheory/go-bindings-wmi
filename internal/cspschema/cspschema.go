// Package cspschema is the committed CSP schema snapshot format for the
// DDF-sourced MDM bindings — the DDF-v2 equivalent of internal/cimschema.
//
// The metadata source is Microsoft's Device Description Framework (DDF) v2
// files, published as a versioned zip (the winmd-NuGet analogue for the MDM
// policy surface). Acquisition is a pipeline stage (cmd/fetchddf: pinned
// download + provenance); codegen is offline and deterministic from the
// committed zip.
package cspschema

import "strings"

// Snapshot is the parsed DDF surface: every CSP area and its node tree.
type Snapshot struct {
	// Provenance pins the DDF release the snapshot was parsed from.
	Provenance Provenance `json:"provenance"`
	// CSPs are the configuration service providers / policy areas, sorted
	// by name.
	CSPs []CSP `json:"csps"`
}

// Provenance records which DDF release a snapshot came from.
type Provenance struct {
	// Release is the human-readable DDF drop, e.g. "February 2026".
	Release string `json:"release"`
	// Source is the download URL of the DDF zip.
	Source string `json:"source"`
	// SHA256 is the hex digest of the downloaded zip.
	SHA256 string `json:"sha256"`
	// Fetched is the acquisition date (YYYY-MM-DD, UTC).
	Fetched string `json:"fetched"`
}

// CSP is one configuration service provider or policy area (one DDF file).
type CSP struct {
	// Name is the CSP/area name (the root node's name), e.g. "AboveLock".
	Name string `json:"name"`
	// Path is the OMA-DM base path, e.g. ./Device/Vendor/MSFT/Policy/Config.
	Path string `json:"path"`
	// Nodes are the CSP's settings, sorted by name.
	Nodes []Node `json:"nodes"`
}

// Node is one CSP node: a leaf setting or a grouping node (Format "node").
type Node struct {
	Name string `json:"name"`
	// Path is the full OMA-DM URI (base path + ancestry + name).
	Path string `json:"path"`
	// Format is the DDF DFFormat (int, bool, chr, xml, node, b64, bin,
	// float, date, time, null).
	Format string `json:"format"`
	// Access are the AccessType verbs (Get, Add, Replace, Delete, Exec),
	// sorted.
	Access []string `json:"access,omitempty"`
	// Description is the node's help text (trimmed).
	Description string `json:"description,omitempty"`
	// Default is the DefaultValue, if declared.
	Default string `json:"default,omitempty"`
	// DeprecatedOSBuild is the OsBuildDeprecated attribute, if the node is
	// marked deprecated.
	DeprecatedOSBuild string `json:"deprecatedOsBuild,omitempty"`
	// Applicability records the build/edition gating.
	Applicability Applicability `json:"applicability"`
	// AllowedValues describes the value constraints, if declared.
	AllowedValues *AllowedValues `json:"allowedValues,omitempty"`
	// Children are nested nodes (present for Format "node").
	Children []Node `json:"children,omitempty"`
}

// Applicability records when a node applies (empty fields mean unconstrained).
type Applicability struct {
	// MinOSBuild is the first build the node shipped in (OsBuildVersion).
	MinOSBuild string `json:"minOsBuild,omitempty"`
	// CSPVersion is the lowest CSP version the node released to.
	CSPVersion string `json:"cspVersion,omitempty"`
	// Editions is the EditionAllowList (raw, as authored).
	Editions string `json:"editions,omitempty"`
	// RequiresAAD reports the RequiresAzureAd tag.
	RequiresAAD bool `json:"requiresAad,omitempty"`
}

// AllowedValues describes a node's value constraints (the MSFT:AllowedValues
// qualifier). Only the fields relevant to Type are populated.
type AllowedValues struct {
	// Type is the ValueType: ENUM, Range, RegEx, ADMX, XSD, JSON, Flag,
	// SDDL, or None.
	Type string `json:"type"`
	// Enum holds the enumeration entries (Type ENUM/Flag).
	Enum []EnumValue `json:"enum,omitempty"`
	// ADMX names the backing ADMX policy (Type ADMX).
	ADMX *ADMXBacking `json:"admx,omitempty"`
}

// EnumValue is one allowed enumeration entry.
type EnumValue struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

// ADMXBacking identifies the ADMX policy behind an ADMX-backed node.
type ADMXBacking struct {
	Area string `json:"area"`
	Name string `json:"name"`
	File string `json:"file"`
}

// GoType maps a DDF node's value format to a Go type. Grouping nodes
// ("node") have no value and map to "" (the caller skips them).
func GoType(n Node) string {
	switch n.Format {
	case "int":
		return "int64"
	case "bool":
		return "bool"
	case "float":
		return "float64"
	case "chr", "xml", "date", "time", "b64":
		return "string"
	case "bin":
		return "[]byte"
	case "node", "null", "":
		return ""
	}
	return "string"
}

// Leaf reports whether a node carries a value (is not a grouping/null node).
func (n Node) Leaf() bool {
	return GoType(n) != ""
}

// PolicyArea reports whether a CSP is a Policy area (its nodes are ADMX/
// policy settings under ./…/Policy/Config).
func (c CSP) PolicyArea() bool {
	return strings.Contains(c.Path, "/Policy/Config")
}
