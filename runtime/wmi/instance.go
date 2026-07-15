//go:build windows

package wmi

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	win32 "github.com/deploymenttheory/go-bindings-win32/bindings/runtime/win32"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/foundation"
	"github.com/deploymenttheory/go-bindings-win32/bindings/win32/system/wmi"
)

// GetInstance fetches one instance by object path (a __PATH from a query or
// a key-qualified path like `Win32_Service.Name="Spooler"`). A missing
// instance returns ErrNotFound.
func (s *Service) GetInstance(objectPath string) (Row, error) {
	path := foundation.SysAllocString(objectPath)
	defer foundation.SysFreeString(path)

	var instance *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(path, 0, nil, &instance, &callResult); err != nil {
		return nil, instanceError("GetObject", objectPath, err)
	}
	defer instance.Release()
	return decodeObject(instance)
}

// CreateInstance creates a new instance of class with the given properties
// (nil values are skipped) and returns the provider-assigned object path
// ("" when the provider does not report one).
func (s *Service) CreateInstance(class string, props map[string]any) (string, error) {
	instance, err := s.embeddedInstance(Instance(class, props))
	if err != nil {
		return "", err
	}
	defer instance.Release()

	// Semisynchronous put: the call result carries the final status and the
	// assigned path of the new instance.
	flags := wmi.WBEM_GENERIC_FLAG_TYPE(wmi.WBEM_FLAG_CREATE_ONLY) | wmi.WBEM_FLAG_RETURN_IMMEDIATELY
	var callResult *wmi.IWbemCallResult
	if err := s.services.PutInstance(instance, flags, nil, &callResult); err != nil {
		return "", fmt.Errorf("wmi: PutInstance(%s): %w", class, err)
	}
	if callResult == nil {
		return "", nil
	}
	defer callResult.Release()

	var status int32
	if err := callResult.GetCallStatus(-1 /*infinite*/, &status); err != nil {
		return "", fmt.Errorf("wmi: PutInstance(%s) status: %w", class, err)
	}
	if hr := win32.HRESULT(status); hr.Failed() {
		return "", fmt.Errorf("wmi: PutInstance(%s): %w", class, hr)
	}

	// Best-effort: not every provider reports the new path.
	var pathB foundation.BSTR
	if err := callResult.GetResultString(-1, &pathB); err != nil || pathB == nil {
		return "", nil
	}
	defer foundation.SysFreeString(pathB)
	return win32.UTF16ToString((*uint16)(pathB)), nil
}

// UpdateInstance sets properties on an existing instance. A nil value sets
// the property to NULL.
func (s *Service) UpdateInstance(objectPath string, props map[string]any) error {
	path := foundation.SysAllocString(objectPath)
	defer foundation.SysFreeString(path)

	var instance *wmi.IWbemClassObject
	var callResult *wmi.IWbemCallResult
	if err := s.services.GetObject(path, 0, nil, &instance, &callResult); err != nil {
		return instanceError("GetObject", objectPath, err)
	}
	defer instance.Release()

	properties := make([]string, 0, len(props))
	for property := range props {
		properties = append(properties, property)
	}
	sort.Strings(properties)
	for _, property := range properties {
		if err := s.putValue(instance, property, props[property]); err != nil {
			return fmt.Errorf("wmi: %s.%s: %w", objectPath, property, err)
		}
	}

	if err := s.services.PutInstance(instance,
		wmi.WBEM_GENERIC_FLAG_TYPE(wmi.WBEM_FLAG_UPDATE_ONLY), nil, nil); err != nil {
		return fmt.Errorf("wmi: PutInstance(%s): %w", objectPath, err)
	}
	return nil
}

// DeleteInstance deletes the instance at the object path. A missing
// instance returns ErrNotFound.
func (s *Service) DeleteInstance(objectPath string) error {
	path := foundation.SysAllocString(objectPath)
	defer foundation.SysFreeString(path)
	if err := s.services.DeleteInstance(path, 0, nil, nil); err != nil {
		return instanceError("DeleteInstance", objectPath, err)
	}
	return nil
}

// AssociatorFilter narrows an Associators traversal — each non-empty field
// becomes the matching ASSOCIATORS OF clause.
type AssociatorFilter struct {
	// AssocClass: the association must be (derived from) this class.
	AssocClass string
	// ResultClass: returned endpoints must be (derived from) this class.
	ResultClass string
	// ResultRole: returned endpoints must play this role in the association.
	ResultRole string
	// Role: the source object must play this role in the association.
	Role string
}

// Associators returns the instances associated with the object at the path
// (WQL "ASSOCIATORS OF") — e.g. the partitions of a disk, the services of a
// process.
func (s *Service) Associators(objectPath string, filter AssociatorFilter) ([]Row, error) {
	query := "ASSOCIATORS OF {" + objectPath + "}"
	var clauses []string
	if filter.AssocClass != "" {
		clauses = append(clauses, "AssocClass = "+filter.AssocClass)
	}
	if filter.ResultClass != "" {
		clauses = append(clauses, "ResultClass = "+filter.ResultClass)
	}
	if filter.ResultRole != "" {
		clauses = append(clauses, "ResultRole = "+filter.ResultRole)
	}
	if filter.Role != "" {
		clauses = append(clauses, "Role = "+filter.Role)
	}
	if len(clauses) > 0 {
		// ASSOCIATORS OF clauses are space-separated, not AND-joined.
		query += " WHERE " + strings.Join(clauses, " ")
	}
	return s.Query(query)
}

// References returns the association instances that reference the object at
// the path (WQL "REFERENCES OF"); resultClass optionally narrows the
// association class.
func (s *Service) References(objectPath, resultClass string) ([]Row, error) {
	query := "REFERENCES OF {" + objectPath + "}"
	if resultClass != "" {
		query += " WHERE ResultClass = " + resultClass
	}
	return s.Query(query)
}

// instanceError wraps a WMI failure, mapping WBEM_E_NOT_FOUND to
// ErrNotFound so callers can errors.Is it.
func instanceError(op, objectPath string, err error) error {
	var hr win32.HRESULT
	if errors.As(err, &hr) && hr == win32.HRESULT(wmi.WBEM_E_NOT_FOUND) {
		return fmt.Errorf("%w: %s", ErrNotFound, objectPath)
	}
	return fmt.Errorf("wmi: %s(%s): %w", op, objectPath, err)
}
