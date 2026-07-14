//go:build windows

package wmi

import "testing"

// TestQueryOperatingSystem drives a real WQL query end-to-end: connect to
// root\cimv2, query Win32_OperatingSystem, and decode its VARIANTs.
func TestQueryOperatingSystem(t *testing.T) {
	svc, err := Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	rows, err := svc.Query("SELECT Caption, BuildNumber, OSArchitecture FROM Win32_OperatingSystem")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("no Win32_OperatingSystem instances")
	}
	caption, _ := rows[0]["Caption"].(string)
	if caption == "" {
		t.Errorf("empty Caption; row = %v", rows[0])
	}
	t.Logf("OS: %v build %v (%v)", rows[0]["Caption"], rows[0]["BuildNumber"], rows[0]["OSArchitecture"])
}
