package bytes

import "testing"

func TestXOR(t *testing.T) {
	got, err := XOR("\x0f\xf0", "\xf0\x0f")
	if err != nil {
		t.Fatal(err)
	}
	if got != "\xff\xff" {
		t.Fatalf("XOR = %q, want ff ff", got)
	}

	if _, err := XOR("a", "ab"); err == nil {
		t.Fatal("XOR mismatched lengths succeeded")
	}
}

func TestHexCompareRepeat(t *testing.T) {
	if got := ToHex("az"); got != "617a" {
		t.Fatalf("ToHex = %q, want 617a", got)
	}
	decoded, err := DecodeHex("617a")
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "az" {
		t.Fatalf("DecodeHex = %q, want az", string(decoded))
	}
	if got := Compare("a", "b"); got >= 0 {
		t.Fatalf("Compare(a,b) = %d, want negative", got)
	}
	repeated, err := Repeat("ab", 3)
	if err != nil {
		t.Fatal(err)
	}
	if repeated != "ababab" {
		t.Fatalf("Repeat = %q, want ababab", repeated)
	}
}
