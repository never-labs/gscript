package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadmeIntroStaysFocused(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	for _, want := range []string{
		"Leia is an embeddable scripting language for Go systems.",
		"Go-style syntax",
		"typed hot-path optimization",
		"native q-style columnar analytics",
		"high-performance in-memory data",
		"tagged dialects",
		"AI is a dialect/stdlib layer, not an AI-native runtime or the language core.",
		"## Surface",
		"## Tooling",
		"## References",
		"model: \"claude\"",
		"q```",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing focused positioning snippet %q", want)
		}
	}
	for _, forbidden := range []string{"## Quick Start", "## Install", "## Project Status", "AI-native syntax", "Go-native runtime"} {
		if strings.Contains(readme, forbidden) {
			t.Fatalf("README must not contain template section %q", forbidden)
		}
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestReadmeMainLeiaExampleStaysRunnableToProviderBoundary(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	snippet := readmeFirstLeiaSnippet(string(data))
	if snippet == "" {
		t.Fatal("README must contain a Leia example")
	}
	for _, want := range []string{"q```", "model {", "agent {", "prompt {", "model: \"claude\""} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("README Leia example missing %q:\n%s", want, snippet)
		}
	}
	file := filepath.Join(t.TempDir(), "readme.leia")
	if err := os.WriteFile(file, []byte(snippet), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/leia", "run", file)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("README Leia example failed before provider boundary: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "llm provider not configured") {
		t.Fatalf("README Leia example stdout = %q, want provider boundary message", stdout.String())
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
			for _, want := range []string{"fmt: ok", "lint: ok", "test: ok", "manifest: skipped", "docs: skipped", "editor: skipped", "examples: skipped"} {
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
		case "playground":
			if len(args) != 2 || args[1] != "--help" {
				t.Fatalf("README playground command args = %#v, want --help", args)
			}
			playgroundGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_playground_test.go"))
			for _, want := range []string{
				"TestReadmePlaygroundTabsMatchAPISurface",
				"go run ./cmd/leia playground --help",
				`data-tab="evaluate"`,
				`url: "/api/examples"`,
				`url: "/api/ai"`,
			} {
				if !strings.Contains(playgroundGate, want) {
					t.Fatalf("playground README guard missing %q", want)
				}
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
			for _, want := range []string{"bash scripts/performance_gate.sh --full", "bash scripts/production_check.sh --full --release-profile", "bash scripts/release_distribution_check.sh --require-goreleaser", "bash scripts/release_artifacts_check.sh --build"} {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("README ci stdout = %q, want %q", stdout.String(), want)
				}
			}
		default:
			t.Fatalf("README command %q is not part of the tooling audit", args[0])
		}
	}
	_ = checkArgs
	if len(docArgs) > 0 && (len(docArgs) < 2 || !strings.HasSuffix(docArgs[1], filepath.Join("scripts", "docs_check.sh"))) {
		t.Fatalf("README doc dispatch args = %#v, want doc check via scripts/docs_check.sh", docArgs)
	}
	if len(benchArgs) == 0 || !containsString(benchArgs, "data/q_operator_pipeline") || !containsString(benchArgs, "--runs") {
		t.Fatalf("README bench dispatch args = %#v, want q operator pipeline compare", benchArgs)
	}
	if len(diagArgs) > 0 && !containsString(diagArgs, "--skip-benchmarks") {
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

func readmeFirstLeiaSnippet(readme string) string {
	for _, marker := range []string{"````leia", "```leia"} {
		start := strings.Index(readme, marker)
		if start < 0 {
			continue
		}
		rest := readme[start+len(marker):]
		endMarker := strings.Repeat("`", strings.Count(marker, "`"))
		blockEnd := strings.Index(rest, endMarker)
		if blockEnd < 0 {
			return ""
		}
		return strings.TrimSpace(rest[:blockEnd]) + "\n"
	}
	return ""
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
