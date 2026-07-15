package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bridgeFile / bridgeCSPFile are the committed references (base names, no
// extension) extracted from a dmmap capture. bridgeFile lists the native
// Policy areas (regular MDM_Policy_Config01_<Area>02 convention); bridgeCSPFile
// lists the irregular non-Policy CSP classes with their properties. Both drive
// the DDF↔bridge join: only classes present here get executable mappings.
const (
	bridgeFile    = "bridge-policy-classes"
	bridgeCSPFile = "bridge-csp-classes"
)

// loadBridgeAreas reads the bridge area list, returning lowercase-name →
// canonical-name (the canonical casing forms the WMI class names). A missing
// file yields an empty map — codegen still runs, emitting a schema-only
// catalog with no Bridge mappings.
func loadBridgeAreas(metadataDir string) (map[string]string, error) {
	path := filepath.Join(metadataDir, bridgeFile+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Areas []string `json:"areas"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := make(map[string]string, len(doc.Areas))
	for _, area := range doc.Areas {
		out[strings.ToLower(area)] = area
	}
	return out, nil
}

// bridgeParentID converts a DDF OMA-DM path to the bridge's ParentID key,
// which is scope-relative: the bridge (running as SYSTEM) drops the ./Device
// or ./User prefix. "./Device/Vendor/MSFT/Policy/Config" →
// "./Vendor/MSFT/Policy/Config". Paths already scope-relative pass through.
func bridgeParentID(cspPath string) string {
	for _, scope := range []string{"./Device/", "./User/"} {
		if strings.HasPrefix(cspPath, scope) {
			return "./" + cspPath[len(scope):]
		}
	}
	return cspPath
}

// cspBridge is a non-Policy bridge class with its value properties, keyed for
// the join by its normalized identity.
type cspBridge struct {
	class string
	props map[string]bool
}

var digitsRE = regexp.MustCompile(`\d+`)

// normalizeBridge reduces a class name or a node-path identity to a
// comparison key: drop digits, underscores, and case. MDM_DevDetail_Ext01 and
// the node path DevDetail/Ext both normalize to "devdetailext".
func normalizeBridge(s string) string {
	s = strings.TrimPrefix(s, "MDM_")
	s = digitsRE.ReplaceAllString(s, "")
	return strings.ToLower(strings.ReplaceAll(s, "_", ""))
}

// loadBridgeCSPClasses reads the non-Policy bridge class reference, returning
// normalized-identity → classes (a slice, so the join can detect ambiguity).
// A missing file yields an empty map.
func loadBridgeCSPClasses(metadataDir string) (map[string][]cspBridge, error) {
	path := filepath.Join(metadataDir, bridgeCSPFile+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return map[string][]cspBridge{}, nil
	}
	if err != nil {
		return nil, err
	}
	var doc struct {
		Classes []struct {
			Name       string   `json:"name"`
			Properties []string `json:"properties"`
		} `json:"classes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	out := map[string][]cspBridge{}
	for _, c := range doc.Classes {
		props := make(map[string]bool, len(c.Properties))
		for _, p := range c.Properties {
			props[p] = true
		}
		key := normalizeBridge(c.Name)
		out[key] = append(out[key], cspBridge{class: c.Name, props: props})
	}
	return out, nil
}
