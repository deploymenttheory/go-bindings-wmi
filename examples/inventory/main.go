//go:build windows

// Command inventory prints host inventory through typed WMI/CIM queries.
//
//	go run ./examples/inventory
package main

import (
	"fmt"

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

	if os, err := cimv2.QueryWin32OperatingSystem(svc, ""); err == nil && len(os) > 0 {
		fmt.Printf("OS:    %s (build %s)\n", os[0].Caption, os[0].BuildNumber)
	}

	if cpus, err := cimv2.QueryWin32Processor(svc, ""); err == nil {
		for _, c := range cpus {
			fmt.Printf("CPU:   %s — %d cores, %d logical\n",
				c.Name, c.NumberOfCores, c.NumberOfLogicalProcessors)
		}
	}

	if disks, err := cimv2.QueryWin32LogicalDisk(svc, "DriveType = 3"); err == nil {
		for _, d := range disks {
			fmt.Printf("Disk:  %s — %d GB free of %d GB\n",
				d.DeviceID, d.FreeSpace/(1<<30), d.Size/(1<<30))
		}
	}

	if svcs, err := cimv2.QueryWin32Service(svc, "State = 'Running' AND StartMode = 'Auto'"); err == nil {
		fmt.Printf("Auto services running: %d\n", len(svcs))
	}
}
