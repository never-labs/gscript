package tests_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/never-labs/leia/internal/support/dialect"
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
		`escaped_char = "\\" ( "\\" | '"' | "'" | "a" | "b" | "f" | "n" | "r" | "t" | "v"`,
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

	if !strings.Contains(overview, "[index.md](index.md)") || !strings.Contains(overview, "compatibility overview") {
		t.Fatal("docs/spec/language.md must point old links to the chaptered spec entrypoint")
	}
	if !strings.Contains(docsHome, "(spec/index.md)") {
		t.Fatal("docs/index.md must link the chaptered language spec entrypoint")
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
		"ai-dialect.md",
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

func TestReleaseMatrixCoveredReleaseCellsHaveExecutableEvidence(t *testing.T) {
	root := findRepoRoot(t)
	matrix := loadReleaseFeatureMatrix(t, root)

	var missing []string
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		for _, field := range []string{"semantic_gate", "conformance_case", "perf_hot_case"} {
			cell := decodeReleaseCoverageCell(t, feature, i, id, field)
			if !releaseGateStatus(cell.Status) {
				continue
			}
			hasExecutable := false
			for _, ref := range cell.Refs {
				if releaseMatrixRefIsExecutableGate(ref) {
					hasExecutable = true
					break
				}
			}
			if !hasExecutable {
				missing = append(missing, id+"."+field)
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("covered release-facing feature matrix cells need at least one executable test, script, benchmark, or runnable example ref: %s", strings.Join(missing, ", "))
	}
}

func TestReleaseMatrixSpecGateCommandsStaySynchronized(t *testing.T) {
	root := findRepoRoot(t)
	releaseMatrixCmd := "go test ./tests -run 'TestReleaseMatrix' -count=1"
	internalReleaseMatrixCmd := "go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1"
	specExamplesCmd := "go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1"
	docsCheckCmd := "bash scripts/docs_check.sh"
	ciReleaseListCmd := "go run ./cmd/leia ci release --list"
	ciReleaseVersionListCmd := "go run ./cmd/leia ci release --release-version vX.Y.Z --list"
	productionFullCmd := "bash scripts/production_check.sh --full --release-profile --release-version vX.Y.Z"
	performanceSmokeCmd := "bash scripts/performance_gate.sh --smoke"
	fullPerfGateCmd := "bash scripts/performance_gate.sh --full"
	publicReleaseBlockersCmd := "bash scripts/public_release_blockers_check.sh --require-resolved"
	releaseDistributionCmd := "bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows"
	releaseNotesCmd := "bash scripts/release_notes_check.sh --require-ready --version vX.Y.Z"
	strictReleaseArtifactsCmd := "bash scripts/release_artifacts_check.sh --build --require-clean --require-tag --version vX.Y.Z"

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
				"feature coverage records under `tests/`",
				"docs/spec/index.md",
			},
		},
		{
			path: "docs/release/index.md",
			snippets: []string{
				"## Machine-Checkable Release Evidence",
				ciReleaseVersionListCmd,
				productionFullCmd,
				"profile is the release validation source of truth",
				"q conformance",
				"local artifact installation evidence",
				releaseDistributionCmd,
				"feature coverage records under `tests/`",
				"docs/spec/index.md",
			},
		},
		{
			path: "docs/release/notes-template.md",
			snippets: []string{
				ciReleaseVersionListCmd,
				productionFullCmd,
				releaseMatrixCmd,
				docsCheckCmd,
				fullPerfGateCmd,
				publicReleaseBlockersCmd,
				releaseDistributionCmd,
				releaseNotesCmd,
				strictReleaseArtifactsCmd,
				"List known issues, or write `None known` after release validation.",
			},
		},
		{
			path: "docs/guides/tooling.md",
			snippets: []string{
				ciReleaseListCmd,
			},
		},
		{
			path: "scripts/docs_check.sh",
			snippets: []string{
				releaseMatrixCmd,
				specExamplesCmd,
				"TestReleaseMatrixFeatureDocsStayCoveredBySpecAndReference|TestReleaseMatrixDocsIndexCoversReferenceEntrypoints",
				"docs/spec/index.md",
				"checked-in local preview",
				"docs/_config.yml must exclude it from GitHub Pages",
				"tests/feature_matrix.json",
				"reference entrypoints",
				"find reference -type f -name '*.md' | sort",
				"generated reference doc is missing from docs",
				"GENERATED_REFERENCE_COUNT",
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
		internalReleaseMatrixCmd,
		specExamplesCmd,
		docsCheckCmd,
		performanceSmokeCmd,
	} {
		if !strings.Contains(productionOut, snippet) {
			t.Fatalf("production_check.sh --quick --list must keep release/spec gate %q; got:\n%s", snippet, productionOut)
		}
	}
	if strings.Contains(productionOut, "--no-luajit") {
		t.Fatalf("production_check.sh --quick --list must not weaken performance gates with --no-luajit; got:\n%s", productionOut)
	}
	fullOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--release-profile", "--list")
	if !strings.Contains(fullOut, fullPerfGateCmd) {
		t.Fatalf("production_check.sh --full --list must keep full LuaJIT performance gate %q; got:\n%s", fullPerfGateCmd, fullOut)
	}
	if strings.Contains(fullOut, "--no-luajit") {
		t.Fatalf("production_check.sh --full --list must not weaken release performance gates with --no-luajit; got:\n%s", fullOut)
	}
	productionCheck := readFileString(t, filepath.Join(root, "scripts", "production_check.sh"))
	criticalStart := strings.Index(productionCheck, "RELEASE_CRITICAL_SKIP_NAMES=(")
	if criticalStart < 0 {
		t.Fatal("scripts/production_check.sh must define RELEASE_CRITICAL_SKIP_NAMES")
	}
	criticalRest := productionCheck[criticalStart:]
	criticalEnd := strings.Index(criticalRest, "\n)")
	if criticalEnd < 0 {
		t.Fatal("scripts/production_check.sh RELEASE_CRITICAL_SKIP_NAMES block must be closed")
	}
	criticalList := criticalRest[:criticalEnd]
	for _, critical := range []string{
		`"Language Conformance Surface"`,
		`"Q Conformance Gate"`,
		`"Public Release Blockers"`,
		`"Release Distribution"`,
		`"Release Notes"`,
		`"Release Artifacts"`,
	} {
		if !strings.Contains(criticalList, critical) {
			t.Fatalf("scripts/production_check.sh release-critical skip list must include %s", critical)
		}
	}
}

func TestReleaseMatrixReleaseProfileFailsCriticalSkips(t *testing.T) {
	root := findRepoRoot(t)
	goPath, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go not available: %v", err)
	}
	tmpBin := t.TempDir()
	if err := os.Symlink(goPath, filepath.Join(tmpBin, "go")); err != nil {
		t.Fatalf("symlink go into test PATH: %v", err)
	}

	cmd := exec.Command("bash", "scripts/production_check.sh", "--full", "--release-profile")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+tmpBin+":/usr/bin:/bin")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("release profile unexpectedly passed with luajit absent from PATH:\n%s", out)
	}
	text := string(out)
	for _, want := range []string{
		"Runnable checks:",
		"Language Conformance Surface: missing luajit",
		"Release profile requires these checks to run instead of skip:",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("release profile critical-skip output missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "=== RUN Correctness ===") {
		t.Fatalf("release profile should fail before running checks when critical skips exist:\n%s", text)
	}
}

func TestReleaseMatrixReadmeLanguageContractFailsThroughReleaseGates(t *testing.T) {
	root := findRepoRoot(t)
	releaseMatrixCmd := "go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1"
	specContractCmd := "go test ./tests/docs/spec -count=1"
	specExamplesCmd := "go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1"
	docsCheckCmd := "bash scripts/docs_check.sh"

	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, snippet := range []string{
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
		"q`sum ${a}`",
		"turn {",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README.md must keep language contract snippet %q", snippet)
		}
	}

	specGate := readFileString(t, filepath.Join(root, "tests", "docs", "spec", "spec_contract_test.go"))
	for _, snippet := range []string{
		"TestReadmeAndSpecStableContractStayAligned",
		"`tests/feature_matrix.json`",
		"at least one semantic or conformance gate",
	} {
		if !strings.Contains(specGate, snippet) {
			t.Fatalf("tests/docs/spec/spec_contract_test.go must keep README/spec contract gate snippet %q", snippet)
		}
	}

	featureMatrixGate := readFileString(t, filepath.Join(root, "tests", "feature_matrix_test.go"))
	for _, snippet := range []string{
		"TestFeatureMatrixCoversReadmeStableContract",
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
		"q`sum ${a}`",
		`requireFeature(t, features, "release_evidence_gates")`,
		`requireFeatureCellRefs(t, releaseEvidence, "release_evidence_gates", "semantic_gate"`,
		`"scripts/docs_check.sh"`,
		`"scripts/production_check.sh"`,
		`"tests/release_matrix_test.go"`,
	} {
		if !strings.Contains(featureMatrixGate, snippet) {
			t.Fatalf("tests/feature_matrix_test.go must keep README/feature-matrix contract gate snippet %q", snippet)
		}
	}

	docsCheck := readFileString(t, filepath.Join(root, "scripts", "docs_check.sh"))
	for _, snippet := range []string{
		specContractCmd,
		specExamplesCmd,
		"README spec link and docs/spec stability contract",
		"`tests/feature_matrix.json`",
	} {
		if !strings.Contains(docsCheck, snippet) {
			t.Fatalf("scripts/docs_check.sh must keep README/spec contract gate snippet %q", snippet)
		}
	}

	quickOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--quick", "--list")
	for _, snippet := range []string{
		releaseMatrixCmd,
		specExamplesCmd,
		docsCheckCmd,
	} {
		if !strings.Contains(quickOut, snippet) {
			t.Fatalf("production_check.sh --quick --list must keep README language-contract gate %q; got:\n%s", snippet, quickOut)
		}
	}

	fullOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--list")
	for _, snippet := range []string{
		"go test ./... -count=1",
		"Release Matrix Metadata: covered by Correctness (go test ./... -count=1)",
		docsCheckCmd,
	} {
		if !strings.Contains(fullOut, snippet) {
			t.Fatalf("production_check.sh --full --list must keep README language-contract gate %q; got:\n%s", snippet, fullOut)
		}
	}
}

func TestReleaseMatrixPackageMetadataPromiseHasProductionGates(t *testing.T) {
	root := findRepoRoot(t)
	docsHome := readFileString(t, filepath.Join(root, "docs", "index.md"))
	for _, snippet := range []string{
		"[Modules](reference/modules/index.md)",
		"[Packages and modules](guides/packages.md)",
	} {
		if !strings.Contains(docsHome, snippet) {
			t.Fatalf("docs/index.md must keep package metadata entrypoint %q", snippet)
		}
	}

	featureMatrixGate := readFileString(t, filepath.Join(root, "tests", "feature_matrix_test.go"))
	for _, snippet := range []string{
		`requireFeature(t, features, "module_package_management")`,
		"capability summaries, and optional Go-native binding metadata",
		"Local metadata commands do not need network access",
	} {
		if !strings.Contains(featureMatrixGate, snippet) {
			t.Fatalf("tests/feature_matrix_test.go must keep package metadata gate snippet %q", snippet)
		}
	}

	releaseDocs := readFileString(t, filepath.Join(root, "docs", "release", "index.md"))
	for _, snippet := range []string{
		"`docs/reference/modules/index.md`",
		"document any experimental language, stdlib, AI, package, or JIT behavior",
	} {
		if !strings.Contains(releaseDocs, snippet) {
			t.Fatalf("docs/release/index.md must keep package metadata release evidence snippet %q", snippet)
		}
	}

	fullOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--list")
	for _, snippet := range []string{
		"Module Path Gate",
		`test "$(go list -m)" = "github.com/never-labs/leia"`,
		"go run ./cmd/leia mod init --module example.com/cli-experience",
		"go run ./cmd/leia mod check --json",
	} {
		if !strings.Contains(fullOut, snippet) {
			t.Fatalf("production_check.sh --full --list must keep package metadata gate %q; got:\n%s", snippet, fullOut)
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
		timeout := 30 * time.Second
		if len(args) >= 4 && args[2] == "doc" && args[3] == "check" && len(args) == 4 {
			timeout = 90 * time.Second
		}
		runCommand(t, root, timeout, "go", args...)
	}
}

func TestReleaseMatrixTopLevelCommandHelpIsSuccessful(t *testing.T) {
	root := findRepoRoot(t)
	for _, command := range []string{
		"eval",
		"evaluate",
		"run",
		"repl",
		"fmt",
		"lint",
		"test",
		"examples",
		"version",
		"env",
		"config",
		"check",
		"capabilities",
		"ci",
		"playground",
		"lsp",
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

	for _, profile := range []string{"smoke", "pr"} {
		out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", profile, "--list")
		if !strings.Contains(out, "github.com/never-labs/leia") {
			t.Fatalf("ci %s --list must include module path gate output; got:\n%s", profile, out)
		}
	}
	releaseOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--release-profile", "--list")
	if !strings.Contains(releaseOut, `test "$(go list -m)" = "github.com/never-labs/leia"`) {
		t.Fatalf("production release profile must include module path gate output; got:\n%s", releaseOut)
	}
}

func TestReleaseMatrixReleaseArtifactsInstallSharedLSP(t *testing.T) {
	root := findRepoRoot(t)
	for _, item := range []struct {
		path     string
		snippets []string
	}{
		{
			path: ".goreleaser.yaml",
			snippets: []string{
				"id: leia-lsp",
				"main: ./cmd/leia-lsp",
				"- leia-lsp",
			},
		},
		{
			path: "scripts/release_artifacts.sh",
			snippets: []string{
				"leia-lsp_${version}_${goos}_${goarch}",
				"ldflags=\"-s -w -X main.cliVersion=$version\"",
				"go build -trimpath -ldflags=\"$ldflags\" -o \"$lsp_binary_path\" ./cmd/leia-lsp",
				"lsp_artifact=$lsp_binary_name",
			},
		},
		{
			path: "scripts/release_artifacts_check.sh",
			snippets: []string{
				"expected 3 checksum entries",
				"built CLI still reports dev version",
				"\"$lsp_binary_path\" --help >/dev/null",
				"package_install_archive",
				"--require-clean",
				"--require-tag",
				"--base-url \"file://$release_dir\"",
				"local install archive verified",
				"lsp_artifact=$lsp_binary_name",
			},
		},
		{
			path: "scripts/install.sh",
			snippets: []string{
				"--base-url URL",
				"LEIA_INSTALL_BASE_URL",
				"fetch_url()",
				"file://*)",
				"validate_archive_entries",
				"unexpected archive entry",
				"lsp_binary_name=\"leia-lsp\"",
				"lsp_install_path=",
				"install -m 0755 \"$extract_dir/$lsp_binary_name\" \"$lsp_install_path\"",
			},
		},
		{
			path: "scripts/release_distribution_check.sh",
			snippets: []string{
				"expected_lsp_path=",
				"lsp_install_path=/tmp/leia-bin/leia-lsp",
				"--require-workflows",
				".github/workflows/release.yml",
				".github/workflows/distribution-check.yml",
				".github/workflows/pages.yml",
				"require_file docs/_config.yml",
				"require_contains docs/_config.yml \"spec/index.html\"",
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				"check_local_install_fixture",
				"install accepted archive with unexpected entry",
				"install accepted zip archive with unexpected entry",
				"--base-url \"file://$release_dir\"",
				"local install fixture verified",
			},
		},
		{
			path: ".github/workflows/ci.yml",
			snippets: []string{
				"name: CI",
				"go run ./cmd/leia ci pr --no-luajit",
				"go-version-file: go.mod",
			},
		},
		{
			path: ".github/workflows/distribution-check.yml",
			snippets: []string{
				"name: Distribution Check",
				".github/workflows/pages.yml",
				"docs/_config.yml",
				"scripts/production_check.sh",
				"scripts/public_release_blockers_check.sh",
				"scripts/release_notes_check.sh",
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				`"$(go env GOPATH)/bin/goreleaser" --version`,
				"bash scripts/release_notes_check.sh",
				"bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows",
				`"$(go env GOPATH)/bin/goreleaser" release --snapshot --clean --skip=publish`,
			},
		},
		{
			path: ".github/workflows/release.yml",
			snippets: []string{
				"name: Release",
				"go run ./cmd/leia ci release",
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				"release tags must match vMAJOR.MINOR.PATCH",
				`"$(go env GOPATH)/bin/goreleaser" --version`,
				"LEIA_RELEASE_REQUIRE_TAG=1",
				"LEIA_RELEASE_ARTIFACT_VERSION=\"${GITHUB_REF_NAME}\"",
				`"$(go env GOPATH)/bin/goreleaser" release --snapshot --clean --skip=publish`,
				`"$(go env GOPATH)/bin/goreleaser" release --clean`,
				`--release-notes "docs/release/notes/${GITHUB_REF_NAME}.md"`,
				"secrets.GITHUB_TOKEN",
			},
		},
		{
			path: ".github/workflows/pages.yml",
			snippets: []string{
				"name: Pages",
				"branches: [main]",
				"go run ./cmd/leia doc check",
				"actions/jekyll-build-pages",
				"source: ./docs",
				"destination: ./_site",
				"actions/deploy-pages",
			},
		},
		{
			path: "docs/_config.yml",
			snippets: []string{
				"title: Leia",
				"exclude:",
				"spec/index.html",
			},
		},
		{
			path: "docs/guides/editors.md",
			snippets: []string{
				"leia-lsp",
				"syntax diagnostics",
				"release archives and `scripts/install.sh` should install both `leia` and",
			},
		},
		{
			path: "docs/release/index.md",
			snippets: []string{
				"Release archives must include both executables",
				"`leia-lsp`, the shared language server",
				"local `file://` tar.gz/zip install fixtures",
				"bash scripts/install.sh --version v0.1.0 --base-url file:///tmp/leia-release --bin-dir /tmp/leia-bin",
			},
		},
	} {
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.path)))
		for _, snippet := range item.snippets {
			if !strings.Contains(text, snippet) {
				t.Fatalf("%s must keep shared LSP release evidence snippet %q", item.path, snippet)
			}
		}
	}
	pagesWorkflow := readFileString(t, filepath.Join(root, ".github", "workflows", "pages.yml"))
	if strings.Contains(pagesWorkflow, "paths:") {
		t.Fatal(".github/workflows/pages.yml must not use a narrow paths filter; generated docs depend on code and registry inputs")
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
	if strings.TrimSpace(releaseOut) != "bash scripts/production_check.sh --full --release-profile" {
		t.Fatalf("ci release --list must delegate to production release profile only; got:\n%s", releaseOut)
	}
	versionedReleaseOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "release", "--release-version", "vX.Y.Z", "--list")
	if strings.TrimSpace(versionedReleaseOut) != "bash scripts/production_check.sh --full --release-profile --release-version vX.Y.Z" {
		t.Fatalf("ci release --release-version --list must delegate to versioned production release profile only; got:\n%s", versionedReleaseOut)
	}
	badReleaseVersion := runCommandResult(root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "release", "--release-version", "bad version")
	if badReleaseVersion.err == nil || !strings.Contains(badReleaseVersion.stderr, "--release-version must match vMAJOR.MINOR.PATCH") {
		t.Fatalf("ci release bad version should fail with a clear format error\nstdout:\n%s\nstderr:\n%s", badReleaseVersion.stdout, badReleaseVersion.stderr)
	}

	productionOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--list")
	for _, want := range []string{"go test ./... -count=1", "python3 tests/manifest.py check tests benchmarks"} {
		if !strings.Contains(productionOut, want) {
			t.Fatalf("production_check.sh --full --list must include %q so dialect/package-managed example tests stay release-gated; got:\n%s", want, productionOut)
		}
	}
}

func TestReleaseMatrixReadmeCLIExperienceCommandsHaveEvidence(t *testing.T) {
	root := findRepoRoot(t)
	for _, item := range []struct {
		command string
		path    string
		snippet string
	}{
		{command: "leia check", path: "cmd/leia/main_ci_test.go", snippet: `runCheckCommand([]string{"--json"`},
		{command: "leia test", path: "cmd/leia/main_test_run_test.go", snippet: `runTestCommand([]string{"--manifest-check"`},
		{command: "leia ci", path: "cmd/leia/main_ci_test.go", snippet: `runCICommand([]string{"release", "--list"`},
		{command: "leia examples", path: "cmd/leia/main_examples_command_test.go", snippet: `runExamplesCommand([]string{"check"`},
		{command: "leia playground", path: "cmd/leia/main_playground_test.go", snippet: `func TestPlaygroundRunAPI`},
		{command: "leia evaluate", path: "cmd/leia/main_examples_test.go", snippet: `runEvaluateCommand([]string{"--json"`},
		{command: "leia mod", path: "cmd/leia/main_mod_test.go", snippet: `runModCommand([]string{"check"`},
		{command: "leia run", path: "cmd/leia/main_run_test.go", snippet: `runRunCommand([]string{"--vm"`},
	} {
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.path)))
		if !strings.Contains(text, item.snippet) {
			t.Fatalf("%s must keep focused test evidence for %s via snippet %q", item.path, item.command, item.snippet)
		}
	}

	productionOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--list")
	for _, want := range []string{
		"CLI Experience",
		"go run ./cmd/leia run tests/smoke/01_basic.leia",
		"go run ./cmd/leia test tests/smoke/01_basic.leia",
		"go run ./cmd/leia check --quick tests/smoke/01_basic.leia",
		"go run ./cmd/leia examples check --jobs=6 examples/hello/fib.leia examples/hello/types_demo.leia examples/hello/dialects.leia",
		"go run ./cmd/leia evaluate --json examples/evaluate/basic_assert.leia",
		"go run ./cmd/leia mod init --module example.com/cli-experience",
		"go run ./cmd/leia mod check --json",
		"go run ./cmd/leia playground --help",
	} {
		if !strings.Contains(productionOut, want) {
			t.Fatalf("production_check.sh --full --list must keep README CLI experience gate %q; got:\n%s", want, productionOut)
		}
	}
}

func TestReleaseMatrixToolingGuideCommandsHaveEvidence(t *testing.T) {
	root := findRepoRoot(t)
	toolingGuide := readFileString(t, filepath.Join(root, "docs", "guides", "tooling.md"))

	for _, item := range []struct {
		category        string
		command         string
		evidencePath    string
		evidenceSnippet string
	}{
		{
			category:        "eval",
			command:         "go run ./cmd/leia eval 'print(1 + 2 + 3)'",
			evidencePath:    "cmd/leia/eval.go",
			evidenceSnippet: "func runEvalCommand",
		},
		{
			category:        "examples",
			command:         "go run ./cmd/leia examples check examples/hello/dialects.leia",
			evidencePath:    "cmd/leia/main_examples_command_test.go",
			evidenceSnippet: "TestExamplesCommandChecksSelectedExamples",
		},
		{
			category:        "docs",
			command:         "go run ./cmd/leia doc check",
			evidencePath:    "cmd/leia/doc.go",
			evidenceSnippet: "func runDocCommand",
		},
		{
			category:        "benchmarks",
			command:         "go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3",
			evidencePath:    "cmd/leia/main_bench_test.go",
			evidenceSnippet: "TestBenchCommandDispatchesCompareHarness",
		},
	} {
		if !strings.Contains(toolingGuide, item.command) {
			t.Fatalf("docs/guides/tooling.md commands must cover %s via %q", item.category, item.command)
		}
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.evidencePath)))
		if !strings.Contains(text, item.evidenceSnippet) {
			t.Fatalf("%s must keep focused test evidence for %s tooling via %q", item.evidencePath, item.category, item.evidenceSnippet)
		}
	}

	for _, snippet := range []string{
		"## Modules",
		"## Documentation",
		"## Diagnostics",
		"## Playground",
		"## Release Evidence",
		"go run ./cmd/leia mod verify --json examples/ui/package_managed",
		"go run ./cmd/leia fmt --check --json tests/smoke/01_basic.leia",
		"go run ./cmd/leia doc check",
		"go run ./cmd/leia doc check --json",
		"GitHub Pages publishes `docs/` through `.github/workflows/pages.yml`",
		"bash scripts/worktree_audit.sh --json",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-go-tests --skip-benchmarks --json",
		"go run ./cmd/leia playground --help",
		"go run ./cmd/leia playground --addr 127.0.0.1:8080",
		"bash scripts/production_check.sh --quick --list --json",
		"go run ./cmd/leia ci release --release-version vX.Y.Z --list --json",
		"bash scripts/performance_gate.sh --validate-only /tmp/leia_performance_gate/timing_gate.json --json",
		"bash scripts/q_conformance_gate.sh --scope core --bench none --json",
		"bash scripts/editor_check.sh --json",
		"bash scripts/release_distribution_check.sh --json",
		"bash scripts/release_artifacts_check.sh --json --version vX.Y.Z",
		"bash scripts/release_artifacts_check.sh",
	} {
		if !strings.Contains(toolingGuide, snippet) {
			t.Fatalf("docs/guides/tooling.md must keep README tooling evidence snippet %q", snippet)
		}
	}

	for _, item := range []struct {
		path    string
		snippet string
	}{
		{path: "scripts/docs_check.sh", snippet: "docs_check.sh: checked"},
		{path: "scripts/docs_check.sh", snippet: `"release_distribution_check": root / "scripts" / "release_distribution_check.sh"`},
		{path: "scripts/diagnostics_bundle.sh", snippet: "Collects git revision/status"},
		{path: "scripts/worktree_audit.sh", snippet: "--json"},
		{path: "scripts/performance_gate.sh", snippet: "benchmarks/timing_compare.py"},
		{path: "scripts/production_check.sh", snippet: "add_release_smoke"},
		{path: "scripts/production_check.sh", snippet: "RELEASE_CRITICAL_SKIP_NAMES"},
		{path: "scripts/production_check.sh", snippet: "Release profile requires these checks to run instead of skip:"},
		{path: "scripts/public_release_blockers_check.sh", snippet: "--require-resolved"},
		{path: "scripts/release_notes_check.sh", snippet: "--require-ready"},
		{path: "scripts/release_artifacts_check.sh", snippet: "Default mode runs a dry-run"},
	} {
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.path)))
		if !strings.Contains(text, item.snippet) {
			t.Fatalf("%s must keep tooling script evidence snippet %q", item.path, item.snippet)
		}
	}
}

func TestReleaseMatrixToolingAuditCommandsStayInToolingGuide(t *testing.T) {
	root := findRepoRoot(t)
	toolingGuide := readFileString(t, filepath.Join(root, "docs", "guides", "tooling.md"))
	for _, command := range releaseToolingAuditCommands() {
		if !strings.Contains(toolingGuide, command) {
			t.Fatalf("docs/guides/tooling.md must document tooling audit command %q", command)
		}
	}
}

func TestReleaseMatrixReleaseEvidenceExamplesStayFeatureGated(t *testing.T) {
	root := findRepoRoot(t)
	features := loadFeatureMatrixFeatureMap(t, root)
	releaseEvidence := requireFeature(t, features, "release_evidence_gates")
	requireFeatureCellRefs(t, releaseEvidence, "release_evidence_gates", "semantic_gate",
		"cmd/leia/main_examples_command_test.go",
		"examples/tooling/release_evidence_pipeline.leia",
		"examples/tooling/release_gate_project/main.leia",
		"examples/automation/release_fixture_matrix.leia",
		"examples/automation/release_risk_digest.leia",
	)

	exampleGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_command_test.go"))
	for _, snippet := range []string{
		"repo-tooling-release_evidence_pipeline",
		"repo-tooling-release_gate_project-main",
		"repo-automation-release_fixture_matrix",
		"repo-automation-release_risk_digest",
	} {
		if !strings.Contains(exampleGate, snippet) {
			t.Fatalf("cmd/leia/main_examples_command_test.go must keep release evidence example check %q", snippet)
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
		"docs/release/decisions.md",
		"docs/contributing/performance.md",
		"docs/reference/platforms/index.md",
	} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			t.Fatalf("release community entrypoint %s is missing: %v", path, err)
		}
	}

	docsIndex := readFileString(t, filepath.Join(root, "docs", "index.md"))
	for _, ref := range []string{
		"contributing/performance.md",
		"reference/platforms/index.md",
		"release/decisions.md",
	} {
		if !strings.Contains(docsIndex, ref) {
			t.Fatalf("docs/index.md must link %s", ref)
		}
	}
	for _, ref := range []string{
		"https://github.com/never-labs/leia/blob/main/SECURITY.md",
		"https://github.com/never-labs/leia/blob/main/CONTRIBUTING.md",
		"https://github.com/never-labs/leia/blob/main/CODE_OF_CONDUCT.md",
		"https://github.com/never-labs/leia/blob/main/examples/README.md",
	} {
		if !strings.Contains(docsIndex, ref) {
			t.Fatalf("docs/index.md must link %s", ref)
		}
	}

	release := readFileString(t, filepath.Join(root, "docs", "release", "index.md"))
	for _, snippet := range []string{
		"choose a license and add a root `LICENSE` file",
		"docs/release/notes-template.md",
		"docs/release/decisions.md",
		"complete the release decisions recorded in `docs/release/decisions.md`",
		"examples/README.md",
		"docs/reference/platforms/index.md",
		"bash scripts/production_check.sh --full --release-profile --release-version vX.Y.Z --list --json",
		"go run ./cmd/leia doc check --json",
		"bash scripts/q_conformance_gate.sh --scope core --bench smoke --json",
		"LEIA_SKIP_TIMING_COMPARE=1 bash benchmarks/q_performance_suite.sh > /tmp/leia-q-perf-output.txt",
		"python3 benchmarks/q_perf_report.py --from-output /tmp/leia-q-perf-output.txt --check --json /tmp/leia-q-perf-report.json --markdown /tmp/leia-q-perf-report.md",
		"bash scripts/editor_check.sh --json",
		"bash scripts/public_release_blockers_check.sh --json",
		"bash scripts/release_notes_check.sh --json --version vX.Y.Z",
		"bash scripts/release_distribution_check.sh --json",
		"bash scripts/install.sh --version vX.Y.Z --os darwin --arch arm64 --bin-dir /tmp/leia-bin --dry-run --json",
		"bash scripts/release_artifacts.sh --dry-run --version vX.Y.Z --json",
		"bash scripts/release_artifacts_check.sh --json --version vX.Y.Z",
	} {
		if !strings.Contains(release, snippet) {
			t.Fatalf("docs/release/index.md must mention %q", snippet)
		}
	}

	decisions := readFileString(t, filepath.Join(root, "docs", "release", "decisions.md"))
	for _, snippet := range []string{
		"Public releases require explicit maintainer decisions",
		"## Required Before Public Release",
		"| Area | Decision Needed | Current Status |",
		"| License | Choose the repository license",
		"| Security reporting | Confirm the private reporting route",
		"| Platform support | Define tested and supported OS/architecture combinations",
		"| Release channels | Decide which channels are public",
		"| Artifact signing | Decide whether SHA256 checksums are sufficient",
		"| Compatibility policy | Define the pre-1.0 compatibility promise",
		"The repository has no selected license until a root `LICENSE` file exists.",
		"whether GitHub private security advisories are enabled",
		"tested OS/architecture combinations",
		"whether `scripts/install.sh` is a supported install path",
		"whether SHA256 checksums are sufficient",
		"checksum and signing requirements",
		"Optimizations, JIT availability, typed kernels, and provider integrations are not compatibility guarantees",
	} {
		if !strings.Contains(decisions, snippet) {
			t.Fatalf("docs/release/decisions.md must keep maintainer decision snippet %q", snippet)
		}
	}
}

func TestReleaseMatrixPublicReleaseBlockersExplainDecisionWork(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/public_release_blockers_check.sh")

	for _, snippet := range []string{
		"unresolved release decision: License: Choose the repository license and whether a `NOTICE` file is required. (Open.)",
		"unresolved release decision: Security reporting: Confirm the private reporting route, contact path, and disclosure policy. (Open.)",
		"unresolved release decision: Platform support: Define tested and supported OS/architecture combinations for the release. (Open.)",
		"unresolved release decision: Release channels: Decide which channels are public: GitHub Releases, install script, `go install`, package managers, or others. (Open.)",
		"unresolved release decision: Artifact signing: Decide whether SHA256 checksums are sufficient or whether cosign, GPG, or another signing flow is required. (Open.)",
		"unresolved release decision: Compatibility policy: Define the pre-1.0 compatibility promise and the intended v1.0 stable surface. (Open.)",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("public release blocker output must include actionable release decision detail %q; got:\n%s", snippet, out)
		}
	}
}

func TestReleaseMatrixPublicReleaseBlockersJSONIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/public_release_blockers_check.sh", "--json")
	var report struct {
		SchemaVersion   int      `json:"schema_version"`
		Status          string   `json:"status"`
		RequireResolved bool     `json:"require_resolved"`
		BlockerCount    int      `json:"blocker_count"`
		Blockers        []string `json:"blockers"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("public release blocker JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "blocked" || report.RequireResolved || report.BlockerCount != len(report.Blockers) {
		t.Fatalf("public release blocker JSON = %+v, want blocked schema v1 report", report)
	}
	for _, snippet := range []string{
		"missing root LICENSE file",
		"unresolved release decision: License: Choose the repository license and whether a `NOTICE` file is required. (Open.)",
		"unresolved release decision: Compatibility policy: Define the pre-1.0 compatibility promise and the intended v1.0 stable surface. (Open.)",
	} {
		if !stringSliceContains(report.Blockers, snippet) {
			t.Fatalf("public release blocker JSON missing blocker %q: %+v", snippet, report.Blockers)
		}
	}
}

func TestReleaseMatrixInspectBytecodeJSONIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "inspect.leia")
	src := `func add(a, b) {
    return a + b
}
print(add(1, 2))
`
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "inspect", "bytecode", "--json", path)
	var report struct {
		SchemaVersion int    `json:"schema_version"`
		OK            bool   `json:"ok"`
		Status        string `json:"status"`
		Source        string `json:"source"`
		SelectedProto string `json:"selected_proto"`
		Recursive     bool   `json:"recursive"`
		ProtoCount    int    `json:"proto_count"`
		Proto         struct {
			DisplayName      string `json:"display_name"`
			NumParams        int    `json:"num_params"`
			InstructionCount int    `json:"instruction_count"`
			ChildProtoCount  int    `json:"child_proto_count"`
			Tier1            struct {
				Allowed bool   `json:"allowed"`
				Reason  string `json:"reason"`
			} `json:"tier1"`
			Tier2 struct {
				Allowed bool   `json:"allowed"`
				Reason  string `json:"reason"`
			} `json:"tier2"`
			Disassembly string `json:"disassembly"`
			Children    []struct {
				DisplayName      string `json:"display_name"`
				NumParams        int    `json:"num_params"`
				InstructionCount int    `json:"instruction_count"`
				Disassembly      string `json:"disassembly"`
			} `json:"children"`
		} `json:"proto"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("inspect bytecode JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || !report.OK || report.Status != "pass" || report.Source != path || report.SelectedProto != "<main>" || !report.Recursive || report.ProtoCount != 2 {
		t.Fatalf("inspect bytecode JSON = %+v, want recursive main schema v1 report", report)
	}
	if report.Proto.DisplayName != "<main>" || report.Proto.InstructionCount == 0 || report.Proto.ChildProtoCount != 1 || len(report.Proto.Children) != 1 {
		t.Fatalf("inspect bytecode main proto = %+v, want one child and bytecode metadata", report.Proto)
	}
	if report.Proto.Tier1.Reason == "" || report.Proto.Tier2.Reason == "" {
		t.Fatalf("inspect bytecode JSON missing JIT callable reasons: %+v %+v", report.Proto.Tier1, report.Proto.Tier2)
	}
	child := report.Proto.Children[0]
	if child.DisplayName != "add" || child.NumParams != 2 || child.InstructionCount == 0 || !strings.Contains(child.Disassembly, "RETURN") {
		t.Fatalf("inspect bytecode child proto = %+v, want add disassembly", child)
	}
}

func TestReleaseMatrixFmtCheckJSONIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "formatted.leia")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "fmt", "--check", "--json", path)
	var report struct {
		SchemaVersion int `json:"schema_version"`
		OK            bool
		Mode          string `json:"mode"`
		Stdin         bool   `json:"stdin"`
		FileCount     int    `json:"file_count"`
		ChangedCount  int    `json:"changed_count"`
		ErrorCount    int    `json:"error_count"`
		Files         []struct {
			Path    string `json:"path"`
			Changed bool   `json:"changed"`
			Written bool   `json:"written"`
			Error   string `json:"error"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("fmt check JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || !report.OK || report.Mode != "check" || report.Stdin || report.FileCount != 1 || report.ChangedCount != 0 || report.ErrorCount != 0 || len(report.Files) != 1 {
		t.Fatalf("fmt check JSON = %+v, want passing schema v1 check report", report)
	}
	if got := report.Files[0]; got.Path != path || got.Changed || got.Written || got.Error != "" {
		t.Fatalf("fmt check JSON file = %+v, want unchanged file %s", got, path)
	}
}

func TestReleaseMatrixLintJSONReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "warn.leia")
	if err := os.WriteFile(path, []byte("xs := {1, 2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "lint", "--json", path)
	var report struct {
		SchemaVersion   int    `json:"schema_version"`
		OK              bool   `json:"ok"`
		Status          string `json:"status"`
		DiagnosticCount int    `json:"diagnostic_count"`
		ErrorCount      int    `json:"error_count"`
		WarningCount    int    `json:"warning_count"`
		Diagnostics     []struct {
			File     string `json:"file"`
			Code     string `json:"code"`
			Severity string `json:"severity"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("lint JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || !report.OK || report.Status != "pass" || report.DiagnosticCount != 1 || report.ErrorCount != 0 || report.WarningCount != 1 || len(report.Diagnostics) != 1 {
		t.Fatalf("lint JSON = %+v, want passing schema v1 warning report", report)
	}
	if report.Diagnostics[0].File != path || report.Diagnostics[0].Code != "LEIA2001" || report.Diagnostics[0].Severity != "warning" {
		t.Fatalf("lint diagnostic = %+v, want LEIA2001 warning for %s", report.Diagnostics[0], path)
	}
}

func TestReleaseMatrixTestCommandJSONReportsAreMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.leia")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".leia")+".out", []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "test", "--json", dir)
	var runReport struct {
		SchemaVersion int    `json:"schema_version"`
		OK            bool   `json:"ok"`
		Status        string `json:"status"`
		Total         int    `json:"total"`
		Passed        int    `json:"passed"`
		Failed        int    `json:"failed"`
		GoldenMode    string `json:"golden_mode"`
		Files         []struct {
			File string `json:"file"`
			OK   bool   `json:"ok"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(runOut), &runReport); err != nil {
		t.Fatalf("test run JSON failed to decode: %v\n%s", err, runOut)
	}
	if runReport.SchemaVersion != 1 || !runReport.OK || runReport.Status != "pass" || runReport.Total != 1 || runReport.Passed != 1 || runReport.Failed != 0 || runReport.GoldenMode != "auto" || len(runReport.Files) != 1 || runReport.Files[0].File != path || !runReport.Files[0].OK {
		t.Fatalf("test run JSON = %+v, want passing schema v1 report", runReport)
	}

	listOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "test", "--list", "--json", dir)
	var listReport struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		ListOnly      bool     `json:"list_only"`
		GoldenMode    string   `json:"golden_mode"`
		FileCount     int      `json:"file_count"`
		Files         []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(listOut), &listReport); err != nil {
		t.Fatalf("test list JSON failed to decode: %v\n%s", err, listOut)
	}
	if listReport.SchemaVersion != 1 || listReport.Status != "pass" || !listReport.ListOnly || listReport.GoldenMode != "auto" || listReport.FileCount != 1 || len(listReport.Files) != 1 || listReport.Files[0] != path {
		t.Fatalf("test list JSON = %+v, want one-file schema v1 list report", listReport)
	}
}

func TestReleaseMatrixCheckJSONReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.leia")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".leia")+".out", []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "check", "--json", "--quick", dir)
	var report struct {
		SchemaVersion int `json:"schema_version"`
		OK            bool
		StepCount     int `json:"step_count"`
		FailedCount   int `json:"failed_count"`
		SkippedCount  int `json:"skipped_count"`
		Steps         []struct {
			Name     string `json:"name"`
			OK       bool   `json:"ok"`
			ExitCode int    `json:"exit_code"`
			Skipped  bool   `json:"skipped"`
		} `json:"steps"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("check JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || !report.OK || report.StepCount != 7 || report.FailedCount != 0 || report.SkippedCount != 4 || len(report.Steps) != 7 {
		t.Fatalf("check JSON = %+v, want passing quick schema v1 report", report)
	}
	for i, step := range report.Steps {
		wantSkipped := i >= 3
		if step.Skipped != wantSkipped || !step.OK {
			t.Fatalf("check step[%d] = %+v, want skipped=%t and ok", i, step, wantSkipped)
		}
	}
}

func TestReleaseMatrixReleaseNotesReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--json")
	var passReport struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		RequireReady  bool     `json:"require_ready"`
		Version       string   `json:"version"`
		FailureCount  int      `json:"failure_count"`
		Failures      []string `json:"failures"`
	}
	if err := json.Unmarshal([]byte(out), &passReport); err != nil {
		t.Fatalf("release notes JSON failed to decode: %v\n%s", err, out)
	}
	if passReport.SchemaVersion != 1 || passReport.Status != "pass" || passReport.RequireReady || passReport.Version != "" || passReport.FailureCount != 0 || len(passReport.Failures) != 0 {
		t.Fatalf("release notes template JSON = %+v, want passing schema v1 report", passReport)
	}

	missingOut := runCommand(t, root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--json", "--version", "v9.9.9")
	var missingReport struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		RequireReady  bool     `json:"require_ready"`
		Version       string   `json:"version"`
		FailureCount  int      `json:"failure_count"`
		Failures      []string `json:"failures"`
	}
	if err := json.Unmarshal([]byte(missingOut), &missingReport); err != nil {
		t.Fatalf("missing release notes JSON failed to decode: %v\n%s", err, missingOut)
	}
	if missingReport.SchemaVersion != 1 || missingReport.Status != "issues" || missingReport.RequireReady || missingReport.Version != "v9.9.9" || missingReport.FailureCount != len(missingReport.Failures) {
		t.Fatalf("missing release notes JSON = %+v, want issues schema v1 report", missingReport)
	}
	if !stringSliceContains(missingReport.Failures, "missing release notes for v9.9.9: docs/release/notes/v9.9.9.md") {
		t.Fatalf("missing release notes JSON missing actionable failure: %+v", missingReport.Failures)
	}
}

func TestReleaseMatrixReleaseDistributionReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_distribution_check.sh", "--json")
	var report struct {
		SchemaVersion       int      `json:"schema_version"`
		Status              string   `json:"status"`
		RequireGoreleaser   bool     `json:"require_goreleaser"`
		RequireWorkflows    bool     `json:"require_workflows"`
		GoreleaserAvailable bool     `json:"goreleaser_available"`
		LocalInstallFixture string   `json:"local_install_fixture"`
		WorkflowFiles       []string `json:"workflow_files"`
		InstallTargets      []string `json:"install_targets"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("release distribution JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.RequireGoreleaser || report.RequireWorkflows || report.LocalInstallFixture != "verified" {
		t.Fatalf("release distribution JSON = %+v, want passing schema v1 report", report)
	}
	for _, target := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"} {
		if !stringSliceContains(report.InstallTargets, target) {
			t.Fatalf("release distribution JSON missing install target %q: %+v", target, report.InstallTargets)
		}
	}
	for _, workflow := range []string{".github/workflows/release.yml", ".github/workflows/distribution-check.yml", ".github/workflows/pages.yml"} {
		if !stringSliceContains(report.WorkflowFiles, workflow) {
			t.Fatalf("release distribution JSON missing workflow %q: %+v", workflow, report.WorkflowFiles)
		}
	}
}

func TestReleaseMatrixDocsCheckReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 120*time.Second, "bash", "scripts/docs_check.sh", "--json")
	var report struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		FailureCount  int      `json:"failure_count"`
		Failures      []string `json:"failures"`
		Counts        struct {
			MarkdownFiles                     int `json:"markdown_files"`
			RelativeDocumentationLinks        int `json:"relative_documentation_links"`
			RepositoryScriptCodeBlockMentions int `json:"repository_script_code_block_mentions"`
			ReleaseGateDocs                   int `json:"release_gate_docs"`
			ReferenceEntrypoints              int `json:"reference_entrypoints"`
			SpecContractDocs                  int `json:"spec_contract_docs"`
			ExamplesIndexDirectories          int `json:"examples_index_directories"`
			ExamplesCapabilityDriftGates      int `json:"examples_capability_drift_gates"`
			ReadmeUserFacingGates             int `json:"readme_user_facing_gates"`
			GeneratedReferenceDocs            int `json:"generated_reference_docs"`
			GeneratedSpecHTML                 int `json:"generated_spec_html"`
			RunnableSpecExamples              int `json:"runnable_spec_examples"`
		} `json:"counts"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("docs check JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.FailureCount != 0 || len(report.Failures) != 0 {
		t.Fatalf("docs check JSON = %+v, want passing schema v1 report", report)
	}
	if report.Counts.MarkdownFiles == 0 || report.Counts.RelativeDocumentationLinks == 0 || report.Counts.RepositoryScriptCodeBlockMentions == 0 {
		t.Fatalf("docs check JSON missing core documentation counts: %+v", report.Counts)
	}
	if report.Counts.ReleaseGateDocs == 0 || report.Counts.ReferenceEntrypoints == 0 || report.Counts.SpecContractDocs == 0 {
		t.Fatalf("docs check JSON missing release/spec counts: %+v", report.Counts)
	}
	if report.Counts.ExamplesIndexDirectories == 0 || report.Counts.ExamplesCapabilityDriftGates == 0 || report.Counts.ReadmeUserFacingGates == 0 {
		t.Fatalf("docs check JSON missing example/readme counts: %+v", report.Counts)
	}
	if report.Counts.GeneratedReferenceDocs == 0 || report.Counts.GeneratedSpecHTML != 1 || report.Counts.RunnableSpecExamples == 0 {
		t.Fatalf("docs check JSON missing generated documentation counts: %+v", report.Counts)
	}
}

func TestReleaseMatrixReleaseArtifactReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_artifacts_check.sh", "--json", "--version", "v1.2.3-rc.1")
	var report struct {
		SchemaVersion          int    `json:"schema_version"`
		Status                 string `json:"status"`
		Version                string `json:"version"`
		Build                  bool   `json:"build"`
		RequireClean           bool   `json:"require_clean"`
		RequireTag             bool   `json:"require_tag"`
		GOOS                   string `json:"goos"`
		GOARCH                 string `json:"goarch"`
		Artifact               string `json:"artifact"`
		LSPArtifact            string `json:"lsp_artifact"`
		Metadata               string `json:"metadata"`
		InstallArchive         string `json:"install_archive"`
		DryRunVerified         bool   `json:"dry_run_verified"`
		BuildVerified          bool   `json:"build_verified"`
		InstallArchiveVerified bool   `json:"install_archive_verified"`
		OutputDir              string `json:"output_dir"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("release artifact JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.Version != "v1.2.3-rc.1" || report.Build || report.RequireClean || report.RequireTag || !report.DryRunVerified || report.BuildVerified || report.InstallArchiveVerified || report.OutputDir != "" {
		t.Fatalf("release artifact JSON = %+v, want passing dry-run schema v1 report", report)
	}
	if report.GOOS == "" || report.GOARCH == "" {
		t.Fatalf("release artifact JSON missing platform: %+v", report)
	}
	for _, want := range []string{"leia_v1.2.3-rc.1_", "leia-lsp_v1.2.3-rc.1_", "metadata.txt", "leia_v1.2.3-rc.1_"} {
		if !strings.Contains(report.Artifact+" "+report.LSPArtifact+" "+report.Metadata+" "+report.InstallArchive, want) {
			t.Fatalf("release artifact JSON missing artifact fragment %q: %+v", want, report)
		}
	}
}

func TestReleaseMatrixReleaseArtifactPlanIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_artifacts.sh", "--dry-run", "--version", "v1.2.3-rc.1", "--json")
	var report struct {
		SchemaVersion   int    `json:"schema_version"`
		Status          string `json:"status"`
		DryRun          bool   `json:"dry_run"`
		OutputDir       string `json:"output_dir"`
		Version         string `json:"version"`
		Module          string `json:"module"`
		GOOS            string `json:"goos"`
		GOARCH          string `json:"goarch"`
		Artifact        string `json:"artifact"`
		LSPArtifact     string `json:"lsp_artifact"`
		Metadata        string `json:"metadata"`
		Checksums       string `json:"checksums"`
		ArtifactPath    string `json:"artifact_path"`
		LSPArtifactPath string `json:"lsp_artifact_path"`
		MetadataPath    string `json:"metadata_path"`
		ChecksumsPath   string `json:"checksums_path"`
		GitCommit       string `json:"git_commit"`
		GitShortCommit  string `json:"git_short_commit"`
		GitBranch       string `json:"git_branch"`
		GitDirty        bool   `json:"git_dirty"`
		GoVersion       string `json:"go_version"`
		BuildTimeUTC    string `json:"build_time_utc"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("release artifact plan JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || !report.DryRun || report.Version != "v1.2.3-rc.1" || report.Module != "github.com/never-labs/leia" {
		t.Fatalf("release artifact plan JSON = %+v, want passing dry-run schema v1 report", report)
	}
	if report.OutputDir == "" || report.GOOS == "" || report.GOARCH == "" || report.GitCommit == "" || report.GitShortCommit == "" || report.GoVersion == "" || report.BuildTimeUTC == "" {
		t.Fatalf("release artifact plan JSON missing environment metadata: %+v", report)
	}
	for _, want := range []string{"leia_v1.2.3-rc.1_", "leia-lsp_v1.2.3-rc.1_", "metadata.txt", "SHA256SUMS"} {
		if !strings.Contains(report.Artifact+" "+report.LSPArtifact+" "+report.Metadata+" "+report.Checksums, want) {
			t.Fatalf("release artifact plan JSON missing artifact fragment %q: %+v", want, report)
		}
	}
	for _, path := range []string{report.ArtifactPath, report.LSPArtifactPath, report.MetadataPath, report.ChecksumsPath} {
		if path == "" || !strings.HasPrefix(path, report.OutputDir) {
			t.Fatalf("release artifact plan path %q must be under output dir %q: %+v", path, report.OutputDir, report)
		}
	}
}

func TestReleaseMatrixProductionPlanReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	quickOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--quick", "--list", "--json")
	var quickReport struct {
		SchemaVersion     int `json:"schema_version"`
		Mode              string
		ReleaseProfile    bool `json:"release_profile"`
		ListOnly          bool `json:"list_only"`
		RunCount          int  `json:"run_count"`
		SkipCount         int  `json:"skip_count"`
		CriticalSkipCount int  `json:"critical_skip_count"`
		RunnableChecks    []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		} `json:"runnable_checks"`
		SkippedChecks        []string `json:"skipped_checks"`
		ReleaseCriticalSkips []string `json:"release_critical_skips"`
	}
	if err := json.Unmarshal([]byte(quickOut), &quickReport); err != nil {
		t.Fatalf("quick production plan JSON failed to decode: %v\n%s", err, quickOut)
	}
	if quickReport.SchemaVersion != 1 || quickReport.Mode != "quick" || quickReport.ReleaseProfile || !quickReport.ListOnly || quickReport.RunCount != len(quickReport.RunnableChecks) || quickReport.SkipCount != len(quickReport.SkippedChecks) || quickReport.CriticalSkipCount != len(quickReport.ReleaseCriticalSkips) {
		t.Fatalf("quick production plan JSON = %+v, want quick schema v1 plan", quickReport)
	}

	out := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--release-profile", "--release-version", "vX.Y.Z", "--list", "--json")
	var report struct {
		SchemaVersion     int `json:"schema_version"`
		Mode              string
		ReleaseProfile    bool   `json:"release_profile"`
		ReleaseVersion    string `json:"release_version"`
		ListOnly          bool   `json:"list_only"`
		RunCount          int    `json:"run_count"`
		SkipCount         int    `json:"skip_count"`
		CriticalSkipCount int    `json:"critical_skip_count"`
		RunnableChecks    []struct {
			Name    string `json:"name"`
			Command string `json:"command"`
		} `json:"runnable_checks"`
		SkippedChecks        []string `json:"skipped_checks"`
		ReleaseCriticalSkips []string `json:"release_critical_skips"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("production plan JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Mode != "full" || !report.ReleaseProfile || report.ReleaseVersion != "vX.Y.Z" || !report.ListOnly || report.RunCount != len(report.RunnableChecks) || report.SkipCount != len(report.SkippedChecks) || report.CriticalSkipCount != len(report.ReleaseCriticalSkips) {
		t.Fatalf("production plan JSON = %+v, want release profile schema v1 plan", report)
	}
	commands := map[string]string{}
	for _, check := range report.RunnableChecks {
		commands[check.Name] = check.Command
	}
	for name, want := range map[string]string{
		"Public Release Blockers": "bash scripts/public_release_blockers_check.sh --require-resolved",
		"Release Distribution":    "bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows",
		"Release Notes":           "bash scripts/release_notes_check.sh --require-ready --version \"vX.Y.Z\"",
		"Release Artifacts":       "bash scripts/release_artifacts_check.sh",
		"Q Performance Gate":      "python3 benchmarks/q_perf_report.py --from-output \"$q_perf_dir/output.txt\" --check --json \"$q_perf_dir/q_perf_report.json\" --markdown \"$q_perf_dir/q_perf_report.md\"",
	} {
		if !strings.Contains(commands[name], want) {
			t.Fatalf("production plan JSON command %q = %q, want fragment %q", name, commands[name], want)
		}
	}
}

func TestReleaseMatrixCIPlanReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "release", "--release-version", "vX.Y.Z", "--list", "--json")
	var report struct {
		SchemaVersion  int    `json:"schema_version"`
		Profile        string `json:"profile"`
		ListOnly       bool   `json:"list_only"`
		NoLuaJIT       bool   `json:"no_luajit"`
		ReleaseVersion string `json:"release_version"`
		CommandCount   int    `json:"command_count"`
		Commands       []struct {
			Name    string   `json:"name"`
			Args    []string `json:"args"`
			Command string   `json:"command"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("ci profile plan JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Profile != "release" || !report.ListOnly || report.NoLuaJIT || report.ReleaseVersion != "vX.Y.Z" || report.CommandCount != 1 || len(report.Commands) != 1 {
		t.Fatalf("ci profile plan JSON = %+v, want release schema v1 plan", report)
	}
	command := report.Commands[0]
	if command.Name != "Production check" || command.Command != "bash scripts/production_check.sh --full --release-profile --release-version vX.Y.Z" {
		t.Fatalf("ci profile plan command = %+v, want versioned production check", command)
	}
	for _, want := range []string{"bash", "scripts/production_check.sh", "--full", "--release-profile", "--release-version", "vX.Y.Z"} {
		if !stringSliceContains(command.Args, want) {
			t.Fatalf("ci profile plan args = %#v, want %q", command.Args, want)
		}
	}
}

func TestReleaseMatrixQConformanceReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 60*time.Second, "bash", "scripts/q_conformance_gate.sh", "--scope", "core", "--bench", "none", "--json")
	var report struct {
		SchemaVersion     int      `json:"schema_version"`
		Status            string   `json:"status"`
		Scope             string   `json:"scope"`
		BenchMode         string   `json:"bench_mode"`
		Jobs              int      `json:"jobs"`
		TimeoutSeconds    int      `json:"timeout_seconds"`
		LanguageCaseCount int      `json:"language_case_count"`
		ExampleCaseCount  int      `json:"example_case_count"`
		BenchmarkCount    int      `json:"benchmark_case_count"`
		BenchmarkJSON     string   `json:"benchmark_json"`
		BenchmarkMarkdown string   `json:"benchmark_markdown"`
		LanguageCases     []string `json:"language_cases"`
		ExampleCases      []string `json:"example_cases"`
		BenchmarkCases    []string `json:"benchmark_cases"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("q conformance JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.Scope != "core" || report.BenchMode != "none" {
		t.Fatalf("q conformance JSON = %+v, want passing core/no-bench schema v1 report", report)
	}
	if report.Jobs <= 0 || report.TimeoutSeconds <= 0 {
		t.Fatalf("q conformance JSON missing execution parameters: %+v", report)
	}
	if report.LanguageCaseCount != len(report.LanguageCases) || report.ExampleCaseCount != len(report.ExampleCases) || report.BenchmarkCount != len(report.BenchmarkCases) {
		t.Fatalf("q conformance JSON counts do not match arrays: %+v", report)
	}
	if report.LanguageCaseCount == 0 || report.ExampleCaseCount == 0 || report.BenchmarkCount == 0 {
		t.Fatalf("q conformance JSON should report q tests, examples, and registered benchmarks: %+v", report)
	}
	if report.BenchmarkJSON != "" || report.BenchmarkMarkdown != "" {
		t.Fatalf("q conformance no-bench JSON should not report benchmark artifacts: %+v", report)
	}
}

func TestReleaseMatrixEditorAssetReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/editor_check.sh", "--json")
	var report struct {
		SchemaVersion     int      `json:"schema_version"`
		Status            string   `json:"status"`
		RequireTreeSitter bool     `json:"require_tree_sitter"`
		TreeSitterStatus  string   `json:"tree_sitter_status"`
		TreeSitterCommand string   `json:"tree_sitter_command"`
		EmacsStatus       string   `json:"emacs_status"`
		EmacsCommand      string   `json:"emacs_command"`
		TextMateGrammars  []string `json:"textmate_grammars"`
		VSCodeAssets      []string `json:"vscode_assets"`
		TreeSitterAssets  []string `json:"tree_sitter_assets"`
		SmokeTests        []string `json:"smoke_tests"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("editor asset JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.RequireTreeSitter {
		t.Fatalf("editor asset JSON = %+v, want passing schema v1 non-strict report", report)
	}
	for _, status := range []string{report.TreeSitterStatus, report.EmacsStatus} {
		if status != "verified" && status != "skipped" {
			t.Fatalf("editor asset JSON has unexpected optional tool status %q: %+v", status, report)
		}
	}
	for _, want := range []string{"tools/syntax/textmate/leia.tmLanguage.json", "tools/syntax/textmate/leia-mod.tmLanguage.json"} {
		if !stringSliceContains(report.TextMateGrammars, want) {
			t.Fatalf("editor asset JSON missing TextMate grammar %q: %+v", want, report.TextMateGrammars)
		}
	}
	for _, want := range []string{"editors/vscode/package.json", "editors/vscode/syntaxes/leia.tmLanguage.json"} {
		if !stringSliceContains(report.VSCodeAssets, want) {
			t.Fatalf("editor asset JSON missing VS Code asset %q: %+v", want, report.VSCodeAssets)
		}
	}
	for _, want := range []string{"tools/tree-sitter-leia/grammar.js", "tools/tree-sitter-leia/src/grammar.json"} {
		if !stringSliceContains(report.TreeSitterAssets, want) {
			t.Fatalf("editor asset JSON missing tree-sitter asset %q: %+v", want, report.TreeSitterAssets)
		}
	}
	if !stringSliceContains(report.SmokeTests, "tools/editor/smoke/editor_smoke.py") {
		t.Fatalf("editor asset JSON missing smoke test: %+v", report.SmokeTests)
	}
}

func TestReleaseMatrixDiagnosticsBundleReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "diag")
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "diag", "bundle", "--output", outDir, "--skip-go-tests", "--skip-benchmarks", "--json")
	var report struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		OutputDir     string   `json:"output_dir"`
		RunGoTests    bool     `json:"run_go_tests"`
		RunBenchmarks bool     `json:"run_benchmarks"`
		FailureCount  int      `json:"failure_count"`
		Summary       string   `json:"summary"`
		Manifest      string   `json:"manifest"`
		Files         []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("diagnostics bundle JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.OutputDir != outDir || report.RunGoTests || report.RunBenchmarks || report.FailureCount != 0 {
		t.Fatalf("diagnostics bundle JSON = %+v, want passing no-test/no-benchmark schema v1 report", report)
	}
	for _, want := range []string{"summary.md", "manifest.txt", "git_revision.txt", "git_status.txt", "git_diff_stat.txt", "go_version.txt", "go_env_summary.txt", "go_test_quick.skipped", "benchmarks.skipped"} {
		if !stringSliceContains(report.Files, want) {
			t.Fatalf("diagnostics bundle JSON missing file %q: %+v", want, report.Files)
		}
		if _, err := os.Stat(filepath.Join(outDir, want)); err != nil {
			t.Fatalf("diagnostics bundle file %s missing on disk: %v", want, err)
		}
	}
	if report.Summary != "summary.md" || report.Manifest != "manifest.txt" {
		t.Fatalf("diagnostics bundle JSON should report relative summary/manifest paths: %+v", report)
	}
}

func TestReleaseMatrixWorktreeAuditReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 60*time.Second, "bash", "scripts/worktree_audit.sh", "--json")
	var report struct {
		SchemaVersion  int    `json:"schema_version"`
		Status         string `json:"status"`
		FailOnFindings bool   `json:"fail_on_findings"`
		FindingCount   int    `json:"finding_count"`
		Findings       []struct {
			Status string `json:"status"`
			Path   string `json:"path"`
			Branch string `json:"branch"`
			Detail string `json:"detail"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("worktree audit JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.FailOnFindings || report.FindingCount != len(report.Findings) {
		t.Fatalf("worktree audit JSON = %+v, want schema v1 report with matching count", report)
	}
	if report.Status != "pass" && report.Status != "findings" {
		t.Fatalf("worktree audit JSON status = %q, want pass or findings", report.Status)
	}
	if report.Status == "pass" && report.FindingCount != 0 {
		t.Fatalf("worktree audit pass report must have no findings: %+v", report)
	}
	if report.Status == "findings" && report.FindingCount == 0 {
		t.Fatalf("worktree audit findings report must include findings: %+v", report)
	}
	for _, finding := range report.Findings {
		if finding.Status == "" || finding.Path == "" || finding.Branch == "" || finding.Detail == "" {
			t.Fatalf("worktree audit finding must include status/path/branch/detail: %+v", finding)
		}
	}
}

func TestReleaseMatrixGoreleaserTargetsMatchInstallDryRun(t *testing.T) {
	root := findRepoRoot(t)
	targets := readGoreleaserTargets(t, filepath.Join(root, ".goreleaser.yaml"))
	for _, target := range targets {
		parts := strings.Split(target, "/")
		if len(parts) != 2 {
			t.Fatalf("bad target %q", target)
		}
		out := runCommand(t, root, 30*time.Second, "bash", "scripts/install.sh", "--dry-run", "--version", "v0.0.0", "--os", parts[0], "--arch", parts[1], "--bin-dir", "/tmp/leia-bin")
		ext := "tar.gz"
		installPath := "/tmp/leia-bin/leia"
		lspInstallPath := "/tmp/leia-bin/leia-lsp"
		if parts[0] == "windows" {
			ext = "zip"
			installPath += ".exe"
			lspInstallPath += ".exe"
		}
		for _, want := range []string{
			"asset=leia_v0.0.0_" + parts[0] + "_" + parts[1] + "." + ext,
			"install_path=" + installPath,
			"lsp_install_path=" + lspInstallPath,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("install dry-run for %s missing %q:\n%s", target, want, out)
			}
		}
	}
}

func TestReleaseMatrixInstallDryRunReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	for _, tc := range []struct {
		goos           string
		goarch         string
		archiveExt     string
		binary         string
		lspBinary      string
		installPath    string
		lspInstallPath string
	}{
		{
			goos:           "linux",
			goarch:         "amd64",
			archiveExt:     "tar.gz",
			binary:         "leia",
			lspBinary:      "leia-lsp",
			installPath:    "/tmp/leia-bin/leia",
			lspInstallPath: "/tmp/leia-bin/leia-lsp",
		},
		{
			goos:           "windows",
			goarch:         "arm64",
			archiveExt:     "zip",
			binary:         "leia.exe",
			lspBinary:      "leia-lsp.exe",
			installPath:    "/tmp/leia-bin/leia.exe",
			lspInstallPath: "/tmp/leia-bin/leia-lsp.exe",
		},
	} {
		out := runCommand(t, root, 30*time.Second, "bash", "scripts/install.sh", "--dry-run", "--version", "v1.2.3-rc.1", "--os", tc.goos, "--arch", tc.goarch, "--bin-dir", "/tmp/leia-bin", "--json")
		var report struct {
			SchemaVersion  int    `json:"schema_version"`
			Status         string `json:"status"`
			DryRun         bool   `json:"dry_run"`
			Verify         bool   `json:"verify"`
			Repo           string `json:"repo"`
			Version        string `json:"version"`
			GOOS           string `json:"goos"`
			GOARCH         string `json:"goarch"`
			ArchiveExt     string `json:"archive_ext"`
			Asset          string `json:"asset"`
			URL            string `json:"url"`
			Checksums      string `json:"checksums"`
			BinDir         string `json:"bin_dir"`
			Binary         string `json:"binary"`
			LSPBinary      string `json:"lsp_binary"`
			InstallPath    string `json:"install_path"`
			LSPInstallPath string `json:"lsp_install_path"`
		}
		if err := json.Unmarshal([]byte(out), &report); err != nil {
			t.Fatalf("install dry-run JSON failed to decode for %s/%s: %v\n%s", tc.goos, tc.goarch, err, out)
		}
		if report.SchemaVersion != 1 || report.Status != "pass" || !report.DryRun || !report.Verify || report.Repo != "never-labs/leia" || report.Version != "v1.2.3-rc.1" {
			t.Fatalf("install dry-run JSON = %+v, want passing verified dry-run schema v1 report", report)
		}
		if report.GOOS != tc.goos || report.GOARCH != tc.goarch || report.ArchiveExt != tc.archiveExt || report.Binary != tc.binary || report.LSPBinary != tc.lspBinary {
			t.Fatalf("install dry-run JSON has wrong platform fields for %s/%s: %+v", tc.goos, tc.goarch, report)
		}
		if report.InstallPath != tc.installPath || report.LSPInstallPath != tc.lspInstallPath || report.BinDir != "/tmp/leia-bin" {
			t.Fatalf("install dry-run JSON has wrong install paths for %s/%s: %+v", tc.goos, tc.goarch, report)
		}
		wantAsset := "leia_v1.2.3-rc.1_" + tc.goos + "_" + tc.goarch + "." + tc.archiveExt
		if report.Asset != wantAsset || !strings.Contains(report.URL, "/"+wantAsset) || !strings.HasSuffix(report.Checksums, "/SHA256SUMS") {
			t.Fatalf("install dry-run JSON has wrong release URLs for %s/%s: %+v", tc.goos, tc.goarch, report)
		}
	}
}

func TestReleaseMatrixArtifactVersionValidationIsExplicit(t *testing.T) {
	root := findRepoRoot(t)
	notesResult := runCommandResult(root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--version", "bad version")
	if notesResult.err == nil {
		t.Fatalf("release_notes_check.sh bad version unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", notesResult.stdout, notesResult.stderr)
	}
	if !strings.Contains(notesResult.stderr, "release notes version must match vMAJOR.MINOR.PATCH") {
		t.Fatalf("release_notes_check.sh bad version stderr = %q", notesResult.stderr)
	}

	checkResult := runCommandResult(root, 30*time.Second, "bash", "scripts/release_artifacts_check.sh", "--version", "bad version")
	if checkResult.err == nil {
		t.Fatalf("release_artifacts_check.sh bad version unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", checkResult.stdout, checkResult.stderr)
	}
	if !strings.Contains(checkResult.stderr, "release artifact check version must match vMAJOR.MINOR.PATCH") {
		t.Fatalf("release_artifacts_check.sh bad version stderr = %q", checkResult.stderr)
	}

	artifactResult := runCommandResult(root, 30*time.Second, "bash", "scripts/release_artifacts.sh", "--dry-run", "--version", "bad version")
	if artifactResult.err == nil {
		t.Fatalf("release_artifacts.sh bad version unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", artifactResult.stdout, artifactResult.stderr)
	}
	if !strings.Contains(artifactResult.stderr, "release artifact version must match vMAJOR.MINOR.PATCH") {
		t.Fatalf("release_artifacts.sh bad version stderr = %q", artifactResult.stderr)
	}

	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_artifacts_check.sh", "--version", "v1.2.3-rc.1")
	if !strings.Contains(out, "release_artifacts_check.sh: pass") {
		t.Fatalf("release_artifacts_check.sh prerelease output = %q, want pass", out)
	}
}

func TestReleaseMatrixInstallInputValidationIsExplicit(t *testing.T) {
	root := findRepoRoot(t)
	badVersion := runCommandResult(root, 30*time.Second, "bash", "scripts/install.sh", "--dry-run", "--version", "bad version", "--os", "linux", "--arch", "amd64", "--bin-dir", "/tmp/leia-bin")
	if badVersion.err == nil {
		t.Fatalf("install.sh bad version unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", badVersion.stdout, badVersion.stderr)
	}
	if !strings.Contains(badVersion.stderr, "--version must match vMAJOR.MINOR.PATCH") {
		t.Fatalf("install.sh bad version stderr = %q", badVersion.stderr)
	}

	badRepo := runCommandResult(root, 30*time.Second, "bash", "scripts/install.sh", "--dry-run", "--version", "v1.2.3", "--repo", "badrepo", "--os", "linux", "--arch", "amd64", "--bin-dir", "/tmp/leia-bin")
	if badRepo.err == nil {
		t.Fatalf("install.sh bad repo unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", badRepo.stdout, badRepo.stderr)
	}
	if !strings.Contains(badRepo.stderr, "--repo must be OWNER/REPO") {
		t.Fatalf("install.sh bad repo stderr = %q", badRepo.stderr)
	}

	out := runCommand(t, root, 30*time.Second, "bash", "scripts/install.sh", "--dry-run", "--version", "v1.2.3-rc.1", "--repo", "never-labs/leia", "--os", "linux", "--arch", "amd64", "--bin-dir", "/tmp/leia-bin")
	if !strings.Contains(out, "asset=leia_v1.2.3-rc.1_linux_amd64.tar.gz") {
		t.Fatalf("install.sh prerelease dry-run output = %q", out)
	}
}

func TestReleaseMatrixDocsIndexCoversReferenceEntrypoints(t *testing.T) {
	root := findRepoRoot(t)
	docsIndex := readFileString(t, filepath.Join(root, "docs", "index.md"))
	entries, err := filepath.Glob(filepath.Join(root, "docs", "reference", "*", "index.md"))
	if err != nil {
		t.Fatalf("glob reference entrypoints: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("docs/reference must contain reference entrypoints")
	}
	for _, entry := range entries {
		ref, err := filepath.Rel(filepath.Join(root, "docs"), entry)
		if err != nil {
			t.Fatalf("relative reference path for %s: %v", entry, err)
		}
		ref = filepath.ToSlash(ref)
		if !strings.Contains(docsIndex, "("+ref+")") {
			t.Fatalf("docs/index.md must link reference entrypoint %s", ref)
		}
	}
}

func TestReleaseMatrixFeatureDocsStayCoveredBySpecAndReference(t *testing.T) {
	root := findRepoRoot(t)
	matrix := loadReleaseFeatureMatrix(t, root)
	docsIndex := readFileString(t, filepath.Join(root, "docs", "index.md"))
	specIndex := readFileString(t, filepath.Join(root, "docs", "spec", "index.md"))
	coverage := releaseFeatureDocCoverageMap()

	seenFeatures := map[string]bool{}
	for i, feature := range matrix.Features {
		featureID := decodeRequiredString(t, feature, i, "id")
		seenFeatures[featureID] = true
		item, ok := coverage[featureID]
		if !ok {
			t.Fatalf("feature_matrix.json feature %s needs releaseFeatureDocCoverageMap entry so docs/spec and reference gates cannot miss it", featureID)
		}
		if len(item.specSections) == 0 {
			t.Fatalf("releaseFeatureDocCoverageMap entry %s must name at least one spec section", featureID)
		}
		if len(item.docPaths) == 0 {
			t.Fatalf("releaseFeatureDocCoverageMap entry %s must name at least one documentation entrypoint", featureID)
		}
		requireFeatureSpecSections(t, feature, featureID, item.specSections...)
		for _, section := range item.specSections {
			if !strings.Contains(specIndex, section) {
				t.Fatalf("docs/spec/index.md must expose %s section %q", featureID, section)
			}
		}
		for _, ref := range item.docPaths {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ref))); err != nil {
				t.Fatalf("%s documentation entrypoint %s is missing: %v", featureID, ref, err)
			}
			docsIndexRef := strings.TrimPrefix(ref, "docs/")
			if !strings.Contains(docsIndex, "("+docsIndexRef+")") {
				t.Fatalf("docs/index.md must link %s documented for %s", docsIndexRef, featureID)
			}
		}
	}
	for featureID := range coverage {
		if !seenFeatures[featureID] {
			t.Fatalf("releaseFeatureDocCoverageMap has stale feature entry %s", featureID)
		}
	}
}

type releaseFeatureDocCoverage struct {
	specSections []string
	docPaths     []string
}

func releaseFeatureDocCoverageMap() map[string]releaseFeatureDocCoverage {
	spec := []string{"docs/spec/index.md"}
	return map[string]releaseFeatureDocCoverage{
		"literals_and_constants": {
			specSections: []string{"Lexical Elements", "Values And Types"},
			docPaths:     spec,
		},
		"numeric_arithmetic": {
			specSections: []string{"Expressions", "Values And Types"},
			docPaths:     spec,
		},
		"comparison_boolean_control": {
			specSections: []string{"Expressions", "Statements"},
			docPaths:     spec,
		},
		"loops_numeric_for": {
			specSections: []string{"Statements", "Grammar Appendix"},
			docPaths:     spec,
		},
		"generic_for_pairs_next": {
			specSections: []string{"Statements", "Tables And Metatables"},
			docPaths:     spec,
		},
		"functions_calls_returns": {
			specSections: []string{"Functions", "Statements"},
			docPaths:     spec,
		},
		"varargs_multireturn": {
			specSections: []string{"Functions", "Tables And Metatables"},
			docPaths:     spec,
		},
		"closures_upvalues": {
			specSections: []string{"Functions", "Declarations And Scope"},
			docPaths:     spec,
		},
		"tables_arrays_fields": {
			specSections: []string{"Tables And Metatables", "Values And Types"},
			docPaths:     spec,
		},
		"metatables_metamethods": {
			specSections: []string{"Tables And Metatables", "Implementation Requirements"},
			docPaths:     spec,
		},
		"strings_patterns_concat": {
			specSections: []string{"Lexical Elements", "Expressions"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/stdlib/index.md"},
		},
		"errors_pcall_xpcall_defer": {
			specSections: []string{"Errors And Diagnostics", "Statements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/diagnostics/index.md"},
		},
		"coroutines": {
			specSections: []string{"Concurrency", "Functions"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/concurrency/index.md"},
		},
		"go_style_concurrency": {
			specSections: []string{"Concurrency", "Statements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/concurrency/index.md"},
		},
		"bitwise_bit32": {
			specSections: []string{"Expressions", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/stdlib/index.md"},
		},
		"table_sort_stdlib": {
			specSections: []string{"Tables And Metatables", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/stdlib/index.md"},
		},
		"utf8": {
			specSections: []string{"Values And Types", "Expressions"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/stdlib/index.md"},
		},
		"host_stdlibs": {
			specSections: []string{"Modules And Loading", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/stdlib/index.md", "docs/reference/security/index.md"},
		},
		"tagged_dialect_syntax": {
			specSections: []string{"Grammar Appendix", "Tagged Dialects", "Expressions", "Statements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/dialects/index.md"},
		},
		"q_analytics_dialect": {
			specSections: []string{"Grammar Appendix", "Tagged Dialects", "q Dialect", "Expressions", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/data-oriented/index.md", "docs/reference/dialects/index.md"},
		},
		"data_stdlib_qsql": {
			specSections: []string{"Expressions", "q Dialect", "Values And Types", "Tables And Metatables", "Implementation Requirements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/data-oriented/index.md", "docs/reference/stdlib/index.md"},
		},
		"spreadsheet_dialects": {
			specSections: []string{"Expressions", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/data-oriented/index.md", "docs/reference/dialects/index.md"},
		},
		"ai_dialect_integration": {
			specSections: []string{"AI Dialect Syntax", "Expressions", "Statements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/ai/index.md", "docs/reference/evaluate/index.md"},
		},
		"module_package_management": {
			specSections: []string{"Modules And Loading", "Declarations And Scope"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/modules/index.md", "docs/guides/packages.md"},
		},
		"module_download_vendor_cache": {
			specSections: []string{"Modules And Loading", "Stability Contract"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/modules/index.md", "docs/guides/packages.md"},
		},
		"cli_repository_tooling": {
			specSections: []string{"Stability Contract", "Implementation Requirements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/cli/index.md", "docs/guides/tooling.md"},
		},
		"editor_lsp_tooling": {
			specSections: []string{"Stability Contract", "Implementation Requirements", "Lexical Elements"},
			docPaths:     []string{"docs/spec/index.md", "docs/guides/editors.md"},
		},
		"bytecode_vm_execution": {
			specSections: []string{"Implementation Requirements", "Stability Contract"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/platforms/index.md"},
		},
		"arm64_jit_runtime_fallback": {
			specSections: []string{"Implementation Requirements", "Stability Contract"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/performance/index.md", "docs/reference/platforms/index.md"},
		},
		"release_evidence_gates": {
			specSections: []string{"Implementation Requirements", "Stability Contract"},
			docPaths:     []string{"docs/spec/index.md", "docs/testing.md", "docs/release/index.md", "docs/release/decisions.md"},
		},
		"release_distribution_surface": {
			specSections: []string{"Implementation Requirements", "Stability Contract"},
			docPaths:     []string{"docs/spec/index.md", "docs/release/index.md", "docs/release/decisions.md", "docs/reference/platforms/index.md"},
		},
		"embedding_host_bindings": {
			specSections: []string{"Modules And Loading", "Values And Types", "Functions", "Errors And Diagnostics"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/embedding/index.md"},
		},
		"sandbox_capabilities_module_loading": {
			specSections: []string{"Modules And Loading", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/security/index.md", "docs/reference/embedding/index.md"},
		},
		"embedding_resource_budgets": {
			specSections: []string{"Implementation Requirements", "Concurrency"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/embedding/index.md", "docs/reference/security/index.md"},
		},
		"embedding_hot_reload": {
			specSections: []string{"Modules And Loading", "Implementation Requirements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/hot-reload/index.md", "docs/reference/embedding/index.md"},
		},
		"matrix_dense_arrays": {
			specSections: []string{"Tables And Metatables", "Values And Types", "Implementation Requirements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/data-oriented/index.md", "docs/reference/scientific/index.md"},
		},
		"classes_methods_oop": {
			specSections: []string{"Grammar Appendix", "Declarations And Scope", "Expressions", "Statements"},
			docPaths:     spec,
		},
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
	if !hasLicense && strings.Contains(readme, unlicensedNotice) {
		t.Fatal("README.md must not expose repository-internal release blockers; scripts/public_release_blockers_check.sh owns the missing-license gate")
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
		"WithSourceName",
		"SecuritySandbox",
		"WithSandbox",
		"WithSecurity",
		"WithLibs",
		"WithCapabilities",
		"WithModuleLoading",
		"WithFilesystem",
		"WithFilesystemRead",
		"WithFilesystemWrite",
		"WithEnvironment",
		"WithEnvironmentRead",
		"WithEnvironmentWrite",
		"WithEnvironmentAllowlist",
		"WithDynamicEval",
		"WithNetworkAccess",
		"WithProcessExecution",
		"WithProcessShell",
		"WithDebugAccess",
		"WithTestkitAccess",
		"WithArgs",
		"WithPrint",
		"WithModuleMode",
		"ModuleOptionsForScript",
		"ModuleOptionsForScriptMode",
		"ValidModuleMode",
		"WithLLMProvider",
		"NewHotLoader",
		"WithHotLoaderVMOptions",
		"ModuleFrom",
		"WithModuleExactNames",
		"WithModuleNameMapper",
		"WithGoImports",
		"WithDialect",
		"MustDecode",
		"LibSafe",
		"LibApp",
		"LibGame",
		"CapSafe",
		"ModuleModeVendor",
		"SecurityPolicy",
		"DialectHandler",
		"DialectOptions",
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
		"ExecFile",
		"ExecFileContext",
		"Run",
		"RunContext",
		"Call",
		"CallContext",
		"CallValue",
		"CallValueContext",
		"CallPublicValue",
		"Set",
		"Get",
		"SetPublicValue",
		"GetPublicValue",
		"RegisterFunc",
		"RegisterTable",
		"RegisterModule",
		"RegisterModuleFrom",
		"BindStruct",
		"BindStructWithConstructor",
		"BindMethod",
		"Reset",
		"SetArgs",
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

func TestReleaseMatrixAIDialectDocsUsePublicLLMSurface(t *testing.T) {
	root := findRepoRoot(t)
	docs := strings.Join([]string{
		readFileString(t, filepath.Join(root, "docs", "reference", "ai", "index.md")),
		readFileString(t, filepath.Join(root, "docs", "guides", "ai-dialect.md")),
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
			t.Fatalf("AI dialect docs must mention host API %s", api)
		}
		if !strings.Contains(rootGoDoc, api) {
			t.Fatalf("AI dialect docs mention %s but root go doc does not expose it", api)
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
			t.Fatalf("AI dialect docs must mention llm API/semantic term %s", api)
		}
		if !strings.Contains(llmGoDoc, api) {
			t.Fatalf("AI dialect docs mention %s but llm go doc does not expose it", api)
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

func TestReleaseMatrixAIDialectExamplesStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	guide := readFileString(t, filepath.Join(root, "docs", "guides", "ai-dialect.md"))
	if !strings.Contains(guide, "Live-provider examples") || !strings.Contains(guide, "examples/llm/glm_smoke.leia") {
		t.Fatal("AI dialect guide must keep live provider examples separated from offline examples")
	}

	for _, example := range []string{
		"examples/llm/agent.leia",
		"examples/llm/agent_as_tool.leia",
		"examples/llm/incident_response.leia",
	} {
		runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "run", example)
	}
}

func TestReleaseMatrixDocumentedSmokeCommandsStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	for _, path := range []string{
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

func TestReleaseMatrixReadmeUserFacingSnippetsHaveFocusedGate(t *testing.T) {
	root := findRepoRoot(t)
	surfaceSnippet := readReleaseReadmeSurfaceLeiaSnippet(t, root)
	for _, snippet := range []string{
		"a := [1,2,3,4,5,6,7,8,6]",
		"x := q`sum ${a}`",
		"answer, err := turn {",
		"prompt { role:",
		"print(x)",
	} {
		if !strings.Contains(surfaceSnippet, snippet) {
			t.Fatalf("README.md Example snippet changed or lost product surface %q:\n%s", snippet, surfaceSnippet)
		}
	}

	focusedGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_readme_tooling_test.go"))
	for _, snippet := range []string{
		"TestReadmeIntroStaysFocused",
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
		"q`sum ${a}`",
		"turn {",
	} {
		if !strings.Contains(focusedGate, snippet) {
			t.Fatalf("cmd/leia/main_readme_tooling_test.go must keep README focused positioning gate snippet %q", snippet)
		}
	}
	for _, snippet := range []string{
		"TestReadmeMainLeiaExampleStaysRunnableToProviderBoundary",
		"readmeFirstLeiaSnippet",
		"README Leia example failed",
		"README Leia example stdout",
		`exec.Command("go", "run", "./cmd/leia", "run", file)`,
		"want 42 fallback without host LLM provider",
	} {
		if !strings.Contains(focusedGate, snippet) {
			t.Fatalf("cmd/leia/main_readme_tooling_test.go must keep README Surface focused gate snippet %q", snippet)
		}
	}
}

func TestReleaseMatrixToolingAuditCommandsStayRunnable(t *testing.T) {
	root := findRepoRoot(t)
	commands := releaseToolingAuditCommands()
	if len(commands) == 0 {
		t.Fatal("tooling audit commands must not be empty")
	}

	diagDir := filepath.Join(os.TempDir(), "leia-diag")
	_ = os.RemoveAll(diagDir)
	t.Cleanup(func() {
		_ = os.RemoveAll(diagDir)
	})

	for _, command := range commands {
		fields, err := dialect.Shellwords(command)
		if err != nil {
			t.Fatalf("tooling audit command is not valid shellwords %q: %v", command, err)
		}
		if len(fields) < 4 || fields[0] != "go" || fields[1] != "run" || fields[2] != "./cmd/leia" {
			t.Fatalf("tooling audit command must use `go run ./cmd/leia ...`: %q", command)
		}
		timeout := 60 * time.Second
		if len(fields) > 3 {
			switch fields[3] {
			case "bench", "diag":
				timeout = 180 * time.Second
			case "doc":
				runCommand(t, root, timeout, fields[0], fields[1], fields[2], "doc", "--help")
				continue
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

func TestReleaseMatrixExamplesIndexCoversRegisteredDirectories(t *testing.T) {
	root := findRepoRoot(t)
	examplesDoc := readFileString(t, filepath.Join(root, "docs", "examples", "index.md"))
	examplesDir := filepath.Join(root, "examples")
	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		t.Fatalf("read examples directory: %v", err)
	}

	var missing []string
	registered := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(examplesDir, entry.Name())
		hasRegisteredSource := false
		err := filepath.WalkDir(dir, func(path string, child os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if child.IsDir() {
				return nil
			}
			switch filepath.Ext(path) {
			case ".leia", ".go":
				hasRegisteredSource = true
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walk example directory %s: %v", entry.Name(), err)
		}
		if !hasRegisteredSource {
			continue
		}
		registered[entry.Name()] = true
		ref := "`examples/" + entry.Name() + "/`"
		if !strings.Contains(examplesDoc, ref) {
			missing = append(missing, ref)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("docs/examples/index.md must index registered example directories: %s", strings.Join(missing, ", "))
	}

	indexedRE := regexp.MustCompile("`examples/([^`/]+)/`")
	var extra []string
	for _, match := range indexedRE.FindAllStringSubmatch(examplesDoc, -1) {
		if len(match) != 2 || registered[match[1]] {
			continue
		}
		extra = append(extra, "`examples/"+match[1]+"/`")
	}
	if len(extra) > 0 {
		sort.Strings(extra)
		t.Fatalf("docs/examples/index.md indexes missing or unregistered example directories: %s", strings.Join(extra, ", "))
	}
}

func TestReleaseMatrixReadmeCapabilitiesStayCoveredByExamples(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	examplesDoc := readFileString(t, filepath.Join(root, "docs", "examples", "index.md"))
	examplesReadme := readFileString(t, filepath.Join(root, "examples", "README.md"))
	cmdExamplesGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_command_test.go"))
	playgroundGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_playground_test.go"))
	examplesList := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "examples", "list", "--json")
	var examplesPayload struct {
		SchemaVersion int    `json:"schema_version"`
		Status        string `json:"status"`
		ExampleCount  int    `json:"example_count"`
		Examples      []struct {
			ID string `json:"id"`
		} `json:"examples"`
	}
	if err := json.Unmarshal([]byte(examplesList), &examplesPayload); err != nil {
		t.Fatalf("decode leia examples list --json: %v\n%s", err, examplesList)
	}
	if examplesPayload.SchemaVersion != 1 || examplesPayload.Status != "pass" || examplesPayload.ExampleCount != len(examplesPayload.Examples) {
		t.Fatalf("leia examples list --json = %+v, want schema v1 pass report with matching example_count", examplesPayload)
	}
	exampleIDs := map[string]bool{}
	for _, example := range examplesPayload.Examples {
		exampleIDs[example.ID] = true
	}
	features := loadFeatureMatrixFeatureMap(t, root)

	for _, promise := range []string{
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, q-style high-throughput in-memory columnar analytics, and first-class extensible domain dialects.",
		"q`sum ${a}`",
		"turn {",
		"prompt {",
	} {
		if !strings.Contains(readme, promise) {
			t.Fatalf("README.md concise surface must keep documented capability entry %q", promise)
		}
	}

	for _, item := range []struct {
		capability string
		dirs       []string
		docTerms   []string
		cliIDs     []string
		featureIDs []string
		docRefs    []string
	}{
		{
			capability: "embedding",
			dirs:       []string{"`examples/embedding/`"},
			docTerms:   []string{"Go embedding examples", "go test ./examples/embedding -run Example -count=1", "project-level reload gate"},
			cliIDs:     []string{"repo-embedding-go-doc-examples", "repo-embedding-hot_reload_project"},
			featureIDs: []string{"embedding_host_bindings", "sandbox_capabilities_module_loading", "embedding_resource_budgets", "embedding_hot_reload"},
			docRefs:    []string{"docs/reference/embedding/index.md", "docs/guides/embedding.md"},
		},
		{
			capability: "AI dialect",
			dirs:       []string{"`examples/ai/`", "`examples/llm/`", "`examples/evaluate/`", "`examples/workflow/`"},
			docTerms:   []string{"manual tool history", "replay fixture", "Live-provider examples", "project-level offline coding-agent gate"},
			cliIDs:     []string{"repo-llm-agent", "repo-ai-coding_agent_replay", "repo-ai-coding_agent_project-main", "repo-evaluate-agent_replay", "repo-workflow-support_triage_replay"},
			featureIDs: []string{"ai_dialect_integration"},
			docRefs:    []string{"docs/reference/ai/index.md", "docs/guides/ai-dialect.md", "docs/reference/evaluate/index.md"},
		},
		{
			capability: "DSL-native dialects",
			dirs:       []string{"`examples/dialects/`", "`examples/web/`", "`examples/tooling/`"},
			docTerms:   []string{"cross-domain release-gate project", "shell/process literals", "q-style columnar aggregation", "fullstack_project"},
			cliIDs:     []string{"repo-dialects-shell_filesystem", "repo-web-serve_dialect_app", "repo-web-fullstack_project-main", "repo-data-db_q_frame_project-main", "repo-tooling-release_gate_project-main"},
			featureIDs: []string{"tagged_dialect_syntax", "q_analytics_dialect", "spreadsheet_dialects"},
			docRefs:    []string{"docs/reference/dialects/index.md", "docs/reference/data-oriented/index.md"},
		},
		{
			capability: "concurrency",
			dirs:       []string{"`examples/concurrency/`"},
			docTerms:   []string{"goroutines", "channels", "select", "context cancellation"},
			cliIDs:     []string{"repo-concurrency-goroutines_channels", "repo-concurrency-select_timeout", "repo-concurrency-sync_group", "repo-concurrency-pipeline_project-main"},
			featureIDs: []string{"go_style_concurrency"},
			docRefs:    []string{"docs/reference/concurrency/index.md"},
		},
		{
			capability: "data-oriented",
			dirs:       []string{"`examples/data/`", "`examples/data_processing/`", "`examples/performance/`"},
			docTerms:   []string{"dense arrays", "matrices", "vectors", "SoA", "SQLite `db.frame`", "q/kdb+-style symbolic vectors"},
			cliIDs:     []string{"repo-data-q_vector_basics", "repo-data-q_trade_analytics_project-main", "repo-data-db_q_frame_project-main", "repo-data_processing-data_oriented-soa_kernels", "repo-performance-execution_modes_matrix"},
			featureIDs: []string{"matrix_dense_arrays", "q_analytics_dialect"},
			docRefs:    []string{"docs/reference/data-oriented/index.md", "docs/reference/scientific/index.md"},
		},
		{
			capability: "CLI tooling",
			dirs:       []string{"`examples/tooling/`", "`examples/testing/`"},
			docTerms:   []string{"release evidence", "diagnostics", "`leia test`", "cross-domain release-gate project"},
			cliIDs:     []string{"repo-tooling-release_evidence_pipeline", "repo-tooling-release_gate_project-main", "repo-testing-jsonl_workflow_test"},
			featureIDs: []string{"cli_repository_tooling", "release_evidence_gates"},
			docRefs:    []string{"docs/reference/cli/index.md", "docs/guides/tooling.md", "docs/release/index.md"},
		},
	} {
		var matrixRefs []string
		for _, featureID := range item.featureIDs {
			feature := requireFeature(t, features, featureID)
			cell := decodeReleaseCoverageCell(t, feature, -1, featureID, "semantic_gate")
			if !releaseGateStatus(cell.Status) {
				t.Fatalf("README %s feature %s semantic_gate status = %q, want covered", item.capability, featureID, cell.Status)
			}
			matrixRefs = append(matrixRefs, cell.Refs...)
		}
		for _, docRef := range item.docRefs {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(docRef))); err != nil {
				t.Fatalf("README %s docs binding %s is missing: %v", item.capability, docRef, err)
			}
			if !stringListContains(matrixRefs, docRef) {
				t.Fatalf("feature_matrix.json must bind README %s capability to docs ref %s via semantic_gate refs; got %#v", item.capability, docRef, matrixRefs)
			}
		}
		for _, dir := range item.dirs {
			if !strings.Contains(examplesDoc, dir) {
				t.Fatalf("docs/examples/index.md must map README %s capability to %s", item.capability, dir)
			}
		}
		combinedDocs := examplesDoc + "\n" + examplesReadme
		for _, term := range item.docTerms {
			if !strings.Contains(combinedDocs, term) {
				t.Fatalf("examples docs must describe README %s capability with %q", item.capability, term)
			}
		}
		for _, id := range item.cliIDs {
			if !exampleIDs[id] {
				t.Fatalf("leia examples list --json must include README %s example id %s", item.capability, id)
			}
		}
		if !releaseMatrixRefsBindCapabilityExamples(matrixRefs, item.dirs) {
			t.Fatalf("feature_matrix.json must bind README %s capability to at least one matching examples/ ref; dirs=%#v refs=%#v", item.capability, item.dirs, matrixRefs)
		}
	}

	for _, snippet := range []string{
		"TestExamplesCommandManifestMatchesPlaygroundRepositoryExamples",
		"TestExamplesCommandDefaultCheckSkipsOnlyOptInExamples",
		"TestExamplesCommandChecksMockFriendlyLLMExamples",
	} {
		if !strings.Contains(cmdExamplesGate, snippet) {
			t.Fatalf("cmd/leia/main_examples_command_test.go must keep examples manifest drift gate %q", snippet)
		}
	}
	for _, snippet := range []string{
		"TestPlaygroundRepositoryCoreExampleCoverage",
		"TestPlaygroundRepositoryAIDialectExamplesHaveExplicitGates",
		"TestPlaygroundRepositoryGameEngineExampleClassification",
	} {
		if !strings.Contains(playgroundGate, snippet) {
			t.Fatalf("cmd/leia/main_playground_test.go must keep playground examples drift gate %q", snippet)
		}
	}
}

func TestReleaseMatrixFeatureMatrixExampleRefsAreDiscoverable(t *testing.T) {
	root := findRepoRoot(t)
	matrix := loadReleaseFeatureMatrix(t, root)
	examplesList := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "examples", "list", "--json")
	var examplesPayload struct {
		SchemaVersion int    `json:"schema_version"`
		Status        string `json:"status"`
		ExampleCount  int    `json:"example_count"`
		Examples      []struct {
			Path      string `json:"path"`
			Runnable  bool   `json:"runnable"`
			Checkable bool   `json:"checkable"`
			Requires  string `json:"requires"`
		} `json:"examples"`
	}
	if err := json.Unmarshal([]byte(examplesList), &examplesPayload); err != nil {
		t.Fatalf("decode leia examples list --json: %v\n%s", err, examplesList)
	}
	if examplesPayload.SchemaVersion != 1 || examplesPayload.Status != "pass" || examplesPayload.ExampleCount != len(examplesPayload.Examples) {
		t.Fatalf("leia examples list --json = %+v, want schema v1 pass report with matching example_count", examplesPayload)
	}
	discovered := map[string]struct {
		runnable  bool
		checkable bool
		requires  string
	}{}
	for _, example := range examplesPayload.Examples {
		if example.Path == "" {
			t.Fatalf("examples list contains entry with empty path: %#v", example)
		}
		discovered[example.Path] = struct {
			runnable  bool
			checkable bool
			requires  string
		}{
			runnable:  example.Runnable,
			checkable: example.Checkable,
			requires:  example.Requires,
		}
	}

	var missing []string
	var inert []string
	for i, feature := range matrix.Features {
		id := decodeRequiredString(t, feature, i, "id")
		for _, field := range matrix.RequiredFields {
			cell := decodeReleaseCoverageCell(t, feature, i, id, field)
			if cell.Status != "covered" {
				continue
			}
			for _, ref := range cell.Refs {
				if !strings.HasPrefix(ref, "examples/") || !strings.HasSuffix(ref, ".leia") {
					continue
				}
				example, ok := discovered[ref]
				if !ok {
					missing = append(missing, id+"."+field+" -> "+ref)
					continue
				}
				if !example.runnable && !example.checkable && strings.TrimSpace(example.requires) == "" {
					inert = append(inert, id+"."+field+" -> "+ref)
				}
			}
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Fatalf("feature_matrix.json example refs must be discoverable by leia examples list --json: %s", strings.Join(missing, ", "))
	}
	if len(inert) > 0 {
		sort.Strings(inert)
		t.Fatalf("feature_matrix.json example refs must be runnable, checkable, or explicitly require opt-in state: %s", strings.Join(inert, ", "))
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

func TestReleaseMatrixReadmeAIDialectConcurrencyDataPromisesHaveGates(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	cmdExamplesGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_command_test.go"))
	features := loadFeatureMatrixFeatureMap(t, root)

	for _, item := range []struct {
		capability   string
		promise      string
		featureID    string
		specSections []string
		refs         []string
		exampleIDs   []string
		docSnippets  map[string][]string
	}{
		{
			capability:   "AI dialect",
			featureID:    "ai_dialect_integration",
			specSections: []string{"AI Dialect Syntax"},
			refs: []string{
				"cmd/leia/main_examples_command_test.go",
				"docs/guides/ai-dialect.md",
				"docs/reference/ai/index.md",
				"docs/reference/evaluate/index.md",
			},
			exampleIDs: []string{
				"repo-llm-agent",
				"repo-ai-coding_agent_replay",
				"repo-ai-coding_agent_project-main",
				"repo-evaluate-agent_replay",
			},
			docSnippets: map[string][]string{
				"docs/guides/ai-dialect.md":        {"The stable contract is in the [AI dialect reference](../reference/ai/index.md).", "Live-provider examples"},
				"docs/reference/ai/index.md":       {"Leia's AI support is an optional standard-library layer", "## Agent Dialect"},
				"docs/reference/evaluate/index.md": {"replay-drift findings"},
			},
		},
		{
			capability:   "DSL-native dialects",
			promise:      "first-class extensible domain dialects",
			featureID:    "tagged_dialect_syntax",
			specSections: []string{"Grammar Appendix", "Expressions", "Statements"},
			refs: []string{
				"cmd/leia/main_examples_command_test.go",
				"docs/reference/dialects/index.md",
				"examples/dialects/shell_filesystem.leia",
				"examples/web/serve_dialect_app.leia",
				"examples/web/fullstack_project/main.leia",
				"examples/data/db_q_frame_project/main.leia",
				"examples/tooling/release_gate_project/main.leia",
			},
			exampleIDs: []string{
				"repo-dialects-shell_filesystem",
				"repo-web-serve_dialect_app",
				"repo-web-fullstack_project-main",
				"repo-data-db_q_frame_project-main",
				"repo-tooling-release_gate_project-main",
			},
			docSnippets: map[string][]string{
				"docs/reference/dialects/index.md": {"Leia supports DSL-native tagged dialects", "## Built-In Dialects"},
			},
		},
		{
			capability:   "concurrency",
			promise:      "",
			featureID:    "go_style_concurrency",
			specSections: []string{"Concurrency"},
			refs: []string{
				"cmd/leia/main_examples_command_test.go",
				"docs/reference/concurrency/index.md",
				"tests/concurrency_contract_test.go",
				"examples/concurrency/pipeline_project/main.leia",
				"scripts/production_check.sh",
			},
			exampleIDs: []string{
				"repo-concurrency-goroutines_channels",
				"repo-concurrency-select_timeout",
				"repo-concurrency-sync_group",
				"repo-concurrency-pipeline_project-main",
			},
			docSnippets: map[string][]string{
				"docs/reference/concurrency/index.md": {"Leia exposes Go-style concurrency", "examples/concurrency/"},
			},
		},
		{
			capability:   "data-oriented",
			promise:      "q-style high-throughput in-memory columnar analytics",
			featureID:    "matrix_dense_arrays",
			specSections: []string{"Tables And Metatables", "Implementation Requirements"},
			refs: []string{
				"cmd/leia/main_examples_command_test.go",
				"docs/reference/data-oriented/index.md",
				"docs/reference/scientific/index.md",
				"examples/data_processing/data_oriented/soa_kernels.leia",
				"examples/data_processing/data_oriented/dense_matrix_vec_kernels.leia",
			},
			exampleIDs: []string{
				"repo-data-q_vector_basics",
				"repo-data-q_trade_analytics_project-main",
				"repo-data-db_q_frame_project-main",
				"repo-data_processing-data_oriented-soa_kernels",
				"repo-data_processing-data_oriented-dense_matrix_vec_kernels",
			},
			docSnippets: map[string][]string{
				"docs/reference/data-oriented/index.md": {"Leia includes data-oriented standard libraries", "## Structure Of Arrays"},
				"docs/reference/scientific/index.md":    {"Leia keeps scientific code in ordinary Leia source", "## Default Numeric Imports", "## Primitive Composition"},
			},
		},
	} {
		if item.promise != "" && !strings.Contains(readme, item.promise) {
			t.Fatalf("README.md concise surface must keep %s entry %q", item.capability, item.promise)
		}

		feature := requireFeature(t, features, item.featureID)
		rawSections := feature["spec_sections"]
		var sections []string
		if err := json.Unmarshal(rawSections, &sections); err != nil {
			t.Fatalf("%s.spec_sections: %v", item.featureID, err)
		}
		sectionSet := map[string]bool{}
		for _, section := range sections {
			sectionSet[section] = true
		}
		for _, section := range item.specSections {
			if !sectionSet[section] {
				t.Fatalf("%s feature %s must link README promise to spec section %q; got %#v", item.capability, item.featureID, section, sections)
			}
		}
		requireFeatureCellRefs(t, feature, item.featureID, "semantic_gate", item.refs...)

		for _, id := range item.exampleIDs {
			if !strings.Contains(cmdExamplesGate, id) {
				t.Fatalf("cmd/leia/main_examples_command_test.go must examples-check README %s id %s", item.capability, id)
			}
		}
		for rel, snippets := range item.docSnippets {
			doc := readFileString(t, filepath.Join(root, filepath.FromSlash(rel)))
			for _, snippet := range snippets {
				if !strings.Contains(doc, snippet) {
					t.Fatalf("%s must document README %s promise with %q", rel, item.capability, snippet)
				}
			}
		}
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

func releaseToolingAuditCommands() []string {
	return []string{
		"go run ./cmd/leia eval 'print(1 + 2 + 3)'",
		"go run ./cmd/leia examples check examples/hello/dialects.leia",
		"go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3",
		"go run ./cmd/leia doc check",
	}
}

func readReleaseReadmeSurfaceLeiaSnippet(t *testing.T, root string) string {
	t.Helper()
	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, marker := range []string{"```go", "````leia", "```leia"} {
		blockStart := strings.Index(readme, marker)
		if blockStart == -1 {
			continue
		}
		rest := readme[blockStart+len(marker):]
		endMarker := strings.Repeat("`", strings.Count(marker, "`"))
		blockEnd := strings.Index(rest, endMarker)
		if blockEnd == -1 {
			t.Fatal("README.md Example code block is unterminated")
		}
		return strings.TrimSpace(rest[:blockEnd])
	}
	t.Fatal("README.md must contain a surface code block")
	return ""
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

func releaseMatrixRefIsExecutableGate(ref string) bool {
	switch {
	case strings.HasSuffix(ref, "_test.go"):
		return true
	case strings.HasPrefix(ref, "tests/language/") && strings.HasSuffix(ref, ".leia"):
		return true
	case strings.HasPrefix(ref, "tests/smoke/") && strings.HasSuffix(ref, ".leia"):
		return true
	case strings.HasPrefix(ref, "examples/") && strings.HasSuffix(ref, ".leia"):
		return true
	case strings.HasPrefix(ref, "benchmarks/") && (strings.HasSuffix(ref, ".leia") || strings.HasSuffix(ref, "_test.py") || strings.HasSuffix(ref, "_test.go")):
		return true
	case strings.HasPrefix(ref, "scripts/") && strings.HasSuffix(ref, ".sh"):
		return true
	default:
		return false
	}
}

func releaseMatrixRefsBindCapabilityExamples(refs, dirs []string) bool {
	for _, ref := range refs {
		if !strings.HasPrefix(ref, "examples/") {
			continue
		}
		for _, dir := range dirs {
			prefix := strings.Trim(dir, "`")
			if strings.HasPrefix(ref, prefix) {
				return true
			}
		}
	}
	return false
}

func stringListContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func readGoreleaserTargets(t *testing.T, path string) []string {
	t.Helper()
	data := readFileString(t, path)
	lines := strings.Split(data, "\n")
	var goos, goarch []string
	var section string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "goos:":
			section = "goos"
			continue
		case "goarch:":
			section = "goarch"
			continue
		}
		if strings.HasSuffix(trimmed, ":") && trimmed != "goos:" && trimmed != "goarch:" {
			section = ""
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		switch section {
		case "goos":
			goos = appendUniqueString(goos, value)
		case "goarch":
			goarch = appendUniqueString(goarch, value)
		}
	}
	if len(goos) == 0 || len(goarch) == 0 {
		t.Fatalf("%s must declare goos and goarch matrices", path)
	}
	targets := make([]string, 0, len(goos)*len(goarch))
	for _, osName := range goos {
		for _, arch := range goarch {
			targets = append(targets, osName+"/"+arch)
		}
	}
	sort.Strings(targets)
	return targets
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
