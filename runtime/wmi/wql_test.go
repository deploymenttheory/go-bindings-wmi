package wmi

import "testing"

func TestQuoteWQL(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Spooler", "'Spooler'"},
		{"", "''"},
		{`O'Brien`, `'O\'Brien'`},
		{`C:\Windows\System32`, `'C:\\Windows\\System32'`},
		{`say "hi"`, `'say \"hi\"'`},
	}
	for _, c := range cases {
		if got := QuoteWQL(c.in); got != c.want {
			t.Errorf("QuoteWQL(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestWQLValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"x", "'x'"},
		{true, "TRUE"},
		{false, "FALSE"},
		{uint32(42), "42"},
		{int64(-7), "-7"},
		{uint64(18446744073709551615), "18446744073709551615"},
		{3.5, "3.5"},
	}
	for _, c := range cases {
		if got := WQLValue(c.in); got != c.want {
			t.Errorf("WQLValue(%#v) = %s, want %s", c.in, got, c.want)
		}
	}
}
