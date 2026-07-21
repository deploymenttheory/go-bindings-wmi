//go:build windows

package wmi

import (
	"fmt"
	"sort"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// ObjectText renders a Row as MOF instance text
// (IWbemClassObject::GetObjectText). Some providers — notably Hyper-V's
// root\virtualization\v2 — declare method parameters as strings carrying
// embedded-instance MOF text rather than object references: build the
// instance with Instance (or start from a queried Row) and pass the text as
// the parameter value.
func (s *Service) ObjectText(row Row) (string, error) {
	instance, err := s.embeddedInstance(row)
	if err != nil {
		return "", err
	}
	defer instance.Release()
	return objectText(instance)
}

// ObjectTextOfPath fetches the instance at objectPath, applies the overrides
// on the in-memory copy (nil values are skipped; nothing is written back to
// the repository), and renders the result as MOF instance text. This is the
// read-modify-encode step providers like Hyper-V expect for their
// Modify*Settings methods.
func (s *Service) ObjectTextOfPath(objectPath string, overrides map[string]any) (string, error) {
	path := foundation.SysAllocString(objectPath)
	defer foundation.SysFreeString(path)

	var instance *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(path, 0, nil, &instance, &callResult); err != nil {
		return "", instanceError("GetObject", objectPath, err)
	}
	defer instance.Release()

	// Deterministic put order keeps failures reproducible.
	properties := make([]string, 0, len(overrides))
	for property := range overrides {
		if overrides[property] != nil {
			properties = append(properties, property)
		}
	}
	sort.Strings(properties)
	for _, property := range properties {
		if err := s.putValue(instance, property, overrides[property]); err != nil {
			return "", fmt.Errorf("wmi: %s.%s: %w", objectPath, property, err)
		}
	}
	return objectText(instance)
}

// objectText extracts an object's MOF text.
func objectText(instance *wmi.IWbemClassObject) (string, error) {
	var text foundation.BSTR
	if err := instance.GetObjectText(0, &text); err != nil {
		return "", fmt.Errorf("wmi: GetObjectText: %w", err)
	}
	defer foundation.SysFreeString(text)
	return win32.UTF16ToString((*uint16)(text)), nil
}
