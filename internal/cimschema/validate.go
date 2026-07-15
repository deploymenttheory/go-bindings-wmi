package cimschema

import (
	"errors"
	"fmt"
)

// ErrInvalidSnapshot is returned (wrapped) for snapshots that violate the
// structural invariants the pipeline relies on.
var ErrInvalidSnapshot = errors.New("invalid snapshot")

// Validate checks the invariants a committed snapshot must hold: namespace
// and provenance present, classes sorted and unique, properties sorted and
// unique per class, CIM types positive with the array flag normalized into
// Array. Unknown CIM types are not an error — the generator degrades them to
// `any` by design.
func (s *Snapshot) Validate() error {
	if s.Namespace == "" {
		return fmt.Errorf("%w: empty namespace", ErrInvalidSnapshot)
	}
	if s.Provenance.OSBuild == "" || s.Provenance.Captured == "" {
		return fmt.Errorf("%w: %s: provenance incomplete (osBuild %q, captured %q)",
			ErrInvalidSnapshot, s.Namespace, s.Provenance.OSBuild, s.Provenance.Captured)
	}
	if len(s.Classes) == 0 {
		return fmt.Errorf("%w: %s: no classes", ErrInvalidSnapshot, s.Namespace)
	}
	for i, class := range s.Classes {
		if class.Name == "" {
			return fmt.Errorf("%w: %s: class %d has an empty name", ErrInvalidSnapshot, s.Namespace, i)
		}
		if i > 0 && s.Classes[i-1].Name >= class.Name {
			return fmt.Errorf("%w: %s: classes not sorted/unique at %q", ErrInvalidSnapshot, s.Namespace, class.Name)
		}
		for j, p := range class.Properties {
			if p.Name == "" {
				return fmt.Errorf("%w: %s.%s: property %d has an empty name", ErrInvalidSnapshot, s.Namespace, class.Name, j)
			}
			if j > 0 && class.Properties[j-1].Name >= p.Name {
				return fmt.Errorf("%w: %s.%s: properties not sorted/unique at %q", ErrInvalidSnapshot, s.Namespace, class.Name, p.Name)
			}
			if p.CIMType <= 0 {
				return fmt.Errorf("%w: %s.%s.%s: non-positive CIM type %d", ErrInvalidSnapshot, s.Namespace, class.Name, p.Name, p.CIMType)
			}
			if p.CIMType&CIMFlagArray != 0 {
				return fmt.Errorf("%w: %s.%s.%s: array flag not normalized into the array field", ErrInvalidSnapshot, s.Namespace, class.Name, p.Name)
			}
		}
		for j, m := range class.Methods {
			if m.Name == "" {
				return fmt.Errorf("%w: %s.%s: method %d has an empty name", ErrInvalidSnapshot, s.Namespace, class.Name, j)
			}
			if j > 0 && class.Methods[j-1].Name >= m.Name {
				return fmt.Errorf("%w: %s.%s: methods not sorted/unique at %q", ErrInvalidSnapshot, s.Namespace, class.Name, m.Name)
			}
			// Parameters keep declaration order (not sorted); check
			// uniqueness and normalization per direction.
			for _, direction := range [][]Param{m.In, m.Out} {
				seen := map[string]bool{}
				for _, p := range direction {
					if p.Name == "" {
						return fmt.Errorf("%w: %s.%s.%s: parameter with an empty name", ErrInvalidSnapshot, s.Namespace, class.Name, m.Name)
					}
					if seen[p.Name] {
						return fmt.Errorf("%w: %s.%s.%s: duplicate parameter %q", ErrInvalidSnapshot, s.Namespace, class.Name, m.Name, p.Name)
					}
					seen[p.Name] = true
					if p.CIMType <= 0 {
						return fmt.Errorf("%w: %s.%s.%s.%s: non-positive CIM type %d", ErrInvalidSnapshot, s.Namespace, class.Name, m.Name, p.Name, p.CIMType)
					}
					if p.CIMType&CIMFlagArray != 0 {
						return fmt.Errorf("%w: %s.%s.%s.%s: array flag not normalized", ErrInvalidSnapshot, s.Namespace, class.Name, m.Name, p.Name)
					}
				}
			}
		}
	}
	return nil
}
