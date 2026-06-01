package vm

import (
	"errors"
	"github.com/never-labs/gscript/internal/testutil/vmtest"
	"path/filepath"
	"testing"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/runtime"
)

func compileAndRunVMConstDefer(t *testing.T, src string, globals map[string]runtime.Value) map[string]runtime.Value {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	proto, err := Compile(prog)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	v := New(globals)
	if _, err := v.Execute(proto); err != nil {
		t.Fatalf("runtime error: %v", err)
	}
	return globals
}

func compileAndRunVMConstDeferErr(t *testing.T, src string, globals map[string]runtime.Value) (map[string]runtime.Value, error) {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer error: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	proto, err := Compile(prog)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	v := New(globals)
	_, err = v.Execute(proto)
	return globals, err
}

func TestVMConstBindingRejectsAssignment(t *testing.T) {
	g := compileAndRunVMConstDefer(t, `
		const x := 10
		ok, err := pcall(func() {
			x = 11
		})
	`, vmtest.NewInterpreterGlobals())

	if g["ok"].Truthy() {
		t.Fatalf("assigning a const binding should fail")
	}
	if got := g["err"].Str(); got == "" {
		t.Fatalf("expected const assignment error")
	}
}

func TestVMConstBindingAllowsTableMutationButNotRebind(t *testing.T) {
	g := compileAndRunVMConstDefer(t, `
		const cfg = {count: 1}
		cfg.count = 2
		ok, err := pcall(func() {
			cfg = {}
		})
		count := cfg.count
	`, vmtest.NewInterpreterGlobals())

	if got := g["count"].Int(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if g["ok"].Truthy() {
		t.Fatalf("rebinding a const table should fail")
	}
}

func TestVMConstBindingCapturedByClosure(t *testing.T) {
	g := compileAndRunVMConstDefer(t, `
		func outer() {
			const x := 3
			trySet := func() {
				x = 4
			}
			ok, err := pcall(trySet)
			return x, ok
		}
		value, ok := outer()
	`, vmtest.NewInterpreterGlobals())

	if got := g["value"].Int(); got != 3 {
		t.Fatalf("value = %d, want 3", got)
	}
	if g["ok"].Truthy() {
		t.Fatalf("captured const reassignment should fail")
	}
}

func TestVMDeferRunsFunctionScopeLIFO(t *testing.T) {
	g := compileAndRunVMConstDefer(t, `
		order := ""
		func record(s) {
			order = order .. s
		}
		func work() {
			x := "one"
			defer record(x)
			x = "two"
			defer record(x)
			order = order .. "body"
			return 7
		}
		result := work()
	`, vmtest.NewInterpreterGlobals())

	if got := g["result"].Int(); got != 7 {
		t.Fatalf("result = %d, want 7", got)
	}
	if got := g["order"].Str(); got != "bodytwoone" {
		t.Fatalf("order = %q, want bodytwoone", got)
	}
}

func TestVMDeferRunsOnErrorAndSupportsMethods(t *testing.T) {
	globals := vmtest.NewInterpreterGlobals()
	path := filepath.Join(t.TempDir(), "defer.txt")
	globals["file"] = runtime.StringValue(path)

	g := compileAndRunVMConstDefer(t, `
		func writeThenFail() {
			f := io.open(file, "w+")
			defer f:close()
			assert(f:write("closed"))
			error("boom")
		}
		ok, err := pcall(writeThenFail)
		f := io.open(file, "r")
		text := f:read("a")
		f:close()
	`, globals)

	if g["ok"].Truthy() {
		t.Fatalf("writeThenFail should fail")
	}
	if got := g["err"].Str(); got != "boom" {
		t.Fatalf("err = %q, want boom", got)
	}
	if got := g["text"].Str(); got != "closed" {
		t.Fatalf("text = %q, want closed", got)
	}
}

func TestVMDeferRunsAtTopLevelScriptExit(t *testing.T) {
	globals := vmtest.NewInterpreterGlobals()
	path := filepath.Join(t.TempDir(), "top.txt")
	globals["file"] = runtime.StringValue(path)

	compileAndRunVMConstDefer(t, `
		f := io.open(file, "w+")
		defer f:close()
		assert(f:write("top"))
	`, globals)

	checkGlobals := vmtest.NewInterpreterGlobals()
	checkGlobals["file"] = runtime.StringValue(path)
	g := compileAndRunVMConstDefer(t, `
		f := io.open(file, "r")
		text := f:read("a")
		f:close()
	`, checkGlobals)
	if got := g["text"].Str(); got != "top" {
		t.Fatalf("text = %q, want top", got)
	}
}

func TestVMOSExitRunsTopLevelDefers(t *testing.T) {
	globals := vmtest.NewInterpreterGlobals()
	path := filepath.Join(t.TempDir(), "top-exit.txt")
	globals["file"] = runtime.StringValue(path)

	_, err := compileAndRunVMConstDeferErr(t, `
		f := io.open(file, "w+")
		defer f:close()
		assert(f:write("top"))
		os.exit(5)
	`, globals)
	var exitErr *runtime.ProcessExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %T %v, want ProcessExitError", err, err)
	}
	if exitErr.Code != 5 {
		t.Fatalf("exit code = %d, want 5", exitErr.Code)
	}

	checkGlobals := vmtest.NewInterpreterGlobals()
	checkGlobals["file"] = runtime.StringValue(path)
	g := compileAndRunVMConstDefer(t, `
		f := io.open(file, "r")
		text := f:read("*a")
		f:close()
	`, checkGlobals)
	if got := g["text"].Str(); got != "top" {
		t.Fatalf("text = %q, want top", got)
	}
}
