package runtime

import (
	"math"
	"strconv"
	"testing"
)

func TestStringFormatSingleIntMinInt64(t *testing.T) {
	v, ok, err := StringFormatSingleInt("%d", math.MinInt64)
	if err != nil {
		t.Fatalf("StringFormatSingleInt failed: %v", err)
	}
	if !ok {
		t.Fatal("StringFormatSingleInt did not accept decimal integer pattern")
	}
	if got, want := v.Str(), "-9223372036854775808"; got != want {
		t.Fatalf("StringFormatSingleInt MinInt64=%q, want %q", got, want)
	}
}

func TestStdStringFormatIdentityGuardRejectsLookalikeGoFunction(t *testing.T) {
	lib := buildStringLib()
	std := lib.RawGetString("format")
	if !IsStdStringFormatFunction(std) {
		t.Fatal("runtime-native string.format was not recognized")
	}

	lookalike := FunctionValue(&GoFunction{
		Name:     "string.format",
		FastArg2: stringFormat2Value,
	})
	if IsStdStringFormatFunction(lookalike) {
		t.Fatal("lookalike GoFunction passed runtime-native string.format identity guard")
	}
}

func TestStringFormatSimpleCachedTwoArgPattern(t *testing.T) {
	v, err := stringFormat3Value(StringValue("item_%d_value_%05d"), IntValue(3), IntValue(21))
	if err != nil {
		t.Fatalf("stringFormat3Value failed: %v", err)
	}
	if !v.IsString() || v.Str() != "item_3_value_00021" {
		t.Fatalf("expected item_3_value_00021, got %v", v)
	}
}

func TestStringFormatSimpleCachedThreeArgPattern(t *testing.T) {
	v, err := stringFormat4Value(StringValue("%s:%d:%05d"), StringValue("svc"), IntValue(3), IntValue(21))
	if err != nil {
		t.Fatalf("stringFormat4Value failed: %v", err)
	}
	if !v.IsString() || v.Str() != "svc:3:00021" {
		t.Fatalf("expected svc:3:00021, got %v", v)
	}
}

func TestStringPredicateFixedArgFastPaths(t *testing.T) {
	lib := buildStringLib()
	cases := []struct {
		name string
		a    Value
		b    Value
		want Value
	}{
		{"hasPrefix", StringValue("abcdef"), StringValue("abc"), BoolValue(true)},
		{"hasSuffix", StringValue("abcdef"), StringValue("def"), BoolValue(true)},
		{"contains", StringValue("abcdef"), StringValue("cd"), BoolValue(true)},
		{"count", StringValue("banana"), StringValue("na"), IntValue(2)},
	}
	for _, tc := range cases {
		fn := lib.RawGetString(tc.name).GoFunction()
		if fn == nil || fn.FastArg2 == nil {
			t.Fatalf("string.%s FastArg2 is nil", tc.name)
		}
		got, err := fn.FastArg2(tc.a, tc.b)
		if err != nil {
			t.Fatalf("string.%s FastArg2 failed: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("string.%s FastArg2 = %s, want %s", tc.name, got.String(), tc.want.String())
		}
	}
}

func TestStringTransformFixedArgFastPaths(t *testing.T) {
	lib := buildStringLib()
	trim := lib.RawGetString("trim").GoFunction()
	if trim == nil || trim.FastArg1 == nil || trim.FastArg2 == nil {
		t.Fatal("string.trim fixed fast paths missing")
	}
	if got, err := trim.FastArg1(StringValue("  abc  ")); err != nil || !got.IsString() || got.Str() != "abc" {
		t.Fatalf("trim FastArg1 got=%s err=%v, want abc nil", got.String(), err)
	}
	if got, err := trim.FastArg2(StringValue("xxabcxy"), StringValue("xy")); err != nil || !got.IsString() || got.Str() != "abc" {
		t.Fatalf("trim FastArg2 got=%s err=%v, want abc nil", got.String(), err)
	}

	for _, name := range []string{"trimLeft", "trimRight"} {
		fn := lib.RawGetString(name).GoFunction()
		if fn == nil || fn.FastArg1 == nil || fn.FastArg2 == nil {
			t.Fatalf("string.%s fixed fast paths missing", name)
		}
	}
	replace := lib.RawGetString("replaceAll").GoFunction()
	if replace == nil || replace.FastArg3 == nil {
		t.Fatal("string.replaceAll FastArg3 is nil")
	}
	if got, err := replace.FastArg3(StringValue("a-b-a"), StringValue("a"), StringValue("x")); err != nil || !got.IsString() || got.Str() != "x-b-x" {
		t.Fatalf("replaceAll FastArg3 got=%s err=%v, want x-b-x nil", got.String(), err)
	}
}

func TestStringFormatSingleIntegerResultCacheReusesValue(t *testing.T) {
	prog, ok, err := compileSimpleFormat("SKU%05d")
	if err != nil || !ok {
		t.Fatalf("compileSimpleFormat: ok=%v err=%v", ok, err)
	}

	args := []Value{StringValue("SKU%05d"), IntValue(42)}
	v1, err := prog.formatValue(args)
	if err != nil {
		t.Fatalf("first formatValue failed: %v", err)
	}
	v2, err := prog.formatValue(args)
	if err != nil {
		t.Fatalf("second formatValue failed: %v", err)
	}
	if v1 != v2 {
		t.Fatalf("expected cached single-integer format to reuse boxed string value")
	}
	if v1.Str() != "SKU00042" {
		t.Fatalf("expected SKU00042, got %q", v1.Str())
	}
}

func TestStringFormatSingleIntegerResultCacheIsBounded(t *testing.T) {
	prog, ok, err := compileSimpleFormat("key_%05d")
	if err != nil || !ok {
		t.Fatalf("compileSimpleFormat: ok=%v err=%v", ok, err)
	}

	for i := 0; i < simpleFormatResultCacheLimit+8; i++ {
		_, err := prog.formatValue([]Value{StringValue("key_%05d"), IntValue(int64(i))})
		if err != nil {
			t.Fatalf("formatValue(%d) failed: %v", i, err)
		}
	}

	prog.resultMu.Lock()
	gotEntries := len(prog.resultCache)
	gotOrder := len(prog.resultOrder)
	prog.resultMu.Unlock()
	if gotEntries > simpleFormatResultCacheLimit || gotOrder > simpleFormatResultCacheLimit {
		t.Fatalf("simple format result cache grew beyond limit: entries=%d order=%d limit=%d", gotEntries, gotOrder, simpleFormatResultCacheLimit)
	}
}

func TestCompileSimpleFormatRejectsFallbackFormats(t *testing.T) {
	if _, ok, err := compileSimpleFormat("progress %% %d"); err != nil || ok {
		t.Fatalf("escaped percent should use fallback parser: ok=%v err=%v", ok, err)
	}
	if _, ok, err := compileSimpleFormat("%.3e"); err != nil || ok {
		t.Fatalf("exponent float should use fallback parser: ok=%v err=%v", ok, err)
	}
}

func TestSimpleFormatCacheIsBounded(t *testing.T) {
	simpleFormatCache.Lock()
	simpleFormatCache.entries = make(map[string]*simpleFormatProgram)
	simpleFormatCache.order = nil
	simpleFormatCache.Unlock()

	for i := 0; i < simpleFormatCacheLimit+8; i++ {
		if _, ok, err := cachedSimpleFormat("value_%0" + strconv.Itoa(i+1) + "d"); err != nil || !ok {
			t.Fatalf("cachedSimpleFormat(%d): ok=%v err=%v", i, ok, err)
		}
	}

	simpleFormatCache.Lock()
	gotEntries := len(simpleFormatCache.entries)
	gotOrder := len(simpleFormatCache.order)
	simpleFormatCache.Unlock()
	if gotEntries > simpleFormatCacheLimit || gotOrder > simpleFormatCacheLimit {
		t.Fatalf("simple format cache grew beyond limit: entries=%d order=%d limit=%d", gotEntries, gotOrder, simpleFormatCacheLimit)
	}
}
