//go:build windows

package csp

import "testing"

func TestInstancePath(t *testing.T) {
	b := &Bridge{
		ConfigClass: "MDM_Policy_Config01_Browser02",
		InstanceID:  "Browser",
		ParentID:    "./Device/Vendor/MSFT/Policy/Config",
	}
	got := instancePath(b.ConfigClass, b)
	want := `MDM_Policy_Config01_Browser02.ParentID="./Device/Vendor/MSFT/Policy/Config",InstanceID="Browser"`
	if got != want {
		t.Errorf("instancePath = %q, want %q", got, want)
	}
}

func TestReadNotExecutable(t *testing.T) {
	if _, err := read(nil, Policy{}, ""); err != ErrNotExecutable {
		t.Errorf("read of non-executable policy err = %v, want ErrNotExecutable", err)
	}
}
