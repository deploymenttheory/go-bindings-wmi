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

func TestWQLValueNamedTypes(t *testing.T) {
	// The generated enum types are named scalars — they must render as their
	// underlying kind, not fall through to fmt.Sprint.
	type enabledState uint16
	type startMode string
	cases := []struct {
		in   any
		want string
	}{
		{enabledState(3), "3"},
		{startMode("Auto"), "'Auto'"},
		{startMode(`O'Brien`), `'O\'Brien'`},
		{nil, "NULL"},
	}
	for _, c := range cases {
		if got := WQLValue(c.in); got != c.want {
			t.Errorf("WQLValue(%#v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestWhere(t *testing.T) {
	cases := []struct {
		expr string
		args []any
		want string
	}{
		{"Name = ?", []any{"Spooler"}, "Name = 'Spooler'"},
		{"Name = ? AND Started = ?", []any{`O'Brien`, true}, `Name = 'O\'Brien' AND Started = TRUE`},
		{"ProcessId = ?", []any{uint32(4)}, "ProcessId = 4"},
		{"A = ? AND B = ?", []any{1}, "A = 1 AND B = ?"}, // surplus ? stays visible
		{"A = ?", []any{1, 2}, "A = 1"},                  // surplus args ignored
		{"", nil, ""},
	}
	for _, c := range cases {
		if got := Where(c.expr, c.args...); got != c.want {
			t.Errorf("Where(%q, %v) = %s, want %s", c.expr, c.args, got, c.want)
		}
	}
}
