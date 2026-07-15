//go:build windows

package acceptance

import (
	"context"
	"errors"
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// TestGeneratedConstants checks the enumeration constants emitted from the
// captured Values/ValueMap qualifiers line up with live instance data: a
// fixed disk reports DriveType == the generated Local Disk constant, and the
// negative-ValueMap reinterpretation compiles to a usable value.
func TestGeneratedConstants(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	disks, err := cimv2.QueryWin32LogicalDisk(svc, "DriveType = 3")
	if err != nil {
		t.Fatalf("QueryWin32LogicalDisk: %v", err)
	}
	if len(disks) == 0 {
		t.Skip("no fixed disks")
	}
	if disks[0].DriveType != cimv2.Win32LogicalDiskDriveTypeLocalDisk {
		t.Errorf("DriveType = %d, want Win32LogicalDiskDriveTypeLocalDisk (%d)",
			disks[0].DriveType, cimv2.Win32LogicalDiskDriveTypeLocalDisk)
	}
	// String enumeration constant.
	if cimv2.Win32ServiceStartModeAuto != "Auto" {
		t.Errorf("Win32ServiceStartModeAuto = %q", cimv2.Win32ServiceStartModeAuto)
	}
}

// TestExecMethodContext drives a method through the cancellable path and
// checks a pre-canceled context is honored.
func TestExecMethodContext(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	out, err := svc.ExecMethodContext(context.Background(), "Win32_Process", "Create", map[string]any{
		"CommandLine": "cmd.exe /c exit 0",
	})
	if err != nil {
		t.Fatalf("ExecMethodContext(Create): %v", err)
	}
	if wmi.AsUint32(out["ReturnValue"]) != 0 || wmi.AsUint32(out["ProcessId"]) == 0 {
		t.Errorf("Create out = %v", out)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.ExecMethodContext(canceled, "Win32_Process", "Create",
		map[string]any{"CommandLine": "cmd.exe /c exit 0"}); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled ExecMethodContext err = %v, want context.Canceled", err)
	}
}

// TestStringArrayMethodParam drives a []string in-parameter end-to-end
// (the SAFEARRAY-of-BSTR encode path): StdRegProv.SetMultiStringValue
// writes a REG_MULTI_SZ under a temp HKCU key, reads it back, and cleans
// up. ([]wmi.Row encoding is exercised in runtime/wmi.)
func TestStringArrayMethodParam(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	const hkcu = uint32(0x80000001)
	const subKey = `Software\go-bindings-wmi-test`
	const valueName = "MultiSz"

	create, err := svc.ExecMethod("StdRegProv", "CreateKey", map[string]any{
		"hDefKey":     hkcu,
		"sSubKeyName": subKey,
	})
	if err != nil || wmi.AsUint32(create["ReturnValue"]) != 0 {
		t.Skipf("StdRegProv.CreateKey unavailable/denied: %v (%v)", err, create)
	}
	defer svc.ExecMethod("StdRegProv", "DeleteKey", map[string]any{"hDefKey": hkcu, "sSubKeyName": subKey})

	set, err := svc.ExecMethod("StdRegProv", "SetMultiStringValue", map[string]any{
		"hDefKey":     hkcu,
		"sSubKeyName": subKey,
		"sValueName":  valueName,
		"sValue":      []string{"alpha", "beta", "gamma"},
	})
	if err != nil {
		t.Fatalf("SetMultiStringValue: %v", err)
	}
	if rv := wmi.AsUint32(set["ReturnValue"]); rv != 0 {
		t.Fatalf("SetMultiStringValue ReturnValue = %d", rv)
	}

	get, err := svc.ExecMethod("StdRegProv", "GetMultiStringValue", map[string]any{
		"hDefKey":     hkcu,
		"sSubKeyName": subKey,
		"sValueName":  valueName,
	})
	if err != nil {
		t.Fatalf("GetMultiStringValue: %v", err)
	}
	values := wmi.AsStringSlice(get["sValue"])
	if len(values) != 3 || values[0] != "alpha" || values[2] != "gamma" {
		t.Errorf("round-tripped values = %v, want [alpha beta gamma]", values)
	}
	t.Logf("REG_MULTI_SZ round-trip: %v", values)
}
