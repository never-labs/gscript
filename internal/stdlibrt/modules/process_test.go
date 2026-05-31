package modules

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/stdlibrt/host"
)

func processInterp(t *testing.T) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	processModule := runtime.TableValue(BuildProcessWithPolicy(host.Options{
		ProcessExecution:      interp.ProcessExecutionEnabled,
		ProcessShell:          interp.ProcessShellEnabled,
		EnvironmentRead:       interp.EnvironmentReadEnabled,
		EnvironmentWrite:      interp.EnvironmentWriteEnabled,
		EnvironmentAllowed:    interp.EnvironmentAllowed,
		FilesystemRoot:        interp.FilesystemRoot,
		ResolveFilesystemPath: interp.ResolveFilesystemPath,
		MaxHostResult:         interp.MaxHostResultBytes,
	}))
	interp.SetGlobal("process", processModule)
	interp.SetModule("process", processModule)
	return interp
}

func TestProcessWhich(t *testing.T) {
	interp := processInterp(t)
	execOnInterp(t, interp, `result := process.which("ls")`)

	v := interp.GetGlobal("result")
	if v.IsNil() {
		t.Errorf("expected 'ls' to be found in PATH")
	}
	if !strings.Contains(v.Str(), "ls") {
		t.Errorf("expected path to contain 'ls', got '%s'", v.Str())
	}
}

func TestProcessWhichNotFound(t *testing.T) {
	interp := processInterp(t)
	execOnInterp(t, interp, `result := process.which("__nonexistent_binary_12345__")`)

	v := interp.GetGlobal("result")
	if !v.IsNil() {
		t.Errorf("expected nil for nonexistent binary, got %v", v)
	}
}

func TestProcessRun(t *testing.T) {
	interp := processInterp(t)
	execOnInterp(t, interp, `result := process.run("echo hello")`)

	v := interp.GetGlobal("result")
	if !v.IsTable() {
		t.Fatalf("expected table result, got %s", v.TypeName())
	}
	tbl := v.Table()
	if !tbl.RawGet(runtime.StringValue("ok")).Bool() {
		t.Errorf("expected ok=true")
	}
	stdout := tbl.RawGet(runtime.StringValue("stdout")).Str()
	if strings.TrimSpace(stdout) != "hello" {
		t.Errorf("expected stdout='hello', got '%s'", stdout)
	}
	if tbl.RawGet(runtime.StringValue("code")).Int() != 0 {
		t.Errorf("expected code=0, got %v", tbl.RawGet(runtime.StringValue("code")))
	}
}

func TestProcessRunContextCancelled(t *testing.T) {
	interp := processInterp(t)
	state := runtime.NewScriptContextState()
	time.AfterFunc(10*time.Millisecond, func() {
		state.Cancel(runtime.StringValue("deadline exceeded"))
	})
	interp.SetGlobal("ctx", runtime.TableValue(runtime.NewScriptContextTable(state)))
	execOnInterp(t, interp, `
result := process.run(ctx, {"sh", "-c", "sleep 1; echo late"})
`)

	v := interp.GetGlobal("result")
	if !v.IsTable() {
		t.Fatalf("expected table result, got %s", v.TypeName())
	}
	tbl := v.Table()
	if tbl.RawGet(runtime.StringValue("ok")).Bool() {
		t.Fatalf("expected ok=false")
	}
	if !tbl.RawGet(runtime.StringValue("cancelled")).Bool() {
		t.Fatalf("expected cancelled=true")
	}
	if got := tbl.RawGet(runtime.StringValue("err")); !got.IsString() || got.Str() != "deadline exceeded" {
		t.Fatalf("err = %v, want deadline exceeded", got)
	}
	if strings.Contains(tbl.RawGet(runtime.StringValue("stdout")).Str(), "late") {
		t.Fatalf("process was not cancelled before late output")
	}
}

func TestProcessShell(t *testing.T) {
	interp := processInterp(t)
	execOnInterp(t, interp, `result := process.shell("echo hello && echo world")`)

	v := interp.GetGlobal("result")
	tbl := v.Table()
	if !tbl.RawGet(runtime.StringValue("ok")).Bool() {
		t.Errorf("expected ok=true")
	}
	stdout := tbl.RawGet(runtime.StringValue("stdout")).Str()
	if !strings.Contains(stdout, "hello") || !strings.Contains(stdout, "world") {
		t.Errorf("expected stdout to contain 'hello' and 'world', got '%s'", stdout)
	}
}

func TestProcessPid(t *testing.T) {
	interp := processInterp(t)
	execOnInterp(t, interp, `pid := process.pid()`)

	v := interp.GetGlobal("pid")
	if v.Int() != int64(os.Getpid()) {
		t.Errorf("expected %d, got %d", os.Getpid(), v.Int())
	}
}

func TestProcessEnv(t *testing.T) {
	interp := processInterp(t)

	os.Setenv("GSCRIPT_TEST_PROC_ENV", "test_value")
	defer os.Unsetenv("GSCRIPT_TEST_PROC_ENV")

	execOnInterp(t, interp, `env := process.env()`)

	v := interp.GetGlobal("env")
	if !v.IsTable() {
		t.Fatalf("expected table, got %s", v.TypeName())
	}
	val := v.Table().RawGet(runtime.StringValue("GSCRIPT_TEST_PROC_ENV"))
	if val.Str() != "test_value" {
		t.Errorf("expected 'test_value', got '%s'", val.Str())
	}
}

func TestProcessExec(t *testing.T) {
	interp := processInterp(t)
	execOnInterp(t, interp, `result := process.exec("echo", "hello")`)

	v := interp.GetGlobal("result")
	if strings.TrimSpace(v.Str()) != "hello" {
		t.Errorf("expected 'hello', got '%s'", v.Str())
	}
}
