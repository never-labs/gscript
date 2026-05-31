package bits

import "testing"

func TestBitsBitwise(t *testing.T) {
	tests := []struct {
		name string
		got  int64
		want int64
	}{
		{name: "and empty", got: And(), want: -1},
		{name: "and many", got: And(255, 15, 7), want: 7},
		{name: "or empty", got: Or(), want: 0},
		{name: "or many", got: Or(240, 15), want: 255},
		{name: "xor empty", got: Xor(), want: 0},
		{name: "xor many", got: Xor(255, 15), want: 240},
		{name: "not", got: Not(0), want: -1},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestBitsShifts(t *testing.T) {
	tests := []struct {
		name string
		got  int64
		want int64
	}{
		{name: "shl", got: Shl(1, 8), want: 256},
		{name: "shl 64", got: Shl(1, 64), want: 0},
		{name: "shr", got: Shr(256, 8), want: 1},
		{name: "shr 64", got: Shr(-1, 64), want: 0},
		{name: "sar negative", got: Sar(-8, 1), want: -4},
		{name: "sar 64 negative", got: Sar(-8, 64), want: -1},
		{name: "sar 64 positive", got: Sar(8, 64), want: 0},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestBitsRotates(t *testing.T) {
	tests := []struct {
		name string
		got  int64
		want int64
	}{
		{name: "rotl", got: Rotl(1, 4), want: 16},
		{name: "rotr", got: Rotr(16, 4), want: 1},
		{name: "rotl negative", got: Rotl(16, -4), want: 1},
		{name: "rotr negative", got: Rotr(1, -4), want: 16},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestBitsPositionsAndCounts(t *testing.T) {
	if !Test(10, 1) {
		t.Fatalf("Test(10, 1) = false, want true")
	}
	if Test(10, 2) {
		t.Fatalf("Test(10, 2) = true, want false")
	}

	checks := map[string]int64{
		"set":    Set(0, 3),
		"clear":  Clear(15, 1),
		"toggle": Toggle(8, 3),
	}
	wants := map[string]int64{
		"set":    8,
		"clear":  13,
		"toggle": 0,
	}
	for name, got := range checks {
		if got != wants[name] {
			t.Fatalf("%s = %d, want %d", name, got, wants[name])
		}
	}

	if got := Ones(255); got != 8 {
		t.Fatalf("Ones(255) = %d, want 8", got)
	}
	if got := LeadingZeros(1); got != 63 {
		t.Fatalf("LeadingZeros(1) = %d, want 63", got)
	}
	if got := TrailingZeros(16); got != 4 {
		t.Fatalf("TrailingZeros(16) = %d, want 4", got)
	}
}
