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

func TestFeatureMatrixCoverageRefsUseExecutableEvidenceByField(t *testing.T) {
	root := findRepoRoot(t)
	path := filepath.Join(root, "tests", "feature_matrix.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}

	var matrix struct {
		Features []map[string]json.RawMessage `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}

	implementationFields := map[string]bool{
		"parser":      true,
		"bytecode":    true,
		"interpreter": true,
		"tier1":       true,
		"tier2":       true,
	}

	var problems []string
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		for field := range implementationFields {
			cell := decodeReleaseCoverageCell(t, feature, i, id, field)
			if cell.Status != "covered" {
				continue
			}
			for _, ref := range cell.Refs {
				if featureMatrixRefIsDocumentation(ref) || featureMatrixRefIsPlainSource(ref) {
					problems = append(problems, id+"."+field+"="+ref)
				}
			}
		}

		cell := decodeReleaseCoverageCell(t, feature, i, id, "perf_hot_case")
		if cell.Status != "covered" {
			continue
		}
		for _, ref := range cell.Refs {
			if !strings.HasPrefix(ref, "benchmarks/") || !strings.HasSuffix(ref, ".leia") {
				problems = append(problems, id+".perf_hot_case="+ref)
				continue
			}
			assertRepoRelativeFileRef(t, root, id, "perf_hot_case", ref)
		}
	}
	if len(problems) > 0 {
		sort.Strings(problems)
		t.Fatalf("feature_matrix covered implementation refs must be executable evidence, and perf_hot_case refs must be benchmark workloads: %s", strings.Join(problems, ", "))
	}
}

func TestFeatureMatrixCoveredRefsDoNotUseAllSkippedGoTestFiles(t *testing.T) {
	root := findRepoRoot(t)
	data, err := os.ReadFile(filepath.Join(root, "tests", "feature_matrix.json"))
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

	var offenders []string
	checked := map[string]bool{}
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
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
			if cell.Status != "covered" {
				continue
			}
			for _, ref := range cell.Refs {
				if checked[ref] || !strings.HasSuffix(ref, "_test.go") {
					continue
				}
				checked[ref] = true
				source := readFileString(t, filepath.Join(root, filepath.FromSlash(ref)))
				if goTestFileHasTests(source) && goTestFileAllTestsUnconditionallySkip(source) {
					offenders = append(offenders, ref)
				}
			}
		}
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("covered feature matrix refs must not point at all-skipped Go test files: %s", strings.Join(offenders, ", "))
	}
}

func TestLanguageGrammarAppendixDocumentsStableSyntax(t *testing.T) {
	root := findRepoRoot(t)
	spec := readFileString(t, filepath.Join(root, "docs", "spec", "language.md"))
	grammar := readFileString(t, filepath.Join(root, "docs", "spec", "grammar.ebnf"))

	for _, snippet := range []string{
		"[`grammar.ebnf`](grammar.ebnf)",
		"Leia supports tagged dialect forms.",
		"The AI surface is an optional standard-library layer",
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
		`tagged_string  = ( identifier | "$" ) [ "!" ] tagged_raw_string_lit ;`,
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
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "interpreter", "tests/dialect_syntax_test.go", "internal/stdlib/bind/dialect_text_test.go", "internal/stdlib/bind/dialect_protocol_test.go")
	requireFeatureCellRefs(t, tagged, "tagged_dialect_syntax", "semantic_gate",
		"tests/architecture/stdlib_boundary_test.go",
		"internal/stdlib/bind/dialect_text_test.go",
		"internal/stdlib/bind/dialect_data_test.go",
		"internal/stdlib/lib/csv/csv_test.go",
		"cmd/leia/main_examples_test.go",
		"cmd/leia/main_examples_command_test.go",
		"examples/dialects/shell_filesystem.leia",
		"examples/dialects/web_text.leia",
		"examples/tooling/release_gate_project/main.leia",
		"docs/reference/dialects/index.md",
	)
	requireFeatureStringList(t, tagged, "tagged_dialect_syntax", "builtin_dialect_tags",
		"sh", "cmd", "shellwords", "glob", "path",
		"re", "regexp", "json", "jsonptr", "jsonl", "csv", "tsv", "mdtable", "markdown", "md", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "yaml", "yml", "semver", "duration", "timestamp", "rfc3339", "tap", "junit", "xml", "template",
		"url", "html_escape", "html", "urlquery", "form", "urlform", "urlpath", "mime", "mailaddr", "emailaddr", "headers", "http_headers", "cookie", "cookies", "httpmsg", "sse", "multipart", "jwt",
		"ipaddr", "cidr", "hostport", "serve",
		"base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary", "q", "pem", "xlsx", "excel",
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
	qAnalytics := requireFeature(t, features, "q_analytics_dialect")
	requireFeatureCellRefs(t, qAnalytics, "q_analytics_dialect", "semantic_gate",
		"internal/stdlib/bind/q_test.go",
		"internal/stdlib/bind/db_test.go",
		"examples/data/q_vector_basics.leia",
		"examples/data/q_trade_analytics_project/main.leia",
		"examples/data/db_q_frame_project/main.leia",
	)
	spreadsheet := requireFeature(t, features, "spreadsheet_dialects")
	requireFeatureCellRefs(t, spreadsheet, "spreadsheet_dialects", "semantic_gate",
		"internal/stdlib/bind/dialect_data_test.go",
		"examples/data/db_q_frame_project/main.leia",
	)
	dbQFrameExample := readFileString(t, filepath.Join(root, "examples", "data", "db_q_frame_project", "main.leia"))
	for _, snippet := range []string{
		"db.memory()",
		"conn.frame(sql",
		"soa.len(frame.soa)",
		"q.query(frame.soa",
		`dialect.eval("xlsx"`,
		`dialect.eval("excel"`,
	} {
		if !strings.Contains(dbQFrameExample, snippet) {
			t.Fatalf("db_q_frame_project must keep db/SoA/q/spreadsheet project evidence snippet %q", snippet)
		}
	}
	stdlibBoundary := readFileString(t, filepath.Join(root, "tests", "architecture", "stdlib_boundary_test.go"))
	if !strings.Contains(stdlibBoundary, `"urlpath"`) {
		t.Fatal("stdlib architecture boundary must keep urlpath in the approved web dialect registry")
	}
	dialectDataGuard := readFileString(t, filepath.Join(root, "internal", "stdlib", "bind", "dialect_data_test.go"))
	for _, snippet := range []string{
		"TestDialectXLSXParsesFirstWorksheet",
		"TestDialectXLSXDecodePreservesSparseCellColumns",
		`StringValue("xlsx")`,
		`StringValue("excel")`,
	} {
		if !strings.Contains(dialectDataGuard, snippet) {
			t.Fatalf("internal/stdlib/bind/dialect_data_test.go must keep xlsx/excel dialect gate snippet %q", snippet)
		}
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
		"Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.",
		"Go-native:",
		"ARM64 JIT",
		"LuaJIT-class workloads",
		"q-style vector syntax",
		"high-throughput in-memory columnar computation",
		"q.sql(",
		"prompt`",
		"Optional LLM support lives in dialects and libraries, not in",
		"leia.New(leia.WithLibs(leia.LibSafe))",
		"## Example",
		"## Tooling",
		"## References",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README concise surface changed or missing expected snippet %q", snippet)
		}
	}
	embeddingRef := readFileString(t, filepath.Join(root, "docs", "reference", "embedding", "index.md"))
	for _, snippet := range []string{
		"`leia.New(opts ...Option) *VM`",
		"`RegisterFunc(name, fn)`",
		"`RegisterModule(name, Module)`",
		"`SecuritySandbox()`",
		"`WithMaxSteps(n)`",
		"`WithMaxNativeCalls(n)`",
		"`WithMaxGoroutines(n)`",
		"`WithMaxChannelCapacity(n)`",
		"`WithMaxHostResultBytes(n)`",
		"`WithMaxModuleBytes(n)`",
		"`type HotLoader`",
		"`*leia.BudgetError`",
	} {
		if !strings.Contains(embeddingRef, snippet) {
			t.Fatalf("embedding reference must cover README embedding contract; missing %q", snippet)
		}
	}

	features := loadFeatureMatrixFeatureMap(t, root)

	literals := requireFeature(t, features, "literals_and_constants")
	requireFeatureSpecSections(t, literals, "literals_and_constants", "Grammar Appendix", "Declarations And Scope", "Values And Types")
	requireFeatureCellRefs(t, literals, "literals_and_constants", "parser",
		"internal/parser/parser_test.go",
		"tests/language/literals_strings_basic.leia",
	)
	requireFeatureCellRefs(t, literals, "literals_and_constants", "semantic_gate",
		"tests/language/literals_long_brackets_more.leia",
		"tests/language/literals_control_escapes_more.leia",
	)
	tables := requireFeature(t, features, "tables_arrays_fields")
	requireFeatureSpecSections(t, tables, "tables_arrays_fields", "Grammar Appendix", "Values And Types", "Tables And Metatables")
	requireFeatureCellRefs(t, tables, "tables_arrays_fields", "semantic_gate",
		"tests/language/table_raw_helpers_more.leia",
		"tests/language/nextvar_table_remove_sequences.leia",
	)
	requireFeatureCellRefs(t, tables, "tables_arrays_fields", "conformance_case",
		"tests/language/sort_table_insert_remove_concat.leia",
	)
	multireturn := requireFeature(t, features, "varargs_multireturn")
	requireFeatureSpecSections(t, multireturn, "varargs_multireturn", "Grammar Appendix", "Functions", "Tables And Metatables")
	requireFeatureCellRefs(t, multireturn, "varargs_multireturn", "interpreter",
		"internal/runtime/multireturn_test.go",
	)
	requireFeatureCellRefs(t, multireturn, "varargs_multireturn", "semantic_gate",
		"tests/language/vararg_pack.leia",
		"tests/language/vararg_select.leia",
	)

	embeddingSecurity := requireFeature(t, features, "sandbox_capabilities_module_loading")
	requireFeatureCellRefs(t, embeddingSecurity, "sandbox_capabilities_module_loading", "semantic_gate",
		"tests/sdk/leia_test.go",
		"tests/sdk/security_api_test.go",
		"examples/embedding/embedding_test.go",
		"examples/embedding/hot_reload_project/hot_reload_project_test.go",
		"docs/testing.md",
		"docs/reference/security/index.md",
		"docs/reference/embedding/index.md",
	)

	hostBindings := requireFeature(t, features, "embedding_host_bindings")
	requireFeatureSpecSections(t, hostBindings, "embedding_host_bindings", "Modules And Loading", "Values And Types", "Functions", "Errors And Diagnostics")
	requireFeatureCellRefs(t, hostBindings, "embedding_host_bindings", "semantic_gate",
		"tests/sdk/register_api_test.go",
		"tests/sdk/module_api_test.go",
		"tests/sdk/error_api_test.go",
		"tests/sdk/public_api_boundary_test.go",
		"tests/sdk/embedding_r1_test.go",
		"examples/embedding/embedding_test.go",
		"examples/embedding/hot_reload_project/hot_reload_project_test.go",
		"docs/reference/embedding/index.md",
	)
	requireFeatureCellRefs(t, hostBindings, "embedding_host_bindings", "perf_hot_case",
		"benchmarks/app/stdlib_host.leia",
	)
	hostBindingGate := readFileString(t, filepath.Join(root, "tests", "sdk", "module_api_test.go"))
	for _, snippet := range []string{
		"TestWithGoImportsRequireAllowlist",
		"TestWithGoImportsImportSyntax",
		"TestWithGoImportsRejectsUnauthorizedGoImport",
		"TestRegisterModuleFromService",
	} {
		if !strings.Contains(hostBindingGate, snippet) {
			t.Fatalf("tests/sdk/module_api_test.go must keep host binding/import guard %q", snippet)
		}
	}

	resourceBudgets := requireFeature(t, features, "embedding_resource_budgets")
	requireFeatureCellRefs(t, resourceBudgets, "embedding_resource_budgets", "semantic_gate",
		"tests/sdk/resource_budget_test.go",
		"tests/sdk/resource_budget_native_test.go",
		"tests/sdk/resource_budget_host_result_test.go",
		"tests/sdk/resource_budget_module_test.go",
		"tests/sdk/resource_budget_concurrency_test.go",
		"tests/sdk/security_api_test.go",
		"tests/sdk/error_api_test.go",
		"tests/sdk/embedding_r1_test.go",
		"examples/embedding/embedding_test.go",
		"examples/embedding/hot_reload_project/hot_reload_project_test.go",
		"docs/reference/embedding/index.md",
	)

	hotReload := requireFeature(t, features, "embedding_hot_reload")
	requireFeatureSpecSections(t, hotReload, "embedding_hot_reload", "Stability Contract", "Modules And Loading", "Implementation Requirements")
	requireFeatureCellRefs(t, hotReload, "embedding_hot_reload", "interpreter",
		"tests/sdk/hotloader_test.go",
		"tests/sdk/hotloader_instance_test.go",
		"tests/sdk/hotloader_instance_rollback_test.go",
		"examples/embedding/hot_reload_project/hot_reload_project_test.go",
	)
	requireFeatureCellRefs(t, hotReload, "embedding_hot_reload", "semantic_gate",
		"tests/sdk/embedding_r1_test.go",
		"examples/embedding/embedding_test.go",
		"examples/embedding/hot_reload_project/hot_reload_project_test.go",
		"docs/reference/hot-reload/index.md",
		"docs/reference/embedding/index.md",
		"docs/guides/embedding.md",
	)
	hotLoaderGate := readFileString(t, filepath.Join(root, "tests", "sdk", "hotloader_test.go"))
	for _, snippet := range []string{
		"TestHotLoaderReloadSwapsProgram",
		"TestHotLoaderReloadFailureKeepsPreviousProgram",
		"TestHotLoaderReloadIfChangedSkipsSameSource",
		"TestHotLoaderInstanceUsesModuleOptionsForScript",
		"WithHotLoaderVMOptions",
		"ModuleOptionsForScript",
	} {
		if !strings.Contains(hotLoaderGate, snippet) {
			t.Fatalf("tests/sdk/hotloader_test.go must keep hot reload/module option guard %q", snippet)
		}
	}

	embeddingExamples := readFileString(t, filepath.Join(root, "examples", "embedding", "embedding_test.go"))
	for _, snippet := range []string{
		"func Example_hostFunctionBinding()",
		"func Example_hostModuleImport()",
		"func Example_hotLoader()",
		"func Example_hotInstance()",
		"func Example_sandboxAndMaxSteps()",
		"func Example_securitySandboxAndBudgets()",
		"func Example_productionEmbedding()",
		"leia.SecuritySandbox()",
		"leia.WithMaxSteps(32)",
		"leia.WithMaxHostResultBytes(8)",
		"safeLib := type(json)",
		"safe lib table",
	} {
		if !strings.Contains(embeddingExamples, snippet) {
			t.Fatalf("embedding examples must keep executable README embedding contract coverage; missing %q", snippet)
		}
	}
	releaseMatrix := readFileString(t, filepath.Join(root, "tests", "release_matrix_test.go"))
	for _, snippet := range []string{
		`"go", "test", "./examples/embedding", "-run", "Example", "-count=1"`,
		"TestReleaseMatrixEmbeddingDocsUsePublicSDKSurface",
	} {
		if !strings.Contains(releaseMatrix, snippet) {
			t.Fatalf("release matrix must keep runnable embedding example gate; missing %q", snippet)
		}
	}

	ai := requireFeature(t, features, "ai_dialect_integration")
	aiName := decodeRequiredString(t, ai, -1, "name")
	for _, snippet := range []string{"model", "turn", "tool", "msg", "chat", "loop", "agent", "stream", "replay", "provider"} {
		if !strings.Contains(aiName, snippet) {
			t.Fatalf("ai_dialect_integration name must expose README AI evidence scope; missing %q in %q", snippet, aiName)
		}
	}
	requireFeatureSpecSections(t, ai, "ai_dialect_integration", "Grammar Appendix", "AI Dialect Syntax", "Values And Types")
	requireFeatureCellRefs(t, ai, "ai_dialect_integration", "semantic_gate",
		"tests/llm/llm_runtime_test.go",
		"tests/llm/llm_agent_examples_test.go",
		"tests/llm/llm_agent_tools_test.go",
		"tests/llm/llm_record_replay_test.go",
		"tests/llm/llm_trace_test.go",
		"tests/llm/llm_ai_dialect_test.go",
		"tests/llm/llm_surface_audit_test.go",
		"tests/integration/llm/llm_openai_provider_test.go",
		"tests/integration/llm/llm_provider_test.go",
		"tests/integration/llm/llm_glm_integration_test.go",
		"cmd/leia/main_examples_test.go",
		"cmd/leia/main_examples_command_test.go",
		"cmd/leia/main_evaluate_llm_test.go",
		"cmd/leia/main_playground_test.go",
		"examples/ai/coding_agent_replay.leia",
		"examples/ai/coding_agent_project/main.leia",
		"examples/ai/tagged_agent_workflow.leia",
		"examples/ai/record_replay_trace_project.leia",
		"examples/llm/agent.leia",
		"examples/llm/agent_as_tool.leia",
		"examples/llm/direct_turn.leia",
		"examples/llm/incident_response.leia",
		"examples/llm/manual_tool_history.leia",
		"examples/llm/prompt_tagged_messages.leia",
		"examples/llm/rich_agent_demo.leia",
		"examples/llm/streaming_turn.leia",
		"examples/llm/glm_smoke.leia",
		"examples/llm/glm_direct_agent_tools.leia",
		"examples/evaluate/llm_replay.leia",
		"examples/evaluate/agent_replay.leia",
		"examples/evaluate/judge_replay.leia",
		"examples/evaluate/judge_replay.records.json",
		"examples/evaluate/multiturn_replay.leia",
		"examples/evaluate/project_agent_regression.leia",
		"examples/evaluate/project_agent_regression.records.json",
		"examples/workflow/support_triage_replay.leia",
		"examples/embedding/embedding_test.go",
		"docs/guides/ai-dialect.md",
		"docs/reference/ai/index.md",
		"docs/reference/evaluate/index.md",
	)
	aiRuntime := readFileString(t, filepath.Join(root, "tests", "llm", "llm_runtime_test.go"))
	for _, snippet := range []string{
		"TestLLMTurnWithMockProvider",
		"llm.turn({",
		"msg.assistant_call(call)",
		"chat.merge(history, more)",
	} {
		if !strings.Contains(aiRuntime, snippet) {
			t.Fatalf("tests/llm/llm_runtime_test.go must keep README AI tool/message/chat evidence snippet %q", snippet)
		}
	}
	aiDialect := readFileString(t, filepath.Join(root, "tests", "llm", "llm_ai_dialect_test.go"))
	for _, snippet := range []string{
		"TestAIDialectUsesLLMStdlibRuntime",
		"TestAIDialectTaggedLiteralRawBlockAgentToolTurnAndStreaming",
		"model {",
		"tool {",
		"agent {",
		"turn {",
		"on_stream: func(event)",
	} {
		if !strings.Contains(aiDialect, snippet) {
			t.Fatalf("tests/llm/llm_ai_dialect_test.go must keep README AI dialect syntax evidence snippet %q", snippet)
		}
	}
	aiExamples := readFileString(t, filepath.Join(root, "tests", "llm", "llm_agent_examples_test.go"))
	for _, snippet := range []string{
		"examples\", \"llm\", \"agent.leia\"",
		"examples\", \"llm\", \"agent_as_tool.leia\"",
		"examples\", \"llm\", \"streaming_turn.leia\"",
		"provider did not receive streaming request",
	} {
		if !strings.Contains(aiExamples, snippet) {
			t.Fatalf("tests/llm/llm_agent_examples_test.go must keep runnable LLM example evidence snippet %q", snippet)
		}
	}
	replayGate := readFileString(t, filepath.Join(root, "tests", "llm", "llm_record_replay_test.go"))
	for _, snippet := range []string{
		"TestLLMRecorderAndReplay",
		"TestLLMRecorderAndReplayStreamingEvents",
		"TestLLMReplaySynthesizesStreamEventForOldFixtures",
	} {
		if !strings.Contains(replayGate, snippet) {
			t.Fatalf("tests/llm/llm_record_replay_test.go must keep README replay/stream evidence snippet %q", snippet)
		}
	}
	providerGate := readFileString(t, filepath.Join(root, "tests", "integration", "llm", "llm_provider_test.go"))
	for _, snippet := range []string{
		"TestLLMCommandProvider",
		"TestAnthropicCompatibleLLMProvider",
		"TestAnthropicCompatibleLLMProviderStreamsContent",
		"TestAnthropicCompatibleLLMProviderStreamsToolUse",
	} {
		if !strings.Contains(providerGate, snippet) {
			t.Fatalf("tests/integration/llm/llm_provider_test.go must keep README provider-adapter evidence snippet %q", snippet)
		}
	}
	openAIGate := readFileString(t, filepath.Join(root, "tests", "integration", "llm", "llm_openai_provider_test.go"))
	for _, snippet := range []string{
		"TestOpenAICompatibleLLMProvider",
		"TestOpenAICompatibleLLMProviderStreamsContent",
		"TestOpenAIProviderThroughEmbeddingOption",
	} {
		if !strings.Contains(openAIGate, snippet) {
			t.Fatalf("tests/integration/llm/llm_openai_provider_test.go must keep README OpenAI provider evidence snippet %q", snippet)
		}
	}

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
		"tests/concurrency_contract_test.go",
		"tests/sdk/resource_budget_concurrency_test.go",
		"cmd/leia/main_examples_test.go",
		"cmd/leia/main_examples_command_test.go",
		"examples/concurrency/select_timeout.leia",
		"examples/concurrency/context_sleep.leia",
		"examples/concurrency/sync_group_cancel.leia",
		"examples/concurrency/pipeline_project/main.leia",
		"docs/reference/concurrency/index.md",
		"scripts/production_check.sh",
	)
	requireFeatureCellRefs(t, concurrency, "go_style_concurrency", "perf_hot_case",
		"benchmarks/concurrency/producer_consumer_pipeline.leia",
		"benchmarks/concurrency/select_timeout.leia",
		"benchmarks/concurrency/sync_group.leia",
	)
	requireConcurrencyPerfHotRefsAreManifested(t, root, concurrency)
	for path, snippets := range map[string][]string{
		"benchmarks/concurrency/producer_consumer_pipeline.leia": {
			"coroutine.create",
			"coroutine.yield(event)",
			"coroutine.resume(co)",
			"N := 650000",
		},
		"benchmarks/concurrency/select_timeout.leia": {
			"select {",
			"case <-time.after",
			"case <-never",
			"Time: %.6fs",
		},
		"benchmarks/concurrency/sync_group.leia": {
			"sync.group()",
			"group.start(func(ctx, id)",
			"group.wait()",
			"make(chan, workers)",
		},
	} {
		source := readFileString(t, filepath.Join(root, filepath.FromSlash(path)))
		for _, snippet := range snippets {
			if !strings.Contains(source, snippet) {
				t.Fatalf("%s must keep concurrency hot-path evidence snippet %q", path, snippet)
			}
		}
	}
	productionCheck := readFileString(t, filepath.Join(root, "scripts", "production_check.sh"))
	for _, snippet := range []string{
		"Concurrency Race Smoke",
		"go test -race ./internal/runtime ./internal/nanbox ./internal/vm ./llm ./tests/sdk ./tests/llm ./cmd/leia -count=1",
		"Go-style Concurrency Contract",
		"go test -race ./tests -run TestGoStyleConcurrencyContract -count=1",
	} {
		if !strings.Contains(productionCheck, snippet) {
			t.Fatalf("production_check.sh must keep concurrency race gate snippet %q", snippet)
		}
	}

	data := requireFeature(t, features, "matrix_dense_arrays")
	requireFeatureCellRefs(t, data, "matrix_dense_arrays", "interpreter",
		"internal/stdlib/bind/vec_test.go",
		"internal/stdlib/bind/matrix_test.go",
		"internal/stdlib/bind/soa_test.go",
		"internal/stdlib/lib/vec/vec_test.go",
		"internal/stdlib/lib/matrix/matrix_test.go",
		"internal/stdlib/lib/soa/ops_test.go",
	)
	requireFeatureCellRefs(t, data, "matrix_dense_arrays", "semantic_gate",
		"tests/language/matrix_host_dense_more.leia",
		"tests/language/vec_color_geometry_hsl_more.leia",
		"internal/stdlib/bind/vec_test.go",
		"internal/stdlib/bind/matrix_test.go",
		"internal/stdlib/bind/soa_test.go",
		"internal/stdlib/lib/vec/vec_test.go",
		"internal/stdlib/lib/matrix/matrix_test.go",
		"internal/stdlib/lib/soa/ops_test.go",
		"cmd/leia/main_examples_command_test.go",
		"examples/data_processing/data_oriented/dense_matrix_vec_kernels.leia",
		"examples/data_processing/data_oriented/soa_kernels.leia",
		"examples/data/db_q_frame_project/main.leia",
		"examples/tooling/release_gate_project/main.leia",
		"docs/reference/data-oriented/index.md",
	)
	requireFeatureCellRefs(t, data, "matrix_dense_arrays", "bytecode",
		"internal/vm/compiler_dense_test.go",
		"internal/vm/matrix_multiply_specialization_test.go",
		"internal/vm/spectral_runtime_specialization_test.go",
		"internal/vm/soa_affine_specialization_test.go",
	)
	requireFeatureCellRefs(t, data, "matrix_dense_arrays", "perf_hot_case",
		"benchmarks/numeric/matmul_dense.leia",
		"benchmarks/numeric/spectral_norm_dense.leia",
		"benchmarks/data/soa_affine_many.leia",
		"benchmarks/data/soa_masked_aggregate.leia",
		"benchmarks/data/soa_filter_gather.leia",
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
		"cmd/leia/main_ci_test.go",
		"cmd/leia/main_readme_tooling_test.go",
		"cmd/leia/main_metadata_test.go",
		"cmd/leia/main_evaluate_test.go",
		"internal/tooling/evaluate/evaluate_test.go",
		"cmd/leia/main_playground_test.go",
		"cmd/leia-lsp/main_test.go",
		"benchmarks/performance_gate_test.py",
		"benchmarks/benchmark_discovery_test.py",
		"examples/tooling/release_evidence_pipeline.leia",
		"examples/tooling/release_gate_project/main.leia",
		"docs/guides/tooling.md",
		"docs/reference/cli/index.md",
	)
	readmeToolingGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_readme_tooling_test.go"))
	for _, snippet := range []string{
		"TestReadmeToolingCommandsMapToCLI",
		"readmeToolingCommands",
		"data/q_operator_pipeline",
		"--runs",
		"want doc check via scripts/docs_check.sh",
	} {
		if !strings.Contains(readmeToolingGate, snippet) {
			t.Fatalf("README tooling guard must keep CLI command evidence snippet %q", snippet)
		}
	}
	examplesCommandGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_command_test.go"))
	for _, snippet := range []string{
		"TestExamplesCommandVerifiesPackageManagedProjects",
		"runModCommand([]string{\"verify\", \"--json\"",
		"package-managed project main is not discoverable by examples CLI",
	} {
		if !strings.Contains(examplesCommandGate, snippet) {
			t.Fatalf("examples command gate must keep package-managed project verification snippet %q", snippet)
		}
	}

	editorTooling := requireFeature(t, features, "editor_lsp_tooling")
	requireFeatureCellRefs(t, editorTooling, "editor_lsp_tooling", "semantic_gate",
		"cmd/leia-lsp/main_test.go",
		"internal/tooling/lsp/server_test.go",
		"tools/editor/smoke/editor_smoke.py",
		"tools/editor/smoke/editor_check_test.py",
		"tools/tree-sitter-leia/package.json",
		"tools/tree-sitter-leia/grammar.js",
		"tools/tree-sitter-leia/queries/highlights.scm",
		"tools/syntax/textmate/leia.tmLanguage.json",
		"tools/syntax/textmate/leia-mod.tmLanguage.json",
		"editors/vscode/package.json",
		"editors/vscode/extension.js",
		"editors/vscode/snippets/leia.json",
		"editors/emacs/leia-mode.el",
		"editors/helix/languages.toml",
		"editors/neovim/queries/leia/highlights.scm",
		"editors/zed/extension.toml",
		"scripts/editor_check.sh",
		"scripts/release_distribution_check.sh",
		"README.md",
	)
	lspServerGate := readFileString(t, filepath.Join(root, "internal", "tooling", "lsp", "server_test.go"))
	for _, snippet := range []string{
		"TestServerInitializeShutdown",
		"TestDocumentedCapabilitiesStaySynchronizedWithInitialize",
		"documentFormattingProvider",
		"semanticTokensProvider",
		"renameProvider",
		"publishDiagnostics",
	} {
		if !strings.Contains(lspServerGate, snippet) {
			t.Fatalf("LSP server gate must keep capability/diagnostic snippet %q", snippet)
		}
	}
	editorSmoke := readFileString(t, filepath.Join(root, "tools", "editor", "smoke", "editor_smoke.py"))
	for _, snippet := range []string{
		"check_vscode()",
		"check_emacs()",
		"check_tree_sitter_assets()",
		"check_packaged_editor_integrations()",
		"check_downstream_docs()",
		"leia.restartLanguageServer",
		"defun leia-eglot-setup",
		"editors/neovim/queries/leia/highlights.scm",
		"editors/helix/queries/leia/highlights.scm",
		"editors/zed/languages/leia/highlights.scm",
	} {
		if !strings.Contains(editorSmoke, snippet) {
			t.Fatalf("editor smoke guard must keep packaged editor evidence snippet %q", snippet)
		}
	}
	vscodePackage := readFileString(t, filepath.Join(root, "editors", "vscode", "package.json"))
	for _, snippet := range []string{
		"leia.runFile",
		"leia.testWorkspace",
		"leia.formatFile",
		"leia.lintWorkspace",
		"leia.checkWorkspace",
		"leia.previewSpec",
		"leia.restartLanguageServer",
		"leia.evaluate.case",
		"leia.languageServer.executable",
	} {
		if !strings.Contains(vscodePackage, snippet) {
			t.Fatalf("VS Code package must keep command/LSP contribution snippet %q", snippet)
		}
	}

	modules := requireFeature(t, features, "module_package_management")
	requireFeatureSpecSections(t, modules, "module_package_management", "Modules And Loading", "Values And Types")
	requireFeatureCellRefs(t, modules, "module_package_management", "semantic_gate",
		"internal/modpkg/modpkg_test.go",
		"modconfig_test.go",
		"tests/architecture/package_boundary_test.go",
		"cmd/leia/main_mod_test.go",
		"docs/reference/modules/index.md",
		"docs/guides/packages.md",
	)
	modconfigGate := readFileString(t, filepath.Join(root, "modconfig_test.go"))
	for _, snippet := range []string{
		"TestModuleOptionsForScriptLoadsLocalReplace",
		"TestModuleOptionsForScriptLoadsDownloadedGitHubCache",
		"TestModuleOptionsForScriptLoadsTransitiveDownloadedGitHubCache",
		"TestModuleOptionsForScriptModeVendorIgnoresCache",
		"TestModuleOptionsForScriptModeVendorLoadsTransitiveVendor",
	} {
		if !strings.Contains(modconfigGate, snippet) {
			t.Fatalf("modconfig_test.go must keep module runtime resolver guard %q", snippet)
		}
	}
	modCommandGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_mod_test.go"))
	for _, snippet := range []string{
		"TestModGraphTidyAndVerifyUseGoStyleImports",
		"TestModGraphTidyAndVerifyUseStaticRequireCalls",
		"TestModPackageWorkflowCoversDocsRuntimeModes",
		"TestModVendorCopiesTransitiveDownloadedModules",
		"TestModDownloadFetchesGitHubArchive",
		"TestModLockWritesSumAndVerifyDetectsLocalMutation",
	} {
		if !strings.Contains(modCommandGate, snippet) {
			t.Fatalf("cmd/leia/main_mod_test.go must keep module command contract guard %q", snippet)
		}
	}
	downloadVendor := requireFeature(t, features, "module_download_vendor_cache")
	requireFeatureCellRefs(t, downloadVendor, "module_download_vendor_cache", "semantic_gate",
		"internal/modpkg/modpkg_test.go",
		"modconfig_test.go",
		"cmd/leia/main_mod_test.go",
		"tests/release_matrix_test.go",
		"docs/reference/modules/index.md",
		"docs/guides/packages.md",
	)
	modulesRef := readFileString(t, filepath.Join(root, "docs", "reference", "modules", "index.md"))
	for _, snippet := range []string{
		"capability summaries, and optional Go-native binding metadata",
		"Local metadata commands do not need network access",
		"`leia mod verify`",
		"`leia mod list` | Print resolved module metadata.",
	} {
		if !strings.Contains(modulesRef, snippet) {
			t.Fatalf("modules reference must keep README package metadata contract; missing %q", snippet)
		}
	}

	bytecodeVM := requireFeature(t, features, "bytecode_vm_execution")
	requireFeatureSpecSections(t, bytecodeVM, "bytecode_vm_execution", "Implementation Requirements", "Stability Contract")
	requireFeatureCellRefs(t, bytecodeVM, "bytecode_vm_execution", "bytecode",
		"internal/vm/compiler_method_test.go",
		"internal/vm/opcode_test.go",
		"internal/vm/vm_test.go",
	)
	requireFeatureCellRefs(t, bytecodeVM, "bytecode_vm_execution", "semantic_gate",
		"cmd/leia/main_metadata_test.go",
		"cmd/leia/main_run_test.go",
		"examples/performance/execution_modes_matrix.leia",
		"docs/reference/platforms/index.md",
	)

	jit := requireFeature(t, features, "arm64_jit_runtime_fallback")
	requireFeatureSpecSections(t, jit, "arm64_jit_runtime_fallback", "Implementation Requirements", "Stability Contract")
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
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "semantic_gate",
		"internal/methodjit/semantic_gate_test.go",
		"internal/methodjit/diagnose_test.go",
		"internal/methodjit/exit_resume_check_test.go",
		"scripts/performance_gate.sh",
		"benchmarks/performance_gate_test.py",
		"benchmarks/perf_submit_guard_test.py",
		"benchmarks/manifest.json",
		"docs/reference/performance/index.md",
		"docs/reference/platforms/index.md",
	)
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "perf_hot_case",
		"benchmarks/numeric/matmul_dense.leia",
		"benchmarks/table/table_field_access.leia",
		"benchmarks/app/mixed_inventory_sim.leia",
	)

	releaseEvidence := requireFeature(t, features, "release_evidence_gates")
	requireFeatureSpecSections(t, releaseEvidence, "release_evidence_gates", "Implementation Requirements", "Stability Contract")
	requireFeatureCellRefs(t, releaseEvidence, "release_evidence_gates", "semantic_gate",
		"tests/release_matrix_test.go",
		"scripts/docs_check.sh",
		"scripts/production_check.sh",
		"scripts/public_release_blockers_check.sh",
		"scripts/release_artifacts_check.sh",
		"scripts/release_distribution_check.sh",
		"cmd/leia/main_ci_test.go",
		"docs/release/index.md",
		"docs/release/decisions.md",
	)

	distribution := requireFeature(t, features, "release_distribution_surface")
	requireFeatureCellRefs(t, distribution, "release_distribution_surface", "semantic_gate",
		"README.md",
		".goreleaser.yaml",
		"scripts/install.sh",
		"scripts/public_release_blockers_check.sh",
		"scripts/release_distribution_check.sh",
		"scripts/release_artifacts_check.sh",
		"cmd/leia/main_readme_tooling_test.go",
		"tests/release_matrix_test.go",
		"docs/release/decisions.md",
	)
}

func TestReadmeExecutionPerformanceContractHasReleaseGates(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	performance := readFileString(t, filepath.Join(root, "docs", "reference", "performance", "index.md"))
	platforms := readFileString(t, filepath.Join(root, "docs", "reference", "platforms", "index.md"))
	release := readFileString(t, filepath.Join(root, "docs", "release", "index.md"))
	gate := readFileString(t, filepath.Join(root, "scripts", "performance_gate.sh"))
	productionCheck := readFileString(t, filepath.Join(root, "scripts", "production_check.sh"))
	benchmarkGateTest := readFileString(t, filepath.Join(root, "benchmarks", "performance_gate_test.py"))

	for _, snippet := range []string{
		"Performance claims are benchmark-bound.",
		"JIT paths must preserve VM/runtime semantics.",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README execution/performance contract missing %q", snippet)
		}
	}
	for _, snippet := range []string{
		"supported hot paths may run natively, but unsupported",
		"operations must fall back to the VM/runtime without changing visible results",
		"The LuaJIT comparison is a validation baseline",
		"`--luajit-threshold` (default `0.80`)",
		"write JSON and Markdown reports from the full and strict benchmark commands",
	} {
		if !strings.Contains(performance, snippet) {
			t.Fatalf("performance reference must stay synced with README execution contract; missing %q", snippet)
		}
	}
	for _, snippet := range []string{
		"The interpreter defines script behavior. The bytecode VM and JIT are",
		"accelerators: they must preserve interpreter-visible results, errors,",
		"Method JIT | Native ARM64 hot-path acceleration.",
	} {
		if !strings.Contains(platforms, snippet) {
			t.Fatalf("platform reference must stay synced with README execution modes; missing %q", snippet)
		}
	}
	for _, snippet := range []string{
		"bash scripts/performance_gate.sh --full",
		"state tested platforms and execution modes",
		"document any experimental language, stdlib, AI, package, or JIT behavior",
	} {
		if !strings.Contains(release, snippet) {
			t.Fatalf("release docs must keep execution/performance release gate; missing %q", snippet)
		}
	}
	for _, snippet := range []string{
		"LUAJIT_THRESHOLD=0.80",
		"validate_luajit_artifact",
		"benchmarks/perf_submit_guard.py",
		"validate_strict_artifact",
	} {
		if !strings.Contains(gate, snippet) {
			t.Fatalf("performance gate must enforce JIT/LuaJIT bottom line; missing %q", snippet)
		}
	}
	for _, snippet := range []string{
		`local cmd="bash scripts/performance_gate.sh --smoke --runs 2 --warmup 1"`,
		`local cmd="bash scripts/performance_gate.sh --full"`,
		"add_performance_smoke",
		"add_performance_gate",
	} {
		if !strings.Contains(productionCheck, snippet) {
			t.Fatalf("production_check.sh must keep README execution/performance gate snippet %q", snippet)
		}
	}
	if strings.Contains(productionCheck, "--no-luajit") {
		t.Fatal("production_check.sh must not weaken README execution/performance gates with --no-luajit")
	}
	for _, snippet := range []string{
		"test_jit_fallback_luajit_contract_keeps_gate_refs",
		"test_validate_only_rejects_luajit_ratio_above_threshold",
		"test_builtin_gate_selectors_are_registered_manifest_workloads",
		"benchmarks/perf_submit_guard_test.py",
	} {
		if !strings.Contains(benchmarkGateTest, snippet) {
			t.Fatalf("benchmark tests must keep README execution/performance gate snippet %q", snippet)
		}
	}

	jit := requireFeature(t, loadFeatureMatrixFeatureMap(t, root), "arm64_jit_runtime_fallback")
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "semantic_gate",
		"scripts/performance_gate.sh",
		"benchmarks/performance_gate_test.py",
		"benchmarks/perf_submit_guard_test.py",
		"benchmarks/manifest.json",
		"docs/reference/performance/index.md",
		"docs/reference/platforms/index.md",
	)
	requireFeatureCellRefs(t, jit, "arm64_jit_runtime_fallback", "perf_hot_case",
		"benchmarks/numeric/matmul_dense.leia",
		"benchmarks/table/table_field_access.leia",
		"benchmarks/app/mixed_inventory_sim.leia",
	)
}

func TestReadmeAIDialectContractHasExplicitGates(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, snippet := range []string{
		"Optional LLM support lives in dialects and libraries, not in",
		"[Optional LLM dialect](docs/reference/ai/index.md)",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README AI dialect contract missing %q", snippet)
		}
	}

	ai := requireFeature(t, loadFeatureMatrixFeatureMap(t, root), "ai_dialect_integration")
	requireFeatureCellRefs(t, ai, "ai_dialect_integration", "interpreter",
		"tests/llm/llm_runtime_test.go",
		"tests/llm/llm_loop_test.go",
		"tests/llm/llm_agent_tools_test.go",
		"tests/llm/llm_record_replay_test.go",
		"tests/llm/llm_ai_dialect_test.go",
		"tests/llm/llm_surface_audit_test.go",
		"tests/integration/llm/llm_openai_provider_test.go",
		"tests/integration/llm/llm_provider_test.go",
	)
	requireFeatureCellRefs(t, ai, "ai_dialect_integration", "semantic_gate",
		"cmd/leia/main_examples_command_test.go",
		"cmd/leia/main_evaluate_llm_test.go",
		"cmd/leia/main_playground_test.go",
		"tests/llm/llm_loop_test.go",
		"examples/llm/direct_turn.leia",
		"examples/llm/agent_as_tool.leia",
		"examples/llm/prompt_tagged_messages.leia",
		"examples/ai/coding_agent_replay.leia",
		"examples/ai/coding_agent_project/main.leia",
		"examples/evaluate/llm_replay.leia",
		"examples/evaluate/agent_replay.leia",
		"examples/evaluate/judge_replay.leia",
		"examples/evaluate/judge_replay.records.json",
		"examples/evaluate/multiturn_replay.leia",
		"examples/evaluate/project_agent_regression.leia",
		"examples/evaluate/project_agent_regression.records.json",
		"examples/tooling/release_gate_project/main.leia",
		"docs/guides/ai-dialect.md",
		"docs/reference/ai/index.md",
		"docs/reference/evaluate/index.md",
	)

	requiredEvidence := map[string]map[string][]string{
		"models": {
			"tests/llm/llm_ai_dialect_test.go":                  {"model {", "model: \"mock-fast\""},
			"tests/integration/llm/llm_provider_test.go":        {"TestLLMModelsProviderConfigPreservesHostProvider", "protocol: \"openai_compatible\""},
			"tests/integration/llm/llm_openai_provider_test.go": {"TestLLMModelsProviderConfigOpenAICompatible", "provider_model: \"provider-fast\""},
			"cmd/leia/main_playground_test.go":                  {"TestPlaygroundAIExamplesCoverRunnableWorkflowShapes", "ai-model-alias"},
			"docs/reference/ai/index.md":                        {"## Model Dialect", "ProviderFactory"},
		},
		"tools": {
			"tests/llm/llm_runtime_test.go":     {"TestLLMTurnWithMockProvider", "llm.dispatch(result.calls[1], tools)"},
			"tests/llm/llm_agent_tools_test.go": {"TestLLMAgentScenarioAgentAsToolStructuredHandoff", "llm.agent_as_tool"},
			"examples/llm/agent_as_tool.leia":   {"tools: {extract_research}", "llm.turn({"},
			"docs/reference/ai/index.md":        {"## Tool Dialect", "`llm.agent_as_tool`"},
		},
		"messages": {
			"tests/llm/llm_runtime_test.go":            {"TestLLMMessageHelpers", "msg.assistant_call(call)"},
			"examples/llm/prompt_tagged_messages.leia": {"prompt_tagged_history_len", "history.find(messages"},
			"docs/reference/ai/index.md":               {"## Messages And History", "`msg.tool_error`"},
		},
		"turns": {
			"tests/llm/llm_runtime_test.go": {"TestLLMTurnWithMockProvider", "llm.turn({"},
			"examples/llm/direct_turn.leia": {"llm.turn({", "direct_text"},
			"docs/reference/ai/index.md":    {"## Turn Dialect", "performs exactly one provider request"},
		},
		"stream": {
			"tests/llm/llm_trace_test.go":         {"TestLLMTurnStreamsToScriptCallback", "on_stream: func(event)"},
			"tests/llm/llm_ai_dialect_test.go":    {"TestAIDialectTaggedLiteralRawBlockAgentToolTurnAndStreaming", "streamReq.Stream"},
			"tests/llm/llm_record_replay_test.go": {"TestLLMRecorderAndReplayStreamingEvents", "StreamEvents"},
			"cmd/leia/main_evaluate_llm_test.go":  {"TestEvaluateLLMRecordModeWritesGlobalAndCaseFixtures", "eval.usage().stream_events"},
			"examples/llm/streaming_turn.leia":    {"stream: true", "on_stream: func(event)"},
			"docs/reference/ai/index.md":          {"`stream`", "`on_stream` / `onStream`"},
			"docs/reference/evaluate/index.md":    {"`stream_events`", "`eval.usage()`"},
		},
		"agents": {
			"tests/llm/llm_agent_examples_test.go":       {"TestLLMAgentExampleSmoke", "agent.leia"},
			"tests/llm/llm_agent_tools_test.go":          {"TestLLMAgentScenarioDirectAgentInToolsList", "llm.run_agent"},
			"examples/ai/coding_agent_replay.leia":       {"coding-agent tools", "tools: {read_file, search_text, apply_patch, run_shell}"},
			"examples/ai/coding_agent_project/main.leia": {"llm.run_agent({", "read_file := tool {", "run_shell := tool {", "test_runs == 2"},
			"docs/reference/ai/index.md":                 {"## Agent Dialect", "## Agent As Tool"},
		},
		"replay": {
			"tests/llm/llm_record_replay_test.go":                     {"TestLLMRecorderAndReplay", "TestLLMReplayRejectsMismatchedRequest"},
			"cmd/leia/main_evaluate_llm_test.go":                      {"TestEvaluateLLMReplayAliasAndFixtureModeGuards", "--llm-replay"},
			"cmd/leia/main_examples_test.go":                          {"TestEvaluateReplayExamplesExecute", "project_agent_regression.records.json"},
			"examples/evaluate/judge_replay.leia":                     {"eval.judge({", "eval.budget({turns: 1, tokens: 8, cost: 0.01})"},
			"examples/evaluate/judge_replay.records.json":             {"\"MaxTokens\": 200", "\"Cost\": 0.004"},
			"examples/evaluate/multiturn_replay.leia":                 {"evaluate \"multiturn replay consumes every turn\"", "llm.turn({"},
			"examples/evaluate/project_agent_regression.leia":         {"evaluate \"project agent regression consumes replay\"", "eval.usage().stream_events"},
			"examples/evaluate/project_agent_regression.records.json": {"\"StreamEvents\"", "Checkout sev2 owned by ops"},
			"docs/reference/evaluate/index.md":                        {"--replay records/agent.records.json examples/evaluate/agent_replay.leia", "replay-drift findings"},
		},
		"provider adapters": {
			"tests/integration/llm/llm_provider_test.go":        {"TestAnthropicCompatibleLLMProvider", "command.Provider"},
			"tests/integration/llm/llm_openai_provider_test.go": {"TestOpenAICompatibleLLMProvider", "openai.Provider"},
			"tests/integration/llm/llm_glm_integration_test.go": {"TestGLMExamplesRunAgainstLocalAnthropicCompatibleProvider", "glm_direct_agent_tools.leia"},
			"cmd/leia/main_playground_test.go":                  {"TestPlaygroundExecAIProfileUsesGLMEnv", "LEIA_GLM_MODEL"},
			"docs/reference/ai/index.md":                        {"Run a live GLM example explicitly", "LEIA_ANTHROPIC_COMPAT_BASE_URL"},
		},
	}
	for label, files := range requiredEvidence {
		for rel, snippets := range files {
			data := readFileString(t, filepath.Join(root, filepath.FromSlash(rel)))
			for _, snippet := range snippets {
				if !strings.Contains(data, snippet) {
					t.Fatalf("README AI dialect %s gate %s missing snippet %q", label, rel, snippet)
				}
			}
		}
	}
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

func requireFeatureSpecSections(t *testing.T, feature map[string]json.RawMessage, featureID string, sections ...string) {
	t.Helper()
	raw, ok := feature["spec_sections"]
	if !ok {
		t.Fatalf("%s missing spec_sections", featureID)
	}
	var got []string
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s.spec_sections: %v", featureID, err)
	}
	have := map[string]bool{}
	for _, section := range got {
		have[section] = true
	}
	for _, section := range sections {
		if !have[section] {
			t.Fatalf("%s.spec_sections = %#v, missing %q", featureID, got, section)
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

func requireConcurrencyPerfHotRefsAreManifested(t *testing.T, root string, feature map[string]json.RawMessage) {
	t.Helper()

	var manifest struct {
		Cases []struct {
			ID     string `json:"id"`
			Path   string `json:"path"`
			Domain string `json:"domain"`
			Status string `json:"status"`
		} `json:"cases"`
		Workloads []struct {
			ID             string `json:"id"`
			Domain         string `json:"domain"`
			Script         string `json:"script"`
			TimeSourceHint string `json:"time_source_hint"`
		} `json:"workloads"`
	}
	data, err := os.ReadFile(filepath.Join(root, "benchmarks", "manifest.json"))
	if err != nil {
		t.Fatalf("read benchmark manifest: %v", err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("decode benchmark manifest: %v", err)
	}

	cases := map[string]string{}
	for _, c := range manifest.Cases {
		if c.Status == "active" && c.Domain == "concurrency" {
			cases[c.ID] = c.Path
		}
	}
	workloads := map[string]struct {
		script         string
		timeSourceHint string
	}{}
	for _, w := range manifest.Workloads {
		if w.Domain == "concurrency" {
			workloads[w.ID] = struct {
				script         string
				timeSourceHint string
			}{script: w.Script, timeSourceHint: w.TimeSourceHint}
		}
	}

	requiredFamilies := map[string]string{
		"channel_pipeline": "concurrency/producer_consumer_pipeline",
		"select_timeout":   "concurrency/select_timeout",
		"sync_group":       "concurrency/sync_group",
	}
	refs := decodeFeatureCellRefs(t, feature, "go_style_concurrency", "perf_hot_case")
	have := map[string]bool{}
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "benchmarks/concurrency/") || !strings.HasSuffix(ref, ".leia") {
			t.Fatalf("go_style_concurrency.perf_hot_case ref %q must be a concurrency benchmark", ref)
		}
		id := strings.TrimSuffix(strings.TrimPrefix(ref, "benchmarks/"), ".leia")
		have[id] = true
		if cases[id] != ref {
			t.Fatalf("go_style_concurrency.perf_hot_case ref %q is not an active manifest case", ref)
		}
		workload, ok := workloads[id]
		if !ok {
			t.Fatalf("go_style_concurrency.perf_hot_case ref %q is not a manifest workload", ref)
		}
		if workload.script != ref {
			t.Fatalf("manifest workload %s script = %q, want %q", id, workload.script, ref)
		}
		if workload.timeSourceHint != "script_time_line" {
			t.Fatalf("manifest workload %s time_source_hint = %q, want script_time_line", id, workload.timeSourceHint)
		}
	}
	for family, id := range requiredFamilies {
		if !have[id] {
			t.Fatalf("go_style_concurrency.perf_hot_case missing %s benchmark %s", family, id)
		}
	}
}

func decodeFeatureCellRefs(t *testing.T, feature map[string]json.RawMessage, featureID, field string) []string {
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
	return cell.Refs
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

func featureMatrixRefIsDocumentation(ref string) bool {
	return ref == "README.md" || strings.HasPrefix(ref, "docs/") || strings.HasSuffix(ref, ".md")
}

func featureMatrixRefIsPlainSource(ref string) bool {
	return strings.HasSuffix(ref, ".go") && !strings.HasSuffix(ref, "_test.go")
}

func goTestFileHasTests(source string) bool {
	return regexp.MustCompile(`(?m)^func Test[A-Za-z0-9_]*\s*\(`).FindStringIndex(source) != nil
}

func goTestFileAllTestsUnconditionallySkip(source string) bool {
	testDecls := regexp.MustCompile(`(?m)^func Test[A-Za-z0-9_]*\s*\(`).FindAllStringIndex(source, -1)
	if len(testDecls) == 0 {
		return false
	}
	for i, loc := range testDecls {
		start := loc[0]
		end := len(source)
		if i+1 < len(testDecls) {
			end = testDecls[i+1][0]
		}
		chunk := source[start:end]
		brace := strings.Index(chunk, "{")
		if brace < 0 {
			return false
		}
		body := stripLeadingGoCommentsAndWhitespace(chunk[brace+1:])
		if !strings.HasPrefix(body, "t.Skip(") && !strings.HasPrefix(body, "t.Skipf(") {
			return false
		}
	}
	return true
}

func stripLeadingGoCommentsAndWhitespace(source string) string {
	for {
		source = strings.TrimLeft(source, " \t\r\n")
		if !strings.HasPrefix(source, "//") {
			return source
		}
		newline := strings.IndexByte(source, '\n')
		if newline < 0 {
			return ""
		}
		source = source[newline+1:]
	}
}
