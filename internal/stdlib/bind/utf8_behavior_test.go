package bind

import (
	"strings"
	"testing"
)

func TestUTF8Len(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.len("hello")
		b := utf8.len("中文")
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	if a.Int() != 5 {
		t.Errorf("expected 5, got %v", a)
	}
	if b.Int() != 2 {
		t.Errorf("expected 2 for '中文', got %v", b)
	}
}

func TestUTF8Len_invalid(t *testing.T) {
	interp := runProgram(t, `
		bad := string.char(255, 254)
		a, err := utf8.len(bad)
	`)
	a := interp.GetGlobal("a")
	err := interp.GetGlobal("err")
	if !a.IsNil() {
		t.Errorf("expected nil for invalid UTF-8, got %v", a)
	}
	if !err.IsInt() || err.Int() != 1 {
		t.Errorf("expected invalid byte position 1, got %v", err)
	}
}

func TestUTF8Char(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.char(72, 101, 108, 108, 111)
		b := utf8.char(20013, 25991)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	if a.Str() != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", a.Str())
	}
	if b.Str() != "中文" {
		t.Errorf("expected '中文', got '%s'", b.Str())
	}
}

func TestUTF8CharRejectsInvalidUnicodeScalars(t *testing.T) {
	if err := runProgramExpectError(t, `utf8.char(0x110000)`); err == nil || !strings.Contains(err.Error(), "value out of range") {
		t.Fatalf("utf8.char(0x110000) error = %v, want value out of range", err)
	}
	if err := runProgramExpectError(t, `utf8.char(0xD800)`); err == nil || !strings.Contains(err.Error(), "value out of range") {
		t.Fatalf("utf8.char(0xD800) error = %v, want value out of range", err)
	}
}

func TestUTF8Codepoint(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.codepoint("A", 1)
		b := utf8.codepoint("中文", 1)
		c := utf8.codepoint("中文", 4)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	c := interp.GetGlobal("c")
	if a.Int() != 65 {
		t.Errorf("expected 65 for 'A', got %v", a)
	}
	if b.Int() != 20013 {
		t.Errorf("expected 20013 for '中', got %v", b)
	}
	if c.Int() != 25991 {
		t.Errorf("expected 25991 for '文', got %v", c)
	}
}

func TestUTF8Codepoint_range(t *testing.T) {
	interp := runProgram(t, `
		a, b, c := utf8.codepoint("ABC", 1, 3)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	c := interp.GetGlobal("c")
	if a.Int() != 65 {
		t.Errorf("expected 65, got %v", a)
	}
	if b.Int() != 66 {
		t.Errorf("expected 66, got %v", b)
	}
	if c.Int() != 67 {
		t.Errorf("expected 67, got %v", c)
	}
}

func TestUTF8Codes(t *testing.T) {
	interp := runProgram(t, `
		result := {}
		for p, c := range utf8.codes("AB") {
			result[#result + 1] = {pos: p, code: c}
		}
	`)
	tbl := interp.GetGlobal("result").Table()
	if tbl.Length() != 2 {
		t.Errorf("expected 2 entries, got %d", tbl.Length())
	}
	first := tbl.RawGet(IntValue(1)).Table()
	if first.RawGet(StringValue("pos")).Int() != 1 {
		t.Errorf("expected pos=1, got %v", first.RawGet(StringValue("pos")))
	}
	if first.RawGet(StringValue("code")).Int() != 65 {
		t.Errorf("expected code=65, got %v", first.RawGet(StringValue("code")))
	}
	second := tbl.RawGet(IntValue(2)).Table()
	if second.RawGet(StringValue("pos")).Int() != 2 {
		t.Errorf("expected pos=2, got %v", second.RawGet(StringValue("pos")))
	}
	if second.RawGet(StringValue("code")).Int() != 66 {
		t.Errorf("expected code=66, got %v", second.RawGet(StringValue("code")))
	}
}

func TestUTF8Offset(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.offset("中文测试", 2)
		b := utf8.offset("ABC", 3)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	if a.Int() != 4 {
		t.Errorf("expected 4, got %v", a)
	}
	if b.Int() != 3 {
		t.Errorf("expected 3, got %v", b)
	}
}

func TestUTF8Valid(t *testing.T) {
	interp := runProgram(t, `
		bad := string.char(255, 254)
		a := utf8.valid("hello")
		b := utf8.valid("中文")
		c := utf8.valid(bad)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	c := interp.GetGlobal("c")
	if !a.Bool() {
		t.Errorf("expected true for 'hello'")
	}
	if !b.Bool() {
		t.Errorf("expected true for '中文'")
	}
	if c.Bool() {
		t.Errorf("expected false for invalid UTF-8")
	}
}

func TestUTF8ValidateAndSanitizeInvalidEdges(t *testing.T) {
	interp := runProgram(t, `
		valid := utf8.validate("A" .. utf8.char(0x1F600))
		truncated := utf8.validate("ok" .. string.char(0xE2))
		overlong := utf8.validate(string.char(0xC0, 0x80))
		surrogate := utf8.validate(string.char(0xED, 0xA0, 0x80))
		limit := utf8.validate(string.char(0xF4, 0x90, 0x80, 0x80))
		clean := utf8.sanitize("a" .. string.char(0x80) .. "b")
		custom := utf8.sanitize(string.char(0xC0, 0x80), "?")
	`)
	if got := interp.GetGlobal("valid").Table(); !got.RawGetString("valid").Bool() || got.RawGetString("runeCount").Int() != 2 {
		t.Fatalf("valid report = %v, want valid with runeCount=2", got)
	}
	if got := interp.GetGlobal("truncated").Table(); got.RawGetString("valid").Bool() || got.RawGetString("pos").Int() != 3 {
		t.Fatalf("truncated report pos = %v, want invalid at byte 3", got.RawGetString("pos"))
	}
	for _, name := range []string{"overlong", "surrogate", "limit"} {
		if got := interp.GetGlobal(name).Table(); got.RawGetString("valid").Bool() || got.RawGetString("pos").Int() != 1 {
			t.Fatalf("%s report = %v, want invalid at byte 1", name, got)
		}
	}
	if got := interp.GetGlobal("clean").Str(); got != "a\uFFFDb" {
		t.Fatalf("clean = %q, want replacement char", got)
	}
	if got := interp.GetGlobal("custom").Str(); got != "??" {
		t.Fatalf("custom = %q, want two custom replacements", got)
	}
}

func TestUTF8Reverse(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.reverse("abc")
		b := utf8.reverse("中文")
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	if a.Str() != "cba" {
		t.Errorf("expected 'cba', got '%s'", a.Str())
	}
	if b.Str() != "文中" {
		t.Errorf("expected '文中', got '%s'", b.Str())
	}
}

func TestUTF8Sub(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.sub("hello", 2, 4)
		b := utf8.sub("中文测试", 2, 3)
		c := utf8.sub("hello", 3)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	c := interp.GetGlobal("c")
	if a.Str() != "ell" {
		t.Errorf("expected 'ell', got '%s'", a.Str())
	}
	if b.Str() != "文测" {
		t.Errorf("expected '文测', got '%s'", b.Str())
	}
	if c.Str() != "llo" {
		t.Errorf("expected 'llo', got '%s'", c.Str())
	}
}

func TestUTF8Upper(t *testing.T) {
	interp := runProgram(t, `
		result := utf8.upper("hello")
	`)
	v := interp.GetGlobal("result")
	if v.Str() != "HELLO" {
		t.Errorf("expected 'HELLO', got '%s'", v.Str())
	}
}

func TestUTF8Lower(t *testing.T) {
	interp := runProgram(t, `
		result := utf8.lower("HELLO")
	`)
	v := interp.GetGlobal("result")
	if v.Str() != "hello" {
		t.Errorf("expected 'hello', got '%s'", v.Str())
	}
}

func TestUTF8Charclass(t *testing.T) {
	interp := runProgram(t, `
		a := utf8.charclass(65)
		b := utf8.charclass(48)
		c := utf8.charclass(32)
		d := utf8.charclass(33)
	`)
	a := interp.GetGlobal("a")
	b := interp.GetGlobal("b")
	c := interp.GetGlobal("c")
	d := interp.GetGlobal("d")
	if a.Str() != "L" {
		t.Errorf("expected 'L' for letter, got '%s'", a.Str())
	}
	if b.Str() != "N" {
		t.Errorf("expected 'N' for number, got '%s'", b.Str())
	}
	if c.Str() != "S" {
		t.Errorf("expected 'S' for space, got '%s'", c.Str())
	}
	if d.Str() != "P" {
		t.Errorf("expected 'P' for punctuation, got '%s'", d.Str())
	}
}

func TestUTF8Charpattern(t *testing.T) {
	interp := runProgram(t, `
		result := utf8.charpattern
	`)
	v := interp.GetGlobal("result")
	if !v.IsString() || v.Str() == "" {
		t.Errorf("expected non-empty charpattern string, got %v", v)
	}
}
