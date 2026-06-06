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
		"go run ./cmd/leia doc check",
		"go run ./cmd/leia mod verify --json examples/ui/package_managed",
		"go run ./cmd/leia bench compare --bench numeric/mandelbrot --runs 3 --warmup 1",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks",
		"go run ./cmd/leia ci release --list",
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

func TestReadmeEmbeddingSnippetStaysRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	snippet := readmeEmbeddingGoSnippet(string(data))
	if snippet == "" {
		t.Fatal("README Embedding section must contain a Go snippet")
	}
	for _, want := range []string{
		`import leia "github.com/never-labs/leia"`,
		"leia.New(leia.WithLibs(leia.LibSafe))",
		`vm.Exec(`,
	} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("README embedding snippet missing %q:\n%s", want, snippet)
		}
	}

	dir := t.TempDir()
	goMod := "module readme_embedding_smoke\n\ngo 1.24\n\nrequire github.com/never-labs/leia v0.0.0\n\nreplace github.com/never-labs/leia => " + filepath.ToSlash(root) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(snippet), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "run", "-mod=mod", ".")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("README embedding snippet failed: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "hello from embedded leia") {
		t.Fatalf("README embedding snippet stdout = %q, want embedded hello", stdout.String())
	}
}

func TestReadmeToolingCommandsMapToCLI(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	commands := readmeToolingCommands(string(data))
	if len(commands) == 0 {
		t.Fatal("README Tooling commands are empty")
	}

	oldBenchExecCommand := benchExecCommand
	oldCheckExecCommand := checkExecCommand
	oldDiagExecCommand := diagExecCommand
	oldDocExecCommand := docExecCommand
	t.Cleanup(func() {
		benchExecCommand = oldBenchExecCommand
		checkExecCommand = oldCheckExecCommand
		diagExecCommand = oldDiagExecCommand
		docExecCommand = oldDocExecCommand
	})
	var benchArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		benchArgs = append([]string{name}, args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}
	var checkArgs []string
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		checkArgs = append([]string{name}, args...)
		helper, helperArgs := testHelperCommand(t, "manifest")
		return exec.Command(helper, helperArgs...)
	}
	var diagArgs []string
	diagExecCommand = func(name string, args ...string) *exec.Cmd {
		diagArgs = append([]string{name}, args...)
		helper, helperArgs := testHelperCommand(t, "diag")
		return exec.Command(helper, helperArgs...)
	}
	var docArgs []string
	docExecCommand = func(name string, args ...string) *exec.Cmd {
		docArgs = append([]string{name}, args...)
		helper, helperArgs := testHelperCommand(t, "doc")
		return exec.Command(helper, helperArgs...)
	}

	for _, command := range commands {
		args := strings.Fields(strings.TrimPrefix(command, "go run ./cmd/leia "))
		if len(args) == 0 {
			t.Fatalf("empty README command args for %q", command)
		}
		spec, ok := lookupCLICommand(args[0])
		if !ok {
			t.Fatalf("README command %q is not registered", args[0])
		}
		switch args[0] {
		case "fmt", "lint", "test":
			var stdout, stderr bytes.Buffer
			if code := spec.Run(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README %s command failed: code=%d stdout=%q stderr=%q", args[0], code, stdout.String(), stderr.String())
			}
		case "check":
			var stdout, stderr bytes.Buffer
			if code := runCheckCommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README check command failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			for _, want := range []string{"fmt: ok", "lint: ok", "test: ok", "manifest: ok", "docs: skipped", "editor: skipped", "examples: skipped"} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("README check stdout = %q, want %q", stdout.String(), want)
				}
			}
		case "examples":
			var stdout, stderr bytes.Buffer
			if code := runExamplesCommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README examples command failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			for _, selector := range []string{"repo-hello-fib", "repo-hello-types_demo", "repo-hello-dialects"} {
				if !strings.Contains(stdout.String(), selector) {
					t.Fatalf("README examples stdout = %q, want %q", stdout.String(), selector)
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
		case "doc":
			var stdout, stderr bytes.Buffer
			if code := runDocCommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README doc command failed dispatch: code=%d stderr=%q", code, stderr.String())
			}
		case "mod":
			var stdout, stderr bytes.Buffer
			if code := runModCommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README mod command failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		case "ci":
			var stdout, stderr bytes.Buffer
			if code := runCICommand(args[1:], &stdout, &stderr); code != 0 {
				t.Fatalf("README ci command failed: code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			for _, want := range []string{"bash scripts/performance_gate.sh --full", "bash scripts/release_distribution_check.sh", "bash scripts/release_artifacts_check.sh --build"} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("README ci stdout = %q, want %q", stdout.String(), want)
				}
			}
		default:
			t.Fatalf("README command %q is not part of the tooling audit", args[0])
		}
	}
	if len(checkArgs) < 2 || !strings.HasSuffix(checkArgs[1], filepath.Join("tests", "manifest.py")) || !containsString(checkArgs, "tests") || !containsString(checkArgs, "benchmarks") {
		t.Fatalf("README check dispatch args = %#v, want manifest coverage via tests/manifest.py", checkArgs)
	}
	if len(docArgs) < 2 || !strings.HasSuffix(docArgs[1], filepath.Join("scripts", "docs_check.sh")) {
		t.Fatalf("README doc dispatch args = %#v, want doc check via scripts/docs_check.sh", docArgs)
	}
	if len(benchArgs) == 0 || !containsString(benchArgs, "numeric/mandelbrot") || !containsString(benchArgs, "--runs") {
		t.Fatalf("README bench dispatch args = %#v, want numeric/mandelbrot compare", benchArgs)
	}
	if len(diagArgs) == 0 || !containsString(diagArgs, "--skip-benchmarks") {
		t.Fatalf("README diag dispatch args = %#v, want bundle --skip-benchmarks", diagArgs)
	}
}

func readmeEmbeddingGoSnippet(readme string) string {
	const marker = "## Embedding"
	start := strings.Index(readme, marker)
	if start < 0 {
		return ""
	}
	rest := readme[start+len(marker):]
	blockStart := strings.Index(rest, "```go")
	if blockStart < 0 {
		return ""
	}
	rest = rest[blockStart+len("```go"):]
	blockEnd := strings.Index(rest, "```")
	if blockEnd < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:blockEnd]) + "\n"
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
