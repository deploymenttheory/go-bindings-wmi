//go:build windows

// Command capture introspects the live CIM repository and writes a
// deterministic schema snapshot (metadata/cim/<namespace>.json). The snapshot
// is the committed source of truth for codegen — capturing it is a
// deliberate, reviewed act, like fetch-metadata in the winmd repos.
//
//	go run ./cmd/capture               # capture the curated v0 class list
//	go run ./cmd/capture -osbuild 26200
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/deploymenttheory/go-bindings-wmi/internal/cimschema"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	namespace := flag.String("namespace", `root\cimv2`, "CIM namespace")
	classFilter := flag.String("classes", "", "comma-separated class list; empty = every class in the namespace")
	osBuild := flag.String("osbuild", "", "OS build recorded in provenance (informational)")
	captured := flag.String("captured", "", "capture date (YYYY-MM-DD) recorded in provenance")
	outDir := flag.String("out", filepath.Join("metadata", "cim"), "snapshot output directory")
	flag.Parse()

	if err := run(*namespace, *classFilter, *osBuild, *captured, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		os.Exit(1)
	}
}

func run(namespace, classFilter, osBuild, captured, outDir string) error {
	svc, err := wmi.Connect(namespace)
	if err != nil {
		return err
	}
	defer svc.Close()

	var classes []string
	if classFilter != "" {
		for _, name := range strings.Split(classFilter, ",") {
			if name = strings.TrimSpace(name); name != "" {
				classes = append(classes, name)
			}
		}
	} else {
		// Default: the entire namespace.
		classes, err = svc.ClassNames()
		if err != nil {
			return err
		}
	}
	sort.Strings(classes)

	snapshot := cimschema.Snapshot{
		Namespace:  namespace,
		Provenance: cimschema.Provenance{OSBuild: osBuild, Captured: captured},
	}
	for _, className := range classes {
		props, err := svc.ClassProperties(className)
		if err != nil {
			return fmt.Errorf("%s: %w", className, err)
		}
		class := cimschema.Class{Name: className}
		for _, p := range props {
			if strings.HasPrefix(p.Name, "__") {
				continue // WMI system properties (__CLASS, __PATH, …) are not class schema
			}
			class.Properties = append(class.Properties, cimschema.Property{
				Name:    p.Name,
				CIMType: p.CIMType &^ cimschema.CIMFlagArray,
				Array:   p.CIMType&cimschema.CIMFlagArray != 0,
			})
		}
		snapshot.Classes = append(snapshot.Classes, class)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	// File name: root\cimv2 → root.cimv2.json.
	fileName := strings.ReplaceAll(namespace, `\`, ".") + ".json"
	path := filepath.Join(outDir, fileName)
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("captured %d classes → %s\n", len(snapshot.Classes), path)
	return nil
}
