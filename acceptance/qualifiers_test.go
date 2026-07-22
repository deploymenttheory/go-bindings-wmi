//go:build windows

package acceptance

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// TestParamEnumQualifiers proves capture's method-parameter qualifier read:
// Win32_Service.ChangeStartMode declares Values/ValueMap on its StartMode
// parameter, and signatureParameters must surface them (they type the
// generated method wrappers).
func TestParamEnumQualifiers(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	methods, err := svc.ClassMethods("Win32_Service")
	if err != nil {
		t.Fatalf("ClassMethods: %v", err)
	}
	for _, m := range methods {
		if m.Name != "ChangeStartMode" {
			continue
		}
		for _, p := range m.In {
			if p.Name != "StartMode" {
				continue
			}
			// The Win32 schema declares StartMode ValueMap-only (the stored
			// strings are their own display names).
			if len(p.Values) == 0 && len(p.ValueMap) == 0 {
				t.Fatalf("StartMode parameter carries no enumeration qualifiers: %+v", p)
			}
			t.Logf("StartMode values: %v %v", p.Values, p.ValueMap)
			return
		}
		t.Fatal("ChangeStartMode has no StartMode in-parameter")
	}
	t.Fatal("Win32_Service has no ChangeStartMode method")
}

// TestInheritedEnumQualifiers proves the __DERIVATION walk: in
// root\virtualization\v2, Msvm_ComputerSystem.EnabledState declares its
// enumeration on CIM_EnabledLogicalElement — the derived class's own
// qualifier set is empty, so only inheritance can surface it.
func TestInheritedEnumQualifiers(t *testing.T) {
	svc := connectOrSkip(t, `root\virtualization\v2`)
	defer svc.Close()

	props, err := svc.ClassProperties("Msvm_ComputerSystem")
	if err != nil {
		t.Fatalf("ClassProperties: %v", err)
	}
	for _, p := range props {
		if p.Name != "EnabledState" {
			continue
		}
		if len(p.Values) == 0 || len(p.ValueMap) == 0 {
			t.Fatalf("EnabledState carries no inherited enumeration: %+v", p)
		}
		t.Logf("EnabledState valueMap: %v", p.ValueMap)
		return
	}
	t.Fatal("Msvm_ComputerSystem has no EnabledState property")
}

// TestWaitJob covers the synchronous WaitJob outcomes against a live
// service: 0 completes, a non-async code is a *JobError, and a started-job
// code with a dangling reference fails on the poll.
func TestWaitJob(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()
	ctx := context.Background()

	if err := svc.WaitJob(ctx, "Demo.Noop", wmi.ReturnCompleted, ""); err != nil {
		t.Errorf("ReturnCompleted: %v", err)
	}

	var jobErr *wmi.JobError
	err = svc.WaitJob(ctx, "Demo.Fail", 32775, "")
	if !errors.As(err, &jobErr) || jobErr.ReturnValue != 32775 {
		t.Errorf("error code: got %v, want *JobError(32775)", err)
	}

	err = svc.WaitJob(ctx, "Demo.Dangling", wmi.ReturnJobStarted, `Msvm_ConcreteJob.InstanceID="missing"`)
	if err == nil || !strings.Contains(err.Error(), "job poll") {
		t.Errorf("dangling job: got %v, want poll failure", err)
	}

	if err := svc.WaitJob(ctx, "Demo.NoRef", wmi.ReturnJobStarted, ""); err == nil {
		t.Error("job started without a reference should fail")
	}
}
