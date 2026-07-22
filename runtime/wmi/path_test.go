package wmi

import (
	"maps"
	"testing"
)

func TestObjectPath(t *testing.T) {
	cases := []struct {
		class string
		keys  map[string]any
		want  string
	}{
		{"Win32_Service", map[string]any{"Name": "Spooler"}, `Win32_Service.Name="Spooler"`},
		{"Win32_OperatingSystem", nil, `Win32_OperatingSystem=@`},
		{"Demo_Class", map[string]any{"B": true, "A": uint32(7)}, `Demo_Class.A=7,B=TRUE`},
		{"Demo_Class", map[string]any{"Path": `C:\x`, "Q": `say "hi"`},
			`Demo_Class.Path="C:\\x",Q="say \"hi\""`},
		{"Demo_Class", map[string]any{"N": int16(-3)}, `Demo_Class.N=-3`},
	}
	for _, c := range cases {
		if got := ObjectPath(c.class, c.keys); got != c.want {
			t.Errorf("ObjectPath(%s, %v) = %s, want %s", c.class, c.keys, got, c.want)
		}
	}
}

func TestObjectPathNamedType(t *testing.T) {
	type enum uint16
	type name string
	if got := ObjectPath("C", map[string]any{"E": enum(3), "S": name("v")}); got != `C.E=3,S="v"` {
		t.Errorf("named types: got %s", got)
	}
}

func TestParsePath(t *testing.T) {
	cases := []struct {
		in   string
		want PathRef
	}{
		{
			`\\HOST\root\virtualization\v2:Msvm_ComputerSystem.Name="4764334D-E001-4176-82EE-5594EC9B530E"`,
			PathRef{Server: "HOST", Namespace: `root\virtualization\v2`, Class: "Msvm_ComputerSystem",
				Keys: map[string]string{"Name": "4764334D-E001-4176-82EE-5594EC9B530E"}},
		},
		{
			`Win32_Service.Name="Spooler"`,
			PathRef{Class: "Win32_Service", Keys: map[string]string{"Name": "Spooler"}},
		},
		{
			`root\cimv2:Win32_LogicalDisk.DeviceID="C:"`,
			PathRef{Namespace: `root\cimv2`, Class: "Win32_LogicalDisk",
				Keys: map[string]string{"DeviceID": "C:"}},
		},
		{
			`Demo_Class.A=7,B=TRUE,S="a,b"`,
			PathRef{Class: "Demo_Class", Keys: map[string]string{"A": "7", "B": "TRUE", "S": "a,b"}},
		},
		{
			`Demo_Class.Path="C:\\x",Q="say \"hi\""`,
			PathRef{Class: "Demo_Class", Keys: map[string]string{"Path": `C:\x`, "Q": `say "hi"`}},
		},
		{
			`Win32_OperatingSystem=@`,
			PathRef{Class: "Win32_OperatingSystem", Singleton: true},
		},
		{
			`Msvm_VirtualSystemManagementService`,
			PathRef{Class: "Msvm_VirtualSystemManagementService"},
		},
	}
	for _, c := range cases {
		got, err := ParsePath(c.in)
		if err != nil {
			t.Errorf("ParsePath(%q): %v", c.in, err)
			continue
		}
		if got.Server != c.want.Server || got.Namespace != c.want.Namespace ||
			got.Class != c.want.Class || got.Singleton != c.want.Singleton ||
			!maps.Equal(got.Keys, c.want.Keys) {
			t.Errorf("ParsePath(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestParsePathRoundTrip(t *testing.T) {
	path := ObjectPath("Demo_Class", map[string]any{"Name": `q"\'x`, "Index": uint32(3)})
	ref, err := ParsePath(path)
	if err != nil {
		t.Fatalf("ParsePath(%q): %v", path, err)
	}
	if ref.Keys["Name"] != `q"\'x` || ref.Keys["Index"] != "3" {
		t.Errorf("round trip lost keys: %+v", ref)
	}
}

func TestParsePathErrors(t *testing.T) {
	for _, in := range []string{
		"",
		`\\HOST`,
		`\\HOST\root\cimv2`,
		`Class.`,
		`Class.Name="unterminated`,
		`Class.Name="x"garbage`,
		`Class.Name="x",`,
		`Class.=7`,
		`Class=bad`,
	} {
		if _, err := ParsePath(in); err == nil {
			t.Errorf("ParsePath(%q): expected error", in)
		}
	}
}
