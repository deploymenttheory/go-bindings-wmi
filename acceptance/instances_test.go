//go:build windows

package acceptance

import (
	"errors"
	"fmt"
	"os"
	"testing"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	wmibind "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// TestGetByKey drives the generated key lookups: a hit on a well-known
// service and a clean ErrNotFound miss.
func TestGetByKey(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	spooler, err := cimv2.GetWin32Service(svc, "Spooler")
	if err != nil {
		t.Fatalf("GetWin32Service(Spooler): %v", err)
	}
	if spooler.Name != "Spooler" {
		t.Errorf("Name = %q", spooler.Name)
	}
	t.Logf("Spooler: state=%s start=%s", spooler.State, spooler.StartMode)

	if _, err := cimv2.GetWin32Service(svc, "NoSuchServiceXyz"); !errors.Is(err, wmi.ErrNotFound) {
		t.Errorf("missing service err = %v, want ErrNotFound", err)
	}
}

// TestInstanceCRUD drives the full create → read → update → delete cycle
// against Win32_Environment — a user-scoped, harmless writable class.
func TestInstanceCRUD(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	const name = "GO_BINDINGS_WMI_TEST"
	userName := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	path := fmt.Sprintf(`Win32_Environment.Name=%q,UserName=%q`, name, userName)
	// Self-clean in every outcome, and prove a stale leftover never skews
	// the run.
	cleanup := func() { _ = svc.DeleteInstance(path) }
	cleanup()
	defer cleanup()

	// Create.
	created, err := svc.CreateInstance("Win32_Environment", map[string]any{
		"Name":          name,
		"UserName":      userName,
		"VariableValue": "initial",
	})
	if err != nil {
		if isAccessDenied(err) {
			t.Skipf("environment writes denied in this context: %v", err)
		}
		t.Fatalf("CreateInstance: %v", err)
	}
	t.Logf("created %q", created)

	// Read — generated key lookup (Name + UserName are the keys).
	row, err := cimv2.GetWin32Environment(svc, name, userName)
	if err != nil {
		t.Fatalf("GetWin32Environment after create: %v", err)
	}
	if row.VariableValue != "initial" {
		t.Errorf("VariableValue = %q, want initial", row.VariableValue)
	}

	// Update.
	if err := svc.UpdateInstance(path, map[string]any{"VariableValue": "updated"}); err != nil {
		t.Fatalf("UpdateInstance: %v", err)
	}
	row, err = cimv2.GetWin32Environment(svc, name, userName)
	if err != nil || row.VariableValue != "updated" {
		t.Fatalf("after update: %v, VariableValue=%q", err, row.VariableValue)
	}

	// Runtime read-by-path.
	direct, err := svc.GetInstance(path)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if wmi.AsString(direct["VariableValue"]) != "updated" {
		t.Errorf("GetInstance VariableValue = %v", direct["VariableValue"])
	}

	// Delete, then prove it is gone through both read paths.
	if err := svc.DeleteInstance(path); err != nil {
		t.Fatalf("DeleteInstance: %v", err)
	}
	if _, err := svc.GetInstance(path); !errors.Is(err, wmi.ErrNotFound) {
		t.Errorf("GetInstance after delete err = %v, want ErrNotFound", err)
	}
	if _, err := cimv2.GetWin32Environment(svc, name, userName); !errors.Is(err, wmi.ErrNotFound) {
		t.Errorf("GetWin32Environment after delete err = %v, want ErrNotFound", err)
	}
}

// TestAssociators walks a real association chain: C: logical disk →
// partition → physical drive.
func TestAssociators(t *testing.T) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	disks, err := svc.Query(`SELECT __PATH FROM Win32_LogicalDisk WHERE DriveType = 3`)
	if err != nil || len(disks) == 0 {
		t.Skipf("no fixed disks: %v", err)
	}
	diskPath := wmi.AsString(disks[0]["__PATH"])

	partitions, err := svc.Associators(diskPath, wmi.AssociatorFilter{ResultClass: "Win32_DiskPartition"})
	if err != nil {
		t.Fatalf("Associators(disk → partition): %v", err)
	}
	if len(partitions) == 0 {
		t.Fatal("fixed disk has no associated partitions")
	}
	partitionPath := wmi.AsString(partitions[0]["__PATH"])
	t.Logf("%s → %s", diskPath, wmi.AsString(partitions[0]["DeviceID"]))

	drives, err := svc.Associators(partitionPath, wmi.AssociatorFilter{ResultClass: "Win32_DiskDrive"})
	if err != nil || len(drives) == 0 {
		t.Fatalf("Associators(partition → drive): %v (%d)", err, len(drives))
	}
	t.Logf("→ %s", wmi.AsString(drives[0]["Model"]))

	references, err := svc.References(diskPath, "Win32_LogicalDiskToPartition")
	if err != nil || len(references) == 0 {
		t.Fatalf("References: %v (%d)", err, len(references))
	}
}

// isAccessDenied reports WBEM_E_ACCESS_DENIED, which some hardened hosts
// return for environment writes.
func isAccessDenied(err error) bool {
	var hr win32.HRESULT
	return errors.As(err, &hr) && hr == win32.HRESULT(wmibind.WBEM_E_ACCESS_DENIED)
}
