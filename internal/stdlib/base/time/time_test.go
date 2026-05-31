package time

import "testing"

func TestLayoutFromStrftimePassesGoLayoutThrough(t *testing.T) {
	layout := "2006-01-02T15:04:05Z07:00"
	if got := LayoutFromStrftime(layout); got != layout {
		t.Fatalf("LayoutFromStrftime(%q) = %q", layout, got)
	}
}

func TestLayoutFromStrftimeConvertsCommonDirectives(t *testing.T) {
	got := LayoutFromStrftime("%Y-%m-%d %H:%M:%S %A %B %Z %%")
	want := "2006-01-02 15:04:05 Monday January MST %"
	if got != want {
		t.Fatalf("LayoutFromStrftime = %q, want %q", got, want)
	}
}

func TestLayoutFromStrftimeLeavesUnknownDirectives(t *testing.T) {
	got := LayoutFromStrftime("%Y/%j")
	want := "2006/%j"
	if got != want {
		t.Fatalf("LayoutFromStrftime unknown directive = %q, want %q", got, want)
	}
}
