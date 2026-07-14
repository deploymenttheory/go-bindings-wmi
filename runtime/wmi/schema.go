//go:build windows

package wmi

import (
	"fmt"
	"sort"
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

	if err := class.BeginEnumeration(0); err != nil {
		return nil, fmt.Errorf("wmi: BeginEnumeration(%s): %w", className, err)
	}
	defer class.EndEnumeration()

	var props []PropertyInfo
	for {
		var propName foundation.BSTR
		var value variant.VARIANT
		variant.VariantInit(&value)
		var cimType, flavor int32
		if err := class.Next(0, &propName, &value, &cimType, &flavor); err != nil {
			break // WBEM_S_NO_MORE_DATA → error
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
	sort.Slice(props, func(i, j int) bool { return props[i].Name < props[j].Name })
	return props, nil
}
