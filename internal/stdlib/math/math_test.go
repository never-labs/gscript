package math

import (
	stdmath "math"
	"testing"
)

func TestIntegralResults(t *testing.T) {
	tests := []struct {
		name string
		got  Number
		want int64
	}{
		{"abs", Abs(Int(-5)), 5},
		{"floor", Floor(Float(3.7)), 3},
		{"ceil", Ceil(Float(3.2)), 4},
		{"clamp", Clamp(Int(15), Int(0), Int(10)), 10},
		{"round", Round(Float(3.7), 0), 4},
		{"trunc", Trunc(Float(-3.7)), -3},
	}
	for _, tt := range tests {
		if !tt.got.IsInt || tt.got.Int != tt.want {
			t.Fatalf("%s = %#v, want int %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestFloorDiv(t *testing.T) {
	tests := []struct {
		a, b int64
		want int64
	}{
		{7, 3, 2},
		{-7, 3, -3},
		{7, -3, -3},
		{-7, -3, 2},
	}
	for _, tt := range tests {
		got, ok := FloorDiv(Int(tt.a), Int(tt.b))
		if !ok || !got.IsInt || got.Int != tt.want {
			t.Fatalf("FloorDiv(%d, %d) = %#v, %v; want %d, true", tt.a, tt.b, got, ok, tt.want)
		}
	}
	if _, ok := FloorDiv(Int(1), Int(0)); ok {
		t.Fatal("FloorDiv by zero succeeded")
	}
}

func TestFloatOperations(t *testing.T) {
	if got := Lerp(Int(0), Int(10), Float(0.25)); got != 2.5 {
		t.Fatalf("Lerp = %v, want 2.5", got)
	}
	if got := Sign(Float(-0.5)); got != -1 {
		t.Fatalf("Sign = %v, want -1", got)
	}
	got, ok := Fmod(Float(7.5), Float(2))
	if !ok || got.IsInt || stdmath.Abs(got.Float-1.5) > 1e-12 {
		t.Fatalf("Fmod = %#v, %v; want 1.5, true", got, ok)
	}
	if _, ok := ToInteger(Float(5.5)); ok {
		t.Fatal("ToInteger accepted non-integral float")
	}
}
