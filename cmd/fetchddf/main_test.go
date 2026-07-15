package main

import (
	"archive/zip"
	"bytes"
	"testing"
)

const tinyDDF = `<?xml version="1.0" encoding="UTF-8"?>
<MgmtTree xmlns:MSFT="http://schemas.microsoft.com/MobileDevice/DM">
  <VerDTD>1.2</VerDTD>
  <Node>
    <NodeName>Demo</NodeName>
    <Path>./Device/Vendor/MSFT</Path>
    <DFProperties><DFFormat><node /></DFFormat></DFProperties>
    <Node>
      <NodeName>Setting</NodeName>
      <DFProperties>
        <AccessType><Get /></AccessType>
        <DFFormat><int /></DFFormat>
      </DFProperties>
    </Node>
  </Node>
</MgmtTree>`

func TestParseZip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"Demo_AreaDDF.xml", "sub/Other.xml", "readme.txt"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(tinyDDF)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	csps, err := parseZip(buf.Bytes())
	if err != nil {
		t.Fatalf("parseZip: %v", err)
	}
	// Two .xml entries parsed; the .txt ignored.
	if len(csps) != 2 {
		t.Fatalf("parsed %d CSPs, want 2 (%v)", len(csps), keys(csps))
	}
	if _, ok := csps["Demo_AreaDDF"]; !ok {
		t.Errorf("missing Demo_AreaDDF; got %v", keys(csps))
	}
	if _, ok := csps["Other"]; !ok { // base name only, path stripped
		t.Errorf("missing Other; got %v", keys(csps))
	}
}

func TestSnapshotName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"AboveLock_AreaDDF.xml", "AboveLock_AreaDDF"},
		{"sub/dir/ActiveSync.xml", "ActiveSync"},
		{`win\Policy.XML`, "Policy"},
	}
	for _, c := range cases {
		if got := snapshotName(c.in); got != c.want {
			t.Errorf("snapshotName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
