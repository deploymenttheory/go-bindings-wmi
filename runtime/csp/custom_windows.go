//go:build windows

package csp

import "github.com/deploymenttheory/go-bindings-wmi/runtime/wmi"

// Target identifies a bridge instance property for a policy the DDF export
// does not cover — a dynamic-instance node ({GUID}), a third-party or custom
// CSP, or any class discovered via the bridge that has no generated binding.
// Build one and drive it with the same Read/Set/Delete as a generated policy.
type Target struct {
	// Class is the bridge WMI class for reads and writes, e.g.
	// MDM_Policy_Config01_Browser02 or a custom MDM_<Csp> class.
	Class string
	// ResultClass is the class for effective-value reads, when the CSP
	// splits config and result (Policy areas). Empty means reads use Class.
	ResultClass string
	// InstanceID and ParentID key the bridge instance (see the inspect
	// diagnostic to discover them for an unfamiliar class).
	InstanceID string
	ParentID   string
	// Property is the setting's property name on the instance.
	Property string
}

// Policy turns a Target into an executable csp.Policy, so the free
// Read/Set/Delete functions and typed helpers accept it directly.
func (t Target) Policy() Policy {
	result := t.ResultClass
	if result == "" {
		result = t.Class
	}
	return Policy{
		Name: t.Property,
		Bridge: &Bridge{
			ConfigClass: t.Class,
			ResultClass: result,
			InstanceID:  t.InstanceID,
			ParentID:    t.ParentID,
			Property:    t.Property,
		},
	}
}

// Custom drives arbitrary bridge Targets — the escape hatch for policies the
// generated bindings do not cover. It is the counterpart of a generated area
// Service, bound to the same connected bridge session.
type Custom struct {
	svc *wmi.Service
}

// NewCustom returns a Custom service bound to a connected bridge session.
func NewCustom(svc *wmi.Service) *Custom { return &Custom{svc: svc} }

// Read returns the target's effective value (ErrNotConfigured when unset).
func (c *Custom) Read(t Target) (any, error) { return Read(c.svc, t.Policy()) }

// ReadDesired returns the target's desired (configured) value.
func (c *Custom) ReadDesired(t Target) (any, error) { return ReadDesired(c.svc, t.Policy()) }

// Set writes the target's value (OMA-DM Add/Replace).
func (c *Custom) Set(t Target, value any) error { return Set(c.svc, t.Policy(), value) }

// Delete removes the target's configuration (OMA-DM Delete).
func (c *Custom) Delete(t Target) error { return Delete(c.svc, t.Policy()) }

// Query enumerates every instance of a bridge class — a raw read for
// discovering an unfamiliar class's instances and their InstanceID/ParentID
// keys (which the DDF export does not reveal).
func (c *Custom) Query(class string) ([]wmi.Row, error) {
	return c.svc.Query("SELECT * FROM " + class)
}
