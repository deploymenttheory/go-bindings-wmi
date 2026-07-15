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
		props[i].Key = propertyIsKey(class, props[i].Name)
	}

	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	return props, nil
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

// propertyIsKey reads a property's [key] qualifier. System properties (__*)
// have no qualifier set; absence of the qualifier means not a key.
func propertyIsKey(class *wmi.IWbemClassObject, property string) bool {
	var qualifiers *wmi.IWbemQualifierSet
	if err := class.GetPropertyQualifierSet(property, &qualifiers); err != nil || qualifiers == nil {
		return false
	}
	defer qualifiers.Release()

	var value variant.VARIANT
	variant.VariantInit(&value)
	defer variant.VariantClear(&value)
	if err := qualifiers.Get("key", 0, &value, nil); err != nil {
		return false
	}
	isKey, ok := decodeVariant(&value).(bool)
	return ok && isKey
}
