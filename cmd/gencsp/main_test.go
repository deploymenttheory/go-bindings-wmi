package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/internal/cspschema"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// TestGenerateGolden runs gencsp against a fixture snapshot exercising the
// long tail — a policy area with an int enum (typed constants), a string
// enum, deprecation, applicability, and a grouping node with a nested leaf
// — and compares each emitted file byte-for-byte. Run with -update to
// regenerate.
func TestGenerateGolden(t *testing.T) {
	outDir := t.TempDir()
	if err := run(filepath.Join("testdata", "snapshot"), outDir); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Demo is a policy area → package "policydemo".
	pkgDir := filepath.Join(outDir, "policydemo")
	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		t.Fatalf("read generated package: %v", err)
	}
	got := map[string][]byte{}
	for _, e := range entries {
		b, _ := os.ReadFile(filepath.Join(pkgDir, e.Name()))
		got[e.Name()] = b
	}

	want := []string{"doc.go", "policydemo_constants.go", "policydemo_policies.go"}
	if names := sortedKeys(got); !slices.Equal(names, want) {
		t.Errorf("generated files = %v, want %v", names, want)
	}
	for _, name := range want {
		golden := filepath.Join("testdata", name+".golden")
		if *update {
			if err := os.WriteFile(golden, got[name], 0o644); err != nil {
				t.Fatal(err)
			}
		}
		wantBytes, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden (run -update to create): %v", err)
		}
		if !bytes.Equal(got[name], wantBytes) {
			t.Errorf("%s differs from golden (rerun -update after review)\n--- got ---\n%s", name, got[name])
		}
	}
}

func TestGenerateDeterministic(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	for _, d := range []string{a, b} {
		if err := run(filepath.Join("testdata", "snapshot"), d); err != nil {
			t.Fatalf("run: %v", err)
		}
	}
	fa, _ := os.ReadFile(filepath.Join(a, "policydemo", "policydemo_policies.go"))
	fb, _ := os.ReadFile(filepath.Join(b, "policydemo", "policydemo_policies.go"))
	if !bytes.Equal(fa, fb) {
		t.Error("two gencsp runs produced different bytes")
	}
}

func TestPackageName(t *testing.T) {
	cases := []struct {
		base       string
		policyArea bool
		want       string
	}{
		{"AboveLock_AreaDDF", true, "policyabovelock"},
		{"Bitlocker_AreaDDF", true, "policybitlocker"},
		{"BitLocker", false, "bitlocker"},
		{"ADMX_AppCompat_AreaDDF", true, "policyadmxappcompat"},
		{"ActiveSync", false, "activesync"},
	}
	for _, c := range cases {
		if got := packageName(c.base, c.policyArea); got != c.want {
			t.Errorf("packageName(%q, %v) = %q, want %q", c.base, c.policyArea, got, c.want)
		}
	}
}

func TestJoinExport(t *testing.T) {
	cases := []struct {
		segs []string
		want string
	}{
		{[]string{"AllowThing"}, "AllowThing"},
		{[]string{"Group", "Nested"}, "GroupNested"},
		{[]string{"PFXCertInstall", "{UniqueID}", "KeyLocation"}, "PFXCertInstallUniqueIDKeyLocation"},
		{[]string{"3D"}, "P3D"}, // leading digit made valid
		{[]string{"a-b/c"}, "ABC"},
	}
	for _, c := range cases {
		if got := joinExport(c.segs); got != c.want {
			t.Errorf("joinExport(%v) = %q, want %q", c.segs, got, c.want)
		}
	}
}

func TestResolveBridge(t *testing.T) {
	classes := map[string][]cspBridge{
		"windowslicensing":       {{class: "MDM_WindowsLicensing", props: map[string]bool{"Edition": true}}},
		"devdetailext":           {{class: "MDM_DevDetail_Ext01", props: map[string]bool{"DeviceHardwareData": true}}},
		"ambiguous":              {{class: "MDM_A"}, {class: "MDM_B"}},
	}

	// Policy area, direct setting → regular convention.
	p := policy{segs: []string{"AllowX"}, node: cspschema.Node{Name: "AllowX"}}
	spec := resolveBridge(p, "Browser", "./Device/Vendor/MSFT/Policy/Config", classes)
	if spec == nil || spec.configClass != "MDM_Policy_Config01_Browser02" ||
		spec.instanceID != "Browser" || spec.parentID != "./Vendor/MSFT/Policy/Config" || spec.property != "AllowX" {
		t.Errorf("policy spec = %+v", spec)
	}
	// Policy area, nested → not an area-instance property.
	if resolveBridge(policy{segs: []string{"Grp", "Leaf"}}, "Browser", "./Device/Vendor/MSFT/Policy/Config", classes) != nil {
		t.Error("nested policy should not resolve")
	}

	// Non-Policy match, verified key convention.
	wl := policy{node: cspschema.Node{Name: "Edition", Path: "./Vendor/MSFT/WindowsLicensing/Edition"}}
	spec = resolveBridge(wl, "", "./Vendor/MSFT", classes)
	if spec == nil || spec.configClass != "MDM_WindowsLicensing" || spec.resultClass != "MDM_WindowsLicensing" ||
		spec.instanceID != "WindowsLicensing" || spec.parentID != "./Vendor/MSFT" || spec.property != "Edition" {
		t.Errorf("non-policy spec = %+v", spec)
	}
	// Top-level DM root (DevDetail is not under Vendor/MSFT).
	dd := policy{node: cspschema.Node{Name: "DeviceHardwareData", Path: "./DevDetail/Ext/DeviceHardwareData"}}
	spec = resolveBridge(dd, "", ".", classes)
	if spec == nil || spec.instanceID != "Ext" || spec.parentID != "./DevDetail" {
		t.Errorf("devdetail spec = %+v", spec)
	}

	// Rejections: dynamic instance, property not on class, ambiguous.
	dyn := policy{node: cspschema.Node{Name: "V", Path: "./Vendor/MSFT/Foo/{Id}/V"}}
	if resolveBridge(dyn, "", "./Vendor/MSFT", classes) != nil {
		t.Error("dynamic-instance node should not resolve")
	}
	noprop := policy{node: cspschema.Node{Name: "Missing", Path: "./Vendor/MSFT/WindowsLicensing/Missing"}}
	if resolveBridge(noprop, "", "./Vendor/MSFT", classes) != nil {
		t.Error("property not on class should not resolve")
	}
	amb := policy{node: cspschema.Node{Name: "X", Path: "./Vendor/MSFT/Ambiguous/X"}}
	if resolveBridge(amb, "", "./Vendor/MSFT", classes) != nil {
		t.Error("ambiguous match should not resolve")
	}
}

func TestIntLiteral(t *testing.T) {
	for _, c := range []struct {
		in    string
		want  string
		ok    bool
	}{
		{"0", "0", true},
		{"6", "6", true},
		{"0x10", "16", true},
		{"-1", "-1", true},
		{"auto", "", false},
		{"", "", false},
	} {
		got, ok := intLiteral(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("intLiteral(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
