//go:build windows

// Command events subscribes to process-creation events for 30 seconds and
// prints each new process as it starts — the ExecNotificationQuery surface.
// Start any program while it runs to see it appear.
//
//	go run ./examples/events
package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		fmt.Println("connect:", err)
		return
	}
	defer svc.Close()

	sub, err := svc.SubscribeEvents(
		"SELECT * FROM __InstanceCreationEvent WITHIN 2 WHERE TargetInstance ISA 'Win32_Process'")
	if err != nil {
		fmt.Println("subscribe:", err)
		return
	}
	defer sub.Close()

	fmt.Println("watching process creation for 30s — start something…")
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		event, err := sub.Next(2 * time.Second)
		if errors.Is(err, wmi.ErrEventTimeout) {
			continue
		}
		if err != nil {
			fmt.Println("next:", err)
			return
		}
		// Intrinsic events embed the new instance as a nested Row.
		process := wmi.AsRow(event["TargetInstance"])
		fmt.Printf("  + pid %v  %s\n",
			process["ProcessId"], wmi.AsString(process["Name"]))
	}
}
