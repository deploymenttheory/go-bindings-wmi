//go:build windows

package csp

import (
	"errors"
	"fmt"

	"github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"
)

// Namespace is the MDM WMI bridge namespace that executes CSP policies.
const Namespace = `root\cimv2\mdm\dmmap`

// ErrNotExecutable is returned when a policy has no bridge mapping (its
// schema is known but it is not drivable through this runtime).
var ErrNotExecutable = errors.New("csp: policy is not executable via the MDM bridge")

// ErrNotConfigured is returned by Read when the policy has no value set on
// the device (the area instance or property is absent).
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
	return read(svc, p, p.bridgeOr().ResultClass)
}

// ReadDesired returns the policy's desired value from the bridge Config
// class (what was set, versus Read's effective/applied value).
func ReadDesired(svc *wmi.Service, p Policy) (any, error) {
	return read(svc, p, p.bridgeOr().ConfigClass)
}

func read(svc *wmi.Service, p Policy, class string) (any, error) {
	if !p.Executable() {
		return nil, ErrNotExecutable
	}
	row, err := svc.GetInstance(instancePath(class, p.Bridge))
	if err != nil {
		if errors.Is(err, wmi.ErrNotFound) {
			return nil, ErrNotConfigured
		}
		return nil, err
	}
	value, ok := row[p.Bridge.Property]
	if !ok || value == nil {
		return nil, ErrNotConfigured
	}
	return value, nil
}

// Set writes the policy's desired value through the bridge Config class
// (OMA-DM Add/Replace). The value is passed to WMI as-is; for a Policy the
// bridge expects the setting's native type (an int for most, a string for
// the rest). Formatting helpers are on the generated typed accessors.
func Set(svc *wmi.Service, p Policy, value any) error {
	if !p.Executable() {
		return ErrNotExecutable
	}
	path := instancePath(p.Bridge.ConfigClass, p.Bridge)
	props := map[string]any{p.Bridge.Property: value}

	// The area instance is a singleton: Replace it if present, else Add it
	// (with its keys). Update-then-create covers both without a probe.
	updateErr := svc.UpdateInstance(path, props)
	if updateErr == nil {
		return nil
	}
	props[keyParentID] = p.Bridge.ParentID
	props[keyInstanceID] = p.Bridge.InstanceID
	if _, createErr := svc.CreateInstance(p.Bridge.ConfigClass, props); createErr != nil {
		return fmt.Errorf("csp: set %s: update: %v; create: %w", p.Bridge.Property, updateErr, createErr)
	}
	return nil
}

// Delete removes the policy's desired value (OMA-DM Delete), reverting it to
// unmanaged. Deleting an already-absent policy returns ErrNotConfigured.
func Delete(svc *wmi.Service, p Policy) error {
	if !p.Executable() {
		return ErrNotExecutable
	}
	// The Config instance holds every set property in the area, so this
	// clears the whole area's managed values, not one property. Prefer
	// Set-to-default to clear a single setting.
	err := svc.DeleteInstance(instancePath(p.Bridge.ConfigClass, p.Bridge))
	if errors.Is(err, wmi.ErrNotFound) {
		return ErrNotConfigured
	}
	return err
}

const (
	keyParentID   = "ParentID"
	keyInstanceID = "InstanceID"
)

// instancePath builds the WMI object path of a bridge area instance, e.g.
// MDM_Policy_Config01_Browser02.ParentID="./Device/Vendor/MSFT/Policy/Config",InstanceID="Browser".
func instancePath(class string, b *Bridge) string {
	return fmt.Sprintf("%s.%s=%q,%s=%q", class, keyParentID, b.ParentID, keyInstanceID, b.InstanceID)
}

// bridgeOr returns the policy's bridge or an empty one (read/ReadDesired
// guard on Executable before dereferencing the class names).
func (p Policy) bridgeOr() *Bridge {
	if p.Bridge != nil {
		return p.Bridge
	}
	return &Bridge{}
}
