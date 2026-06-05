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

func TestReadmeQuickStartCommandsStayRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := readmeQuickStartCommands(string(data))
	want := []string{
		"go run ./cmd/leia help",
		`go run ./cmd/leia eval 'print("hello from leia")'`,
		"go run ./cmd/leia run tests/smoke/01_basic.leia",
		"go run ./cmd/leia run examples/hello/fib.leia",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("README Quick Start commands changed:\ngot  %#v\nwant %#v", got, want)
	}

	commands := []struct {
		args       []string
		wantStdout string
	}{
		{args: []string{"help"}, wantStdout: "usage: leia"},
		{args: []string{"eval", `print("hello from leia")`}, wantStdout: "hello from leia"},
		{args: []string{"run", "tests/smoke/01_basic.leia"}},
		{args: []string{"run", "examples/hello/fib.leia"}},
	}
	for _, command := range commands {
		cmd := exec.Command("go", append([]string{"run", "./cmd/leia"}, command.args...)...)
		cmd.Dir = root
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("README Quick Start command `go run ./cmd/leia %s` failed: %v\nstdout:\n%s\nstderr:\n%s",
				strings.Join(command.args, " "), err, stdout.String(), stderr.String())
		}
		if command.wantStdout != "" && !strings.Contains(stdout.String(), command.wantStdout) {
			t.Fatalf("README Quick Start command `go run ./cmd/leia %s` stdout = %q, want %q",
				strings.Join(command.args, " "), stdout.String(), command.wantStdout)
		}
	}
}

func TestReadmeInstallCommandsStayRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := readmeInstallCommands(string(data))
	want := []string{
		"go install ./cmd/leia",
		"leia version",
		"leia run tests/smoke/01_basic.leia",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("README install commands changed:\ngot  %#v\nwant %#v", got, want)
	}

	gobin := t.TempDir()
	env := append(os.Environ(), "GOBIN="+gobin)
	install := exec.Command("go", "install", "./cmd/leia")
	install.Dir = root
	install.Env = env
	var installStdout, installStderr bytes.Buffer
	install.Stdout = &installStdout
	install.Stderr = &installStderr
	if err := install.Run(); err != nil {
		t.Fatalf("README install command `go install ./cmd/leia` failed: %v\nstdout:\n%s\nstderr:\n%s",
			err, installStdout.String(), installStderr.String())
	}

	leia := filepath.Join(gobin, "leia")
	commands := [][]string{
		{"version"},
		{"run", "tests/smoke/01_basic.leia"},
	}
	for _, args := range commands {
		cmd := exec.Command(leia, args...)
		cmd.Dir = root
		cmd.Env = env
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("README install command `leia %s` failed: %v\nstdout:\n%s\nstderr:\n%s",
				strings.Join(args, " "), err, stdout.String(), stderr.String())
		}
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

func readmeQuickStartCommands(readme string) []string {
	const marker = "## Quick Start"
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

func readmeInstallCommands(readme string) []string {
	const marker = "Install the CLI from a checkout:"
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
