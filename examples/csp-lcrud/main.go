//go:build windows

// Command csp-lcrud demonstrates each LCRUD operation on a CSP policy
// through the MDM WMI bridge, one per subcommand. The target is a benign,
// reversible Edge setting (Browser/AllowCookies) from the generated catalog.
//
// The bridge requires the SYSTEM account (elevation alone is not enough), so
// run this via scripts/Invoke-CspLcrud.ps1, which executes it as SYSTEM from
// an elevated prompt. `list` needs no device access and runs anywhere.
//
//	go run ./examples/csp-lcrud list           # L: list the area's policies
//	go run ./examples/csp-lcrud read           # R: read the effective value
//	go run ./examples/csp-lcrud set 1          # C/U: set the value (Add/Replace)
//	go run ./examples/csp-lcrud delete         # D: unmanage the area
//	go run ./examples/csp-lcrud cycle          # full LCRUD, self-restoring
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybrowser"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/csp"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// target is the policy every subcommand operates on.
var target = policybrowser.AllowCookies

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		return
	}
	switch args[0] {
	case "list":
		list()
		return
	}

	// Every other verb drives the bridge, which needs SYSTEM.
	svc, err := csp.Connect()
	if err != nil {
		fmt.Printf("connect to the MDM bridge failed: %v\n", err)
		fmt.Println("the bridge needs the SYSTEM account - run via scripts/Invoke-CspLcrud.ps1.")
		os.Exit(1)
	}
	defer svc.Close()

	switch args[0] {
	case "read":
		read(svc)
	case "set":
		if len(args) < 2 {
			fmt.Println("usage: csp-lcrud set <value>")
			os.Exit(2)
		}
		set(svc, args[1])
	case "delete":
		del(svc)
	case "cycle":
		cycle(svc)
	default:
		usage()
	}
}

// list — L: enumerate the area's policies from the generated catalog (no
// device access needed).
func list() {
	fmt.Printf("Browser area - %d policies:\n", len(policybrowser.All))
	for _, p := range policybrowser.All {
		mark := " "
		if p.Executable() {
			mark = "*"
		}
		fmt.Printf("  %s %-40s %s\n", mark, p.Name, p.Format)
	}
	fmt.Printf("\n(* = executable through the bridge; target = %s)\n", target.Name)
}

// read — R: the effective (applied) value.
func read(svc *wmi.Service) {
	v, err := csp.Read(svc, target)
	fmt.Printf("%s = %s\n", target.Name, describe(v, err))
}

// set — C/U: Add/Replace the value.
func set(svc *wmi.Service, raw string) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fmt.Printf("value must be an integer (%s is a %s policy): %v\n", target.Name, target.Format, err)
		os.Exit(2)
	}
	if err := csp.Set(svc, target, value); err != nil {
		fmt.Printf("set failed: %v\n", err)
		os.Exit(1)
	}
	after, err := csp.Read(svc, target)
	fmt.Printf("set %s = %d; now reads %s\n", target.Name, value, describe(after, err))
}

// del — D: unmanage the area (clears its managed settings).
func del(svc *wmi.Service) {
	err := csp.Delete(svc, target)
	if errors.Is(err, csp.ErrNotConfigured) {
		fmt.Printf("%s was not configured\n", target.Name)
		return
	}
	if err != nil {
		fmt.Printf("delete failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("deleted (unmanaged) the %s area\n", target.Bridge.InstanceID)
}

// cycle — the full LCRUD, restoring the original value at the end.
func cycle(svc *wmi.Service) {
	fmt.Printf("L: Browser area has %d policies (%d executable)\n", len(policybrowser.All), countExecutable())

	original, origErr := csp.Read(svc, target)
	fmt.Printf("R: %s = %s\n", target.Name, describe(original, origErr))

	next := int64(policybrowser.AllowCookiesAllowAllCookiesFromAllSites)
	if origErr == nil && fmt.Sprintf("%v", original) == strconv.FormatInt(next, 10) {
		next = int64(policybrowser.AllowCookiesBlockAllCookiesFromAllSites)
	}
	if err := csp.Set(svc, target, next); err != nil {
		fmt.Printf("U: set failed: %v\n", err)
		os.Exit(1)
	}
	got, err := csp.Read(svc, target)
	fmt.Printf("U: set %s = %d; now reads %s\n", target.Name, next, describe(got, err))

	fmt.Print("D: restoring original ... ")
	if errors.Is(origErr, csp.ErrNotConfigured) {
		err = csp.Delete(svc, target)
	} else {
		err = csp.Set(svc, target, original)
	}
	if err != nil && !errors.Is(err, csp.ErrNotConfigured) {
		fmt.Printf("restore failed: %v\n", err)
		os.Exit(1)
	}
	final, err := csp.Read(svc, target)
	fmt.Printf("restored; now reads %s\n", describe(final, err))
}

func countExecutable() int {
	n := 0
	for _, p := range policybrowser.All {
		if p.Executable() {
			n++
		}
	}
	return n
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

func usage() {
	fmt.Println("usage: csp-lcrud <list|read|set <value>|delete|cycle>")
}
