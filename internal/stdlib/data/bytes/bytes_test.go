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

func TestClampReadRange(t *testing.T) {
	from, to, ok := ClampReadRange(5, -2, 9)
	if from != 0 || to != 5 || !ok {
		t.Fatalf("ClampReadRange = %d, %d, %v; want 0, 5, true", from, to, ok)
	}
	from, to, ok = ClampReadRange(5, 4, 2)
	if from != 3 || to != 2 || ok {
		t.Fatalf("ClampReadRange empty = %d, %d, %v; want 3, 2, false", from, to, ok)
	}
}

func TestByteIndex(t *testing.T) {
	if pos, ok := ByteIndex(3, 2); pos != 1 || !ok {
		t.Fatalf("ByteIndex valid = %d, %v; want 1, true", pos, ok)
	}
	if _, ok := ByteIndex(3, 0); ok {
		t.Fatalf("ByteIndex accepted zero position")
	}
	if _, ok := ByteIndex(3, 4); ok {
		t.Fatalf("ByteIndex accepted out-of-range position")
	}
}
