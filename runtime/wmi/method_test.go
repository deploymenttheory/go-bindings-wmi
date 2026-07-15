//go:build windows

package wmi

import "testing"

func TestClassOfPath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Win32_Process", "Win32_Process"},
		{`Win32_Process.Handle="42"`, "Win32_Process"},
		{`\\HOST\root\cimv2:Win32_Process.Handle="42"`, "Win32_Process"},
		{`\\.\root\cimv2:Win32_Service.Name="Spooler"`, "Win32_Service"},
		// Key values may contain the delimiters.
		{`Win32_Service.Name="odd:name.exe"`, "Win32_Service"},
		{`CIM_DataFile.Name="C:\\x.txt"`, "CIM_DataFile"},
	}
	for _, c := range cases {
		if got := classOfPath(c.in); got != c.want {
			t.Errorf("classOfPath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
