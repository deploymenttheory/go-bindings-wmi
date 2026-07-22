//go:build windows

package acceptance

import (
	"strings"
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// TestObjectText renders a built-up embedded instance as CIM DTD 2.0 XML —
// the string shape providers like Hyper-V's virtualization namespace take
// for their *Settings method parameters (MOF text is rejected there).
func TestObjectText(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	text, err := svc.ObjectText(wmi.Instance("Win32_ProcessStartup", map[string]any{
		"Title":         "objecttext-probe",
		"ShowWindow":    0,
		"PriorityClass": 32,
	}))
	if err != nil {
		t.Fatalf("ObjectText: %v", err)
	}
	for _, want := range []string{
		`<INSTANCE CLASSNAME="Win32_ProcessStartup"`,
		`<PROPERTY NAME="Title"`,
		`<VALUE>objecttext-probe</VALUE>`,
		`<VALUE>32</VALUE>`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("ObjectText missing %q in:\n%s", want, text)
		}
	}

	// No __CLASS → actionable error.
	if _, err := svc.ObjectText(wmi.Row{"Title": "x"}); err == nil {
		t.Error("ObjectText without __CLASS should fail")
	}
}

// TestParseObjectText round-trips an embedded instance through its CIM-XML
// text form: ObjectText → ParseObjectText must yield the same typed values.
// This is the decode path for providers that return embedded instances as
// serialized strings (Hyper-V KVP items).
func TestParseObjectText(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	text, err := svc.ObjectText(wmi.Instance("Win32_ProcessStartup", map[string]any{
		"Title":         "parse-probe",
		"PriorityClass": 32,
	}))
	if err != nil {
		t.Fatalf("ObjectText: %v", err)
	}
	row, err := wmi.ParseObjectText(text)
	if err != nil {
		t.Fatalf("ParseObjectText: %v", err)
	}
	if class := wmi.AsString(row["__CLASS"]); class != "Win32_ProcessStartup" {
		t.Errorf("__CLASS = %q", class)
	}
	if title := wmi.AsString(row["Title"]); title != "parse-probe" {
		t.Errorf("Title = %q", title)
	}
	if priority := wmi.AsInt64(row["PriorityClass"]); priority != 32 {
		t.Errorf("PriorityClass = %d, want 32 (typed, not a string)", priority)
	}

	// Garbage in → error out, not a zero Row.
	if _, err := wmi.ParseObjectText("not xml"); err == nil {
		t.Error("ParseObjectText of garbage should fail")
	}
}

// TestObjectTextOfPath round-trips a live instance into CIM-XML text with an
// in-memory override, without persisting anything.
func TestObjectTextOfPath(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	rows, err := svc.Query(`SELECT __PATH, Name FROM Win32_Service WHERE Name = 'Spooler'`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) == 0 {
		t.Skip("no Spooler service on this host")
	}
	path := wmi.AsString(rows[0]["__PATH"])

	text, err := svc.ObjectTextOfPath(path, map[string]any{"Description": "objecttext-override"})
	if err != nil {
		t.Fatalf("ObjectTextOfPath: %v", err)
	}
	if !strings.Contains(text, `<INSTANCE CLASSNAME="Win32_Service"`) {
		t.Errorf("ObjectTextOfPath missing class header in:\n%s", text)
	}
	if !strings.Contains(text, `<VALUE>objecttext-override</VALUE>`) {
		t.Errorf("override not applied in:\n%s", text)
	}

	// The override must not have been written back.
	after, err := svc.GetInstance(path)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if wmi.AsString(after["Description"]) == "objecttext-override" {
		t.Error("ObjectTextOfPath persisted its override — it must be in-memory only")
	}
}
