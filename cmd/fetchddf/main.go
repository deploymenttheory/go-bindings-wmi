// Command fetchddf is the acquisition stage of the CSP/DDF pipeline — the
// analogue of fetch-metadata in the winmd repos. It downloads a pinned,
// versioned DDF v2 zip (Microsoft's canonical, machine-readable CSP schema),
// verifies its SHA-256, parses every CSP/policy area, and writes the
// committed snapshot tree (metadata/csp/<area>.json) plus provenance.
//
// Codegen (cmd/gencsp) is offline and deterministic from the committed
// snapshots; fetching a fresh DDF release is a deliberate, reviewed act.
//
//	go run ./cmd/fetchddf                     # download the pinned release
//	go run ./cmd/fetchddf -zip local.zip      # parse an already-downloaded zip (offline)
//	go run ./cmd/fetchddf -url <u> -release <r> -sha256 <hex>
package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/deploymenttheory/go-bindings-wmi/internal/cspschema"
)

// Pinned DDF v2 release. Bump these (and re-run) to move to a new drop.
const (
	defaultRelease = "February 2026"
	defaultURL     = "https://download.microsoft.com/download/015bd9f5-9cca-4821-8a85-a4c5f9a5d0f2/DDFv2Feb2026.zip"
)

func main() {
	release := flag.String("release", defaultRelease, "DDF release label recorded in provenance")
	url := flag.String("url", defaultURL, "DDF v2 zip URL")
	zipPath := flag.String("zip", "", "parse a local zip instead of downloading (offline)")
	wantSHA := flag.String("sha256", "", "expected zip SHA-256 (hex); verified when set")
	fetched := flag.String("fetched", "", "fetch date YYYY-MM-DD recorded in provenance (default: today UTC)")
	outDir := flag.String("out", filepath.Join("metadata", "csp"), "snapshot output directory")
	flag.Parse()

	if *fetched == "" {
		*fetched = time.Now().UTC().Format("2006-01-02")
	}
	if err := run(*release, *url, *zipPath, *wantSHA, *fetched, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "fetchddf:", err)
		os.Exit(1)
	}
}

func run(release, url, zipPath, wantSHA, fetched, outDir string) error {
	data, source, err := acquire(url, zipPath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	if wantSHA != "" && !strings.EqualFold(wantSHA, digest) {
		return fmt.Errorf("sha256 mismatch: got %s, want %s", digest, wantSHA)
	}

	csps, err := parseZip(data)
	if err != nil {
		return err
	}
	if len(csps) == 0 {
		return fmt.Errorf("no DDF (.xml) entries in the zip")
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	written := map[string]bool{}
	names := make([]string, 0, len(csps))
	for name := range csps {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(outDir, name+".json")
		body, err := json.MarshalIndent(csps[name], "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
			return err
		}
		written[path] = true
	}

	prov := cspschema.Provenance{Release: release, Source: source, SHA256: digest, Fetched: fetched}
	provBody, err := json.MarshalIndent(prov, "", "  ")
	if err != nil {
		return err
	}
	provPath := filepath.Join(outDir, "PROVENANCE.json")
	if err := os.WriteFile(provPath, append(provBody, '\n'), 0o644); err != nil {
		return err
	}
	written[provPath] = true

	if err := pruneStale(outDir, written); err != nil {
		return err
	}
	fmt.Printf("fetched %s (%s): %d CSPs → %s\n", release, digest[:12], len(csps), outDir)
	return nil
}

// acquire returns the zip bytes and a source label, from a local file or by
// downloading.
func acquire(url, zipPath string) (data []byte, source string, err error) {
	if zipPath != "" {
		data, err = os.ReadFile(zipPath)
		return data, url, err
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, url, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, url, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, url, fmt.Errorf("download %s: HTTP %d", url, resp.StatusCode)
	}
	data, err = io.ReadAll(resp.Body)
	return data, url, err
}

// parseZip parses every .xml entry into a CSP, keyed by the entry base name.
func parseZip(data []byte) (map[string]*cspschema.CSP, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	out := map[string]*cspschema.CSP{}
	for _, entry := range zr.File {
		if entry.FileInfo().IsDir() || !strings.EqualFold(filepath.Ext(entry.Name), ".xml") {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, err
		}
		csp, err := cspschema.ParseDDF(body)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name, err)
		}
		name := snapshotName(entry.Name)
		if _, clash := out[name]; clash {
			return nil, fmt.Errorf("snapshot name collision: %q", name)
		}
		out[name] = csp
	}
	return out, nil
}

// snapshotName derives a stable snapshot file base from a DDF entry name,
// dropping the path and .xml extension.
func snapshotName(entry string) string {
	base := filepath.Base(strings.ReplaceAll(entry, `\`, "/"))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// pruneStale removes snapshot files this run did not write (a shrinking DDF
// drop), leaving non-JSON files alone.
func pruneStale(outDir string, written map[string]bool) error {
	entries, err := filepath.Glob(filepath.Join(outDir, "*.json"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !written[entry] {
			if err := os.Remove(entry); err != nil {
				return err
			}
		}
	}
	return nil
}
