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
		"Leia is a Go-native embedded scripting language with JIT execution, q-style in-memory analytics, and first-class extensible dialects.",
		"Go-native:",
		"Performance-oriented:",
		"LuaJIT-class workloads",
		"Analytics-native:",
		"q-style vector syntax",
		"Dialect-native:",
		"native DSL extension lets domain syntax live beside Leia code",
		"## References",
		"[Documentation](docs/index.md)",
		"[Playground](docs/playground.md)",
		"[CLI reference](docs/reference/cli/index.md)",
		"[Optional LLM dialect](docs/reference/ai/index.md)",
		"q`sum ${a}`",
		"turn {",
		"prompt {",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README missing focused positioning snippet %q", want)
		}
	}
	for _, forbidden := range []string{
		"## Quick Start",
		"## Install",
		"## Project Status",
		"## Tooling",
		"Performance claims are benchmark-bound",
		"AI" + "-native syntax",
		"AI" + "-native runtime",
	} {
		if strings.Contains(readme, forbidden) {
			t.Fatalf("README must not contain template section %q", forbidden)
		}
	}
}

func TestTopLevelHelpShowsCommandList(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	cmd := exec.Command("go", "run", "./cmd/leia", "--help")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("leia --help failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	for _, want := range []string{"usage: leia <command> [args]", "Commands:", "examples", "help"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("leia --help stdout = %q, want %q", stdout.String(), want)
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
	for _, want := range []string{"a := [1,2,3,4,5,6,7,8,6]", "x := q`sum ${a}`", "turn {", "prompt {", "print(x)"} {
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
		t.Fatalf("README Leia example failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "42" {
		t.Fatalf("README Leia example stdout = %q, want 42 fallback without host LLM provider", stdout.String())
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
	if !strings.Contains(stdout.String(), "AAPL") || !strings.Contains(stdout.String(), "18") || !strings.Contains(stdout.String(), "100.375") {
		t.Fatalf("README embedding snippet stdout = %q, want q analytics result", stdout.String())
	}
}

func readmeEmbeddingGoSnippet(readme string) string {
	rest := readme
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
