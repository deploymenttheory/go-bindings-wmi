package wmi

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// CIM datetime properties arrive as 25-character DMTF strings, either a
// timestamp (yyyymmddHHMMSS.mmmmmm±UUU, offset in minutes from UTC) or an
// interval (ddddddddHHMMSS.mmmmmm:000). ParseDMTF and ParseDMTFInterval turn
// them into time.Time / time.Duration.

// ErrInvalidDMTF is returned (wrapped) for strings that are not valid DMTF
// datetime values.
var ErrInvalidDMTF = errors.New("invalid DMTF datetime")

const dmtfLen = 25

// ParseDMTF parses a DMTF timestamp such as "20260714120000.000000+060"
// into a time.Time in the fixed zone the offset records. Wildcarded
// microseconds ("******", produced by some providers) are treated as zero.
// Interval values (":" separator) are rejected — use ParseDMTFInterval.
func ParseDMTF(s string) (time.Time, error) {
	if len(s) != dmtfLen || s[14] != '.' {
		return time.Time{}, fmt.Errorf("wmi: %w: %q", ErrInvalidDMTF, s)
	}
	sign := 0
	switch s[21] {
	case '+':
		sign = 1
	case '-':
		sign = -1
	case ':':
		return time.Time{}, fmt.Errorf("wmi: %w: %q is an interval (use ParseDMTFInterval)", ErrInvalidDMTF, s)
	default:
		return time.Time{}, fmt.Errorf("wmi: %w: %q", ErrInvalidDMTF, s)
	}

	fields, err := dmtfFields(s, [][2]int{{0, 4}, {4, 6}, {6, 8}, {8, 10}, {10, 12}, {12, 14}, {22, 25}})
	if err != nil {
		return time.Time{}, err
	}
	year, month, day := fields[0], fields[1], fields[2]
	hour, minute, second, offsetMinutes := fields[3], fields[4], fields[5], fields[6]

	micro, err := dmtfMicroseconds(s)
	if err != nil {
		return time.Time{}, err
	}

	zone := time.FixedZone("", sign*offsetMinutes*60)
	return time.Date(year, time.Month(month), day, hour, minute, second, micro*1000, zone), nil
}

// ParseDMTFInterval parses a DMTF interval such as "00000001020304.000000:000"
// (1 day, 2 hours, 3 minutes, 4 seconds) into a time.Duration.
func ParseDMTFInterval(s string) (time.Duration, error) {
	if len(s) != dmtfLen || s[14] != '.' || s[21] != ':' {
		return 0, fmt.Errorf("wmi: %w: %q is not an interval", ErrInvalidDMTF, s)
	}
	fields, err := dmtfFields(s, [][2]int{{0, 8}, {8, 10}, {10, 12}, {12, 14}})
	if err != nil {
		return 0, err
	}
	days, hours, minutes, seconds := fields[0], fields[1], fields[2], fields[3]

	micro, err := dmtfMicroseconds(s)
	if err != nil {
		return 0, err
	}

	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(minutes)*time.Minute +
		time.Duration(seconds)*time.Second +
		time.Duration(micro)*time.Microsecond, nil
}

// dmtfFields parses the given [start, end) digit runs of a DMTF string.
func dmtfFields(s string, spans [][2]int) ([]int, error) {
	out := make([]int, len(spans))
	for i, span := range spans {
		n, err := strconv.Atoi(s[span[0]:span[1]])
		if err != nil || n < 0 {
			return nil, fmt.Errorf("wmi: %w: %q", ErrInvalidDMTF, s)
		}
		out[i] = n
	}
	return out, nil
}

// dmtfMicroseconds parses the microsecond field, allowing the all-wildcard
// form some providers emit.
func dmtfMicroseconds(s string) (int, error) {
	if s[15:21] == "******" {
		return 0, nil
	}
	micro, err := strconv.Atoi(s[15:21])
	if err != nil || micro < 0 {
		return 0, fmt.Errorf("wmi: %w: %q", ErrInvalidDMTF, s)
	}
	return micro, nil
}
