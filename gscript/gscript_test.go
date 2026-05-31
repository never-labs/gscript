package gscript_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

func TestOSEexitReturnsCatchableExitError(t *testing.T) {
	for _, tc := range []struct {
		name string
		vm   *gs.VM
	}{
		{name: "interpreter", vm: gs.New()},
		{name: "bytecode", vm: gs.New(gs.WithVM())},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.vm.Exec(`os.exit(7)`)
			if err == nil {
				t.Fatal("expected exit error")
			}
			var exitErr *gs.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("expected ExitError, got %T %v", err, err)
			}
			if exitErr.Code != 7 {
				t.Fatalf("exit code = %d, want 7", exitErr.Code)
			}
		})
	}
}

func TestOSEexitBooleanStatus(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`os.exit(false)`)
	var exitErr *gs.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitError, got %T %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("exit code = %d, want 1", exitErr.Code)
	}
}

// --- Error handling tests ---

func TestError_parseError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`func {`)
	if err == nil {
		t.Fatal("expected error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrParse {
		t.Fatalf("expected ErrParse, got %s", gsErr.Kind)
	}
}

func TestError_runtimeError(t *testing.T) {
	vm := gs.New()
	err := vm.Exec(`x := 1 + "abc"`)
	if err == nil {
		t.Fatal("expected runtime error")
	}
	gsErr, ok := err.(*gs.Error)
	if !ok {
		t.Fatalf("expected *gscript.Error, got %T", err)
	}
	if gsErr.Kind != gs.ErrRuntime {
		t.Fatalf("expected ErrRuntime, got %s", gsErr.Kind)
	}
}

// --- Options tests ---

func TestWithPrint(t *testing.T) {
	var captured []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		captured = append(captured, strings.Join(parts, " "))
	}))
	vm.Exec(`print("test", 123)`)
	if len(captured) != 1 {
		t.Fatalf("expected 1 captured, got %d", len(captured))
	}
	if captured[0] != "test 123" {
		t.Fatalf("expected 'test 123', got %q", captured[0])
	}
}

func TestWithLibs(t *testing.T) {
	// LibSafe should still work for basic math
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	err := vm.Exec(`x := 1 + 2`)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWithLibsRestrictsUnsafeGlobals(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe))
	err := vm.Exec(`
		hasMath := type(math)
		hasJSON := type(json)
		hasBytes := type(bytes)
		hasURL := type(url)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hasMath", "hasJSON", "hasBytes", "hasURL"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != "table" {
			t.Fatalf("%s = %v, want table", name, got)
		}
	}
	for _, name := range []string{"io", "os", "fs", "net", "http", "process", "script", "debug", "testkit"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithLibsRestrictsBytecodeVM(t *testing.T) {
	vm := gs.New(gs.WithLibs(gs.LibSafe), gs.WithVM())
	err := vm.Exec(`
		hasString := type(string)
		hasBytes := type(bytes)
	`)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hasString", "hasBytes"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != "table" {
			t.Fatalf("%s = %v, want table", name, got)
		}
	}
	for _, name := range []string{"http", "debug", "testkit"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
}

func TestWithSandboxDisablesFilesystemCapabilities(t *testing.T) {
	vm := gs.New(gs.WithSandbox())
	if err := vm.Exec(`hasJSON := type(json)`); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"fs", "dofile", "loadfile"} {
		got, err := vm.Get(name)
		if err != nil {
			t.Fatal(err)
		}
		if got != nil {
			t.Fatalf("%s = %v, want nil", name, got)
		}
	}
	got, err := vm.Get("hasJSON")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("hasJSON = %v, want table", got)
	}
}

func TestSecuritySandboxDisablesHostCapabilitiesAndJIT(t *testing.T) {
	vm := gs.New(gs.WithJIT(), gs.SecuritySandbox(), gs.WithMaxSteps(16))
	if err := vm.Exec(`hasJSON := type(require("json"))`); err != nil {
		t.Fatalf("safe stdlib should remain available: %v", err)
	}
	for _, src := range []string{
		`fs.readfile("x")`,
		`os.getenv("PATH")`,
		`process.pid()`,
		`require("helper")`,
	} {
		if err := vm.Exec(src); err == nil {
			t.Fatalf("SecuritySandbox allowed %s", src)
		}
	}
	err := vm.Exec(`for {}`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) {
		t.Fatalf("expected step budget in sandboxed loop, got %T %v", err, err)
	}
	if err := vm.Exec(`fn, loadErr := load("x := 1")`); err != nil {
		t.Fatal(err)
	}
	loadErr, err := vm.Get("loadErr")
	if err != nil {
		t.Fatal(err)
	}
	if msg, ok := loadErr.(string); !ok || !strings.Contains(msg, "dynamic eval disabled") {
		t.Fatalf("loadErr = %v, want dynamic eval disabled", loadErr)
	}
}

func TestWithDynamicEvalFalseBlocksScriptStringCompilation(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithDynamicEval(false)}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`fn, loadErr := load("x := 1")`); err != nil {
				t.Fatal(err)
			}
			fn, err := vm.Get("fn")
			if err != nil {
				t.Fatal(err)
			}
			if fn != nil {
				t.Fatalf("fn = %v, want nil", fn)
			}
			loadErr, err := vm.Get("loadErr")
			if err != nil {
				t.Fatal(err)
			}
			if msg, ok := loadErr.(string); !ok || !strings.Contains(msg, "dynamic eval disabled") {
				t.Fatalf("loadErr = %v, want dynamic eval disabled", loadErr)
			}
			err = vm.Exec(`script.eval("x := 1")`)
			if err == nil || !strings.Contains(err.Error(), "dynamic eval disabled") {
				t.Fatalf("script.eval err = %v, want dynamic eval disabled", err)
			}
		})
	}
}

func TestWithSecurityAppliesSandboxAndBudgets(t *testing.T) {
	vm := gs.New(gs.WithJIT(), gs.WithSecurity(gs.SecurityPolicy{
		Libs:                    gs.LibSafe,
		Capabilities:            gs.CapSafe,
		DisableModuleLoading:    true,
		DisableJIT:              true,
		MaxSteps:                32,
		MaxNativeCalls:          4,
		MaxCallDepth:            8,
		MaxGoroutines:           1,
		MaxChannelCapacity:      2,
		MaxHostResultBytes:      4,
		MaxModuleBytes:          128,
		MaxModuleDepth:          1,
		MaxFilesystemReadBytes:  128,
		MaxFilesystemWriteBytes: 128,
		EnvironmentAllowlist:    []string{"GSCRIPT_PUBLIC_ENV_CAP_TEST"},
		DisableDynamicEval:      true,
		DisableNetworkAccess:    true,
		DisableDebugAccess:      true,
		DisableTestkitAccess:    true,
		DisableProcessExecution: true,
		DisableProcessShell:     true,
	}))
	if err := vm.RegisterFunc("large", func() string { return "12345" }); err != nil {
		t.Fatal(err)
	}
	if got, err := vm.Get("json"); err != nil || got == nil {
		t.Fatalf("safe stdlib should remain available: got=%v err=%v", got, err)
	}
	if err := vm.Exec(`fs.readfile("x")`); err == nil {
		t.Fatal("WithSecurity allowed filesystem API")
	}
	err := vm.Exec(`value := large()`)
	var budgetErr *gs.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected host_result_bytes budget 4, got %T %v", err, err)
	}
	err = vm.Exec(`for {}`)
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "steps" || budgetErr.Limit != 32 {
		t.Fatalf("expected steps budget 32, got %T %v", err, err)
	}
}

func TestEnvironmentCapabilities(t *testing.T) {
	t.Setenv("GSCRIPT_PUBLIC_ENV_CAP_TEST", "visible")

	tests := []struct {
		name    string
		opts    []gs.Option
		src     string
		wantErr string
	}{
		{
			name:    "environment disabled blocks getenv",
			opts:    []gs.Option{gs.WithEnvironment(false)},
			src:     `value := os.getenv("GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "read disabled blocks expand",
			opts:    []gs.Option{gs.WithEnvironmentRead(false)},
			src:     `value := os.expand("$GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
			wantErr: "environment read access disabled",
		},
		{
			name:    "write disabled blocks setenv",
			opts:    []gs.Option{gs.WithEnvironmentWrite(false)},
			src:     `ok := os.setenv("GSCRIPT_PUBLIC_ENV_WRITE_TEST", "blocked")`,
			wantErr: "environment write access disabled",
		},
		{
			name: "read only still reads",
			opts: []gs.Option{gs.WithEnvironmentWrite(false)},
			src:  `value := os.getenv("GSCRIPT_PUBLIC_ENV_CAP_TEST")`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(tc.opts...)
			err := vm.Exec(tc.src)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Exec error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("value")
			if err != nil {
				t.Fatal(err)
			}
			if got != "visible" {
				t.Fatalf("value = %v, want visible", got)
			}
		})
	}
}

func TestWithProcessShellFalseBlocksShell(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithProcessShell(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.shell("echo blocked")`)
			if err == nil || !strings.Contains(err.Error(), "process shell access disabled") {
				t.Fatalf("process.shell err = %v, want process shell access disabled", err)
			}
		})
	}
}

func TestWithProcessExecutionFalseBlocksRunExecAndWhich(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithProcessExecution(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`result := process.run("echo blocked")`,
				`result := process.exec("echo", "blocked")`,
				`result := process.which("echo")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "process execution access disabled") {
					t.Fatalf("%s err = %v, want process execution access disabled", src, err)
				}
			}
		})
	}
}

func TestWithFilesystemRootConfinesProcessRunDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithFilesystemRoot(root),
			}, tc.opts...)
			vm := gs.New(opts...)
			src := fmt.Sprintf(`result := process.run({"pwd"}, {dir: %q})`, outside)
			err := vm.Exec(src)
			if err == nil || !strings.Contains(err.Error(), "filesystem access denied") {
				t.Fatalf("process.run dir escape err = %v, want filesystem access denied", err)
			}
		})
	}
}

func TestProcessRunEnvFollowsEnvironmentPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name+"/write-disabled", func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithEnvironmentWrite(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.run({"pwd"}, {env: {GSCRIPT_PROCESS_ENV_POLICY_TEST: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment write access disabled") {
				t.Fatalf("process.run env err = %v, want environment write access disabled", err)
			}
		})

		t.Run(tc.name+"/allowlist", func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibProcess),
				gs.WithEnvironmentAllowlist("GSCRIPT_PROCESS_ENV_ALLOWED"),
			}, tc.opts...)
			vm := gs.New(opts...)
			err := vm.Exec(`result := process.run({"pwd"}, {env: {GSCRIPT_PROCESS_ENV_BLOCKED: "blocked"}})`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed: GSCRIPT_PROCESS_ENV_BLOCKED") {
				t.Fatalf("process.run env allowlist err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithNetworkAccessFalseBlocksNetAndHTTP(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibNet | gs.LibHTTP),
				gs.WithNetworkAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`resp := net.get("http://127.0.0.1:1")`,
				`resp := net.request({url: "http://127.0.0.1:1"})`,
				`resp := http.get("http://127.0.0.1:1")`,
				`server := http.listen("127.0.0.1:0", func(req, res) {}, {background: true})`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "network access disabled") {
					t.Fatalf("%s err = %v, want network access disabled", src, err)
				}
			}
		})
	}
}

func TestWithDebugAccessFalseBlocksDebugAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibDebug),
				gs.WithDebugAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`stack := debug.stack()`,
				`globals := debug.globals()`,
				`raw := debug.goStack()`,
				`debug.setHook(func(event) {})`,
				`debug.emit("blocked")`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "debug access disabled") {
					t.Fatalf("%s err = %v, want debug access disabled", src, err)
				}
			}
		})
	}
}

func TestWithTestkitAccessFalseBlocksTestkitAPIs(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{
				gs.WithLibs(gs.LibString | gs.LibTestkit),
				gs.WithTestkitAccess(false),
			}, tc.opts...)
			vm := gs.New(opts...)
			for _, src := range []string{
				`stats := testkit.memory()`,
				`info := testkit.value(42)`,
				`kind := testkit.typeOf(42)`,
				`result := testkit.protect(func() { return 1 })`,
				`same := testkit.sameFunction(print, print)`,
			} {
				err := vm.Exec(src)
				if err == nil || !strings.Contains(err.Error(), "testkit access disabled") {
					t.Fatalf("%s err = %v, want testkit access disabled", src, err)
				}
			}
		})
	}
}

func TestEnvironmentAllowlist(t *testing.T) {
	t.Setenv("GSCRIPT_PUBLIC_ENV_ALLOWED", "visible")
	t.Setenv("GSCRIPT_PUBLIC_ENV_BLOCKED", "secret")

	for _, tc := range []struct {
		name string
		opts []gs.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []gs.Option{gs.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]gs.Option{gs.WithEnvironmentAllowlist("GSCRIPT_PUBLIC_ENV_ALLOWED")}, tc.opts...)
			vm := gs.New(opts...)
			if err := vm.Exec(`
				allowed := os.getenv("GSCRIPT_PUBLIC_ENV_ALLOWED")
				blocked := os.getenv("GSCRIPT_PUBLIC_ENV_BLOCKED")
				expanded := os.expand("$GSCRIPT_PUBLIC_ENV_ALLOWED:$GSCRIPT_PUBLIC_ENV_BLOCKED")
				all := os.environ()
				procEnv := process.env()
			`); err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]interface{}{
				"allowed":  "visible",
				"blocked":  nil,
				"expanded": "visible:",
			} {
				got, err := vm.Get(name)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("%s = %v, want %v", name, got, want)
				}
			}
			for _, tableName := range []string{"all", "procEnv"} {
				got, err := vm.Get(tableName)
				if err != nil {
					t.Fatal(err)
				}
				env, ok := got.(map[string]interface{})
				if !ok {
					t.Fatalf("%s = %T, want map", tableName, got)
				}
				if env["GSCRIPT_PUBLIC_ENV_ALLOWED"] != "visible" {
					t.Fatalf("%s allowed = %v, want visible", tableName, env["GSCRIPT_PUBLIC_ENV_ALLOWED"])
				}
				if _, ok := env["GSCRIPT_PUBLIC_ENV_BLOCKED"]; ok {
					t.Fatalf("%s exposed blocked environment variable", tableName)
				}
			}
			err := vm.Exec(`os.setenv("GSCRIPT_PUBLIC_ENV_BLOCKED", "changed")`)
			if err == nil || !strings.Contains(err.Error(), "environment variable not allowed") {
				t.Fatalf("setenv blocked err = %v, want environment variable not allowed", err)
			}
		})
	}
}

func TestWithModuleLoadingFalseAllowsStdlibRequireButBlocksFileModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithModuleLoading(false))
	if err := vm.Exec(`result := type(require("json"))`); err != nil {
		t.Fatal(err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("stdlib require result = %v, want table", got)
	}
	err = vm.Exec(`require("helper")`)
	if err == nil || !strings.Contains(err.Error(), "module loading disabled") {
		t.Fatalf("require helper error = %v, want module loading disabled", err)
	}
}

func TestWithModuleLoadingFalseRestrictsBytecodeVM(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.gs"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := gs.New(gs.WithRequirePath(dir), gs.WithModuleLoading(false), gs.WithVM())
	if err := vm.Exec(`stdlibResult := type(require("json"))`); err != nil {
		t.Fatalf("stdlib require should still work with module loading disabled: %v", err)
	}
	got, err := vm.Get("stdlibResult")
	if err != nil {
		t.Fatal(err)
	}
	if got != "table" {
		t.Fatalf("stdlibResult = %v, want table", got)
	}
	err = vm.Exec(`require("helper")`)
	if err == nil {
		t.Fatal("expected require to fail when module loading is disabled")
	}
}

func TestEachPublicLibFlagExposesNamedGlobal(t *testing.T) {
	tests := []struct {
		name   string
		flag   gs.LibFlags
		global string
	}{
		{"bytes", gs.LibBytes, "bytes"},
		{"url", gs.LibURL, "url"},
		{"bits", gs.LibBits, "bits"},
		{"csv", gs.LibCSV, "csv"},
		{"uuid", gs.LibUUID, "uuid"},
		{"matrix", gs.LibMatrix, "matrix"},
		{"compress", gs.LibCompress, "compress"},
		{"container", gs.LibContainer, "container"},
		{"rl", gs.LibRL, "rl"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm := gs.New(gs.WithLibs(tc.flag))
			if err := vm.Exec(`result := type(` + tc.global + `)`); err != nil {
				t.Fatal(err)
			}
			got, err := vm.Get("result")
			if err != nil {
				t.Fatal(err)
			}
			if got != "table" {
				t.Fatalf("type(%s) = %v, want table", tc.global, got)
			}
		})
	}
}

// --- Integration: Go functions called from GScript ---

func TestIntegration_goFuncWithScriptCallback(t *testing.T) {
	var output []string
	vm := gs.New(gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	}))

	vm.RegisterFunc("applyTwice", func(x int64) int64 {
		return x * 2 * 2
	})

	err := vm.Exec(`
		result := applyTwice(5)
		print(result)
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0] != "20" {
		t.Fatalf("expected '20', got %v", output)
	}
}
