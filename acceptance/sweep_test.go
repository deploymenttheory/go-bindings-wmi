//go:build windows

package acceptance

import (
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/internal/cimschema"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// TestSchemaSweep samples the live namespace and drives the same
// introspection path capture uses — ClassProperties including the qualifier
// reads — across it. This is the WMI analogue of the winmd repos' generated
// ABI test: the snapshot is host-specific by design, so the assertion is
// that the schema data plane handles whatever the host repository holds,
// not that the host matches the committed snapshot.
func TestSchemaSweep(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	names, err := svc.ClassNames()
	if err != nil {
		t.Fatalf("ClassNames: %v", err)
	}
	if len(names) < 100 {
		t.Fatalf("implausibly small namespace: %d classes", len(names))
	}

	const stride = 25 // ~50 classes over a typical root\cimv2
	swept, keyed := 0, 0
	for i := 0; i < len(names); i += stride {
		props, err := svc.ClassProperties(names[i])
		if err != nil {
			t.Errorf("ClassProperties(%s): %v", names[i], err)
			continue
		}
		swept++
		for _, p := range props {
			// Every live CIM type must map (worst case `any`), matching what
			// capture would snapshot for this class.
			goType := cimschema.GoType(cimschema.Property{
				CIMType: p.CIMType &^ cimschema.CIMFlagArray,
				Array:   p.CIMType&cimschema.CIMFlagArray != 0,
			})
			if goType == "" {
				t.Errorf("%s.%s: empty Go type for CIM type %d", names[i], p.Name, p.CIMType)
			}
			if p.Key {
				keyed++
			}
		}
	}
	if keyed == 0 {
		t.Error("no key-qualified properties in the sweep (qualifier reads regressed?)")
	}
	t.Logf("swept %d classes, %d key properties", swept, keyed)
}

// TestInstanceDecodeSweep runs full SELECT * decodes over classes that exist
// on every supported Windows edition and are cheap to enumerate (never
// CIM_DataFile or Win32_Product, whose enumeration walks the filesystem /
// reconfigures MSI packages).
func TestInstanceDecodeSweep(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	classes := []string{
		"Win32_OperatingSystem",
		"Win32_ComputerSystem",
		"Win32_Processor",
		"Win32_BIOS",
		"Win32_LogicalDisk",
		"Win32_Service",
		"Win32_NetworkAdapterConfiguration",
		"Win32_TimeZone",
		"Win32_Environment",
	}
	for _, class := range classes {
		rows, err := svc.Query("SELECT * FROM " + class)
		if err != nil {
			t.Errorf("Query(%s): %v", class, err)
			continue
		}
		nonNil := 0
		for _, row := range rows {
			for _, v := range row {
				if v != nil {
					nonNil++
				}
			}
		}
		if len(rows) > 0 && nonNil == 0 {
			t.Errorf("%s: %d rows decoded but every property is nil", class, len(rows))
		}
		t.Logf("%s: %d rows, %d non-nil properties", class, len(rows), nonNil)
	}
}
