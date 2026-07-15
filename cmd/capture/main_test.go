//go:build windows

package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	wmibind "github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

func TestAccessHint(t *testing.T) {
	denied := fmt.Errorf("wmi: ConnectServer: %w", win32.HRESULT(wmibind.WBEM_E_ACCESS_DENIED))
	hint := accessHint(denied, `root\cimv2\mdm\dmmap`)
	if !strings.Contains(hint, "SYSTEM") || !strings.Contains(hint, "Capture-MdmBridge.ps1") {
		t.Errorf("access-denied hint = %q, want SYSTEM + script guidance", hint)
	}
	if !strings.Contains(hint, `root\cimv2\mdm\dmmap`) {
		t.Errorf("hint omits the namespace: %q", hint)
	}

	// Unrelated failures get no hint.
	if h := accessHint(errors.New("some other failure"), `root\cimv2`); h != "" {
		t.Errorf("non-access-denied hint = %q, want empty", h)
	}
	notFound := fmt.Errorf("x: %w", win32.HRESULT(wmibind.WBEM_E_NOT_FOUND))
	if h := accessHint(notFound, `root\cimv2`); h != "" {
		t.Errorf("WBEM_E_NOT_FOUND hint = %q, want empty", h)
	}
}
