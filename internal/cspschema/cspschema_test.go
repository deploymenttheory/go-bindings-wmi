package cspschema

import "testing"

// A representative DDF fragment: a Policy area root, an int leaf with enum +
// applicability + deprecation, an ADMX-backed leaf, and a grouping node with
// a nested leaf.
const fixtureDDF = `<?xml version="1.0" encoding="UTF-8"?>
<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">
  <VerDTD>1.2</VerDTD>
  <Node>
    <NodeName>Demo</NodeName>
    <Path>./Device/Vendor/MSFT/Policy/Config</Path>
    <DFProperties><DFFormat><node /></DFFormat></DFProperties>
    <Node>
      <NodeName>AllowThing</NodeName>
      <DFProperties>
        <AccessType><Get /><Replace /><Add /><Delete /></AccessType>
        <Description>Line one.
Line two.</Description>
        <DFFormat><int /></DFFormat>
        <DefaultValue>1</DefaultValue>
        <MSFT:Applicability>
          <MSFT:OsBuildVersion>10.0.10240</MSFT:OsBuildVersion>
          <MSFT:CspVersion>1.0</MSFT:CspVersion>
          <MSFT:EditionAllowList>0x4;</MSFT:EditionAllowList>
          <MSFT:RequiresAzureAd />
        </MSFT:Applicability>
        <MSFT:AllowedValues ValueType="ENUM">
          <MSFT:Enum><MSFT:Value>0</MSFT:Value><MSFT:ValueDescription>Off.</MSFT:ValueDescription></MSFT:Enum>
          <MSFT:Enum><MSFT:Value>1</MSFT:Value><MSFT:ValueDescription>On.</MSFT:ValueDescription></MSFT:Enum>
        </MSFT:AllowedValues>
        <MSFT:Deprecated OsBuildDeprecated="10.0.22000" />
      </DFProperties>
    </Node>
    <Node>
      <NodeName>AdmxThing</NodeName>
      <DFProperties>
        <AccessType><Get /><Replace /></AccessType>
        <DFFormat><chr /></DFFormat>
        <MSFT:AllowedValues ValueType="ADMX">
          <MSFT:AdmxBacked Area="Foo~Policy~Bar" Name="ThePolicy" File="foo.admx" />
        </MSFT:AllowedValues>
      </DFProperties>
    </Node>
    <Node>
      <NodeName>Group</NodeName>
      <DFProperties><DFFormat><node /></DFFormat></DFProperties>
      <Node>
        <NodeName>Nested</NodeName>
        <DFProperties>
          <AccessType><Get /></AccessType>
          <DFFormat><bool /></DFFormat>
        </DFProperties>
      </Node>
    </Node>
  </Node>
</MgmtTree>`

func TestParseDDF(t *testing.T) {
	csp, err := ParseDDF([]byte(fixtureDDF))
	if err != nil {
		t.Fatalf("ParseDDF: %v", err)
	}
	if csp.Name != "Demo" || csp.Path != "./Device/Vendor/MSFT/Policy/Config" {
		t.Fatalf("CSP = %+v", csp)
	}
	if !csp.PolicyArea() {
		t.Error("Demo should be a policy area")
	}
	// Nodes sorted: AdmxThing, AllowThing, Group.
	if len(csp.Nodes) != 3 || csp.Nodes[0].Name != "AdmxThing" || csp.Nodes[1].Name != "AllowThing" {
		t.Fatalf("nodes = %v", names(csp.Nodes))
	}

	allow := csp.Nodes[1]
	if allow.Path != "./Device/Vendor/MSFT/Policy/Config/Demo/AllowThing" {
		t.Errorf("AllowThing path = %q", allow.Path)
	}
	if allow.Format != "int" || allow.Default != "1" {
		t.Errorf("AllowThing format/default = %q/%q", allow.Format, allow.Default)
	}
	// Access sorted.
	if got := allow.Access; len(got) != 4 || got[0] != "Add" || got[3] != "Replace" {
		t.Errorf("access = %v", got)
	}
	if allow.DeprecatedOSBuild != "10.0.22000" {
		t.Errorf("deprecated = %q", allow.DeprecatedOSBuild)
	}
	if a := allow.Applicability; a.MinOSBuild != "10.0.10240" || a.CSPVersion != "1.0" || a.Editions != "0x4;" || !a.RequiresAAD {
		t.Errorf("applicability = %+v", a)
	}
	if av := allow.AllowedValues; av == nil || av.Type != "ENUM" || len(av.Enum) != 2 ||
		av.Enum[0].Value != "0" || av.Enum[0].Description != "Off." {
		t.Errorf("allowedValues = %+v", allow.AllowedValues)
	}

	admx := csp.Nodes[0]
	if admx.AllowedValues == nil || admx.AllowedValues.Type != "ADMX" || admx.AllowedValues.ADMX == nil ||
		admx.AllowedValues.ADMX.Name != "ThePolicy" || admx.AllowedValues.ADMX.File != "foo.admx" {
		t.Errorf("admx = %+v", admx.AllowedValues)
	}

	group := csp.Nodes[2]
	if group.Format != "node" || group.Leaf() {
		t.Errorf("Group should be a non-leaf node")
	}
	if len(group.Children) != 1 || group.Children[0].Name != "Nested" ||
		group.Children[0].Path != "./Device/Vendor/MSFT/Policy/Config/Demo/Group/Nested" {
		t.Errorf("nested = %+v", group.Children)
	}
}

func TestGoType(t *testing.T) {
	cases := []struct{ format, want string }{
		{"int", "int64"}, {"bool", "bool"}, {"float", "float64"},
		{"chr", "string"}, {"xml", "string"}, {"b64", "string"},
		{"bin", "[]byte"}, {"node", ""}, {"null", ""}, {"", ""},
		{"unknownfuture", "string"},
	}
	for _, c := range cases {
		if got := GoType(Node{Format: c.format}); got != c.want {
			t.Errorf("GoType(%q) = %q, want %q", c.format, got, c.want)
		}
	}
}

func names(nodes []Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}
