package cimschema

import (
	"fmt"
	"strings"
)

// DiffReport is the semantic difference between two snapshots of the same
// namespace — what a fresh capture changed, in reviewable terms. It backs
// the PR body of the scheduled schema-update workflow.
type DiffReport struct {
	Namespace      string
	Old, New       Provenance
	AddedClasses   []string
	RemovedClasses []string
	ChangedClasses []ClassDiff
}

// ClassDiff is one class's property- and method-level changes. Methods are
// carried as rendered signatures — the reviewable unit is the whole
// signature.
type ClassDiff struct {
	Name              string
	AddedProperties   []Property
	RemovedProperties []string
	ChangedProperties []PropertyChange
	AddedMethods      []string
	RemovedMethods    []string
	ChangedMethods    []MethodChange
}

// MethodChange is one method whose signature changed.
type MethodChange struct {
	Name     string
	Old, New string // rendered signatures
}

// PropertyChange is one property whose type, array flag, or key flag changed.
type PropertyChange struct {
	Name     string
	Old, New Property
}

// Diff compares two snapshots of the same namespace. Both are assumed valid
// (classes and properties sorted by name).
func Diff(oldSnap, newSnap *Snapshot) *DiffReport {
	report := &DiffReport{
		Namespace: newSnap.Namespace,
		Old:       oldSnap.Provenance,
		New:       newSnap.Provenance,
	}

	oldClasses := map[string]Class{}
	for _, c := range oldSnap.Classes {
		oldClasses[c.Name] = c
	}
	newClasses := map[string]bool{}
	for _, c := range newSnap.Classes {
		newClasses[c.Name] = true
	}

	for _, c := range oldSnap.Classes {
		if !newClasses[c.Name] {
			report.RemovedClasses = append(report.RemovedClasses, c.Name)
		}
	}
	for _, c := range newSnap.Classes {
		before, existed := oldClasses[c.Name]
		if !existed {
			report.AddedClasses = append(report.AddedClasses, c.Name)
			continue
		}
		if d := diffClass(before, c); d != nil {
			report.ChangedClasses = append(report.ChangedClasses, *d)
		}
	}
	return report
}

func diffClass(before, after Class) *ClassDiff {
	old := map[string]Property{}
	for _, p := range before.Properties {
		old[p.Name] = p
	}
	current := map[string]bool{}
	for _, p := range after.Properties {
		current[p.Name] = true
	}

	d := ClassDiff{Name: after.Name}
	for _, p := range before.Properties {
		if !current[p.Name] {
			d.RemovedProperties = append(d.RemovedProperties, p.Name)
		}
	}
	for _, p := range after.Properties {
		prev, existed := old[p.Name]
		if !existed {
			d.AddedProperties = append(d.AddedProperties, p)
			continue
		}
		if !prev.Equal(p) {
			d.ChangedProperties = append(d.ChangedProperties, PropertyChange{Name: p.Name, Old: prev, New: p})
		}
	}

	oldMethods := map[string]string{}
	for _, m := range before.Methods {
		oldMethods[m.Name] = MethodSignature(m)
	}
	currentMethods := map[string]bool{}
	for _, m := range after.Methods {
		currentMethods[m.Name] = true
	}
	for _, m := range before.Methods {
		if !currentMethods[m.Name] {
			d.RemovedMethods = append(d.RemovedMethods, oldMethods[m.Name])
		}
	}
	for _, m := range after.Methods {
		signature := MethodSignature(m)
		prev, existed := oldMethods[m.Name]
		if !existed {
			d.AddedMethods = append(d.AddedMethods, signature)
			continue
		}
		if prev != signature {
			d.ChangedMethods = append(d.ChangedMethods, MethodChange{Name: m.Name, Old: prev, New: signature})
		}
	}

	if len(d.AddedProperties) == 0 && len(d.RemovedProperties) == 0 && len(d.ChangedProperties) == 0 &&
		len(d.AddedMethods) == 0 && len(d.RemovedMethods) == 0 && len(d.ChangedMethods) == 0 {
		return nil
	}
	return &d
}

// MethodSignature renders a method for diff output and review, e.g.
// "Create(CommandLine string, ProcessStartupInformation any) → (ProcessId uint32, ReturnValue uint32) [static]".
func MethodSignature(m Method) string {
	var b strings.Builder
	b.WriteString(m.Name)
	b.WriteString("(")
	for i, p := range m.In {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(p.Name + " " + describeParam(p))
	}
	b.WriteString(")")
	if len(m.Out) > 0 {
		b.WriteString(" → (")
		for i, p := range m.Out {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(p.Name + " " + describeParam(p))
		}
		b.WriteString(")")
	}
	if m.Static {
		b.WriteString(" [static]")
	}
	return b.String()
}

// Empty reports whether the two snapshots have identical schema (provenance
// aside).
func (r *DiffReport) Empty() bool {
	return len(r.AddedClasses) == 0 && len(r.RemovedClasses) == 0 && len(r.ChangedClasses) == 0
}

// Markdown renders the report for a PR body. An empty report renders the
// stable sentence "No schema changes." that automation keys off.
func (r *DiffReport) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# `%s` schema diff\n\n", r.Namespace)
	fmt.Fprintf(&b, "Build %s (captured %s) → build %s (captured %s)\n\n",
		r.Old.OSBuild, r.Old.Captured, r.New.OSBuild, r.New.Captured)

	if r.Empty() {
		b.WriteString("No schema changes.\n")
		return b.String()
	}

	if len(r.AddedClasses) > 0 {
		fmt.Fprintf(&b, "## Added classes (%d)\n\n", len(r.AddedClasses))
		for _, name := range r.AddedClasses {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if len(r.RemovedClasses) > 0 {
		fmt.Fprintf(&b, "## Removed classes (%d)\n\n", len(r.RemovedClasses))
		for _, name := range r.RemovedClasses {
			fmt.Fprintf(&b, "- `%s`\n", name)
		}
		b.WriteString("\n")
	}
	if len(r.ChangedClasses) > 0 {
		fmt.Fprintf(&b, "## Changed classes (%d)\n\n", len(r.ChangedClasses))
		for _, class := range r.ChangedClasses {
			fmt.Fprintf(&b, "### `%s`\n\n", class.Name)
			for _, p := range class.AddedProperties {
				fmt.Fprintf(&b, "- added `%s` (%s)\n", p.Name, describeProperty(p))
			}
			for _, name := range class.RemovedProperties {
				fmt.Fprintf(&b, "- removed `%s`\n", name)
			}
			for _, change := range class.ChangedProperties {
				fmt.Fprintf(&b, "- changed `%s`: %s → %s\n",
					change.Name, describeProperty(change.Old), describeProperty(change.New))
			}
			for _, signature := range class.AddedMethods {
				fmt.Fprintf(&b, "- added method `%s`\n", signature)
			}
			for _, signature := range class.RemovedMethods {
				fmt.Fprintf(&b, "- removed method `%s`\n", signature)
			}
			for _, change := range class.ChangedMethods {
				fmt.Fprintf(&b, "- changed method `%s` → `%s`\n", change.Old, change.New)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// describeProperty renders a property's shape for diff output, e.g.
// "uint64", "[]string", "string, key", "uint16, enum". The enum/bitmask
// markers keep qualifier-only recaptures compact — the value tables
// themselves stay out of the PR body.
func describeProperty(p Property) string {
	s := GoType(p)
	if p.Key {
		s += ", key"
	}
	if len(p.Values) > 0 || len(p.ValueMap) > 0 {
		s += ", enum"
	}
	if len(p.BitValues) > 0 || len(p.BitMap) > 0 {
		s += ", bitmask"
	}
	return s
}

// describeParam renders a parameter's shape for method signatures, e.g.
// "uint16", "uint16{enum}" — the marker surfaces qualifier captures without
// dumping value tables.
func describeParam(p Param) string {
	s := ParamGoType(p)
	if len(p.Values) > 0 || len(p.ValueMap) > 0 {
		s += "{enum}"
	}
	if len(p.BitValues) > 0 || len(p.BitMap) > 0 {
		s += "{bitmask}"
	}
	return s
}
