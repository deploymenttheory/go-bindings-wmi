//go:build windows

package csp

import "testing"

func TestInstanceQuery(t *testing.T) {
	got := instanceQuery("*", "MDM_Policy_Result01_Browser02", "./Vendor/MSFT/Policy/Result", "Browser")
	want := `SELECT * FROM MDM_Policy_Result01_Browser02 WHERE ParentID='./Vendor/MSFT/Policy/Result' AND InstanceID='Browser'`
	if got != want {
		t.Errorf("instanceQuery = %q, want %q", got, want)
	}
}

func TestResultParent(t *testing.T) {
	if got := resultParent("./Vendor/MSFT/Policy/Config"); got != "./Vendor/MSFT/Policy/Result" {
		t.Errorf("resultParent = %q", got)
	}
}

func TestReadNotExecutable(t *testing.T) {
	if _, err := Read(nil, Policy{}); err != ErrNotExecutable {
		t.Errorf("Read of non-executable policy err = %v, want ErrNotExecutable", err)
	}
	if err := Set(nil, Policy{}, 1); err != ErrNotExecutable {
		t.Errorf("Set err = %v, want ErrNotExecutable", err)
	}
	if err := Delete(nil, Policy{}); err != ErrNotExecutable {
		t.Errorf("Delete err = %v, want ErrNotExecutable", err)
	}
}
