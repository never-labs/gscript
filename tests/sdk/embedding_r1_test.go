package leia_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
)

func TestProductionEmbeddingSandboxGoImportBudgetAndHotReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "policy.leia")
	writeScript(t, path, `
import "go:host/safe" as host

state := { accepted: 0 }

func handle(input) {
	state.accepted += 1
	return host.label(input, state.accepted)
}
`)

	loader := leia.NewHotLoader(leia.WithHotLoaderVMOptions(
		leia.SecuritySandbox(),
		leia.WithLibs(leia.LibSafe),
		leia.WithGoImports(map[string]any{
			"go:host/safe": leia.Module{
				"label": func(name string, n int64) string {
					return fmt.Sprintf("%s-%02d", strings.ToUpper(name), n)
				},
				"large": func() string {
					return strings.Repeat("x", 65)
				},
			},
		}),
		leia.WithEnvironmentAllowlist("LEIA_EMBEDDING_MODE"),
		leia.WithMaxSteps(2_000),
		leia.WithMaxNativeCalls(64),
		leia.WithMaxHostResultBytes(64),
	))
	inst, err := loader.LoadInstance(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCallResult(t, inst, "handle", "job", "JOB-01")

	err = inst.VM().Exec(`import "go:os" as os`)
	if err == nil || !strings.Contains(err.Error(), `go import "go:os" is not allowed`) {
		t.Fatalf("unauthorized go import error = %v, want go:os rejection", err)
	}

	err = inst.VM().Exec(`
import "go:host/safe" as host
too_large := host.large()
`)
	var budgetErr *leia.BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Resource != "host_result_bytes" || budgetErr.Limit != 64 {
		t.Fatalf("host result budget error = %T %v, want host_result_bytes limit 64", err, err)
	}

	writeScript(t, path, `
import "go:host/safe" as host

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
	assertCallResult(t, inst, "handle", "job", "JOB-11")
}
