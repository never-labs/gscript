package stringlib

import "testing"

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
