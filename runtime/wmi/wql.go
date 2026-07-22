package wmi

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
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
// QuoteWQL, booleans as TRUE/FALSE, numbers in decimal, nil as NULL. It
// works on the reflected kind, so named types with scalar underlying types —
// the generated enum types — render the same way. Used by the generated
// Get<Class> key lookups and by Where.
func WQLValue(v any) string {
	if v == nil {
		return "NULL"
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return QuoteWQL(rv.String())
	case reflect.Bool:
		if rv.Bool() {
			return "TRUE"
		}
		return "FALSE"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

// Where renders a WHERE clause body, substituting each ? with the WQL
// literal of the corresponding arg (via WQLValue, so strings are quoted and
// escaped):
//
//	v2.QueryMsvmComputerSystem(svc, wmi.Where("Name = ?", id))
//
// Every value should arrive through an arg — a literal ? elsewhere in expr
// is substituted too. Count mismatches are programming errors and stay
// visible: surplus ?s are left as-is, surplus args are ignored.
func Where(expr string, args ...any) string {
	var b strings.Builder
	next := 0
	for i := range len(expr) {
		if expr[i] == '?' && next < len(args) {
			b.WriteString(WQLValue(args[next]))
			next++
			continue
		}
		b.WriteByte(expr[i])
	}
	return b.String()
}
