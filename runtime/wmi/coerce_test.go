package wmi

import (
	"reflect"
	"testing"
)

// The coercers exist because WMI does not return values in the CIM-declared
// width: most integers arrive as int64 (VT_I4), 64-bit values arrive as
// strings (BSTR), and arrays arrive as []any. These tables pin that contract.

func TestAsString(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"plain", "plain"},
		{nil, ""},
		{true, "true"},
		{false, "false"},
		{int64(-42), "-42"},
		{uint64(18446744073709551615), "18446744073709551615"},
		{float64(1.5), "1.5"},
		{[]any{"not scalar"}, ""},
	}
	for _, c := range cases {
		if got := AsString(c.in); got != c.want {
			t.Errorf("AsString(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAsBool(t *testing.T) {
	cases := []struct {
		in   any
		want bool
	}{
		{true, true},
		{false, false},
		{int64(1), true},
		{int64(0), false},
		{uint64(2), true},
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"false", false},
		{nil, false},
	}
	for _, c := range cases {
		if got := AsBool(c.in); got != c.want {
			t.Errorf("AsBool(%#v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAsInt64(t *testing.T) {
	cases := []struct {
		in   any
		want int64
	}{
		{int64(-7), -7},
		{uint64(7), 7},
		{float64(3.9), 3},
		{true, 1},
		{false, 0},
		{"-123", -123},
		{"not a number", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := AsInt64(c.in); got != c.want {
			t.Errorf("AsInt64(%#v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestAsUint64(t *testing.T) {
	cases := []struct {
		in   any
		want uint64
	}{
		{uint64(7), 7},
		{int64(7), 7},
		{int64(-1), 0}, // negative never wraps
		{float64(-1), 0},
		{"18446744073709551615", 18446744073709551615}, // WMI's BSTR shape for CIM uint64
		{"junk", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := AsUint64(c.in); got != c.want {
			t.Errorf("AsUint64(%#v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestAsFloat64(t *testing.T) {
	cases := []struct {
		in   any
		want float64
	}{
		{float64(1.25), 1.25},
		{int64(-2), -2},
		{uint64(2), 2},
		{"3.5", 3.5},
		{"junk", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := AsFloat64(c.in); got != c.want {
			t.Errorf("AsFloat64(%#v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNarrowingCoercers(t *testing.T) {
	if got := AsInt32(int64(-40)); got != -40 {
		t.Errorf("AsInt32 = %d", got)
	}
	if got := AsUint32(int64(12)); got != 12 {
		t.Errorf("AsUint32 = %d", got)
	}
	if got := AsUint16("65535"); got != 65535 {
		t.Errorf("AsUint16 = %d", got)
	}
	if got := AsInt8(int64(-128)); got != -128 {
		t.Errorf("AsInt8 = %d", got)
	}
	if got := AsFloat32(float64(0.5)); got != 0.5 {
		t.Errorf("AsFloat32 = %v", got)
	}
}

func TestSliceCoercers(t *testing.T) {
	// The runtime decodes SAFEARRAYs to []any with widened elements; slice
	// coercers narrow per element.
	in := []any{int64(1), int64(2), int64(3)}
	if got := AsUint16Slice(any(in)); !reflect.DeepEqual(got, []uint16{1, 2, 3}) {
		t.Errorf("AsUint16Slice = %v", got)
	}
	if got := AsStringSlice(any([]any{"a", "b"})); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("AsStringSlice = %v", got)
	}
	if got := AsUint64Slice(any([]any{"9007199254740993"})); !reflect.DeepEqual(got, []uint64{9007199254740993}) {
		t.Errorf("AsUint64Slice = %v", got)
	}
	if got := AsBoolSlice(any([]any{true, int64(0)})); !reflect.DeepEqual(got, []bool{true, false}) {
		t.Errorf("AsBoolSlice = %v", got)
	}
	// Non-arrays (nil included) coerce to a nil slice, mirroring the scalar
	// zero-on-mismatch behavior.
	if got := AsStringSlice(nil); got != nil {
		t.Errorf("AsStringSlice(nil) = %v, want nil", got)
	}
	if got := AsInt32Slice("scalar"); got != nil {
		t.Errorf("AsInt32Slice(scalar) = %v, want nil", got)
	}
	// Empty arrays stay empty, not nil.
	if got := AsFloat64Slice(any([]any{})); got == nil || len(got) != 0 {
		t.Errorf("AsFloat64Slice(empty) = %#v, want empty non-nil", got)
	}
}
