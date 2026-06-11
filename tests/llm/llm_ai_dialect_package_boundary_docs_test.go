package leia_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestAIDialectPackageBoundaryDocsReferenceExistingPackageDirs(t *testing.T) {
	root := repoRoot(t)
	docRel := filepath.Join("examples", "ai", "finrobot_translation", "ai_dialect_index", "PACKAGE_BOUNDARIES.md")
	data, err := os.ReadFile(filepath.Join(root, docRel))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(data)
	block := extractAIDialectBoundaryPackageList(t, doc)

	matches := regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(block, -1)
	if len(matches) == 0 {
		t.Fatalf("%s contains no documented package directories", docRel)
	}

	var got []string
	seen := map[string]bool{}
	for _, match := range matches {
		rel := filepath.ToSlash(match[1])
		got = append(got, rel)
		if seen[rel] {
			t.Fatalf("duplicate documented package directory %q", rel)
		}
		seen[rel] = true
		if !strings.HasPrefix(rel, "examples/ai/finrobot_translation/live_packages/generic_") {
			t.Fatalf("documented boundary path %q is not a generic AI live package", rel)
		}
		if filepath.IsAbs(rel) || strings.Contains(rel, "../") {
			t.Fatalf("documented boundary path %q is not a safe relative path", rel)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("documented package directory %q: %v", rel, err)
		}
		if !info.IsDir() {
			t.Fatalf("documented package boundary %q is not a directory", rel)
		}
	}

	want := documentedAIDialectGenericPackageDirs(t, root)
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("documented package directories:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	lowerDoc := strings.ToLower(doc)
	for _, required := range []string{
		"not a built-in language feature",
		"not a finrobot-only surface",
		"small amount of leia code",
		"package composition",
		"provider-free replay",
		"responsibility boundaries",
		"record/replay",
		"evaluation",
		"model",
		"turn",
		"tool",
		"agent",
		"workflow",
		"eval",
		"replay",
		"trace",
		"approval",
		"package-audit",
	} {
		if !strings.Contains(lowerDoc, required) {
			t.Fatalf("%s does not document required boundary term %q", docRel, required)
		}
	}
}

func documentedAIDialectGenericPackageDirs(t *testing.T, root string) []string {
	t.Helper()
	packagesRoot := filepath.Join(root, "examples", "ai", "finrobot_translation", "live_packages")
	entries, err := os.ReadDir(packagesRoot)
	if err != nil {
		t.Fatal(err)
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "generic_") {
			continue
		}
		dirs = append(dirs, filepath.ToSlash(filepath.Join("examples", "ai", "finrobot_translation", "live_packages", entry.Name())))
	}
	sort.Strings(dirs)
	return dirs
}

func TestFinRobotStatusDocsMarkGenericAIDialectAsCheckedInBoundary(t *testing.T) {
	root := repoRoot(t)
	docs := []string{
		filepath.Join("examples", "ai", "finrobot_translation", "COVERAGE.md"),
		filepath.Join("examples", "ai", "finrobot_translation", "VERIFICATION.md"),
		filepath.Join("examples", "ai", "finrobot_translation", "GAPS.md"),
	}

	for _, rel := range docs {
		data, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatal(err)
		}
		doc := strings.ToLower(string(data))
		for _, required := range []string{
			"generic ai",
			"checked-in package boundar",
			"live_packages/generic_*",
		} {
			if !strings.Contains(doc, required) {
				t.Fatalf("%s must document generic AI checked-in package boundary status; missing %q", rel, required)
			}
		}
		clarifiesGenericScope := false
		for _, required := range []string{
			"not finrobot-only",
			"not a finrobot-only",
			"not finrobot-specific",
			"not a finrobot-specific",
		} {
			if strings.Contains(doc, required) {
				clarifiesGenericScope = true
				break
			}
		}
		if !clarifiesGenericScope {
			t.Fatalf("%s must clarify the generic AI dialect boundary is not FinRobot-specific", rel)
		}

		for _, forbidden := range []string{
			"generic ai dialect convergence status is missing",
			"generic ai dialect convergence status is planned",
			"generic ai dialect package boundary is missing",
			"generic ai dialect package boundary is planned",
		} {
			if strings.Contains(doc, forbidden) {
				t.Fatalf("%s regressed generic AI dialect status with %q", rel, forbidden)
			}
		}
	}
}

func TestAIDialectPackageBoundaryReadmePointsToArchitecture(t *testing.T) {
	root := repoRoot(t)
	readmeRel := filepath.Join("examples", "ai", "finrobot_translation", "ai_dialect_index", "README.md")
	data, err := os.ReadFile(filepath.Join(root, readmeRel))
	if err != nil {
		t.Fatal(err)
	}
	lowerReadme := strings.ToLower(string(data))
	for _, required := range []string{
		"package_boundaries.md",
		"small leia assembly",
		"complex ai projects",
		"package composition",
		"provider-free replay",
		"approval",
		"trace",
		"evaluation",
		"record/replay",
		"docs tests parse the package list",
	} {
		if !strings.Contains(lowerReadme, required) {
			t.Fatalf("%s does not document required architecture pointer %q", readmeRel, required)
		}
	}
}

func extractAIDialectBoundaryPackageList(t *testing.T, doc string) string {
	t.Helper()
	start := "<!-- ai-dialect-boundary-package-list:start -->"
	end := "<!-- ai-dialect-boundary-package-list:end -->"
	startIndex := strings.Index(doc, start)
	if startIndex < 0 {
		t.Fatalf("boundary package list start marker missing")
	}
	endIndex := strings.Index(doc[startIndex:], end)
	if endIndex < 0 {
		t.Fatalf("boundary package list end marker missing")
	}
	return doc[startIndex : startIndex+endIndex]
}
