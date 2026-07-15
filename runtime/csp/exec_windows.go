//go:build windows

package csp

import (
	"errors"
	"fmt"
	"strings"

	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// Namespace is the MDM WMI bridge namespace that executes CSP policies.
const Namespace = `root\cimv2\mdm\dmmap`

// ErrNotExecutable is returned when a policy has no bridge mapping (its
// schema is known but it is not drivable through this runtime).
var ErrNotExecutable = errors.New("csp: policy is not executable via the MDM bridge")

// ErrNotConfigured is returned when a policy has no value set on the device
// (its area instance or property is absent).
var ErrNotConfigured = errors.New("csp: policy is not configured")

// Connect opens the MDM bridge namespace. The calling process must run as
// SYSTEM — the bridge denies access below it — and the returned Service is
// bound to the calling goroutine (see wmi.Connect). Close it when done.
func Connect() (*wmi.Service, error) {
	return wmi.Connect(Namespace)
}

// Read returns the policy's effective (applied) value from the bridge Result
// class. A policy with no value set returns ErrNotConfigured.
func Read(svc *wmi.Service, p Policy) (any, error) {
	if !p.Executable() {
		return nil, ErrNotExecutable
	}
	return readProperty(svc, p, p.Bridge.ResultClass)
}

// ReadDesired returns the policy's desired value from the bridge Config
// class (what was set, versus Read's effective/applied value).
func ReadDesired(svc *wmi.Service, p Policy) (any, error) {
	if !p.Executable() {
		return nil, ErrNotExecutable
	}
	return readProperty(svc, p, p.Bridge.ConfigClass)
}

// Set writes the policy's desired value through the bridge Config class
// (OMA-DM Add/Replace). If the area instance already holds settings it is
// modified; otherwise it is created. The value is passed to WMI as-is (an
// int for most Policy settings, a string for the rest).
func Set(svc *wmi.Service, p Policy, value any) error {
	if !p.Executable() {
		return ErrNotExecutable
	}
	path, err := instancePath(svc, p.Bridge.ConfigClass, p.Bridge.ParentID, p.Bridge.InstanceID)
	if err != nil && !errors.Is(err, ErrNotConfigured) {
		return err
	}
	if err == nil {
		return svc.UpdateInstance(path, map[string]any{p.Bridge.Property: value})
	}
	// No area instance yet — create it with its keys and the setting.
	_, cerr := svc.CreateInstance(p.Bridge.ConfigClass, map[string]any{
		keyParentID:       p.Bridge.ParentID,
		keyInstanceID:     p.Bridge.InstanceID,
		p.Bridge.Property: value,
	})
	if cerr != nil {
		return fmt.Errorf("csp: set %s: %w", p.Bridge.Property, cerr)
	}
	return nil
}

// Delete removes the area's desired configuration (OMA-DM Delete), reverting
// its settings to unmanaged. Deleting an absent area returns ErrNotConfigured.
//
// The bridge Config instance is a per-area singleton holding every set
// property, so this clears the whole area — prefer Set-to-default to clear
// one setting.
func Delete(svc *wmi.Service, p Policy) error {
	if !p.Executable() {
		return ErrNotExecutable
	}
	path, err := instancePath(svc, p.Bridge.ConfigClass, p.Bridge.ParentID, p.Bridge.InstanceID)
	if err != nil {
		return err // ErrNotConfigured when absent
	}
	return svc.DeleteInstance(path)
}

const (
	keyParentID   = "ParentID"
	keyInstanceID = "InstanceID"
)

// readProperty enumerates a bridge class (the dynamic MDM provider answers a
// class query, not a key-bound GetObject) and returns one property from the
// area's instance. The class is area-specific, so it matches InstanceID or
// falls back to the sole row; absence or a nil property is ErrNotConfigured.
func readProperty(svc *wmi.Service, p Policy, class string) (any, error) {
	rows, err := svc.Query("SELECT * FROM " + class)
	if err != nil {
		return nil, err
	}
	row := pickInstance(rows, p.Bridge.InstanceID)
	if row == nil {
		return nil, ErrNotConfigured
	}
	value, ok := row[p.Bridge.Property]
	if !ok || value == nil {
		return nil, ErrNotConfigured
	}
	return value, nil
}

// pickInstance selects the area's row by InstanceID, falling back to the
// sole row when the class carries exactly one (the common singleton case).
func pickInstance(rows []wmi.Row, instanceID string) wmi.Row {
	for _, r := range rows {
		if wmi.AsString(r["InstanceID"]) == instanceID {
			return r
		}
	}
	if len(rows) == 1 {
		return rows[0]
	}
	return nil
}

// instancePath resolves the __PATH of an area's Config instance (used for
// modify/delete, which need a concrete object path rather than a key-bound
// GetObject). ErrNotConfigured when the area has no instance.
func instancePath(svc *wmi.Service, class, parentID, instanceID string) (string, error) {
	rows, err := svc.Query(instanceQuery("__PATH", class, parentID, instanceID))
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", ErrNotConfigured
	}
	path := wmi.AsString(rows[0]["__PATH"])
	if path == "" {
		return "", ErrNotConfigured
	}
	return path, nil
}

// instanceQuery builds the WQL that selects an area's singleton instance by
// its keys.
func instanceQuery(sel, class, parentID, instanceID string) string {
	return fmt.Sprintf("SELECT %s FROM %s WHERE ParentID=%s AND InstanceID=%s",
		sel, class, wmi.QuoteWQL(parentID), wmi.QuoteWQL(instanceID))
}

// resultParent maps a Config ParentID to the sibling Result path
// (./Vendor/MSFT/Policy/Config → ./Vendor/MSFT/Policy/Result).
func resultParent(configParent string) string {
	return strings.Replace(configParent, "/Policy/Config", "/Policy/Result", 1)
}
