package gscript_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/never-labs/gscript"
)

type facadeLLMProvider struct{}

func (facadeLLMProvider) Turn(context.Context, gscript.LLMTurnRequest) (gscript.LLMTurnResult, error) {
	return gscript.LLMTurnResult{Status: "final_answer", Text: "ok"}, nil
}

func TestRootFacadeEmbeddingAPI(t *testing.T) {
	prog, err := gscript.Compile(`
		func add(a, b) {
			return a + b
		}
		result := add(seed, 5)
	`, gscript.WithSourceName("facade.gs"))
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if got := prog.SourceName(); got != "facade.gs" {
		t.Fatalf("SourceName = %q", got)
	}

	vm := gscript.New(gscript.WithLibs(gscript.LibSafe))
	if err := vm.Set("seed", gscript.Int(37)); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := vm.Run(prog); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, err := vm.Get("result")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != int64(42) {
		t.Fatalf("result = %#v", got)
	}
}

func TestRootFacadeModulePoolAndLLMAPI(t *testing.T) {
	vm := gscript.New(
		gscript.WithLibs(gscript.LibSafe|gscript.LibLLM),
		gscript.WithLLMProvider(facadeLLMProvider{}),
	)
	if err := vm.RegisterModule("go:labels", gscript.Module{
		"format": func(prefix string, id int64) string {
			return fmt.Sprintf("%s-%d", prefix, id)
		},
	}); err != nil {
		t.Fatalf("RegisterModule: %v", err)
	}

	pool := gscript.NewPool(1, func() *gscript.VM { return vm })
	if err := pool.Do(func(pooled *gscript.VM) error {
		return pooled.Exec(`
labels := require("go:labels")
tag := labels.format("job", 7)
`)
	}); err != nil {
		t.Fatalf("Pool.Do: %v", err)
	}
	if got, err := vm.Get("tag"); err != nil || got != "job-7" {
		t.Fatalf("tag = %#v, err = %v", got, err)
	}

	recorder := gscript.NewLLMRecorder()
	if recorder == nil {
		t.Fatal("NewLLMRecorder returned nil")
	}
	var _ gscript.LLMProvider = facadeLLMProvider{}
}
