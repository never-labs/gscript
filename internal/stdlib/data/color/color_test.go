package color

import (
	"math"
	"testing"
)

func close(a, b float64) bool {
	return math.Abs(a-b) < 1e-10
}

func TestFromHex(t *testing.T) {
	tests := []struct {
		input string
		want  RGBA
	}{
		{"#F00", RGBA{R: 1, G: 0, B: 0, A: 1}},
		{"#FF0000", RGBA{R: 1, G: 0, B: 0, A: 1}},
		{"#FF000080", RGBA{R: 1, G: 0, B: 0, A: 128.0 / 255.0}},
	}
	for _, tc := range tests {
		got, err := FromHex(tc.input)
		if err != nil {
			t.Fatalf("FromHex(%q): %v", tc.input, err)
		}
		if !close(got.R, tc.want.R) || !close(got.G, tc.want.G) || !close(got.B, tc.want.B) || !close(got.A, tc.want.A) {
			t.Fatalf("FromHex(%q) = %#v, want %#v", tc.input, got, tc.want)
		}
	}
}

func TestFromHexInvalid(t *testing.T) {
	if _, err := FromHex("invalid"); err == nil {
		t.Fatal("expected invalid hex to fail")
	}
}

func TestHexRoundTrip(t *testing.T) {
	got := ToHex(RGBA{R: 1, G: 128.0 / 255.0, B: 0, A: 1})
	if got != "#FF8000" {
		t.Fatalf("ToHex = %q", got)
	}
}

func TestNewAndEqual(t *testing.T) {
	c := New(0.1, 0.2, 0.3, 0.4)
	if !Equal(c, RGBA{R: 0.1, G: 0.2, B: 0.3, A: 0.4}) {
		t.Fatalf("New/Equal = %#v", c)
	}
	if Equal(c, WithAlpha(c, 0.5)) {
		t.Fatalf("Equal ignored alpha")
	}
}

func TestHSVAndHSL(t *testing.T) {
	red := FromHSV(0, 1, 1)
	if !close(red.R, 1) || !close(red.G, 0) || !close(red.B, 0) || !close(red.A, 1) {
		t.Fatalf("FromHSV red = %#v", red)
	}
	h, s, v := ToHSV(red)
	if !close(h, 0) || !close(s, 1) || !close(v, 1) {
		t.Fatalf("ToHSV red = %v, %v, %v", h, s, v)
	}

	hsl := FromHSL(120, 1, 0.25)
	h, s, l := ToHSL(hsl)
	if !close(h, 120) || !close(s, 1) || !close(l, 0.25) {
		t.Fatalf("HSL round trip = %v, %v, %v", h, s, l)
	}
}

func TestManipulation(t *testing.T) {
	a := RGBA{R: 0.2, G: 0.4, B: 0.6, A: 0.8}
	b := RGBA{R: 0.8, G: 0.4, B: 0.2, A: 0.5}

	if got := Add(a, b); !close(got.R, 1) || !close(got.A, 1) {
		t.Fatalf("Add = %#v", got)
	}
	if got := Scale(a, 2); !close(got.R, 0.4) || !close(got.A, 0.8) {
		t.Fatalf("Scale = %#v", got)
	}
	if got := Lerp(a, b, 0.5); !close(got.R, 0.5) || !close(got.G, 0.4) || !close(got.B, 0.4) {
		t.Fatalf("Lerp = %#v", got)
	}
	if got := WithAlpha(a, 0.25); !close(got.A, 0.25) {
		t.Fatalf("WithAlpha = %#v", got)
	}
}
