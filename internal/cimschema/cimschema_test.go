package cimschema

import (
	"errors"
	"strings"
	"testing"
)

func validSnapshot() *Snapshot {
	return &Snapshot{
		Namespace:  `root\demo`,
		Provenance: Provenance{OSBuild: "26200", Captured: "2026-07-14"},
		Classes: []Class{
			{Name: "Demo_A", Properties: []Property{
				{Name: "Name", CIMType: CIMString, Key: true},
				{Name: "Size", CIMType: CIMUint64},
			}},
			{Name: "Demo_B"},
		},
	}
}

func TestValidateAccepts(t *testing.T) {
	if err := validSnapshot().Validate(); err != nil {
		t.Errorf("valid snapshot rejected: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(*Snapshot)
	}{
		{"empty namespace", func(s *Snapshot) { s.Namespace = "" }},
		{"missing osBuild", func(s *Snapshot) { s.Provenance.OSBuild = "" }},
		{"missing captured", func(s *Snapshot) { s.Provenance.Captured = "" }},
		{"no classes", func(s *Snapshot) { s.Classes = nil }},
		{"empty class name", func(s *Snapshot) { s.Classes[0].Name = "" }},
		{"unsorted classes", func(s *Snapshot) { s.Classes[0].Name = "Demo_Z" }},
		{"duplicate classes", func(s *Snapshot) { s.Classes[1].Name = "Demo_A" }},
		{"empty property name", func(s *Snapshot) { s.Classes[0].Properties[0].Name = "" }},
		{"unsorted properties", func(s *Snapshot) { s.Classes[0].Properties[0].Name = "Zz" }},
		{"non-positive CIM type", func(s *Snapshot) { s.Classes[0].Properties[0].CIMType = 0 }},
		{"unnormalized array flag", func(s *Snapshot) { s.Classes[0].Properties[0].CIMType = CIMString | CIMFlagArray }},
	}
	for _, c := range cases {
		snapshot := validSnapshot()
		c.corrupt(snapshot)
		if err := snapshot.Validate(); !errors.Is(err, ErrInvalidSnapshot) {
			t.Errorf("%s: err = %v, want ErrInvalidSnapshot", c.name, err)
		}
	}
}

func TestGoType(t *testing.T) {
	cases := []struct {
		p    Property
		want string
	}{
		{Property{CIMType: CIMString}, "string"},
		{Property{CIMType: CIMDatetime}, "string"},
		{Property{CIMType: CIMUint64}, "uint64"},
		{Property{CIMType: CIMBoolean}, "bool"},
		{Property{CIMType: CIMReal32}, "float32"},
		{Property{CIMType: CIMChar16}, "uint16"},
		{Property{CIMType: CIMString, Array: true}, "[]string"},
		{Property{CIMType: CIMUint16 | CIMFlagArray}, "[]uint16"}, // legacy un-normalized shape
		{Property{CIMType: 102}, "any"},                           // unknown degrades, never fails
	}
	for _, c := range cases {
		if got := GoType(c.p); got != c.want {
			t.Errorf("GoType(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}

func TestMethodSignature(t *testing.T) {
	m := Method{
		Name:   "Create",
		Static: true,
		In:     []Param{{Name: "CommandLine", CIMType: CIMString}, {Name: "Flags", CIMType: CIMUint32, Array: true}},
		Out:    []Param{{Name: "ProcessId", CIMType: CIMUint32}, {Name: "ReturnValue", CIMType: CIMUint32}},
	}
	want := "Create(CommandLine string, Flags []uint32) → (ProcessId uint32, ReturnValue uint32) [static]"
	if got := MethodSignature(m); got != want {
		t.Errorf("MethodSignature = %q, want %q", got, want)
	}
}

func TestDiffReportsMethodChanges(t *testing.T) {
	before := validSnapshot()
	after := validSnapshot()
	method := Method{Name: "Reset", Out: []Param{{Name: "ReturnValue", CIMType: CIMUint32}}}
	after.Classes[0].Methods = []Method{method}

	report := Diff(before, after)
	if report.Empty() {
		t.Fatal("method addition not reported")
	}
	if len(report.ChangedClasses) != 1 || len(report.ChangedClasses[0].AddedMethods) != 1 {
		t.Fatalf("diff = %+v", report.ChangedClasses)
	}
	if want := "Reset() → (ReturnValue uint32)"; report.ChangedClasses[0].AddedMethods[0] != want {
		t.Errorf("added method = %q, want %q", report.ChangedClasses[0].AddedMethods[0], want)
	}
}

func TestDiffEmpty(t *testing.T) {
	report := Diff(validSnapshot(), validSnapshot())
	if !report.Empty() {
		t.Errorf("identical snapshots diff non-empty: %+v", report)
	}
	if !strings.Contains(report.Markdown(), "No schema changes.") {
		t.Errorf("empty report markdown missing sentinel:\n%s", report.Markdown())
	}
}

func TestDiffReportsChanges(t *testing.T) {
	before := validSnapshot()
	after := validSnapshot()

	// Add a class, remove a class, and change Demo_A three ways.
	after.Classes = append(after.Classes[:1], Class{Name: "Demo_C", Properties: []Property{{Name: "P", CIMType: CIMSint32}}})
	after.Classes[0].Properties[1].CIMType = CIMString                                     // Size: uint64 → string
	after.Classes[0].Properties = append(after.Classes[0].Properties, Property{Name: "Zz", CIMType: CIMBoolean}) // added

	report := Diff(before, after)
	if report.Empty() {
		t.Fatal("changed snapshots diff empty")
	}
	if len(report.AddedClasses) != 1 || report.AddedClasses[0] != "Demo_C" {
		t.Errorf("AddedClasses = %v", report.AddedClasses)
	}
	if len(report.RemovedClasses) != 1 || report.RemovedClasses[0] != "Demo_B" {
		t.Errorf("RemovedClasses = %v", report.RemovedClasses)
	}
	if len(report.ChangedClasses) != 1 {
		t.Fatalf("ChangedClasses = %+v", report.ChangedClasses)
	}
	change := report.ChangedClasses[0]
	if change.Name != "Demo_A" ||
		len(change.AddedProperties) != 1 || change.AddedProperties[0].Name != "Zz" ||
		len(change.ChangedProperties) != 1 || change.ChangedProperties[0].Name != "Size" {
		t.Errorf("class diff = %+v", change)
	}

	md := report.Markdown()
	for _, want := range []string{"Demo_C", "Demo_B", "`Demo_A`", "added `Zz` (bool)", "changed `Size`: uint64 → string"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
}
