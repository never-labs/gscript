package runtime

import (
	"testing"
)

// ==================================================================
// String operations edge cases (beyond what string_test.go covers)
// ==================================================================

// --- Concat edge cases ---

func TestConcatMultipleStrings(t *testing.T) {
	v := getGlobal(t, `result := "a" .. "b" .. "c" .. "d" .. "e"`, "result")
	if v.Str() != "abcde" {
		t.Errorf("expected 'abcde', got %q", v.Str())
	}
}

func TestConcatBoolToString(t *testing.T) {
	// Booleans cannot be concatenated
	err := runProgramExpectError(t, `result := "val: " .. true`)
	if err == nil {
		t.Fatal("expected error for concatenating boolean")
	}
}

func TestConcatNilToString(t *testing.T) {
	err := runProgramExpectError(t, `result := "val: " .. nil`)
	if err == nil {
		t.Fatal("expected error for concatenating nil")
	}
}

// --- String length edge cases ---

func TestStringLenWithSpaces(t *testing.T) {
	v := getGlobal(t, `result := #"  hello  "`, "result")
	if v.Int() != 9 {
		t.Errorf("expected 9, got %v", v)
	}
}

func TestStringLenSingleChar(t *testing.T) {
	v := getGlobal(t, `result := #"x"`, "result")
	if v.Int() != 1 {
		t.Errorf("expected 1, got %v", v)
	}
}

func TestStringLenOperatorEmpty(t *testing.T) {
	v := getGlobal(t, `result := #""`, "result")
	if v.Int() != 0 {
		t.Errorf("expected 0, got %v", v)
	}
}

// --- tostring edge cases ---

func TestTostringNegInt(t *testing.T) {
	v := getGlobal(t, `result := tostring(-42)`, "result")
	if v.Str() != "-42" {
		t.Errorf("expected '-42', got %q", v.Str())
	}
}

func TestTostringZero(t *testing.T) {
	v := getGlobal(t, `result := tostring(0)`, "result")
	if v.Str() != "0" {
		t.Errorf("expected '0', got %q", v.Str())
	}
}

func TestTostringFloatWholeNumber(t *testing.T) {
	v := getGlobal(t, `result := tostring(3.0)`, "result")
	// Should have decimal point to show it's float
	if v.Str() != "3.0" {
		t.Errorf("expected '3.0', got %q", v.Str())
	}
}

// --- tonumber edge cases ---

func TestTonumberNegativeString(t *testing.T) {
	v := getGlobal(t, `result := tonumber("-42")`, "result")
	if !v.IsInt() || v.Int() != -42 {
		t.Errorf("expected -42, got %v", v)
	}
}

func TestTonumberScientific(t *testing.T) {
	v := getGlobal(t, `result := tonumber("1e3")`, "result")
	if !v.IsFloat() || v.Float() != 1000.0 {
		t.Errorf("expected 1000.0, got %v", v)
	}
}

func TestTonumberEmptyString(t *testing.T) {
	v := getGlobal(t, `result := tonumber("")`, "result")
	if !v.IsNil() {
		t.Errorf("expected nil for empty string, got %v", v)
	}
}

func TestTonumberTable(t *testing.T) {
	v := getGlobal(t, `result := tonumber({})`, "result")
	if !v.IsNil() {
		t.Errorf("expected nil for table, got %v", v)
	}
}

// --- String comparison edge cases ---

func TestStringCompareEmpty(t *testing.T) {
	v := getGlobal(t, `result := "" < "a"`, "result")
	if !v.Bool() {
		t.Errorf("empty string should be less than 'a'")
	}
}

func TestStringCompareCase(t *testing.T) {
	v := getGlobal(t, `result := "A" < "a"`, "result")
	if !v.Bool() {
		t.Errorf("'A' should be less than 'a' (ASCII order)")
	}
}

func TestStringCompareNotEqual(t *testing.T) {
	v := getGlobal(t, `result := "abc" != "def"`, "result")
	if !v.Bool() {
		t.Errorf("expected true")
	}
}

func TestStringComparisonLess(t *testing.T) {
	v := getGlobal(t, `result := "abc" < "abd"`, "result")
	if !v.Bool() {
		t.Errorf("expected true")
	}
}

func TestStringComparisonEqual(t *testing.T) {
	v := getGlobal(t, `result := "abc" == "abc"`, "result")
	if !v.Bool() {
		t.Errorf("expected true")
	}
}

func TestStringComparisonGreater(t *testing.T) {
	v := getGlobal(t, `result := "z" > "a"`, "result")
	if !v.Bool() {
		t.Errorf("expected true")
	}
}

// --- String in concatenation with numbers ---

func TestConcatIntZero(t *testing.T) {
	v := getGlobal(t, `result := 0 .. ""`, "result")
	if v.Str() != "0" {
		t.Errorf("expected '0', got %q", v.Str())
	}
}

func TestConcatNegativeNumber(t *testing.T) {
	v := getGlobal(t, `result := "value: " .. -5`, "result")
	if v.Str() != "value: -5" {
		t.Errorf("expected 'value: -5', got %q", v.Str())
	}
}

func TestEmptyStringConcat(t *testing.T) {
	v := getGlobal(t, `result := "" .. ""`, "result")
	if v.Str() != "" {
		t.Errorf("expected empty string, got %q", v.Str())
	}
}

// --- String used as table keys ---

func TestStringTableKey(t *testing.T) {
	v := getGlobal(t, `
		t := {}
		key := "mykey"
		t[key] = 42
		result := t[key]
	`, "result")
	if !v.IsInt() || v.Int() != 42 {
		t.Errorf("expected 42, got %v", v)
	}
}

func TestStringDynamicFieldAccess(t *testing.T) {
	v := getGlobal(t, `
		t := {a: 1, b: 2, c: 3}
		key := "b"
		result := t[key]
	`, "result")
	if !v.IsInt() || v.Int() != 2 {
		t.Errorf("expected 2, got %v", v)
	}
}
