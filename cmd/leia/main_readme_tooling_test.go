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
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
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
		"## References",
		"leia.New(leia.WithLibs",
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

func TestDocsHomeMainLeiaExampleStaysRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	snippet := readmeFirstLeiaSnippet(string(data))
	if snippet == "" {
		t.Fatal("docs/index.md must contain a Leia example")
	}
	for _, want := range []string{"trades := q```", "q.sql(", "print(leader[1].sym"} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("docs home Leia example missing %q:\n%s", want, snippet)
		}
	}
	file := filepath.Join(t.TempDir(), "docs-home.leia")
	if err := os.WriteFile(file, []byte(snippet), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/leia", "run", file)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("docs home Leia example failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if got := strings.TrimSpace(stdout.String()); got != "AAPL\t18\t100.375" {
		t.Fatalf("docs home Leia example stdout = %q, want qSQL rollup", stdout.String())
	}
}

func TestReferenceDialectsIntroExampleStaysRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "reference", "dialects", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	snippet := readmeFirstLeiaSnippet(string(data))
	if snippet == "" {
		t.Fatal("docs/reference/dialects/index.md must contain a Leia example")
	}
	for _, want := range []string{"name := \"Leia\"", "sh`git status --short`", "json`{\"name\": ${name}}`", "agent {"} {
		if !strings.Contains(snippet, want) {
			t.Fatalf("dialects reference Leia example missing %q:\n%s", want, snippet)
		}
	}
	file := filepath.Join(t.TempDir(), "dialects-reference.leia")
	if err := os.WriteFile(file, []byte(snippet), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/leia", "run", file)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("dialects reference Leia example failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
}

func TestReferenceDataOrientedExamplesStayRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "reference", "data-oriented", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := markdownLeiaSnippets(string(data))
	if len(blocks) < 8 {
		t.Fatalf("docs/reference/data-oriented/index.md Leia examples = %d, want data-oriented walkthrough", len(blocks))
	}
	source := strings.Join(blocks, "\n\n") + `
assert(#roundtrip == 1)
assert(total[1].channel_id == 1)
assert(total[1].amount == 120.0)
assert(sum == 90.0)
assert(soa.len(window) == 3)
`
	file := filepath.Join(t.TempDir(), "data-oriented-reference.leia")
	if err := os.WriteFile(file, []byte(source), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/leia", "run", file)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("data-oriented reference Leia examples failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
}

func TestReferenceConcurrencyExamplesStayRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "reference", "concurrency", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := markdownLeiaSnippets(string(data))
	if len(blocks) < 8 {
		t.Fatalf("docs/reference/concurrency/index.md Leia examples = %d, want concurrency walkthrough", len(blocks))
	}
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "goroutine",
			source: blocks[0] + `
assert(sum == 500500)
`,
		},
		{
			name: "channel",
			source: blocks[1] + `
assert(value == "ready")
` + blocks[2] + `
assert(ok == false)
`,
		},
		{
			name: "select",
			source: `left := make(chan, 1)
right := make(chan, 1)
left <- 10
` + blocks[3],
		},
		{
			name: "timeout",
			source: `done := make(chan)
` + blocks[5],
		},
		{
			name:   "waitgroup",
			source: blocks[6],
		},
		{
			name:   "group",
			source: blocks[7],
		},
		{
			name:   "shared-state",
			source: blocks[8] + "\nassert(msg.count == 1)\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runLeiaDocSnippet(t, root, "concurrency-"+tc.name+".leia", tc.source)
		})
	}
}

func readmeFirstLeiaSnippet(readme string) string {
	for _, marker := range []string{"```go", "````leia", "```leia"} {
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

func markdownLeiaSnippets(markdown string) []string {
	var snippets []string
	lines := strings.Split(markdown, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		var fence string
		switch {
		case strings.HasPrefix(line, "````leia"):
			fence = "````"
		case strings.HasPrefix(line, "```leia"):
			fence = "```"
		default:
			continue
		}
		var block []string
		for i++; i < len(lines); i++ {
			if strings.HasPrefix(lines[i], fence) {
				break
			}
			block = append(block, lines[i])
		}
		snippets = append(snippets, strings.TrimSpace(strings.Join(block, "\n")))
	}
	return snippets
}

func runLeiaDocSnippet(t *testing.T, root, name, source string) {
	t.Helper()
	file := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(file, []byte(strings.TrimSpace(source)+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", "./cmd/leia", "run", file)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s failed: %v\nsource:\n%s\nstdout:\n%s\nstderr:\n%s", name, err, source, stdout.String(), stderr.String())
	}
}
