package tests_test

import (
	"fmt"
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
		t.Fatal("docs/spec must contain at least one stable runnable example fence: ```leia run all")
	}
	t.Log(specRunnableCoverageReport(examples))
	leiaBin := filepath.Join(t.TempDir(), "leia")
	runCommand(t, root, 60*time.Second, "go", "build", "-o", leiaBin, "./cmd/leia")
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
					args := append([]string{"run"}, mode.flags...)
					args = append(args, path)
					result := runCommandResult(root, 30*time.Second, leiaBin, args...)
					if mode.wantFailure {
						if result.timedOut {
							t.Fatalf("%s timed out after %s", commandLine(leiaBin, args), 30*time.Second)
						}
						if result.err == nil {
							t.Fatalf("%s succeeded; expected failure\nstdout:\n%s\nstderr:\n%s", commandLine(leiaBin, args), result.stdout, result.stderr)
						}
						return
					}
					if result.err != nil {
						if result.timedOut {
							t.Fatalf("%s timed out after %s", commandLine(leiaBin, args), 30*time.Second)
						}
						t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", commandLine(leiaBin, args), result.err, result.stdout, result.stderr)
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
			if info == "leia run" || info == "leia fail" {
				bad = append(bad, entry.Name()+":"+strconv.Itoa(i+1)+": stable spec examples must use all execution modes: use ```leia run all or ```leia fail all")
				continue
			}
			if _, ok := specExampleModes(info); ok {
				continue
			}
			bad = append(bad, entry.Name()+":"+strconv.Itoa(i+1)+": ```"+info)
		}
	}
	if len(bad) > 0 {
		t.Fatalf("docs/spec Leia code fences must use a stable spec gate (leia run all or leia fail all) or a non-Leia info string:\n%s", strings.Join(bad, "\n"))
	}
}

type runnableSpecExample struct {
	name   string
	file   string
	line   int
	info   string
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
						file:   entry.Name(),
						line:   startLine,
						info:   strings.TrimSpace(strings.TrimPrefix(lines[startLine-1], "```")),
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

func specRunnableCoverageReport(examples []runnableSpecExample) string {
	type fileCoverage struct {
		run  int
		fail int
	}

	byFile := make(map[string]fileCoverage)
	var run, fail int
	for _, example := range examples {
		coverage := byFile[example.file]
		if strings.HasPrefix(example.info, "leia fail") {
			fail++
			coverage.fail++
		} else {
			run++
			coverage.run++
		}
		byFile[example.file] = coverage
	}

	var b strings.Builder
	fmt.Fprintf(&b, "docs/spec runnable Leia coverage: %d examples (%d run, %d fail) across %d files", len(examples), run, fail, len(byFile))
	emitted := make(map[string]bool)
	for _, example := range examples {
		if emitted[example.file] {
			continue
		}
		coverage := byFile[example.file]
		fmt.Fprintf(&b, "\n  %s: %d run, %d fail", example.file, coverage.run, coverage.fail)
		emitted[example.file] = true
	}
	return b.String()
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
