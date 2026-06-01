package uuid

import (
	"bytes"
	"regexp"
	"testing"
)

func TestV4FromSetsVersionAndVariant(t *testing.T) {
	src := bytes.NewReader([]byte{
		0x55, 0x0e, 0x84, 0x00,
		0xe2, 0x9b,
		0x01, 0xd4,
		0x27, 0x16,
		0x44, 0x66, 0x55, 0x44, 0x00, 0x00,
	})

	got, err := V4From(src)
	if err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^550e8400-e29b-41d4-a716-446655440000$`)
	if !want.MatchString(got) {
		t.Fatalf("V4From() = %q", got)
	}
}

func TestParse(t *testing.T) {
	got, ok := Parse("550e8400-e29b-41d4-a716-446655440000")
	if !ok {
		t.Fatal("Parse() rejected valid UUID")
	}
	if got.Version != 4 || got.Variant != "RFC4122" || got.Bytes != "550e8400e29b41d4a716446655440000" {
		t.Fatalf("Parse() = %#v", got)
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	if _, ok := Parse("not-a-uuid"); ok {
		t.Fatal("Parse() accepted invalid UUID")
	}
}
