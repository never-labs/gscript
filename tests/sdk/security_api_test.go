package leia_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestWithLibsRestrictsUnsafeGlobals(t *testing.T) {
	vm := leia.New(leia.WithLibs(leia.LibSafe))
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
	vm := leia.New(leia.WithLibs(leia.LibSafe), leia.WithVM())
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
	vm := leia.New(leia.WithSandbox())
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
	vm := leia.New(leia.WithJIT(), leia.SecuritySandbox(), leia.WithMaxSteps(16))
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
	var budgetErr *leia.BudgetError
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
		opts []leia.Option
	}{
		{name: "interpreter"},
		{name: "bytecode", opts: []leia.Option{leia.WithVM()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := append([]leia.Option{leia.WithDynamicEval(false)}, tc.opts...)
			vm := leia.New(opts...)
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
	vm := leia.New(leia.WithJIT(), leia.WithSecurity(leia.SecurityPolicy{
		Libs:                    leia.LibSafe,
		Capabilities:            leia.CapSafe,
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
		EnvironmentAllowlist:    []string{"LEIA_PUBLIC_ENV_CAP_TEST"},
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
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 4 {
		t.Fatalf("expected host_result_bytes budget 4, got %T %v", err, err)
	}
	err = vm.Exec(`for {}`)
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "steps" || budgetErr.Limit != 32 {
		t.Fatalf("expected steps budget 32, got %T %v", err, err)
	}
}

func TestWithModuleLoadingFalseAllowsStdlibRequireButBlocksFileModules(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "helper.leia"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithRequirePath(dir), leia.WithModuleLoading(false))
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
	if err := os.WriteFile(filepath.Join(dir, "helper.leia"), []byte(`return { value: 42 }`), 0644); err != nil {
		t.Fatal(err)
	}
	vm := leia.New(leia.WithRequirePath(dir), leia.WithModuleLoading(false), leia.WithVM())
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
