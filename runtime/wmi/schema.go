//go:build windows

package wmi

import (
	"fmt"
	"sort"

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
	cimFlagArray                 = 0x2000
)

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
