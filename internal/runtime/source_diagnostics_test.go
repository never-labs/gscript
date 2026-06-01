package runtime_test

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	stdlibinstall "github.com/never-labs/leia/internal/stdlib/install"
)

func execRuntimeTestProgram(t *testing.T, interp *runtime.Interpreter, src string) {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if err := interp.Exec(prog); err != nil {
		t.Fatalf("exec error: %v", err)
	}
}

func TestCompileStringSourceNameInSyntaxDiagnostics(t *testing.T) {
	interp := runtime.NewCore()
	_, err := interp.CompileString("value := @", runtime.WithScriptSourceName("host/generated.leia"))
	if err == nil {
		t.Fatal("CompileString succeeded, want syntax error")
	}
	msg := err.Error()
	for _, want := range []string{"host/generated.leia", "1:10", "unexpected character"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("diagnostic %q missing %q", msg, want)
		}
	}
}

func TestScriptCompileSourceNameInDebugFramesAndRuntimeDiagnostics(t *testing.T) {
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)

	execRuntimeTestProgram(t, interp, `
		fn := script.compile("func inner() {\n  info := debug.info(0)\n  trace := debug.traceback(\"boom\")\n  return info.sourceName, info.line, info.column, trace\n}\nreturn inner()", {sourceName: "virtual/generated.leia"})
		sourceName, line, column, trace := fn()

		fail := script.compile("func fail() {\n  return missing\n}\nreturn fail()", "virtual/runtime.leia")
		ok, runtimeErr := pcall(fail)
	`)

	if got := interp.GetGlobal("sourceName").Str(); got != "virtual/generated.leia" {
		t.Fatalf("debug.info sourceName = %q", got)
	}
	if got := interp.GetGlobal("line").Int(); got != 1 {
		t.Fatalf("debug.info line = %d, want 1", got)
	}
	if got := interp.GetGlobal("column").Int(); got != 1 {
		t.Fatalf("debug.info column = %d, want 1", got)
	}
	trace := interp.GetGlobal("trace").Str()
	for _, want := range []string{"boom", "script inner", "virtual/generated.leia:1:1"} {
		if !strings.Contains(trace, want) {
			t.Fatalf("traceback %q missing %q", trace, want)
		}
	}
	if got := interp.GetGlobal("ok").Bool(); got {
		t.Fatal("pcall unexpectedly succeeded")
	}
	runtimeErr := interp.GetGlobal("runtimeErr").Str()
	for _, want := range []string{"virtual/runtime.leia", "2:10", "undefined variable: missing"} {
		if !strings.Contains(runtimeErr, want) {
			t.Fatalf("runtime error %q missing %q", runtimeErr, want)
		}
	}
}

func TestLoadSourceNameInReturnedSyntaxError(t *testing.T) {
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)

	execRuntimeTestProgram(t, interp, `
		fn, compileErr := load("func broken( {", "virtual/syntax.leia")
	`)

	if !interp.GetGlobal("fn").IsNil() {
		t.Fatal("load returned function for invalid source")
	}
	compileErr := interp.GetGlobal("compileErr").Str()
	for _, want := range []string{"virtual/syntax.leia", "parse error at 1:14", "LBRACE", "\"{\""} {
		if !strings.Contains(compileErr, want) {
			t.Fatalf("compile error %q missing %q", compileErr, want)
		}
	}
}
