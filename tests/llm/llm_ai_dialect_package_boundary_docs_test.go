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

	want := []string{
		"examples/ai/finrobot_translation/live_packages/generic_agent_runner",
		"examples/ai/finrobot_translation/live_packages/generic_approval_policy",
		"examples/ai/finrobot_translation/live_packages/generic_evaluation_harness",
		"examples/ai/finrobot_translation/live_packages/generic_model_registry",
		"examples/ai/finrobot_translation/live_packages/generic_package_boundary_auditor",
		"examples/ai/finrobot_translation/live_packages/generic_record_replay",
		"examples/ai/finrobot_translation/live_packages/generic_tool_contracts",
		"examples/ai/finrobot_translation/live_packages/generic_trace_events",
		"examples/ai/finrobot_translation/live_packages/generic_turn_runner",
		"examples/ai/finrobot_translation/live_packages/generic_workflow_orchestrator",
	}
	sort.Strings(got)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("documented package directories:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	lowerDoc := strings.ToLower(doc)
	for _, required := range []string{
		"not a built-in language feature",
		"not a finrobot-only surface",
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
