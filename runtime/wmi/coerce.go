package wmi

import "strconv"

// Coercion helpers used by the generated Query<Class> decoders. WMI does not
// return values in the CIM-declared width: most integers arrive as VT_I4
// (decoded here as int64), and 64-bit values (disk sizes, memory) arrive as
// BSTR strings. A plain type assertion against the declared Go type would fail
// and leave the field zero, so the generated code coerces instead.

// AsString returns v as a string ("" for nil; numbers are formatted).
func AsString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	case bool:
		if t {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(t, 10)
	case uint64:
		return strconv.FormatUint(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'g', -1, 64)
	}
	return ""
}

// AsBool returns v as a bool.
func AsBool(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case int64:
		return t != 0
	case uint64:
		return t != 0
	case string:
		return t == "true" || t == "TRUE" || t == "1"
	}
	return false
}

// AsInt64 returns v as an int64 (parsing strings).
func AsInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case uint64:
		return int64(t)
	case float64:
		return int64(t)
	case bool:
		if t {
			return 1
		}
	case string:
		if n, err := strconv.ParseInt(t, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// AsUint64 returns v as a uint64 (parsing strings — WMI's shape for CIM
// uint64 properties such as disk sizes).
func AsUint64(v any) uint64 {
	switch t := v.(type) {
	case uint64:
		return t
	case int64:
		if t >= 0 {
			return uint64(t)
		}
	case float64:
		if t >= 0 {
			return uint64(t)
		}
	case string:
		if n, err := strconv.ParseUint(t, 10, 64); err == nil {
			return n
		}
	}
	return 0
}

// AsFloat64 returns v as a float64 (parsing strings).
func AsFloat64(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case uint64:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	}
	return 0
}

// Narrowing wrappers over the widest coercions.

func AsInt8(v any) int8       { return int8(AsInt64(v)) }
func AsInt16(v any) int16     { return int16(AsInt64(v)) }
func AsInt32(v any) int32     { return int32(AsInt64(v)) }
func AsUint8(v any) uint8     { return uint8(AsUint64(v)) }
func AsUint16(v any) uint16   { return uint16(AsUint64(v)) }
func AsUint32(v any) uint32   { return uint32(AsUint64(v)) }
func AsFloat32(v any) float32 { return float32(AsFloat64(v)) }

// asSlice coerces the []any that the runtime produces for SAFEARRAY
// properties into a typed slice, coercing each element like the scalar
// helpers do. Non-array values (including nil) coerce to a nil slice.
func asSlice[T any](v any, coerce func(any) T) []T {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]T, len(items))
	for i, item := range items {
		out[i] = coerce(item)
	}
	return out
}

// Slice coercers used by the generated Query<Class> decoders for CIM array
// properties.

func AsStringSlice(v any) []string   { return asSlice(v, AsString) }
func AsBoolSlice(v any) []bool       { return asSlice(v, AsBool) }
func AsInt8Slice(v any) []int8       { return asSlice(v, AsInt8) }
func AsInt16Slice(v any) []int16     { return asSlice(v, AsInt16) }
func AsInt32Slice(v any) []int32     { return asSlice(v, AsInt32) }
func AsInt64Slice(v any) []int64     { return asSlice(v, AsInt64) }
func AsUint8Slice(v any) []uint8     { return asSlice(v, AsUint8) }
func AsUint16Slice(v any) []uint16   { return asSlice(v, AsUint16) }
func AsUint32Slice(v any) []uint32   { return asSlice(v, AsUint32) }
func AsUint64Slice(v any) []uint64   { return asSlice(v, AsUint64) }
func AsFloat32Slice(v any) []float32 { return asSlice(v, AsFloat32) }
func AsFloat64Slice(v any) []float64 { return asSlice(v, AsFloat64) }
