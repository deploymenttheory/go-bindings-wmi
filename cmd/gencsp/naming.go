package main

import (
	"strconv"
	"strings"
)

// packageName derives a Go package name from a CSP snapshot base name,
// dropping the "_AreaDDF" suffix and lowercasing to alphanumerics
// (AboveLock_AreaDDF → abovelock, ADMX_AppCompat_AreaDDF → admxappcompat).
// Policy areas are prefixed "policy" so an area (Bitlocker_AreaDDF →
// policybitlocker) never collides with a same-named standalone CSP
// (BitLocker → bitlocker).
func packageName(base string, policyArea bool) string {
	base = strings.TrimSuffix(base, "_AreaDDF")
	var b strings.Builder
	if policyArea {
		b.WriteString("policy")
	}
	for _, r := range base {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r - 'A' + 'a')
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" || name == "policy" {
		name += "csp"
	}
	return name
}

// joinExport exports and concatenates path segments into a unique Go
// identifier (segments below the CSP root make a leaf's name unique).
func joinExport(segments []string) string {
	var b strings.Builder
	for _, seg := range segments {
		b.WriteString(exportName(seg))
	}
	name := b.String()
	if name == "" || (name[0] >= '0' && name[0] <= '9') {
		name = "P" + name // keep it a valid, exported identifier
	}
	return name
}

// exportName upper-cases the first letter of each word and drops characters
// Go rejects in identifiers. Word boundaries: '_', '-', '/', '.', spaces,
// and other punctuation (CSP node names include braces and slashes).
func exportName(name string) string {
	var out []rune
	upperNext := true
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			if upperNext {
				r = r - 'a' + 'A'
			}
			out = append(out, r)
			upperNext = false
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			out = append(out, r)
			upperNext = false
		default:
			upperNext = true
		}
	}
	return string(out)
}

// constName renders an enum description as an identifier fragment.
func constName(display string) string {
	return exportName(display)
}

// intLiteral parses a stored enum value as an int64 literal.
func intLiteral(stored string) (string, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(stored), 0, 64)
	if err != nil {
		return "", false
	}
	return strconv.FormatInt(n, 10), true
}

// firstLine returns the first line of s, collapsed to a single comment line.
func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// quoteJoin renders a string slice as quoted, comma-separated Go literals.
func quoteJoin(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = strconv.Quote(s)
	}
	return strings.Join(quoted, ", ")
}
