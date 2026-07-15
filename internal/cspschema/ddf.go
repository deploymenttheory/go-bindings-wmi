package cspschema

import (
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// msftNS is the DDF v2 Microsoft extension namespace.
const msftNS = "http://schemas.microsoft.com/MobileDevice/DM"

// ParseDDF parses one DDF v2 file (one CSP / policy area) into a CSP.
func ParseDDF(data []byte) (*CSP, error) {
	var tree ddfMgmtTree
	if err := xml.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("cspschema: parse DDF: %w", err)
	}
	if tree.Node == nil {
		return nil, fmt.Errorf("cspschema: DDF has no root node")
	}
	root := tree.Node
	csp := &CSP{Name: root.NodeName, Path: root.Path}
	base := strings.TrimRight(root.Path, "/") + "/" + root.NodeName
	csp.Nodes = buildNodes(root.Nodes, base)
	return csp, nil
}

// buildNodes converts DDF child nodes into schema nodes, threading the full
// OMA-DM path and sorting by name for determinism.
func buildNodes(nodes []ddfNode, parentPath string) []Node {
	out := make([]Node, 0, len(nodes))
	for i := range nodes {
		out = append(out, buildNode(&nodes[i], parentPath))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func buildNode(n *ddfNode, parentPath string) Node {
	path := strings.TrimRight(parentPath, "/") + "/" + n.NodeName
	node := Node{
		Name:        n.NodeName,
		Path:        path,
		Format:      n.Props.Format.choice(),
		Access:      n.Props.Access.verbs(),
		Description: strings.TrimSpace(n.Props.Description),
		Default:     strings.TrimSpace(n.Props.DefaultValue),
	}
	if n.Props.Applicability != nil {
		a := n.Props.Applicability
		node.Applicability = Applicability{
			MinOSBuild:  strings.TrimSpace(a.OsBuildVersion),
			CSPVersion:  strings.TrimSpace(a.CspVersion),
			Editions:    strings.TrimSpace(a.EditionAllowList),
			RequiresAAD: a.RequiresAzureAd != nil,
		}
	}
	if n.Props.Deprecated != nil {
		node.DeprecatedOSBuild = n.Props.Deprecated.OsBuildDeprecated
		if node.DeprecatedOSBuild == "" {
			node.DeprecatedOSBuild = "deprecated" // marked, build unspecified
		}
	}
	if av := n.Props.AllowedValues; av != nil {
		node.AllowedValues = av.convert()
	}
	if len(n.Nodes) > 0 {
		node.Children = buildNodes(n.Nodes, path)
	}
	return node
}

// --- DDF XML shapes -------------------------------------------------------

type ddfMgmtTree struct {
	XMLName xml.Name `xml:"MgmtTree"`
	Node    *ddfNode `xml:"Node"`
}

type ddfNode struct {
	NodeName string        `xml:"NodeName"`
	Path     string        `xml:"Path"`
	Props    ddfProperties `xml:"DFProperties"`
	Nodes    []ddfNode     `xml:"Node"`
}

type ddfProperties struct {
	Access        ddfChoice         `xml:"AccessType"`
	Description   string            `xml:"Description"`
	Format        ddfChoice         `xml:"DFFormat"`
	DefaultValue  string            `xml:"DefaultValue"`
	Applicability *ddfApplicability `xml:"Applicability"`
	AllowedValues *ddfAllowedValues `xml:"AllowedValues"`
	Deprecated    *ddfDeprecated    `xml:"Deprecated"`
}

// ddfChoice captures a set of empty child elements (AccessType verbs,
// DFFormat's single format element) by their local names.
type ddfChoice struct {
	Elems []ddfElem `xml:",any"`
}

type ddfElem struct {
	XMLName xml.Name
}

// choice returns the single child's local name (DFFormat, Scope, …).
func (c ddfChoice) choice() string {
	if len(c.Elems) == 0 {
		return ""
	}
	return c.Elems[0].XMLName.Local
}

// verbs returns the child local names sorted (AccessType).
func (c ddfChoice) verbs() []string {
	if len(c.Elems) == 0 {
		return nil
	}
	out := make([]string, 0, len(c.Elems))
	for _, e := range c.Elems {
		out = append(out, e.XMLName.Local)
	}
	sort.Strings(out)
	return out
}

type ddfApplicability struct {
	OsBuildVersion   string    `xml:"OsBuildVersion"`
	CspVersion       string    `xml:"CspVersion"`
	EditionAllowList string    `xml:"EditionAllowList"`
	RequiresAzureAd  *struct{} `xml:"RequiresAzureAd"`
}

type ddfDeprecated struct {
	OsBuildDeprecated string `xml:"OsBuildDeprecated,attr"`
}

type ddfAllowedValues struct {
	ValueType string     `xml:"ValueType,attr"`
	Enums     []ddfEnum  `xml:"Enum"`
	Admx      *ddfAdmx   `xml:"AdmxBacked"`
}

type ddfEnum struct {
	Value       string `xml:"Value"`
	Description string `xml:"ValueDescription"`
}

type ddfAdmx struct {
	Area string `xml:"Area,attr"`
	Name string `xml:"Name,attr"`
	File string `xml:"File,attr"`
}

func (a *ddfAllowedValues) convert() *AllowedValues {
	out := &AllowedValues{Type: a.ValueType}
	for _, e := range a.Enums {
		out.Enum = append(out.Enum, EnumValue{
			Value:       strings.TrimSpace(e.Value),
			Description: strings.TrimSpace(e.Description),
		})
	}
	if a.Admx != nil {
		out.ADMX = &ADMXBacking{Area: a.Admx.Area, Name: a.Admx.Name, File: a.Admx.File}
	}
	return out
}
