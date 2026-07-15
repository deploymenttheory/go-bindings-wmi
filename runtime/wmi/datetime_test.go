package wmi

import (
	"errors"
	"testing"
	"time"
)

func TestParseDMTF(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{
			// UTC.
			"20260714120000.000000+000",
			time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
		},
		{
			// BST (+60 minutes), with fractional seconds.
			"20260714091657.500000+060",
			time.Date(2026, 7, 14, 9, 16, 57, 500_000_000, time.FixedZone("", 3600)),
		},
		{
			// Negative offset.
			"19991231235959.999999-300",
			time.Date(1999, 12, 31, 23, 59, 59, 999_999_000, time.FixedZone("", -300*60)),
		},
		{
			// Wildcarded microseconds (some providers emit this).
			"20260101000000.******+000",
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, c := range cases {
		got, err := ParseDMTF(c.in)
		if err != nil {
			t.Errorf("ParseDMTF(%q): %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("ParseDMTF(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseDMTFRejects(t *testing.T) {
	bad := []string{
		"",
		"20260714120000",              // too short
		"20260714120000.000000+0000",  // too long
		"20260714120000.000000:000",   // interval, wrong parser
		"20260714120000x000000+000",   // no dot
		"2026071412000o.000000+000",   // non-digit
		"20260714120000.00000o+000",   // non-digit micros
		"20260714120000.000000*000",   // bad sign
	}
	for _, s := range bad {
		if _, err := ParseDMTF(s); !errors.Is(err, ErrInvalidDMTF) {
			t.Errorf("ParseDMTF(%q) err = %v, want ErrInvalidDMTF", s, err)
		}
	}
}

func TestParseDMTFInterval(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{
			"00000001020304.000000:000",
			24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second,
		},
		{
			"00000000000000.500000:000",
			500 * time.Millisecond,
		},
		{
			"00000365000000.000000:000",
			365 * 24 * time.Hour,
		},
	}
	for _, c := range cases {
		got, err := ParseDMTFInterval(c.in)
		if err != nil {
			t.Errorf("ParseDMTFInterval(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseDMTFInterval(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseDMTFIntervalRejects(t *testing.T) {
	bad := []string{
		"",
		"20260714120000.000000+000", // timestamp, wrong parser
		"0000000102030.000000:000",  // too short
		"0000000102030o.000000:000", // non-digit
	}
	for _, s := range bad {
		if _, err := ParseDMTFInterval(s); !errors.Is(err, ErrInvalidDMTF) {
			t.Errorf("ParseDMTFInterval(%q) err = %v, want ErrInvalidDMTF", s, err)
		}
	}
}
