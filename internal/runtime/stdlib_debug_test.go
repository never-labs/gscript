package runtime

import (
	"strings"
	"testing"
)

func TestDebugStackAndTraceback(t *testing.T) {
	interp := NewCore()
	debugModule := TableValue(buildDebugLib(interp))
	interp.SetGlobal("debug", debugModule)
	interp.SetModule("debug", debugModule)

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
	interp := NewCore()
	debugModule := TableValue(buildDebugLib(interp))
	interp.SetGlobal("debug", debugModule)
	interp.SetModule("debug", debugModule)

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

func TestDebugHookAndSink(t *testing.T) {
	interp := NewCore()
	debugModule := TableValue(buildDebugLib(interp))
	interp.SetGlobal("debug", debugModule)
	interp.SetModule("debug", debugModule)

	execBinaryIOTest(t, interp, `
		events := {}
		sinkEvents := {}
		func hook(event) {
			events[#events + 1] = event.type .. ":" .. event.kind .. ":" .. event.name
		}
		func sink(event) {
			sinkEvents[#sinkEvents + 1] = event.name .. ":" .. event.data
		}
		debug.setHook(hook, {script: true, native: false})
		debug.setSink(sink)
		func work() {
			debug.emit("progress", "ok")
			return 1
		}
		value := work()
		gotHook, opts := debug.getHook()
		debug.setHook(nil)
		debug.setSink(nil)
	`)

	if got := interp.GetGlobal("value").Int(); got != 1 {
		t.Fatalf("value = %d, want 1", got)
	}
	events := interp.GetGlobal("events").Table()
	if got := events.RawGet(IntValue(1)).Str(); got != "call:script:work" {
		t.Fatalf("events[1] = %q, want call:script:work", got)
	}
	if got := events.RawGet(IntValue(2)).Str(); got != "emit:diagnostic:progress" {
		t.Fatalf("events[2] = %q, want emit:diagnostic:progress", got)
	}
	if got := events.RawGet(IntValue(3)).Str(); got != "return:script:work" {
		t.Fatalf("events[3] = %q, want return:script:work", got)
	}
	if !interp.GetGlobal("gotHook").IsFunction() {
		t.Fatalf("gotHook should be function")
	}
	opts := interp.GetGlobal("opts").Table()
	if !opts.RawGetString("call").Bool() || opts.RawGetString("native").Bool() {
		t.Fatalf("unexpected hook opts")
	}
	sinkEvents := interp.GetGlobal("sinkEvents").Table()
	if got := sinkEvents.RawGet(IntValue(1)).Str(); got != "progress:ok" {
		t.Fatalf("sinkEvents[1] = %q, want progress:ok", got)
	}
}
