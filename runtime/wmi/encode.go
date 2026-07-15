//go:build windows

package wmi

import (
	"fmt"
	"math"
	"strconv"
	"unsafe"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/ole"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
)

// encodeVariant fills v (already VariantInit'd) from a Go value, following
// WMI's put conventions: integers travel as VT_I4 when they fit 32 bits and
// as decimal BSTR strings otherwise (WMI's own 64-bit convention), strings
// as BSTR, string slices as a SAFEARRAY of BSTR. The caller owns v and must
// VariantClear it (which frees the BSTR/array). Unsupported types error.
func encodeVariant(value any, v *variant.VARIANT) error {
	scalar := (*variantScalar)(unsafe.Pointer(&v.Anonymous))
	pointer := (*variantPtr)(unsafe.Pointer(&v.Anonymous))

	switch t := value.(type) {
	case string:
		pointer.Vt = uint16(variant.VT_BSTR)
		pointer.P = unsafe.Pointer(foundation.SysAllocString(t))
	case bool:
		scalar.Vt = uint16(variant.VT_BOOL)
		if t {
			scalar.U64 = 0xFFFF // VARIANT_TRUE
		}
	case int8:
		encodeInt(scalar, pointer, int64(t))
	case int16:
		encodeInt(scalar, pointer, int64(t))
	case int32:
		encodeInt(scalar, pointer, int64(t))
	case int:
		encodeInt(scalar, pointer, int64(t))
	case int64:
		encodeInt(scalar, pointer, t)
	case uint8:
		encodeUint(scalar, pointer, uint64(t))
	case uint16:
		encodeUint(scalar, pointer, uint64(t))
	case uint32:
		encodeUint(scalar, pointer, uint64(t))
	case uint:
		encodeUint(scalar, pointer, uint64(t))
	case uint64:
		encodeUint(scalar, pointer, t)
	case float32:
		scalar.Vt = uint16(variant.VT_R4)
		scalar.U64 = uint64(math.Float32bits(t))
	case float64:
		scalar.Vt = uint16(variant.VT_R8)
		scalar.U64 = math.Float64bits(t)
	case []string:
		return encodeArray(pointer, variant.VT_BSTR, t, putBSTRElement)
	case []bool:
		return encodeArray(pointer, variant.VT_BOOL, t, func(v bool) (unsafe.Pointer, func()) {
			e := int16(0)
			if v {
				e = -1 // VARIANT_TRUE
			}
			return unsafe.Pointer(&e), nil
		})
	case []int8:
		return encodeArray(pointer, variant.VT_I2, t, scalarElement(func(v int8) int16 { return int16(v) }))
	case []int16:
		return encodeArray(pointer, variant.VT_I2, t, scalarElement(func(v int16) int16 { return v }))
	case []int32:
		return encodeArray(pointer, variant.VT_I4, t, scalarElement(func(v int32) int32 { return v }))
	case []int64:
		// CIM sint64/uint64 arrays travel as strings, like their scalars.
		return encodeArray(pointer, variant.VT_BSTR, t, func(v int64) (unsafe.Pointer, func()) {
			return putBSTRElement(strconv.FormatInt(v, 10))
		})
	case []uint8:
		return encodeArray(pointer, variant.VT_UI1, t, scalarElement(func(v uint8) uint8 { return v }))
	case []uint16:
		return encodeArray(pointer, variant.VT_I4, t, scalarElement(func(v uint16) int32 { return int32(v) }))
	case []uint32:
		return encodeArray(pointer, variant.VT_I4, t, scalarElement(func(v uint32) int32 { return int32(v) }))
	case []uint64:
		return encodeArray(pointer, variant.VT_BSTR, t, func(v uint64) (unsafe.Pointer, func()) {
			return putBSTRElement(strconv.FormatUint(v, 10))
		})
	case []float32:
		return encodeArray(pointer, variant.VT_R4, t, scalarElement(func(v float32) float32 { return v }))
	case []float64:
		return encodeArray(pointer, variant.VT_R8, t, scalarElement(func(v float64) float64 { return v }))
	default:
		return fmt.Errorf("wmi: cannot encode %T into a VARIANT", value)
	}
	return nil
}

// encodeInt follows WMI's convention: VT_I4 when it fits, decimal BSTR when
// it doesn't (CIM sint64/uint64 properties expect the string form).
func encodeInt(scalar *variantScalar, pointer *variantPtr, n int64) {
	if n >= math.MinInt32 && n <= math.MaxInt32 {
		scalar.Vt = uint16(variant.VT_I4)
		scalar.U64 = uint64(uint32(int32(n)))
		return
	}
	pointer.Vt = uint16(variant.VT_BSTR)
	pointer.P = unsafe.Pointer(foundation.SysAllocString(strconv.FormatInt(n, 10)))
}

// encodeUint mirrors encodeInt: values fitting 32 bits travel as VT_I4 bit
// patterns (what CIM uint8..uint32 properties expect), larger as strings.
func encodeUint(scalar *variantScalar, pointer *variantPtr, n uint64) {
	if n <= math.MaxUint32 {
		scalar.Vt = uint16(variant.VT_I4)
		scalar.U64 = n
		return
	}
	pointer.Vt = uint16(variant.VT_BSTR)
	pointer.P = unsafe.Pointer(foundation.SysAllocString(strconv.FormatUint(n, 10)))
}

// encodeArray builds a one-dimensional SAFEARRAY of vt from a Go slice and
// stores it in the VARIANT. element yields a pointer to one element's
// marshaled form plus an optional cleanup (SafeArrayPutElement copies, so
// temporaries are freed immediately; VariantClear destroys the array).
func encodeArray[T any](pointer *variantPtr, vt variant.VARENUM, values []T,
	element func(T) (unsafe.Pointer, func()),
) error {
	bound := com.SAFEARRAYBOUND{CElements: uint32(len(values))}
	psa := ole.SafeArrayCreate(vt, 1, &bound)
	if psa == nil {
		return fmt.Errorf("wmi: SafeArrayCreate(%d) failed for %d elements", vt, len(values))
	}
	for i, v := range values {
		p, cleanup := element(v)
		index := int32(i)
		err := ole.SafeArrayPutElement(psa, &index, p)
		if cleanup != nil {
			cleanup()
		}
		if err != nil {
			_ = ole.SafeArrayDestroy(psa)
			return fmt.Errorf("wmi: SafeArrayPutElement(%d): %w", i, err)
		}
	}
	pointer.Vt = uint16(variant.VT_ARRAY | vt)
	pointer.P = unsafe.Pointer(psa)
	return nil
}

// scalarElement adapts a fixed-width conversion for encodeArray.
func scalarElement[T, E any](conv func(T) E) func(T) (unsafe.Pointer, func()) {
	return func(v T) (unsafe.Pointer, func()) {
		e := conv(v)
		return unsafe.Pointer(&e), nil
	}
}

// putBSTRElement marshals one string element; SafeArrayPutElement copies
// BSTRs, so ours is freed right after the put.
func putBSTRElement(s string) (unsafe.Pointer, func()) {
	b := foundation.SysAllocString(s)
	return unsafe.Pointer(b), func() { foundation.SysFreeString(b) }
}
