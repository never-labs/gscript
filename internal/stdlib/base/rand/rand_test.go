package rand

import (
	"math"
	"strings"
	"testing"
)

func TestMaskInt48(t *testing.T) {
	if got := MaskInt48(-1); got != Int48Mask {
		t.Fatalf("MaskInt48(-1) = %x, want %x", got, Int48Mask)
	}
}

func TestInclusiveSpan(t *testing.T) {
	got, err := InclusiveSpan(3, 5)
	if err != nil || got != 3 {
		t.Fatalf("InclusiveSpan(3, 5) = %d, %v; want 3, nil", got, err)
	}
	if _, err := InclusiveSpan(5, 3); err == nil {
		t.Fatal("InclusiveSpan inverted range returned nil error")
	}
}

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

func TestIntBelow(t *testing.T) {
	got, err := IntBelow(func(n int64) int64 { return n - 1 }, 10)
	if err != nil {
		t.Fatal(err)
	}
	if got != 9 {
		t.Fatalf("IntBelow = %d, want 9", got)
	}
	if _, err := IntBelow(func(n int64) int64 { return 0 }, 0); err == nil {
		t.Fatal("IntBelow accepted non-positive max")
	}
}

func TestClampSampleCount(t *testing.T) {
	got, err := ClampSampleCount(8, 3)
	if err != nil || got != 3 {
		t.Fatalf("ClampSampleCount(8, 3) = %d, %v; want 3, nil", got, err)
	}
	if _, err := ClampSampleCount(-1, 3); err == nil {
		t.Fatal("ClampSampleCount negative count returned nil error")
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

func TestFormatUUIDV4(t *testing.T) {
	var bytes [16]byte
	for i := range bytes {
		bytes[i] = byte(i)
	}
	PrepareUUIDV4(&bytes)
	if bytes[6]>>4 != 4 {
		t.Fatalf("version nibble = %x, want 4", bytes[6]>>4)
	}
	if bytes[8]>>6 != 2 {
		t.Fatalf("variant bits = %b, want 10", bytes[8]>>6)
	}
	if got, want := FormatUUID(bytes), "00010203-0405-4607-8809-0a0b0c0d0e0f"; got != want {
		t.Fatalf("FormatUUID = %q, want %q", got, want)
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

func TestWeightedIndex(t *testing.T) {
	weights := []float64{1, 2, 3}
	total, err := ValidateWeights(weights)
	if err != nil || total != 6 {
		t.Fatalf("ValidateWeights = %v, %v; want 6, nil", total, err)
	}
	for _, tc := range []struct {
		point float64
		want  int
	}{
		{0, 0},
		{1, 1},
		{2.9, 1},
		{3, 2},
		{5.9, 2},
		{6, 2},
	} {
		if got := WeightedIndex(weights, tc.point); got != tc.want {
			t.Fatalf("WeightedIndex(%v) = %d, want %d", tc.point, got, tc.want)
		}
	}
}

func TestValidateWeightsRejectsInvalid(t *testing.T) {
	for _, weights := range [][]float64{{0, 0}, {-1}, {math.Inf(1)}} {
		if _, err := ValidateWeights(weights); err == nil {
			t.Fatalf("ValidateWeights(%v) returned nil error", weights)
		}
	}
}
