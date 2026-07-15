// Command csp-catalog explores the DDF-sourced policy catalog: list the
// policies in an area, read their schema (type, applicability, allowed
// values), and see which are executable through the MDM bridge. Pure
// metadata — no device, no elevation — so it runs on any OS.
//
//	go run ./examples/csp-catalog
package main

import (
	"fmt"

	"github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybitlocker"
	"github.com/deploymenttheory/go-bindings-wmi/bindings/csp/policybrowser"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/csp"
)

func main() {
	// List: enumerate a policy area and summarize it.
	area := policybrowser.All
	executable := 0
	for _, p := range area {
		if p.Executable() {
			executable++
		}
	}
	fmt.Printf("Browser: %d policies, %d executable via the MDM bridge\n\n", len(area), executable)

	// Read (schema): inspect one policy's definition.
	p := policybrowser.AllowCookies
	fmt.Printf("%s\n", p.Name)
	fmt.Printf("  URI:        %s\n", p.URI)
	fmt.Printf("  Format:     %s\n", p.Format)
	fmt.Printf("  Since build: %s (CSP %s)\n", p.MinOSBuild, p.CSPVersion)
	fmt.Printf("  Default:    %s\n", p.Default)
	if p.Allowed != nil {
		fmt.Printf("  Allowed (%s):\n", p.Allowed.Type)
		for _, e := range p.Allowed.Enum {
			fmt.Printf("    %s = %s\n", e.Value, e.Description)
		}
	}
	if p.Executable() {
		fmt.Printf("  Bridge:     %s.%s\n", p.Bridge.ConfigClass, p.Bridge.Property)
	}

	// Typed enum constants are generated alongside the descriptors.
	fmt.Printf("\nTyped constant: policybrowser.AllowCookiesBlockAllCookiesFromAllSites = %d\n",
		policybrowser.AllowCookiesBlockAllCookiesFromAllSites)

	// Another area, with each policy's applicability at a glance.
	fmt.Printf("\nBitLocker area (%d policies):\n", len(policybitlocker.All))
	for _, p := range policybitlocker.All {
		fmt.Printf("  %-32s %s\n", p.Name, applicability(p))
	}
}

func applicability(p csp.Policy) string {
	s := "build " + p.MinOSBuild
	if p.RequiresAAD {
		s += ", Entra-joined"
	}
	if p.Deprecated() {
		s += ", DEPRECATED " + p.DeprecatedOSBuild
	}
	return s
}
