package modules

import (
	"strings"
	"testing"
)

func TestUTF8HotBuiltinsExposeFastArgPaths(t *testing.T) {
	lib := BuildUTF8(nil)
	codepoint := lib.RawGetString("codepoint").GoFunction()
	if codepoint == nil || codepoint.FastArg1 == nil || codepoint.FastArg2 == nil {
		t.Fatalf("utf8.codepoint missing fast paths: %#v", codepoint)
	}
	got, err := codepoint.FastArg2(StringValue("AB"), IntValue(2))
	if err != nil || !got.IsInt() || got.Int() != 66 {
		t.Fatalf("utf8.codepoint FastArg2 got=%s err=%v", got.String(), err)
	}

	codes := lib.RawGetString("codes").GoFunction()
	if codes == nil || codes.Fast1 == nil || codes.FastArg1 == nil {
		t.Fatalf("utf8.codes missing fast paths: %#v", codes)
	}
	iterValue, err := codes.FastArg1(StringValue("AB"))
	if err != nil {
		t.Fatalf("utf8.codes FastArg1: %v", err)
	}
	iter := iterValue.GoFunction()
	if iter == nil || iter.FastArg2Ret2 == nil {
		t.Fatalf("utf8.codes iterator missing FastArg2Ret2: %#v", iter)
	}
	pos, cp, n, err := iter.FastArg2Ret2(NilValue(), NilValue())
	if err != nil || n != 2 || pos.Int() != 1 || cp.Int() != 65 {
		t.Fatalf("utf8.codes iterator first got pos=%s cp=%s n=%d err=%v", pos.String(), cp.String(), n, err)
	}
	pos, cp, n, err = iter.FastArg2Ret2(NilValue(), NilValue())
	if err != nil || n != 2 || pos.Int() != 2 || cp.Int() != 66 {
		t.Fatalf("utf8.codes iterator second got pos=%s cp=%s n=%d err=%v", pos.String(), cp.String(), n, err)
	}
	_, _, n, err = iter.FastArg2Ret2(NilValue(), NilValue())
	if err != nil || n != 0 {
		t.Fatalf("utf8.codes iterator end n=%d err=%v", n, err)
	}
}

func TestUTF8BuildUsesHostResultLimit(t *testing.T) {
	lib := BuildUTF8(func() int64 { return 4 })
	char := lib.RawGetString("char").GoFunction()
	if char == nil {
		t.Fatal("utf8.char missing")
	}
	_, err := char.Fn([]Value{IntValue(49), IntValue(50), IntValue(51), IntValue(52), IntValue(53)})
	if err == nil || !strings.Contains(err.Error(), "host result byte limit exceeded (4)") {
		t.Fatalf("utf8.char limit err = %v, want host result budget", err)
	}
}
