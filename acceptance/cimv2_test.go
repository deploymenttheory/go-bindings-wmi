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
	for _, d := range disks {
		t.Logf("%s size=%d free=%d", d.DeviceID, d.Size, d.FreeSpace)
		if d.DeviceID == "" {
			t.Error("logical disk with empty DeviceID")
		}
	}
}
