//go:build windows

// Package wmi is the hand-written runtime for the generated WMI/CIM bindings.
// It connects to the CIM repository through go-bindings-win32's generated COM
// interfaces, runs WQL queries, and decodes result VARIANTs into Go values.
//
// Built entirely on go-bindings-win32 (IWbemLocator/IWbemServices, VARIANT,
// BSTR, SAFEARRAY) — this module adds no third-party dependency.
package wmi

import (
	"context"
	"fmt"
	"iter"
	"math"
	"runtime"
	"strings"
	"syscall"
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
	// enumQualCache memoizes ancestor-class enumeration qualifiers during a
	// schema capture (ClassProperties walks __DERIVATION; CIM base classes
	// repeat across a namespace). Lazily initialized; keyed by class name.
	enumQualCache map[string]map[string]enumQuals
}

// ConnectOptions controls ConnectWith. The zero value connects locally as
// the current security context.
type ConnectOptions struct {
	// Host is a remote machine (DNS name or IP); empty connects locally.
	Host string
	// User and Password authenticate against a remote host (User may be
	// `DOMAIN\user`). Leave empty to use the current token. WMI rejects
	// explicit credentials on local connections.
	User, Password string
	// Locale (e.g. "MS_409") and Authority (e.g. "ntlmdomain:DOMAIN") pass
	// through to ConnectServer; usually empty.
	Locale, Authority string
}

// Connect initializes COM on the calling goroutine's thread and connects to
// the given namespace (e.g. `root\cimv2`). Call Close when done.
//
// Thread affinity: Connect locks the calling goroutine to its OS thread
// (COM init is per-thread) and Close unlocks it, so a Service must be
// connected, used, and closed on the same goroutine. For a long-lived
// Service shared across goroutines, dedicate one goroutine to WMI and feed
// it work over a channel.
func Connect(namespace string) (*Service, error) {
	return ConnectWith(namespace, ConnectOptions{})
}

// ConnectWith is Connect with options — remote host, credentials, locale.
// The same thread-affinity contract applies.
func ConnectWith(namespace string, opts ConnectOptions) (*Service, error) {
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

	path := namespace
	if opts.Host != "" {
		path = `\\` + opts.Host + `\` + namespace
	}
	ns := foundation.SysAllocString(path)
	defer foundation.SysFreeString(ns)
	user := optionalBSTR(opts.User)
	defer foundation.SysFreeString(user)
	password := optionalBSTR(opts.Password)
	defer foundation.SysFreeString(password)
	locale := optionalBSTR(opts.Locale)
	defer foundation.SysFreeString(locale)
	authority := optionalBSTR(opts.Authority)
	defer foundation.SysFreeString(authority)

	var services *wmi.IWbemServices
	if err := locator.ConnectServer(ns, user, password, locale, 0, authority, nil, &services); err != nil {
		locator.Release()
		com.CoUninitialize()
		runtime.UnlockOSThread()
		return nil, fmt.Errorf("wmi: ConnectServer(%s): %w", path, err)
	}

	// The proxy blanket must carry the same credentials as ConnectServer, or
	// remote calls run as the caller's token and fail with access-denied.
	// Build a COAUTHIDENTITY for explicit credentials; local connections use
	// the default (nil auth identity).
	authIdentity, freeIdentity := authIdentityFor(opts)
	defer freeIdentity()
	if err := com.CoSetProxyBlanket((*com.IUnknown)(unsafe.Pointer(services)),
		10 /*RPC_C_AUTHN_WINNT*/, 0 /*RPC_C_AUTHZ_NONE*/, "",
		rpcCAuthnLevelDefault, rpcCImpLevelImpersonate, authIdentity, eoacNone); err != nil {
		// Non-fatal for local queries.
	}
	return &Service{locator: locator, services: services}, nil
}

// authIdentityFor builds a COAUTHIDENTITY for the proxy blanket when explicit
// credentials are set, returning a pointer (nil for the default identity)
// and a cleanup that keeps the UTF-16 buffers alive until the blanket call
// returns.
func authIdentityFor(opts ConnectOptions) (unsafe.Pointer, func()) {
	if opts.User == "" && opts.Password == "" {
		return nil, func() {}
	}
	user, domain := opts.User, ""
	if slash := strings.LastIndexAny(user, `\/`); slash >= 0 {
		domain, user = user[:slash], user[slash+1:]
	}
	// UTF16FromString appends a NUL; the length fields below exclude it. NUL
	// bytes inside credentials are rejected (they cannot occur in valid ones).
	userUTF16, _ := syscall.UTF16FromString(user)
	domainUTF16, _ := syscall.UTF16FromString(domain)
	passwordUTF16, _ := syscall.UTF16FromString(opts.Password)

	identity := &com.COAUTHIDENTITY{
		User:           &userUTF16[0],
		UserLength:     uint32(len(userUTF16) - 1), // exclude the NUL terminator
		Domain:         &domainUTF16[0],
		DomainLength:   uint32(len(domainUTF16) - 1),
		Password:       &passwordUTF16[0],
		PasswordLength: uint32(len(passwordUTF16) - 1),
		Flags:          2, // SEC_WINNT_AUTH_IDENTITY_UNICODE
	}
	// runtime.KeepAlive in the cleanup pins the backing slices (identity only
	// holds element pointers) across the CoSetProxyBlanket call.
	return unsafe.Pointer(identity), func() {
		runtime.KeepAlive(userUTF16)
		runtime.KeepAlive(domainUTF16)
		runtime.KeepAlive(passwordUTF16)
		runtime.KeepAlive(identity)
	}
}

// optionalBSTR allocates a BSTR for non-empty strings; empty stays nil
// (WMI's "use the default" convention). SysFreeString(nil) is a no-op.
func optionalBSTR(s string) foundation.BSTR {
	if s == "" {
		return nil
	}
	return foundation.SysAllocString(s)
}

// Close releases the session and uninitializes COM on this thread. It must
// run on the goroutine that called Connect (see Connect's thread-affinity
// contract).
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

// Query runs a WQL query and returns every instance's decoded properties.
func (s *Service) Query(wql string) ([]Row, error) {
	enum, err := s.execQuery(wql)
	if err != nil {
		return nil, err
	}
	defer enum.Release()

	var rows []Row
	for {
		obj, status, err := nextObject(enum, -1 /*WBEM_INFINITE*/)
		if err != nil {
			return nil, err
		}
		if status == enumDone {
			return rows, nil
		}
		row, derr := decodeObject(obj)
		obj.Release()
		if derr != nil {
			return nil, derr
		}
		rows = append(rows, row)
	}
}

// QuerySeq runs a WQL query and streams each decoded row, releasing the
// enumerator when the caller stops early — use it for large result sets. A
// failure yields (nil, err) once and ends the sequence. Consume it on the
// connecting goroutine (see Connect's thread-affinity contract).
func (s *Service) QuerySeq(wql string) iter.Seq2[Row, error] {
	return func(yield func(Row, error) bool) {
		enum, err := s.execQuery(wql)
		if err != nil {
			yield(nil, err)
			return
		}
		defer enum.Release()
		for {
			obj, status, err := nextObject(enum, -1 /*WBEM_INFINITE*/)
			if err != nil {
				yield(nil, err)
				return
			}
			if status == enumDone {
				return
			}
			row, derr := decodeObject(obj)
			obj.Release()
			if !yield(row, derr) || derr != nil {
				return
			}
		}
	}
}

// QueryContext is Query with cancellation: between short enumerator waits it
// checks ctx and returns its error once canceled. WMI has no server-side
// cancel for a semisynchronous query, so cancellation abandons the
// enumerator rather than stopping the provider.
func (s *Service) QueryContext(ctx context.Context, wql string) ([]Row, error) {
	enum, err := s.execQuery(wql)
	if err != nil {
		return nil, err
	}
	defer enum.Release()

	const pollMs = 250
	var rows []Row
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		obj, status, err := nextObject(enum, pollMs)
		if err != nil {
			return nil, err
		}
		switch status {
		case enumDone:
			return rows, nil
		case enumTimedOut:
			continue
		}
		row, derr := decodeObject(obj)
		obj.Release()
		if derr != nil {
			return nil, derr
		}
		rows = append(rows, row)
	}
}

// execQuery issues the WQL and returns the (forward-only, semisynchronous)
// enumerator.
func (s *Service) execQuery(wql string) (*wmi.IEnumWbemClassObject, error) {
	lang := foundation.SysAllocString("WQL")
	defer foundation.SysFreeString(lang)
	query := foundation.SysAllocString(wql)
	defer foundation.SysFreeString(query)

	var enum *wmi.IEnumWbemClassObject
	if err := s.services.ExecQuery(lang, query,
		wbemFlagForwardOnly|wbemFlagReturnImmediately, nil, &enum); err != nil {
		return nil, fmt.Errorf("wmi: ExecQuery(%q): %w", wql, err)
	}
	return enum, nil
}

// nextObject steps an enumerator: one object, end-of-enumeration, or (when
// the timeout elapses first) enumTimedOut. The informational-success shape
// of Next carries all three outcomes.
const (
	objectReady = iota
	enumDone
	enumTimedOut
)

func nextObject(enum *wmi.IEnumWbemClassObject, timeoutMs int32) (*wmi.IWbemClassObject, int, error) {
	objects := make([]*wmi.IWbemClassObject, 1)
	var returned uint32
	hr, err := enum.Next(timeoutMs, objects, &returned)
	if err != nil {
		return nil, enumDone, fmt.Errorf("wmi: enumerate: %w", err)
	}
	if hr == win32.HRESULT(wmi.WBEM_S_TIMEDOUT) {
		return nil, enumTimedOut, nil
	}
	if returned == 0 || hr == win32.HRESULT(wmi.WBEM_S_FALSE) {
		return nil, enumDone, nil
	}
	return objects[0], objectReady, nil
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
			// End-of-enumeration is WBEM_S_NO_MORE_DATA — a *success* HRESULT
			// (surfaced as a nil error and a nil name below); an error here is
			// a genuine failure.
			return nil, fmt.Errorf("wmi: enumerate properties: %w", err)
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

// decodeVariant converts a VARIANT to a Go value: scalars widen to string,
// int64, uint64, bool, or float64; SAFEARRAYs decode to typed slices of those
// widened elements; embedded CIM objects (VT_UNKNOWN/VT_DISPATCH) decode to
// a nested Row. Unsupported types return nil.
func decodeVariant(v *variant.VARIANT) any {
	s := (*variantScalar)(unsafe.Pointer(&v.Anonymous))
	if s.Vt&uint16(variant.VT_ARRAY) != 0 {
		p := (*variantPtr)(unsafe.Pointer(&v.Anonymous))
		return decodeSafeArray((*com.SAFEARRAY)(p.P))
	}
	switch s.Vt {
	case uint16(variant.VT_EMPTY), uint16(variant.VT_NULL):
		return nil
	case uint16(variant.VT_BSTR):
		p := (*variantPtr)(unsafe.Pointer(&v.Anonymous))
		return win32.UTF16ToString((*uint16)(p.P))
	case uint16(variant.VT_UNKNOWN), uint16(variant.VT_DISPATCH):
		p := (*variantPtr)(unsafe.Pointer(&v.Anonymous))
		return decodeEmbedded(p.P)
	case uint16(variant.VT_BOOL):
		return int16(s.U64) != 0
	case uint16(variant.VT_I1):
		return int64(int8(s.U64))
	case uint16(variant.VT_I2):
		return int64(int16(s.U64))
	case uint16(variant.VT_I4), uint16(variant.VT_INT):
		return int64(int32(s.U64))
	case uint16(variant.VT_UI1):
		return uint64(uint8(s.U64))
	case uint16(variant.VT_UI2):
		return uint64(uint16(s.U64))
	case uint16(variant.VT_UI4), uint16(variant.VT_UINT):
		return uint64(uint32(s.U64))
	case uint16(variant.VT_I8):
		return int64(s.U64)
	case uint16(variant.VT_UI8):
		return s.U64
	case uint16(variant.VT_R4):
		return float64(math.Float32frombits(uint32(s.U64)))
	case uint16(variant.VT_R8):
		return math.Float64frombits(s.U64)
	}
	return nil
}

// decodeEmbedded decodes an embedded CIM object (WMI hands them out as
// IWbemClassObject behind IUnknown/IDispatch) into a Row without taking
// ownership — the VARIANT (or locked SAFEARRAY) keeps the reference.
func decodeEmbedded(p unsafe.Pointer) any {
	if p == nil {
		return nil
	}
	row, err := decodeObject((*wmi.IWbemClassObject)(p))
	if err != nil {
		return nil
	}
	return row
}
