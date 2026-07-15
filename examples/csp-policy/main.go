//go:build windows

// Command csp-policy drives a CSP policy through the MDM WMI bridge:
// read its value, and (with -write) set it and restore the original — the
// R/U/D of policy values on the local device.
//
// The MDM bridge requires the SYSTEM account (elevation is not enough), so
// run this as SYSTEM — e.g. via scripts/Capture-MdmBridge.ps1's technique,
// or PsExec -s. Without -write it only reads; with -write it changes real
// device configuration and then restores it.
//
//	go run ./examples/csp-policy            # read only
//	go run ./examples/csp-policy -write     # read → set → restore (as SYSTEM)
package main

import (
	"errors"
	"flag"
	"fmt"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybrowser"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/csp"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	write := flag.Bool("write", false, "set the policy and restore it (mutates device config; needs SYSTEM)")
	flag.Parse()

	// The policy to drive. Any executable policy works — this one is a
	// benign, reversible Edge setting.
	policy := policybrowser.AllowCookies
	if !policy.Executable() {
		fmt.Printf("%s is not executable via the bridge on this build\n", policy.Name)
		return
	}

	svc, err := csp.Connect()
	if err != nil {
		fmt.Printf("connect to the MDM bridge failed: %v\n", err)
		fmt.Println("the bridge requires the SYSTEM account — run this as SYSTEM.")
		return
	}
	defer svc.Close()

	original, err := readValue(svc, policy)
	fmt.Printf("current %s: %s\n", policy.Name, describe(original, err))

	if !*write {
		fmt.Println("(read-only; pass -write to set and restore)")
		return
	}

	// U: set a new value (Allow all cookies), then read it back.
	target := int64(policybrowser.AllowCookiesAllowAllCookiesFromAllSites)
	fmt.Printf("setting %s = %d …\n", policy.Name, target)
	if err := csp.Set(svc, policy, target); err != nil {
		fmt.Printf("set failed: %v\n", err)
		return
	}
	now, err := readValue(svc, policy)
	fmt.Printf("after set: %s\n", describe(now, err))

	// Restore: put the original back, or delete if it was unset.
	fmt.Println("restoring original …")
	if errors.Is(readErr(original, err), csp.ErrNotConfigured) {
		if err := csp.Delete(svc, policy); err != nil && !errors.Is(err, csp.ErrNotConfigured) {
			fmt.Printf("restore (delete) failed: %v\n", err)
			return
		}
	} else if err := csp.Set(svc, policy, original); err != nil {
		fmt.Printf("restore (set) failed: %v\n", err)
		return
	}
	restored, err := readValue(svc, policy)
	fmt.Printf("after restore: %s\n", describe(restored, err))
}

// readValue reads the effective value, normalizing "not configured".
func readValue(svc *wmi.Service, p csp.Policy) (any, error) {
	v, err := csp.Read(svc, p)
	return v, err
}

func describe(v any, err error) string {
	if errors.Is(err, csp.ErrNotConfigured) {
		return "(not configured)"
	}
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("%v", v)
}

// readErr threads the original read error for the restore decision.
func readErr(_ any, err error) error { return err }
