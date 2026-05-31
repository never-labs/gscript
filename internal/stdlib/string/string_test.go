package stringlib

import (
	"strings"
	"testing"
)

func TestStringTransforms(t *testing.T) {
	if got := Upper("héllo"); got != "HÉLLO" {
		t.Fatalf("Upper = %q", got)
	}
	if got := Lower("HÉLLO"); got != "héllo" {
		t.Fatalf("Lower = %q", got)
	}
	if got := Reverse("ab界"); got != "界ba" {
		t.Fatalf("Reverse = %q", got)
	}
	if got := RepeatJoin("ha", 3, "-"); got != "ha-ha-ha" {
		t.Fatalf("RepeatJoin = %q", got)
	}
}

func TestTrimAndPredicates(t *testing.T) {
	if got := TrimSpace("\t hi \n"); got != "hi" {
		t.Fatalf("TrimSpace = %q", got)
	}
	if got := Trim("..hi..", "."); got != "hi" {
		t.Fatalf("Trim = %q", got)
	}
	if !HasPrefix("gscript", "gs") || !HasSuffix("gscript", "script") || !Contains("gscript", "cri") {
		t.Fatal("string predicates failed")
	}
	if got := Count("banana", "an"); got != 2 {
		t.Fatalf("Count = %d", got)
	}
}

func TestReplaceTitlePadNumeric(t *testing.T) {
	if got := ReplaceAll("a-b-a", "a", "x"); got != "x-b-x" {
		t.Fatalf("ReplaceAll = %q", got)
	}
	if got := Title("hello world"); got != "Hello World" {
		t.Fatalf("Title = %q", got)
	}
	if got := PadLeft("go", 5, "0"); got != "000go" {
		t.Fatalf("PadLeft = %q", got)
	}
	if got := PadRight("go", 5, "ab"); got != "goaba" {
		t.Fatalf("PadRight = %q", got)
	}
	if !IsNumeric(" 3.14 ") || IsNumeric(" ") || IsNumeric("no") {
		t.Fatal("IsNumeric failed")
	}
}

func TestLuaSub(t *testing.T) {
	cases := []struct {
		name   string
		s      string
		start  int64
		end    int64
		hasEnd bool
		want   string
	}{
		{name: "middle", s: "abcdef", start: 2, end: 4, hasEnd: true, want: "bcd"},
		{name: "open end", s: "abcdef", start: 4, want: "def"},
		{name: "negative start", s: "abcdef", start: -3, want: "def"},
		{name: "negative end", s: "abcdef", start: 2, end: -2, hasEnd: true, want: "bcde"},
		{name: "empty range", s: "abcdef", start: 5, end: 2, hasEnd: true, want: ""},
		{name: "clamp", s: "abcdef", start: -99, end: 99, hasEnd: true, want: "abcdef"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := LuaSub(tc.s, tc.start, tc.end, tc.hasEnd); got != tc.want {
				t.Fatalf("LuaSub = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestLuaByteRange(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		start, end int64
		hasEnd     bool
		wantStart  int
		wantEnd    int
		wantOK     bool
	}{
		{name: "single default", s: "abc", start: 2, wantStart: 1, wantEnd: 1, wantOK: true},
		{name: "negative", s: "abc", start: -2, wantStart: 1, wantEnd: 1, wantOK: true},
		{name: "clamped range", s: "abc", start: -9, end: 9, hasEnd: true, wantStart: 0, wantEnd: 2, wantOK: true},
		{name: "empty", s: "abc", start: 4, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, ok := LuaByteRange(tt.s, tt.start, tt.end, tt.hasEnd)
			if start != tt.wantStart || end != tt.wantEnd || ok != tt.wantOK {
				t.Fatalf("LuaByteRange = (%d, %d, %v), want (%d, %d, %v)", start, end, ok, tt.wantStart, tt.wantEnd, tt.wantOK)
			}
		})
	}
}

func TestLuaByteAt(t *testing.T) {
	if b, ok := LuaByteAt("abc", -1); !ok || b != 'c' {
		t.Fatalf("LuaByteAt negative = (%q, %v), want c,true", b, ok)
	}
	if _, ok := LuaByteAt("", 1); ok {
		t.Fatalf("LuaByteAt empty string succeeded")
	}
}

func TestLuaBytes(t *testing.T) {
	buf, ok := LuaBytes("abcd", 2, -1, true)
	if !ok || string(buf) != "bcd" {
		t.Fatalf("LuaBytes = %q,%v, want bcd,true", string(buf), ok)
	}
	if _, ok := LuaBytes("abc", 4, 4, false); ok {
		t.Fatalf("LuaBytes accepted empty range")
	}
}

func TestLuaSearchStart(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		init       int64
		wantStart  int
		wantSearch string
		wantOK     bool
	}{
		{name: "default", s: "abcdef", init: 1, wantStart: 1, wantSearch: "abcdef", wantOK: true},
		{name: "middle", s: "abcdef", init: 3, wantStart: 3, wantSearch: "cdef", wantOK: true},
		{name: "negative", s: "abcdef", init: -2, wantStart: 5, wantSearch: "ef", wantOK: true},
		{name: "clamped low", s: "abcdef", init: -99, wantStart: 1, wantSearch: "abcdef", wantOK: true},
		{name: "end plus one", s: "abcdef", init: 7, wantStart: 7, wantSearch: "", wantOK: true},
		{name: "past end", s: "abcdef", init: 8, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, search, ok := LuaSearchStart(tt.s, tt.init)
			if start != tt.wantStart || search != tt.wantSearch || ok != tt.wantOK {
				t.Fatalf("LuaSearchStart = (%d, %q, %v), want (%d, %q, %v)", start, search, ok, tt.wantStart, tt.wantSearch, tt.wantOK)
			}
		})
	}
}

func TestCharBytes(t *testing.T) {
	buf, ok := CharBytes([]int64{65, 66, 67})
	if !ok || string(buf) != "ABC" {
		t.Fatalf("CharBytes = %q,%v, want ABC,true", string(buf), ok)
	}
	if _, ok := CharBytes([]int64{256}); ok {
		t.Fatalf("CharBytes accepted out-of-range byte")
	}
}

func TestSplitHelpers(t *testing.T) {
	if got := Split("a,b,,c", ","); !sameStrings(got, []string{"a", "b", "", "c"}) {
		t.Fatalf("Split comma = %#v", got)
	}
	if got := Split("ab", ""); !sameStrings(got, []string{"a", "b"}) {
		t.Fatalf("Split empty = %#v", got)
	}
	if got := Split("a--b--", "--"); !sameStrings(got, []string{"a", "b", ""}) {
		t.Fatalf("Split multi = %#v", got)
	}
	var each []string
	SplitEach("a|b|", "|", func(part string) {
		each = append(each, part)
	})
	if !sameStrings(each, []string{"a", "b", ""}) {
		t.Fatalf("SplitEach = %#v", each)
	}
	if got, ok := SplitProject("a,b,c", ",", 2); !ok || got != "b" {
		t.Fatalf("SplitProject = %q, %v", got, ok)
	}
	if got, ok := SplitProject("ab", "", 2); !ok || got != "b" {
		t.Fatalf("SplitProject empty = %q, %v", got, ok)
	}
	if _, ok := SplitProject("a,b", ",", 3); ok {
		t.Fatal("SplitProject out of range returned ok")
	}
}

func TestJoinProjectedLen(t *testing.T) {
	if got := JoinProjectedLen([]string{"a", "bb", "ccc"}, "::"); got != len("a::bb::ccc") {
		t.Fatalf("JoinProjectedLen = %d", got)
	}
	if got := JoinProjectedLen([]string{"single"}, "::"); got != len("single") {
		t.Fatalf("JoinProjectedLen single = %d", got)
	}
	if got := JoinProjectedLen(nil, "::"); got != 0 {
		t.Fatalf("JoinProjectedLen nil = %d", got)
	}
}

func TestFormatHelpers(t *testing.T) {
	for _, b := range []byte{'-', '+', ' ', '#', '0'} {
		if !IsFormatFlag(b) {
			t.Fatalf("IsFormatFlag(%q) = false", b)
		}
	}
	if IsFormatFlag('9') {
		t.Fatal("IsFormatFlag accepted digit")
	}

	var buf strings.Builder
	if !WriteFastIntegerFormat(&buf, "%05d", 'd', -12) {
		t.Fatal("WriteFastIntegerFormat rejected %05d")
	}
	if got := buf.String(); got != "-0012" {
		t.Fatalf("WriteFastIntegerFormat = %q", got)
	}
	buf.Reset()
	WritePaddedInteger(&buf, 'X', ' ', 4, 0x2a)
	if got := buf.String(); got != "  2A" {
		t.Fatalf("WritePaddedInteger = %q", got)
	}
}

func TestLuaQuoteHelpers(t *testing.T) {
	if got := LuaQuoteNil(); got != "nil" {
		t.Fatalf("LuaQuoteNil = %q", got)
	}
	if got := LuaQuoteBool(true); got != "true" {
		t.Fatalf("LuaQuoteBool = %q", got)
	}
	if got := LuaQuoteInt(-42); got != "-42" {
		t.Fatalf("LuaQuoteInt = %q", got)
	}
	if got := LuaQuoteFloat(1.25); got != "1.25" {
		t.Fatalf("LuaQuoteFloat = %q", got)
	}
	if got := LuaQuoteString("a\n\"b\\"); got != `"a\n\"b\\"` {
		t.Fatalf("LuaQuoteString = %q", got)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
