package encoding

import "testing"

func TestHexRoundTrip(t *testing.T) {
	encoded := HexEncode("hello")
	if encoded != "68656c6c6f" {
		t.Fatalf("HexEncode() = %q", encoded)
	}
	decoded, err := HexDecode(encoded)
	if err != nil {
		t.Fatalf("HexDecode() error = %v", err)
	}
	if decoded != "hello" {
		t.Fatalf("HexDecode() = %q", decoded)
	}
}

func TestBase32RoundTrip(t *testing.T) {
	encoded := Base32Encode("hello")
	if encoded != "NBSWY3DP" {
		t.Fatalf("Base32Encode() = %q", encoded)
	}
	decoded, err := Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode() error = %v", err)
	}
	if decoded != "hello" {
		t.Fatalf("Base32Decode() = %q", decoded)
	}
}

func TestBase32HexRoundTrip(t *testing.T) {
	encoded := Base32HexEncode("hello world")
	decoded, err := Base32HexDecode(encoded)
	if err != nil {
		t.Fatalf("Base32HexDecode() error = %v", err)
	}
	if decoded != "hello world" {
		t.Fatalf("Base32HexDecode() = %q", decoded)
	}
}

func TestDecodeINI(t *testing.T) {
	doc := DecodeINI("; comment\nkey=value\n\n[database]\nhost=localhost\nport=5432\n")
	if len(doc.Globals) != 1 || doc.Globals[0] != (INIKeyValue{Key: "key", Value: "value"}) {
		t.Fatalf("Globals = %#v", doc.Globals)
	}
	if len(doc.Sections) != 1 {
		t.Fatalf("Sections = %#v", doc.Sections)
	}
	section := doc.Sections[0]
	if section.Name != "database" {
		t.Fatalf("Section name = %q", section.Name)
	}
	want := []INIKeyValue{{Key: "host", Value: "localhost"}, {Key: "port", Value: "5432"}}
	if len(section.Items) != len(want) {
		t.Fatalf("Section items = %#v", section.Items)
	}
	for i := range want {
		if section.Items[i] != want[i] {
			t.Fatalf("Section item %d = %#v", i, section.Items[i])
		}
	}
}

func TestEncodeINI(t *testing.T) {
	got := EncodeINI(INIDocument{
		Globals: []INIKeyValue{{Key: "key", Value: "value"}},
		Sections: []INISection{
			{Name: "database", Items: []INIKeyValue{{Key: "host", Value: "localhost"}}},
			{Name: "app", Items: []INIKeyValue{{Key: "name", Value: "demo"}}},
		},
	})
	want := "key=value\n\n[database]\nhost=localhost\n\n[app]\nname=demo\n"
	if got != want {
		t.Fatalf("EncodeINI() = %q, want %q", got, want)
	}
}

func TestXMLUnescapeNumericReferences(t *testing.T) {
	got, err := XMLUnescape("&#65;&#x42;&lt;&amp;")
	if err != nil {
		t.Fatalf("XMLUnescape() error = %v", err)
	}
	if got != "AB<&" {
		t.Fatalf("XMLUnescape() = %q", got)
	}
}

func TestXMLUnescapeInvalidNumericReference(t *testing.T) {
	if _, err := XMLUnescape("&#x;"); err == nil {
		t.Fatal("XMLUnescape() error = nil")
	}
}
