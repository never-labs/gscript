package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestReadmeToolingCommandsDoNotDrift(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := readmeToolingCommands(string(data))
	want := []string{
		"go run ./cmd/leia fmt --check tests/smoke/01_basic.leia",
		"go run ./cmd/leia lint tests/smoke/01_basic.leia",
		"go run ./cmd/leia test tests/smoke/01_basic.leia",
		"go run ./cmd/leia check --no-docs --no-editor --no-examples .",
		"go run ./cmd/leia examples check examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia",
		"go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("README Tooling commands changed:\ngot  %#v\nwant %#v", got, want)
	}
}

func TestReadmeToolingCommandsMapToCLI(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	commands := readmeToolingCommands(string(data))
	if len(commands) == 0 {
		t.Fatal("README Tooling commands are empty")
	}

	oldBenchExecCommand := benchExecCommand
	oldDiagExecCommand := diagExecCommand
	t.Cleanup(func() {
		benchExecCommand = oldBenchExecCommand
		diagExecCommand = oldDiagExecCommand
	})
	var benchArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		benchArgs = append([]string{name}, args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}
	var diagArgs []string
	diagExecCommand = func(name string, args ...string) *exec.Cmd {
		diagArgs = append([]string{name}, args...)
		helper, helperArgs := testHelperCommand(t, "diag")
		return exec.Command(helper, helperArgs...)
	}

	for _, command := range commands {
		args := strings.Fields(strings.TrimPrefix(command, "go run ./cmd/leia "))
		if len(args) == 0 {
			t.Fatalf("empty README command args for %q", command)
		}
		switch args[0] {
		case "fmt", "lint", "test", "check":
			if !knownLeiaCommand(args[0]) {
				t.Fatalf("README command %q is not registered", args[0])
			}
		case "examples":
			for _, selector := range args[2:] {
				if strings.HasPrefix(selector, "-") {
					continue
				}
				if _, _, ok, err := resolveCLIExample(selector); err != nil || !ok {
					t.Fatalf("README examples selector %q resolved ok=%t err=%v", selector, ok, err)
				}
			}
		case "bench":
			var stdout, stderr bytes.Buffer
			if code := runBenchCommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README bench command failed dispatch: code=%d stderr=%q", code, stderr.String())
			}
		case "diag":
			var stdout, stderr bytes.Buffer
			if code := runDiagCommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README diag command failed dispatch: code=%d stderr=%q", code, stderr.String())
			}
		default:
			t.Fatalf("README command %q is not part of the tooling audit", args[0])
		}
	}
	if len(benchArgs) == 0 || !containsString(benchArgs, "numeric/mandelbrot") || !containsString(benchArgs, "--runs") {
		t.Fatalf("README bench dispatch args = %#v, want numeric/mandelbrot compare", benchArgs)
	}
	if len(diagArgs) == 0 || !containsString(diagArgs, "--skip-benchmarks") {
		t.Fatalf("README diag dispatch args = %#v, want bundle --skip-benchmarks", diagArgs)
	}
}

func readmeToolingCommands(readme string) []string {
	const marker = "## Tooling"
	start := strings.Index(readme, marker)
	if start < 0 {
		return nil
	}
	rest := readme[start+len(marker):]
	blockStart := strings.Index(rest, "```bash")
	if blockStart < 0 {
		return nil
	}
	rest = rest[blockStart+len("```bash"):]
	blockEnd := strings.Index(rest, "```")
	if blockEnd < 0 {
		return nil
	}
	lines := strings.Split(rest[:blockEnd], "\n")
	var commands []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			commands = append(commands, line)
		}
	}
	return commands
}

func knownLeiaCommand(name string) bool {
	for _, command := range cliCommands() {
		if command.Name == name {
			return true
		}
	}
	return false
}
