package modules

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestStringMethodChaining(t *testing.T) {
	v := getGlobal(t, `result := ("hello"):upper():sub(1, 3)`, "result")
	if v.Str() != "HEL" {
		t.Errorf("expected 'HEL', got %q", v.Str())
	}
}

func TestStringFindFromMiddle(t *testing.T) {
	interp := runProgram(t, `s, e := string.find("hello hello", "hello", 2, true)`)
	s := interp.GetGlobal("s")
	e := interp.GetGlobal("e")
	if s.Int() != 7 || e.Int() != 11 {
		t.Errorf("expected 7,11, got %v,%v", s, e)
	}
}

func TestStringFindEmptyString(t *testing.T) {
	interp := runProgram(t, `s, e := string.find("hello", "", 1, true)`)
	s := interp.GetGlobal("s")
	if s.Int() != 1 {
		t.Errorf("expected start=1, got %v", s)
	}
}

func TestStringGsubPattern(t *testing.T) {
	interp := runProgram(t, `result, count := string.gsub("hello 123 world 456", "%d+", "NUM")`)
	if interp.GetGlobal("result").Str() != "hello NUM world NUM" {
		t.Errorf("expected 'hello NUM world NUM', got %q", interp.GetGlobal("result").Str())
	}
	if interp.GetGlobal("count").Int() != 2 {
		t.Errorf("expected count=2, got %v", interp.GetGlobal("count"))
	}
}

func TestStringRepOne(t *testing.T) {
	v := getGlobal(t, `result := string.rep("hello", 1)`, "result")
	if v.Str() != "hello" {
		t.Errorf("expected 'hello', got %q", v.Str())
	}
}

func TestStringRepNegative(t *testing.T) {
	v := getGlobal(t, `result := string.rep("hello", -1)`, "result")
	if v.Str() != "" {
		t.Errorf("expected empty string for negative rep, got %q", v.Str())
	}
}

func TestStringReverseSingleChar(t *testing.T) {
	v := getGlobal(t, `result := string.reverse("x")`, "result")
	if v.Str() != "x" {
		t.Errorf("expected 'x', got %q", v.Str())
	}
}

func TestStringReversePalindrome(t *testing.T) {
	v := getGlobal(t, `result := string.reverse("racecar")`, "result")
	if v.Str() != "racecar" {
		t.Errorf("expected 'racecar', got %q", v.Str())
	}
}

func TestStringByteAtPosition(t *testing.T) {
	v := getGlobal(t, `result := string.byte("ABC", 2)`, "result")
	if v.Int() != 66 {
		t.Errorf("expected 66 (B), got %v", v)
	}
}

func TestStringCharEmpty(t *testing.T) {
	v := getGlobal(t, `result := string.char()`, "result")
	if v.Str() != "" {
		t.Errorf("expected empty string, got %q", v.Str())
	}
}

func TestStringCharNewline(t *testing.T) {
	v := getGlobal(t, `result := string.char(10)`, "result")
	if v.Str() != "\n" {
		t.Errorf("expected newline, got %q", v.Str())
	}
}

func TestStringSplitNoMatch(t *testing.T) {
	interp := runProgram(t, `parts := string.split("hello", ",")`)
	tbl := interp.GetGlobal("parts").Table()
	if tbl.Length() != 1 {
		t.Errorf("expected 1 part, got %d", tbl.Length())
	}
	if tbl.RawGet(runtime.IntValue(1)).Str() != "hello" {
		t.Errorf("expected 'hello', got %q", tbl.RawGet(runtime.IntValue(1)).Str())
	}
}

func TestStringSplitMultiCharSep(t *testing.T) {
	interp := runProgram(t, `parts := string.split("a::b::c", "::")`)
	tbl := interp.GetGlobal("parts").Table()
	if tbl.Length() != 3 {
		t.Errorf("expected 3 parts, got %d", tbl.Length())
	}
	if tbl.RawGet(runtime.IntValue(1)).Str() != "a" {
		t.Errorf("expected 'a', got %q", tbl.RawGet(runtime.IntValue(1)).Str())
	}
	if tbl.RawGet(runtime.IntValue(2)).Str() != "b" {
		t.Errorf("expected 'b', got %q", tbl.RawGet(runtime.IntValue(2)).Str())
	}
}

func TestStringFormatEmptyString(t *testing.T) {
	v := getGlobal(t, `result := string.format("")`, "result")
	if v.Str() != "" {
		t.Errorf("expected empty string, got %q", v.Str())
	}
}

func TestStringFormatNoSpecifiers(t *testing.T) {
	v := getGlobal(t, `result := string.format("hello world")`, "result")
	if v.Str() != "hello world" {
		t.Errorf("expected 'hello world', got %q", v.Str())
	}
}

func TestStringFormatNegativeInt(t *testing.T) {
	v := getGlobal(t, `result := string.format("%d", -42)`, "result")
	if v.Str() != "-42" {
		t.Errorf("expected '-42', got %q", v.Str())
	}
}

func TestStringMatchAnchored(t *testing.T) {
	v := getGlobal(t, `result := string.match("hello123", "^%a+")`, "result")
	if v.Str() != "hello" {
		t.Errorf("expected 'hello', got %q", v.Str())
	}
}

func TestStringMatchEndAnchor(t *testing.T) {
	v := getGlobal(t, `result := string.match("hello123", "%d+$")`, "result")
	if v.Str() != "123" {
		t.Errorf("expected '123', got %q", v.Str())
	}
}
