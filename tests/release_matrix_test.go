package tests_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
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

func TestReleaseMatrixLanguageSpecMatchesGrammarAppendix(t *testing.T) {
	root := findRepoRoot(t)
	spec := normalizeGrammarText(readFileString(t, filepath.Join(root, "docs", "spec", "language.md")))
	grammar := normalizeGrammarText(readFileString(t, filepath.Join(root, "docs", "spec", "grammar.ebnf")))

	for _, production := range []string{
		`const_decl = "const" identifier ( "=" | ":=" ) expr ;`,
	} {
		if !strings.Contains(spec, production) {
			t.Fatalf("docs/spec/language.md overview must include grammar production %q", production)
		}
		if !strings.Contains(grammar, production) {
			t.Fatalf("docs/spec/grammar.ebnf must include grammar production %q", production)
		}
	}
}

func TestReleaseMatrixGrammarAppendixMatchesParserSurface(t *testing.T) {
	root := findRepoRoot(t)
	grammar := normalizeGrammarText(readFileString(t, filepath.Join(root, "docs", "spec", "grammar.ebnf")))

	for _, production := range []string{
		`method_suffix = ":" identifier "(" [ expr_list ] ")" ;`,
		`call_expr = postfix ( call_suffix | method_suffix ) ;`,
		`additive = multiplicative { ( "+" | "-" | "|" | "^" ) multiplicative } ;`,
		`multiplicative = unary { ( "*" | "/" | "%" | "<<" | ">>" | "&" | "&^" ) unary } ;`,
		`dense_type = "i32" | "i64" | "f32" | "f64" | "bool" ;`,
		`escaped_char = "\\" ( "\\" | '"' | "a" | "b" | "f" | "n" | "r" | "t" | "v"`,
		`| decimal_escape ) ;`,
		`decimal_escape = digit [ digit [ digit ] ] ;`,
	} {
		if !strings.Contains(grammar, production) {
			t.Fatalf("docs/spec/grammar.ebnf must include parser-aligned production %q", production)
		}
	}
	for _, unsupported := range []string{
		`"%="`,
		`"u8"`,
		`"u16"`,
		`"u32"`,
		`"u64"`,
	} {
		if strings.Contains(grammar, unsupported) {
			t.Fatalf("docs/spec/grammar.ebnf advertises unsupported parser surface %s", unsupported)
		}
	}
}

func TestReleaseMatrixStableKeywordsMatchLexer(t *testing.T) {
	root := findRepoRoot(t)
	spec := readFileString(t, filepath.Join(root, "docs", "spec", "language.md"))
	lexerSource := readFileString(t, filepath.Join(root, "internal", "lexer", "token.go"))

	specKeywords := parseSpecKeywordList(t, spec)
	lexerKeywords := parseLexerKeywordMap(t, lexerSource)
	if !sameStringSet(specKeywords, lexerKeywords) {
		t.Fatalf("stable keyword list must match lexer keywords\nspec:  %s\nlexer: %s", strings.Join(specKeywords, ", "), strings.Join(lexerKeywords, ", "))
	}
}

func TestReleaseMatrixSpecIndexDocumentsChapteredReference(t *testing.T) {
	root := findRepoRoot(t)
	index := readFileString(t, filepath.Join(root, "docs", "spec", "index.md"))
	overview := readFileString(t, filepath.Join(root, "docs", "spec", "language.md"))
	docsHome := readFileString(t, filepath.Join(root, "docs", "index.md"))
	readme := readFileString(t, filepath.Join(root, "README.md"))

	if !strings.Contains(overview, "[index.md](index.md)") || !strings.Contains(overview, "compatibility overview") {
		t.Fatal("docs/spec/language.md must point old links to the chaptered spec entrypoint")
	}
	if !strings.Contains(docsHome, "(spec/index.md)") {
		t.Fatal("docs/index.md must link the chaptered language spec entrypoint")
	}
	if !strings.Contains(readme, "(docs/spec/index.md)") {
		t.Fatal("README.md must link the chaptered language spec entrypoint")
	}

	for _, chapter := range []string{
		"notation.md",
		"source.md",
		"lexical.md",
		"declarations.md",
		"values.md",
		"expressions.md",
		"statements.md",
		"functions.md",
		"tables.md",
		"concurrency.md",
		"ai-native.md",
		"modules.md",
		"errors.md",
		"implementation.md",
		"grammar.ebnf",
	} {
		if !strings.Contains(index, "("+chapter+")") {
			t.Fatalf("docs/spec/index.md must link %s", chapter)
		}
		if _, err := os.Stat(filepath.Join(root, "docs", "spec", chapter)); err != nil {
			t.Fatalf("spec chapter %s is missing: %v", chapter, err)
		}
	}
}

func normalizeGrammarText(s string) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		lines = append(lines, strings.Join(fields, " "))
	}
	return strings.Join(lines, "\n")
}

func parseSpecKeywordList(t *testing.T, spec string) []string {
	t.Helper()
	re := regexp.MustCompile("(?s)Stable keywords:\\s*```text\\s*(.*?)\\s*```")
	match := re.FindStringSubmatch(spec)
	if len(match) != 2 {
		t.Fatal("docs/spec/language.md overview must contain a Stable keywords text block")
	}
	return sortedUniqueStrings(strings.Fields(match[1]))
}

func parseLexerKeywordMap(t *testing.T, lexerSource string) []string {
	t.Helper()
	re := regexp.MustCompile("(?s)var keywords = map\\[string\\]TokenType\\{(.*?)\\n\\}")
	match := re.FindStringSubmatch(lexerSource)
	if len(match) != 2 {
		t.Fatal("internal/lexer/token.go must contain lexer keyword map")
	}
	keyRE := regexp.MustCompile(`"([^"]+)"\s*:`)
	var keywords []string
	for _, key := range keyRE.FindAllStringSubmatch(match[1], -1) {
		keywords = append(keywords, key[1])
	}
	return sortedUniqueStrings(keywords)
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sortedUniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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

func TestReleaseMatrixSpecGateCommandsStaySynchronized(t *testing.T) {
	root := findRepoRoot(t)
	releaseMatrixCmd := "go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1"
	specExamplesCmd := "go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1"
	docsCheckCmd := "bash scripts/docs_check.sh"
	productionQuickCmd := "bash scripts/production_check.sh --quick"

	for _, item := range []struct {
		path     string
		snippets []string
	}{
		{
			path: "docs/testing.md",
			snippets: []string{
				releaseMatrixCmd,
				specExamplesCmd,
				docsCheckCmd,
				"tests/feature_matrix.json",
				"docs/spec/index.md",
			},
		},
		{
			path: "docs/release/index.md",
			snippets: []string{
				"## Machine-Checkable Release Evidence",
				productionQuickCmd,
				releaseMatrixCmd,
				docsCheckCmd,
				"tests/feature_matrix.json",
				"docs/spec/index.md",
			},
		},
		{
			path: "docs/release/notes-template.md",
			snippets: []string{
				productionQuickCmd,
				releaseMatrixCmd,
				docsCheckCmd,
			},
		},
		{
			path: "scripts/docs_check.sh",
			snippets: []string{
				releaseMatrixCmd,
				specExamplesCmd,
				"docs/spec/index.md",
				"tests/feature_matrix.json",
			},
		},
	} {
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.path)))
		for _, snippet := range item.snippets {
			if !strings.Contains(text, snippet) {
				t.Fatalf("%s must keep release/spec gate snippet %q", item.path, snippet)
			}
		}
	}

	productionOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--quick", "--list")
	for _, snippet := range []string{
		releaseMatrixCmd,
		specExamplesCmd,
		docsCheckCmd,
	} {
		if !strings.Contains(productionOut, snippet) {
			t.Fatalf("production_check.sh --quick --list must keep release/spec gate %q; got:\n%s", snippet, productionOut)
		}
	}
}

func TestReleaseMatrixDocCommandSurfaceIsUsable(t *testing.T) {
	root := findRepoRoot(t)
	for _, args := range [][]string{
		{"run", "./cmd/leia", "doc", "help"},
		{"run", "./cmd/leia", "doc", "generate", "--help"},
		{"run", "./cmd/leia", "doc", "check", "--help"},
		{"run", "./cmd/leia", "doc", "generate", "--format", "json"},
		{"run", "./cmd/leia", "doc", "check"},
	} {
		runCommand(t, root, 30*time.Second, "go", args...)
	}
}

func TestReleaseMatrixTopLevelCommandHelpIsSuccessful(t *testing.T) {
	root := findRepoRoot(t)
	for _, command := range []string{
		"eval",
		"run",
		"repl",
		"fmt",
		"lint",
		"test",
		"version",
		"env",
		"config",
		"check",
		"capabilities",
		"ci",
		"diag",
		"diagnose",
		"inspect",
		"bench",
		"doc",
		"mod",
	} {
		runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", command, "--help")
	}
}

func TestReleaseMatrixModuleHelpSurfaceIsUsable(t *testing.T) {
	root := findRepoRoot(t)
	for _, mode := range []string{
		"init",
		"add",
		"tidy",
		"check",
		"download",
		"vendor",
		"lock",
		"list",
		"graph",
		"explain",
		"capability",
		"gomod",
		"verify",
	} {
		runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", mode, "--help")
	}
}

func TestReleaseMatrixNestedCommandHelpIsSuccessful(t *testing.T) {
	root := findRepoRoot(t)
	for _, args := range [][]string{
		{"inspect", "bytecode", "--help"},
		{"inspect", "directives", "--help"},
		{"bench", "compare", "--help"},
		{"bench", "strict", "--help"},
		{"bench", "diagnose", "--help"},
	} {
		runCommand(t, root, 30*time.Second, "go", append([]string{"run", "./cmd/leia"}, args...)...)
	}
}

func TestReleaseMatrixDocumentedCIProfilesAreInspectable(t *testing.T) {
	root := findRepoRoot(t)
	for _, path := range []string{
		"CONTRIBUTING.md",
		"docs/guides/tooling.md",
	} {
		data := readFileString(t, filepath.Join(root, filepath.FromSlash(path)))
		for _, profile := range []string{"smoke", "pr"} {
			command := "go run ./cmd/leia ci " + profile + " --list"
			if !strings.Contains(data, command) {
				t.Fatalf("%s must document inspectable CI command %q", path, command)
			}
		}
	}

	for _, profile := range []string{"smoke", "pr", "release"} {
		out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", profile, "--list")
		if !strings.Contains(out, "github.com/never-labs/leia") {
			t.Fatalf("ci %s --list must include module path gate output; got:\n%s", profile, out)
		}
	}
}

func TestReleaseMatrixCIProfilesKeepExampleImportGuards(t *testing.T) {
	root := findRepoRoot(t)

	smokeOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "smoke", "--list")
	for _, want := range []string{"go test ./cmd/leia", "python3 tests/manifest.py check tests benchmarks"} {
		if !strings.Contains(smokeOut, want) {
			t.Fatalf("ci smoke --list must include %q so example/import guards stay in the smoke test matrix; got:\n%s", want, smokeOut)
		}
	}
	examplesCommandGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_command_test.go"))
	for _, snippet := range []string{
		`runExamplesCommand([]string{"check",`,
		`runExamplesCommand([]string{"check", "--json",`,
	} {
		if !strings.Contains(examplesCommandGate, snippet) {
			t.Fatalf("cmd/leia/main_examples_command_test.go must keep release-gated `leia examples check` coverage snippet %q", snippet)
		}
	}

	releaseOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "release", "--list")
	for _, want := range []string{"bash scripts/production_check.sh --full", "bash scripts/release_distribution_check.sh"} {
		if !strings.Contains(releaseOut, want) {
			t.Fatalf("ci release --list must include %q; got:\n%s", want, releaseOut)
		}
	}

	productionOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--list")
	for _, want := range []string{"go test ./... -count=1", "python3 tests/manifest.py check tests benchmarks"} {
		if !strings.Contains(productionOut, want) {
			t.Fatalf("production_check.sh --full --list must include %q so dialect/package-managed example tests stay release-gated; got:\n%s", want, productionOut)
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

func TestReleaseMatrixLicenseNoticeMatchesRepositoryState(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	_, licenseErr := os.Stat(filepath.Join(root, "LICENSE"))
	hasLicense := licenseErr == nil
	if licenseErr != nil && !errors.Is(licenseErr, os.ErrNotExist) {
		t.Fatalf("stat LICENSE: %v", licenseErr)
	}
	unlicensedNotice := "No license has been selected in this repository yet."
	if hasLicense && strings.Contains(readme, unlicensedNotice) {
		t.Fatal("README.md still says no license has been selected even though LICENSE exists")
	}
	if !hasLicense && !strings.Contains(readme, unlicensedNotice) {
		t.Fatal("README.md must keep the no-license notice until a root LICENSE exists")
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

	packages := readFileString(t, filepath.Join(root, "docs", "guides", "packages.md"))
	for _, snippet := range []string{
		"decentralized package model",
		"no npm-style publish step or central registry",
		"go run ./cmd/leia mod add github.com/never-labs/leia-raylib@v0.1.0",
		"go run ./cmd/leia mod download --json",
		"go run ./cmd/leia mod verify --json",
		"go run ./cmd/leia mod vendor --json",
		"go run ./cmd/leia run --mod=vendor main.leia",
		"go run ./cmd/leia run --mod=readonly main.leia",
		"go run ./cmd/leia mod gomod",
	} {
		if !strings.Contains(packages, snippet) {
			t.Fatalf("docs/guides/packages.md must document %q", snippet)
		}
	}
}

func TestReleaseMatrixModuleDocsKeepFirstRunCommandsLocal(t *testing.T) {
	root := findRepoRoot(t)
	gettingStarted := readFileString(t, filepath.Join(root, "docs", "tutorial", "getting-started.md"))
	for _, forbidden := range []string{
		"mod add github.com/never-labs/leia-raylib",
		"mod download",
		"mod vendor",
	} {
		if strings.Contains(gettingStarted, forbidden) {
			t.Fatalf("getting-started module path must stay local-only; found %q", forbidden)
		}
	}

	tmp := t.TempDir()
	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "init", "--module", "github.com/example/project", "--dir", tmp)
	for _, args := range [][]string{
		{"run", "./cmd/leia", "mod", "list", "--json", tmp},
		{"run", "./cmd/leia", "mod", "graph", "--json", tmp},
		{"run", "./cmd/leia", "mod", "capability", "--json", tmp},
		{"run", "./cmd/leia", "mod", "verify", "--json", tmp},
		{"run", "./cmd/leia", "mod", "gomod", tmp},
	} {
		runCommand(t, root, 30*time.Second, "go", args...)
	}
}

func TestReleaseMatrixSecurityDocsUseModuleScopedCapabilityCommand(t *testing.T) {
	root := findRepoRoot(t)
	security := readFileString(t, filepath.Join(root, "docs", "reference", "security", "index.md"))
	if !strings.Contains(security, "cd path/to/module\nleia mod capability --json") {
		t.Fatal("security reference must show module capability command from a module directory")
	}
	if !strings.Contains(security, "controlled by separate switches") {
		t.Fatal("security reference must distinguish capability bits from network/debug/process switches")
	}

	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "leia.mod"), []byte("module github.com/example/secure\nleia 0.1\ncapability fs.read\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "capability", "--json", tmp)
}

func TestReleaseMatrixEmbeddingDocsUsePublicSDKSurface(t *testing.T) {
	root := findRepoRoot(t)
	docs := strings.Join([]string{
		readFileString(t, filepath.Join(root, "docs", "reference", "embedding", "index.md")),
		readFileString(t, filepath.Join(root, "docs", "guides", "embedding.md")),
	}, "\n")
	goDoc := runCommand(t, root, 30*time.Second, "go", "doc", "-all", ".")
	for _, api := range []string{
		"New",
		"Compile",
		"CompileFile",
		"SecuritySandbox",
		"WithLibs",
		"WithCapabilities",
		"WithModuleMode",
		"ModuleOptionsForScript",
		"WithLLMProvider",
		"NewHotLoader",
		"WithHotLoaderVMOptions",
		"ModuleFrom",
		"WithGoImports",
		"LibSafe",
		"LibApp",
		"LibGame",
		"CapSafe",
		"ModuleModeVendor",
	} {
		if !strings.Contains(docs, api) {
			t.Fatalf("embedding docs must mention public API %s", api)
		}
		if !strings.Contains(goDoc, api) {
			t.Fatalf("embedding docs mention %s but root go doc does not expose it", api)
		}
	}
	for _, method := range []string{
		"Exec",
		"ExecContext",
		"Run",
		"RunContext",
		"Call",
		"CallContext",
		"RegisterFunc",
		"RegisterTable",
		"RegisterModule",
		"RegisterModuleFrom",
		"BindStruct",
		"BindStructWithConstructor",
		"BindMethod",
		"Reset",
	} {
		if !strings.Contains(docs, method) {
			t.Fatalf("embedding docs must mention VM.%s", method)
		}
		if !strings.Contains(goDoc, method) {
			t.Fatalf("embedding docs mention VM.%s but root go doc does not expose it", method)
		}
	}

	runCommand(t, root, 30*time.Second, "go", "test", "./examples/embedding", "-run", "Example", "-count=1")
}

func TestReleaseMatrixHotReloadDocsUsePublicSDKSurface(t *testing.T) {
	root := findRepoRoot(t)
	docs := readFileString(t, filepath.Join(root, "docs", "reference", "hot-reload", "index.md"))
	goDoc := runCommand(t, root, 30*time.Second, "go", "doc", "-all", ".")
	for _, api := range []string{
		"NewHotLoader",
		"WithHotLoaderCompileOptions",
		"WithHotLoaderVMOptions",
		"HotLoader",
		"ModuleHandle",
		"HotInstance",
		"ReloadResult",
		"Load",
		"LoadContext",
		"Reload",
		"ReloadContext",
		"ReloadIfChanged",
		"ReloadIfChangedContext",
		"LoadInstance",
		"LoadInstanceContext",
		"Generation",
		"Program",
		"Run",
		"RunContext",
		"Call",
		"CallContext",
		"VM",
	} {
		if !strings.Contains(docs, api) {
			t.Fatalf("hot reload docs must mention public API/semantic term %s", api)
		}
		if !strings.Contains(goDoc, api) {
			t.Fatalf("hot reload docs mention %s but root go doc does not expose it", api)
		}
	}
	for _, snippet := range []string{
		"does not start a filesystem watcher",
		"does not register files into `require()`",
		"`ModuleHandle.Call` runs the latest top-level program",
		"running goroutines are not migrated",
	} {
		if !strings.Contains(docs, snippet) {
			t.Fatalf("hot reload docs missing semantic warning %q", snippet)
		}
	}
}

func TestReleaseMatrixAINativeDocsUsePublicLLMSurface(t *testing.T) {
	root := findRepoRoot(t)
	docs := strings.Join([]string{
		readFileString(t, filepath.Join(root, "docs", "reference", "ai", "index.md")),
		readFileString(t, filepath.Join(root, "docs", "guides", "ai-native.md")),
	}, "\n")
	rootGoDoc := runCommand(t, root, 30*time.Second, "go", "doc", "-all", ".")
	llmGoDoc := runCommand(t, root, 30*time.Second, "go", "doc", "-all", "./llm")

	for _, api := range []string{
		"WithLLMProvider",
		"WithLLMProviderFactory",
		"WithLLMRecorder",
		"WithLLMReplay",
		"WithLLMTrace",
	} {
		if !strings.Contains(docs, api) {
			t.Fatalf("AI-native docs must mention host API %s", api)
		}
		if !strings.Contains(rootGoDoc, api) {
			t.Fatalf("AI-native docs mention %s but root go doc does not expose it", api)
		}
	}

	for _, api := range []string{
		"Provider",
		"ProviderConfig",
		"ProviderFactory",
		"TurnRequest",
		"TurnResult",
		"Tool",
		"ToolCall",
		"Message",
		"NewRecorder",
		"LoadRecords",
		"SaveRecords",
		"NewReplayProvider",
		"NewTraceRecorder",
		"ProviderErrorNetwork",
	} {
		if !strings.Contains(docs, api) {
			t.Fatalf("AI-native docs must mention llm API/semantic term %s", api)
		}
		if !strings.Contains(llmGoDoc, api) {
			t.Fatalf("AI-native docs mention %s but llm go doc does not expose it", api)
		}
	}

	aiRef := readFileString(t, filepath.Join(root, "docs", "reference", "ai", "index.md"))
	if strings.Contains(aiRef, "`money`") || strings.Contains(aiRef, "money accounting accuracy") {
		t.Fatal("AI reference must not promise money as a stable script-level budget dimension")
	}
	for _, dimension := range []string{"`turns`", "`calls`", "`tokens`", "`time`"} {
		if !strings.Contains(aiRef, dimension) {
			t.Fatalf("AI reference must document public budget dimension %s", dimension)
		}
	}
}

func TestReleaseMatrixAINativeExamplesStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	guide := readFileString(t, filepath.Join(root, "docs", "guides", "ai-native.md"))
	for _, forbidden := range []string{
		"examples/llm/agent_as_tool.leia",
	} {
		if strings.Contains(guide, forbidden) {
			t.Fatalf("docs/guides/ai-native.md must not recommend currently failing AI example %s", forbidden)
		}
	}
	if !strings.Contains(guide, "Live-provider examples") || !strings.Contains(guide, "examples/llm/glm_smoke.leia") {
		t.Fatal("AI-native guide must keep live provider examples separated from offline examples")
	}

	for _, example := range []string{
		"examples/llm/agent.leia",
		"examples/llm/incident_response.leia",
	} {
		runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "run", example)
	}
}

func TestReleaseMatrixDocumentedSmokeCommandsStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	for _, path := range []string{
		"README.md",
		"CONTRIBUTING.md",
		"docs/tutorial/getting-started.md",
		"docs/guides/tooling.md",
	} {
		data := readFileString(t, filepath.Join(root, filepath.FromSlash(path)))
		if strings.Contains(data, "go run ./cmd/leia test tests/smoke\n") {
			t.Fatalf("%s must not document the whole smoke directory as a stable test command", path)
		}
		if !strings.Contains(data, "go run ./cmd/leia test tests/smoke/01_basic.leia") {
			t.Fatalf("%s must document the stable single-file smoke test command", path)
		}
	}

	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "eval", `print("hello from leia")`)
	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "test", "tests/smoke/01_basic.leia")
}

func TestReleaseMatrixReadmeToolingCommandsStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	commands := readReleaseReadmeToolingCommands(t, root)
	if len(commands) == 0 {
		t.Fatal("README.md must contain runnable Tooling commands")
	}

	diagDir := filepath.Join(os.TempDir(), "leia-diag")
	_ = os.RemoveAll(diagDir)
	t.Cleanup(func() {
		_ = os.RemoveAll(diagDir)
	})

	for _, command := range commands {
		fields := strings.Fields(command)
		if len(fields) < 4 || fields[0] != "go" || fields[1] != "run" || fields[2] != "./cmd/leia" {
			t.Fatalf("README Tooling command must use `go run ./cmd/leia ...`: %q", command)
		}
		timeout := 60 * time.Second
		if len(fields) > 3 {
			switch fields[3] {
			case "bench", "diag":
				timeout = 180 * time.Second
			}
		}
		runCommand(t, root, timeout, fields[0], fields[1:]...)
	}
}

func TestReleaseMatrixGettingStartedExamplesStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	gettingStarted := readFileString(t, filepath.Join(root, "docs", "tutorial", "getting-started.md"))
	for _, forbidden := range []string{
		"go run ./cmd/leia run examples/hello/metatables.leia",
		"go run ./cmd/leia run examples/llm/agent_as_tool.leia",
		"go run ./cmd/leia run examples/game_engine/game_of_life.leia",
		"go run ./cmd/leia run examples/game_engine/tetris.leia",
		"go run ./cmd/leia run examples/web/hello_server.leia",
	} {
		if strings.Contains(gettingStarted, forbidden) {
			t.Fatalf("docs/tutorial/getting-started.md must not present non-smoke command %q as first-run runnable", forbidden)
		}
	}

	for _, example := range []string{
		"examples/hello/fib.leia",
		"examples/hello/types_demo.leia",
		"examples/concurrency/goroutines_channels.leia",
		"examples/data_processing/data_oriented/soa_kernels.leia",
		"examples/data_processing/data_oriented/dense_matrix_vec_kernels.leia",
	} {
		runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "run", example)
	}
}

func TestReleaseMatrixExampleRunCommentsUseCurrentCLI(t *testing.T) {
	root := findRepoRoot(t)
	var offenders []string
	err := filepath.WalkDir(filepath.Join(root, "examples"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".leia" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		data := readFileString(t, path)
		for lineNo, line := range strings.Split(data, "\n") {
			if !strings.Contains(line, "Run:") && !strings.Contains(line, "go run") && !strings.Contains(line, "leia ") {
				continue
			}
			if strings.Contains(line, "./leia") || strings.Contains(line, "leia examples/") || strings.Contains(line, "./cmd/leia examples/") {
				offenders = append(offenders, rel+":"+strconv.Itoa(lineNo+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk examples: %v", err)
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Fatalf("example run comments must use current `leia run` CLI shape:\n%s", strings.Join(offenders, "\n"))
	}
}

func TestReleaseMatrixDocumentedExampleCommandsStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	examplesDoc := readFileString(t, filepath.Join(root, "docs", "examples", "index.md"))
	for _, forbidden := range []string{
		"go run ./cmd/leia run examples/hello/metatables.leia",
		"go run ./cmd/leia run examples/llm/agent_as_tool.leia",
		"go run ./cmd/leia run examples/game_engine/game_of_life.leia",
		"go run ./cmd/leia run examples/game_engine/tetris.leia",
		"go run ./cmd/leia run examples/web/hello_server.leia",
	} {
		if strings.Contains(examplesDoc, forbidden) {
			t.Fatalf("docs/examples/index.md must not present non-smoke command %q as first-run runnable", forbidden)
		}
	}

	commands := documentedExampleRunCommands(t, examplesDoc)
	if len(commands) == 0 {
		t.Fatal("docs/examples/index.md must contain runnable example commands")
	}
	for _, args := range commands {
		runCommand(t, root, 30*time.Second, "go", args...)
	}
}

func TestReleaseMatrixExamplesReadmeCommandsStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	examplesReadme := readFileString(t, filepath.Join(root, "examples", "README.md"))
	for _, forbidden := range []string{
		"go run ../cmd/leia run hello/metatables.leia",
		"go run ../cmd/leia run llm/agent_as_tool.leia",
		"go run ../cmd/leia run game_engine/game_of_life.leia",
		"go run ../cmd/leia run game_engine/tetris.leia",
		"go run ../cmd/leia run web/hello_server.leia",
	} {
		if strings.Contains(examplesReadme, forbidden) {
			t.Fatalf("examples/README.md must not present non-smoke command %q as no-network runnable", forbidden)
		}
	}

	commands := documentedExamplesReadmeRunCommands(t, examplesReadme)
	if len(commands) == 0 {
		t.Fatal("examples/README.md must contain runnable no-network example commands")
	}
	examplesDir := filepath.Join(root, "examples")
	for _, args := range commands {
		runCommand(t, examplesDir, 30*time.Second, "go", args...)
	}
}

func TestReleaseMatrixPerformanceDocsUseCurrentBenchCommands(t *testing.T) {
	root := findRepoRoot(t)
	for _, path := range []string{
		"docs/reference/performance/index.md",
		"docs/guides/tooling.md",
		"docs/contributing/performance.md",
	} {
		data := readFileString(t, filepath.Join(root, filepath.FromSlash(path)))
		if strings.Contains(data, "bench list") {
			t.Fatalf("%s must not document removed bench list command", path)
		}
		for _, line := range strings.Split(data, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.Contains(trimmed, "bench strict") {
				continue
			}
			if strings.Contains(trimmed, "--json") || strings.HasSuffix(trimmed, "\\") {
				continue
			}
			t.Fatalf("%s documents bench strict without explicit artifact paths: %q", path, trimmed)
		}
	}

	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "bench", "compare", "--bench", "control/sieve", "--runs", "1", "--warmup", "0", "--no-luajit", "--json", "/tmp/leia-release-matrix-timing.json", "--markdown", "/tmp/leia-release-matrix-timing.md")
	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "bench", "strict", "--bench", "control/sieve", "--runs", "1", "--warmup", "0", "--no-luajit", "--json", "/tmp/leia-release-matrix-strict.json", "--markdown", "/tmp/leia-release-matrix-strict.md")
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

func documentedExampleRunCommands(t *testing.T, doc string) [][]string {
	t.Helper()
	var commands [][]string
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "go run ./cmd/leia examples ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			t.Fatalf("unexpected documented example command shape %q", line)
		}
		commands = append(commands, fields[1:])
	}
	return commands
}

func documentedExamplesReadmeRunCommands(t *testing.T, doc string) [][]string {
	t.Helper()
	var commands [][]string
	for _, line := range strings.Split(doc, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "go run ../cmd/leia examples ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			t.Fatalf("unexpected examples README command shape %q", line)
		}
		commands = append(commands, fields[1:])
	}
	return commands
}

func readReleaseReadmeToolingCommands(t *testing.T, root string) []string {
	t.Helper()
	readme := readFileString(t, filepath.Join(root, "README.md"))
	const marker = "## Tooling"
	start := strings.Index(readme, marker)
	if start < 0 {
		t.Fatal("README.md must contain a Tooling section")
	}
	rest := readme[start+len(marker):]
	blockStart := strings.Index(rest, "```bash")
	if blockStart < 0 {
		t.Fatal("README.md Tooling section must contain a bash command block")
	}
	rest = rest[blockStart+len("```bash"):]
	blockEnd := strings.Index(rest, "```")
	if blockEnd < 0 {
		t.Fatal("README.md Tooling bash command block is unterminated")
	}
	var commands []string
	for _, line := range strings.Split(rest[:blockEnd], "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		commands = append(commands, line)
	}
	return commands
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
	return status == "covered"
}

func releaseIgnoredSpecSections() map[string]string {
	return map[string]string{
		"Notation":                   "spec notation and normative wording, not a language feature",
		"Source Code Representation": "source text model covered by lexical/directive gates",
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
