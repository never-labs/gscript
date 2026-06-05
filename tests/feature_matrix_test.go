package tests_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

func TestFeatureMatrixHasNoIncompleteCells(t *testing.T) {
	root := findRepoRoot(t)
	path := filepath.Join(root, "tests", "feature_matrix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}

	var matrix struct {
		RequiredFields []string                     `json:"required_fields"`
		Features       []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}

	var incomplete []string
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		for _, field := range matrix.RequiredFields {
			raw, ok := feature[field]
			if !ok {
				t.Fatalf("features[%d] %s missing required field %q", i, id, field)
			}
			var cell struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(raw, &cell); err != nil {
				t.Fatalf("features[%d] %s.%s: %v", i, id, field, err)
			}
			switch cell.Status {
			case "partial", "missing":
				incomplete = append(incomplete, id+"."+field+"="+cell.Status)
			}
		}
	}
	if len(incomplete) > 0 {
		sort.Strings(incomplete)
		t.Fatalf("feature_matrix.json must not ship incomplete README/stable-contract cells: %s", strings.Join(incomplete, ", "))
	}
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
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "interpreter", "tests/dialect_syntax_test.go", "internal/stdlib/bind/dialect_protocol_test.go")
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "semantic_gate", "tests/architecture/stdlib_boundary_test.go", "cmd/leia/main_examples_test.go")
	requireFeatureStringList(t, tagged, "tagged_dialect_syntax", "builtin_dialect_tags",
		"sh", "cmd", "shellwords", "glob", "path",
		"re", "regexp", "json", "jsonptr", "jsonl", "csv", "tsv", "mdtable", "markdown", "md", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "yaml", "yml", "semver", "duration", "timestamp", "rfc3339", "tap", "junit", "xml", "template",
		"url", "html_escape", "html", "urlquery", "form", "urlform", "urlpath", "mime", "mailaddr", "emailaddr", "headers", "http_headers", "cookie", "cookies", "httpmsg", "sse", "multipart", "jwt",
		"ipaddr", "cidr", "hostport",
		"base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary", "q", "pem",
		"sql",
		"prompt", "quote", "model", "turn", "tool", "agent",
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
	if !strings.Contains(dialectGuard, "TestQSymbolicDialectMilestone1ExecutesThroughStdlib") || !strings.Contains(dialectGuard, "q`+/1 2 3`") {
		t.Fatal("tests/dialect_syntax_test.go must keep q symbolic vector dialect coverage")
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

func TestFeatureMatrixCoversReadmeStableContract(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, snippet := range []string{
		"Go embedding API with sandbox, resource budgets, host bindings, and hot reload.",
		"AI-native syntax and stdlib support for models, tools, messages, turns,",
		"Go-style concurrency primitives: `go`, channels, `select`, sync helpers",
		"Data-oriented helpers for dense arrays, matrices, vectors, and SoA layouts.",
		"CLI tooling for format, lint, test, docs, diagnostics, modules, benchmarks,",
		"The JIT accelerates supported hot paths and falls back to the VM/runtime",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README stable contract changed or missing expected snippet %q", snippet)
		}
	}

	features := loadFeatureMatrixFeatureMap(t, root)

	embeddingSecurity := requireFeature(t, features, "sandbox_capabilities_module_loading")
	requireFeatureCellRefs(t, embeddingSecurity, "sandbox_capabilities_module_loading", "semantic_gate",
		"tests/sdk/leia_test.go",
		"tests/sdk/security_api_test.go",
		"docs/testing.md",
	)

	hostBindings := requireFeature(t, features, "embedding_host_bindings")
	requireFeatureCellRefs(t, hostBindings, "embedding_host_bindings", "semantic_gate",
		"tests/sdk/register_api_test.go",
		"tests/sdk/module_api_test.go",
		"tests/sdk/error_api_test.go",
		"tests/sdk/public_api_boundary_test.go",
		"examples/embedding/embedding_test.go",
	)
	requireFeatureCellRefs(t, hostBindings, "embedding_host_bindings", "perf_hot_case",
		"tests/sdk/embedding_perf_test.go",
	)

	resourceBudgets := requireFeature(t, features, "embedding_resource_budgets")
	requireFeatureCellRefs(t, resourceBudgets, "embedding_resource_budgets", "semantic_gate",
		"tests/sdk/resource_budget_test.go",
		"tests/sdk/resource_budget_native_test.go",
		"tests/sdk/resource_budget_host_result_test.go",
		"tests/sdk/resource_budget_module_test.go",
		"tests/sdk/resource_budget_concurrency_test.go",
		"tests/sdk/security_api_test.go",
		"tests/sdk/error_api_test.go",
	)

	hotReload := requireFeature(t, features, "embedding_hot_reload")
	requireFeatureCellRefs(t, hotReload, "embedding_hot_reload", "interpreter",
		"tests/sdk/hotloader_test.go",
		"tests/sdk/hotloader_instance_test.go",
		"tests/sdk/hotloader_instance_rollback_test.go",
	)

	ai := requireFeature(t, features, "llm_native_integration")
	requireFeatureCellRefs(t, ai, "llm_native_integration", "semantic_gate",
		"tests/llm/llm_runtime_test.go",
		"tests/llm/llm_agent_examples_test.go",
		"tests/llm/llm_record_replay_test.go",
		"tests/llm/llm_trace_test.go",
		"tests/llm/llm_ai_dialect_test.go",
		"tests/integration/llm/llm_openai_provider_test.go",
		"tests/integration/llm/llm_provider_test.go",
		"tests/integration/llm/llm_glm_integration_test.go",
		"cmd/leia/main_examples_test.go",
		"cmd/leia/main_examples_command_test.go",
		"examples/ai/coding_agent_replay.leia",
		"examples/evaluate/agent_replay.leia",
		"examples/workflow/support_triage_replay.leia",
		"examples/embedding/embedding_test.go",
	)

	stringsPatterns := requireFeature(t, features, "strings_patterns_concat")
	requireFeatureCellRefs(t, stringsPatterns, "strings_patterns_concat", "tier2",
		"internal/methodjit/string_patterns_feature_matrix_test.go",
		"internal/methodjit/string_tier2_test.go",
	)

	errorsDefer := requireFeature(t, features, "errors_pcall_xpcall_defer")
	requireFeatureCellRefs(t, errorsDefer, "errors_pcall_xpcall_defer", "tier1",
		"internal/methodjit/errors_defer_feature_matrix_test.go",
		"internal/methodjit/exit_resume_check_test.go",
	)
	requireFeatureCellRefs(t, errorsDefer, "errors_pcall_xpcall_defer", "tier2",
		"internal/methodjit/errors_defer_feature_matrix_test.go",
		"internal/methodjit/tier2_entry_deopt_test.go",
	)

	bitwise := requireFeature(t, features, "bitwise_bit32")
	requireFeatureCellRefs(t, bitwise, "bitwise_bit32", "tier1",
		"internal/methodjit/bitwise_feature_matrix_test.go",
		"internal/methodjit/emit_ops_test.go",
	)

	concurrency := requireFeature(t, features, "go_style_concurrency")
	requireFeatureCellRefs(t, concurrency, "go_style_concurrency", "semantic_gate",
		"tests/language/go_channel_host_more.leia",
		"tests/language/go_channel_edges_more.leia",
		"tests/sdk/resource_budget_concurrency_test.go",
		"cmd/leia/main_examples_test.go",
	)

	data := requireFeature(t, features, "matrix_dense_arrays")
	requireFeatureCellRefs(t, data, "matrix_dense_arrays", "semantic_gate",
		"tests/language/matrix_host_dense_more.leia",
		"tests/language/vec_color_geometry_hsl_more.leia",
		"examples/data_processing/data_oriented/dense_matrix_vec_kernels.leia",
	)
	requireFeatureCellRefs(t, data, "matrix_dense_arrays", "bytecode",
		"internal/vm/compiler_dense_test.go",
		"internal/vm/matrix_multiply_specialization_test.go",
		"internal/vm/spectral_runtime_specialization_test.go",
		"internal/vm/soa_affine_specialization_test.go",
	)

	tooling := requireFeature(t, features, "cli_repository_tooling")
	requireFeatureCellRefs(t, tooling, "cli_repository_tooling", "semantic_gate",
		"cmd/leia/main_fmt_test.go",
		"cmd/leia/main_lint_test.go",
		"cmd/leia/main_test_run_test.go",
		"cmd/leia/main_doc_test.go",
		"cmd/leia/main_diag_test.go",
		"cmd/leia/main_mod_test.go",
		"cmd/leia/main_bench_test.go",
		"cmd/leia/main_examples_command_test.go",
		"cmd/leia/main_readme_tooling_test.go",
		"docs/guides/tooling.md",
	)

	modules := requireFeature(t, features, "module_package_management")
	requireFeatureCellRefs(t, modules, "module_package_management", "semantic_gate",
		"internal/modpkg/modpkg_test.go",
		"tests/architecture/package_boundary_test.go",
		"cmd/leia/main_mod_test.go",
		"docs/reference/modules/index.md",
		"docs/guides/packages.md",
	)

	bytecodeVM := requireFeature(t, features, "bytecode_vm_execution")
	requireFeatureCellRefs(t, bytecodeVM, "bytecode_vm_execution", "bytecode",
		"internal/vm/compiler_method_test.go",
		"internal/vm/opcode_test.go",
		"internal/vm/vm_test.go",
	)
	requireFeatureCellRefs(t, bytecodeVM, "bytecode_vm_execution", "semantic_gate",
		"cmd/leia/main_metadata_test.go",
		"cmd/leia/main_run_test.go",
		"docs/reference/platforms/index.md",
	)

	jit := requireFeature(t, features, "arm64_jit_runtime_fallback")
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "tier1",
		"tests/jit_test.go",
		"tests/jit_loop_control_test.go",
		"tests/jit_side_exit_test.go",
		"internal/methodjit/semantic_gate_test.go",
	)
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "tier2",
		"internal/methodjit/emit_tier2_correctness_test.go",
		"internal/methodjit/exit_resume_check_test.go",
		"internal/methodjit/op_spec_oracle_test.go",
	)
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "perf_hot_case",
		"scripts/performance_gate.sh",
		"benchmarks/performance_gate_test.py",
		"benchmarks/perf_submit_guard_test.py",
	)

	releaseEvidence := requireFeature(t, features, "release_evidence_gates")
	requireFeatureCellRefs(t, releaseEvidence, "release_evidence_gates", "semantic_gate",
		"tests/release_matrix_test.go",
		"scripts/docs_check.sh",
		"scripts/production_check.sh",
		"scripts/release_artifacts_check.sh",
		"scripts/release_distribution_check.sh",
		"cmd/leia/main_ci_test.go",
		"docs/release/index.md",
	)
}

func TestActiveDocsUseLeiaNamingAndNoLegacyAskAgentDesign(t *testing.T) {
	root := findRepoRoot(t)
	legacyProjectName := regexp.MustCompile(`(?i)\b(gscript|gs)\b`)
	var offenders []string
	for _, base := range []string{"README.md", "docs"} {
		start := filepath.Join(root, filepath.FromSlash(base))
		err := filepath.WalkDir(start, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if strings.HasPrefix(rel, "docs/archive") {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".md" && filepath.Ext(path) != ".html" {
				return nil
			}
			data := readFileString(t, path)
			if legacyProjectName.MatchString(data) {
				offenders = append(offenders, rel+": legacy project name")
			}
			for _, oldAI := range []string{"agent ask(", "llm.ask("} {
				if strings.Contains(data, oldAI) {
					offenders = append(offenders, rel+": legacy AI ask surface "+oldAI)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", base, err)
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("active docs must use Leia names and avoid legacy ask-style AI design:\n%s", strings.Join(offenders, "\n"))
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

func requireFeature(t *testing.T, features map[string]map[string]json.RawMessage, id string) map[string]json.RawMessage {
	t.Helper()
	feature := features[id]
	if feature == nil {
		t.Fatalf("feature_matrix.json missing %s feature", id)
	}
	return feature
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
