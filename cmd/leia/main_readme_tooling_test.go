package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	stdinstall "github.com/never-labs/leia/internal/stdlib/install"
)

func TestReadmeIntroStaysFocused(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(data)
	if !strings.Contains(readme, "GitHub Linguist does not yet register Leia") || !strings.Contains(readme, "```go\nfunc greet(name)") {
		t.Fatal("README must keep the documented GitHub syntax-highlighting fallback")
	}
	for _, want := range []string{
		"Leia is a general-purpose scripting language designed to run standalone or inside Go applications.",
		"func greet(name)",
		"total += n",
		`print(greet("Leia"))`,
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

func TestReadmeMainLeiaExampleStaysRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	snippet := readmeFirstLeiaSnippet(string(data))
	if snippet == "" {
		t.Fatal("README must contain a Leia example")
	}
	for _, want := range []string{"func greet(name)", "numbers := [1, 2, 3, 4, 5]", "total += n", `print(greet("Leia"))`} {
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
	if strings.TrimSpace(stdout.String()) != "Hello, Leia!\nsum:\t15" {
		t.Fatalf("README Leia example stdout = %q, want greeting and sum", stdout.String())
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
	for _, want := range []string{"rows := [", "total := 0", "print(total)"} {
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
	if got := strings.TrimSpace(stdout.String()); got != "18" {
		t.Fatalf("docs home Leia example stdout = %q, want core data loop result", stdout.String())
	}
}

func TestDocsHomeLayoutPrimaryNavStaysProductFocused(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	layout := readFileString(t, filepath.Join(root, "docs", "_layouts", "home.html"))
	for _, want := range []string{
		`href="{{ '/spec/' | relative_url }}"`,
		`href="{{ '/playground.html' | relative_url }}"`,
		`href="{{ '/reference/cli/' | relative_url }}"`,
		`href="{{ '/guides/embedding.html' | relative_url }}"`,
		`href="{{ '/reference/dialects/' | relative_url }}"`,
		`href="{{ '/reference/performance/' | relative_url }}"`,
	} {
		if !strings.Contains(layout, want) {
			t.Fatalf("home layout primary nav missing %q", want)
		}
	}
	if strings.Contains(strings.ToLower(layout), "blog") {
		t.Fatal("home layout must not restore blog navigation")
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
assert(total == 120.0)
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

func TestReferenceScientificNumericExampleStaysRunnable(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "reference", "scientific", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	blocks := markdownLeiaSnippets(string(data))
	if len(blocks) != 1 {
		t.Fatalf("docs/reference/scientific/index.md Leia examples = %d, want one primitive-composition example", len(blocks))
	}
	for _, want := range []string{
		"F := mat([[1.0, 1.0], [0.0, 1.0]])",
		"H := row(1.0, 0.0)",
		"Q := eye(2, 0.01)",
		"stats.gaussian_state",
		"stats.linear_predict",
		"stats.linear_update",
		"dot([state.x.position, state.x.velocity]",
		"checksum := sum([state.x.position, state.x.velocity])",
		"assert(near(checksum, state.x.position + state.x.velocity, 0.000000001))",
	} {
		if !strings.Contains(blocks[0], want) {
			t.Fatalf("scientific numeric reference example missing %q:\n%s", want, blocks[0])
		}
	}
	runLeiaDocSnippet(t, root, "scientific-reference.leia", blocks[0])
}

func TestReferenceScientificDefaultImportsStaySyncedWithPrelude(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	data, err := os.ReadFile(filepath.Join(root, "docs", "reference", "scientific", "index.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := parseScientificDefaultImportTable(t, string(data))
	want := map[string]map[string]bool{}
	for _, alias := range stdinstall.DefaultAliases() {
		if want[alias.Module] == nil {
			want[alias.Module] = map[string]bool{}
		}
		want[alias.Module][alias.Name] = true
	}
	if !stringSetMapEqual(got, want) {
		t.Fatalf("scientific default import table drifted\ngot  %#v\nwant %#v", got, want)
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

func parseScientificDefaultImportTable(t *testing.T, markdown string) map[string]map[string]bool {
	t.Helper()
	imports := map[string]map[string]bool{}
	inTable := false
	for _, line := range strings.Split(markdown, "\n") {
		line = strings.TrimSpace(line)
		if line == "| Source module | Default helpers |" {
			inTable = true
			continue
		}
		if !inTable {
			continue
		}
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "|---") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			t.Fatalf("malformed default import row %q", line)
		}
		module := strings.Trim(strings.TrimSpace(parts[1]), "`")
		if module == "" {
			t.Fatalf("default import row missing module: %q", line)
		}
		if imports[module] == nil {
			imports[module] = map[string]bool{}
		}
		for _, raw := range strings.Split(parts[2], ",") {
			name := strings.Trim(strings.TrimSpace(raw), "`")
			if name == "" {
				continue
			}
			imports[module][name] = true
		}
	}
	if !inTable {
		t.Fatal("scientific reference missing default import table")
	}
	return imports
}

func stringSetMapEqual(got, want map[string]map[string]bool) bool {
	if len(got) != len(want) {
		return false
	}
	for module, wantNames := range want {
		gotNames, ok := got[module]
		if !ok || len(gotNames) != len(wantNames) {
			return false
		}
		for name := range wantNames {
			if !gotNames[name] {
				return false
			}
		}
	}
	return true
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
