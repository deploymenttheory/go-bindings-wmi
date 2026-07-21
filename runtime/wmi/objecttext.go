//go:build windows

package wmi

import (
	"fmt"
	"sort"
	"unsafe"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// CLSID_WbemObjectTextSrc (not carried in the winmd, so declared here).
// {8D1C559D-84F0-4BB3-A7D5-56A7435A9BA6}, "Microsoft WBEM WMI Object
// Factory" in fastprox.dll.
var clsidWbemObjectTextSrc = win32.GUID{
	Data1: 0x8d1c559d, Data2: 0x84f0, Data3: 0x4bb3,
	Data4: [8]byte{0xa7, 0xd5, 0x56, 0xa7, 0x43, 0x5a, 0x9b, 0xa6},
}

// ObjectText renders a Row as CIM DTD 2.0 XML embedded-instance text
// (IWbemObjectTextSrc::GetText). Some providers — notably Hyper-V's
// root\virtualization\v2 — declare method parameters as strings carrying
// embedded instances: build the instance with Instance (or start from a
// queried Row) and pass the text as the parameter value.
//
// The XML form is the one such providers accept; Hyper-V's VMMS rejects MOF
// text (IWbemClassObject::GetObjectText) with "invalid parameter" (32773).
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
// the repository), and renders the result as CIM DTD 2.0 XML embedded-
// instance text. This is the read-modify-encode step providers like Hyper-V
// expect for their Modify*Settings methods.
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

// objectText serializes an object as CIM DTD 2.0 XML via a transient
// WbemObjectTextSrc (an in-proc factory; creation is cheap).
func objectText(instance *wmi.IWbemClassObject) (string, error) {
	var unk *win32.IUnknown
	if err := com.CoCreateInstance(&clsidWbemObjectTextSrc, nil, clsctxInprocServer,
		&wmi.IID_IWbemObjectTextSrc, &unk); err != nil {
		return "", fmt.Errorf("wmi: CoCreateInstance(WbemObjectTextSrc): %w", err)
	}
	src := (*wmi.IWbemObjectTextSrc)(unsafe.Pointer(unk))
	defer src.Release()

	var text foundation.BSTR
	if err := src.GetText(0, instance, uint32(wmi.WMI_OBJ_TEXT_CIM_DTD_2_0), nil, &text); err != nil {
		return "", fmt.Errorf("wmi: IWbemObjectTextSrc.GetText: %w", err)
	}
	defer foundation.SysFreeString(text)
	return win32.UTF16ToString((*uint16)(text)), nil
}
