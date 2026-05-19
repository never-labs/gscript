package runtime

import (
	"strings"
	"testing"
)

func TestDebugStackAndTraceback(t *testing.T) {
	interp := New()

	execBinaryIOTest(t, interp, `
		func inner() {
			stack := debug.stack()
			trace := debug.traceback("boom")
			topName := stack[#stack].name
			parentName := stack[#stack - 1].name
			return topName, parentName, trace
		}
		func outer() {
			return inner()
		}
		top, parent, trace := outer()
	`)

	if got := interp.GetGlobal("top").Str(); got != "inner" {
		t.Fatalf("top = %q, want inner", got)
	}
	if got := interp.GetGlobal("parent").Str(); got != "outer" {
		t.Fatalf("parent = %q, want outer", got)
	}
	trace := interp.GetGlobal("trace").Str()
	if !strings.Contains(trace, "boom") || !strings.Contains(trace, "script inner") || !strings.Contains(trace, "script outer") {
		t.Fatalf("traceback missing expected frames: %q", trace)
	}
}

func TestDebugFunctionInfoGlobalsAndValue(t *testing.T) {
	interp := New()

	execBinaryIOTest(t, interp, `
		globalValue := 42
		func sample(a, b, ...) {
			return a + b
		}
		info := debug.info(sample)
		globals := debug.globals()
		value := debug.value({x: 1})
	`)

	info := interp.GetGlobal("info").Table()
	if got := info.RawGetString("name").Str(); got != "sample" {
		t.Fatalf("info.name = %q, want sample", got)
	}
	if got := info.RawGetString("kind").Str(); got != "script" {
		t.Fatalf("info.kind = %q, want script", got)
	}
	if got := info.RawGetString("vararg").Bool(); !got {
		t.Fatalf("info.vararg = false, want true")
	}
	if got := interp.GetGlobal("globals").Table().RawGetString("globalValue").Int(); got != 42 {
		t.Fatalf("globals.globalValue = %d, want 42", got)
	}
	value := interp.GetGlobal("value").Table()
	if got := value.RawGetString("type").Str(); got != "table" {
		t.Fatalf("value.type = %q, want table", got)
	}
	if got := value.RawGetString("truthy").Bool(); !got {
		t.Fatalf("value.truthy = false, want true")
	}
}
