package gscript_test

import (
	"path/filepath"
	"runtime"
	"testing"

	gs "github.com/never-labs/gscript"
)

func llmScenarioOptions(provider gs.LLMProvider, opts ...gs.Option) []gs.Option {
	base := []gs.Option{
		gs.WithLibs(gs.LibString | gs.LibLLM),
		gs.WithLLMProvider(provider),
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
