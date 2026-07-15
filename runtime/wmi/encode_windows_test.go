//go:build windows

package wmi

import (
	"testing"
	"unsafe"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
)

// TestEncodeRowArray exercises the []wmi.Row → SAFEARRAY-of-VT_UNKNOWN
// encode path against a live service (spawning embedded instances needs
// GetObject). It builds an array of Win32_ProcessStartup rows and asserts
// the VARIANT shape, without needing a provider method that consumes one.
func TestEncodeRowArray(t *testing.T) {
	svc, err := Connect(`root\cimv2`)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer svc.Close()

	rows := []Row{
		Instance("Win32_ProcessStartup", map[string]any{"ShowWindow": uint16(0)}),
		Instance("Win32_ProcessStartup", map[string]any{"ShowWindow": uint16(1)}),
	}

	var v variant.VARIANT
	variant.VariantInit(&v)
	defer variant.VariantClear(&v)
	if err := svc.encodeRowArray(rows, &v); err != nil {
		t.Fatalf("encodeRowArray: %v", err)
	}

	scalar := (*variantScalar)(unsafe.Pointer(&v.Anonymous))
	if want := uint16(variant.VT_ARRAY | variant.VT_UNKNOWN); scalar.Vt != want {
		t.Errorf("VARIANT type = %#x, want %#x", scalar.Vt, want)
	}

	// The SAFEARRAY decodes back to two embedded Rows.
	decoded, ok := decodeVariant(&v).([]any)
	if !ok || len(decoded) != 2 {
		t.Fatalf("decoded = %#v, want 2 elements", decoded)
	}
	for i, elem := range decoded {
		row, ok := elem.(Row)
		if !ok {
			t.Errorf("element %d is %T, want Row", i, elem)
			continue
		}
		if class := AsString(row["__CLASS"]); class != "Win32_ProcessStartup" {
			t.Errorf("element %d __CLASS = %q", i, class)
		}
	}
}
