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
		psa, err := stringSafeArray(t)
		if err != nil {
			return err
		}
		pointer.Vt = uint16(variant.VT_ARRAY | variant.VT_BSTR)
		pointer.P = unsafe.Pointer(psa)
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

// stringSafeArray builds a one-dimensional SAFEARRAY of BSTR.
// SafeArrayPutElement copies string elements, so the temporaries are freed
// here; SafeArrayDestroy (via VariantClear) frees the copies.
func stringSafeArray(values []string) (*com.SAFEARRAY, error) {
	bound := com.SAFEARRAYBOUND{CElements: uint32(len(values))}
	psa := ole.SafeArrayCreate(variant.VT_BSTR, 1, &bound)
	if psa == nil {
		return nil, fmt.Errorf("wmi: SafeArrayCreate failed for %d elements", len(values))
	}
	for i, s := range values {
		b := foundation.SysAllocString(s)
		index := int32(i)
		err := ole.SafeArrayPutElement(psa, &index, unsafe.Pointer(b))
		foundation.SysFreeString(b)
		if err != nil {
			_ = ole.SafeArrayDestroy(psa)
			return nil, fmt.Errorf("wmi: SafeArrayPutElement(%d): %w", i, err)
		}
	}
	return psa, nil
}
