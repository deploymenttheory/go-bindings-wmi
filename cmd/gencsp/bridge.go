package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// bridgeFile is the committed reference (base name, no extension) listing the
// native Policy areas the MDM WMI bridge exposes — extracted from a dmmap
// capture. It drives the DDF↔bridge join: only areas listed here get
// executable Bridge mappings.
const bridgeFile = "bridge-policy-classes"

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

// bridgeParentID converts a DDF OMA-DM area path to the bridge's ParentID
// key, which is scope-relative: the bridge (running as SYSTEM) drops the
// ./Device or ./User prefix. "./Device/Vendor/MSFT/Policy/Config" →
// "./Vendor/MSFT/Policy/Config".
func bridgeParentID(cspPath string) string {
	for _, scope := range []string{"./Device/", "./User/"} {
		if strings.HasPrefix(cspPath, scope) {
			return "./" + cspPath[len(scope):]
		}
	}
	return cspPath
}
