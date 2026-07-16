//go:build windows

// Command csp-lcrud demonstrates the generated CSP registry: one connected
// client (mdm.Open) exposing every executable area as a typed service, with
// Get/Set/Delete bound per policy — no descriptor plumbing at the call site.
// Plus Custom, the escape hatch for bridge targets outside the DDF export.
//
// The bridge requires the SYSTEM account, so run this via
// scripts/Invoke-CspLcrud.ps1. `list` needs no device access.
//
//	go run ./examples/csp-lcrud list                # catalog (any OS)
//	go run ./examples/csp-lcrud read                # client.PolicyBrowser.GetAllowCookies()
//	go run ./examples/csp-lcrud set 1               # ...SetAllowCookies(1)
//	go run ./examples/csp-lcrud delete              # ...DeleteAllowCookies()
//	go run ./examples/csp-lcrud cycle               # full LCRUD, self-restoring
//	go run ./examples/csp-lcrud nonpolicy           # non-Policy area services
//	go run ./examples/csp-lcrud custom              # drive a raw bridge target
//	go run ./examples/csp-lcrud inspect <class>     # dump a bridge class
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/csp/mdm"
	"github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybrowser"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/csp"
)

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		usage()
		return
	}
	// list reads the generated catalog; no device access.
	if args[0] == "list" {
		list()
		return
	}

	// Everything else drives the bridge through the registry — one call to
	// connect (SYSTEM), then typed services.
	client, err := mdm.Open()
	if err != nil {
		fmt.Printf("connect to the MDM bridge failed: %v\n", err)
		fmt.Println("the bridge needs the SYSTEM account - run via scripts/Invoke-CspLcrud.ps1.")
		os.Exit(1)
	}
	defer client.Close()

	switch args[0] {
	case "read":
		v, err := client.PolicyBrowser.GetAllowCookies()
		fmt.Printf("AllowCookies = %s\n", describe(v, err))
	case "set":
		if len(args) < 2 {
			fmt.Println("usage: csp-lcrud set <value>")
			os.Exit(2)
		}
		set(client, args[1])
	case "delete":
		del(client)
	case "cycle":
		cycle(client)
	case "nonpolicy":
		nonpolicy(client)
	case "custom":
		custom(client)
	case "inspect":
		if len(args) < 2 {
			fmt.Println("usage: csp-lcrud inspect <BridgeClassName>")
			os.Exit(2)
		}
		inspect(client, args[1])
	default:
		usage()
	}
}

// set — C/U via the typed service method.
func set(c *mdm.Client, raw string) {
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fmt.Printf("value must be an integer: %v\n", err)
		os.Exit(2)
	}
	if err := c.PolicyBrowser.SetAllowCookies(value); err != nil {
		fmt.Printf("set failed: %v\n", err)
		os.Exit(1)
	}
	after, err := c.PolicyBrowser.GetAllowCookies()
	fmt.Printf("set AllowCookies = %d; now reads %s\n", value, describe(after, err))
}

// del — D via the typed service method (unmanages the Browser area).
func del(c *mdm.Client) {
	err := c.PolicyBrowser.DeleteAllowCookies()
	if errors.Is(err, csp.ErrNotConfigured) {
		fmt.Println("AllowCookies was not configured")
		return
	}
	if err != nil {
		fmt.Printf("delete failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("deleted (unmanaged) the Browser area")
}

// cycle — the full LCRUD through the registry, restoring the original value.
func cycle(c *mdm.Client) {
	fmt.Printf("L: Browser area has %d policies\n", len(policybrowser.All))

	original, origErr := c.PolicyBrowser.GetAllowCookies()
	fmt.Printf("R: AllowCookies = %s\n", describe64(original, origErr))

	next := int64(policybrowser.AllowCookiesAllowAllCookiesFromAllSites)
	if origErr == nil && original == next {
		next = int64(policybrowser.AllowCookiesBlockAllCookiesFromAllSites)
	}
	if err := c.PolicyBrowser.SetAllowCookies(next); err != nil {
		fmt.Printf("U: set failed: %v\n", err)
		os.Exit(1)
	}
	got, err := c.PolicyBrowser.GetAllowCookies()
	fmt.Printf("U: set AllowCookies = %d; now reads %s\n", next, describe64(got, err))

	fmt.Print("D: restoring original ... ")
	if errors.Is(origErr, csp.ErrNotConfigured) {
		err = c.PolicyBrowser.DeleteAllowCookies()
	} else {
		err = c.PolicyBrowser.SetAllowCookies(original)
	}
	if err != nil && !errors.Is(err, csp.ErrNotConfigured) {
		fmt.Printf("restore failed: %v\n", err)
		os.Exit(1)
	}
	final, err := c.PolicyBrowser.GetAllowCookies()
	fmt.Printf("restored; now reads %s\n", describe64(final, err))
}

// nonpolicy — non-Policy area services (read-only device state).
func nonpolicy(c *mdm.Client) {
	edition, err := c.WindowsLicensing.GetEdition()
	fmt.Printf("WindowsLicensing.Edition = %s\n", describe64(edition, err))
	status, err := c.WindowsLicensing.GetStatus()
	fmt.Printf("WindowsLicensing.Status  = %s\n", describe64(status, err))
	keyType, err := c.WindowsLicensing.GetLicenseKeyType()
	fmt.Printf("WindowsLicensing.LicenseKeyType = %s\n", describe(keyType, err))
}

// custom — the escape hatch: drive a bridge target with no generated
// binding by supplying its coordinates. Here it reads AllowCookies the
// "manual" way, to show the shape (use inspect to discover real classes).
func custom(c *mdm.Client) {
	target := csp.Target{
		Class:       "MDM_Policy_Result01_Browser02",
		InstanceID:  "Browser",
		ParentID:    "./Vendor/MSFT/Policy/Result",
		Property:    "AllowCookies",
	}
	v, err := c.Custom.Read(target)
	fmt.Printf("custom read %s.%s = %s\n", target.Class, target.Property, describe(v, err))
}

// inspect enumerates a raw bridge class — read-only discovery.
func inspect(c *mdm.Client, class string) {
	rows, err := c.Custom.Query(class)
	if err != nil {
		fmt.Printf("query %s failed: %v\n", class, err)
		os.Exit(1)
	}
	fmt.Printf("%s: %d instance(s)\n", class, len(rows))
	for i, row := range rows {
		fmt.Printf("--- instance %d: InstanceID=%q ParentID=%q ---\n", i, row["InstanceID"], row["ParentID"])
		for k, v := range row {
			if k != "InstanceID" && k != "ParentID" && (len(k) < 2 || k[:2] != "__") && v != nil {
				fmt.Printf("  %s = %v\n", k, v)
			}
		}
	}
}

// list — L: enumerate the catalog (cross-platform, no device access).
func list() {
	executable := 0
	for _, p := range policybrowser.All {
		if p.Executable() {
			executable++
		}
	}
	fmt.Printf("Browser: %d policies, %d executable via the bridge\n", len(policybrowser.All), executable)
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

func describe64(v int64, err error) string {
	if errors.Is(err, csp.ErrNotConfigured) {
		return "(not configured)"
	}
	if err != nil {
		return "error: " + err.Error()
	}
	return strconv.FormatInt(v, 10)
}

func usage() {
	fmt.Println("usage: csp-lcrud <list|read|set <value>|delete|cycle|nonpolicy|custom|inspect <class>>")
}
