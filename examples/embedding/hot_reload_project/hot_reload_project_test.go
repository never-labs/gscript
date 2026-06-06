package hot_reload_project_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestEmbeddedHotReloadProjectPreservesStateAndRollback(t *testing.T) {
	path := copyFixture(t, filepath.Join("testdata", "policy.leia"))

	loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(
		leia.SecuritySandbox(),
		leia.WithLibs(leia.LibSafe),
		leia.WithGoImports(map[string]any{
			"go:host/policy": leia.Module{
				"label": func(name string, n int64) string {
					return fmt.Sprintf("%s-%02d", strings.ToUpper(name), n)
				},
				"large": func() string {
					return strings.Repeat("x", 65)
				},
			},
		}),
		leia.WithMaxSteps(2_000),
		leia.WithMaxNativeCalls(16),
		leia.WithMaxHostResultBytes(64),
	))

	inst, err := loader.LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "handle", "job", "JOB-01")
	assertCallResult(t, inst, "handle", "job", "JOB-02")

	if err := inst.VM().Exec(`import "go:os" as os`); err == nil || !strings.Contains(err.Error(), `go import "go:os" is not allowed`) {
		t.Fatalf("unauthorized import error = %v, want go:os rejection", err)
	}

	writeScript(t, path, `
import "go:host/policy" as host

state := { accepted: 0 }

func handle(input) {
	state.accepted += 10
	return host.label(input, state.accepted)
}
`)
	if err := inst.Reload(); err != nil {
		t.Fatal(err)
	}
	if inst.Generation() != 2 {
		t.Fatalf("generation = %d, want 2", inst.Generation())
	}
	assertCallResult(t, inst, "handle", "job", "JOB-12")

	writeScript(t, path, `func handle(input) {`)
	if err := inst.Reload(); err == nil {
		t.Fatal("expected failed reload for invalid replacement")
	}
	if inst.Generation() != 2 {
		t.Fatalf("generation after failed reload = %d, want 2", inst.Generation())
	}
	assertCallResult(t, inst, "handle", "job", "JOB-22")

	writeScript(t, path, `
import "go:host/policy" as host

state := { accepted: 0 }
probe := host.large()

func handle(input) {
	state.accepted += 100
	return host.large()
}
`)
	err = inst.Reload()
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 64 {
		t.Fatalf("reload error = %T %v, want host_result_bytes budget error at 64", err, err)
	}
	if inst.Generation() != 2 {
		t.Fatalf("generation after budget failure = %d, want 2", inst.Generation())
	}
	assertCallResult(t, inst, "handle", "job", "JOB-32")
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	path := filepath.Join(t.TempDir(), filepath.Base(name))
	if err := os.WriteFile(path, src, 0644); err != nil {
		t.Fatalf("copy fixture %s: %v", name, err)
	}
	return path
}

func writeScript(t *testing.T, path, src string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatalf("write script %s: %v", path, err)
	}
}

func assertCallResult(t *testing.T, inst *leia.HotInstance, name string, argsAndWant ...interface{}) {
	t.Helper()
	if len(argsAndWant) == 0 {
		t.Fatal("assertCallResult needs a want value")
	}
	want := argsAndWant[len(argsAndWant)-1]
	args := argsAndWant[:len(argsAndWant)-1]
	got, err := inst.Call(name, args...)
	if err != nil {
		t.Fatalf("%s(%v): %v", name, args, err)
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s(%v) = %#v, want %#v", name, args, got, want)
	}
}
