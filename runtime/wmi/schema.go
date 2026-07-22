//go:build windows

package wmi

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unsafe"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// PropertyInfo describes one CIM class property (schema, not a value).
type PropertyInfo struct {
	Name    string
	CIMType int32 // CIMTYPE_ENUMERATION, with the CIM_FLAG_ARRAY (0x2000) bit intact
	Key     bool  // the [key] qualifier
	// Values and ValueMap are the CIM enumeration qualifiers (display names
	// and stored values respectively); BitValues and BitMap the bitmask
	// equivalents (flag names and bit positions).
	Values    []string
	ValueMap  []string
	BitValues []string
	BitMap    []string
}

// enumQuals are the enumeration/bitmask qualifiers a derived class inherits
// from the declaring base class.
type enumQuals struct {
	values, valueMap, bitValues, bitMap []string
}

func (q enumQuals) empty() bool {
	return len(q.values) == 0 && len(q.valueMap) == 0 && len(q.bitValues) == 0 && len(q.bitMap) == 0
}

// MethodInfo describes one CIM class method: its in/out parameters in
// declaration ([ID]) order, out including ReturnValue last.
type MethodInfo struct {
	Name   string
	Static bool // the [static] qualifier
	In     []PropertyInfo
	Out    []PropertyInfo
}

const (
	wbemFlagUseAmendedQualifiers = 0x20000
	wbemFlagDeep                 = 0x0
	cimFlagArray                 = 0x2000
)

// ClassNames enumerates every class in the connected namespace (deep), sorted
// for deterministic snapshots. Use with ClassProperties to capture a whole
// namespace.
func (s *Service) ClassNames() ([]string, error) {
	var enum *wmi.IEnumWbemClassObject
	if err := s.services.CreateClassEnum(nil,
		wbemFlagDeep|wbemFlagForwardOnly|wbemFlagReturnImmediately, nil, &enum); err != nil {
		return nil, fmt.Errorf("wmi: CreateClassEnum: %w", err)
	}
	defer enum.Release()

	var names []string
	for {
		var obj *wmi.IWbemClassObject
		var returned uint32
		objects := []*wmi.IWbemClassObject{obj}
		hr, err := enum.Next(-1, objects, &returned)
		if err != nil {
			return nil, fmt.Errorf("wmi: enumerate classes: %w", err)
		}
		if returned == 0 || hr == win32.HRESULT(wmi.WBEM_S_FALSE) {
			break
		}
		name := classNameOf(objects[0])
		objects[0].Release()
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// classNameOf reads the __CLASS system property of a class object.
func classNameOf(obj *wmi.IWbemClassObject) string {
	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := obj.Get("__CLASS", 0, &value, nil, nil); err != nil {
		return ""
	}
	p := (*classNameVariant)(unsafe.Pointer(&value.Anonymous))
	if p.Vt != vtBSTR {
		return ""
	}
	return win32.UTF16ToString((*uint16)(p.P))
}

const vtBSTR = 8

type classNameVariant struct {
	Vt uint16
	_  [6]byte
	P  unsafe.Pointer
	_  uint64
}

// ClassProperties returns the schema (property name + CIM type) of a class,
// sorted by name for deterministic snapshots.
func (s *Service) ClassProperties(className string) ([]PropertyInfo, error) {
	name := foundation.SysAllocString(className)
	defer foundation.SysFreeString(name)

	var class *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(name, wbemFlagUseAmendedQualifiers, nil, &class, &callResult); err != nil {
		return nil, fmt.Errorf("wmi: GetObject(%s): %w", className, err)
	}
	defer class.Release()

	props, err := enumerateProperties(class)
	if err != nil {
		return nil, fmt.Errorf("wmi: %s: %w", className, err)
	}

	// Qualifier reads happen after EndEnumeration — qualifier-set calls must
	// not interleave with a property enumeration in flight.
	for i := range props {
		readPropertyQualifiers(class, &props[i])
	}

	// Enumeration qualifiers are declared once, on the class that introduces
	// the property — a derived class's qualifier set does not carry them
	// (EnabledState's values live on CIM_EnabledLogicalElement, not on
	// Msvm_ComputerSystem). Fill the gaps from the derivation chain.
	s.inheritEnumQualifiers(class, props)

	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	return props, nil
}

// inheritEnumQualifiers fills missing Values/ValueMap/BitValues/BitMap from
// the class's __DERIVATION chain (nearest ancestor first; the first ancestor
// declaring any of them for the property wins). Ancestor qualifier tables
// are cached on the Service — base classes repeat across a namespace sweep.
func (s *Service) inheritEnumQualifiers(class *wmi.IWbemClassObject, props []PropertyInfo) {
	var derivation []string
	loaded := false
	for i := range props {
		p := &props[i]
		if strings.HasPrefix(p.Name, "__") ||
			len(p.Values) > 0 || len(p.ValueMap) > 0 || len(p.BitValues) > 0 || len(p.BitMap) > 0 {
			continue
		}
		if !loaded {
			derivation = classDerivation(class)
			loaded = true
		}
		for _, ancestor := range derivation {
			if q, ok := s.ancestorEnumQualifiers(ancestor)[p.Name]; ok {
				p.Values, p.ValueMap, p.BitValues, p.BitMap = q.values, q.valueMap, q.bitValues, q.bitMap
				break
			}
		}
	}
}

// classDerivation reads the __DERIVATION system property: the ancestor class
// names, nearest first.
func classDerivation(class *wmi.IWbemClassObject) []string {
	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := class.Get("__DERIVATION", 0, &value, nil, nil); err != nil {
		return nil
	}
	return AsStringSlice(decodeVariant(&value))
}

// ancestorEnumQualifiers returns the enumeration/bitmask qualifiers an
// ancestor class declares, keyed by property name (only properties declaring
// any are present). Results — including misses — are memoized on the Service.
func (s *Service) ancestorEnumQualifiers(className string) map[string]enumQuals {
	if cached, ok := s.enumQualCache[className]; ok {
		return cached
	}
	if s.enumQualCache == nil {
		s.enumQualCache = map[string]map[string]enumQuals{}
	}
	quals := map[string]enumQuals{}
	s.enumQualCache[className] = quals // even on failure: don't refetch

	name := foundation.SysAllocString(className)
	defer foundation.SysFreeString(name)
	var class *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(name, wbemFlagUseAmendedQualifiers, nil, &class, &callResult); err != nil {
		return quals
	}
	defer class.Release()

	props, err := enumerateProperties(class)
	if err != nil {
		return quals
	}
	for i := range props {
		if strings.HasPrefix(props[i].Name, "__") {
			continue
		}
		readPropertyQualifiers(class, &props[i])
		q := enumQuals{
			values: props[i].Values, valueMap: props[i].ValueMap,
			bitValues: props[i].BitValues, bitMap: props[i].BitMap,
		}
		if !q.empty() {
			quals[props[i].Name] = q
		}
	}
	return quals
}

// readPropertyQualifiers fills a property's [key], Values/ValueMap, and
// BitValues/BitMap qualifiers. System properties (__*) have no qualifier
// set; absence means zero values.
func readPropertyQualifiers(class *wmi.IWbemClassObject, p *PropertyInfo) {
	var qualifiers *wmi.IWbemQualifierSet
	if err := class.GetPropertyQualifierSet(p.Name, &qualifiers); err != nil || qualifiers == nil {
		return
	}
	defer qualifiers.Release()

	p.Key = boolQualifier(qualifiers, "key")
	p.Values = stringsQualifier(qualifiers, "Values")
	p.ValueMap = stringsQualifier(qualifiers, "ValueMap")
	p.BitValues = stringsQualifier(qualifiers, "BitValues")
	p.BitMap = stringsQualifier(qualifiers, "BitMap")
}

// boolQualifier reads one boolean qualifier; absence is false.
func boolQualifier(qualifiers *wmi.IWbemQualifierSet, name string) bool {
	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := qualifiers.Get(name, 0, &value, nil); err != nil {
		return false
	}
	b, ok := decodeVariant(&value).(bool)
	return ok && b
}

// stringsQualifier reads one string-array qualifier; absence is nil.
func stringsQualifier(qualifiers *wmi.IWbemQualifierSet, name string) []string {
	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := qualifiers.Get(name, 0, &value, nil); err != nil {
		return nil
	}
	return AsStringSlice(decodeVariant(&value))
}

// enumerateProperties walks a class object's property schema.
func enumerateProperties(class *wmi.IWbemClassObject) ([]PropertyInfo, error) {
	if err := class.BeginEnumeration(0); err != nil {
		return nil, fmt.Errorf("BeginEnumeration: %w", err)
	}
	defer class.EndEnumeration()

	var props []PropertyInfo
	for {
		var propName foundation.BSTR
		var value variant.VARIANT
		variant.VariantInit(&value)
		var cimType, flavor int32
		if err := class.Next(0, &propName, &value, &cimType, &flavor); err != nil {
			// End-of-enumeration is WBEM_S_NO_MORE_DATA — a success HRESULT
			// (nil error, nil name below); an error is a genuine failure.
			return nil, fmt.Errorf("enumerate properties: %w", err)
		}
		if propName == nil {
			variant.VariantClear(&value)
			break
		}
		props = append(props, PropertyInfo{
			Name:    win32.UTF16ToString((*uint16)(propName)),
			CIMType: cimType,
		})
		variant.VariantClear(&value)
		foundation.SysFreeString(propName)
	}
	return props, nil
}

// ClassMethods returns a class's method schemas, sorted by name. Parameters
// keep their declaration order (the [ID] qualifier); ReturnValue, which has
// no ID, sorts last among the out-parameters.
func (s *Service) ClassMethods(className string) ([]MethodInfo, error) {
	name := foundation.SysAllocString(className)
	defer foundation.SysFreeString(name)

	var class *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(name, wbemFlagUseAmendedQualifiers, nil, &class, &callResult); err != nil {
		return nil, fmt.Errorf("wmi: GetObject(%s): %w", className, err)
	}
	defer class.Release()

	methods, err := enumerateMethods(class)
	if err != nil {
		return nil, fmt.Errorf("wmi: %s: %w", className, err)
	}

	// Qualifier reads happen after EndMethodEnumeration, mirroring the
	// property/key pass.
	for i := range methods {
		methods[i].Static = methodIsStatic(class, methods[i].Name)
	}

	sort.Slice(methods, func(i, j int) bool { return methods[i].Name < methods[j].Name })
	return methods, nil
}

// enumerateMethods walks a class object's method schema.
func enumerateMethods(class *wmi.IWbemClassObject) ([]MethodInfo, error) {
	if err := class.BeginMethodEnumeration(0); err != nil {
		return nil, fmt.Errorf("BeginMethodEnumeration: %w", err)
	}
	defer class.EndMethodEnumeration()

	var methods []MethodInfo
	for {
		var methodName foundation.BSTR
		var inSignature, outSignature *wmi.IWbemClassObject
		if err := class.NextMethod(0, &methodName, &inSignature, &outSignature); err != nil {
			return nil, fmt.Errorf("enumerate methods: %w", err)
		}
		if methodName == nil {
			break
		}
		method := MethodInfo{Name: win32.UTF16ToString((*uint16)(methodName))}
		foundation.SysFreeString(methodName)

		var err error
		if method.In, err = signatureParameters(inSignature); err == nil {
			method.Out, err = signatureParameters(outSignature)
		}
		if inSignature != nil {
			inSignature.Release()
		}
		if outSignature != nil {
			outSignature.Release()
		}
		if err != nil {
			return nil, fmt.Errorf("%s: %w", method.Name, err)
		}
		methods = append(methods, method)
	}
	return methods, nil
}

// signatureParameters reads a method signature object (the __PARAMETERS
// class of one direction) into declaration order via each parameter's [ID]
// qualifier. A nil signature means no parameters in that direction.
func signatureParameters(signature *wmi.IWbemClassObject) ([]PropertyInfo, error) {
	if signature == nil {
		return nil, nil
	}
	props, err := enumerateProperties(signature)
	if err != nil {
		return nil, err
	}
	params := props[:0]
	for _, p := range props {
		if strings.HasPrefix(p.Name, "__") {
			continue // signature system properties are not parameters
		}
		params = append(params, p)
	}
	// Parameter qualifier sets carry the enumeration/bitmask qualifiers
	// directly (parameters do not inherit); Key is meaningless here and the
	// capture layer drops it.
	for i := range params {
		readPropertyQualifiers(signature, &params[i])
	}
	ids := make(map[string]int32, len(params))
	for _, p := range params {
		ids[p.Name] = parameterID(signature, p.Name)
	}
	sort.SliceStable(params, func(i, j int) bool {
		if ids[params[i].Name] != ids[params[j].Name] {
			return ids[params[i].Name] < ids[params[j].Name]
		}
		return params[i].Name < params[j].Name
	})
	return params, nil
}

// parameterID reads a signature property's [ID] qualifier; parameters
// without one (ReturnValue) sort last.
func parameterID(signature *wmi.IWbemClassObject, property string) int32 {
	var qualifiers *wmi.IWbemQualifierSet
	if err := signature.GetPropertyQualifierSet(property, &qualifiers); err != nil || qualifiers == nil {
		return math.MaxInt32
	}
	defer qualifiers.Release()

	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := qualifiers.Get("ID", 0, &value, nil); err != nil {
		return math.MaxInt32
	}
	if id, ok := decodeVariant(&value).(int64); ok {
		return int32(id)
	}
	return math.MaxInt32
}

// methodIsStatic reads a method's [static] qualifier.
func methodIsStatic(class *wmi.IWbemClassObject, method string) bool {
	var qualifiers *wmi.IWbemQualifierSet
	if err := class.GetMethodQualifierSet(method, &qualifiers); err != nil || qualifiers == nil {
		return false
	}
	defer qualifiers.Release()

	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := qualifiers.Get("static", 0, &value, nil); err != nil {
		return false
	}
	static, ok := decodeVariant(&value).(bool)
	return ok && static
}

