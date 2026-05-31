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
