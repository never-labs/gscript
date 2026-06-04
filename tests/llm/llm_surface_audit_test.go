package leia_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLLMExamplesStayOnStdlibAISurface(t *testing.T) {
	exampleDir := filepath.Join(repoRoot(t), "examples", "llm")
	entries, err := os.ReadDir(exampleDir)
	if err != nil {
		t.Fatalf("ReadDir examples/llm: %v", err)
	}

	legacyForms := map[string]bool{
		"agent":    true,
		"models":   true,
		"tool":     true,
		"turn":     true,
		"messages": true,
		"evaluate": true,
		"flow":     true,
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
			continue
		}
		if strings.Contains(entry.Name(), "ai_native") {
			t.Fatalf("examples/llm contains legacy ai_native filename %q", entry.Name())
		}

		path := filepath.Join(exampleDir, entry.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", path, err)
		}
		for lineNo, line := range strings.Split(string(src), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "//") {
				continue
			}
			first, _, _ := strings.Cut(trimmed, " ")
			first, _, _ = strings.Cut(first, "{")
			if !legacyForms[first] || !strings.Contains(trimmed, "{") {
				continue
			}
			if strings.Contains(trimmed, ":") || strings.Contains(trimmed, ":=") || strings.Contains(trimmed, ".") {
				continue
			}
			t.Fatalf("%s:%d appears to use legacy AI block syntax: %s", path, lineNo+1, trimmed)
		}
	}
}
