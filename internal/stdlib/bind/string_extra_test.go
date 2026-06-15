package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestStringTrim(t *testing.T) {
	interp := runProgram(t, `
		a := string.trim("  hello  ")
		b := string.trim("xxhelloxx", "x")
	`)
	if interp.GetGlobal("a").Str() != "hello" {
		t.Errorf("expected 'hello', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "hello" {
		t.Errorf("expected 'hello', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestStringPureBaseAdapters(t *testing.T) {
	interp := runProgram(t, `
		upper := string.upper("héllo")
		lower := string.lower("HÉLLO")
		reverse := string.reverse("ab界")
		repeated := string.rep("ha", 3, "-")
		replaced := string.replaceAll("a-b-a", "a", "x")
		titled := string.title("hello world")
		left := string.padLeft("go", 5, "0")
		right := string.padRight("go", 5, "ab")
		numeric := string.isNumeric(" 3.14 ")
	`)
	checks := map[string]string{
		"upper":    "HÉLLO",
		"lower":    "héllo",
		"reverse":  "界ba",
		"repeated": "ha-ha-ha",
		"replaced": "x-b-x",
		"titled":   "Hello World",
		"left":     "000go",
		"right":    "goaba",
	}
	for name, want := range checks {
		if got := interp.GetGlobal(name).Str(); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
	if !interp.GetGlobal("numeric").Bool() {
		t.Fatal("numeric = false, want true")
	}
}

func TestStringTrimLeft(t *testing.T) {
	interp := runProgram(t, `
		a := string.trimLeft("  hello  ")
		b := string.trimLeft("xxhello", "x")
	`)
	if interp.GetGlobal("a").Str() != "hello  " {
		t.Errorf("expected 'hello  ', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "hello" {
		t.Errorf("expected 'hello', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestStringTrimRight(t *testing.T) {
	interp := runProgram(t, `
		a := string.trimRight("  hello  ")
		b := string.trimRight("helloxx", "x")
	`)
	if interp.GetGlobal("a").Str() != "  hello" {
		t.Errorf("expected '  hello', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "hello" {
		t.Errorf("expected 'hello', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestStringSplitEmpty(t *testing.T) {
	interp := runProgram(t, `
		parts := string.split("abc", "")
	`)
	tbl := interp.GetGlobal("parts").Table()
	if tbl.Length() != 3 {
		t.Errorf("expected 3 parts, got %d", tbl.Length())
	}
	if tbl.RawGet(runtime.IntValue(1)).Str() != "a" {
		t.Errorf("expected 'a', got '%s'", tbl.RawGet(runtime.IntValue(1)).Str())
	}
	if tbl.RawGet(runtime.IntValue(2)).Str() != "b" {
		t.Errorf("expected 'b', got '%s'", tbl.RawGet(runtime.IntValue(2)).Str())
	}
	if tbl.RawGet(runtime.IntValue(3)).Str() != "c" {
		t.Errorf("expected 'c', got '%s'", tbl.RawGet(runtime.IntValue(3)).Str())
	}
}

func TestStringHasPrefix(t *testing.T) {
	interp := runProgram(t, `
		a := string.hasPrefix("hello world", "hello")
		b := string.hasPrefix("hello world", "world")
	`)
	if !interp.GetGlobal("a").Bool() {
		t.Errorf("expected true for hasPrefix 'hello'")
	}
	if interp.GetGlobal("b").Bool() {
		t.Errorf("expected false for hasPrefix 'world'")
	}
}

func TestStringHasSuffix(t *testing.T) {
	interp := runProgram(t, `
		a := string.hasSuffix("hello world", "world")
		b := string.hasSuffix("hello world", "hello")
	`)
	if !interp.GetGlobal("a").Bool() {
		t.Errorf("expected true for hasSuffix 'world'")
	}
	if interp.GetGlobal("b").Bool() {
		t.Errorf("expected false for hasSuffix 'hello'")
	}
}

func TestStringContains(t *testing.T) {
	interp := runProgram(t, `
		a := string.contains("hello world", "lo wo")
		b := string.contains("hello world", "xyz")
	`)
	if !interp.GetGlobal("a").Bool() {
		t.Errorf("expected true for contains 'lo wo'")
	}
	if interp.GetGlobal("b").Bool() {
		t.Errorf("expected false for contains 'xyz'")
	}
}

func TestStringCount(t *testing.T) {
	interp := runProgram(t, `
		a := string.count("hello", "l")
		b := string.count("banana", "na")
	`)
	if interp.GetGlobal("a").Int() != 2 {
		t.Errorf("expected 2, got %d", interp.GetGlobal("a").Int())
	}
	if interp.GetGlobal("b").Int() != 2 {
		t.Errorf("expected 2, got %d", interp.GetGlobal("b").Int())
	}
}

func TestStringReplaceAll(t *testing.T) {
	interp := runProgram(t, `
		result := string.replaceAll("hello world", "o", "0")
	`)
	if interp.GetGlobal("result").Str() != "hell0 w0rld" {
		t.Errorf("expected 'hell0 w0rld', got '%s'", interp.GetGlobal("result").Str())
	}
}

func TestStringJoin(t *testing.T) {
	interp := runProgram(t, `
		result := string.join(["a", "b", "c"], ", ")
	`)
	if interp.GetGlobal("result").Str() != "a, b, c" {
		t.Errorf("expected 'a, b, c', got '%s'", interp.GetGlobal("result").Str())
	}
}

func TestStringTitle(t *testing.T) {
	interp := runProgram(t, `
		result := string.title("hello world foo")
	`)
	if interp.GetGlobal("result").Str() != "Hello World Foo" {
		t.Errorf("expected 'Hello World Foo', got '%s'", interp.GetGlobal("result").Str())
	}
}

func TestStringPadLeft(t *testing.T) {
	interp := runProgram(t, `
		a := string.padLeft("hi", 5)
		b := string.padLeft("hi", 5, "0")
	`)
	if interp.GetGlobal("a").Str() != "   hi" {
		t.Errorf("expected '   hi', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "000hi" {
		t.Errorf("expected '000hi', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestStringPadRight(t *testing.T) {
	interp := runProgram(t, `
		a := string.padRight("hi", 5)
		b := string.padRight("hi", 5, "0")
	`)
	if interp.GetGlobal("a").Str() != "hi   " {
		t.Errorf("expected 'hi   ', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "hi000" {
		t.Errorf("expected 'hi000', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestStringRepeatAlias(t *testing.T) {
	interp := runProgram(t, `
		a := string.repeat("ab", 3)
		b := string.repeat("x", 0)
	`)
	if interp.GetGlobal("a").Str() != "ababab" {
		t.Errorf("expected 'ababab', got '%s'", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "" {
		t.Errorf("expected '', got '%s'", interp.GetGlobal("b").Str())
	}
}

func TestStringIsNumeric(t *testing.T) {
	interp := runProgram(t, `
		a := string.isNumeric("123")
		b := string.isNumeric("3.14")
		c := string.isNumeric("-42")
		d := string.isNumeric("hello")
		e := string.isNumeric("")
		f := string.isNumeric("1e10")
	`)
	if !interp.GetGlobal("a").Bool() {
		t.Errorf("expected true for '123'")
	}
	if !interp.GetGlobal("b").Bool() {
		t.Errorf("expected true for '3.14'")
	}
	if !interp.GetGlobal("c").Bool() {
		t.Errorf("expected true for '-42'")
	}
	if interp.GetGlobal("d").Bool() {
		t.Errorf("expected false for 'hello'")
	}
	if interp.GetGlobal("e").Bool() {
		t.Errorf("expected false for ''")
	}
	if !interp.GetGlobal("f").Bool() {
		t.Errorf("expected true for '1e10'")
	}
}

func TestLuaPatternEscapeCompatibility(t *testing.T) {
	interp := runProgram(t, `
		hex := string.match("0alo alo", "%x*")
		nothex := string.match("0alo alo", "%X+")
		nul := string.match("a" .. string.char(0) .. "b", "%z")
		notnul := string.match("a" .. string.char(0) .. "b", "%Z+")
		word := string.match("-", "[%W]")
		notword := string.match("abc-123", "[^%W]+")
		line := string.match("a\nb", "a.b")
	`)
	if got := interp.GetGlobal("hex").Str(); got != "0a" {
		t.Fatalf("hex = %q, want 0a", got)
	}
	if got := interp.GetGlobal("nothex").Str(); got != "lo " {
		t.Fatalf("nothex = %q, want lo ", got)
	}
	if got := interp.GetGlobal("nul").Str(); got != "\x00" {
		t.Fatalf("nul = %q, want NUL", got)
	}
	if got := interp.GetGlobal("notnul").Str(); got != "a" {
		t.Fatalf("notnul = %q, want a", got)
	}
	if got := interp.GetGlobal("word").Str(); got != "-" {
		t.Fatalf("word = %q, want -", got)
	}
	if got := interp.GetGlobal("notword").Str(); got != "abc" {
		t.Fatalf("notword = %q, want abc", got)
	}
	if got := interp.GetGlobal("line").Str(); got != "a\nb" {
		t.Fatalf("line = %q, want a\\nb", got)
	}
}

func TestLuaBalancedPatternStandalone(t *testing.T) {
	interp := runProgram(t, `
		a := string.match("a (b (c) d) z", "%b()")
		b, n := string.gsub("alo 'oi' alo", "%b''", "\"")
		c, m := string.gsub("a (b (c) d) z", "%b()", "")
	`)
	if got := interp.GetGlobal("a").Str(); got != "(b (c) d)" {
		t.Fatalf("balanced match = %q, want nested parens", got)
	}
	if got := interp.GetGlobal("b").Str(); got != `alo " alo` {
		t.Fatalf("quote gsub = %q", got)
	}
	if got := interp.GetGlobal("n").Int(); got != 1 {
		t.Fatalf("quote count = %d, want 1", got)
	}
	if got := interp.GetGlobal("c").Str(); got != "a  z" {
		t.Fatalf("paren gsub = %q", got)
	}
	if got := interp.GetGlobal("m").Int(); got != 1 {
		t.Fatalf("paren count = %d, want 1", got)
	}
}

func TestLuaPatternFrontierCompatibility(t *testing.T) {
	interp := runProgram(t, `
		first := string.match("abc def", "%f[%w]%w+")
		second := string.match("abc,def", "%w+%f[%W]")
		i, e := string.find("abc123 def", "%f[%d]%d+")
		atEndStart, atEndEnd := string.find("abc", "%f[%z]")
		replaced, count := string.gsub("one two", "%f[%w](%w+)", "[%1]")
		replacedWhole, wholeCount := string.gsub("a b", "%f[%w]%w", "%1%0")
		words := {}
		for w := range string.gmatch("a-b c", "%f[%w]%w+%f[%W]") {
			words[#words + 1] = w
		}
	`)
	if got := interp.GetGlobal("first").Str(); got != "abc" {
		t.Fatalf("frontier first = %q, want abc", got)
	}
	if got := interp.GetGlobal("second").Str(); got != "abc" {
		t.Fatalf("frontier second = %q, want abc", got)
	}
	if got := interp.GetGlobal("i").Int(); got != 4 {
		t.Fatalf("digit frontier start = %d, want 4", got)
	}
	if got := interp.GetGlobal("e").Int(); got != 6 {
		t.Fatalf("digit frontier end = %d, want 6", got)
	}
	if got := interp.GetGlobal("atEndStart").Int(); got != 4 {
		t.Fatalf("end frontier start = %d, want 4", got)
	}
	if got := interp.GetGlobal("atEndEnd").Int(); got != 3 {
		t.Fatalf("end frontier end = %d, want 3", got)
	}
	if got := interp.GetGlobal("replaced").Str(); got != "[one] [two]" {
		t.Fatalf("frontier gsub = %q, want [one] [two]", got)
	}
	if got := interp.GetGlobal("count").Int(); got != 2 {
		t.Fatalf("frontier gsub count = %d, want 2", got)
	}
	if got := interp.GetGlobal("replacedWhole").Str(); got != "aa bb" {
		t.Fatalf("frontier whole replacement = %q, want aa bb", got)
	}
	if got := interp.GetGlobal("wholeCount").Int(); got != 2 {
		t.Fatalf("frontier whole replacement count = %d, want 2", got)
	}
	words := interp.GetGlobal("words").Table()
	if got := words.RawGet(runtime.IntValue(1)).Str(); got != "a" {
		t.Fatalf("word 1 = %q, want a", got)
	}
	if got := words.RawGet(runtime.IntValue(2)).Str(); got != "b" {
		t.Fatalf("word 2 = %q, want b", got)
	}
	if got := words.RawGet(runtime.IntValue(3)).Str(); got != "c" {
		t.Fatalf("word 3 = %q, want c", got)
	}
}
