//go:build windows

// Command capture introspects the live CIM repository and writes a
// deterministic schema snapshot (metadata/cim/<namespace>.json). The snapshot
// is the committed source of truth for codegen — capturing it is a
// deliberate, reviewed act, like fetch-metadata in the winmd repos.
//
//	go run ./cmd/capture               # capture the curated v0 class list
//	go run ./cmd/capture -osbuild 26200
//
// Some namespaces gate schema reads: the MDM bridge (root\cimv2\mdm\dmmap)
// requires the SYSTEM account, not merely an elevated admin. Capture from a
// SYSTEM context — scripts/Capture-MdmBridge.ps1 automates it.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	wmibind "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"

	"github.com/deploymenttheory/go-bindings-wmi/internal/cimschema"
	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

func main() {
	namespace := flag.String("namespace", `root\cimv2`, "CIM namespace")
	classFilter := flag.String("classes", "", "comma-separated class list; empty = every class in the namespace")
	osBuild := flag.String("osbuild", "", "OS build recorded in provenance (default: read from the live OS)")
	captured := flag.String("captured", "", "capture date (YYYY-MM-DD) recorded in provenance (default: today, UTC)")
	outDir := flag.String("out", filepath.Join("metadata", "cim"), "snapshot output directory")
	flag.Parse()

	if err := run(*namespace, *classFilter, *osBuild, *captured, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "capture:", err)
		if hint := accessHint(err, *namespace); hint != "" {
			fmt.Fprintln(os.Stderr, hint)
		}
		os.Exit(1)
	}
}

// accessHint returns actionable guidance when a capture fails with
// access-denied — most often the MDM bridge, which the WMI provider gates
// behind the SYSTEM account rather than mere elevation.
func accessHint(err error, namespace string) string {
	var hr win32.HRESULT
	if !errors.As(err, &hr) || hr != win32.HRESULT(wmibind.WBEM_E_ACCESS_DENIED) {
		return ""
	}
	return fmt.Sprintf("hint: %s denied access. This namespace likely requires the SYSTEM\n"+
		"      account (elevation alone is not enough). Run the capture as SYSTEM —\n"+
		"      scripts/Capture-MdmBridge.ps1 automates it from an elevated shell.", namespace)
}

func run(namespace, classFilter, osBuild, captured, outDir string) error {
	svc, err := wmi.Connect(namespace)
	if err != nil {
		return err
	}
	defer svc.Close()

	// Provenance defaults come from the capture host itself — a snapshot must
	// never land without a build and date to review against.
	if osBuild == "" {
		if osBuild, err = liveOSBuild(); err != nil {
			return err
		}
	}
	if captured == "" {
		captured = time.Now().UTC().Format("2006-01-02")
	}

	var classes []string
	if classFilter != "" {
		for name := range strings.SplitSeq(classFilter, ",") {
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
				Name:      p.Name,
				CIMType:   p.CIMType &^ cimschema.CIMFlagArray,
				Array:     p.CIMType&cimschema.CIMFlagArray != 0,
				Key:       p.Key,
				Values:    p.Values,
				ValueMap:  p.ValueMap,
				BitValues: p.BitValues,
				BitMap:    p.BitMap,
			})
		}
		methods, err := svc.ClassMethods(className)
		if err != nil {
			return fmt.Errorf("%s: %w", className, err)
		}
		for _, m := range methods {
			class.Methods = append(class.Methods, cimschema.Method{
				Name:   m.Name,
				Static: m.Static,
				In:     captureParams(m.In),
				Out:    captureParams(m.Out),
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

// captureParams converts runtime parameter schemas (declaration-ordered)
// into snapshot params. Key is meaningless on parameters and dropped; the
// enumeration/bitmask qualifiers carry through to type the generated method
// surfaces.
func captureParams(params []wmi.PropertyInfo) []cimschema.Param {
	out := make([]cimschema.Param, 0, len(params))
	for _, p := range params {
		out = append(out, cimschema.Param{
			Name:      p.Name,
			CIMType:   p.CIMType &^ cimschema.CIMFlagArray,
			Array:     p.CIMType&cimschema.CIMFlagArray != 0,
			Values:    p.Values,
			ValueMap:  p.ValueMap,
			BitValues: p.BitValues,
			BitMap:    p.BitMap,
		})
	}
	return out
}

// liveOSBuild reads the capture host's build number. It uses its own cimv2
// connection: the capture target may be a different namespace.
func liveOSBuild() (string, error) {
	svc, err := wmi.Connect(`root\cimv2`)
	if err != nil {
		return "", err
	}
	defer svc.Close()
	rows, err := svc.Query("SELECT BuildNumber FROM Win32_OperatingSystem")
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("capture: no Win32_OperatingSystem instance")
	}
	build := wmi.AsString(rows[0]["BuildNumber"])
	if build == "" {
		return "", fmt.Errorf("capture: empty BuildNumber")
	}
	return build, nil
}
