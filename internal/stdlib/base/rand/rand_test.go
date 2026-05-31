package rand

import (
	"strings"
	"testing"
)

func TestIntRange(t *testing.T) {
	got, err := IntRange(func(n int64) int64 { return n - 1 }, 10, 12)
	if err != nil {
		t.Fatal(err)
	}
	if got != 12 {
		t.Fatalf("IntRange = %d, want 12", got)
	}
	if _, err := IntRange(func(n int64) int64 { return 0 }, 12, 10); err == nil {
		t.Fatal("IntRange accepted min > max")
	}
}

func TestNormalAndExponential(t *testing.T) {
	normal, err := Normal(func() float64 { return 2 }, 10, 3)
	if err != nil {
		t.Fatal(err)
	}
	if normal != 16 {
		t.Fatalf("Normal = %v, want 16", normal)
	}
	if _, err := Normal(func() float64 { return 0 }, 0, -1); err == nil {
		t.Fatal("Normal accepted negative stddev")
	}
	exp, err := Exponential(func() float64 { return 8 }, 2)
	if err != nil {
		t.Fatal(err)
	}
	if exp != 4 {
		t.Fatalf("Exponential = %v, want 4", exp)
	}
	if _, err := Exponential(func() float64 { return 0 }, 0); err == nil {
		t.Fatal("Exponential accepted non-positive rate")
	}
}

func TestBytes(t *testing.T) {
	next := byte('a')
	got, err := Bytes(3, func() byte {
		b := next
		next++
		return b
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "abc" {
		t.Fatalf("Bytes = %q, want abc", string(got))
	}
	if _, err := Bytes(-1, func() byte { return 0 }); err == nil {
		t.Fatal("Bytes accepted negative length")
	}
}

func TestUUIDV4(t *testing.T) {
	next := byte(0)
	got := UUIDV4(func() byte {
		b := next
		next++
		return b
	})
	if len(got) != 36 || strings.Count(got, "-") != 4 {
		t.Fatalf("UUIDV4 = %q, want canonical form", got)
	}
	if got[14] != '4' {
		t.Fatalf("UUIDV4 version = %q, want 4", got[14])
	}
	if got[19] != '8' && got[19] != '9' && got[19] != 'a' && got[19] != 'b' {
		t.Fatalf("UUIDV4 variant = %q, want 8/9/a/b", got[19])
	}
}
