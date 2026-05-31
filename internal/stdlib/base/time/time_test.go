package time

import (
	"testing"
	gotime "time"
)

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

func TestLuaDateFormatConvertsRuntimeDirectives(t *testing.T) {
	tm := gotime.Date(2026, gotime.May, 31, 23, 4, 5, 0, gotime.UTC)
	got := LuaDateFormat("%Y-%m-%d %H:%M:%S %a %b %%", tm)
	want := "2026-05-31 23:04:05 Sun May %"
	if got != want {
		t.Fatalf("LuaDateFormat = %q, want %q", got, want)
	}
}
