package wmi

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// ObjectPath renders a key-qualified relative object path for GetInstance,
// ExecMethod, and the generated method wrappers:
//
//	wmi.ObjectPath("Msvm_ComputerSystem", map[string]any{"Name": id})
//	// Msvm_ComputerSystem.Name="..."
//
// Keys are emitted sorted (deterministic); string values are double-quoted
// with \ and " backslash-escaped (WMI path syntax), booleans render as
// TRUE/FALSE, integers in decimal. No keys renders the singleton form
// "Class=@".
func ObjectPath(class string, keys map[string]any) string {
	if len(keys) == 0 {
		return class + "=@"
	}
	names := make([]string, 0, len(keys))
	for name := range keys {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = name + "=" + pathValue(keys[name])
	}
	return class + "." + strings.Join(parts, ",")
}

// pathValue renders one key value in object-path syntax. Like WQLValue it
// works on the reflected kind, so named types (the generated enums) render
// as their underlying scalar.
func pathValue(v any) string {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return quotePath(rv.String())
	case reflect.Bool:
		if rv.Bool() {
			return "TRUE"
		}
		return "FALSE"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	}
	return fmt.Sprint(v)
}

// quotePath renders s as a double-quoted path literal, escaping backslashes
// and quotes.
func quotePath(s string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(s) + `"`
}

// PathRef is a parsed WMI object path.
type PathRef struct {
	// Server and Namespace are set for absolute paths
	// (\\server\root\cimv2:...); empty for relative paths.
	Server    string
	Namespace string
	Class     string
	// Keys maps key property name to its unescaped value ("Spooler" for
	// Name="Spooler", "0" for Index=0). Empty for class and singleton paths.
	Keys map[string]string
	// Singleton reports the Class=@ form.
	Singleton bool
}

// ParsePath splits a __PATH or __RELPATH into its parts — the structured
// counterpart of the strings the runtime and generated wrappers pass around:
//
//	ref, _ := wmi.ParsePath(`\\HOST\root\virtualization\v2:Msvm_ComputerSystem.Name="4764334d-..."`)
//	ref.Keys["Name"] // "4764334d-..."
//
// Handles the optional \\server\namespace: prefix, a bare namespace: prefix,
// quoted key values with \" and \\ escapes, and the singleton form Class=@.
func ParsePath(path string) (PathRef, error) {
	var ref PathRef
	rest := path

	if strings.HasPrefix(rest, `\\`) {
		body := rest[2:]
		i := strings.IndexByte(body, '\\')
		if i < 0 {
			return ref, fmt.Errorf("wmi: ParsePath(%q): no namespace after server", path)
		}
		ref.Server = body[:i]
		body = body[i+1:]
		j := strings.IndexByte(body, ':')
		if j < 0 {
			return ref, fmt.Errorf("wmi: ParsePath(%q): no ':' after namespace", path)
		}
		ref.Namespace = body[:j]
		rest = body[j+1:]
	} else if i := strings.IndexByte(rest, ':'); i >= 0 && i < firstIndexAny(rest, `."=`) {
		// Namespace-qualified relative path (root\cimv2:Class...).
		ref.Namespace = rest[:i]
		rest = rest[i+1:]
	}

	// Class runs to the first '.' (keys follow) or '=' (singleton).
	end := firstIndexAny(rest, ".=")
	if end == len(rest) {
		ref.Class = rest
		if ref.Class == "" {
			return ref, fmt.Errorf("wmi: ParsePath(%q): empty class", path)
		}
		return ref, nil
	}
	ref.Class = rest[:end]
	if ref.Class == "" {
		return ref, fmt.Errorf("wmi: ParsePath(%q): empty class", path)
	}
	if rest[end] == '=' {
		if rest[end:] != "=@" {
			return ref, fmt.Errorf("wmi: ParsePath(%q): malformed singleton", path)
		}
		ref.Singleton = true
		return ref, nil
	}
	rest = rest[end+1:]

	ref.Keys = make(map[string]string)
	for len(rest) > 0 {
		eq := strings.IndexByte(rest, '=')
		if eq <= 0 {
			return ref, fmt.Errorf("wmi: ParsePath(%q): malformed key clause %q", path, rest)
		}
		key := rest[:eq]
		rest = rest[eq+1:]
		var value string
		if len(rest) > 0 && rest[0] == '"' {
			var ok bool
			value, rest, ok = unquotePath(rest)
			if !ok {
				return ref, fmt.Errorf("wmi: ParsePath(%q): unterminated string for key %s", path, key)
			}
			if len(rest) > 0 && rest[0] != ',' {
				return ref, fmt.Errorf("wmi: ParsePath(%q): trailing garbage after key %s", path, key)
			}
		} else if i := strings.IndexByte(rest, ','); i >= 0 {
			value, rest = rest[:i], rest[i:]
		} else {
			value, rest = rest, ""
		}
		ref.Keys[key] = value
		if len(rest) > 0 {
			rest = rest[1:] // skip the ','
			if rest == "" {
				return ref, fmt.Errorf("wmi: ParsePath(%q): trailing ','", path)
			}
		}
	}
	if len(ref.Keys) == 0 {
		return ref, fmt.Errorf("wmi: ParsePath(%q): no keys after '.'", path)
	}
	return ref, nil
}

// unquotePath consumes a leading double-quoted path literal (with \" and \\
// escapes), returning the unescaped value and the remainder after the
// closing quote.
func unquotePath(s string) (value, rest string, ok bool) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if i+1 >= len(s) {
				return "", "", false
			}
			i++
			b.WriteByte(s[i])
		case '"':
			return b.String(), s[i+1:], true
		default:
			b.WriteByte(s[i])
		}
	}
	return "", "", false
}

// firstIndexAny is strings.IndexAny returning len(s) instead of -1.
func firstIndexAny(s, chars string) int {
	if i := strings.IndexAny(s, chars); i >= 0 {
		return i
	}
	return len(s)
}
