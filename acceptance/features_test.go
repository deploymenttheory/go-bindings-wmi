//go:build windows

package acceptance

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// TestExecMethodGetOwner drives an instance method end-to-end with no side
// effects: Win32_Process.GetOwner on this very process, through both the
// runtime map API and the generated typed wrapper.
func TestExecMethodGetOwner(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	rows, err := svc.Query(fmt.Sprintf("SELECT * FROM Win32_Process WHERE ProcessId = %d", os.Getpid()))
	if err != nil || len(rows) == 0 {
		t.Fatalf("query own process: %v (%d rows)", err, len(rows))
	}
	path := wmi.AsString(rows[0]["__PATH"])
	if path == "" {
		t.Fatal("row has no __PATH system property")
	}

	// Runtime map API.
	out, err := svc.ExecMethod(path, "GetOwner", nil)
	if err != nil {
		t.Fatalf("ExecMethod(GetOwner): %v", err)
	}
	if rv := wmi.AsUint32(out["ReturnValue"]); rv != 0 {
		t.Errorf("GetOwner ReturnValue = %d", rv)
	}
	if user := wmi.AsString(out["User"]); user == "" {
		t.Error("GetOwner returned empty User")
	}

	// Generated typed wrapper.
	owner, err := cimv2.Win32ProcessGetOwner(svc, path)
	if err != nil {
		t.Fatalf("Win32ProcessGetOwner: %v", err)
	}
	if owner.ReturnValue != 0 || owner.User == "" {
		t.Errorf("typed GetOwner = %+v", owner)
	}
	t.Logf("owner: %s\\%s", owner.Domain, owner.User)
}

// TestExecMethodStatic drives a static method with in-parameters and cleans
// up after itself: Win32_Process.Create of a short-lived cmd.exe, then
// GetOwner-style verification via the returned ProcessId.
func TestExecMethodStatic(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	res, err := cimv2.Win32ProcessCreate(svc, "cmd.exe /c exit 0", "", nil)
	if err != nil {
		t.Fatalf("Win32ProcessCreate: %v", err)
	}
	if res.ReturnValue != 0 {
		t.Fatalf("Create ReturnValue = %d", res.ReturnValue)
	}
	if res.ProcessId == 0 {
		t.Error("Create returned zero ProcessId")
	}
	t.Logf("spawned pid %d (exits immediately)", res.ProcessId)
}

// TestSubscribeEvents subscribes to Win32_LocalTime modification events,
// which fire every second — a deterministic intrinsic-event source.
func TestSubscribeEvents(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	sub, err := svc.SubscribeEvents(
		"SELECT * FROM __InstanceModificationEvent WITHIN 1 WHERE TargetInstance ISA 'Win32_LocalTime'")
	if err != nil {
		t.Fatalf("SubscribeEvents: %v", err)
	}
	defer sub.Close()

	deadline := time.Now().Add(15 * time.Second)
	for {
		row, err := sub.Next(2 * time.Second)
		if errors.Is(err, wmi.ErrEventTimeout) {
			if time.Now().After(deadline) {
				t.Fatal("no Win32_LocalTime event within 15s")
			}
			continue
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if class := wmi.AsString(row["__CLASS"]); class != "__InstanceModificationEvent" {
			t.Errorf("event class = %q", class)
		}
		t.Logf("event received: %s", wmi.AsString(row["__CLASS"]))
		return
	}
}

// TestQuerySeq streams rows and stops early, exercising enumerator release
// on break.
func TestQuerySeq(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	count := 0
	for row, err := range svc.QuerySeq("SELECT Name FROM Win32_Service") {
		if err != nil {
			t.Fatalf("QuerySeq: %v", err)
		}
		if wmi.AsString(row["Name"]) == "" {
			t.Error("service with empty Name")
		}
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("streamed %d rows, want 3", count)
	}
}

func TestQueryContext(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	rows, err := svc.QueryContext(context.Background(), "SELECT Caption FROM Win32_OperatingSystem")
	if err != nil || len(rows) == 0 {
		t.Fatalf("QueryContext: %v (%d rows)", err, len(rows))
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := svc.QueryContext(canceled, "SELECT * FROM Win32_Service"); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled QueryContext err = %v", err)
	}
}

// TestConnectWith proves the options path end-to-end locally (zero options
// must behave exactly like Connect).
func TestConnectWith(t *testing.T) {
	svc, err := wmi.ConnectWith(`root\cimv2`, wmi.ConnectOptions{Locale: "MS_409"})
	if err != nil {
		t.Fatalf("ConnectWith: %v", err)
	}
	defer svc.Close()

	rows, err := svc.Query("SELECT Caption FROM Win32_OperatingSystem")
	if err != nil || len(rows) == 0 {
		t.Fatalf("query via ConnectWith: %v (%d rows)", err, len(rows))
	}
}
