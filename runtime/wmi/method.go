//go:build windows

package wmi

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"unsafe"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/com"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/ole"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/variant"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// ExecMethod invokes a WMI method and returns its out-parameters (including
// ReturnValue) as a Row. objectPath is the class name for static methods or
// an instance path for instance methods — instance paths come back from
// queries as the __PATH system property. Values in `in` are encoded per
// WMI's put conventions (see encodeVariant); nil values are skipped, which
// leaves the parameter NULL so the provider applies its default.
func (s *Service) ExecMethod(objectPath, method string, in map[string]any) (Row, error) {
	var inInstance *wmi.IWbemClassObject
	if len(in) > 0 {
		instance, err := s.spawnInParameters(classOfPath(objectPath), method, in)
		if err != nil {
			return nil, err
		}
		defer instance.Release()
		inInstance = instance
	}

	path := foundation.SysAllocString(objectPath)
	defer foundation.SysFreeString(path)
	name := foundation.SysAllocString(method)
	defer foundation.SysFreeString(name)

	var out *wmi.IWbemClassObject
	if err := s.services.ExecMethod(path, name, 0, nil, inInstance, &out, nil); err != nil {
		return nil, fmt.Errorf("wmi: ExecMethod(%s.%s): %w", objectPath, method, err)
	}
	if out == nil {
		return Row{}, nil
	}
	defer out.Release()
	return decodeObject(out)
}

// ExecMethodContext is ExecMethod with cancellation: the call runs
// semisynchronously and the completion poll checks ctx between short waits.
// WMI cannot abort a provider mid-call — cancellation abandons the call;
// the provider may finish it anyway.
func (s *Service) ExecMethodContext(ctx context.Context, objectPath, method string, in map[string]any) (Row, error) {
	var inInstance *wmi.IWbemClassObject
	if len(in) > 0 {
		instance, err := s.spawnInParameters(classOfPath(objectPath), method, in)
		if err != nil {
			return nil, err
		}
		defer instance.Release()
		inInstance = instance
	}

	path := foundation.SysAllocString(objectPath)
	defer foundation.SysFreeString(path)
	name := foundation.SysAllocString(method)
	defer foundation.SysFreeString(name)

	var callResult *wmi.IWbemCallResult
	if err := s.services.ExecMethod(path, name, wmi.WBEM_FLAG_RETURN_IMMEDIATELY,
		nil, inInstance, nil, &callResult); err != nil {
		return nil, fmt.Errorf("wmi: ExecMethod(%s.%s): %w", objectPath, method, err)
	}
	if callResult == nil {
		return Row{}, nil
	}
	defer callResult.Release()

	const pollMs = 250
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// GetCallStatus returns WBEM_S_TIMEDOUT (a success HRESULT the
		// binding maps to a nil error) without touching plStatus while the
		// call is in flight — the sentinel detects that.
		status := int32(math.MinInt32)
		if err := callResult.GetCallStatus(pollMs, &status); err != nil {
			return nil, fmt.Errorf("wmi: ExecMethod(%s.%s) status: %w", objectPath, method, err)
		}
		if status == int32(math.MinInt32) {
			continue // timed out; poll again
		}
		if hr := win32.HRESULT(status); hr.Failed() {
			return nil, fmt.Errorf("wmi: ExecMethod(%s.%s): %w", objectPath, method, hr)
		}
		break
	}

	var out *wmi.IWbemClassObject
	if err := callResult.GetResultObject(-1, &out); err != nil || out == nil {
		return Row{}, nil // void methods produce no out-parameters object
	}
	defer out.Release()
	return decodeObject(out)
}

// spawnInParameters builds the method's in-parameters instance: the class's
// method signature spawns an instance and each provided value is put on it.
func (s *Service) spawnInParameters(className, method string, in map[string]any) (*wmi.IWbemClassObject, error) {
	name := foundation.SysAllocString(className)
	defer foundation.SysFreeString(name)

	var class *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(name, 0, nil, &class, &callResult); err != nil {
		return nil, fmt.Errorf("wmi: GetObject(%s): %w", className, err)
	}
	defer class.Release()

	var signature *wmi.IWbemClassObject
	if err := class.GetMethod(method, 0, &signature, nil); err != nil {
		return nil, fmt.Errorf("wmi: GetMethod(%s.%s): %w", className, method, err)
	}
	if signature == nil {
		return nil, fmt.Errorf("wmi: %s.%s accepts no in-parameters", className, method)
	}
	defer signature.Release()

	var instance *wmi.IWbemClassObject
	if err := signature.SpawnInstance(0, &instance); err != nil {
		return nil, fmt.Errorf("wmi: SpawnInstance(%s.%s): %w", className, method, err)
	}

	// Deterministic put order keeps failures reproducible.
	params := make([]string, 0, len(in))
	for param := range in {
		params = append(params, param)
	}
	sort.Strings(params)
	for _, param := range params {
		if in[param] == nil {
			continue
		}
		if err := s.putValue(instance, param, in[param]); err != nil {
			instance.Release()
			return nil, fmt.Errorf("wmi: %s.%s parameter %s: %w", className, method, param, err)
		}
	}
	return instance, nil
}

// putValue encodes one value onto an instance property. Rows (and plain
// maps) become spawned embedded instances — their __CLASS key names the
// class; everything else goes through encodeVariant.
func (s *Service) putValue(target *wmi.IWbemClassObject, property string, value any) error {
	var v variant.VARIANT
	variant.VariantInit(&v)
	defer variant.VariantClear(&v)

	switch t := value.(type) {
	case Row:
		embedded, err := s.embeddedInstance(t)
		if err != nil {
			return err
		}
		// The VARIANT owns our reference now; VariantClear releases it.
		pointer := (*variantPtr)(unsafe.Pointer(&v.Anonymous))
		pointer.Vt = uint16(variant.VT_UNKNOWN)
		pointer.P = unsafe.Pointer(embedded)
	case map[string]any:
		return s.putValue(target, property, Row(t))
	case []Row:
		if err := s.encodeRowArray(t, &v); err != nil {
			return err
		}
	default:
		if err := encodeVariant(value, &v); err != nil {
			return err
		}
	}
	if err := target.Put(property, 0, &v, 0); err != nil {
		return fmt.Errorf("Put: %w", err)
	}
	return nil
}

// encodeRowArray builds a SAFEARRAY of embedded instances (VT_UNKNOWN) from
// rows. SafeArrayPutElement AddRefs each interface, so our references are
// released as we go; SafeArrayDestroy (via VariantClear) releases the
// array's.
func (s *Service) encodeRowArray(rows []Row, v *variant.VARIANT) error {
	bound := com.SAFEARRAYBOUND{CElements: uint32(len(rows))}
	psa := ole.SafeArrayCreate(variant.VT_UNKNOWN, 1, &bound)
	if psa == nil {
		return fmt.Errorf("wmi: SafeArrayCreate(VT_UNKNOWN) failed for %d rows", len(rows))
	}
	for i, row := range rows {
		embedded, err := s.embeddedInstance(row)
		if err != nil {
			_ = ole.SafeArrayDestroy(psa)
			return err
		}
		index := int32(i)
		err = ole.SafeArrayPutElement(psa, &index, unsafe.Pointer(embedded))
		embedded.Release()
		if err != nil {
			_ = ole.SafeArrayDestroy(psa)
			return fmt.Errorf("wmi: SafeArrayPutElement(%d): %w", i, err)
		}
	}
	pointer := (*variantPtr)(unsafe.Pointer(&v.Anonymous))
	pointer.Vt = uint16(variant.VT_ARRAY | variant.VT_UNKNOWN)
	pointer.P = unsafe.Pointer(psa)
	return nil
}

// embeddedInstance spawns an instance of an embedded class described by a
// Row: __CLASS names the class (wmi.Instance sets it; queried rows carry
// it), the remaining non-system keys become properties — recursively, so
// embedded objects can nest.
func (s *Service) embeddedInstance(row Row) (*wmi.IWbemClassObject, error) {
	class, _ := row["__CLASS"].(string)
	if class == "" {
		return nil, fmt.Errorf("wmi: embedded object row has no __CLASS (build it with wmi.Instance)")
	}

	name := foundation.SysAllocString(class)
	defer foundation.SysFreeString(name)
	var classObj *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(name, 0, nil, &classObj, &callResult); err != nil {
		return nil, fmt.Errorf("wmi: GetObject(%s): %w", class, err)
	}
	defer classObj.Release()

	var instance *wmi.IWbemClassObject
	if err := classObj.SpawnInstance(0, &instance); err != nil {
		return nil, fmt.Errorf("wmi: SpawnInstance(%s): %w", class, err)
	}

	properties := make([]string, 0, len(row))
	for property := range row {
		if row[property] == nil || strings.HasPrefix(property, "__") {
			continue // system properties travel implicitly
		}
		properties = append(properties, property)
	}
	sort.Strings(properties)
	for _, property := range properties {
		if err := s.putValue(instance, property, row[property]); err != nil {
			instance.Release()
			return nil, fmt.Errorf("wmi: %s.%s: %w", class, property, err)
		}
	}
	return instance, nil
}

// classOfPath extracts the class from a WMI object path, e.g.
// `\\HOST\root\cimv2:Win32_Process.Handle="42"` → `Win32_Process`. Key
// values are cut first — they may contain ':' or '.'.
func classOfPath(path string) string {
	if q := strings.IndexByte(path, '"'); q >= 0 {
		path = path[:q]
	}
	if i := strings.LastIndexByte(path, ':'); i >= 0 {
		path = path[i+1:]
	}
	if i := strings.IndexByte(path, '.'); i >= 0 {
		path = path[:i]
	}
	return path
}
