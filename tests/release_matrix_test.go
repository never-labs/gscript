package tests_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

type releaseFeatureMatrix struct {
	RequiredFields []string                     `json:"required_fields"`
	Features       []map[string]json.RawMessage `json:"features"`
}

type releaseCoverageCell struct {
	Status string   `json:"status"`
	Refs   []string `json:"refs"`
}

func TestReleaseMatrixSpecSectionsHaveSemanticGate(t *testing.T) {
	root := findRepoRoot(t)
	matrix := loadReleaseFeatureMatrix(t, root)
	specSections := loadLanguageSpecSections(t, root)

	ignored := releaseIgnoredSpecSections()
	coveredByGate := map[string][]string{}
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		sections := decodeRequiredStringList(t, feature, i, "spec_sections")
		for _, field := range []string{"semantic_gate", "conformance_case"} {
			cell := decodeReleaseCoverageCell(t, feature, i, id, field)
			if !releaseGateStatus(cell.Status) || len(cell.Refs) == 0 {
				continue
			}
			for _, section := range sections {
				coveredByGate[section] = append(coveredByGate[section], id+"."+field)
			}
		}
	}

	var missing []string
	for section := range specSections {
		if ignored[section] != "" || len(coveredByGate[section]) > 0 {
			continue
		}
		missing = append(missing, section)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("stable language spec sections need semantic_gate or conformance_case refs in feature_matrix.json: %s", strings.Join(missing, ", "))
	}
}

func TestReleaseMatrixConformanceCasesHaveStatusAndClassification(t *testing.T) {
	root := findRepoRoot(t)
	pairs := readConformanceCasePairs(t, root)
	manifestCases, manifestCount := readConformanceManifestCases(t, root)
	knownFailureCases := readBacktickCaseNames(t, filepath.Join(root, "tests", "language", "KNOWN_FAILURES.md"))

	if manifestCount != len(manifestCases) {
		t.Fatalf("MANIFEST current translated passing count = %d, but table contains %d cases", manifestCount, len(manifestCases))
	}
	if manifestCount != len(pairs) {
		t.Fatalf("MANIFEST current translated passing count = %d, but language conformance directory contains %d paired .lua/.leia cases", manifestCount, len(pairs))
	}

	var missing []string
	for name := range pairs {
		if manifestCases[name] || knownFailureCases[name] {
			continue
		}
		missing = append(missing, name)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("language conformance cases need a passing MANIFEST row or skipped KNOWN_FAILURES entry: %s", strings.Join(missing, ", "))
	}
}

func TestReleaseMatrixKnownGapDocsAreReleaseGateInputs(t *testing.T) {
	root := findRepoRoot(t)
	testMatrix := readFileString(t, filepath.Join(root, "docs", "testing.md"))
	for _, ref := range []string{
		"tests/language/MISSING_CAPABILITIES.md",
		"tests/language/KNOWN_FAILURES.md",
		"tests/language/MANIFEST.md",
		"docs/reference/stdlib/index.md",
	} {
		if !strings.Contains(testMatrix, ref) {
			t.Fatalf("docs/testing.md must name %s as a release-gate input", ref)
		}
	}
}

func TestReleaseMatrixCommunityEntrypointsAreLinked(t *testing.T) {
	root := findRepoRoot(t)
	for _, path := range []string{
		"README.md",
		"SECURITY.md",
		"CONTRIBUTING.md",
		"CODE_OF_CONDUCT.md",
		".github/ISSUE_TEMPLATE/bug_report.md",
		".github/ISSUE_TEMPLATE/performance_regression.md",
		".github/ISSUE_TEMPLATE/language_proposal.md",
		".github/pull_request_template.md",
		"examples/README.md",
		"docs/release/notes-template.md",
		"docs/contributing/performance.md",
		"docs/reference/platforms/index.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("release community entrypoint %s is missing: %v", path, err)
		}
	}

	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, ref := range []string{"SECURITY.md", "CONTRIBUTING.md", "CODE_OF_CONDUCT.md"} {
		if !strings.Contains(readme, ref) {
			t.Fatalf("README.md must link %s", ref)
		}
	}

	docsIndex := readFileString(t, filepath.Join(root, "docs", "index.md"))
	for _, ref := range []string{
		"../SECURITY.md",
		"../CONTRIBUTING.md",
		"../CODE_OF_CONDUCT.md",
		"../examples/README.md",
		"contributing/performance.md",
		"reference/platforms/index.md",
	} {
		if !strings.Contains(docsIndex, ref) {
			t.Fatalf("docs/index.md must link %s", ref)
		}
	}

	release := readFileString(t, filepath.Join(root, "docs", "release", "index.md"))
	for _, snippet := range []string{
		"choose a license and add a root `LICENSE` file",
		"docs/release/notes-template.md",
		"examples/README.md",
		"docs/reference/platforms/index.md",
	} {
		if !strings.Contains(release, snippet) {
			t.Fatalf("docs/release/index.md must mention %q", snippet)
		}
	}
}

func TestReleaseMatrixModuleReferenceDocumentsCommandSurface(t *testing.T) {
	root := findRepoRoot(t)
	modules := readFileString(t, filepath.Join(root, "docs", "reference", "modules", "index.md"))
	for _, command := range []string{
		"leia mod init",
		"leia mod add",
		"leia mod tidy",
		"leia mod check",
		"leia mod download",
		"leia mod vendor",
		"leia mod lock",
		"leia mod verify",
		"leia mod list",
		"leia mod graph",
		"leia mod explain",
		"leia mod capability",
		"leia mod gomod",
	} {
		if !strings.Contains(modules, "`"+command+"`") {
			t.Fatalf("docs/reference/modules/index.md must document %s", command)
		}
	}
}

func TestReleaseMatrixStdlibContractHasConformanceCoverageEntry(t *testing.T) {
	root := findRepoRoot(t)
	modules := readStdlibContractRows(t, root)
	searchText := strings.ToLower(strings.Join([]string{
		readFileString(t, filepath.Join(root, "tests", "feature_matrix.json")),
		readFileString(t, filepath.Join(root, "tests", "language", "MANIFEST.md")),
		readFileString(t, filepath.Join(root, "tests", "language", "MISSING_CAPABILITIES.md")),
	}, "\n"))

	var missing []string
	for module := range modules {
		if regexp.MustCompile(`(^|[^[:alnum:]_])` + regexp.QuoteMeta(strings.ToLower(module)) + `([^[:alnum:]_]|$)`).MatchString(searchText) {
			continue
		}
		missing = append(missing, module)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("stdlib contract modules need a conformance case or documented capability entry: %s", strings.Join(missing, ", "))
	}
}

func loadReleaseFeatureMatrix(t *testing.T, root string) releaseFeatureMatrix {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "tests", "feature_matrix.json"))
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}
	var matrix releaseFeatureMatrix
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}
	if len(matrix.Features) == 0 {
		t.Fatal("feature matrix must contain features")
	}
	return matrix
}

func decodeReleaseCoverageCell(t *testing.T, feature map[string]json.RawMessage, index int, id, field string) releaseCoverageCell {
	t.Helper()
	raw, ok := feature[field]
	if !ok {
		t.Fatalf("features[%d] %s missing %q", index, id, field)
	}
	var cell releaseCoverageCell
	if err := json.Unmarshal(raw, &cell); err != nil {
		t.Fatalf("features[%d] %s.%s: %v", index, id, field, err)
	}
	return cell
}

func releaseGateStatus(status string) bool {
	return status == "covered" || status == "partial"
}

func releaseIgnoredSpecSections() map[string]string {
	return map[string]string{
		"Phase 0 Hard Deliverables": "phase acceptance criteria, not a language feature",
		"Production Roadmap":        "planning metadata, not a language feature",
		"Change-Control Checklist":  "change process metadata, not a language feature",
	}
}

func readConformanceCasePairs(t *testing.T, root string) map[string]bool {
	t.Helper()
	cases := readManifestConformanceCases(t, root)
	pairs := map[string]bool{}
	for _, testCase := range cases {
		if pairs[testCase.Name] {
			t.Fatalf("tests manifest has duplicate language conformance case %q", testCase.Name)
		}
		pairs[testCase.Name] = true
	}
	return pairs
}

func readConformanceManifestCases(t *testing.T, root string) (map[string]bool, int) {
	t.Helper()
	data := readFileString(t, filepath.Join(root, "tests", "language", "MANIFEST.md"))
	countRE := regexp.MustCompile(`Current translated passing cases:\s*([0-9]+)\.`)
	countMatch := countRE.FindStringSubmatch(data)
	if countMatch == nil {
		t.Fatal("MANIFEST missing current translated passing case count")
	}
	count, err := strconv.Atoi(countMatch[1])
	if err != nil {
		t.Fatalf("parse MANIFEST passing case count: %v", err)
	}

	rowRE := regexp.MustCompile("^\\|\\s*`([^`]+)`\\s*\\|\\s*([^|]+?)\\s*\\|\\s*(.+?)\\s*\\|\\s*$")
	cases := map[string]bool{}
	for lineNo, line := range strings.Split(data, "\n") {
		match := rowRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name := match[1]
		sourceArea := strings.TrimSpace(match[2])
		notes := strings.TrimSpace(match[3])
		if sourceArea == "" || notes == "" {
			t.Fatalf("MANIFEST.md:%d case %q needs source-area classification and notes", lineNo+1, name)
		}
		if cases[name] {
			t.Fatalf("MANIFEST.md:%d duplicate case %q", lineNo+1, name)
		}
		cases[name] = true
	}
	if len(cases) == 0 {
		t.Fatal("MANIFEST contains no machine-checkable case rows")
	}
	return cases, count
}

func readBacktickCaseNames(t *testing.T, path string) map[string]bool {
	t.Helper()
	data := readFileString(t, path)
	caseRE := regexp.MustCompile("`([a-z0-9][a-z0-9_]*[a-z0-9])`")
	cases := map[string]bool{}
	for _, match := range caseRE.FindAllStringSubmatch(data, -1) {
		cases[match[1]] = true
	}
	return cases
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
