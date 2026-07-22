package wmi

import (
	"reflect"
	"testing"
)

// kvpItemXML is the shape Hyper-V returns in
// Msvm_KvpExchangeComponent.GuestIntrinsicExchangeItems.
const kvpItemXML = `<INSTANCE CLASSNAME="Msvm_KvpExchangeDataItem">
<PROPERTY NAME="Caption" PROPAGATED="true" TYPE="string"></PROPERTY>
<PROPERTY NAME="Data" TYPE="string"><VALUE>10.0.26200</VALUE></PROPERTY>
<PROPERTY NAME="Name" TYPE="string"><VALUE>OSVersion</VALUE></PROPERTY>
<PROPERTY NAME="Source" TYPE="uint16"><VALUE>2</VALUE></PROPERTY>
</INSTANCE>`

func TestParseObjectTextKVPItem(t *testing.T) {
	row, err := ParseObjectText(kvpItemXML)
	if err != nil {
		t.Fatalf("ParseObjectText: %v", err)
	}
	if got := AsString(row["__CLASS"]); got != "Msvm_KvpExchangeDataItem" {
		t.Errorf("__CLASS = %q", got)
	}
	if got := AsString(row["Name"]); got != "OSVersion" {
		t.Errorf("Name = %q", got)
	}
	if got := AsString(row["Data"]); got != "10.0.26200" {
		t.Errorf("Data = %q", got)
	}
	if got, ok := row["Source"].(int64); !ok || got != 2 {
		t.Errorf("Source = %#v, want int64(2)", row["Source"])
	}
	// NULL property (no VALUE child) is present as nil.
	if v, ok := row["Caption"]; !ok || v != nil {
		t.Errorf("Caption = %#v (present %v), want present nil", v, ok)
	}
}

func TestParseObjectTextShapes(t *testing.T) {
	const text = `<?xml version="1.0"?>
<INSTANCE CLASSNAME="Demo_Widget">
 <QUALIFIER NAME="dynamic" TYPE="boolean"><VALUE>TRUE</VALUE></QUALIFIER>
 <PROPERTY NAME="Enabled" TYPE="boolean"><VALUE>TRUE</VALUE></PROPERTY>
 <PROPERTY NAME="Size" TYPE="uint64"><VALUE>18446744073709551615</VALUE></PROPERTY>
 <PROPERTY NAME="Ratio" TYPE="real32"><VALUE>2.5</VALUE></PROPERTY>
 <PROPERTY NAME="Escaped" TYPE="string"><VALUE>a &lt;b&gt; &amp; c</VALUE></PROPERTY>
 <PROPERTY.ARRAY NAME="Caps" TYPE="uint16">
  <VALUE.ARRAY><VALUE>1</VALUE><VALUE>2</VALUE></VALUE.ARRAY>
 </PROPERTY.ARRAY>
 <PROPERTY.ARRAY NAME="Tags" TYPE="string">
  <VALUE.ARRAY><VALUE>a</VALUE><VALUE>b</VALUE></VALUE.ARRAY>
 </PROPERTY.ARRAY>
 <PROPERTY.REFERENCE NAME="Owner" REFERENCECLASS="Demo_Owner">
  <VALUE.REFERENCE>Demo_Owner.Name="root"</VALUE.REFERENCE>
 </PROPERTY.REFERENCE>
 <PROPERTY.OBJECT NAME="Startup">
  <VALUE.OBJECT>
   <INSTANCE CLASSNAME="Demo_Startup">
    <PROPERTY NAME="ShowWindow" TYPE="uint16"><VALUE>0</VALUE></PROPERTY>
   </INSTANCE>
  </VALUE.OBJECT>
 </PROPERTY.OBJECT>
</INSTANCE>`

	row, err := ParseObjectText(text)
	if err != nil {
		t.Fatalf("ParseObjectText: %v", err)
	}
	if v, ok := row["Enabled"].(bool); !ok || !v {
		t.Errorf("Enabled = %#v", row["Enabled"])
	}
	if v, ok := row["Size"].(uint64); !ok || v != 18446744073709551615 {
		t.Errorf("Size = %#v", row["Size"])
	}
	if v, ok := row["Ratio"].(float64); !ok || v != 2.5 {
		t.Errorf("Ratio = %#v", row["Ratio"])
	}
	if v := AsString(row["Escaped"]); v != "a <b> & c" {
		t.Errorf("Escaped = %q", v)
	}
	if v, ok := row["Caps"].([]int64); !ok || !reflect.DeepEqual(v, []int64{1, 2}) {
		t.Errorf("Caps = %#v", row["Caps"])
	}
	if v, ok := row["Tags"].([]string); !ok || !reflect.DeepEqual(v, []string{"a", "b"}) {
		t.Errorf("Tags = %#v", row["Tags"])
	}
	if v := AsString(row["Owner"]); v != `Demo_Owner.Name="root"` {
		t.Errorf("Owner = %q", v)
	}
	nested, ok := row["Startup"].(Row)
	if !ok {
		t.Fatalf("Startup = %#v, want nested Row", row["Startup"])
	}
	if got := AsString(nested["__CLASS"]); got != "Demo_Startup" {
		t.Errorf("nested __CLASS = %q", got)
	}
	if got, _ := nested["ShowWindow"].(int64); got != 0 {
		t.Errorf("nested ShowWindow = %#v", nested["ShowWindow"])
	}
	// The qualifier element is schema, not data.
	if _, present := row["dynamic"]; present {
		t.Error("qualifier leaked into the row")
	}
}

func TestParseObjectTextErrors(t *testing.T) {
	for _, in := range []string{
		"",
		"not xml",
		"<PROPERTY NAME='x'/>",      // no INSTANCE
		`<INSTANCE CLASSNAME="X">*`, // truncated
	} {
		if _, err := ParseObjectText(in); err == nil {
			t.Errorf("ParseObjectText(%q): expected error", in)
		}
	}
}
