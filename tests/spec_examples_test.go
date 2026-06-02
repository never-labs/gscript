package tests_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSpecRunnableExamples(t *testing.T) {
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
			for _, mode := range example.modes {
				mode := mode
				t.Run(mode.name, func(t *testing.T) {
					args := append([]string{"run", "./cmd/leia", "run"}, mode.flags...)
					args = append(args, path)
					result := runCommandResult(root, 30*time.Second, "go", args...)
					if mode.wantFailure {
						if result.timedOut {
							t.Fatalf("%s timed out after %s", commandLine("go", args), 30*time.Second)
						}
						if result.err == nil {
							t.Fatalf("%s succeeded; expected failure\nstdout:\n%s\nstderr:\n%s", commandLine("go", args), result.stdout, result.stderr)
						}
						return
					}
					if result.err != nil {
						if result.timedOut {
							t.Fatalf("%s timed out after %s", commandLine("go", args), 30*time.Second)
						}
						t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", commandLine("go", args), result.err, result.stdout, result.stderr)
					}
				})
			}
		})
	}
}

func TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable(t *testing.T) {
	root := findRepoRoot(t)
	specDir := filepath.Join(root, "docs", "spec")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Fatalf("read docs/spec: %v", err)
	}
	var bad []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		lines := strings.Split(readFileString(t, path), "\n")
		for i, line := range lines {
			if !strings.HasPrefix(line, "```") {
				continue
			}
			info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
			if !strings.HasPrefix(info, "leia") {
				continue
			}
			if _, ok := specExampleModes(info); ok {
				continue
			}
			bad = append(bad, entry.Name()+":"+strconv.Itoa(i+1)+": ```"+info)
		}
	}
	if len(bad) > 0 {
		t.Fatalf("docs/spec Leia code fences must use a runnable gate (leia run, leia run all, leia fail, leia fail all) or a non-Leia info string:\n%s", strings.Join(bad, "\n"))
	}
}

type runnableSpecExample struct {
	name   string
	source string
	modes  []specExampleMode
}

type specExampleMode struct {
	name        string
	flags       []string
	wantFailure bool
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
		var currentModes []specExampleMode
		for i, line := range lines {
			lineNo := i + 1
			if strings.HasPrefix(line, "```") {
				info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
				if inRunnableFence {
					examples = append(examples, runnableSpecExample{
						name:   strings.TrimSuffix(entry.Name(), ".md") + "_line_" + strconv.Itoa(startLine),
						source: strings.Join(block, "\n") + "\n",
						modes:  append([]specExampleMode(nil), currentModes...),
					})
					inRunnableFence = false
					block = nil
					currentModes = nil
					continue
				}
				modes, ok := specExampleModes(info)
				if ok {
					inRunnableFence = true
					startLine = lineNo
					block = nil
					currentModes = modes
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

func specExampleModes(info string) ([]specExampleMode, bool) {
	switch info {
	case "leia run":
		return []specExampleMode{{name: "interpreter", flags: []string{"-jit=false"}}}, true
	case "leia run all":
		return []specExampleMode{
			{name: "interpreter", flags: []string{"-jit=false"}},
			{name: "vm", flags: []string{"-vm"}},
			{name: "default", flags: nil},
		}, true
	case "leia fail":
		return []specExampleMode{{name: "interpreter", flags: []string{"-jit=false"}, wantFailure: true}}, true
	case "leia fail all":
		return []specExampleMode{
			{name: "interpreter", flags: []string{"-jit=false"}, wantFailure: true},
			{name: "vm", flags: []string{"-vm"}, wantFailure: true},
			{name: "default", flags: nil, wantFailure: true},
		}, true
	default:
		return nil, false
	}
}
