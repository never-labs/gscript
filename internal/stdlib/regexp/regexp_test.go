package regexp

import (
	"reflect"
	stdregexp "regexp"
	"testing"
)

func TestFastFindStringDigitPatterns(t *testing.T) {
	for _, pattern := range []string{"[0-9]+", "\\d+"} {
		got, found, ok := FastFindString(pattern, "abc123def45")
		if !ok || !found || got != "123" {
			t.Fatalf("FastFindString(%q) = %q, %v, %v; want 123, true, true", pattern, got, found, ok)
		}

		got, found, ok = FastFindString(pattern, "abcdef")
		if !ok || found || got != "" {
			t.Fatalf("FastFindString(%q no match) = %q, %v, %v; want empty, false, true", pattern, got, found, ok)
		}
	}
}

func TestFastFindStringUnsupported(t *testing.T) {
	got, found, ok := FastFindString("[a-z]+", "abc123")
	if ok || found || got != "" {
		t.Fatalf("FastFindString unsupported = %q, %v, %v; want empty, false, false", got, found, ok)
	}
}

func TestCompileCachesRegexp(t *testing.T) {
	first, err := Compile("[a-z]+")
	if err != nil {
		t.Fatalf("Compile first: %v", err)
	}
	second, err := Compile("[a-z]+")
	if err != nil {
		t.Fatalf("Compile second: %v", err)
	}
	if first != second {
		t.Fatalf("Compile returned different pointers for same pattern")
	}
}

func TestCompileInvalidPattern(t *testing.T) {
	if _, err := Compile("[invalid"); err == nil {
		t.Fatalf("Compile invalid pattern returned nil error")
	}
}

func TestPureHelpersMatchRegexp(t *testing.T) {
	matched, err := Match("^hello", "hello world")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if !matched {
		t.Fatalf("Match = false, want true")
	}

	found, ok, err := Find("[0-9]+", "abc123")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if !ok || found != "123" {
		t.Fatalf("Find = %q, %v; want 123, true", found, ok)
	}

	found, ok, err = Find("^", "abc")
	if err != nil {
		t.Fatalf("Find empty match: %v", err)
	}
	if !ok || found != "" {
		t.Fatalf("Find empty match = %q, %v; want empty, true", found, ok)
	}

	if out, err := ReplaceFirst("[0-9]+", "a1b22", "X"); err != nil || out != "aXb22" {
		t.Fatalf("ReplaceFirst = %q, %v; want aXb22, nil", out, err)
	}

	if out, err := ReplaceAllString("[0-9]+", "a1b22", "X"); err != nil || out != "aXbX" {
		t.Fatalf("ReplaceAllString = %q, %v; want aXbX, nil", out, err)
	}

	parts, err := Split(",\\s*", "a, b, c", -1)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if want := []string{"a", "b", "c"}; !reflect.DeepEqual(parts, want) {
		t.Fatalf("Split = %#v, want %#v", parts, want)
	}
}

func TestFastFindAllStringsMatchesRegexp(t *testing.T) {
	for _, n := range []int{-1, 0, 1, 2, 10} {
		got, ok := FastFindAllStrings("[0-9]+", "a1b22c333", n)
		if !ok {
			t.Fatalf("FastFindAllStrings returned ok=false")
		}
		want := stdregexp.MustCompile("[0-9]+").FindAllString("a1b22c333", n)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("FastFindAllStrings n=%d = %#v, want %#v", n, got, want)
		}
	}
}

func TestFastReplaceAllStringMatchesRegexp(t *testing.T) {
	for _, input := range []string{"a1b22c333", "abcdef"} {
		got, ok := FastReplaceAllString("\\d+", input, "X")
		if !ok {
			t.Fatalf("FastReplaceAllString returned ok=false")
		}
		want := stdregexp.MustCompile("\\d+").ReplaceAllString(input, "X")
		if got != want {
			t.Fatalf("FastReplaceAllString(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestFastSplitStringsMatchesRegexp(t *testing.T) {
	input := " one\t two\nthree  four "
	for _, pattern := range []string{"\\s+", "[[:space:]]+"} {
		for _, n := range []int{-1, 0, 1, 2, 10} {
			got, ok := FastSplitStrings(pattern, input, n)
			if !ok {
				t.Fatalf("FastSplitStrings returned ok=false")
			}
			want := stdregexp.MustCompile(pattern).Split(input, n)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("FastSplitStrings(%q, n=%d) = %#v, want %#v", pattern, n, got, want)
			}
		}
	}
}

func TestFastFindSubmatchIndexKeyValue(t *testing.T) {
	input := "skip abc=def path=/bin bad= key=value2"
	pattern := "([a-z]+)=([a-z0-9/]+)"

	got, ok := FastFindSubmatchIndex(pattern, input)
	if !ok {
		t.Fatalf("FastFindSubmatchIndex returned ok=false")
	}
	want := stdregexp.MustCompile(pattern).FindStringSubmatchIndex(input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("FastFindSubmatchIndex = %#v, want %#v", got, want)
	}

	allGot, ok := FastFindAllSubmatchIndex(pattern, input, -1)
	if !ok {
		t.Fatalf("FastFindAllSubmatchIndex returned ok=false")
	}
	allWant := stdregexp.MustCompile(pattern).FindAllStringSubmatchIndex(input, -1)
	if !reflect.DeepEqual(allGot, allWant) {
		t.Fatalf("FastFindAllSubmatchIndex = %#v, want %#v", allGot, allWant)
	}
}
