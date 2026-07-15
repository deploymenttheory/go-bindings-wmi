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

	// Instance method: query our own process row, then call GetOwner on its
	// __PATH.
	rows, err := svc.Query(fmt.Sprintf("SELECT __PATH FROM Win32_Process WHERE ProcessId = %d", os.Getpid()))
	if err != nil || len(rows) == 0 {
		fmt.Println("query self:", err)
		return
	}
	owner, err := cimv2.Win32ProcessGetOwner(svc, wmi.AsString(rows[0]["__PATH"]))
	if err != nil {
		fmt.Println("GetOwner:", err)
		return
	}
	fmt.Printf("this process runs as %s\\%s\n", owner.Domain, owner.User)

	// Static method with an embedded-object in-parameter: spawn a hidden,
	// immediately-exiting cmd.exe.
	startup := wmi.Instance("Win32_ProcessStartup", map[string]any{
		"ShowWindow": uint16(0), // SW_HIDE
	})
	res, err := cimv2.Win32ProcessCreate(svc, "cmd.exe /c exit 0", "", startup)
	if err != nil {
		fmt.Println("Create:", err)
		return
	}
	fmt.Printf("spawned hidden pid %d (ReturnValue %d)\n", res.ProcessId, res.ReturnValue)
}
