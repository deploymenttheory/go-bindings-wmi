//go:build windows

package wmi

import (
	"unsafe"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/ole"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
)

// decodeSafeArray decodes a one-dimensional SAFEARRAY into a TYPED Go slice
// — []string, []bool, []byte, []int16..., []Row — preserving the element
// width so a decoded Row can be encoded back (putValue/encodeVariant have a
// case for each of these types; CIM uint64/sint64 arrays travel as decimal
// strings both ways). Unsupported element types and multi-dimensional
// arrays (which WMI does not produce for class properties) decode to nil.
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
	if n < 0 {
		return nil
	}

	var data unsafe.Pointer
	if n > 0 {
		if err := ole.SafeArrayAccessData(psa, &data); err != nil {
			return nil
		}
		defer func() { _ = ole.SafeArrayUnaccessData(psa) }()
	}
	size := uintptr(ole.SafeArrayGetElemsize(psa))
	at := func(i int) unsafe.Pointer { return unsafe.Add(data, uintptr(i)*size) }

	switch vt {
	case variant.VT_BSTR:
		return decodeElements(n, func(i int) string {
			if b := *(**uint16)(at(i)); b != nil {
				return win32.UTF16ToString(b)
			}
			return ""
		})
	case variant.VT_BOOL:
		return decodeElements(n, func(i int) bool { return *(*int16)(at(i)) != 0 })
	case variant.VT_UI1:
		return decodeElements(n, func(i int) byte { return *(*uint8)(at(i)) })
	case variant.VT_I1:
		return decodeElements(n, func(i int) int8 { return *(*int8)(at(i)) })
	case variant.VT_I2:
		return decodeElements(n, func(i int) int16 { return *(*int16)(at(i)) })
	case variant.VT_I4, variant.VT_INT:
		return decodeElements(n, func(i int) int32 { return *(*int32)(at(i)) })
	case variant.VT_I8:
		return decodeElements(n, func(i int) int64 { return *(*int64)(at(i)) })
	case variant.VT_UI2:
		return decodeElements(n, func(i int) uint16 { return *(*uint16)(at(i)) })
	case variant.VT_UI4, variant.VT_UINT:
		return decodeElements(n, func(i int) uint32 { return *(*uint32)(at(i)) })
	case variant.VT_UI8:
		return decodeElements(n, func(i int) uint64 { return *(*uint64)(at(i)) })
	case variant.VT_R4:
		return decodeElements(n, func(i int) float32 { return *(*float32)(at(i)) })
	case variant.VT_R8:
		return decodeElements(n, func(i int) float64 { return *(*float64)(at(i)) })
	case variant.VT_UNKNOWN, variant.VT_DISPATCH:
		rows := make([]Row, n)
		for i := range rows {
			row, _ := decodeEmbedded(*(*unsafe.Pointer)(at(i))).(Row)
			rows[i] = row
		}
		return rows
	}
	return nil
}

// decodeElements materializes n elements through get.
func decodeElements[T any](n int, get func(int) T) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = get(i)
	}
	return out
}
