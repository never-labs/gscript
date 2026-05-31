package stringpattern

import "testing"

func TestScanASCIIDigits(t *testing.T) {
	tests := []struct {
		name string
		s    string
		pos  int
		want int
	}{
		{name: "at start", s: "123abc", pos: 0, want: 3},
		{name: "from middle", s: "xx123abc", pos: 2, want: 5},
		{name: "no digit", s: "abc", pos: 0, want: 0},
		{name: "negative clamps", s: "12", pos: -3, want: 2},
		{name: "past end", s: "12", pos: 4, want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ScanASCIIDigits(tt.s, tt.pos); got != tt.want {
				t.Fatalf("ScanASCIIDigits(%q, %d) = %d, want %d", tt.s, tt.pos, got, tt.want)
			}
		})
	}
}

func TestHasStringAt(t *testing.T) {
	if !HasStringAt("abcde", 1, "bc") {
		t.Fatalf("expected match")
	}
	if HasStringAt("abcde", -1, "a") {
		t.Fatalf("negative offset must not match")
	}
	if HasStringAt("abcde", 4, "de") {
		t.Fatalf("out of range match")
	}
	if !HasStringAt("abcde", 5, "") {
		t.Fatalf("empty suffix at end should match")
	}
}

func TestNextSearchStart(t *testing.T) {
	tests := []struct {
		name       string
		start, end int
		want       int
	}{
		{name: "non empty", start: 2, end: 5, want: 5},
		{name: "zero width", start: 2, end: 2, want: 3},
		{name: "zero width at end", start: 3, end: 3, want: 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NextSearchStart("abc", tt.start, tt.end); got != tt.want {
				t.Fatalf("NextSearchStart = %d, want %d", got, tt.want)
			}
		})
	}
}
