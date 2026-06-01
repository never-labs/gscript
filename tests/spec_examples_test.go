package tests_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSpecRunnableExamplesInterpreterSemantics(t *testing.T) {
	root := findRepoRoot(t)
	examples := collectRunnableSpecExamples(t, root)
	if len(examples) == 0 {
		t.Fatal("docs/spec must contain at least one ```leia run example")
	}
	for _, example := range examples {
		example := example
		t.Run(example.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "example.leia")
			if err := os.WriteFile(path, []byte(example.source), 0o644); err != nil {
				t.Fatalf("write example: %v", err)
			}
			runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "run", "-jit=false", path)
		})
	}
}

type runnableSpecExample struct {
	name   string
	source string
}

func collectRunnableSpecExamples(t *testing.T, root string) []runnableSpecExample {
	t.Helper()
	specDir := filepath.Join(root, "docs", "spec")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Fatalf("read docs/spec: %v", err)
	}
	var examples []runnableSpecExample
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		lines := strings.Split(readFileString(t, path), "\n")
		inRunnableFence := false
		startLine := 0
		var block []string
		for i, line := range lines {
			lineNo := i + 1
			if strings.HasPrefix(line, "```") {
				info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if inRunnableFence {
					examples = append(examples, runnableSpecExample{
						name:   strings.TrimSuffix(entry.Name(), ".md") + "_line_" + strconv.Itoa(startLine),
						source: strings.Join(block, "\n") + "\n",
					})
					inRunnableFence = false
					block = nil
					continue
				}
				if info == "leia run" {
					inRunnableFence = true
					startLine = lineNo
					block = nil
				}
				continue
			}
			if inRunnableFence {
				block = append(block, line)
			}
		}
		if inRunnableFence {
			t.Fatalf("%s:%d: unclosed runnable spec example fence", entry.Name(), startLine)
		}
	}
	return examples
}
