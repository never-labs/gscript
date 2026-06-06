package spec_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestDocsSpecGateEntrypointsStaySynchronized(t *testing.T) {
	root := findRepoRoot(t)
	docsCheck := readFileString(t, filepath.Join(root, "scripts", "docs_check.sh"))

	for _, snippet := range []string{
		"go test ./tests/docs/spec -count=1",
		"go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1",
		"README stable contract and docs/spec stability contract",
	} {
		if !strings.Contains(docsCheck, snippet) {
			t.Fatalf("scripts/docs_check.sh must keep docs/spec contract gate snippet %q", snippet)
		}
	}
}

func TestReadmeAndSpecStableContractStayAligned(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	specIndex := readFileString(t, filepath.Join(root, "docs", "spec", "index.md"))
	stability := markdownSection(t, specIndex, "Stability Contract")
	stableContractSource := `The stable contract is the language spec plus\nfeature matrix and release gates.`

	for _, snippet := range []string{
		strings.ReplaceAll(stableContractSource, `\n`, "\n"),
		"(docs/spec/index.md)",
		"Experimental behavior should be documented as\nsuch before users depend on it.",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README.md stable contract must keep snippet %q", snippet)
		}
	}

	for _, snippet := range []string{
		"a normative section in this specification",
		"`tests/feature_matrix.json`",
		"at least one semantic or conformance gate",
		"release notes or migration notes",
		"must not be advertised as stable",
	} {
		if !strings.Contains(stability, snippet) {
			t.Fatalf("docs/spec/index.md Stability Contract must keep snippet %q", snippet)
		}
	}
}

func TestSpecIndexNormativeDocumentsMatchFiles(t *testing.T) {
	root := findRepoRoot(t)
	specDir := filepath.Join(root, "docs", "spec")
	specIndex := readFileString(t, filepath.Join(specDir, "index.md"))
	normative := markdownSection(t, specIndex, "Normative Documents")

	targetRE := regexp.MustCompile(`^\- \[[^\]]+\]\(([^)]+)\):`)
	linked := map[string]bool{}
	for _, line := range strings.Split(normative, "\n") {
		match := targetRE.FindStringSubmatch(line)
		if len(match) != 2 {
			continue
		}
		target := match[1]
		linked[target] = true
		if _, err := os.Stat(filepath.Join(specDir, filepath.FromSlash(target))); err != nil {
			t.Fatalf("docs/spec/index.md links missing normative target %s: %v", target, err)
		}
	}
	if len(linked) == 0 {
		t.Fatal("docs/spec/index.md must list normative documents")
	}

	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Fatalf("read docs/spec: %v", err)
	}
	var unlinked []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || name == "index.md" || name == "language.md" || name == "index.html" {
			continue
		}
		if strings.HasSuffix(name, ".md") || name == "grammar.ebnf" {
			if !linked[name] {
				unlinked = append(unlinked, name)
			}
		}
	}
	if len(unlinked) > 0 {
		sort.Strings(unlinked)
		t.Fatalf("docs/spec files must be linked from Normative Documents: %s", strings.Join(unlinked, ", "))
	}
}

func TestRunnableSpecFencesKeepStableAllModeCoverage(t *testing.T) {
	root := findRepoRoot(t)
	specDir := filepath.Join(root, "docs", "spec")
	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Fatalf("read docs/spec: %v", err)
	}

	var runAll, failAll int
	var aiNativeRunAll int
	var bad []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(specDir, entry.Name())
		for lineNo, line := range strings.Split(readFileString(t, path), "\n") {
			if !strings.HasPrefix(line, "```leia") {
				continue
			}
			info := strings.TrimSpace(strings.TrimPrefix(line, "```"))
			switch info {
			case "leia run all":
				runAll++
				if entry.Name() == "ai-native.md" {
					aiNativeRunAll++
				}
			case "leia fail all":
				failAll++
			case "leia run", "leia fail":
				bad = append(bad, entry.Name()+":"+strconv.Itoa(lineNo+1)+": use all-mode stable fence tag: ```"+info+" all")
			default:
				bad = append(bad, entry.Name()+":"+strconv.Itoa(lineNo+1)+": unsupported spec Leia fence tag: ```"+info)
			}
		}
	}
	if len(bad) > 0 {
		t.Fatalf("docs/spec Leia fences must use stable all-mode tags:\n%s", strings.Join(bad, "\n"))
	}
	if runAll == 0 || failAll == 0 {
		t.Fatalf("docs/spec runnable fence coverage must include success and failure examples; got %d run all, %d fail all", runAll, failAll)
	}
	if aiNativeRunAll == 0 {
		t.Fatal("docs/spec/ai-native.md must include at least one stable runnable Leia example")
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("could not find repo root containing go.mod")
		}
		wd = parent
	}
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func markdownSection(t *testing.T, doc, title string) string {
	t.Helper()
	marker := "## " + title
	start := strings.Index(doc, marker)
	if start < 0 {
		t.Fatalf("markdown document must contain section %q", title)
	}
	rest := doc[start+len(marker):]
	next := regexp.MustCompile(`(?m)^## `).FindStringIndex(rest)
	if next != nil {
		rest = rest[:next[0]]
	}
	return rest
}
