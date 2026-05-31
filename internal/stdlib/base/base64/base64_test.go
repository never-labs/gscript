package base64

import "testing"

func TestEncodeDecode(t *testing.T) {
	got := Encode("hello world")
	if got != "aGVsbG8gd29ybGQ=" {
		t.Fatalf("Encode() = %q, want %q", got, "aGVsbG8gd29ybGQ=")
	}

	decoded, err := Decode(got)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != "hello world" {
		t.Fatalf("Decode() = %q, want %q", decoded, "hello world")
	}
}

func TestDecodeRejectsInvalid(t *testing.T) {
	if _, err := Decode("!!!invalid!!!"); err == nil {
		t.Fatal("Decode() accepted invalid base64")
	}
}

func TestURLEncodeDecode(t *testing.T) {
	input := "Hello+World/Test==Foo"
	got := URLEncode(input)
	if got != "SGVsbG8rV29ybGQvVGVzdD09Rm9v" {
		t.Fatalf("URLEncode() = %q, want %q", got, "SGVsbG8rV29ybGQvVGVzdD09Rm9v")
	}

	decoded, err := URLDecode(got)
	if err != nil {
		t.Fatal(err)
	}
	if decoded != input {
		t.Fatalf("URLDecode() = %q, want %q", decoded, input)
	}
}

func TestURLDecodeRejectsInvalid(t *testing.T) {
	if _, err := URLDecode("!!!invalid!!!"); err == nil {
		t.Fatal("URLDecode() accepted invalid base64")
	}
}

func TestLengths(t *testing.T) {
	std := "aGVsbG8gd29ybGQ="
	if got := EncodedLen(len("hello world")); got != len(std) {
		t.Fatalf("EncodedLen() = %d, want %d", got, len(std))
	}
	if got := DecodedLen(len(std)); got < len("hello world") {
		t.Fatalf("DecodedLen() = %d, want at least %d", got, len("hello world"))
	}

	rawURL := "SGVsbG8rV29ybGQvVGVzdD09Rm9v"
	rawInput := "Hello+World/Test==Foo"
	if got := URLEncodedLen(len(rawInput)); got != len(rawURL) {
		t.Fatalf("URLEncodedLen() = %d, want %d", got, len(rawURL))
	}
	if got := URLDecodedLen(len(rawURL)); got < len(rawInput) {
		t.Fatalf("URLDecodedLen() = %d, want at least %d", got, len(rawInput))
	}
}
