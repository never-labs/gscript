package modules

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestStringLen(t *testing.T) {
	interp := runProgram(t, `
		result := string.len("hello")
	`)
	v := interp.GetGlobal("result")
	if v.Int() != 5 {
		t.Errorf("expected 5, got %v", v)
	}
}

func TestStringSub(t *testing.T) {
	tests := []struct {
		src    string
		expect string
	}{
		{`result := string.sub("hello", 2, 4)`, "ell"},
		{`result := string.sub("hello", 2)`, "ello"},
		{`result := string.sub("hello", -3)`, "llo"},
		{`result := string.sub("hello", 1, -2)`, "hell"},
		{`result := string.sub("hello", 3, 2)`, ""},
	}
	for _, tt := range tests {
		interp := runProgram(t, tt.src)
		v := interp.GetGlobal("result")
		if v.Str() != tt.expect {
			t.Errorf("%s: expected %q, got %q", tt.src, tt.expect, v.Str())
		}
	}
}

func TestStringUpperLower(t *testing.T) {
	interp := runProgram(t, `
		u := string.upper("hello")
		l := string.lower("WORLD")
	`)
	if interp.GetGlobal("u").Str() != "HELLO" {
		t.Errorf("expected HELLO")
	}
	if interp.GetGlobal("l").Str() != "world" {
		t.Errorf("expected world")
	}
}

func TestStringRep(t *testing.T) {
	interp := runProgram(t, `
		a := string.rep("ab", 3)
		b := string.rep("ab", 3, ",")
		c := string.rep("x", 0)
	`)
	if interp.GetGlobal("a").Str() != "ababab" {
		t.Errorf("expected ababab, got %s", interp.GetGlobal("a").Str())
	}
	if interp.GetGlobal("b").Str() != "ab,ab,ab" {
		t.Errorf("expected ab,ab,ab, got %s", interp.GetGlobal("b").Str())
	}
	if interp.GetGlobal("c").Str() != "" {
		t.Errorf("expected empty string, got %s", interp.GetGlobal("c").Str())
	}
}

func TestStringReverse(t *testing.T) {
	interp := runProgram(t, `result := string.reverse("hello")`)
	if interp.GetGlobal("result").Str() != "olleh" {
		t.Errorf("expected olleh, got %s", interp.GetGlobal("result").Str())
	}
}

func TestStringByteChar(t *testing.T) {
	interp := runProgram(t, `
		b := string.byte("A")
		c := string.char(65, 66, 67)
	`)
	if interp.GetGlobal("b").Int() != 65 {
		t.Errorf("expected 65, got %v", interp.GetGlobal("b"))
	}
	if interp.GetGlobal("c").Str() != "ABC" {
		t.Errorf("expected ABC, got %s", interp.GetGlobal("c").Str())
	}
}

func TestStringFindPlain(t *testing.T) {
	interp := runProgram(t, `
		s, e := string.find("hello world", "world", 1, true)
	`)
	s := interp.GetGlobal("s")
	e := interp.GetGlobal("e")
	if s.Int() != 7 {
		t.Errorf("expected start=7, got %v", s)
	}
	if e.Int() != 11 {
		t.Errorf("expected end=11, got %v", e)
	}
}

func TestStringFindNotFound(t *testing.T) {
	interp := runProgram(t, `
		s := string.find("hello", "xyz", 1, true)
	`)
	if !interp.GetGlobal("s").IsNil() {
		t.Errorf("expected nil for not found, got %v", interp.GetGlobal("s"))
	}
}

func TestStringFindPattern(t *testing.T) {
	interp := runProgram(t, `
		s, e := string.find("hello123", "%d+")
	`)
	s := interp.GetGlobal("s")
	e := interp.GetGlobal("e")
	if s.Int() != 6 {
		t.Errorf("expected start=6, got %v", s)
	}
	if e.Int() != 8 {
		t.Errorf("expected end=8, got %v", e)
	}
}

func TestStringFormatBasic(t *testing.T) {
	interp := runProgram(t, `
		result := string.format("hello %s, you are %d", "world", 42)
	`)
	v := interp.GetGlobal("result")
	if v.Str() != "hello world, you are 42" {
		t.Errorf("expected 'hello world, you are 42', got '%s'", v.Str())
	}
}

func TestStringFormatTypes(t *testing.T) {
	interp := runProgram(t, `
		a := string.format("%d", 42)
		b := string.format("%f", 3.14)
		c := string.format("%x", 255)
		d := string.format("%05d", 42)
		e := string.format("%%")
	`)
	if interp.GetGlobal("a").Str() != "42" {
		t.Errorf("%%d: expected 42, got %s", interp.GetGlobal("a").Str())
	}
	if !strings.HasPrefix(interp.GetGlobal("b").Str(), "3.14") {
		t.Errorf("%%f: expected 3.14..., got %s", interp.GetGlobal("b").Str())
	}
	if interp.GetGlobal("c").Str() != "ff" {
		t.Errorf("%%x: expected ff, got %s", interp.GetGlobal("c").Str())
	}
	if interp.GetGlobal("d").Str() != "00042" {
		t.Errorf("%%05d: expected 00042, got %s", interp.GetGlobal("d").Str())
	}
	if interp.GetGlobal("e").Str() != "%" {
		t.Errorf("%%%% expected %%, got %s", interp.GetGlobal("e").Str())
	}
}

func TestStringGsub(t *testing.T) {
	interp := runProgram(t, `
		result, count := string.gsub("hello world", "o", "0")
	`)
	if interp.GetGlobal("result").Str() != "hell0 w0rld" {
		t.Errorf("expected hell0 w0rld, got %s", interp.GetGlobal("result").Str())
	}
	if interp.GetGlobal("count").Int() != 2 {
		t.Errorf("expected count=2, got %v", interp.GetGlobal("count"))
	}
}

func TestStringGsubLimit(t *testing.T) {
	interp := runProgram(t, `
		result, count := string.gsub("aaa", "a", "b", 2)
	`)
	if interp.GetGlobal("result").Str() != "bba" {
		t.Errorf("expected bba, got %s", interp.GetGlobal("result").Str())
	}
	if interp.GetGlobal("count").Int() != 2 {
		t.Errorf("expected count=2, got %v", interp.GetGlobal("count"))
	}
}

func TestStringGmatch(t *testing.T) {
	interp := runProgram(t, `
		result := {}
		i := 1
		for w := range string.gmatch("hello world foo", "%a+") {
			result[i] = w
			i = i + 1
		}
	`)
	tbl := interp.GetGlobal("result").Table()
	if tbl.Length() != 3 {
		t.Errorf("expected 3 matches, got %d", tbl.Length())
	}
	if tbl.RawGet(runtime.IntValue(1)).Str() != "hello" {
		t.Errorf("expected 'hello', got '%s'", tbl.RawGet(runtime.IntValue(1)).Str())
	}
	if tbl.RawGet(runtime.IntValue(2)).Str() != "world" {
		t.Errorf("expected 'world', got '%s'", tbl.RawGet(runtime.IntValue(2)).Str())
	}
	if tbl.RawGet(runtime.IntValue(3)).Str() != "foo" {
		t.Errorf("expected 'foo', got '%s'", tbl.RawGet(runtime.IntValue(3)).Str())
	}
}

func TestStringMatch(t *testing.T) {
	interp := runProgram(t, `
		result := string.match("hello123world", "%d+")
	`)
	if interp.GetGlobal("result").Str() != "123" {
		t.Errorf("expected '123', got '%s'", interp.GetGlobal("result").Str())
	}
}

func TestStringMatchCaptures(t *testing.T) {
	interp := runProgram(t, `
		y, m, d := string.match("2026-03-10", "(%d+)-(%d+)-(%d+)")
	`)
	if interp.GetGlobal("y").Str() != "2026" {
		t.Errorf("expected '2026', got '%s'", interp.GetGlobal("y").Str())
	}
	if interp.GetGlobal("m").Str() != "03" {
		t.Errorf("expected '03', got '%s'", interp.GetGlobal("m").Str())
	}
	if interp.GetGlobal("d").Str() != "10" {
		t.Errorf("expected '10', got '%s'", interp.GetGlobal("d").Str())
	}
}

func TestStringSplit(t *testing.T) {
	interp := runProgram(t, `
		parts := string.split("a,b,c", ",")
	`)
	tbl := interp.GetGlobal("parts").Table()
	if tbl.Length() != 3 {
		t.Errorf("expected 3 parts, got %d", tbl.Length())
	}
	if tbl.RawGet(runtime.IntValue(1)).Str() != "a" {
		t.Errorf("expected 'a', got '%s'", tbl.RawGet(runtime.IntValue(1)).Str())
	}
}

func TestStringMethodSyntax(t *testing.T) {
	interp := runProgram(t, `
		result := ("hello"):upper()
	`)
	if interp.GetGlobal("result").Str() != "HELLO" {
		t.Errorf("expected HELLO, got %s", interp.GetGlobal("result").Str())
	}
}
