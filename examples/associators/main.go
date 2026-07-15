//go:build windows

// Command associators walks WMI association chains — the ASSOCIATORS OF /
// REFERENCES OF surface: each fixed logical disk → its partition → the
// physical drive it lives on.
//
//	go run ./examples/associators
package main

import (
	"fmt"

	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		fmt.Println("connect:", err)
		return
	}
	defer svc.Close()

	disks, err := svc.Query(`SELECT __PATH, DeviceID FROM Win32_LogicalDisk WHERE DriveType = 3`)
	if err != nil {
		fmt.Println("query:", err)
		return
	}

	for _, disk := range disks {
		fmt.Printf("%s\n", wmi.AsString(disk["DeviceID"]))

		partitions, err := svc.Associators(wmi.AsString(disk["__PATH"]),
			wmi.AssociatorFilter{ResultClass: "Win32_DiskPartition"})
		if err != nil {
			fmt.Println("  associators:", err)
			continue
		}
		for _, partition := range partitions {
			fmt.Printf("  └─ %s\n", wmi.AsString(partition["DeviceID"]))

			drives, err := svc.Associators(wmi.AsString(partition["__PATH"]),
				wmi.AssociatorFilter{ResultClass: "Win32_DiskDrive"})
			if err != nil {
				continue
			}
			for _, drive := range drives {
				fmt.Printf("      └─ %s (%d GB)\n",
					wmi.AsString(drive["Model"]), wmi.AsUint64(drive["Size"])/(1<<30))
			}
		}

		// REFERENCES OF returns the association instances themselves.
		refs, _ := svc.References(wmi.AsString(disk["__PATH"]), "Win32_LogicalDiskToPartition")
		fmt.Printf("  (%d Win32_LogicalDiskToPartition references)\n", len(refs))
	}
}
