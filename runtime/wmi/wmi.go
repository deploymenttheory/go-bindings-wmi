//go:build windows

// Package wmi is the hand-written runtime for the generated WMI/CIM bindings.
// It connects to the CIM repository through go-bindings-win32's generated COM
// interfaces, runs WQL queries, and decodes result VARIANTs into Go values.
//
// Built entirely on go-bindings-win32 (IWbemLocator/IWbemServices, VARIANT,
// BSTR, SAFEARRAY) — this module adds no third-party dependency.
package wmi

import (
	"fmt"
	"math"
	"runtime"
	"unsafe"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/security"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// CLSID_WbemLocator is the class id of the WMI locator (not carried in the
// winmd, so declared here). {4590F811-1D3A-11D0-891F-00AA004B2E24}.
var clsidWbemLocator = win32.GUID{
	Data1: 0x4590f811, Data2: 0x1d3a, Data3: 0x11d0,
	Data4: [8]byte{0x89, 0x1f, 0x00, 0xaa, 0x00, 0x4b, 0x2e, 0x24},
}

const (
	rpcCAuthnLevelDefault    = 0
	rpcCImpLevelImpersonate  = 3
	eoacNone                 = 0
	clsctxInprocServer       = 0x1
	wbemFlagForwardOnly      = 0x20
	wbemFlagReturnImmediately = 0x10
)

// VARIANT union overlays (amd64/arm64 LLP64): the type discriminator sits at
// offset 0, the value word at offset 8. Two views share offset 8 so pointer
// values are read as unsafe.Pointer directly (never via uintptr, which vet
// rightly flags).
type variantScalar struct {
	Vt  uint16
	_   [6]byte
	U64 uint64
	_   uint64
}

type variantPtr struct {
	Vt uint16
	_  [6]byte
	P  unsafe.Pointer
	_  uint64
}

// Service is a connected WMI namespace session.
type Service struct {
	locator  *wmi.IWbemLocator
	services *wmi.IWbemServices
}

// Connect initializes COM on the calling goroutine's thread and connects to
// the given namespace (e.g. `root\cimv2`). Call Close when done. The caller
// should runtime.LockOSThread for the duration if issuing many calls; Connect
// locks internally for the CoInitializeEx/blanket setup.
func Connect(namespace string) (*Service, error) {
	runtime.LockOSThread()
	if _, err := com.CoInitializeEx(0x0); err != nil { // COINIT_MULTITHREADED
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("wmi: CoInitializeEx: %w", err)
	}
	if err := com.CoInitializeSecurity(security.PSECURITY_DESCRIPTOR(0), -1, nil,
		rpcCAuthnLevelDefault, rpcCImpLevelImpersonate, nil, eoacNone); err != nil {
		// RPC_E_TOO_LATE (already set) is fine; other failures are not fatal
		// to a single-process client, so continue.
	}

	var unk *win32.IUnknown
	if err := com.CoCreateInstance(&clsidWbemLocator, nil, clsctxInprocServer,
		&wmi.IID_IWbemLocator, &unk); err != nil {
		com.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("wmi: CoCreateInstance(WbemLocator): %w", err)
	}
	locator := (*wmi.IWbemLocator)(unsafe.Pointer(unk))

	ns := foundation.SysAllocString(namespace)
	defer foundation.SysFreeString(ns)
	var services *wmi.IWbemServices
	if err := locator.ConnectServer(ns, nil, nil, nil, 0, nil, nil, &services); err != nil {
		locator.Release()
		com.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("wmi: ConnectServer(%s): %w", namespace, err)
	}
	if err := com.CoSetProxyBlanket((*com.IUnknown)(unsafe.Pointer(services)),
		10 /*RPC_C_AUTHN_WINNT*/, 0 /*RPC_C_AUTHZ_NONE*/, "",
		rpcCAuthnLevelDefault, rpcCImpLevelImpersonate, nil, eoacNone); err != nil {
		// Non-fatal for local queries.
	}
	return &Service{locator: locator, services: services}, nil
}

// Close releases the session and uninitializes COM on this thread.
func (s *Service) Close() {
	if s.services != nil {
		s.services.Release()
	}
	if s.locator != nil {
		s.locator.Release()
	}
	com.CoUninitialize()
	runtime.UnlockOSThread()
}

// Row is one WMI instance: property name → decoded Go value (string, int64,
// uint64, bool, float64, or nil).
type Row map[string]any

// Query runs a WQL query and returns every instance's decoded properties.
func (s *Service) Query(wql string) ([]Row, error) {
	lang := foundation.SysAllocString("WQL")
	defer foundation.SysFreeString(lang)
	query := foundation.SysAllocString(wql)
	defer foundation.SysFreeString(query)

	var enum *wmi.IEnumWbemClassObject
	if err := s.services.ExecQuery(lang, query,
		wbemFlagForwardOnly|wbemFlagReturnImmediately, nil, &enum); err != nil {
		return nil, fmt.Errorf("wmi: ExecQuery(%q): %w", wql, err)
	}
	defer enum.Release()

	var rows []Row
	for {
		var obj *wmi.IWbemClassObject
		var returned uint32
		objects := []*wmi.IWbemClassObject{obj}
		hr, err := enum.Next(-1 /*WBEM_INFINITE*/, objects, &returned)
		if err != nil {
			return nil, fmt.Errorf("wmi: enumerate: %w", err)
		}
		if returned == 0 || hr == win32.HRESULT(wmi.WBEM_S_FALSE) {
			break
		}
		row, derr := decodeObject(objects[0])
		objects[0].Release()
		if derr != nil {
			return nil, derr
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// decodeObject reads every property of a class object into a Row.
func decodeObject(obj *wmi.IWbemClassObject) (Row, error) {
	if err := obj.BeginEnumeration(0); err != nil {
		return nil, fmt.Errorf("wmi: BeginEnumeration: %w", err)
	}
	defer obj.EndEnumeration()

	row := Row{}
	for {
		var name foundation.BSTR
		var value variant.VARIANT
		variant.VariantInit(&value)
		var cimType, flavor int32
		if err := obj.Next(0, &name, &value, &cimType, &flavor); err != nil {
			// WBEM_S_NO_MORE_DATA surfaces as a failed HRESULT → error; stop.
			break
		}
		if name == nil {
			variant.VariantClear(&value)
			break
		}
		propName := win32.UTF16ToString((*uint16)(name))
		row[propName] = decodeVariant(&value)
		variant.VariantClear(&value)
		foundation.SysFreeString(name)
	}
	return row, nil
}

// decodeVariant converts a VARIANT to a Go value for the common CIM scalar
// types. Unsupported/array types return nil.
func decodeVariant(v *variant.VARIANT) any {
	s := (*variantScalar)(unsafe.Pointer(&v.Anonymous))
	switch s.Vt {
	case uint16(variant.VT_EMPTY), uint16(variant.VT_NULL):
		return nil
	case uint16(variant.VT_BSTR):
		p := (*variantPtr)(unsafe.Pointer(&v.Anonymous))
		return win32.UTF16ToString((*uint16)(p.P))
	case uint16(variant.VT_BOOL):
		return int16(s.U64) != 0
	case uint16(variant.VT_I4):
		return int64(int32(s.U64))
	case uint16(variant.VT_UI4):
		return uint64(uint32(s.U64))
	case uint16(variant.VT_I8):
		return int64(s.U64)
	case uint16(variant.VT_UI8):
		return s.U64
	case uint16(variant.VT_R8):
		return math.Float64frombits(s.U64)
	}
	return nil
}
