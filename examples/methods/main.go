//go:build windows

// Command methods demonstrates typed CIM method invocation: an instance
// method on this very process (GetOwner via its __PATH), and a static
// method with an embedded-object parameter (Create with a hidden-window
// Win32_ProcessStartup).
//
//	go run ./examples/methods
package main

import (
	"fmt"
	"os"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/cim/cimv2"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		fmt.Println("connect:", err)
		return
	}
	defer svc.Close()

	// Instance method: fetch our own process (typed, WMIPath included), then
	// call GetOwner on its path.
	proc, err := cimv2.QueryOneWin32Process(svc, wmi.Where("ProcessId = ?", os.Getpid()))
	if err != nil {
		fmt.Println("query self:", err)
		return
	}
	owner, err := cimv2.Win32ProcessGetOwner(svc, proc.WMIPath)
	if err != nil {
		fmt.Println("GetOwner:", err)
		return
	}
	fmt.Printf("this process runs as %s\\%s\n", owner.Domain, owner.User)

	// Static method with an embedded-object in-parameter: spawn a hidden,
	// immediately-exiting cmd.exe. Nil in-parameters are omitted; wmi.Ptr
	// builds the present ones inline.
	startup := wmi.Instance("Win32_ProcessStartup", map[string]any{
		"ShowWindow": uint16(0), // SW_HIDE
	})
	res, err := cimv2.Win32ProcessCreate(svc, wmi.Ptr("cmd.exe /c exit 0"), nil, startup)
	if err != nil {
		fmt.Println("Create:", err)
		return
	}
	if err := res.Err(); err != nil {
		fmt.Println("Create:", err)
		return
	}
	fmt.Printf("spawned hidden pid %d\n", res.ProcessId)
}
