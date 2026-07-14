//go:build windows

// Package acceptance drives the generated CIM bindings against the live WMI
// repository, proving the capture → snapshot → generate → query loop.
package acceptance

import (
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func TestQueryOperatingSystem(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	os, err := cimv2.QueryWin32OperatingSystem(svc, "")
	if err != nil {
		t.Fatalf("QueryWin32OperatingSystem: %v", err)
	}
	if len(os) == 0 {
		t.Fatal("no Win32_OperatingSystem instances")
	}
	if os[0].Caption == "" {
		t.Errorf("empty Caption: %+v", os[0])
	}
	t.Logf("OS: %s (build %s, %s)", os[0].Caption, os[0].BuildNumber, os[0].OSArchitecture)
}

func TestQueryLogicalDisks(t *testing.T) {
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
	for _, d := range disks {
		t.Logf("%s size=%d free=%d", d.DeviceID, d.Size, d.FreeSpace)
		if d.DeviceID == "" {
			t.Error("logical disk with empty DeviceID")
		}
		// Size is a CIM uint64 that WMI returns as a BSTR string; a zero here
		// would mean the coercion regressed to a failed type assertion.
		if d.Size == 0 {
			t.Errorf("disk %s has zero Size (uint64 coercion regressed?)", d.DeviceID)
		}
	}
}

func TestNumericFieldsCoerced(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	cpus, err := cimv2.QueryWin32Processor(svc, "")
	if err != nil {
		t.Fatalf("QueryWin32Processor: %v", err)
	}
	if len(cpus) == 0 {
		t.Fatal("no processors")
	}
	// NumberOfCores is a CIM uint32 returned by WMI as VT_I4; a zero would mean
	// the width coercion regressed.
	if cpus[0].NumberOfCores == 0 {
		t.Errorf("processor NumberOfCores is 0 (uint32 coercion regressed?)")
	}
	t.Logf("%s: %d cores, %d logical", cpus[0].Name, cpus[0].NumberOfCores, cpus[0].NumberOfLogicalProcessors)
}
