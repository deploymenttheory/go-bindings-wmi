package main

import (
	"bytes"
	"flag"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files")

// generatedFiles runs the generator on the fixture and returns the demo
// package's files keyed by name.
func generatedFiles(t *testing.T) map[string][]byte {
	t.Helper()
	outDir := t.TempDir()
	if err := run(filepath.Join("testdata", "snapshot"), outDir); err != nil {
		t.Fatalf("run: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(outDir, "demo"))
	if err != nil {
		t.Fatalf("read generated package: %v", err)
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		content, err := os.ReadFile(filepath.Join(outDir, "demo", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name()] = content
	}
	return files
}

// TestGenerateGolden runs the generator against a fixture snapshot that
// exercises the long tail — zero-property classes, class- and property-name
// collisions, a property named like its class, arrays, keys, methods, and
// an unknown CIM type — and compares each emitted file (doc.go, _structs,
// _queries, _methods) byte-for-byte against a committed golden. Run with
// -update to regenerate the goldens.
func TestGenerateGolden(t *testing.T) {
	files := generatedFiles(t)
	want := []string{"demo_methods.go", "demo_queries.go", "demo_structs.go", "doc.go"}
	if len(files) != len(want) {
		t.Errorf("generated files = %v, want %v", slices.Sorted(maps.Keys(files)), want)
	}
	for _, name := range want {
		got, ok := files[name]
		if !ok {
			t.Errorf("missing generated file %s", name)
			continue
		}
		golden := filepath.Join("testdata", name+".golden")
		if *update {
			if err := os.WriteFile(golden, got, 0o644); err != nil {
				t.Fatalf("update golden: %v", err)
			}
		}
		wantBytes, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("read golden (run with -update to create): %v", err)
		}
		if !bytes.Equal(got, wantBytes) {
			t.Errorf("%s differs from its golden (rerun with -update after reviewing)\n--- got ---\n%s", name, got)
		}
	}
}

// TestGenerateDeterministic pins the byte-determinism contract CI relies on:
// two runs over the same snapshot produce identical files.
func TestGenerateDeterministic(t *testing.T) {
	first := generatedFiles(t)
	second := generatedFiles(t)
	if len(first) != len(second) {
		t.Fatalf("run produced %d then %d files", len(first), len(second))
	}
	for name, a := range first {
		if !bytes.Equal(a, second[name]) {
			t.Errorf("%s: two generator runs produced different bytes", name)
		}
	}
}

// TestPruneStale verifies self-cleaning: generated files this run did not
// write are removed, hand-written files are preserved.
func TestPruneStale(t *testing.T) {
	outDir := t.TempDir()
	staleDir := filepath.Join(outDir, "gone")
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stale := filepath.Join(staleDir, "gone_classes.go")
	if err := os.WriteFile(stale, []byte(header+"package gone\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	handwritten := filepath.Join(outDir, "keep.go")
	if err := os.WriteFile(handwritten, []byte("package keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := run(filepath.Join("testdata", "snapshot"), outDir); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale generated file survived: %v", err)
	}
	if _, err := os.Stat(handwritten); err != nil {
		t.Errorf("hand-written file was pruned: %v", err)
	}
}

// TestPackageCollision proves two namespaces sharing a leaf fail fast rather
// than silently overwriting each other's generated package.
func TestPackageCollision(t *testing.T) {
	metadataDir := t.TempDir()
	for _, ns := range []string{`root\demo`, `root\other\demo`} {
		snapshot := fmt.Sprintf(`{
  "namespace": %q,
  "provenance": {"osBuild": "1", "captured": "2026-07-14"},
  "classes": [{"name": "A", "properties": [{"name": "P", "cimType": 8}]}]
}`, ns)
		name := strings.ReplaceAll(ns, `\`, ".") + ".json"
		if err := os.WriteFile(filepath.Join(metadataDir, name), []byte(snapshot), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := run(metadataDir, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "package collision") {
		t.Errorf("err = %v, want package collision", err)
	}
}

func TestExportName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Win32_OperatingSystem", "Win32OperatingSystem"},
		{"lowercase", "Lowercase"},
		{"__SystemClass", "SystemClass"},
		{"Already", "Already"},
		{"snake_case_name", "SnakeCaseName"},
		{"", ""},
		{"___", ""},
	}
	for _, c := range cases {
		if got := exportName(c.in); got != c.want {
			t.Errorf("exportName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPackageName(t *testing.T) {
	cases := []struct{ in, want string }{
		{`root\cimv2`, "cimv2"},
		{`root\StandardCimv2`, "standardcimv2"},
		{`root\Microsoft\Windows\Storage`, "storage"},
		{`root`, "root"},
	}
	for _, c := range cases {
		if got := packageName(c.in); got != c.want {
			t.Errorf("packageName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
