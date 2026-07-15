//go:build windows

package wmi

import (
	"unsafe"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/ole"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
)

// decodeSafeArray decodes a one-dimensional SAFEARRAY of scalar elements into
// []any, widening each element exactly as decodeVariant widens scalars
// (string, int64, uint64, bool, float64). Elements of unsupported types
// decode to nil; multi-dimensional arrays (which WMI does not produce for
// class properties) decode to nil as a whole.
func decodeSafeArray(psa *com.SAFEARRAY) any {
	if psa == nil || ole.SafeArrayGetDim(psa) != 1 {
		return nil
	}
	var vt variant.VARENUM
	if err := ole.SafeArrayGetVartype(psa, &vt); err != nil {
		return nil
	}
	var lo, hi int32
	if err := ole.SafeArrayGetLBound(psa, 1, &lo); err != nil {
		return nil
	}
	if err := ole.SafeArrayGetUBound(psa, 1, &hi); err != nil {
		return nil
	}
	n := int(hi) - int(lo) + 1
	if n <= 0 {
		return []any{}
	}

	var data unsafe.Pointer
	if err := ole.SafeArrayAccessData(psa, &data); err != nil {
		return nil
	}
	defer func() { _ = ole.SafeArrayUnaccessData(psa) }()

	size := uintptr(ole.SafeArrayGetElemsize(psa))
	out := make([]any, n)
	for i := range out {
		out[i] = decodeArrayElement(vt, unsafe.Add(data, uintptr(i)*size))
	}
	return out
}

// decodeArrayElement reads one SAFEARRAY element in place. The element is
// owned by the (locked) array; BSTRs are copied out, never retained.
func decodeArrayElement(vt variant.VARENUM, p unsafe.Pointer) any {
	switch vt {
	case variant.VT_BSTR:
		b := *(**uint16)(p)
		if b == nil {
			return ""
		}
		return win32.UTF16ToString(b)
	case variant.VT_BOOL:
		return *(*int16)(p) != 0
	case variant.VT_I1:
		return int64(*(*int8)(p))
	case variant.VT_I2:
		return int64(*(*int16)(p))
	case variant.VT_I4, variant.VT_INT:
		return int64(*(*int32)(p))
	case variant.VT_I8:
		return *(*int64)(p)
	case variant.VT_UI1:
		return uint64(*(*uint8)(p))
	case variant.VT_UI2:
		return uint64(*(*uint16)(p))
	case variant.VT_UI4, variant.VT_UINT:
		return uint64(*(*uint32)(p))
	case variant.VT_UI8:
		return *(*uint64)(p)
	case variant.VT_R4:
		return float64(*(*float32)(p))
	case variant.VT_R8:
		return *(*float64)(p)
	case variant.VT_UNKNOWN, variant.VT_DISPATCH:
		return decodeEmbedded(*(*unsafe.Pointer)(p))
	}
	return nil
}
