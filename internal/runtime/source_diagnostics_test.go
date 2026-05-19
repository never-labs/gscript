package runtime

import (
	"strings"
	"testing"
)

func TestCompileStringSourceNameInSyntaxDiagnostics(t *testing.T) {
	interp := New()
	_, err := interp.CompileString("value := @", WithScriptSourceName("host/generated.gs"))
	if err == nil {
		t.Fatal("CompileString succeeded, want syntax error")
	}
	msg := err.Error()
	for _, want := range []string{"host/generated.gs", "1:10", "unexpected character"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("diagnostic %q missing %q", msg, want)
		}
	}
}

func TestScriptCompileSourceNameInDebugFramesAndRuntimeDiagnostics(t *testing.T) {
	interp := New()

	execBinaryIOTest(t, interp, `
		fn := script.compile("func inner() {\n  info := debug.info(0)\n  trace := debug.traceback(\"boom\")\n  return info.sourceName, info.line, info.column, trace\n}\nreturn inner()", {sourceName: "virtual/generated.gs"})
		sourceName, line, column, trace := fn()

		fail := script.compile("func fail() {\n  return missing\n}\nreturn fail()", "virtual/runtime.gs")
		ok, runtimeErr := pcall(fail)
	`)

	if got := interp.GetGlobal("sourceName").Str(); got != "virtual/generated.gs" {
		t.Fatalf("debug.info sourceName = %q", got)
	}
	if got := interp.GetGlobal("line").Int(); got != 1 {
		t.Fatalf("debug.info line = %d, want 1", got)
	}
	if got := interp.GetGlobal("column").Int(); got != 1 {
		t.Fatalf("debug.info column = %d, want 1", got)
	}
	trace := interp.GetGlobal("trace").Str()
	for _, want := range []string{"boom", "script inner", "virtual/generated.gs:1:1"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("traceback %q missing %q", trace, want)
		}
	}
	if got := interp.GetGlobal("ok").Bool(); got {
		t.Fatal("pcall unexpectedly succeeded")
	}
	runtimeErr := interp.GetGlobal("runtimeErr").Str()
	for _, want := range []string{"virtual/runtime.gs", "2:10", "undefined variable: missing"} {
		if !strings.Contains(runtimeErr, want) {
			t.Fatalf("runtime error %q missing %q", runtimeErr, want)
		}
	}
}

func TestLoadSourceNameInReturnedSyntaxError(t *testing.T) {
	interp := New()

	execBinaryIOTest(t, interp, `
		fn, compileErr := load("func broken( {", "virtual/syntax.gs")
	`)

	if !interp.GetGlobal("fn").IsNil() {
		t.Fatal("load returned function for invalid source")
	}
	compileErr := interp.GetGlobal("compileErr").Str()
	for _, want := range []string{"virtual/syntax.gs", "parse error at 1:14", "LBRACE", "\"{\""} {
		if !strings.Contains(compileErr, want) {
			t.Fatalf("compile error %q missing %q", compileErr, want)
		}
	}
}
