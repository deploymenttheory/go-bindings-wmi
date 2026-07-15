//go:build windows

package wmi

import (
	"fmt"
	"sort"
	"strings"

	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
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
		value := in[param]
		if value == nil {
			continue
		}
		var v variant.VARIANT
		variant.VariantInit(&v)
		err := encodeVariant(value, &v)
		if err == nil {
			err = instance.Put(param, 0, &v, 0)
		}
		variant.VariantClear(&v)
		if err != nil {
			instance.Release()
			return nil, fmt.Errorf("wmi: %s.%s parameter %s: %w", className, method, param, err)
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
