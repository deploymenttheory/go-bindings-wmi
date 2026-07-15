package csp

import "testing"

func TestExecutable(t *testing.T) {
	if (Policy{}).Executable() {
		t.Error("policy with no Bridge should not be executable")
	}
	p := Policy{Bridge: &Bridge{ConfigClass: "MDM_Policy_Config01_Browser02"}}
	if !p.Executable() {
		t.Error("policy with a Bridge should be executable")
	}
}

func TestDeprecated(t *testing.T) {
	if (Policy{}).Deprecated() {
		t.Error("policy with no DeprecatedOSBuild should not be deprecated")
	}
	if !(Policy{DeprecatedOSBuild: "10.0.22000"}).Deprecated() {
		t.Error("policy with DeprecatedOSBuild should be deprecated")
	}
}
