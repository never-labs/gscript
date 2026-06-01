package leia_test

import (
	"path/filepath"
	"runtime"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

func llmScenarioOptions(provider llm.Provider, opts ...leia.Option) []leia.Option {
	base := []leia.Option{
		leia.WithLibs(leia.LibString | leia.LibLLM),
		leia.WithLLMProvider(provider),
	}
	return append(base, opts...)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
