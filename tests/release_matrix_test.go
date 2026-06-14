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
	releaseMatrixCmd := "go test ./tests -run 'TestFeatureMatrix|TestReleaseMatrix' -count=1"
	specExamplesCmd := "go test ./tests -run 'TestSpecRunnableExamples|TestSpecLeiaCodeFencesAreExecutableOrExplicitlyNonExecutable' -count=1"
	docsCheckCmd := "bash scripts/docs_check.sh"
	ciReleaseListCmd := "go run ./cmd/leia ci release --list"
	productionFullCmd := "bash scripts/production_check.sh --full --release-profile"
	performanceSmokeCmd := "bash scripts/performance_gate.sh --smoke"
	fullPerfGateCmd := "bash scripts/performance_gate.sh --full"
	publicReleaseBlockersCmd := "bash scripts/public_release_blockers_check.sh --require-resolved"
	releaseDistributionCmd := "bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows"
	strictReleaseArtifactsCmd := "bash scripts/release_artifacts_check.sh --build --require-clean"

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
				ciReleaseListCmd,
				productionFullCmd,
				"profile is the release gate source of truth",
				"q conformance",
				"local artifact installation evidence",
				releaseDistributionCmd,
				"tests/feature_matrix.json",
				"docs/spec/index.md",
			},
		},
		{
			path: "docs/release/notes-template.md",
			snippets: []string{
				ciReleaseListCmd,
				productionFullCmd,
				releaseMatrixCmd,
				docsCheckCmd,
				fullPerfGateCmd,
				publicReleaseBlockersCmd,
				releaseDistributionCmd,
				strictReleaseArtifactsCmd,
			},
		},
		{
			path: "scripts/docs_check.sh",
			snippets: []string{
				releaseMatrixCmd,
				specExamplesCmd,
				"TestReleaseMatrixFeatureDocsStayCoveredBySpecAndReference|TestReleaseMatrixDocsIndexCoversReferenceEntrypoints|TestReleaseMatrixReadmeReferencesEntrypointsStayGated",
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
		releaseMatrixCmd,
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
		"[Language specification](docs/spec/index.md)",
		"Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.",
		"## Example",
		"## References",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README.md must keep language contract snippet %q", snippet)
		}
	}

	specGate := readFileString(t, filepath.Join(root, "tests", "docs", "spec", "spec_contract_test.go"))
	for _, snippet := range []string{
		"TestReadmeAndSpecStableContractStayAligned",
		"[Language specification](docs/spec/index.md)",
		"## Example",
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
		"Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.",
		"ARM64 JIT",
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
		"[Language specification](docs/spec/index.md)",
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

func TestReleaseMatrixReadmePackageMetadataPromiseHasProductionGates(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, snippet := range []string{
		"[Modules](docs/reference/modules/index.md)",
		"[Packages and modules](docs/guides/packages.md)",
	} {
		if !strings.Contains(readme, snippet) {
			t.Fatalf("README.md must keep package metadata promise snippet %q", snippet)
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
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				"bash scripts/release_distribution_check.sh --require-goreleaser --require-workflows",
				"goreleaser release --snapshot --clean --skip=publish",
			},
		},
		{
			path: ".github/workflows/release.yml",
			snippets: []string{
				"name: Release",
				"go run ./cmd/leia ci release",
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				"LEIA_RELEASE_REQUIRE_TAG=1",
				"LEIA_RELEASE_ARTIFACT_VERSION=\"${GITHUB_REF_NAME}\"",
				"goreleaser release --snapshot --clean --skip=publish",
				"goreleaser release --clean",
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

func TestReleaseMatrixReadmeToolingPromiseHasEvidence(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))

	for _, item := range []struct {
		category        string
		readmeCommand   string
		evidencePath    string
		evidenceSnippet string
	}{
		{
			category:        "eval",
			readmeCommand:   "go run ./cmd/leia eval 'print(1 + 2 + 3)'",
			evidencePath:    "cmd/leia/eval.go",
			evidenceSnippet: "func runEvalCommand",
		},
		{
			category:        "examples",
			readmeCommand:   "go run ./cmd/leia examples check examples/hello/dialects.leia",
			evidencePath:    "cmd/leia/main_examples_command_test.go",
			evidenceSnippet: "TestExamplesCommandChecksSelectedExamples",
		},
		{
			category:        "docs",
			readmeCommand:   "go run ./cmd/leia doc check",
			evidencePath:    "cmd/leia/doc.go",
			evidenceSnippet: "func runDocCommand",
		},
		{
			category:        "benchmarks",
			readmeCommand:   "go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3",
			evidencePath:    "cmd/leia/main_bench_test.go",
			evidenceSnippet: "TestBenchCommandDispatchesCompareHarness",
		},
	} {
		if !strings.Contains(readme, item.readmeCommand) {
			t.Fatalf("README.md Tooling commands must cover %s via %q", item.category, item.readmeCommand)
		}
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.evidencePath)))
		if !strings.Contains(text, item.evidenceSnippet) {
			t.Fatalf("%s must keep focused test evidence for README %s tooling via %q", item.evidencePath, item.category, item.evidenceSnippet)
		}
	}

	toolingGuide := readFileString(t, filepath.Join(root, "docs", "guides", "tooling.md"))
	for _, snippet := range []string{
		"## Modules",
		"## Documentation",
		"## Diagnostics",
		"## Playground",
		"## Release Evidence",
		"go run ./cmd/leia mod verify --json examples/ui/package_managed",
		"go run ./cmd/leia doc check",
		"GitHub Pages publishes `docs/` through `.github/workflows/pages.yml`",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks",
		"go run ./cmd/leia playground --help",
		"go run ./cmd/leia playground --addr 127.0.0.1:8080",
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
		{path: "scripts/performance_gate.sh", snippet: "benchmarks/timing_compare.py"},
		{path: "scripts/production_check.sh", snippet: "add_release_smoke"},
		{path: "scripts/production_check.sh", snippet: "RELEASE_CRITICAL_SKIP_NAMES"},
		{path: "scripts/production_check.sh", snippet: "Release profile requires these checks to run instead of skip:"},
		{path: "scripts/public_release_blockers_check.sh", snippet: "--require-resolved"},
		{path: "scripts/release_artifacts_check.sh", snippet: "Default mode runs a dry-run"},
	} {
		text := readFileString(t, filepath.Join(root, filepath.FromSlash(item.path)))
		if !strings.Contains(text, item.snippet) {
			t.Fatalf("%s must keep README tooling script evidence snippet %q", item.path, item.snippet)
		}
	}
}

func TestReleaseMatrixReadmeToolingCommandsStayInToolingGuide(t *testing.T) {
	root := findRepoRoot(t)
	toolingGuide := readFileString(t, filepath.Join(root, "docs", "guides", "tooling.md"))
	for _, command := range readReleaseReadmeToolingCommands(t, root) {
		if !strings.Contains(toolingGuide, command) {
			t.Fatalf("docs/guides/tooling.md must document README Tooling command %q", command)
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

	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, ref := range []string{"SECURITY.md", "CONTRIBUTING.md", "CODE_OF_CONDUCT.md"} {
		if !strings.Contains(readme, ref) {
			t.Fatalf("README.md must link %s", ref)
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

func TestReleaseMatrixReadmeReferencesEntrypointsStayLinked(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	refs := readReadmeReferencesEntrypoints(t, readme)
	for _, ref := range refs {
		if !strings.Contains(readme, "("+ref+")") {
			t.Fatalf("README.md References section must link %s", ref)
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(ref))); err != nil {
			t.Fatalf("README.md References link target %s is missing: %v", ref, err)
		}
	}
}

func TestReleaseMatrixReadmeReferencesEntrypointsStayGated(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	docsCheck := readFileString(t, filepath.Join(root, "scripts", "docs_check.sh"))
	releaseMatrix := readFileString(t, filepath.Join(root, "tests", "release_matrix_test.go"))
	refs := readReadmeReferencesEntrypoints(t, readme)

	for _, snippet := range []string{
		"check_readme_reference_entrypoints",
		"README References entrypoints",
		"check_markdown_links(doc_file)",
		"find reference -type f -name '*.md' | sort",
		"generated reference doc is missing from docs",
	} {
		if !strings.Contains(docsCheck, snippet) {
			t.Fatalf("scripts/docs_check.sh must keep README References gate snippet %q", snippet)
		}
	}
	for _, snippet := range []string{
		"TestReleaseMatrixReadmeReferencesEntrypointsStayLinked",
		"TestReleaseMatrixReadmeReferencesEntrypointsStayGated",
		"readReadmeReferencesEntrypoints",
	} {
		if !strings.Contains(releaseMatrix, snippet) {
			t.Fatalf("tests/release_matrix_test.go must keep README References drift gate snippet %q", snippet)
		}
	}

	generatedEntrypoints := map[string]string{
		"docs/reference/dialects/index.md": "go run ./cmd/leia doc generate --layout site --output",
	}
	seenGenerated := map[string]bool{}
	for _, ref := range refs {
		if generator, ok := generatedEntrypoints[ref]; ok {
			seenGenerated[ref] = true
			for _, snippet := range []string{"stale", generator, "find reference -type f -name '*.md' | sort", "generated_doc=\"docs/$generated\""} {
				if !strings.Contains(docsCheck, snippet) {
					t.Fatalf("scripts/docs_check.sh must keep generated/stale gate for README References entrypoint %s via %q", ref, snippet)
				}
			}
		}
	}
	for ref := range generatedEntrypoints {
		if !seenGenerated[ref] {
			t.Fatalf("README.md References section must keep generated entrypoint %s covered by docs_check.sh stale checks", ref)
		}
	}
}

func TestReleaseMatrixDocsIndexCoversReadmeReferencesEntrypoints(t *testing.T) {
	root := findRepoRoot(t)
	readme := readFileString(t, filepath.Join(root, "README.md"))
	docsIndex := readFileString(t, filepath.Join(root, "docs", "index.md"))
	refs := readReadmeReferencesEntrypoints(t, readme)

	for _, ref := range refs {
		if ref == "docs/index.md" {
			if !strings.Contains(docsIndex, "# Leia") {
				t.Fatal("docs/index.md must remain the documentation home linked from README.md")
			}
			continue
		}

		indexRef := ref
		if strings.HasPrefix(ref, "docs/") {
			indexRef = strings.TrimPrefix(ref, "docs/")
		} else {
			indexRef = "../" + ref
		}
		githubRef := "https://github.com/never-labs/leia/blob/main/" + ref
		if !strings.Contains(docsIndex, "("+indexRef+")") && !strings.Contains(docsIndex, "("+githubRef+")") {
			t.Fatalf("docs/index.md must link README References entrypoint %s as %s or %s", ref, indexRef, githubRef)
		}
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
			specSections: []string{"Grammar Appendix", "Expressions", "Statements"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/dialects/index.md"},
		},
		"q_analytics_dialect": {
			specSections: []string{"Grammar Appendix", "Expressions", "Values And Types"},
			docPaths:     []string{"docs/spec/index.md", "docs/reference/data-oriented/index.md", "docs/reference/dialects/index.md"},
		},
		"data_stdlib_qsql": {
			specSections: []string{"Expressions", "Values And Types", "Tables And Metatables", "Implementation Requirements"},
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
			docPaths:     []string{"docs/spec/index.md", "docs/reference/data-oriented/index.md"},
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
		"trades := q```flip `sym`price`size`notional!",
		"total := q`+/10000 12180 8060 15337.5`",
		"leaders := q.sql(",
		"note := prompt`Top symbol ${leaders[1].sym}",
		"print(note.text)",
	} {
		if !strings.Contains(surfaceSnippet, snippet) {
			t.Fatalf("README.md Example snippet changed or lost product surface %q:\n%s", snippet, surfaceSnippet)
		}
	}

	embeddingSnippet := readReleaseReadmeEmbeddingGoSnippet(t, root)
	for _, snippet := range []string{
		`import leia "github.com/never-labs/leia"`,
		"leia.New(leia.WithLibs(leia.LibSafe))",
		`vm.Exec(`,
		"print(q",
		"`+/1 2 3`",
	} {
		if !strings.Contains(embeddingSnippet, snippet) {
			t.Fatalf("README.md Embedding snippet changed or lost executable public SDK surface %q:\n%s", snippet, embeddingSnippet)
		}
	}

	focusedGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_readme_tooling_test.go"))
	for _, snippet := range []string{
		"TestReadmeIntroStaysFocused",
		"Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.",
		"Performance-oriented:",
		"LuaJIT-class workloads",
		"Analytics-native:",
		"q-style vector syntax",
		"Dialect-native:",
		"AI tags such",
		"AI support lives in dialects and libraries, not in the core language runtime.",
		"## Example",
		"## Tooling",
		"## References",
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
		"Top symbol",
	} {
		if !strings.Contains(focusedGate, snippet) {
			t.Fatalf("cmd/leia/main_readme_tooling_test.go must keep README Surface focused gate snippet %q", snippet)
		}
	}
	for _, snippet := range []string{
		"TestReadmeEmbeddingSnippetStaysRunnable",
		"readmeEmbeddingGoSnippet",
		"README embedding snippet failed",
		"README embedding snippet stdout",
		`exec.Command("go", "run", "-mod=mod", ".")`,
		`replace github.com/never-labs/leia => `,
	} {
		if !strings.Contains(focusedGate, snippet) {
			t.Fatalf("cmd/leia/main_readme_tooling_test.go must keep README Embedding focused gate snippet %q", snippet)
		}
	}
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
		fields, err := dialect.Shellwords(command)
		if err != nil {
			t.Fatalf("README Tooling command is not valid shellwords %q: %v", command, err)
		}
		if len(fields) < 4 || fields[0] != "go" || fields[1] != "run" || fields[2] != "./cmd/leia" {
			t.Fatalf("README Tooling command must use `go run ./cmd/leia ...`: %q", command)
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
		Examples []struct {
			ID string `json:"id"`
		} `json:"examples"`
	}
	if err := json.Unmarshal([]byte(examplesList), &examplesPayload); err != nil {
		t.Fatalf("decode leia examples list --json: %v\n%s", err, examplesList)
	}
	exampleIDs := map[string]bool{}
	for _, example := range examplesPayload.Examples {
		exampleIDs[example.ID] = true
	}
	features := loadFeatureMatrixFeatureMap(t, root)

	for _, promise := range []string{
		"Leia is a Go-native scripting language built for DSLs, dialects, and embedded automation.",
		"Go-native:",
		"LuaJIT-class workloads",
		"q-style vector syntax",
		"high-throughput in-memory columnar computation",
		"q.sql(",
		"prompt`",
		"AI support lives in dialects and libraries, not in the core language runtime.",
		"go run ./cmd/leia bench compare --bench data/q_operator_pipeline --runs 3",
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
			docRefs:    []string{"docs/reference/data-oriented/index.md"},
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
		Examples []struct {
			Path      string `json:"path"`
			Runnable  bool   `json:"runnable"`
			Checkable bool   `json:"checkable"`
			Requires  string `json:"requires"`
		} `json:"examples"`
	}
	if err := json.Unmarshal([]byte(examplesList), &examplesPayload); err != nil {
		t.Fatalf("decode leia examples list --json: %v\n%s", err, examplesList)
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
			promise:      "AI support lives in dialects and libraries, not in the core language runtime.",
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
			promise:      "Dialect-native: `q`, `sql`, `json`, `yaml`, `prompt`, `quote`, AI tags",
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
			promise:      "q-style vector syntax, qSQL, typed runtime kernels",
			featureID:    "matrix_dense_arrays",
			specSections: []string{"Tables And Metatables", "Implementation Requirements"},
			refs: []string{
				"cmd/leia/main_examples_command_test.go",
				"docs/reference/data-oriented/index.md",
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

func readReleaseReadmeSurfaceLeiaSnippet(t *testing.T, root string) string {
	t.Helper()
	readme := readFileString(t, filepath.Join(root, "README.md"))
	const marker = "## Example"
	start := strings.Index(readme, marker)
	if start == -1 {
		t.Fatal("README.md must contain an Example section")
	}
	rest := readme[start+len(marker):]
	blockStart := strings.Index(rest, "````leia")
	if blockStart == -1 {
		t.Fatal("README.md Example section must contain a Leia code block")
	}
	rest = rest[blockStart+len("````leia"):]
	blockEnd := strings.Index(rest, "````")
	if blockEnd == -1 {
		t.Fatal("README.md Example Leia code block is unterminated")
	}
	return strings.TrimSpace(rest[:blockEnd])
}

func readReleaseReadmeEmbeddingGoSnippet(t *testing.T, root string) string {
	t.Helper()
	readme := readFileString(t, filepath.Join(root, "README.md"))
	const marker = "## Embedding"
	start := strings.Index(readme, marker)
	if start == -1 {
		t.Fatal("README.md must contain an Embedding section")
	}
	rest := readme[start+len(marker):]
	blockStart := strings.Index(rest, "```go")
	if blockStart == -1 {
		t.Fatal("README.md Embedding section must contain a Go code block")
	}
	rest = rest[blockStart+len("```go"):]
	blockEnd := strings.Index(rest, "```")
	if blockEnd == -1 {
		t.Fatal("README.md Embedding Go code block is unterminated")
	}
	return strings.TrimSpace(rest[:blockEnd])
}

func readReadmeReferencesEntrypoints(t *testing.T, readme string) []string {
	t.Helper()
	start := strings.Index(readme, "## References")
	if start == -1 {
		t.Fatal("README.md must contain a References section")
	}
	end := strings.Index(readme[start+len("## References"):], "\n## ")
	section := readme[start:]
	if end != -1 {
		section = readme[start : start+len("## References")+end]
	}

	linkRE := regexp.MustCompile(`\[[^\]\n]+\]\(([^)\s]+)`)
	seen := map[string]bool{}
	var refs []string
	for _, line := range strings.Split(section, "\n") {
		if !strings.HasPrefix(line, "- ") {
			continue
		}
		match := linkRE.FindStringSubmatch(line)
		if len(match) != 2 {
			t.Fatalf("README.md References entry must be a Markdown link: %q", line)
		}
		ref := strings.Split(strings.Split(match[1], "#")[0], "?")[0]
		if ref == "" {
			t.Fatalf("README.md References entry has empty link target: %q", line)
		}
		if seen[ref] {
			t.Fatalf("README.md References section links %s more than once", ref)
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	if len(refs) == 0 {
		t.Fatal("README.md References section must list documentation entrypoints")
	}
	return refs
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

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
