package wmi

import "maps"

// Row is one WMI instance: property name → decoded Go value. Scalars widen
// to string, int64, uint64, bool, or float64; array properties decode to
// typed slices of those; embedded CIM objects decode to a nested Row (its __CLASS
// system property names the embedded class); NULL properties are nil.
type Row map[string]any

// Instance builds a Row describing an embedded CIM object for a method
// in-parameter, e.g.
//
//	wmi.Instance("Win32_ProcessStartup", map[string]any{"ShowWindow": uint16(0)})
//
// The __CLASS key tells the runtime which class to spawn; rows returned by
// queries already carry it.
func Instance(class string, props map[string]any) Row {
	row := Row{"__CLASS": class}
	maps.Copy(row, props)
	return row
}
