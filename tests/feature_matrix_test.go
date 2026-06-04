package tests_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestFeatureMatrixSchema(t *testing.T) {
	root := findRepoRoot(t)
	path := filepath.Join(root, "tests", "feature_matrix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}

	var matrix struct {
		SchemaVersion  int                          `json:"schema_version"`
		StatusValues   []string                     `json:"status_values"`
		RequiredFields []string                     `json:"required_fields"`
		FieldNotes     map[string]string            `json:"field_notes"`
		Features       []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}
	if matrix.SchemaVersion != 2 {
		t.Fatalf("schema_version = %d, want 2", matrix.SchemaVersion)
	}
	if len(matrix.Features) == 0 {
		t.Fatal("feature matrix must contain at least one feature")
	}
	specSections := loadLanguageSpecSections(t, root)

	statuses := make(map[string]bool, len(matrix.StatusValues))
	for _, status := range matrix.StatusValues {
		if status == "" {
			t.Fatal("status_values must not contain empty strings")
		}
		statuses[status] = true
	}
	if len(statuses) != len(matrix.StatusValues) {
		t.Fatal("status_values must not contain duplicates")
	}

	required := make(map[string]bool, len(matrix.RequiredFields))
	for _, field := range matrix.RequiredFields {
		if field == "" {
			t.Fatal("required_fields must not contain empty strings")
		}
		required[field] = true
		if matrix.FieldNotes[field] == "" {
			t.Fatalf("missing field_notes entry for %q", field)
		}
	}
	for _, field := range []string{"parser", "bytecode", "interpreter", "tier1", "tier2", "semantic_gate", "conformance_case", "perf_hot_case"} {
		if !required[field] {
			t.Fatalf("required_fields missing %q", field)
		}
	}

	seenIDs := map[string]bool{}
	referencedSpecSections := map[string]bool{}
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		if seenIDs[id] {
			t.Fatalf("features[%d] duplicate id %q", i, id)
		}
		seenIDs[id] = true
		_ = decodeRequiredString(t, feature, i, "name")
		_ = decodeRequiredString(t, feature, i, "category")
		sections := decodeRequiredStringList(t, feature, i, "spec_sections")
		for _, section := range sections {
			if !specSections[section] {
				t.Fatalf("features[%d] %s.spec_sections references missing language spec section %q", i, id, section)
			}
			referencedSpecSections[section] = true
		}

		for _, field := range matrix.RequiredFields {
			raw, ok := feature[field]
			if !ok {
				t.Fatalf("features[%d] %s missing required field %q", i, id, field)
			}
			var cell struct {
				Status string   `json:"status"`
				Refs   []string `json:"refs"`
			}
			if err := json.Unmarshal(raw, &cell); err != nil {
				t.Fatalf("features[%d] %s.%s: %v", i, id, field, err)
			}
			if !statuses[cell.Status] {
				t.Fatalf("features[%d] %s.%s has invalid status %q", i, id, field, cell.Status)
			}
			for _, ref := range cell.Refs {
				assertRepoRelativeFileRef(t, root, id, field, ref)
			}
		}
	}
	assertLanguageSpecSectionsCovered(t, specSections, referencedSpecSections)
}

func TestLanguageGrammarAppendixDocumentsStableSyntax(t *testing.T) {
	root := findRepoRoot(t)
	spec := readFileString(t, filepath.Join(root, "docs", "spec", "language.md"))
	grammar := readFileString(t, filepath.Join(root, "docs", "spec", "grammar.ebnf"))

	for _, snippet := range []string{
		"[`grammar.ebnf`](grammar.ebnf)",
		"Leia supports tagged dialect forms.",
		"The AI surface is a standard-library layer",
		"[`../reference/directives/index.md`](../reference/directives/index.md)",
		"all configuration values that the body needs\nmust be passed or closed over explicitly",
		"logical `&&`",
		"logical `||`",
		"Unary logical negation is\n`!`",
	} {
		if !strings.Contains(spec, snippet) {
			t.Fatalf("language spec must document grammar appendix, dialects, and stdlib AI surface; missing %q", snippet)
		}
	}

	requiredProductions := []string{
		`import_decl    = "import" ( import_spec | "(" { import_spec } ")" ) ;`,
		`const_decl     = "const" identifier ( "=" | ":=" ) expr ;`,
		`tagged_string  = [ identifier | "$" ] [ "!" ] raw_string_lit ;`,
		`tagged_block   = identifier [ "!" ] config_block ;`,
		`select_stmt    = "select" "{" { select_case } "}" ;`,
		`dense_lit      = "[" [ integer_lit ] "]" dense_type "{"`,
	}
	for _, production := range requiredProductions {
		if !strings.Contains(grammar, production) {
			t.Fatalf("grammar appendix missing stable production snippet %q", production)
		}
	}
}

func TestFeatureMatrixCoversTaggedDialectAndModpkgReleaseGuards(t *testing.T) {
	root := findRepoRoot(t)
	features := loadFeatureMatrixFeatureMap(t, root)

	tagged := features["tagged_dialect_syntax"]
	if tagged == nil {
		t.Fatal("feature_matrix.json missing tagged_dialect_syntax feature")
	}
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "parser", "internal/parser/dialect_syntax_test.go")
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "interpreter", "tests/dialect_syntax_test.go", "internal/stdlib/bind/dialect_web_test.go")
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "semantic_gate", "tests/architecture/stdlib_boundary_test.go", "cmd/leia/main_examples_test.go")
	requireFeatureStringList(t, tagged, "tagged_dialect_syntax", "builtin_dialect_tags",
		"sh", "cmd", "shellwords", "glob", "path",
		"re", "regexp", "json", "jsonl", "csv", "tsv", "mdtable", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "semver", "duration", "tap", "junit", "xml", "template",
		"url", "html_escape", "urlquery", "urlpath", "mime", "headers", "http_headers", "cookie", "cookies", "httpmsg",
		"ipaddr", "cidr", "hostport",
		"base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary",
		"prompt", "quote",
	)

	parserGuard := readFileString(t, filepath.Join(root, "internal", "parser", "dialect_syntax_test.go"))
	for _, snippet := range []string{
		"TestTaggedDialectStringsRequireRawStringLiteral",
		"$!`printf hello`",
		`{method: "eval_raw", tag: "quote", failFast: true}`,
	} {
		if !strings.Contains(parserGuard, snippet) {
			t.Fatalf("internal/parser/dialect_syntax_test.go must keep tagged syntax guard snippet %q", snippet)
		}
	}
	dialectGuard := readFileString(t, filepath.Join(root, "tests", "dialect_syntax_test.go"))
	if !strings.Contains(dialectGuard, "urlpath`a b/米`") || !strings.Contains(dialectGuard, `dialect.eval(\"urlpath\"`) {
		t.Fatal("tests/dialect_syntax_test.go must keep urlpath tagged literal and eval coverage")
	}
	stdlibBoundary := readFileString(t, filepath.Join(root, "tests", "architecture", "stdlib_boundary_test.go"))
	if !strings.Contains(stdlibBoundary, `"urlpath"`) {
		t.Fatal("stdlib architecture boundary must keep urlpath in the approved web dialect registry")
	}
	exampleGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_test.go"))
	for _, snippet := range []string{
		"TestRunCommandDialectExamplesCoverApprovedBuiltinTags",
		"approvedBuiltinDialectTags",
		"collectDialectExampleTags",
	} {
		if !strings.Contains(exampleGate, snippet) {
			t.Fatalf("cmd/leia/main_examples_test.go must keep builtin dialect example gate snippet %q", snippet)
		}
	}

	modpkg := features["module_package_management"]
	if modpkg == nil {
		t.Fatal("feature_matrix.json missing module_package_management feature")
	}
	requireFeatureCellRefs(t, modpkg, "module_package_management", "semantic_gate",
		"internal/modpkg/modpkg_test.go",
		"tests/architecture/package_boundary_test.go",
		"tests/architecture/stdlib_boundary_test.go",
	)
	modpkgGuard := readFileString(t, filepath.Join(root, "internal", "modpkg", "modpkg_test.go"))
	for _, snippet := range []string{
		"TestVerifyChecksTransitiveDependencyManifests",
		"TestDownloadFetchesTransitiveGitHubRequires",
		"TestVendorCopiesTransitiveDownloadedModules",
	} {
		if !strings.Contains(modpkgGuard, snippet) {
			t.Fatalf("internal/modpkg/modpkg_test.go must keep transitive module package guard %q", snippet)
		}
	}
	packageBoundary := readFileString(t, filepath.Join(root, "tests", "architecture", "package_boundary_test.go"))
	if !strings.Contains(packageBoundary, `internal("modpkg")`) {
		t.Fatal("package architecture boundary must keep modpkg dependency direction guard")
	}
}

func decodeRequiredString(t *testing.T, feature map[string]json.RawMessage, index int, field string) string {
	t.Helper()
	raw, ok := feature[field]
	if !ok {
		t.Fatalf("features[%d] missing %q", index, field)
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("features[%d].%s: %v", index, field, err)
	}
	if value == "" {
		t.Fatalf("features[%d].%s must not be empty", index, field)
	}
	return value
}

func decodeRequiredStringList(t *testing.T, feature map[string]json.RawMessage, index int, field string) []string {
	t.Helper()
	raw, ok := feature[field]
	if !ok {
		t.Fatalf("features[%d] missing %q", index, field)
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatalf("features[%d].%s: %v", index, field, err)
	}
	if len(values) == 0 {
		t.Fatalf("features[%d].%s must not be empty", index, field)
	}
	seen := map[string]bool{}
	for _, value := range values {
		if value == "" {
			t.Fatalf("features[%d].%s must not contain empty strings", index, field)
		}
		if seen[value] {
			t.Fatalf("features[%d].%s contains duplicate section %q", index, field, value)
		}
		seen[value] = true
	}
	return values
}

func loadFeatureMatrixFeatureMap(t *testing.T, root string) map[string]map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "tests", "feature_matrix.json"))
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}
	var matrix struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}
	features := map[string]map[string]json.RawMessage{}
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		if features[id] != nil {
			t.Fatalf("duplicate feature id %q", id)
		}
		features[id] = feature
	}
	return features
}

func requireFeatureCellRefs(t *testing.T, feature map[string]json.RawMessage, featureID, field string, refs ...string) {
	t.Helper()
	raw, ok := feature[field]
	if !ok {
		t.Fatalf("%s missing %s", featureID, field)
	}
	var cell struct {
		Status string   `json:"status"`
		Refs   []string `json:"refs"`
	}
	if err := json.Unmarshal(raw, &cell); err != nil {
		t.Fatalf("%s.%s: %v", featureID, field, err)
	}
	if cell.Status != "covered" {
		t.Fatalf("%s.%s status = %q, want covered", featureID, field, cell.Status)
	}
	have := map[string]bool{}
	for _, ref := range cell.Refs {
		have[ref] = true
	}
	for _, ref := range refs {
		if !have[ref] {
			t.Fatalf("%s.%s refs = %#v, missing %q", featureID, field, cell.Refs, ref)
		}
	}
}

func requireFeatureStringList(t *testing.T, feature map[string]json.RawMessage, featureID, field string, values ...string) {
	t.Helper()
	raw, ok := feature[field]
	if !ok {
		t.Fatalf("%s missing %s", featureID, field)
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s.%s: %v", featureID, field, err)
	}
	if strings.Join(got, ",") != strings.Join(values, ",") {
		t.Fatalf("%s.%s = %#v, want %#v", featureID, field, got, values)
	}
}

func loadLanguageSpecSections(t *testing.T, root string) map[string]bool {
	t.Helper()
	indexPath := filepath.Join(root, "docs", "spec", "index.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("read language spec index: %v", err)
	}
	sections := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "- [") {
			continue
		}
		titleEnd := strings.Index(line, "](")
		linkEnd := strings.Index(line, "):")
		if titleEnd < 3 || linkEnd <= titleEnd {
			continue
		}
		title := strings.TrimSpace(line[3:titleEnd])
		target := strings.TrimSpace(line[titleEnd+2 : linkEnd])
		if title == "" || target == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, "docs", "spec", target)); err != nil {
			t.Fatalf("language spec index references missing normative document %q: %v", target, err)
		}
		sections[title] = true
	}
	if len(sections) == 0 {
		t.Fatal("language spec index must contain normative document links")
	}
	sections["Stability Contract"] = true
	return sections
}

func assertLanguageSpecSectionsCovered(t *testing.T, specSections, referencedSpecSections map[string]bool) {
	t.Helper()
	ignoredSpecSections := map[string]string{
		"Notation":                   "spec notation and normative wording, not a language feature",
		"Source Code Representation": "source text model covered by lexical/directive gates",
	}
	for section := range ignoredSpecSections {
		if !specSections[section] {
			t.Fatalf("ignored language spec section %q does not exist", section)
		}
		if referencedSpecSections[section] {
			t.Fatalf("ignored language spec section %q must not also be referenced by a feature", section)
		}
	}

	var uncovered []string
	for section := range specSections {
		if referencedSpecSections[section] || ignoredSpecSections[section] != "" {
			continue
		}
		uncovered = append(uncovered, section)
	}
	if len(uncovered) > 0 {
		sort.Strings(uncovered)
		t.Fatalf("language spec sections must be referenced by at least one feature or explicitly ignored: %s", strings.Join(uncovered, ", "))
	}
}

func assertRepoRelativeFileRef(t *testing.T, root, featureID, field, ref string) {
	t.Helper()
	if ref == "" {
		t.Fatalf("%s.%s has empty ref", featureID, field)
	}
	if filepath.IsAbs(ref) || strings.Contains(ref, "\\") {
		t.Fatalf("%s.%s ref %q must be a repo-relative slash path", featureID, field, ref)
	}
	clean := filepath.Clean(ref)
	if clean == "." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || clean == ".." {
		t.Fatalf("%s.%s ref %q escapes repository root", featureID, field, ref)
	}
	if _, err := os.Stat(filepath.Join(root, clean)); err != nil {
		t.Fatalf("%s.%s ref %q does not resolve to a file: %v", featureID, field, ref, err)
	}
}
