package main

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"
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
