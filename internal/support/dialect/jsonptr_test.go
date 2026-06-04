package dialect

import (
	"reflect"
	"testing"
)

func TestParseJSONPointerDecodesTokens(t *testing.T) {
	got, err := ParseJSONPointer("/a~1b/c~0d/0")
	if err != nil {
		t.Fatalf("ParseJSONPointer: %v", err)
	}
	want := []string{"a/b", "c~d", "0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens = %#v, want %#v", got, want)
	}
	if encoded := EncodeJSONPointer(got); encoded != "/a~1b/c~0d/0" {
		t.Fatalf("EncodeJSONPointer = %q", encoded)
	}
}

func TestParseJSONPointerRejectsInvalid(t *testing.T) {
	for _, input := range []string{"missing-slash", "/bad~2escape", "/bad~"} {
		if _, err := ParseJSONPointer(input); err == nil {
			t.Fatalf("ParseJSONPointer(%q) returned nil error", input)
		}
	}
}

func TestJSONPointerIndex(t *testing.T) {
	if got, ok := JSONPointerIndex("0"); !ok || got != 0 {
		t.Fatalf("index 0 = %d %v, want 0 true", got, ok)
	}
	for _, input := range []string{"01", "-", "-1", "x"} {
		if _, ok := JSONPointerIndex(input); ok {
			t.Fatalf("JSONPointerIndex(%q) accepted invalid index", input)
		}
	}
}
