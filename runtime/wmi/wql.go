package wmi

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound reports that no instance matched — returned by GetInstance
// and the generated Get<Class> key lookups.
var ErrNotFound = errors.New("wmi: instance not found")

// QuoteWQL renders s as a single-quoted WQL string literal, escaping
// backslashes and quotes (WQL escapes with backslash).
func QuoteWQL(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `'`, `\'`, `"`, `\"`)
	return "'" + replacer.Replace(s) + "'"
}

// WQLValue renders a Go scalar as a WQL literal: strings quoted via
// QuoteWQL, booleans as TRUE/FALSE, numbers in decimal. Used by the
// generated Get<Class> key lookups.
func WQLValue(v any) string {
	switch t := v.(type) {
	case string:
		return QuoteWQL(t)
	case bool:
		if t {
			return "TRUE"
		}
		return "FALSE"
	default:
		return fmt.Sprint(v)
	}
}
