package wmi

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// ParseObjectText parses CIM DTD 2.0 XML embedded-instance text into a Row —
// the inverse of Service.ObjectText. Some providers return embedded
// instances as serialized strings (notably Hyper-V's
// Msvm_KvpExchangeComponent.GuestIntrinsicExchangeItems); this decodes them
// with typed values instead of hand-parsing the XML. Pure Go: Windows
// declines to implement the COM inverse
// (IWbemObjectTextSrc::CreateFromText → WBEM_E_METHOD_NOT_IMPLEMENTED).
//
// Values are typed by each property's TYPE attribute using the Row
// conventions (integers to int64, uint64 to uint64, real to float64,
// boolean to bool, everything else string); array properties become typed
// slices; embedded VALUE.OBJECT instances become nested Rows; properties
// without a VALUE are nil.
func ParseObjectText(text string) (Row, error) {
	decoder := xml.NewDecoder(strings.NewReader(text))
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("wmi: ParseObjectText: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "INSTANCE" {
			// CIM/DECLGROUP/VALUE.OBJECT wrappers — descend until the
			// instance itself.
			continue
		}
		row, err := parseInstanceElement(decoder, start)
		if err != nil {
			return nil, fmt.Errorf("wmi: ParseObjectText: %w", err)
		}
		return row, nil
	}
}

// parseInstanceElement decodes one <INSTANCE> element (the decoder is
// positioned just past its start tag) into a Row.
func parseInstanceElement(decoder *xml.Decoder, start xml.StartElement) (Row, error) {
	row := Row{}
	if class := attrValue(start, "CLASSNAME"); class != "" {
		row["__CLASS"] = class
	}
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			name := attrValue(t, "NAME")
			cimType := attrValue(t, "TYPE")
			switch t.Name.Local {
			case "PROPERTY":
				value, err := parseScalarProperty(decoder, cimType)
				if err != nil {
					return nil, err
				}
				setProperty(row, name, value)
			case "PROPERTY.ARRAY":
				value, err := parseArrayProperty(decoder, t, cimType)
				if err != nil {
					return nil, err
				}
				setProperty(row, name, value)
			case "PROPERTY.REFERENCE":
				value, err := parseReferenceProperty(decoder)
				if err != nil {
					return nil, err
				}
				setProperty(row, name, value)
			case "PROPERTY.OBJECT":
				value, err := parseObjectProperty(decoder, t)
				if err != nil {
					return nil, err
				}
				setProperty(row, name, value)
			default: // QUALIFIER and anything unrecognized
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			return row, nil
		}
	}
}

// setProperty stores a decoded property, keeping NULL (no VALUE child)
// properties present as nil.
func setProperty(row Row, name string, value any) {
	if name == "" {
		return
	}
	row[name] = value
}

// parseScalarProperty consumes a <PROPERTY> subtree and returns its typed
// value (nil when no VALUE child is present).
func parseScalarProperty(decoder *xml.Decoder, cimType string) (any, error) {
	var value any
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local != "VALUE" {
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			text, err := elementText(decoder)
			if err != nil {
				return nil, err
			}
			value = convertCIMValue(text, cimType)
		case xml.EndElement:
			return value, nil
		}
	}
}

// parseArrayProperty consumes a <PROPERTY.ARRAY> subtree and returns a
// typed slice of its VALUE.ARRAY entries (nil when absent).
func parseArrayProperty(decoder *xml.Decoder, start xml.StartElement, cimType string) (any, error) {
	var texts []string
	seen := false
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "VALUE.ARRAY":
				seen = true // descend
			case "VALUE":
				text, err := elementText(decoder)
				if err != nil {
					return nil, err
				}
				texts = append(texts, text)
			default:
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				if !seen {
					return nil, nil
				}
				return convertCIMSlice(texts, cimType), nil
			}
		}
	}
}

// parseReferenceProperty consumes a <PROPERTY.REFERENCE> subtree and
// returns the referenced object path as a string (nil when absent). The
// path may be a bare VALUE.REFERENCE text or a structured
// CLASSPATH/INSTANCEPATH tree — the flattened character data is kept.
func parseReferenceProperty(decoder *xml.Decoder) (any, error) {
	var value any
	var depth int
	var b strings.Builder
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
		case xml.CharData:
			if depth > 0 {
				b.Write(t)
			}
		case xml.EndElement:
			if depth == 0 {
				return value, nil
			}
			depth--
			if depth == 0 {
				value = strings.TrimSpace(b.String())
			}
		}
	}
}

// parseObjectProperty consumes a <PROPERTY.OBJECT> subtree and returns its
// embedded instance as a nested Row (nil when absent).
func parseObjectProperty(decoder *xml.Decoder, start xml.StartElement) (any, error) {
	var value any
	for {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "VALUE.OBJECT": // descend
			case "INSTANCE":
				row, err := parseInstanceElement(decoder, t)
				if err != nil {
					return nil, err
				}
				value = row
			default:
				if err := decoder.Skip(); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local {
				return value, nil
			}
		}
	}
}

// elementText consumes the current element's character data through its end
// tag (nested elements are flattened).
func elementText(decoder *xml.Decoder) (string, error) {
	var b strings.Builder
	depth := 0
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
		case xml.CharData:
			b.Write(t)
		case xml.EndElement:
			if depth == 0 {
				return b.String(), nil
			}
			depth--
		}
	}
}

// convertCIMValue types one VALUE text by its CIM TYPE attribute, following
// the Row widening conventions. Unparseable numerics fall back to the raw
// string — the As* coercers handle them downstream.
func convertCIMValue(text, cimType string) any {
	switch cimType {
	case "boolean":
		return strings.EqualFold(text, "true")
	case "sint8", "sint16", "sint32", "sint64", "uint8", "uint16", "uint32", "char16":
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n
		}
	case "uint64":
		if n, err := strconv.ParseUint(text, 10, 64); err == nil {
			return n
		}
	case "real32", "real64":
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return f
		}
	}
	return text
}

// convertCIMSlice types a VALUE.ARRAY's texts as the typed slice the Row
// conventions promise for array properties.
func convertCIMSlice(texts []string, cimType string) any {
	switch cimType {
	case "boolean":
		out := make([]bool, len(texts))
		for i, t := range texts {
			out[i] = strings.EqualFold(t, "true")
		}
		return out
	case "sint8", "sint16", "sint32", "sint64", "uint8", "uint16", "uint32", "char16":
		out := make([]int64, len(texts))
		for i, t := range texts {
			out[i], _ = strconv.ParseInt(t, 10, 64)
		}
		return out
	case "uint64":
		out := make([]uint64, len(texts))
		for i, t := range texts {
			out[i], _ = strconv.ParseUint(t, 10, 64)
		}
		return out
	case "real32", "real64":
		out := make([]float64, len(texts))
		for i, t := range texts {
			out[i], _ = strconv.ParseFloat(t, 64)
		}
		return out
	}
	out := make([]string, len(texts))
	copy(out, texts)
	return out
}

// attrValue reads one attribute of a start element ("" when absent).
func attrValue(start xml.StartElement, name string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == name {
			return attr.Value
		}
	}
	return ""
}
