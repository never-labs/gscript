package tests_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	releaseMatrixCmd := "scripts/run.sh test release-matrix"
	internalReleaseMatrixCmd := releaseMatrixCmd
	specExamplesCmd := "scripts/run.sh test spec-examples"
	docsCheckCmd := "scripts/run.sh docs"
	ciReleaseListCmd := "go run ./cmd/leia ci release --list"
	ciReleaseVersionListCmd := "go run ./cmd/leia ci release --release-version vX.Y.Z --list"
	productionFullCmd := "scripts/run.sh production --full --release-profile --release-version vX.Y.Z"
	performanceSmokeCmd := "scripts/run.sh perf --smoke"
	fullPerfGateCmd := "scripts/run.sh perf --full"
	shellSyntaxCmd := "scripts/run.sh shell-syntax"
	publicReleaseBlockersCmd := "scripts/run.sh public-blockers --require-resolved"
	releaseDistributionCmd := "scripts/run.sh release-dist --require-goreleaser"
	releaseNotesCmd := "scripts/run.sh release-notes --require-ready --version vX.Y.Z"
	strictReleaseArtifactsCmd := "scripts/run.sh release-check --build --require-clean --require-tag --version vX.Y.Z"

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
				"`leia_vX.Y.Z_darwin_amd64.tar.gz`",
				"`leia_vX.Y.Z_darwin_arm64.tar.gz`",
				"`leia_vX.Y.Z_linux_amd64.tar.gz`",
				"`leia_vX.Y.Z_linux_arm64.tar.gz`",
				"`leia_vX.Y.Z_windows_amd64.zip`",
				"`leia_vX.Y.Z_windows_arm64.zip`",
				"Each archive includes `leia` and `leia-lsp`.",
			},
		},
		{
			path: "docs/guides/tooling.md",
			snippets: []string{
				ciReleaseListCmd,
				"`status_field`",
				"`scalar_fields`",
				"`count_fields`",
				"`collection_fields`",
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
		shellSyntaxCmd,
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
	if !strings.Contains(fullOut, shellSyntaxCmd) {
		t.Fatalf("production_check.sh --full --list must keep shell syntax gate %q; got:\n%s", shellSyntaxCmd, fullOut)
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
		`"Shell Script Syntax"`,
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
		t.Fatalf("release profile unexpectedly passed with language oracle absent from PATH:\n%s", out)
	}
	text := string(out)
	for _, want := range []string{
		"Runnable checks:",
		"Language Conformance Surface: missing lua",
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
	releaseMatrixCmd := "scripts/run.sh test release-matrix"
	specContractCmd := "go test ./tests/docs/spec -count=1"
	specExamplesCmd := "scripts/run.sh test spec-examples"
	docsCheckCmd := "scripts/run.sh docs"

	readme := readFileString(t, filepath.Join(root, "README.md"))
	for _, snippet := range []string{
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, high-throughput in-memory data runtime, and first-class extensible domain dialects.",
		"x := sum(a)",
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
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, high-throughput in-memory data runtime, and first-class extensible domain dialects.",
		"x := sum(a)",
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
		"scripts/run.sh test correctness",
		"Release Matrix Metadata: covered by Correctness (scripts/run.sh test correctness)",
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
		"scripts/run.sh module-path github.com/never-labs/leia",
		"scripts/run.sh cli-experience",
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
	if !strings.Contains(releaseOut, "scripts/run.sh module-path github.com/never-labs/leia") {
		t.Fatalf("production release profile must include module path gate output; got:\n%s", releaseOut)
	}
}

func TestReleaseMatrixShellScriptsParse(t *testing.T) {
	root := findRepoRoot(t)
	var scripts []string
	for _, pattern := range []string{"scripts/*.sh", "benchmarks/*.sh"} {
		matches, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(pattern)))
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		scripts = append(scripts, matches...)
	}
	if len(scripts) == 0 {
		t.Fatal("release shell syntax gate found no scripts")
	}
	sort.Strings(scripts)
	for _, script := range scripts {
		rel, err := filepath.Rel(root, script)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(filepath.ToSlash(rel), func(t *testing.T) {
			runCommand(t, root, 30*time.Second, "bash", "-n", rel)
		})
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
				"log_info \"release_distribution_check.sh: $file not present; skipping hosted workflow check\"",
				`require_contains .github/workflows/distribution-check.yml "- scripts/run.sh"`,
				"require_file docs/_config.yml",
				"require_contains docs/_config.yml \"spec/index.html\"",
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				"check_local_install_fixture",
				"install accepted archive with unexpected entry",
				"install accepted zip archive with unexpected entry",
				"--base-url \"file://$release_dir\"",
				"local install fixture verified",
				`go run ./cmd/leia ci release --release-version "${GITHUB_REF_NAME}"`,
				"scripts/run.sh release-snapshot --dist-dir dist --bin-dir /tmp/leia-snapshot-bin",
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
				"- scripts/run.sh",
				"scripts/production_check.sh",
				"scripts/public_release_blockers_check.sh",
				"scripts/release_notes_check.sh",
				"scripts/release_snapshot_install_check.sh",
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				`"$(go env GOPATH)/bin/goreleaser" --version`,
				"scripts/run.sh release-notes",
				"scripts/run.sh release-dist --require-goreleaser --require-workflows",
				`"$(go env GOPATH)/bin/goreleaser" release --snapshot --clean --skip=publish`,
				"scripts/run.sh release-snapshot --dist-dir dist --bin-dir /tmp/leia-snapshot-bin",
			},
		},
		{
			path: ".github/workflows/release.yml",
			snippets: []string{
				"name: Release",
				"go run ./cmd/leia ci release",
				`go run ./cmd/leia ci release --release-version "${GITHUB_REF_NAME}"`,
				"go install github.com/goreleaser/goreleaser/v2@v2.16.0",
				"release tags must match vMAJOR.MINOR.PATCH",
				`"$(go env GOPATH)/bin/goreleaser" --version`,
				"LEIA_RELEASE_REQUIRE_TAG=1",
				"LEIA_RELEASE_ARTIFACT_VERSION=\"${GITHUB_REF_NAME}\"",
				`scripts/run.sh release-notes --require-ready --version "${GITHUB_REF_NAME}"`,
				`"$(go env GOPATH)/bin/goreleaser" release --snapshot --clean --skip=publish`,
				"scripts/run.sh release-snapshot --dist-dir dist --bin-dir /tmp/leia-snapshot-bin",
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
				"scripts/run.sh docs",
				"actions/jekyll-build-pages",
				"source: ./docs",
				"destination: ./_site",
				"scripts/run.sh site --site-dir ./_site",
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
	for _, want := range []string{"go test ./cmd/leia", "scripts/run.sh manifest-check tests benchmarks"} {
		if !strings.Contains(smokeOut, want) {
			t.Fatalf("ci smoke --list must include %q so example/import guards stay in the smoke test matrix; got:\n%s", want, smokeOut)
		}
	}
	prOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "pr", "--no-luajit", "--list")
	for _, want := range []string{"scripts/run.sh test correctness", "go run ./cmd/leia examples check --jobs=6", "scripts/run.sh docs", "scripts/run.sh perf --smoke --no-luajit"} {
		if !strings.Contains(prOut, want) {
			t.Fatalf("ci pr --list must include %q so PR validation matches product gates; got:\n%s", want, prOut)
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
	if strings.TrimSpace(releaseOut) != "scripts/run.sh production --full --release-profile" {
		t.Fatalf("ci release --list must delegate to production release profile only; got:\n%s", releaseOut)
	}
	versionedReleaseOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "release", "--release-version", "vX.Y.Z", "--list")
	if strings.TrimSpace(versionedReleaseOut) != "scripts/run.sh production --full --release-profile --release-version vX.Y.Z" {
		t.Fatalf("ci release --release-version --list must delegate to versioned production release profile only; got:\n%s", versionedReleaseOut)
	}
	badReleaseVersion := runCommandResult(root, 30*time.Second, "go", "run", "./cmd/leia", "ci", "release", "--release-version", "bad version")
	if badReleaseVersion.err == nil || !strings.Contains(badReleaseVersion.stderr, "--release-version must match vMAJOR.MINOR.PATCH") {
		t.Fatalf("ci release bad version should fail with a clear format error\nstdout:\n%s\nstderr:\n%s", badReleaseVersion.stdout, badReleaseVersion.stderr)
	}

	productionOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--list")
	for _, want := range []string{"scripts/run.sh test correctness", "scripts/run.sh manifest-check tests benchmarks"} {
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
		"scripts/run.sh cli-experience",
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
		"scripts/run.sh worktree --json",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-benchmarks",
		"go run ./cmd/leia diag bundle --output /tmp/leia-diag --skip-go-tests --skip-benchmarks --json",
		"go run ./cmd/leia playground --help",
		"go run ./cmd/leia playground --addr 127.0.0.1:8080",
		"scripts/run.sh production --quick --list --json",
		"scripts/run.sh production --quick --list --out-dir /tmp/leia-release-plan",
		"go run ./cmd/leia ci release --release-version vX.Y.Z --list --json",
		"scripts/run.sh perf --validate-only /tmp/leia_performance_gate/timing_gate.json --json",
		"scripts/run.sh editor --json",
		"scripts/run.sh release-dist --json",
		"scripts/run.sh release-check --json --version vX.Y.Z",
		"scripts/run.sh release-check",
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
		{path: "scripts/performance_gate.sh", snippet: "go run ./cmd/leia bench compare"},
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

	diagnosticsRef := readFileString(t, filepath.Join(root, "docs", "reference", "diagnostics", "index.md"))
	for _, snippet := range []string{
		"`tooling.reports` JSON report registry",
		"`status_field`",
		"`scalar_fields`",
		"`count_fields`",
		"`collection_fields`",
		"field names are listed in `leia capabilities --json` under `tooling.reports`",
	} {
		if !strings.Contains(diagnosticsRef, snippet) {
			t.Fatalf("docs/reference/diagnostics/index.md must keep JSON registry snippet %q", snippet)
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
		"examples/automation/release_fixture_matrix.leia",
		"examples/automation/release_risk_digest.leia",
	)

	exampleGate := readFileString(t, filepath.Join(root, "cmd", "leia", "main_examples_command_test.go"))
	for _, snippet := range []string{
		"repo-tooling-release_evidence_pipeline",
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
		"scripts/run.sh production --full --release-profile --release-version vX.Y.Z --list --out-dir /tmp/leia-release-plan",
		"scripts/run.sh production --full --release-profile --release-version vX.Y.Z --list --json",
		"go run ./cmd/leia doc check --json",
		"scripts/run.sh editor --json",
		"scripts/run.sh public-blockers --json",
		"scripts/run.sh release-notes --json --version vX.Y.Z",
		"scripts/run.sh release-dist --json",
		"bash scripts/install.sh --version vX.Y.Z --os darwin --arch arm64 --bin-dir /tmp/leia-bin --dry-run --json",
		"scripts/run.sh release-artifacts --dry-run --version vX.Y.Z --json",
		"scripts/run.sh release-check --json --version vX.Y.Z",
		"`blocker_count` plus kind-specific counts",
		"`release_decision_count`",
		"`open_blocker_count`",
		"`status_field`",
		"`scalar_fields`",
		"`count_fields`",
		"`collection_fields`",
		"`plan.json`",
		"`commands.log`",
		"## Release-Critical Gates",
	} {
		if !strings.Contains(release, snippet) {
			t.Fatalf("docs/release/index.md must mention %q", snippet)
		}
	}
	for _, gate := range []string{
		"Correctness",
		"Manifest Coverage",
		"Module Path Gate",
		"Shell Script Syntax",
		"Documentation References",
		"Editor Assets",
		"Performance Gate",
		"Language Conformance Surface",
		"Release Smoke",
		"CLI Experience",
		"Public Release Blockers",
		"Release Distribution",
		"Release Notes",
		"Release Artifacts",
	} {
		if !strings.Contains(release, "| "+gate+" |") {
			t.Fatalf("docs/release/index.md must document release-critical gate %q", gate)
		}
	}

	releaseNotesIndex := readFileString(t, filepath.Join(root, "docs", "release", "notes", "README.md"))
	for _, snippet := range []string{
		"`leia_vX.Y.Z_darwin_amd64.tar.gz`",
		"`leia_vX.Y.Z_darwin_arm64.tar.gz`",
		"`leia_vX.Y.Z_linux_amd64.tar.gz`",
		"`leia_vX.Y.Z_linux_arm64.tar.gz`",
		"`leia_vX.Y.Z_windows_amd64.zip`",
		"`leia_vX.Y.Z_windows_arm64.zip`",
		"`SHA256SUMS`",
	} {
		if !strings.Contains(releaseNotesIndex, snippet) {
			t.Fatalf("docs/release/notes/README.md must mention %q", snippet)
		}
	}

	decisions := readFileString(t, filepath.Join(root, "docs", "release", "decisions.md"))
	for _, snippet := range []string{
		"Public releases require explicit maintainer decisions",
		"## Required Before Public Release",
		"| Area | Decision Needed | Current Status |",
		"| License | Use Apache-2.0",
		"| Security reporting | Use GitHub private security advisories",
		"| Platform support | Test and support darwin/linux on amd64/arm64",
		"| Release channels | Publish GitHub Releases, `scripts/install.sh`, and `go install`",
		"| Artifact signing | Publish SHA256 checksums for release archives",
		"| Compatibility policy | Use a pre-1.0 compatibility policy",
		"authoritative license text",
		"GitHub private security advisories are the primary route",
		"Initial supported combinations are `darwin/amd64`, `darwin/arm64`,",
		"Official initial public channels are GitHub Releases, `scripts/install.sh`,",
		"SHA256 checksums are sufficient for the initial public release",
		"checksum and signing policy",
		"Optimizations, JIT availability, typed kernels, and provider integrations are not compatibility guarantees",
		"Resolved.",
	} {
		if !strings.Contains(decisions, snippet) {
			t.Fatalf("docs/release/decisions.md must keep maintainer decision snippet %q", snippet)
		}
	}
}

func TestReleaseMatrixPublicReleaseBlockersExplainDecisionWork(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/public_release_blockers_check.sh")
	if strings.TrimSpace(out) != "public_release_blockers_check.sh: pass" {
		t.Fatalf("public release blocker audit output = %q, want pass", out)
	}
	requireResolvedOut := runCommand(t, root, 30*time.Second, "bash", "scripts/public_release_blockers_check.sh", "--require-resolved")
	if strings.TrimSpace(requireResolvedOut) != "public_release_blockers_check.sh: pass" {
		t.Fatalf("public release blocker require-resolved output = %q, want pass", requireResolvedOut)
	}
}

func TestReleaseMatrixPublicReleaseBlockersJSONIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/public_release_blockers_check.sh", "--json")
	var report struct {
		SchemaVersion        int      `json:"schema_version"`
		Status               string   `json:"status"`
		RequireResolved      bool     `json:"require_resolved"`
		BlockerCount         int      `json:"blocker_count"`
		MissingFiles         int      `json:"missing_file_count"`
		Decisions            int      `json:"release_decision_count"`
		StaleText            int      `json:"stale_text_count"`
		Unconfirmed          int      `json:"unconfirmed_policy_count"`
		MissingGuidance      int      `json:"missing_guidance_count"`
		MissingDocs          int      `json:"missing_doc_snippet_count"`
		OpenBlockers         int      `json:"open_blocker_count"`
		BlockerStatusCount   int      `json:"blocker_status_count"`
		BlockerStatuses      []string `json:"blocker_statuses"`
		BlockerStatusDetails []struct {
			Status string `json:"status"`
			Count  int    `json:"count"`
		} `json:"blocker_status_details"`
		DecisionAreaCount int      `json:"decision_area_count"`
		DecisionAreas     []string `json:"decision_areas"`
		Blockers          []string `json:"blockers"`
		BlockerDetails    []struct {
			Message        string `json:"message"`
			Kind           string `json:"kind"`
			Area           string `json:"area"`
			Action         string `json:"action"`
			DecisionStatus string `json:"decision_status"`
			Path           string `json:"path"`
		} `json:"blocker_details"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("public release blocker JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.RequireResolved || report.BlockerCount != len(report.Blockers) || report.BlockerCount != len(report.BlockerDetails) {
		t.Fatalf("public release blocker JSON = %+v, want passing schema v1 report", report)
	}
	kindCounts := map[string]int{}
	statusCounts := map[string]int{}
	for _, detail := range report.BlockerDetails {
		kindCounts[detail.Kind]++
		statusCounts[detail.DecisionStatus]++
	}
	if report.MissingFiles != kindCounts["missing_file"] || report.Decisions != kindCounts["release_decision"] || report.StaleText != kindCounts["stale_text"] || report.Unconfirmed != kindCounts["unconfirmed_policy"] || report.MissingGuidance != kindCounts["missing_guidance"] || report.MissingDocs != kindCounts["missing_doc_snippet"] {
		t.Fatalf("public release blocker kind counts = missing_file:%d/%d release_decision:%d/%d stale_text:%d/%d unconfirmed:%d/%d missing_guidance:%d/%d missing_doc:%d/%d", report.MissingFiles, kindCounts["missing_file"], report.Decisions, kindCounts["release_decision"], report.StaleText, kindCounts["stale_text"], report.Unconfirmed, kindCounts["unconfirmed_policy"], report.MissingGuidance, kindCounts["missing_guidance"], report.MissingDocs, kindCounts["missing_doc_snippet"])
	}
	if report.OpenBlockers != 0 || report.OpenBlockers != statusCounts["Open"] || report.BlockerStatusCount != len(report.BlockerStatuses) || len(report.BlockerStatuses) != 0 {
		t.Fatalf("public release blocker status counts = open:%d/%d status_count:%d/%d statuses:%+v", report.OpenBlockers, statusCounts["Open"], report.BlockerStatusCount, len(report.BlockerStatuses), report.BlockerStatuses)
	}
	if report.BlockerStatusCount != len(report.BlockerStatusDetails) {
		t.Fatalf("public release blocker status details = %d/%d %+v", report.BlockerStatusCount, len(report.BlockerStatusDetails), report.BlockerStatusDetails)
	}
	for _, detail := range report.BlockerStatusDetails {
		if detail.Status == "" || detail.Count <= 0 || statusCounts[detail.Status] != detail.Count {
			t.Fatalf("public release blocker status detail = %+v actual=%+v", report.BlockerStatusDetails, statusCounts)
		}
	}
	if report.DecisionAreaCount != len(report.DecisionAreas) || report.DecisionAreaCount != 6 {
		t.Fatalf("public release blocker decision areas = %d/%d %+v, want 6 required areas", report.DecisionAreaCount, len(report.DecisionAreas), report.DecisionAreas)
	}
	for _, area := range []string{"License", "Security reporting", "Platform support", "Release channels", "Artifact signing", "Compatibility policy"} {
		if !stringSliceContains(report.DecisionAreas, area) {
			t.Fatalf("public release blocker decision areas = %+v, want %q", report.DecisionAreas, area)
		}
	}
	if report.MissingFiles != 0 || report.Decisions != 0 || report.BlockerCount != 0 || len(report.Blockers) != 0 || len(report.BlockerDetails) != 0 {
		t.Fatalf("public release blocker JSON = %+v, want resolved blockers", report)
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
	assertNestedJSONFieldPresentAndNonNull(t, out, "inspect bytecode JSON", "proto", "children")
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

func TestReleaseMatrixInspectDirectivesJSONIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "directives.leia")
	src := `//leia:cap fs.read, net.client
print("ok")
`
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "inspect", "directives", "--json", path)
	var report struct {
		SchemaVersion  int    `json:"schema_version"`
		OK             bool   `json:"ok"`
		Status         string `json:"status"`
		Source         string `json:"source"`
		DirectiveCount int    `json:"directive_count"`
		Directives     []struct {
			Kind   string   `json:"kind"`
			Args   []string `json:"args"`
			Text   string   `json:"text"`
			Line   int      `json:"line"`
			Column int      `json:"column"`
		} `json:"directives"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("inspect directives JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || !report.OK || report.Status != "pass" || report.Source != path || report.DirectiveCount != len(report.Directives) || report.DirectiveCount != 1 {
		t.Fatalf("inspect directives JSON = %+v, want passing schema v1 report", report)
	}
	directive := report.Directives[0]
	if directive.Kind != "cap" || directive.Line != 1 || directive.Column != 1 || len(directive.Args) != 2 || directive.Args[0] != "fs.read" || directive.Args[1] != "net.client" {
		t.Fatalf("inspect directive = %+v, want cap fs.read/net.client at 1:1", directive)
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

func TestReleaseMatrixReportRegistryCollectionFieldsMatchSmokeOutputs(t *testing.T) {
	root := findRepoRoot(t)
	registry := releaseReportRegistry(t, root)

	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "leia.toml"), []byte("[project]\nname = \"demo\"\nunknown = \"kept-for-json-diagnostic-smoke\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	testDir := t.TempDir()
	testPath := filepath.Join(testDir, "ok.leia")
	if err := os.WriteFile(testPath, []byte("print(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(testPath, ".leia")+".out", []byte("ok\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	lintDir := t.TempDir()
	lintPath := filepath.Join(lintDir, "warn.leia")
	if err := os.WriteFile(lintPath, []byte("xs := {1, 2}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	inspectDir := t.TempDir()
	inspectPath := filepath.Join(inspectDir, "fn.leia")
	if err := os.WriteFile(inspectPath, []byte("func add(a, b) { return a + b }\nprint(add(1, 2))\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	directivePath := filepath.Join(inspectDir, "directives.leia")
	if err := os.WriteFile(directivePath, []byte("//leia:cap fs.read, net.client\nprint(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	modDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(modDir, "leia.mod"), []byte("module github.com/example/project\nleia 0.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "main.leia"), []byte("print(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	richModDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(richModDir, "leia.mod"), []byte(`module github.com/example/project
leia 0.1
capability net.client
require example.com/lib v1.0.0
replace example.com/lib => ./lib
collection vendor ./vendor
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(richModDir, "lib"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(richModDir, "lib", "leia.mod"), []byte("module example.com/lib\nleia 0.1\ncapability fs.read\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(richModDir, "vendor"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(richModDir, "main.leia"), []byte("require(\"example.com/lib\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tidyModDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tidyModDir, "leia.mod"), []byte(`module example.com/tidy-demo
leia 0.1
require example.com/unused v1.0.0
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tidyModDir, "main.leia"), []byte("print(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	missingModDir := t.TempDir()
	invalidModDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(invalidModDir, "leia.mod"), []byte("not a module\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diagOutDir := filepath.Join(t.TempDir(), "bundle")
	remoteModDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(remoteModDir, "leia.mod"), []byte(`module example.com/remote-demo
leia 0.1
require github.com/acme/toolkit v1.2.3
`), 0o600); err != nil {
		t.Fatal(err)
	}
	remoteArchive := releaseMatrixZip(t, map[string]string{
		"toolkit-1.2.3/leia.mod":  "module github.com/acme/toolkit\nleia 0.1\n",
		"toolkit-1.2.3/main.leia": "print(\"ok\")\n",
	})
	remoteServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/acme/toolkit/archive/refs/tags/v1.2.3.zip" {
			t.Fatalf("unexpected module download path = %q", r.URL.Path)
		}
		_, _ = w.Write(remoteArchive)
	}))
	defer remoteServer.Close()
	remoteCache := filepath.Join(remoteModDir, "cache")

	for _, tc := range []struct {
		reportCommand string
		args          []string
		scalars       []string
		counts        []string
		fields        []string
		itemFields    []string
		matches       []releaseReportCountMatch
		wantFailure   bool
	}{
		{reportCommand: "leia capabilities --json", args: []string{"go", "run", "./cmd/leia", "capabilities", "--json"}, counts: []string{"command_count", "stdlib_module_count", "stdlib_layer_count", "default_import_count", "dialect_count", "tooling.report_count"}, fields: []string{"commands", "stdlib_modules", "stdlib_layers", "default_imports", "dialects", "tooling.reports"}, itemFields: []string{"tooling.reports[].command", "tooling.reports[].formats", "tooling.reports[].schema_version", "tooling.reports[].status_field"}, matches: []releaseReportCountMatch{{"command_count", "commands"}, {"stdlib_module_count", "stdlib_modules"}, {"stdlib_layer_count", "stdlib_layers"}, {"default_import_count", "default_imports"}, {"dialect_count", "dialects"}, {"tooling.report_count", "tooling.reports"}}},
		{reportCommand: "leia check --json", args: []string{"go", "run", "./cmd/leia", "check", "--json", "--quick", testDir}, counts: []string{"step_count", "failed_count", "skipped_count"}, fields: []string{"steps"}, itemFields: []string{"steps[].name", "steps[].ok", "steps[].exit_code"}, matches: []releaseReportCountMatch{{"step_count", "steps"}}},
		{reportCommand: "leia ci --list --json", args: []string{"go", "run", "./cmd/leia", "ci", "release", "--release-version", "vX.Y.Z", "--list", "--json"}, counts: []string{"command_count", "commands[].arg_count"}, fields: []string{"commands", "commands[].args"}, itemFields: []string{"commands[].name", "commands[].command", "commands[].arg_count", "commands[].args"}, matches: []releaseReportCountMatch{{"command_count", "commands"}, {"commands[].arg_count", "commands[].args"}}},
		{reportCommand: "leia config --json", args: []string{"go", "run", "./cmd/leia", "config", "--json", configDir}, counts: []string{"diagnostic_count"}, fields: []string{"diagnostics"}, itemFields: []string{"diagnostics[].severity", "diagnostics[].code", "diagnostics[].message", "diagnostics[].line"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia diag bundle --json", args: []string{"go", "run", "./cmd/leia", "diag", "bundle", "--output", diagOutDir, "--skip-go-tests", "--skip-benchmarks", "--json"}, counts: []string{"failure_count", "file_count"}, fields: []string{"failure_details", "files"}, itemFields: []string{"files[]"}, matches: []releaseReportCountMatch{{"failure_count", "failure_details"}, {"file_count", "files"}}},
		{reportCommand: "leia doc check --json", args: []string{"go", "run", "./cmd/leia", "doc", "check", "--json"}, counts: []string{"failure_count", "failure_kind_count", "counts.markdown_files", "counts.relative_documentation_links", "counts.runnable_spec_examples"}, fields: []string{"failures", "failure_kinds", "failure_details"}, matches: []releaseReportCountMatch{{"failure_count", "failures"}, {"failure_count", "failure_details"}, {"failure_kind_count", "failure_kinds"}}},
		{reportCommand: "leia doc generate --format=json", args: []string{"go", "run", "./cmd/leia", "doc", "generate", "--format=json"}, counts: []string{"cli.command_count", "stdlib.layer_count", "stdlib.default_import_count", "dialects.dialect_count"}, fields: []string{"cli.commands", "stdlib.layers", "stdlib.default_imports", "dialects.dialects"}, itemFields: []string{"cli.commands[].name", "cli.commands[].usage", "cli.commands[].summary", "stdlib.layers[].name", "stdlib.default_imports[].name", "stdlib.default_imports[].module", "stdlib.default_imports[].member", "dialects.dialects[].name", "dialects.dialects[].category", "dialects.dialects[].builtin", "dialects.dialects[].eval", "dialects.dialects[].block"}, matches: []releaseReportCountMatch{{"cli.command_count", "cli.commands"}, {"stdlib.layer_count", "stdlib.layers"}, {"stdlib.default_import_count", "stdlib.default_imports"}, {"dialects.dialect_count", "dialects.dialects"}}},
		{reportCommand: "leia env --json", args: []string{"go", "run", "./cmd/leia", "env", "--json"}, counts: []string{"capabilities.command_count", "capabilities.stdlib_module_count", "capabilities.stdlib_layer_count", "capabilities.default_import_count", "capabilities.dialect_count", "capabilities.tooling.report_count"}, fields: []string{"capabilities.commands", "capabilities.stdlib_modules", "capabilities.stdlib_layers", "capabilities.default_imports", "capabilities.dialects", "capabilities.tooling.reports"}, itemFields: []string{"capabilities.tooling.reports[].command", "capabilities.tooling.reports[].formats", "capabilities.tooling.reports[].schema_version", "capabilities.tooling.reports[].status_field"}, matches: []releaseReportCountMatch{{"capabilities.command_count", "capabilities.commands"}, {"capabilities.stdlib_module_count", "capabilities.stdlib_modules"}, {"capabilities.stdlib_layer_count", "capabilities.stdlib_layers"}, {"capabilities.default_import_count", "capabilities.default_imports"}, {"capabilities.dialect_count", "capabilities.dialects"}, {"capabilities.tooling.report_count", "capabilities.tooling.reports"}}},
		{reportCommand: "leia evaluate --json", args: []string{"go", "run", "./cmd/leia", "evaluate", "--json", "examples/evaluate/corpus_metrics.leia"}, counts: []string{"summary.files", "summary.evaluate_blocks", "summary.cases_selected", "summary.cases_passed", "summary.cases_failed", "summary.cases_listed", "summary.cases_skipped", "summary.assertions", "summary.todos", "metrics[].count"}, fields: []string{"inputs", "cases", "metrics", "findings", "notes"}, itemFields: []string{"inputs[].path", "inputs[].status", "cases[].case_id", "cases[].name", "cases[].source_path", "cases[].status", "metrics[].name", "metrics[].type", "metrics[].count"}},
		{reportCommand: "leia examples check --json", args: []string{"go", "run", "./cmd/leia", "examples", "check", "--json", "--jobs=1", "repo-evaluate-basic_assert"}, counts: []string{"result_count", "runnable", "skipped", "failed"}, fields: []string{"results"}, itemFields: []string{"results[].id", "results[].path", "results[].status", "results[].duration"}, matches: []releaseReportCountMatch{{"result_count", "results"}}},
		{reportCommand: "leia examples list --json", args: []string{"go", "run", "./cmd/leia", "examples", "list", "--json"}, counts: []string{"example_count"}, fields: []string{"examples"}, itemFields: []string{"examples[].id", "examples[].title", "examples[].section", "examples[].path", "examples[].runnable", "examples[].runner"}, matches: []releaseReportCountMatch{{"example_count", "examples"}}},
		{reportCommand: "leia fmt --json", args: []string{"go", "run", "./cmd/leia", "fmt", "--check", "--json", testPath}, counts: []string{"file_count", "changed_count", "error_count"}, fields: []string{"files"}, itemFields: []string{"files[].path", "files[].changed", "files[].written"}, matches: []releaseReportCountMatch{{"file_count", "files"}}},
		{reportCommand: "leia inspect bytecode --json", args: []string{"go", "run", "./cmd/leia", "inspect", "bytecode", "--json", inspectPath}, counts: []string{"proto_count", "proto.instruction_count", "proto.constant_count", "proto.upvalue_count", "proto.child_proto_count"}, fields: []string{"proto.children"}, itemFields: []string{"proto.children[].name", "proto.children[].display_name", "proto.children[].instruction_count", "proto.children[].child_proto_count"}},
		{reportCommand: "leia inspect directives --json", args: []string{"go", "run", "./cmd/leia", "inspect", "directives", "--json", directivePath}, counts: []string{"directive_count"}, fields: []string{"directives"}, itemFields: []string{"directives[].kind", "directives[].text", "directives[].line", "directives[].column", "directives[].args"}, matches: []releaseReportCountMatch{{"directive_count", "directives"}}},
		{reportCommand: "leia lint --json", args: []string{"go", "run", "./cmd/leia", "lint", "--json", lintPath}, counts: []string{"diagnostic_count", "error_count", "warning_count"}, fields: []string{"diagnostics"}, itemFields: []string{"diagnostics[].file", "diagnostics[].code", "diagnostics[].severity", "diagnostics[].message", "diagnostics[].line", "diagnostics[].column"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod capability --json", args: []string{"go", "run", "./cmd/leia", "mod", "capability", "--json", richModDir}, counts: []string{"capability_count", "module_count", "diagnostic_count"}, fields: []string{"capabilities", "modules", "matrix", "diagnostics"}, itemFields: []string{"modules[].path", "modules[].kind", "modules[].root", "modules[].manifest", "modules[].capabilities"}, matches: []releaseReportCountMatch{{"capability_count", "capabilities"}, {"module_count", "modules"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod check --json", args: []string{"go", "run", "./cmd/leia", "mod", "check", "--json", richModDir}, counts: []string{"diagnostic_count", "graph.file_count", "graph.diagnostic_count"}, fields: []string{"graph.files", "diagnostics"}, itemFields: []string{"graph.files[].file"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}, {"graph.file_count", "graph.files"}}},
		{reportCommand: "leia mod download --json", args: []string{"go", "run", "./cmd/leia", "mod", "download", "--json", "--cache", remoteCache, "--github-base", remoteServer.URL, remoteModDir}, counts: []string{"module_count", "diagnostic_count"}, fields: []string{"modules", "diagnostics"}, itemFields: []string{"modules[].path", "modules[].version", "modules[].repo", "modules[].url", "modules[].zip", "modules[].extract_dir", "modules[].downloaded", "modules[].extracted"}, matches: []releaseReportCountMatch{{"module_count", "modules"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod explain --json", args: []string{"go", "run", "./cmd/leia", "mod", "explain", "--json", "--dir", richModDir, "example.com/lib"}, counts: []string{"diagnostic_count"}, fields: []string{"diagnostics"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod explain --json", args: []string{"go", "run", "./cmd/leia", "mod", "explain", "--json", "--dir", missingModDir, "example.com/missing"}, counts: []string{"diagnostic_count"}, fields: []string{"diagnostics"}, itemFields: []string{"diagnostics[].severity", "diagnostics[].code", "diagnostics[].message", "diagnostics[].file"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}}, wantFailure: true},
		{reportCommand: "leia mod gomod --json", args: []string{"go", "run", "./cmd/leia", "mod", "gomod", "--json", richModDir}, counts: []string{"diagnostic_count"}, fields: []string{"diagnostics"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod gomod --json", args: []string{"go", "run", "./cmd/leia", "mod", "gomod", "--json", invalidModDir}, counts: []string{"diagnostic_count"}, fields: []string{"diagnostics"}, itemFields: []string{"diagnostics[].severity", "diagnostics[].code", "diagnostics[].message", "diagnostics[].file"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}}, wantFailure: true},
		{reportCommand: "leia mod graph --json", args: []string{"go", "run", "./cmd/leia", "mod", "graph", "--json", modDir}, counts: []string{"file_count", "diagnostic_count"}, fields: []string{"files", "diagnostics"}, itemFields: []string{"files[].file"}, matches: []releaseReportCountMatch{{"file_count", "files"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod list --json", args: []string{"go", "run", "./cmd/leia", "mod", "list", "--json", richModDir}, counts: []string{"require_count", "replace_count", "collection_count", "diagnostic_count"}, fields: []string{"requires", "replaces", "collections", "diagnostics"}, itemFields: []string{"requires[].path", "requires[].version", "requires[].kind", "requires[].source", "requires[].file", "replaces[].path", "replaces[].new_path", "replaces[].local", "replaces[].root", "collections[].name", "collections[].path", "collections[].root"}, matches: []releaseReportCountMatch{{"require_count", "requires"}, {"replace_count", "replaces"}, {"collection_count", "collections"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod lock --json", args: []string{"go", "run", "./cmd/leia", "mod", "lock", "--json", richModDir}, counts: []string{"entry_count", "diagnostic_count"}, fields: []string{"entries", "diagnostics"}, itemFields: []string{"entries[].kind", "entries[].path", "entries[].target", "entries[].hash"}, matches: []releaseReportCountMatch{{"entry_count", "entries"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod tidy --json", args: []string{"go", "run", "./cmd/leia", "mod", "tidy", "--json", "--dir", tidyModDir}, counts: []string{"removed_count", "missing_count", "diagnostic_count"}, fields: []string{"removed", "missing", "diagnostics"}, itemFields: []string{"removed[]"}, matches: []releaseReportCountMatch{{"removed_count", "removed"}, {"missing_count", "missing"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod vendor --json", args: []string{"go", "run", "./cmd/leia", "mod", "vendor", "--json", "--cache", remoteCache, remoteModDir}, counts: []string{"module_count", "diagnostic_count"}, fields: []string{"modules", "diagnostics"}, itemFields: []string{"modules[].path", "modules[].version", "modules[].source", "modules[].target"}, matches: []releaseReportCountMatch{{"module_count", "modules"}, {"diagnostic_count", "diagnostics"}}},
		{reportCommand: "leia mod verify --json", args: []string{"go", "run", "./cmd/leia", "mod", "verify", "--json", modDir}, counts: []string{"diagnostic_count", "graph.file_count", "graph.diagnostic_count"}, fields: []string{"graph.files", "diagnostics"}, itemFields: []string{"graph.files[].file"}, matches: []releaseReportCountMatch{{"diagnostic_count", "diagnostics"}, {"graph.file_count", "graph.files"}}},
		{reportCommand: "leia test --json", args: []string{"go", "run", "./cmd/leia", "test", "--json", testDir}, counts: []string{"total", "passed", "failed"}, fields: []string{"files"}, itemFields: []string{"files[].file", "files[].ok"}},
		{reportCommand: "leia test --list --json", args: []string{"go", "run", "./cmd/leia", "test", "--list", "--json", testDir}, counts: []string{"file_count"}, fields: []string{"files"}, itemFields: []string{"files[]"}, matches: []releaseReportCountMatch{{"file_count", "files"}}},
		{reportCommand: "leia version --json", args: []string{"go", "run", "./cmd/leia", "version", "--json"}},
	} {
		assertReleaseReportRegistrySmoke(t, root, registry, releaseReportSmokeCase(tc))
	}
}

func TestReleaseMatrixScriptReportRegistryFieldsMatchSmokeOutputs(t *testing.T) {
	root := findRepoRoot(t)
	registry := releaseReportRegistry(t, root)
	installBinDir := filepath.Join(t.TempDir(), "bin")
	diagScriptOutDir := filepath.Join(t.TempDir(), "script-bundle")
	snapshotDistDir := filepath.Join(t.TempDir(), "snapshot-dist")
	if err := os.MkdirAll(snapshotDistDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshotDistDir, "leia_0.0.1-next_linux_amd64.tar.gz"), releaseMatrixTarGz(t, map[string]string{
		"leia":     "#!/bin/sh\nexit 0\n",
		"leia-lsp": "#!/bin/sh\nexit 0\n",
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	siteDir := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(filepath.Join(siteDir, "guide"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "style.css"), []byte("body{font-family:sans-serif}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(`<!doctype html><html><head><link rel="stylesheet" href="/style.css"></head><body><h1 id="top">Leia</h1><a href="/guide/">Guide</a><a href="#top">Top</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "guide", "index.html"), []byte(`<!doctype html><html><body><h1 id="intro">Guide</h1><a href="/index.html#top">Home</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	perfDir := t.TempDir()
	perfTimingJSON := filepath.Join(perfDir, "timing.json")
	if err := os.WriteFile(perfTimingJSON, []byte(`{
  "modes": ["default"],
  "results": [
    {
      "group": "control",
      "benchmark": "sieve",
      "modes": {
        "default": {
          "current": {"status": "ok", "seconds": 0.018, "source": "script_repeat", "stats": {"median": 0.018, "cv_pct": 0}},
          "head": {"status": "ok", "seconds": 0.018, "source": "script_repeat", "stats": {"median": 0.018, "cv_pct": 0}}
        }
      }
    }
  ]
}
`), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		reportCommand string
		args          []string
		scalars       []string
		counts        []string
		fields        []string
		itemFields    []string
		matches       []releaseReportCountMatch
		wantFailure   bool
	}{
		{reportCommand: "scripts/run.sh arch --json", args: []string{"scripts/run.sh", "arch", "--json"}, scalars: []string{"module"}, counts: []string{"source_file_count", "source_line_count", "test_file_count", "test_line_count", "test_ratio_pct", "top_file_count", "large_file_count", "pass_pipeline_line_count", "tiering_manager_mention_count", "debt_marker_count", "missing_test_count", "same_name_test_gap_count"}, fields: []string{"top_file_details", "large_file_details", "pass_pipeline_lines", "tiering_manager_mentions", "debt_marker_details", "missing_test_files"}, matches: []releaseReportCountMatch{{"top_file_count", "top_file_details"}, {"large_file_count", "large_file_details"}, {"pass_pipeline_line_count", "pass_pipeline_lines"}, {"tiering_manager_mention_count", "tiering_manager_mentions"}, {"debt_marker_count", "debt_marker_details"}, {"missing_test_count", "missing_test_files"}, {"same_name_test_gap_count", "missing_test_files"}}},
		{reportCommand: "scripts/run.sh diagnostics --json", args: []string{"scripts/run.sh", "diagnostics", "--output", diagScriptOutDir, "--skip-go-tests", "--skip-benchmarks", "--json"}, scalars: []string{"output_dir"}, counts: []string{"failure_count", "file_count"}, fields: []string{"failure_details", "files"}, itemFields: []string{"files[]"}, matches: []releaseReportCountMatch{{"failure_count", "failure_details"}, {"file_count", "files"}}},
		{reportCommand: "scripts/run.sh docs --json", args: []string{"scripts/run.sh", "docs", "--json"}, counts: []string{"failure_count", "failure_kind_count", "counts.markdown_files", "counts.relative_documentation_links", "counts.runnable_spec_examples"}, fields: []string{"failures", "failure_kinds", "failure_details"}, matches: []releaseReportCountMatch{{"failure_count", "failures"}, {"failure_count", "failure_details"}, {"failure_kind_count", "failure_kinds"}}},
		{reportCommand: "scripts/run.sh editor --json", args: []string{"scripts/run.sh", "editor", "--json"}, scalars: []string{"require_tree_sitter", "tree_sitter_status", "tree_sitter_command", "emacs_status", "emacs_command"}, counts: []string{"failure_kind_count", "failure_count", "textmate_grammar_count", "vscode_asset_count", "tree_sitter_asset_count", "smoke_test_count"}, fields: []string{"failure_kinds", "failure_details", "textmate_grammars", "vscode_assets", "tree_sitter_assets", "smoke_tests"}, itemFields: []string{"textmate_grammars[]", "vscode_assets[]", "tree_sitter_assets[]", "smoke_tests[]"}, matches: []releaseReportCountMatch{{"failure_kind_count", "failure_kinds"}, {"failure_count", "failure_details"}, {"textmate_grammar_count", "textmate_grammars"}, {"vscode_asset_count", "vscode_assets"}, {"tree_sitter_asset_count", "tree_sitter_assets"}, {"smoke_test_count", "smoke_tests"}}},
		{reportCommand: "scripts/install.sh --dry-run --json", args: []string{"bash", "scripts/install.sh", "--dry-run", "--version", "v1.2.3-rc.1", "--os", "darwin", "--arch", "arm64", "--bin-dir", installBinDir, "--json"}, scalars: []string{"dry_run", "verify", "repo", "version", "goos", "goarch", "archive_ext", "asset", "url", "checksums", "bin_dir", "binary", "lsp_binary", "install_path", "lsp_install_path"}, counts: []string{"install_count", "binary_count", "install_path_count"}, fields: []string{"binaries", "install_paths", "install_entries"}, itemFields: []string{"install_entries[].role", "install_entries[].name", "install_entries[].path"}, matches: []releaseReportCountMatch{{"install_count", "install_entries"}, {"binary_count", "binaries"}, {"install_path_count", "install_paths"}}},
		{reportCommand: "scripts/run.sh perf --validate-only FILE --json", args: []string{"scripts/run.sh", "perf", "--validate-only", perfTimingJSON, "--no-luajit", "--json"}, scalars: []string{"validate_only", "timing_json", "validate_target.path", "validate_target.exists", "validate_target.is_file", "no_luajit", "threshold", "wall_threshold", "luajit_threshold"}, counts: []string{"validate_target.size_bytes", "failure_count", "failure_kind_count", "output_line_count"}, fields: []string{"failure_kinds", "failures", "failure_details", "output_lines"}, itemFields: []string{"output_lines[]"}, matches: []releaseReportCountMatch{{"failure_count", "failures"}, {"failure_count", "failure_details"}, {"failure_kind_count", "failure_kinds"}, {"output_line_count", "output_lines"}}},
		{reportCommand: "scripts/run.sh production --list --json", args: []string{"scripts/run.sh", "production", "--quick", "--list", "--json"}, scalars: []string{"mode", "release_profile", "release_version", "output_dir", "list_only"}, counts: []string{"run_count", "skip_count", "release_critical_run_count", "critical_skip_count", "release_critical_skip_name_count"}, fields: []string{"runnable_checks", "skipped_checks", "skipped_check_details", "release_critical_runs", "release_critical_skip_names", "release_critical_skips", "release_critical_skip_details"}, itemFields: []string{"runnable_checks[].name", "runnable_checks[].command", "runnable_checks[].release_critical"}, matches: []releaseReportCountMatch{{"run_count", "runnable_checks"}, {"skip_count", "skipped_checks"}, {"skip_count", "skipped_check_details"}, {"release_critical_run_count", "release_critical_runs"}, {"critical_skip_count", "release_critical_skips"}, {"critical_skip_count", "release_critical_skip_details"}, {"release_critical_skip_name_count", "release_critical_skip_names"}}},
		{reportCommand: "scripts/run.sh public-blockers --json", args: []string{"scripts/run.sh", "public-blockers", "--json"}, scalars: []string{"require_resolved"}, counts: []string{"blocker_count", "missing_file_count", "release_decision_count", "stale_text_count", "unconfirmed_policy_count", "missing_guidance_count", "missing_doc_snippet_count", "open_blocker_count", "blocker_status_count", "decision_area_count"}, fields: []string{"blockers", "blocker_details", "blocker_statuses", "blocker_status_details", "decision_areas"}, matches: []releaseReportCountMatch{{"blocker_count", "blockers"}, {"blocker_count", "blocker_details"}, {"blocker_status_count", "blocker_statuses"}, {"blocker_status_count", "blocker_status_details"}, {"decision_area_count", "decision_areas"}}},
		{reportCommand: "scripts/run.sh release-artifacts --dry-run --json", args: []string{"scripts/run.sh", "release-artifacts", "--dry-run", "--version", "v1.2.3-rc.1", "--json"}, scalars: []string{"dry_run", "output_dir", "version", "goos", "goarch", "git_commit", "git_branch", "git_dirty"}, counts: []string{"artifact_count", "checksum_entry_count"}, fields: []string{"artifact_files", "artifact_entries"}, matches: []releaseReportCountMatch{{"artifact_count", "artifact_files"}, {"artifact_count", "artifact_entries"}}},
		{reportCommand: "scripts/run.sh release-check --json", args: []string{"scripts/run.sh", "release-check", "--json", "--version", "v1.2.3-rc.1"}, scalars: []string{"version", "build", "require_clean", "require_tag", "goos", "goarch", "dry_run_verified", "build_verified", "install_archive_verified", "output_dir"}, counts: []string{"artifact_count", "checksum_entry_count", "install_archive_checksum_count", "failure_kind_count", "failure_count"}, fields: []string{"artifact_files", "artifact_entries", "failure_kinds", "failure_details"}, matches: []releaseReportCountMatch{{"artifact_count", "artifact_files"}, {"artifact_count", "artifact_entries"}, {"failure_kind_count", "failure_kinds"}, {"failure_count", "failure_details"}}},
		{reportCommand: "scripts/run.sh release-dist --json", args: []string{"scripts/run.sh", "release-dist", "--json"}, scalars: []string{"require_goreleaser", "require_workflows", "goreleaser_available", "local_install_fixture"}, counts: []string{"failure_kind_count", "failure_count", "workflow_count", "install_target_count"}, fields: []string{"failure_kinds", "failure_details", "workflow_files", "install_targets", "install_target_details"}, itemFields: []string{"install_target_details[].target", "install_target_details[].goos", "install_target_details[].goarch"}, matches: []releaseReportCountMatch{{"failure_kind_count", "failure_kinds"}, {"failure_count", "failure_details"}, {"workflow_count", "workflow_files"}, {"install_target_count", "install_targets"}, {"install_target_count", "install_target_details"}}},
		{reportCommand: "scripts/run.sh release-notes --json", args: []string{"scripts/run.sh", "release-notes", "--json"}, scalars: []string{"require_ready", "version"}, counts: []string{"checked_file_count", "required_artifact_count", "artifact_checksum_count", "failure_kind_count", "failure_count"}, fields: []string{"checked_files", "checked_file_details", "required_artifact_details", "failure_kinds", "failures", "failure_details"}, itemFields: []string{"checked_file_details[].path", "checked_file_details[].role", "checked_file_details[].required", "checked_file_details[].exists"}, matches: []releaseReportCountMatch{{"checked_file_count", "checked_files"}, {"checked_file_count", "checked_file_details"}, {"required_artifact_count", "required_artifact_details"}, {"failure_kind_count", "failure_kinds"}, {"failure_count", "failures"}, {"failure_count", "failure_details"}}},
		{reportCommand: "scripts/run.sh release-snapshot --dist-dir DIST_DIR --bin-dir BIN_DIR --json", args: []string{"scripts/run.sh", "release-snapshot", "--dist-dir", snapshotDistDir, "--bin-dir", filepath.Join(t.TempDir(), "snapshot-bin"), "--os", "linux", "--arch", "amd64", "--json"}, scalars: []string{"dist_dir", "goos", "goarch", "archive", "archive_name", "snapshot_version", "installer_version", "staged_asset", "staged_release_dir", "bin_dir"}, counts: []string{"install_count", "failure_kind_count", "failure_count"}, fields: []string{"installed_paths", "failure_kinds", "failure_details"}, matches: []releaseReportCountMatch{{"install_count", "installed_paths"}, {"failure_kind_count", "failure_kinds"}, {"failure_count", "failure_details"}}},
		{reportCommand: "scripts/run.sh site --site-dir SITE_DIR --json", args: []string{"scripts/run.sh", "site", "--site-dir", siteDir, "--json"}, scalars: []string{"site_dir"}, counts: []string{"html_file_count", "local_link_count", "asset_ref_count", "fragment_check_count", "failure_kind_count", "failure_count"}, fields: []string{"failure_kinds", "failure_details"}, matches: []releaseReportCountMatch{{"failure_kind_count", "failure_kinds"}, {"failure_count", "failure_details"}}},
		{reportCommand: "scripts/run.sh worktree --json", args: []string{"scripts/run.sh", "worktree", "--json"}, scalars: []string{"fail_on_findings"}, counts: []string{"finding_count", "finding_status_count"}, fields: []string{"findings", "finding_statuses"}, matches: []releaseReportCountMatch{{"finding_count", "findings"}, {"finding_status_count", "finding_statuses"}}},
	} {
		assertReleaseReportRegistrySmoke(t, root, registry, releaseReportSmokeCase(tc))
	}
}

func TestReleaseMatrixCoversEveryAdvertisedReportRegistryEntry(t *testing.T) {
	root := findRepoRoot(t)
	releaseMatrix := readFileString(t, filepath.Join(root, "tests", "release_matrix_test.go"))
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "capabilities", "--json")
	var payload struct {
		Tooling struct {
			ReportCount int `json:"report_count"`
			Reports     []struct {
				Command string `json:"command"`
			} `json:"reports"`
		} `json:"tooling"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("capabilities JSON failed to decode: %v\n%s", err, out)
	}
	if payload.Tooling.ReportCount != len(payload.Tooling.Reports) {
		t.Fatalf("capabilities tooling.report_count = %d, want %d reports", payload.Tooling.ReportCount, len(payload.Tooling.Reports))
	}
	var missing []string
	for _, report := range payload.Tooling.Reports {
		if report.Command == "" {
			t.Fatal("capabilities report registry contains an empty command")
		}
		if !strings.Contains(releaseMatrix, report.Command) {
			missing = append(missing, report.Command)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("tests/release_matrix_test.go must keep smoke or schema evidence for every capabilities report registry entry; missing: %s", strings.Join(missing, ", "))
	}
}

func TestReleaseMatrixArchitectureReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/arch_check.sh", "--json")
	var report struct {
		SchemaVersion              int    `json:"schema_version"`
		Status                     string `json:"status"`
		Module                     string `json:"module"`
		SourceFileCount            int    `json:"source_file_count"`
		SourceLineCount            int    `json:"source_line_count"`
		TestFileCount              int    `json:"test_file_count"`
		TestLineCount              int    `json:"test_line_count"`
		TestRatioPct               int    `json:"test_ratio_pct"`
		TopFileCount               int    `json:"top_file_count"`
		LargeFileCount             int    `json:"large_file_count"`
		PassPipelineLineCount      int    `json:"pass_pipeline_line_count"`
		TieringManagerMentionCount int    `json:"tiering_manager_mention_count"`
		DebtMarkerCount            int    `json:"debt_marker_count"`
		MissingTestCount           int    `json:"missing_test_count"`
		SameNameTestGapCount       int    `json:"same_name_test_gap_count"`
		TopFileDetails             []struct {
			Path  string `json:"path"`
			Lines int    `json:"lines"`
		} `json:"top_file_details"`
		LargeFileDetails []struct {
			Path     string `json:"path"`
			Lines    int    `json:"lines"`
			Severity string `json:"severity"`
		} `json:"large_file_details"`
		PassPipelineLines      []string `json:"pass_pipeline_lines"`
		TieringManagerMentions []string `json:"tiering_manager_mentions"`
		DebtMarkerDetails      []struct {
			Path string `json:"path"`
			Line int    `json:"line"`
			Text string `json:"text"`
		} `json:"debt_marker_details"`
		MissingTestFiles []string `json:"missing_test_files"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("architecture JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Module != "internal/methodjit" || (report.Status != "pass" && report.Status != "issues") {
		t.Fatalf("architecture JSON = %+v, want schema v1 methodjit report", report)
	}
	if report.SourceFileCount <= 0 || report.SourceLineCount <= 0 || report.TestFileCount <= 0 || report.TestLineCount <= 0 || report.TestRatioPct < 0 {
		t.Fatalf("architecture summary counts = %+v, want positive source/test summary", report)
	}
	if report.TopFileCount != len(report.TopFileDetails) || report.LargeFileCount != len(report.LargeFileDetails) || report.PassPipelineLineCount != len(report.PassPipelineLines) || report.TieringManagerMentionCount != len(report.TieringManagerMentions) || report.PassPipelineLineCount != report.TieringManagerMentionCount || report.DebtMarkerCount != len(report.DebtMarkerDetails) || report.MissingTestCount != len(report.MissingTestFiles) || report.SameNameTestGapCount != len(report.MissingTestFiles) {
		t.Fatalf("architecture JSON count mismatch: top %d/%d large %d/%d pass %d/%d tiering %d/%d debt %d/%d missing %d/%d same-name %d/%d", report.TopFileCount, len(report.TopFileDetails), report.LargeFileCount, len(report.LargeFileDetails), report.PassPipelineLineCount, len(report.PassPipelineLines), report.TieringManagerMentionCount, len(report.TieringManagerMentions), report.DebtMarkerCount, len(report.DebtMarkerDetails), report.MissingTestCount, len(report.MissingTestFiles), report.SameNameTestGapCount, len(report.MissingTestFiles))
	}
	if report.TopFileCount == 0 || !strings.HasPrefix(report.TopFileDetails[0].Path, "internal/methodjit/") || report.TopFileDetails[0].Lines <= 0 {
		t.Fatalf("architecture top file details = %+v, want methodjit file entries", report.TopFileDetails)
	}
	for _, detail := range report.LargeFileDetails {
		if !strings.HasPrefix(detail.Path, "internal/methodjit/") || detail.Lines <= 800 || (detail.Severity != "split" && detail.Severity != "over_limit") {
			t.Fatalf("architecture large file detail = %+v, want methodjit split/over-limit entry", detail)
		}
	}
	for _, detail := range report.DebtMarkerDetails {
		if !strings.HasPrefix(detail.Path, "internal/methodjit/") || detail.Line <= 0 || detail.Text == "" {
			t.Fatalf("architecture debt marker detail = %+v, want methodjit marker entry", detail)
		}
	}
	for _, path := range report.MissingTestFiles {
		if !strings.HasPrefix(path, "internal/methodjit/") || !strings.HasSuffix(path, ".go") {
			t.Fatalf("architecture missing test path = %q, want methodjit Go source", path)
		}
	}
}

func TestReleaseMatrixSiteReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	siteDir := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(filepath.Join(siteDir, "guide"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "style.css"), []byte("body{font-family:sans-serif}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(`<!doctype html><html><head><link rel="stylesheet" href="/style.css"></head><body><h1 id="top">Leia</h1><a href="/guide/">Guide</a><a href="#top">Top</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "guide", "index.html"), []byte(`<!doctype html><html><body><h1 id="intro">Guide</h1><a href="/index.html#top">Home</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/site_check.sh", "--site-dir", siteDir, "--json")
	var passReport struct {
		SchemaVersion      int      `json:"schema_version"`
		Status             string   `json:"status"`
		SiteDir            string   `json:"site_dir"`
		HTMLFileCount      int      `json:"html_file_count"`
		LocalLinkCount     int      `json:"local_link_count"`
		AssetRefCount      int      `json:"asset_ref_count"`
		FragmentCheckCount int      `json:"fragment_check_count"`
		FailureKindCount   int      `json:"failure_kind_count"`
		FailureCount       int      `json:"failure_count"`
		FailureKinds       []string `json:"failure_kinds"`
		FailureDetails     []struct {
			Kind      string `json:"kind"`
			Path      string `json:"path"`
			Attribute string `json:"attribute"`
			Value     string `json:"value"`
			Target    string `json:"target"`
			Fragment  string `json:"fragment"`
			Message   string `json:"message"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(out), &passReport); err != nil {
		t.Fatalf("site JSON failed to decode: %v\n%s", err, out)
	}
	if passReport.SchemaVersion != 1 || passReport.Status != "pass" || passReport.HTMLFileCount != 2 || passReport.LocalLinkCount != 3 || passReport.AssetRefCount != 1 || passReport.FragmentCheckCount != 2 || passReport.FailureKindCount != 0 || passReport.FailureCount != 0 || len(passReport.FailureKinds) != 0 || len(passReport.FailureDetails) != 0 {
		t.Fatalf("site JSON = %+v, want passing rendered-site report", passReport)
	}

	brokenDir := filepath.Join(t.TempDir(), "broken-site")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, "index.html"), []byte(`<!doctype html><html><body><h1 id="top">Leia</h1><a href="/missing.html">Missing</a><a href="#absent">Bad anchor</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	broken := runCommandResult(root, 30*time.Second, "bash", "scripts/site_check.sh", "--site-dir", brokenDir, "--json")
	if broken.err == nil {
		t.Fatalf("broken site unexpectedly passed:\nstdout:\n%s\nstderr:\n%s", broken.stdout, broken.stderr)
	}
	var brokenReport struct {
		Status           string   `json:"status"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureCount     int      `json:"failure_count"`
		FailureKinds     []string `json:"failure_kinds"`
		FailureDetails   []struct {
			Kind     string `json:"kind"`
			Path     string `json:"path"`
			Value    string `json:"value"`
			Target   string `json:"target"`
			Fragment string `json:"fragment"`
			Message  string `json:"message"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(broken.stdout), &brokenReport); err != nil {
		t.Fatalf("broken site JSON failed to decode: %v\nstdout:\n%s\nstderr:\n%s", err, broken.stdout, broken.stderr)
	}
	if brokenReport.Status != "issues" || brokenReport.FailureKindCount != len(brokenReport.FailureKinds) || brokenReport.FailureCount != len(brokenReport.FailureDetails) || brokenReport.FailureCount != 2 {
		t.Fatalf("broken site JSON = %+v, want two structured failures", brokenReport)
	}
	if !stringSliceContains(brokenReport.FailureKinds, "missing_target") || !stringSliceContains(brokenReport.FailureKinds, "missing_anchor") {
		t.Fatalf("broken site failure kinds = %+v, want missing target and anchor", brokenReport.FailureKinds)
	}
}

func TestReleaseMatrixSnapshotInstallReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	distDir := filepath.Join(t.TempDir(), "dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(distDir, "leia_0.0.1-next_linux_amd64.tar.gz"), releaseMatrixTarGz(t, map[string]string{
		"leia":     "#!/bin/sh\nexit 0\n",
		"leia-lsp": "#!/bin/sh\nexit 0\n",
	}), 0o600); err != nil {
		t.Fatal(err)
	}
	binDir := filepath.Join(t.TempDir(), "bin")
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_snapshot_install_check.sh", "--dist-dir", distDir, "--bin-dir", binDir, "--os", "linux", "--arch", "amd64", "--json")
	var report struct {
		SchemaVersion    int      `json:"schema_version"`
		Status           string   `json:"status"`
		DistDir          string   `json:"dist_dir"`
		GOOS             string   `json:"goos"`
		GOARCH           string   `json:"goarch"`
		Archive          string   `json:"archive"`
		ArchiveName      string   `json:"archive_name"`
		SnapshotVersion  string   `json:"snapshot_version"`
		InstallerVersion string   `json:"installer_version"`
		StagedAsset      string   `json:"staged_asset"`
		StagedReleaseDir string   `json:"staged_release_dir"`
		BinDir           string   `json:"bin_dir"`
		InstallCount     int      `json:"install_count"`
		InstalledPaths   []string `json:"installed_paths"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureCount     int      `json:"failure_count"`
		FailureKinds     []string `json:"failure_kinds"`
		FailureDetails   []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("snapshot install JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.GOOS != "linux" || report.GOARCH != "amd64" || report.ArchiveName != "leia_0.0.1-next_linux_amd64.tar.gz" || report.SnapshotVersion != "0.0.1-next" || report.InstallerVersion != "v0.0.1-next" || report.StagedAsset != "leia_v0.0.1-next_linux_amd64.tar.gz" || report.InstallCount != 2 || len(report.InstalledPaths) != 2 || report.FailureKindCount != 0 || report.FailureCount != 0 || len(report.FailureKinds) != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("snapshot install JSON = %+v, want passing staged installer report", report)
	}
	for _, path := range report.InstalledPaths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("installed path %q is missing: %v", path, err)
		}
		if info.Mode()&0o111 == 0 {
			t.Fatalf("installed path %q is not executable: %v", path, info.Mode())
		}
	}

	missing := runCommandResult(root, 30*time.Second, "bash", "scripts/release_snapshot_install_check.sh", "--dist-dir", filepath.Join(t.TempDir(), "missing-dist"), "--os", "linux", "--arch", "amd64", "--json")
	if missing.err == nil {
		t.Fatalf("missing snapshot dist unexpectedly passed:\nstdout:\n%s\nstderr:\n%s", missing.stdout, missing.stderr)
	}
	var missingReport struct {
		Status           string   `json:"status"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureCount     int      `json:"failure_count"`
		FailureKinds     []string `json:"failure_kinds"`
		FailureDetails   []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(missing.stdout), &missingReport); err != nil {
		t.Fatalf("missing snapshot install JSON failed to decode: %v\nstdout:\n%s\nstderr:\n%s", err, missing.stdout, missing.stderr)
	}
	if missingReport.Status != "fail" || missingReport.FailureKindCount != 1 || missingReport.FailureCount != 1 || !stringSliceContains(missingReport.FailureKinds, "missing_dist_dir") || len(missingReport.FailureDetails) != 1 || missingReport.FailureDetails[0].Kind != "missing_dist_dir" {
		t.Fatalf("missing snapshot install JSON = %+v, want missing_dist_dir failure", missingReport)
	}
}

func TestReleaseMatrixReleaseNotesReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--json")
	var passReport struct {
		SchemaVersion         int      `json:"schema_version"`
		Status                string   `json:"status"`
		RequireReady          bool     `json:"require_ready"`
		Version               string   `json:"version"`
		CheckedFileCount      int      `json:"checked_file_count"`
		RequiredArtifactCount int      `json:"required_artifact_count"`
		ArtifactChecksumCount int      `json:"artifact_checksum_count"`
		FailureKindCount      int      `json:"failure_kind_count"`
		CheckedFiles          []string `json:"checked_files"`
		CheckedFileDetails    []struct {
			Path     string `json:"path"`
			Role     string `json:"role"`
			Required bool   `json:"required"`
			Exists   bool   `json:"exists"`
		} `json:"checked_file_details"`
		RequiredArtifactDetails []struct {
			Path            string `json:"path"`
			Artifact        string `json:"artifact"`
			ChecksumPresent bool   `json:"checksum_present"`
		} `json:"required_artifact_details"`
		FailureKinds   []string `json:"failure_kinds"`
		FailureCount   int      `json:"failure_count"`
		Failures       []string `json:"failures"`
		FailureDetails []struct {
			Message string `json:"message"`
			Kind    string `json:"kind"`
			Path    string `json:"path"`
			Value   string `json:"value"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(out), &passReport); err != nil {
		t.Fatalf("release notes JSON failed to decode: %v\n%s", err, out)
	}
	if passReport.SchemaVersion != 1 || passReport.Status != "pass" || passReport.RequireReady || passReport.Version != "" || passReport.CheckedFileCount != 1 || passReport.RequiredArtifactCount != 0 || passReport.ArtifactChecksumCount != 0 || passReport.FailureKindCount != 0 || len(passReport.CheckedFiles) != 1 || passReport.CheckedFiles[0] != "docs/release/notes-template.md" || len(passReport.CheckedFileDetails) != 1 || passReport.CheckedFileDetails[0].Path != "docs/release/notes-template.md" || passReport.CheckedFileDetails[0].Role != "template" || !passReport.CheckedFileDetails[0].Required || !passReport.CheckedFileDetails[0].Exists || len(passReport.RequiredArtifactDetails) != 0 || len(passReport.FailureKinds) != 0 || passReport.FailureCount != 0 || len(passReport.Failures) != 0 || len(passReport.FailureDetails) != 0 {
		t.Fatalf("release notes template JSON = %+v, want passing schema v1 report", passReport)
	}

	missingOut := runCommand(t, root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--json", "--version", "v9.9.9")
	var missingReport struct {
		SchemaVersion         int      `json:"schema_version"`
		Status                string   `json:"status"`
		RequireReady          bool     `json:"require_ready"`
		Version               string   `json:"version"`
		CheckedFileCount      int      `json:"checked_file_count"`
		RequiredArtifactCount int      `json:"required_artifact_count"`
		ArtifactChecksumCount int      `json:"artifact_checksum_count"`
		FailureKindCount      int      `json:"failure_kind_count"`
		CheckedFiles          []string `json:"checked_files"`
		CheckedFileDetails    []struct {
			Path     string `json:"path"`
			Role     string `json:"role"`
			Required bool   `json:"required"`
			Exists   bool   `json:"exists"`
		} `json:"checked_file_details"`
		RequiredArtifactDetails []struct {
			Path            string `json:"path"`
			Artifact        string `json:"artifact"`
			ChecksumPresent bool   `json:"checksum_present"`
		} `json:"required_artifact_details"`
		FailureKinds   []string `json:"failure_kinds"`
		FailureCount   int      `json:"failure_count"`
		Failures       []string `json:"failures"`
		FailureDetails []struct {
			Message string `json:"message"`
			Kind    string `json:"kind"`
			Path    string `json:"path"`
			Value   string `json:"value"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(missingOut), &missingReport); err != nil {
		t.Fatalf("missing release notes JSON failed to decode: %v\n%s", err, missingOut)
	}
	if missingReport.SchemaVersion != 1 || missingReport.Status != "issues" || missingReport.RequireReady || missingReport.Version != "v9.9.9" || missingReport.CheckedFileCount != len(missingReport.CheckedFiles) || missingReport.CheckedFileCount != len(missingReport.CheckedFileDetails) || missingReport.CheckedFileCount != 2 || missingReport.RequiredArtifactCount != 0 || len(missingReport.RequiredArtifactDetails) != 0 || missingReport.ArtifactChecksumCount != 0 || missingReport.FailureKindCount != len(missingReport.FailureKinds) || missingReport.FailureKindCount != 1 || missingReport.FailureCount != len(missingReport.Failures) || missingReport.FailureCount != len(missingReport.FailureDetails) {
		t.Fatalf("missing release notes JSON = %+v, want issues schema v1 report", missingReport)
	}
	if !stringSliceContains(missingReport.CheckedFiles, "docs/release/notes-template.md") || !stringSliceContains(missingReport.CheckedFiles, "docs/release/notes/v9.9.9.md") {
		t.Fatalf("missing release notes JSON checked files = %+v, want template and version notes", missingReport.CheckedFiles)
	}
	checkedDetails := map[string]struct {
		Role     string
		Required bool
		Exists   bool
	}{}
	for _, detail := range missingReport.CheckedFileDetails {
		checkedDetails[detail.Path] = struct {
			Role     string
			Required bool
			Exists   bool
		}{Role: detail.Role, Required: detail.Required, Exists: detail.Exists}
	}
	if checkedDetails["docs/release/notes-template.md"].Role != "template" || !checkedDetails["docs/release/notes-template.md"].Required || !checkedDetails["docs/release/notes-template.md"].Exists {
		t.Fatalf("missing release notes template detail = %+v", checkedDetails["docs/release/notes-template.md"])
	}
	if checkedDetails["docs/release/notes/v9.9.9.md"].Role != "version_notes" || !checkedDetails["docs/release/notes/v9.9.9.md"].Required || checkedDetails["docs/release/notes/v9.9.9.md"].Exists {
		t.Fatalf("missing release notes version detail = %+v", checkedDetails["docs/release/notes/v9.9.9.md"])
	}
	if !stringSliceContains(missingReport.Failures, "missing release notes for v9.9.9: docs/release/notes/v9.9.9.md") {
		t.Fatalf("missing release notes JSON missing actionable failure: %+v", missingReport.Failures)
	}
	if !stringSliceContains(missingReport.FailureKinds, "missing_file") || len(missingReport.FailureDetails) != 1 || missingReport.FailureDetails[0].Kind != "missing_file" || missingReport.FailureDetails[0].Path != "docs/release/notes/v9.9.9.md" || missingReport.FailureDetails[0].Value != "v9.9.9" {
		t.Fatalf("missing release notes details = %+v, want missing_file detail", missingReport)
	}

	incompleteVersion := "v9.9.8"
	incompleteNotes := filepath.Join(root, "docs", "release", "notes", incompleteVersion+".md")
	incompleteContent := fmt.Sprintf(`# Leia %[1]s

## Validation

scripts/run.sh release-check --build --require-clean --require-tag --version %[1]s

## Known Issues

None known

## Checksums And Artifacts

| Artifact | SHA256 |
|---|---|
| leia_%[1]s_darwin_amd64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_darwin_arm64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_linux_amd64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_linux_arm64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_windows_amd64.zip | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_windows_arm64.zip | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| SHA256SUMS | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |

Each archive includes leia and leia-lsp.

## Release Decisions

- License: pending project decision
- Security reporting: pending project decision
- Platform support:
- Release channels: pending project decision
- Artifact signing: pending project decision
- Compatibility policy: pending project decision
`, incompleteVersion)
	if err := os.WriteFile(incompleteNotes, []byte(incompleteContent), 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(incompleteNotes); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove temporary release notes: %v", err)
		}
	}()
	incompleteOut := runCommandResult(root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--json", "--require-ready", "--version", incompleteVersion)
	if incompleteOut.err == nil {
		t.Fatalf("release notes with empty Platform support unexpectedly passed:\nstdout:\n%s\nstderr:\n%s", incompleteOut.stdout, incompleteOut.stderr)
	}
	var incompleteReport struct {
		Status         string   `json:"status"`
		Version        string   `json:"version"`
		FailureKinds   []string `json:"failure_kinds"`
		Failures       []string `json:"failures"`
		FailureDetails []struct {
			Message string `json:"message"`
			Kind    string `json:"kind"`
			Path    string `json:"path"`
			Value   string `json:"value"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(incompleteOut.stdout), &incompleteReport); err != nil {
		t.Fatalf("incomplete release notes JSON failed to decode: %v\n%s", err, incompleteOut.stdout)
	}
	if incompleteReport.Status != "issues" || incompleteReport.Version != incompleteVersion || !stringSliceContains(incompleteReport.Failures, "docs/release/notes/"+incompleteVersion+".md still contains template placeholder: - Platform support:") {
		t.Fatalf("incomplete release notes report = %+v, want Platform support placeholder failure", incompleteReport)
	}
	if !stringSliceContains(incompleteReport.FailureKinds, "placeholder") || len(incompleteReport.FailureDetails) == 0 || incompleteReport.FailureDetails[0].Kind != "placeholder" || incompleteReport.FailureDetails[0].Path != "docs/release/notes/"+incompleteVersion+".md" {
		t.Fatalf("incomplete release notes failure details = %+v, want placeholder detail", incompleteReport)
	}

	badChecksumVersion := "v9.9.7"
	badChecksumNotes := filepath.Join(root, "docs", "release", "notes", badChecksumVersion+".md")
	badChecksumContent := fmt.Sprintf(`# Leia %[1]s

## Validation

scripts/run.sh release-check --build --require-clean --require-tag --version %[1]s

## Known Issues

None known

## Checksums And Artifacts

| Artifact | SHA256 |
|---|---|
| leia_%[1]s_darwin_amd64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_darwin_arm64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_linux_amd64.tar.gz | missing |
| leia_%[1]s_linux_arm64.tar.gz | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_windows_amd64.zip | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| leia_%[1]s_windows_arm64.zip | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |
| SHA256SUMS | 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef |

Each archive includes leia and leia-lsp.

## Release Decisions

- License: pending project decision
- Security reporting: pending project decision
- Platform support: pending project decision
- Release channels: pending project decision
- Artifact signing: pending project decision
- Compatibility policy: pending project decision
`, badChecksumVersion)
	if err := os.WriteFile(badChecksumNotes, []byte(badChecksumContent), 0o600); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(badChecksumNotes); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove temporary checksum release notes: %v", err)
		}
	}()
	badChecksumOut := runCommandResult(root, 30*time.Second, "bash", "scripts/release_notes_check.sh", "--json", "--require-ready", "--version", badChecksumVersion)
	if badChecksumOut.err == nil {
		t.Fatalf("release notes with missing archive checksum unexpectedly passed:\nstdout:\n%s\nstderr:\n%s", badChecksumOut.stdout, badChecksumOut.stderr)
	}
	var badChecksumReport struct {
		Status                  string `json:"status"`
		Version                 string `json:"version"`
		RequiredArtifactCount   int    `json:"required_artifact_count"`
		ArtifactChecksumCount   int    `json:"artifact_checksum_count"`
		RequiredArtifactDetails []struct {
			Path            string `json:"path"`
			Artifact        string `json:"artifact"`
			ChecksumPresent bool   `json:"checksum_present"`
		} `json:"required_artifact_details"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureKinds     []string `json:"failure_kinds"`
		Failures         []string `json:"failures"`
		FailureDetails   []struct {
			Message string `json:"message"`
			Kind    string `json:"kind"`
			Path    string `json:"path"`
			Value   string `json:"value"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(badChecksumOut.stdout), &badChecksumReport); err != nil {
		t.Fatalf("bad-checksum release notes JSON failed to decode: %v\n%s", err, badChecksumOut.stdout)
	}
	wantChecksumFailure := "docs/release/notes/" + badChecksumVersion + ".md must include a 64-hex SHA256 checksum for leia_" + badChecksumVersion + "_linux_amd64.tar.gz"
	if badChecksumReport.Status != "issues" || badChecksumReport.Version != badChecksumVersion || !stringSliceContains(badChecksumReport.Failures, wantChecksumFailure) {
		t.Fatalf("bad-checksum release notes report = %+v, want %q", badChecksumReport, wantChecksumFailure)
	}
	if badChecksumReport.RequiredArtifactCount != 7 || badChecksumReport.ArtifactChecksumCount != 6 {
		t.Fatalf("bad-checksum artifact counts = %d/%d, want 6 valid checksums across 7 required artifacts", badChecksumReport.ArtifactChecksumCount, badChecksumReport.RequiredArtifactCount)
	}
	if badChecksumReport.RequiredArtifactCount != len(badChecksumReport.RequiredArtifactDetails) {
		t.Fatalf("bad-checksum required artifact details = %d/%d", badChecksumReport.RequiredArtifactCount, len(badChecksumReport.RequiredArtifactDetails))
	}
	var checksumPresentCount int
	var foundMissingArtifactDetail bool
	for _, detail := range badChecksumReport.RequiredArtifactDetails {
		if detail.ChecksumPresent {
			checksumPresentCount++
		}
		if detail.Path == "docs/release/notes/"+badChecksumVersion+".md" && detail.Artifact == "leia_"+badChecksumVersion+"_linux_amd64.tar.gz" && !detail.ChecksumPresent {
			foundMissingArtifactDetail = true
		}
	}
	if checksumPresentCount != badChecksumReport.ArtifactChecksumCount || !foundMissingArtifactDetail {
		t.Fatalf("bad-checksum required artifact details = %+v, want one missing linux amd64 checksum", badChecksumReport.RequiredArtifactDetails)
	}
	if badChecksumReport.FailureKindCount != len(badChecksumReport.FailureKinds) || !stringSliceContains(badChecksumReport.FailureKinds, "missing_checksum") {
		t.Fatalf("bad-checksum failure kinds = %+v, want missing_checksum", badChecksumReport)
	}
	var foundChecksumDetail bool
	for _, detail := range badChecksumReport.FailureDetails {
		if detail.Kind == "missing_checksum" && detail.Value == "leia_"+badChecksumVersion+"_linux_amd64.tar.gz" {
			foundChecksumDetail = true
			break
		}
	}
	if !foundChecksumDetail {
		t.Fatalf("bad-checksum failure details = %+v, want checksum artifact detail", badChecksumReport.FailureDetails)
	}
}

func TestReleaseMatrixReleaseDistributionReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	platformDocs := readFileString(t, filepath.Join(root, "docs", "reference", "platforms", "index.md"))
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_distribution_check.sh", "--json")
	var report struct {
		SchemaVersion       int      `json:"schema_version"`
		Status              string   `json:"status"`
		RequireGoreleaser   bool     `json:"require_goreleaser"`
		RequireWorkflows    bool     `json:"require_workflows"`
		GoreleaserAvailable bool     `json:"goreleaser_available"`
		LocalInstallFixture string   `json:"local_install_fixture"`
		FailureKindCount    int      `json:"failure_kind_count"`
		FailureKinds        []string `json:"failure_kinds"`
		FailureCount        int      `json:"failure_count"`
		FailureDetails      []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
		WorkflowCount        int      `json:"workflow_count"`
		WorkflowFiles        []string `json:"workflow_files"`
		InstallTargetCount   int      `json:"install_target_count"`
		InstallTargets       []string `json:"install_targets"`
		InstallTargetDetails []struct {
			Target string `json:"target"`
			GOOS   string `json:"goos"`
			GOARCH string `json:"goarch"`
		} `json:"install_target_details"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("release distribution JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.RequireGoreleaser || report.RequireWorkflows || report.LocalInstallFixture != "verified" {
		t.Fatalf("release distribution JSON = %+v, want passing schema v1 report", report)
	}
	if report.FailureKindCount != 0 || len(report.FailureKinds) != 0 || report.FailureCount != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("release distribution JSON failures = kinds %d/%d details %d/%d, want none", report.FailureKindCount, len(report.FailureKinds), report.FailureCount, len(report.FailureDetails))
	}
	if report.WorkflowCount != len(report.WorkflowFiles) || report.InstallTargetCount != len(report.InstallTargets) || report.InstallTargetCount != len(report.InstallTargetDetails) {
		t.Fatalf("release distribution JSON counts = workflows %d/%d targets %d/%d/%d", report.WorkflowCount, len(report.WorkflowFiles), report.InstallTargetCount, len(report.InstallTargets), len(report.InstallTargetDetails))
	}
	targetDetails := map[string]struct {
		GOOS   string
		GOARCH string
	}{}
	for _, detail := range report.InstallTargetDetails {
		targetDetails[detail.Target] = struct {
			GOOS   string
			GOARCH string
		}{GOOS: detail.GOOS, GOARCH: detail.GOARCH}
	}
	for _, target := range []string{"darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64", "windows/amd64", "windows/arm64"} {
		if !stringSliceContains(report.InstallTargets, target) {
			t.Fatalf("release distribution JSON missing install target %q: %+v", target, report.InstallTargets)
		}
		parts := strings.Split(target, "/")
		if targetDetails[target].GOOS != parts[0] || targetDetails[target].GOARCH != parts[1] {
			t.Fatalf("release distribution install target detail for %q = %+v", target, targetDetails[target])
		}
		if !strings.Contains(platformDocs, "`"+target+"`") {
			t.Fatalf("platform reference must document release distribution target %q", target)
		}
	}
	for _, workflow := range []string{".github/workflows/release.yml", ".github/workflows/distribution-check.yml", ".github/workflows/pages.yml"} {
		if !stringSliceContains(report.WorkflowFiles, workflow) {
			t.Fatalf("release distribution JSON missing workflow %q: %+v", workflow, report.WorkflowFiles)
		}
	}
	tmpBin := t.TempDir()
	for _, name := range []string{"awk", "bash", "chmod", "cp", "dirname", "env", "grep", "install", "mkdir", "mktemp", "openssl", "rm", "sh", "shasum", "sha256sum", "tar", "unzip", "zip"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if err := os.Symlink(path, filepath.Join(tmpBin, name)); err != nil && !errors.Is(err, os.ErrExist) {
			t.Fatalf("symlink %s into test PATH: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(tmpBin, "bash")); err != nil {
		t.Fatalf("test PATH needs bash: %v", err)
	}

	cmd := exec.Command("bash", "scripts/release_distribution_check.sh", "--require-goreleaser", "--json")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PATH="+tmpBin)
	failureOut, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("release_distribution_check.sh --require-goreleaser unexpectedly succeeded without go or goreleaser in PATH\noutput:\n%s", failureOut)
	}
	var failedReport struct {
		SchemaVersion       int      `json:"schema_version"`
		Status              string   `json:"status"`
		RequireGoreleaser   bool     `json:"require_goreleaser"`
		GoreleaserAvailable bool     `json:"goreleaser_available"`
		GoreleaserCheck     string   `json:"goreleaser_check"`
		GoreleaserSource    string   `json:"goreleaser_check_source"`
		FailureKindCount    int      `json:"failure_kind_count"`
		FailureKinds        []string `json:"failure_kinds"`
		FailureCount        int      `json:"failure_count"`
		FailureDetails      []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal(failureOut, &failedReport); err != nil {
		t.Fatalf("release distribution failure JSON failed to decode: %v\noutput:\n%s", err, failureOut)
	}
	if failedReport.SchemaVersion != 1 || failedReport.Status != "fail" || !failedReport.RequireGoreleaser || failedReport.GoreleaserAvailable || failedReport.GoreleaserCheck != "skipped" || failedReport.GoreleaserSource != "none" {
		t.Fatalf("release distribution failure JSON = %+v, want require-goreleaser failure report", failedReport)
	}
	if failedReport.FailureKindCount != 1 || failedReport.FailureCount != 1 || !stringSliceContains(failedReport.FailureKinds, "missing_command") || len(failedReport.FailureDetails) != 1 || failedReport.FailureDetails[0].Kind != "missing_command" || !strings.Contains(failedReport.FailureDetails[0].Message, "pinned go-run fallback") {
		t.Fatalf("release distribution failure details = %+v, want missing goreleaser detail", failedReport)
	}
}

func TestReleaseMatrixReleaseDistributionJSONStaysCleanWithoutHostedWorkflows(t *testing.T) {
	root := findRepoRoot(t)
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "scripts"), 0o755); err != nil {
		t.Fatalf("mkdir temp scripts: %v", err)
	}
	for _, path := range []string{
		".goreleaser.yaml",
		"scripts/install.sh",
		"scripts/release_distribution_check.sh",
		"scripts/release_snapshot_install_check.sh",
	} {
		src := readFileString(t, filepath.Join(root, path))
		dst := filepath.Join(tmp, path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(dst), err)
		}
		if err := os.WriteFile(dst, []byte(src), 0o755); err != nil {
			t.Fatalf("write %s: %v", dst, err)
		}
	}

	out := runCommand(t, tmp, 30*time.Second, "bash", "scripts/release_distribution_check.sh", "--json")
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Fatalf("release distribution JSON was prefixed by non-JSON output:\n%s", out)
	}
	var report struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		WorkflowCount int      `json:"workflow_count"`
		WorkflowFiles []string `json:"workflow_files"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("release distribution JSON without workflows failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.WorkflowCount != 0 || len(report.WorkflowFiles) != 0 {
		t.Fatalf("release distribution JSON without workflows = %+v, want clean pass with no workflow files", report)
	}
}

func TestReleaseMatrixDocGenerateReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "doc", "generate", "--format=json")
	var report struct {
		SchemaVersion int    `json:"schema_version"`
		Status        string `json:"status"`
		CLI           struct {
			SchemaVersion int `json:"schema_version"`
			CommandCount  int `json:"command_count"`
			Commands      []struct {
				Name    string `json:"name"`
				Usage   string `json:"usage"`
				Summary string `json:"summary"`
			} `json:"commands"`
		} `json:"cli"`
		Stdlib struct {
			SchemaVersion int `json:"schema_version"`
			LayerCount    int `json:"layer_count"`
			DefaultCount  int `json:"default_import_count"`
			Layers        []struct {
				Name string `json:"name"`
			} `json:"layers"`
			DefaultImports []struct {
				Name   string `json:"name"`
				Module string `json:"module"`
				Member string `json:"member"`
			} `json:"default_imports"`
		} `json:"stdlib"`
		Dialects struct {
			SchemaVersion int `json:"schema_version"`
			DialectCount  int `json:"dialect_count"`
			Dialects      []struct {
				Name string `json:"name"`
			} `json:"dialects"`
		} `json:"dialects"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("doc generate JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.CLI.SchemaVersion != 1 || report.Stdlib.SchemaVersion != 1 || report.Dialects.SchemaVersion != 1 {
		t.Fatalf("doc generate JSON schema versions = %+v, want schema v1 bundle", report)
	}
	if report.CLI.CommandCount != len(report.CLI.Commands) || report.CLI.CommandCount == 0 {
		t.Fatalf("doc generate CLI count = %d/%d", report.CLI.CommandCount, len(report.CLI.Commands))
	}
	if report.Stdlib.LayerCount != len(report.Stdlib.Layers) || report.Stdlib.LayerCount == 0 || report.Stdlib.DefaultCount != len(report.Stdlib.DefaultImports) || report.Stdlib.DefaultCount == 0 {
		t.Fatalf("doc generate stdlib counts = layers %d/%d defaults %d/%d", report.Stdlib.LayerCount, len(report.Stdlib.Layers), report.Stdlib.DefaultCount, len(report.Stdlib.DefaultImports))
	}
	if report.Dialects.DialectCount != len(report.Dialects.Dialects) || report.Dialects.DialectCount == 0 {
		t.Fatalf("doc generate dialect count = %d/%d", report.Dialects.DialectCount, len(report.Dialects.Dialects))
	}
}

func TestReleaseMatrixDocsCheckReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 120*time.Second, "bash", "scripts/docs_check.sh", "--json")
	var report struct {
		SchemaVersion    int      `json:"schema_version"`
		Status           string   `json:"status"`
		FailureCount     int      `json:"failure_count"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureKinds     []string `json:"failure_kinds"`
		Failures         []string `json:"failures"`
		FailureDetails   []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
		Counts struct {
			MarkdownFiles                     int `json:"markdown_files"`
			RelativeDocumentationLinks        int `json:"relative_documentation_links"`
			RepositoryScriptCodeBlockMentions int `json:"repository_script_code_block_mentions"`
			ReleaseGateDocs                   int `json:"release_gate_docs"`
			ReferenceEntrypoints              int `json:"reference_entrypoints"`
			SpecContractDocs                  int `json:"spec_contract_docs"`
			ExamplesIndexDirectories          int `json:"examples_index_directories"`
			ExamplesCapabilityDriftGates      int `json:"examples_capability_drift_gates"`
			ReadmeUserFacingGates             int `json:"readme_user_facing_gates"`
			RetiredPathMentions               int `json:"retired_path_mentions"`
			RetiredNameMentions               int `json:"retired_name_mentions"`
			GeneratedReferenceDocs            int `json:"generated_reference_docs"`
			GeneratedSpecHTML                 int `json:"generated_spec_html"`
			RunnableSpecExamples              int `json:"runnable_spec_examples"`
		} `json:"counts"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("docs check JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.FailureCount != 0 || len(report.Failures) != 0 || report.FailureKindCount != 0 || len(report.FailureKinds) != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("docs check JSON = %+v, want passing schema v1 report", report)
	}
	if report.Counts.MarkdownFiles == 0 || report.Counts.RelativeDocumentationLinks == 0 || report.Counts.RepositoryScriptCodeBlockMentions < 0 {
		t.Fatalf("docs check JSON missing core documentation counts: %+v", report.Counts)
	}
	if report.Counts.RetiredPathMentions != 0 || report.Counts.RetiredNameMentions != 0 {
		t.Fatalf("docs check JSON found retired documentation mentions: %+v", report.Counts)
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

func TestReleaseMatrixPerformanceGateReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	missingTiming := filepath.Join(t.TempDir(), "missing-timing.json")
	out := runCommandResult(root, 30*time.Second, "bash", "scripts/performance_gate.sh", "--validate-only", missingTiming, "--no-luajit", "--json")
	if out.err == nil {
		t.Fatalf("performance gate missing timing JSON unexpectedly passed:\nstdout:\n%s\nstderr:\n%s", out.stdout, out.stderr)
	}
	var report struct {
		SchemaVersion  int    `json:"schema_version"`
		Status         string `json:"status"`
		ValidateOnly   bool   `json:"validate_only"`
		TimingJSON     string `json:"timing_json"`
		ValidateTarget struct {
			Path      string `json:"path"`
			Exists    bool   `json:"exists"`
			IsFile    bool   `json:"is_file"`
			SizeBytes int    `json:"size_bytes"`
		} `json:"validate_target"`
		NoLuaJIT         bool     `json:"no_luajit"`
		Threshold        float64  `json:"threshold"`
		WallThreshold    float64  `json:"wall_threshold"`
		LuaJITThreshold  float64  `json:"luajit_threshold"`
		FailureCount     int      `json:"failure_count"`
		FailureKindCount int      `json:"failure_kind_count"`
		OutputLineCount  int      `json:"output_line_count"`
		FailureKinds     []string `json:"failure_kinds"`
		Failures         []string `json:"failures"`
		FailureDetails   []struct {
			Message string `json:"message"`
			Kind    string `json:"kind"`
			Value   string `json:"value"`
		} `json:"failure_details"`
		OutputLines []string `json:"output_lines"`
	}
	if err := json.Unmarshal([]byte(out.stdout), &report); err != nil {
		t.Fatalf("performance gate JSON failed to decode: %v\n%s", err, out.stdout)
	}
	if report.SchemaVersion != 1 || report.Status != "issues" || !report.ValidateOnly || report.TimingJSON != missingTiming || !report.NoLuaJIT {
		t.Fatalf("performance gate JSON = %+v, want issue schema v1 validate-only report", report)
	}
	if report.ValidateTarget.Path != missingTiming || report.ValidateTarget.Exists || report.ValidateTarget.IsFile || report.ValidateTarget.SizeBytes != 0 {
		t.Fatalf("performance gate validate target = %+v, want missing timing target", report.ValidateTarget)
	}
	if report.Threshold <= 0 || report.WallThreshold <= 0 || report.LuaJITThreshold <= 0 {
		t.Fatalf("performance gate thresholds missing: %+v", report)
	}
	if report.FailureCount != len(report.Failures) || report.FailureCount != len(report.FailureDetails) || report.FailureCount != 1 || report.FailureKindCount != len(report.FailureKinds) || report.FailureKindCount != 1 {
		t.Fatalf("performance gate failure counts = %+v, want one structured timing failure", report)
	}
	if !stringSliceContains(report.Failures, "timing validation failed") || !stringSliceContains(report.FailureKinds, "timing_validation") {
		t.Fatalf("performance gate failures = %+v/%+v, want timing_validation", report.Failures, report.FailureKinds)
	}
	if report.FailureDetails[0].Kind != "timing_validation" || report.FailureDetails[0].Value != missingTiming {
		t.Fatalf("performance gate failure detail = %+v, want timing JSON path", report.FailureDetails)
	}
	if report.OutputLineCount != len(report.OutputLines) || report.OutputLineCount == 0 {
		t.Fatalf("performance gate output lines = %d/%d, want captured validator output", report.OutputLineCount, len(report.OutputLines))
	}

	validTiming := filepath.Join(t.TempDir(), "timing.json")
	validTimingPayload := []byte(`{
  "modes": ["default"],
  "results": [
    {
      "group": "control",
      "benchmark": "sieve",
      "modes": {
        "default": {
          "current": {"status": "ok", "seconds": 0.018, "source": "script_repeat", "stats": {"median": 0.018, "cv_pct": 0}},
          "head": {"status": "ok", "seconds": 0.018, "source": "script_repeat", "stats": {"median": 0.018, "cv_pct": 0}}
        }
      }
    }
  ]
}
`)
	if err := os.WriteFile(validTiming, validTimingPayload, 0o600); err != nil {
		t.Fatal(err)
	}
	validOut := runCommand(t, root, 30*time.Second, "bash", "scripts/performance_gate.sh", "--validate-only", validTiming, "--no-luajit", "--json")
	var validReport struct {
		SchemaVersion  int    `json:"schema_version"`
		Status         string `json:"status"`
		ValidateTarget struct {
			Path      string `json:"path"`
			Exists    bool   `json:"exists"`
			IsFile    bool   `json:"is_file"`
			SizeBytes int    `json:"size_bytes"`
		} `json:"validate_target"`
		FailureCount int `json:"failure_count"`
	}
	if err := json.Unmarshal([]byte(validOut), &validReport); err != nil {
		t.Fatalf("performance gate valid JSON failed to decode: %v\n%s", err, validOut)
	}
	if validReport.SchemaVersion != 1 || validReport.Status != "pass" || validReport.FailureCount != 0 || validReport.ValidateTarget.Path != validTiming || !validReport.ValidateTarget.Exists || !validReport.ValidateTarget.IsFile || validReport.ValidateTarget.SizeBytes != len(validTimingPayload) {
		t.Fatalf("performance gate valid target report = %+v, want existing timing target", validReport)
	}
}

func TestReleaseMatrixReleaseArtifactReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	out := runCommand(t, root, 30*time.Second, "bash", "scripts/release_artifacts_check.sh", "--json", "--version", "v1.2.3-rc.1")
	var report struct {
		SchemaVersion               int      `json:"schema_version"`
		Status                      string   `json:"status"`
		Version                     string   `json:"version"`
		Build                       bool     `json:"build"`
		RequireClean                bool     `json:"require_clean"`
		RequireTag                  bool     `json:"require_tag"`
		GOOS                        string   `json:"goos"`
		GOARCH                      string   `json:"goarch"`
		Artifact                    string   `json:"artifact"`
		LSPArtifact                 string   `json:"lsp_artifact"`
		Metadata                    string   `json:"metadata"`
		InstallArchive              string   `json:"install_archive"`
		ArtifactCount               int      `json:"artifact_count"`
		ChecksumEntryCount          int      `json:"checksum_entry_count"`
		InstallArchiveChecksumCount int      `json:"install_archive_checksum_count"`
		FailureKindCount            int      `json:"failure_kind_count"`
		FailureKinds                []string `json:"failure_kinds"`
		FailureCount                int      `json:"failure_count"`
		FailureDetails              []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
		ArtifactFiles   []string `json:"artifact_files"`
		ArtifactEntries []struct {
			Role string `json:"role"`
			Name string `json:"name"`
			Path string `json:"path,omitempty"`
		} `json:"artifact_entries"`
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
	if report.ArtifactCount != len(report.ArtifactFiles) || report.ArtifactCount != len(report.ArtifactEntries) || report.ArtifactCount != 4 {
		t.Fatalf("release artifact JSON artifact counts = %d/%d/%d, want 4", report.ArtifactCount, len(report.ArtifactFiles), len(report.ArtifactEntries))
	}
	if report.ChecksumEntryCount != 0 || report.InstallArchiveChecksumCount != 0 {
		t.Fatalf("release artifact dry-run checksum counts = %d/%d, want 0/0", report.ChecksumEntryCount, report.InstallArchiveChecksumCount)
	}
	if report.FailureKindCount != 0 || len(report.FailureKinds) != 0 || report.FailureCount != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("release artifact dry-run failures = kinds %d/%d details %d/%d, want none", report.FailureKindCount, len(report.FailureKinds), report.FailureCount, len(report.FailureDetails))
	}
	if !stringSliceContains(report.ArtifactFiles, "SHA256SUMS") || stringSliceContains(report.ArtifactFiles, report.InstallArchive) {
		t.Fatalf("release artifact files = %+v, want SHA256SUMS and no install archive fixture", report.ArtifactFiles)
	}
	artifactEntries := map[string]string{}
	for _, entry := range report.ArtifactEntries {
		if entry.Path != "" {
			t.Fatalf("release artifact dry-run check entry %q unexpectedly has path %q", entry.Role, entry.Path)
		}
		artifactEntries[entry.Role] = entry.Name
	}
	if artifactEntries["cli"] != report.Artifact || artifactEntries["lsp"] != report.LSPArtifact || artifactEntries["metadata"] != report.Metadata || artifactEntries["checksums"] != "SHA256SUMS" {
		t.Fatalf("release artifact entries do not map roles to artifact names: %+v", report.ArtifactEntries)
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
		SchemaVersion      int      `json:"schema_version"`
		Status             string   `json:"status"`
		DryRun             bool     `json:"dry_run"`
		OutputDir          string   `json:"output_dir"`
		Version            string   `json:"version"`
		Module             string   `json:"module"`
		GOOS               string   `json:"goos"`
		GOARCH             string   `json:"goarch"`
		Artifact           string   `json:"artifact"`
		LSPArtifact        string   `json:"lsp_artifact"`
		Metadata           string   `json:"metadata"`
		Checksums          string   `json:"checksums"`
		ArtifactCount      int      `json:"artifact_count"`
		ChecksumEntryCount int      `json:"checksum_entry_count"`
		ArtifactFiles      []string `json:"artifact_files"`
		ArtifactEntries    []struct {
			Role string `json:"role"`
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"artifact_entries"`
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
	if report.ArtifactCount != len(report.ArtifactFiles) || report.ArtifactCount != len(report.ArtifactEntries) || report.ArtifactCount != 4 {
		t.Fatalf("release artifact plan JSON artifact counts = %d/%d/%d, want 4: %+v", report.ArtifactCount, len(report.ArtifactFiles), len(report.ArtifactEntries), report)
	}
	if report.ChecksumEntryCount != 3 {
		t.Fatalf("release artifact plan JSON checksum count = %d, want 3: %+v", report.ChecksumEntryCount, report)
	}
	for _, want := range []string{"leia_v1.2.3-rc.1_", "leia-lsp_v1.2.3-rc.1_", "metadata.txt", "SHA256SUMS"} {
		if !strings.Contains(report.Artifact+" "+report.LSPArtifact+" "+report.Metadata+" "+report.Checksums, want) {
			t.Fatalf("release artifact plan JSON missing artifact fragment %q: %+v", want, report)
		}
		if !strings.Contains(strings.Join(report.ArtifactFiles, " "), want) {
			t.Fatalf("release artifact plan JSON missing counted artifact fragment %q: %+v", want, report.ArtifactFiles)
		}
	}
	for _, path := range []string{report.ArtifactPath, report.LSPArtifactPath, report.MetadataPath, report.ChecksumsPath} {
		if path == "" || !strings.HasPrefix(path, report.OutputDir) {
			t.Fatalf("release artifact plan path %q must be under output dir %q: %+v", path, report.OutputDir, report)
		}
	}
	entries := map[string]struct {
		Name string
		Path string
	}{}
	for _, entry := range report.ArtifactEntries {
		entries[entry.Role] = struct {
			Name string
			Path string
		}{Name: entry.Name, Path: entry.Path}
	}
	if entries["cli"].Name != report.Artifact || entries["cli"].Path != report.ArtifactPath ||
		entries["lsp"].Name != report.LSPArtifact || entries["lsp"].Path != report.LSPArtifactPath ||
		entries["metadata"].Name != report.Metadata || entries["metadata"].Path != report.MetadataPath ||
		entries["checksums"].Name != report.Checksums || entries["checksums"].Path != report.ChecksumsPath {
		t.Fatalf("release artifact plan entries do not map names to paths: %+v", report.ArtifactEntries)
	}
}

func TestReleaseMatrixProductionPlanReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	badJSON := runCommandResult(root, 30*time.Second, "bash", "scripts/production_check.sh", "--quick", "--json")
	if badJSON.err == nil {
		t.Fatalf("production --quick --json unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", badJSON.stdout, badJSON.stderr)
	}
	if strings.TrimSpace(badJSON.stdout) != "" {
		t.Fatalf("production --quick --json stdout = %q, want empty usage-error stdout", badJSON.stdout)
	}
	if !strings.Contains(badJSON.stderr, "--json is only supported with --list") {
		t.Fatalf("production --quick --json stderr = %q, want list-only JSON error", badJSON.stderr)
	}

	quickOut := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--quick", "--list", "--json")
	var quickReport struct {
		SchemaVersion     int    `json:"schema_version"`
		Status            string `json:"status"`
		Mode              string
		ReleaseProfile    bool   `json:"release_profile"`
		OutputDir         string `json:"output_dir"`
		ListOnly          bool   `json:"list_only"`
		RunCount          int    `json:"run_count"`
		SkipCount         int    `json:"skip_count"`
		CriticalRunCount  int    `json:"release_critical_run_count"`
		CriticalSkipCount int    `json:"critical_skip_count"`
		CriticalNameCount int    `json:"release_critical_skip_name_count"`
		RunnableChecks    []struct {
			Name            string `json:"name"`
			Command         string `json:"command"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"runnable_checks"`
		SkippedChecks       []string `json:"skipped_checks"`
		SkippedCheckDetails []struct {
			Name            string `json:"name"`
			Reason          string `json:"reason"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"skipped_check_details"`
		ReleaseCriticalRuns        []string `json:"release_critical_runs"`
		ReleaseCriticalNames       []string `json:"release_critical_skip_names"`
		ReleaseCriticalSkips       []string `json:"release_critical_skips"`
		ReleaseCriticalSkipDetails []struct {
			Name            string `json:"name"`
			Reason          string `json:"reason"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"release_critical_skip_details"`
	}
	if err := json.Unmarshal([]byte(quickOut), &quickReport); err != nil {
		t.Fatalf("quick production plan JSON failed to decode: %v\n%s", err, quickOut)
	}
	if quickReport.SchemaVersion != 1 || quickReport.Status != "pass" || quickReport.Mode != "quick" || quickReport.ReleaseProfile || quickReport.OutputDir != "" || !quickReport.ListOnly || quickReport.RunCount != len(quickReport.RunnableChecks) || quickReport.SkipCount != len(quickReport.SkippedChecks) || quickReport.SkipCount != len(quickReport.SkippedCheckDetails) || quickReport.CriticalRunCount != len(quickReport.ReleaseCriticalRuns) || quickReport.CriticalSkipCount != len(quickReport.ReleaseCriticalSkips) || quickReport.CriticalSkipCount != len(quickReport.ReleaseCriticalSkipDetails) || quickReport.CriticalNameCount != len(quickReport.ReleaseCriticalNames) {
		t.Fatalf("quick production plan JSON = %+v, want quick schema v1 plan", quickReport)
	}
	for i, detail := range quickReport.SkippedCheckDetails {
		if detail.Name == "" || detail.Reason == "" || quickReport.SkippedChecks[i] != detail.Name+": "+detail.Reason {
			t.Fatalf("quick production skip detail %d = %+v, skipped_checks=%+v", i, detail, quickReport.SkippedChecks)
		}
	}
	planDir := t.TempDir()
	runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--quick", "--list", "--out-dir", planDir)
	for _, name := range []string{"plan.txt", "plan.json", "commands.log"} {
		if _, err := os.Stat(filepath.Join(planDir, name)); err != nil {
			t.Fatalf("production --out-dir missing %s: %v", name, err)
		}
	}
	planJSON := readFileString(t, filepath.Join(planDir, "plan.json"))
	var planArtifact struct {
		SchemaVersion int      `json:"schema_version"`
		Status        string   `json:"status"`
		Mode          string   `json:"mode"`
		OutputDir     string   `json:"output_dir"`
		RunCount      int      `json:"run_count"`
		CriticalRuns  []string `json:"release_critical_runs"`
		Runnable      []struct {
			Name            string `json:"name"`
			Command         string `json:"command"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"runnable_checks"`
	}
	if err := json.Unmarshal([]byte(planJSON), &planArtifact); err != nil {
		t.Fatalf("production plan artifact JSON failed to decode: %v\n%s", err, planJSON)
	}
	if planArtifact.SchemaVersion != 1 || planArtifact.Status != "pass" || planArtifact.Mode != "quick" || planArtifact.OutputDir != planDir || planArtifact.RunCount != len(planArtifact.Runnable) || planArtifact.RunCount == 0 {
		t.Fatalf("production plan artifact JSON = %+v, want quick schema v1 plan artifact", planArtifact)
	}

	out := runCommand(t, root, 30*time.Second, "bash", "scripts/production_check.sh", "--full", "--release-profile", "--release-version", "vX.Y.Z", "--list", "--json")
	var report struct {
		SchemaVersion     int    `json:"schema_version"`
		Status            string `json:"status"`
		Mode              string
		ReleaseProfile    bool   `json:"release_profile"`
		ReleaseVersion    string `json:"release_version"`
		OutputDir         string `json:"output_dir"`
		ListOnly          bool   `json:"list_only"`
		RunCount          int    `json:"run_count"`
		SkipCount         int    `json:"skip_count"`
		CriticalRunCount  int    `json:"release_critical_run_count"`
		CriticalSkipCount int    `json:"critical_skip_count"`
		CriticalNameCount int    `json:"release_critical_skip_name_count"`
		RunnableChecks    []struct {
			Name            string `json:"name"`
			Command         string `json:"command"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"runnable_checks"`
		SkippedChecks       []string `json:"skipped_checks"`
		SkippedCheckDetails []struct {
			Name            string `json:"name"`
			Reason          string `json:"reason"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"skipped_check_details"`
		ReleaseCriticalRuns        []string `json:"release_critical_runs"`
		ReleaseCriticalNames       []string `json:"release_critical_skip_names"`
		ReleaseCriticalSkips       []string `json:"release_critical_skips"`
		ReleaseCriticalSkipDetails []struct {
			Name            string `json:"name"`
			Reason          string `json:"reason"`
			ReleaseCritical bool   `json:"release_critical"`
		} `json:"release_critical_skip_details"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("production plan JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.Mode != "full" || !report.ReleaseProfile || report.ReleaseVersion != "vX.Y.Z" || report.OutputDir != "" || !report.ListOnly || report.RunCount != len(report.RunnableChecks) || report.SkipCount != len(report.SkippedChecks) || report.SkipCount != len(report.SkippedCheckDetails) || report.CriticalRunCount != len(report.ReleaseCriticalRuns) || report.CriticalSkipCount != len(report.ReleaseCriticalSkips) || report.CriticalSkipCount != len(report.ReleaseCriticalSkipDetails) || report.CriticalNameCount != len(report.ReleaseCriticalNames) {
		t.Fatalf("production plan JSON = %+v, want release profile schema v1 plan", report)
	}
	for i, detail := range report.SkippedCheckDetails {
		if detail.Name == "" || detail.Reason == "" || report.SkippedChecks[i] != detail.Name+": "+detail.Reason {
			t.Fatalf("production plan skip detail %d = %+v, skipped_checks=%+v", i, detail, report.SkippedChecks)
		}
		if detail.ReleaseCritical != stringSliceContains(report.ReleaseCriticalNames, detail.Name) {
			t.Fatalf("production plan skip detail %q release_critical=%v, release critical names=%+v", detail.Name, detail.ReleaseCritical, report.ReleaseCriticalNames)
		}
	}
	for i, detail := range report.ReleaseCriticalSkipDetails {
		if detail.Name == "" || detail.Reason == "" || !detail.ReleaseCritical || report.ReleaseCriticalSkips[i] != detail.Name+": "+detail.Reason {
			t.Fatalf("production plan critical skip detail %d = %+v, release_critical_skips=%+v", i, detail, report.ReleaseCriticalSkips)
		}
	}
	for _, want := range []string{"Correctness", "Architecture Health", "Performance Gate", "Public Release Blockers", "Release Smoke", "Release Distribution", "Release Notes", "Release Artifacts"} {
		if !stringSliceContains(report.ReleaseCriticalNames, want) {
			t.Fatalf("production plan JSON release critical names missing %q: %+v", want, report.ReleaseCriticalNames)
		}
		if !stringSliceContains(report.ReleaseCriticalRuns, want) {
			t.Fatalf("production plan JSON release critical runs missing %q: %+v", want, report.ReleaseCriticalRuns)
		}
	}
	for _, check := range report.RunnableChecks {
		if check.ReleaseCritical != stringSliceContains(report.ReleaseCriticalRuns, check.Name) {
			t.Fatalf("production plan JSON runnable check %q release_critical=%v, release_critical_runs=%+v", check.Name, check.ReleaseCritical, report.ReleaseCriticalRuns)
		}
	}
	commands := map[string]string{}
	for _, check := range report.RunnableChecks {
		commands[check.Name] = check.Command
	}
	for name, want := range map[string]string{
		"Architecture Health":     "scripts/run.sh arch --json",
		"Module Path Gate":        "scripts/run.sh module-path github.com/never-labs/leia",
		"Shell Script Syntax":     "scripts/run.sh shell-syntax",
		"Public Release Blockers": "scripts/run.sh public-blockers --require-resolved",
		"Release Distribution":    "scripts/run.sh release-dist --require-goreleaser",
		"Release Notes":           "scripts/run.sh release-notes-gate --version \"vX.Y.Z\"",
		"Release Artifacts":       "scripts/run.sh release-artifacts-gate",
		"Release Smoke":           "scripts/run.sh release-smoke",
		"CLI Experience":          "scripts/run.sh cli-experience",
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
		Status         string `json:"status"`
		Profile        string `json:"profile"`
		ListOnly       bool   `json:"list_only"`
		NoLuaJIT       bool   `json:"no_luajit"`
		ReleaseVersion string `json:"release_version"`
		CommandCount   int    `json:"command_count"`
		Commands       []struct {
			Name     string   `json:"name"`
			Args     []string `json:"args"`
			ArgCount int      `json:"arg_count"`
			Command  string   `json:"command"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("ci profile plan JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.Profile != "release" || !report.ListOnly || report.NoLuaJIT || report.ReleaseVersion != "vX.Y.Z" || report.CommandCount != 1 || len(report.Commands) != 1 {
		t.Fatalf("ci profile plan JSON = %+v, want release schema v1 plan", report)
	}
	command := report.Commands[0]
	if command.Name != "Production check" || command.Command != "scripts/run.sh production --full --release-profile --release-version vX.Y.Z" {
		t.Fatalf("ci profile plan command = %+v, want versioned production check", command)
	}
	if command.ArgCount != len(command.Args) || command.ArgCount != 6 {
		t.Fatalf("ci profile plan arg_count = %d args = %#v, want count 6 matching args", command.ArgCount, command.Args)
	}
	for _, want := range []string{"scripts/run.sh", "production", "--full", "--release-profile", "--release-version", "vX.Y.Z"} {
		if !stringSliceContains(command.Args, want) {
			t.Fatalf("ci profile plan args = %#v, want %q", command.Args, want)
		}
	}
}

func TestReleaseMatrixLauncherRoutesTaskHelp(t *testing.T) {
	root := findRepoRoot(t)
	entrypointsDoc := readFileString(t, filepath.Join(root, "docs", "design", "script-entrypoints.md"))
	for _, want := range []string{
		"scripts/run.sh help <task>",
		"discoverable from `scripts/run.sh --help`",
		"implementation files rather than public entrypoints",
	} {
		if !strings.Contains(entrypointsDoc, want) {
			t.Fatalf("docs/design/script-entrypoints.md must document launcher help and private task files; missing %q", want)
		}
	}
	docsHome := readFileString(t, filepath.Join(root, "docs", "index.md"))
	if !strings.Contains(docsHome, "(design/script-entrypoints.md)") {
		t.Fatal("docs/index.md must link the script entrypoint policy")
	}
	for _, tc := range []struct {
		task string
		want string
	}{
		{task: "test", want: "Usage: scripts/run.sh test <profile>"},
		{task: "manifest-check", want: "Usage: scripts/run.sh manifest-check [ROOT...]"},
		{task: "module-path", want: "Usage: scripts/run.sh module-path [EXPECTED_MODULE]"},
		{task: "release-smoke", want: "Usage: scripts/run.sh release-smoke [SMOKE_SCRIPT]"},
		{task: "shell-syntax", want: "Usage: scripts/run.sh shell-syntax"},
	} {
		out := runCommand(t, root, 30*time.Second, "scripts/run.sh", "help", tc.task)
		if !strings.Contains(out, tc.want) {
			t.Fatalf("scripts/run.sh help %s = %q, want %q", tc.task, out, tc.want)
		}
	}
}

func TestReleaseMatrixQConformanceReportIsMachineReadable(t *testing.T) {
	t.Skip("q conformance is optional extension coverage, not a core release report")
	root := findRepoRoot(t)
	out := runCommand(t, root, 60*time.Second, "bash", "scripts/q_conformance_gate.sh", "--scope", "core", "--bench", "none", "--json")
	var report struct {
		SchemaVersion    int      `json:"schema_version"`
		Status           string   `json:"status"`
		Scope            string   `json:"scope"`
		BenchMode        string   `json:"bench_mode"`
		Jobs             int      `json:"jobs"`
		TimeoutSeconds   int      `json:"timeout_seconds"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureCount     int      `json:"failure_count"`
		FailureKinds     []string `json:"failure_kinds"`
		FailureDetails   []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
			Value   string `json:"value"`
		} `json:"failure_details"`
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
	if report.FailureKindCount != 0 || len(report.FailureKinds) != 0 || report.FailureCount != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("q conformance JSON failures = kinds %d/%d details %d/%d, want none", report.FailureKindCount, len(report.FailureKinds), report.FailureCount, len(report.FailureDetails))
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

	failed := runCommandResult(root, 30*time.Second, "bash", "scripts/q_conformance_gate.sh", "--scope", "core", "--bench", "none", "--jobs", "0", "--json")
	if failed.err == nil {
		t.Fatalf("q conformance invalid jobs unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", failed.stdout, failed.stderr)
	}
	var failedReport struct {
		SchemaVersion    int      `json:"schema_version"`
		Status           string   `json:"status"`
		Jobs             int      `json:"jobs"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureCount     int      `json:"failure_count"`
		FailureKinds     []string `json:"failure_kinds"`
		FailureDetails   []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
			Value   string `json:"value"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(failed.stdout), &failedReport); err != nil {
		t.Fatalf("q conformance invalid jobs JSON failed to decode: %v\nstdout:\n%s\nstderr:\n%s", err, failed.stdout, failed.stderr)
	}
	if failedReport.SchemaVersion != 1 || failedReport.Status != "fail" || failedReport.Jobs != 0 {
		t.Fatalf("q conformance invalid jobs JSON = %+v, want failed schema v1 report", failedReport)
	}
	if failedReport.FailureKindCount != 1 || failedReport.FailureCount != 1 || !stringSliceContains(failedReport.FailureKinds, "invalid_jobs") || len(failedReport.FailureDetails) != 1 || failedReport.FailureDetails[0].Kind != "invalid_jobs" || failedReport.FailureDetails[0].Value != "0" {
		t.Fatalf("q conformance invalid jobs failure details = %+v, want invalid_jobs", failedReport)
	}

	invalidScope := runCommandResult(root, 30*time.Second, "bash", "scripts/q_conformance_gate.sh", "--scope", "language", "--bench", "none", "--json")
	if invalidScope.err == nil {
		t.Fatalf("q conformance invalid scope unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", invalidScope.stdout, invalidScope.stderr)
	}
	var invalidScopeReport struct {
		SchemaVersion  int    `json:"schema_version"`
		Status         string `json:"status"`
		Scope          string `json:"scope"`
		FailureDetails []struct {
			Kind  string `json:"kind"`
			Value string `json:"value"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(invalidScope.stdout), &invalidScopeReport); err != nil {
		t.Fatalf("q conformance invalid scope JSON failed to decode: %v\nstdout:\n%s\nstderr:\n%s", err, invalidScope.stdout, invalidScope.stderr)
	}
	if invalidScopeReport.SchemaVersion != 1 || invalidScopeReport.Status != "fail" || invalidScopeReport.Scope != "language" || len(invalidScopeReport.FailureDetails) != 1 || invalidScopeReport.FailureDetails[0].Kind != "invalid_scope" || invalidScopeReport.FailureDetails[0].Value != "language" {
		t.Fatalf("q conformance invalid scope JSON = %+v, want invalid_scope", invalidScopeReport)
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
		FailureKindCount  int      `json:"failure_kind_count"`
		FailureCount      int      `json:"failure_count"`
		FailureKinds      []string `json:"failure_kinds"`
		FailureDetails    []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
			Value   string `json:"value"`
		} `json:"failure_details"`
		TextMateCount    int      `json:"textmate_grammar_count"`
		VSCodeCount      int      `json:"vscode_asset_count"`
		TreeSitterCount  int      `json:"tree_sitter_asset_count"`
		SmokeTestCount   int      `json:"smoke_test_count"`
		TextMateGrammars []string `json:"textmate_grammars"`
		VSCodeAssets     []string `json:"vscode_assets"`
		TreeSitterAssets []string `json:"tree_sitter_assets"`
		SmokeTests       []string `json:"smoke_tests"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("editor asset JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.RequireTreeSitter {
		t.Fatalf("editor asset JSON = %+v, want passing schema v1 non-strict report", report)
	}
	if report.FailureKindCount != 0 || report.FailureCount != 0 || len(report.FailureKinds) != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("editor asset JSON failures = kinds %d/%d details %d/%d, want none", report.FailureKindCount, len(report.FailureKinds), report.FailureCount, len(report.FailureDetails))
	}
	for _, status := range []string{report.TreeSitterStatus, report.EmacsStatus} {
		if status != "verified" && status != "skipped" {
			t.Fatalf("editor asset JSON has unexpected optional tool status %q: %+v", status, report)
		}
	}
	if report.TextMateCount != len(report.TextMateGrammars) || report.TextMateCount != 2 || report.VSCodeCount != len(report.VSCodeAssets) || report.VSCodeCount != 5 || report.TreeSitterCount != len(report.TreeSitterAssets) || report.TreeSitterCount != 3 || report.SmokeTestCount != len(report.SmokeTests) || report.SmokeTestCount != 1 {
		t.Fatalf("editor asset JSON counts = %+v, want counted asset collections", report)
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
	if !stringSliceContains(report.SmokeTests, "cmd/leia/editor_smoke.go") {
		t.Fatalf("editor asset JSON missing smoke test: %+v", report.SmokeTests)
	}
}

func TestReleaseMatrixDiagnosticsBundleReportIsMachineReadable(t *testing.T) {
	root := findRepoRoot(t)
	tmp := t.TempDir()
	outDir := filepath.Join(tmp, "diag")
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "diag", "bundle", "--output", outDir, "--skip-go-tests", "--skip-benchmarks", "--json")
	var report struct {
		SchemaVersion  int    `json:"schema_version"`
		Status         string `json:"status"`
		OutputDir      string `json:"output_dir"`
		RunGoTests     bool   `json:"run_go_tests"`
		RunBenchmarks  bool   `json:"run_benchmarks"`
		FailureCount   int    `json:"failure_count"`
		FileCount      int    `json:"file_count"`
		Summary        string `json:"summary"`
		Manifest       string `json:"manifest"`
		FailureDetails []struct {
			Name       string `json:"name"`
			Log        string `json:"log"`
			StatusFile string `json:"status_file"`
			ExitStatus int    `json:"exit_status"`
		} `json:"failure_details"`
		Files []string `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("diagnostics bundle JSON failed to decode: %v\n%s", err, out)
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.OutputDir != outDir || report.RunGoTests || report.RunBenchmarks || report.FailureCount != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("diagnostics bundle JSON = %+v, want passing no-test/no-benchmark schema v1 report", report)
	}
	if report.FileCount != len(report.Files) {
		t.Fatalf("diagnostics bundle JSON file count = %d/%d in %+v", report.FileCount, len(report.Files), report)
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
	registry := releaseReportRegistry(t, root)
	declared, ok := registry["scripts/run.sh worktree --json"]
	if !ok {
		t.Fatal("capabilities report registry missing scripts/run.sh worktree --json")
	}
	if !stringSliceContains(declared.CountFields, "finding_count") || !stringSliceContains(declared.CountFields, "finding_status_count") || !stringSliceContains(declared.CollectionFields, "findings") || !stringSliceContains(declared.CollectionFields, "finding_statuses") {
		t.Fatalf("worktree audit registry = %+v, want finding and status count fields", declared)
	}
	out := runCommand(t, root, 60*time.Second, "bash", "scripts/worktree_audit.sh", "--json")
	assertReleaseReportSchemaVersion(t, out, "scripts/run.sh worktree --json", declared.SchemaVersion)
	assertReleaseReportStatusField(t, out, "scripts/run.sh worktree --json", declared.StatusField)
	assertNestedJSONCountMatchesCollection(t, out, "scripts/run.sh worktree --json", []string{"finding_count"}, []string{"findings"})
	assertNestedJSONCountMatchesCollection(t, out, "scripts/run.sh worktree --json", []string{"finding_status_count"}, []string{"finding_statuses"})
	var report struct {
		SchemaVersion      int    `json:"schema_version"`
		Status             string `json:"status"`
		FailOnFindings     bool   `json:"fail_on_findings"`
		FindingCount       int    `json:"finding_count"`
		FindingStatusCount int    `json:"finding_status_count"`
		FindingStatuses    []struct {
			Status string `json:"status"`
			Count  int    `json:"count"`
		} `json:"finding_statuses"`
		Findings []struct {
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
	statusCounts := map[string]int{}
	for _, finding := range report.Findings {
		if finding.Status == "" || finding.Path == "" || finding.Branch == "" || finding.Detail == "" {
			t.Fatalf("worktree audit finding must include status/path/branch/detail: %+v", finding)
		}
		statusCounts[finding.Status]++
	}
	if report.FindingStatusCount != len(report.FindingStatuses) || report.FindingStatusCount != len(statusCounts) {
		t.Fatalf("worktree audit status counts = %d/%d actual %d", report.FindingStatusCount, len(report.FindingStatuses), len(statusCounts))
	}
	for _, status := range report.FindingStatuses {
		if status.Status == "" || status.Count <= 0 || statusCounts[status.Status] != status.Count {
			t.Fatalf("worktree audit status summary = %+v actual=%+v", report.FindingStatuses, statusCounts)
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
			SchemaVersion    int      `json:"schema_version"`
			Status           string   `json:"status"`
			DryRun           bool     `json:"dry_run"`
			Verify           bool     `json:"verify"`
			Repo             string   `json:"repo"`
			Version          string   `json:"version"`
			GOOS             string   `json:"goos"`
			GOARCH           string   `json:"goarch"`
			ArchiveExt       string   `json:"archive_ext"`
			Asset            string   `json:"asset"`
			URL              string   `json:"url"`
			Checksums        string   `json:"checksums"`
			BinDir           string   `json:"bin_dir"`
			Binary           string   `json:"binary"`
			LSPBinary        string   `json:"lsp_binary"`
			InstallCount     int      `json:"install_count"`
			BinaryCount      int      `json:"binary_count"`
			InstallPathCount int      `json:"install_path_count"`
			Binaries         []string `json:"binaries"`
			InstallPaths     []string `json:"install_paths"`
			InstallEntries   []struct {
				Role string `json:"role"`
				Name string `json:"name"`
				Path string `json:"path"`
			} `json:"install_entries"`
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
		if report.InstallCount != 2 || report.BinaryCount != 2 || report.InstallPathCount != 2 || len(report.Binaries) != report.BinaryCount || len(report.InstallPaths) != report.InstallPathCount || len(report.InstallEntries) != report.InstallCount {
			t.Fatalf("install dry-run JSON install counts = install:%d/%d binary:%d/%d path:%d/%d", report.InstallCount, len(report.InstallEntries), report.BinaryCount, len(report.Binaries), report.InstallPathCount, len(report.InstallPaths))
		}
		if !stringSliceContains(report.Binaries, report.Binary) || !stringSliceContains(report.Binaries, report.LSPBinary) || !stringSliceContains(report.InstallPaths, report.InstallPath) || !stringSliceContains(report.InstallPaths, report.LSPInstallPath) {
			t.Fatalf("install dry-run JSON collection fields do not match scalar fields for %s/%s: %+v", tc.goos, tc.goarch, report)
		}
		if report.InstallPath != tc.installPath || report.LSPInstallPath != tc.lspInstallPath || report.BinDir != "/tmp/leia-bin" {
			t.Fatalf("install dry-run JSON has wrong install paths for %s/%s: %+v", tc.goos, tc.goarch, report)
		}
		entries := map[string]struct {
			Name string
			Path string
		}{}
		for _, entry := range report.InstallEntries {
			entries[entry.Role] = struct {
				Name string
				Path string
			}{Name: entry.Name, Path: entry.Path}
		}
		if entries["cli"].Name != report.Binary || entries["cli"].Path != report.InstallPath || entries["lsp"].Name != report.LSPBinary || entries["lsp"].Path != report.LSPInstallPath {
			t.Fatalf("install dry-run JSON install entries do not map binaries to paths for %s/%s: %+v", tc.goos, tc.goarch, report.InstallEntries)
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
	jsonCheckResult := runCommandResult(root, 30*time.Second, "bash", "scripts/release_artifacts_check.sh", "--json", "--version", "bad version")
	if jsonCheckResult.err == nil {
		t.Fatalf("release_artifacts_check.sh bad JSON version unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", jsonCheckResult.stdout, jsonCheckResult.stderr)
	}
	var checkReport struct {
		SchemaVersion    int      `json:"schema_version"`
		Status           string   `json:"status"`
		Version          string   `json:"version"`
		FailureKindCount int      `json:"failure_kind_count"`
		FailureKinds     []string `json:"failure_kinds"`
		FailureCount     int      `json:"failure_count"`
		FailureDetails   []struct {
			Kind    string `json:"kind"`
			Message string `json:"message"`
		} `json:"failure_details"`
	}
	if err := json.Unmarshal([]byte(jsonCheckResult.stdout), &checkReport); err != nil {
		t.Fatalf("release_artifacts_check.sh bad version JSON failed to decode: %v\nstdout:\n%s\nstderr:\n%s", err, jsonCheckResult.stdout, jsonCheckResult.stderr)
	}
	if checkReport.SchemaVersion != 1 || checkReport.Status != "fail" || checkReport.Version != "bad version" {
		t.Fatalf("release_artifacts_check.sh bad version JSON = %+v, want failed schema v1 report", checkReport)
	}
	if checkReport.FailureKindCount != 1 || checkReport.FailureCount != 1 || !stringSliceContains(checkReport.FailureKinds, "invalid_version") || len(checkReport.FailureDetails) != 1 || checkReport.FailureDetails[0].Kind != "invalid_version" || !strings.Contains(checkReport.FailureDetails[0].Message, "release artifact check version must match") {
		t.Fatalf("release_artifacts_check.sh bad version failure details = %+v, want invalid_version", checkReport)
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

func TestReleaseMatrixModuleGraphJSONReportsCounts(t *testing.T) {
	root := findRepoRoot(t)
	tmp := t.TempDir()
	runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "init", "--module", "github.com/example/project", "--dir", tmp)
	source := filepath.Join(tmp, "main.leia")
	if err := os.WriteFile(source, []byte("require(\"json\")\nprint(\"ok\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "graph", "--json", tmp)
	var report struct {
		SchemaVersion   int  `json:"schema_version"`
		OK              bool `json:"ok"`
		FileCount       int  `json:"file_count"`
		DiagnosticCount int  `json:"diagnostic_count"`
		Files           []struct {
			File     string   `json:"file"`
			Requires []string `json:"requires"`
		} `json:"files"`
		Diagnostics []struct {
			Code string `json:"code"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("mod graph JSON failed to decode: %v\n%s", err, out)
	}
	assertJSONFieldsPresentAndNonNull(t, out, "mod graph JSON", "files", "diagnostics")
	if report.SchemaVersion != 1 || !report.OK || report.FileCount != len(report.Files) || report.FileCount != 1 || report.DiagnosticCount != len(report.Diagnostics) || report.DiagnosticCount != 0 {
		t.Fatalf("mod graph JSON = %+v, want counted schema v1 graph", report)
	}
	if report.Files[0].File != "main.leia" || !stringSliceContains(report.Files[0].Requires, "json") {
		t.Fatalf("mod graph file = %+v, want main.leia requiring json", report.Files[0])
	}
}

func TestReleaseMatrixModuleMetadataReportsCountCollections(t *testing.T) {
	root := findRepoRoot(t)
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "leia.mod"), []byte(`module github.com/example/project
leia 0.1
capability net.client
require example.com/lib v1.0.0
replace example.com/lib => ./lib
collection vendor ./vendor
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "lib"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "lib", "leia.mod"), []byte("module example.com/lib\nleia 0.1\ncapability fs.read\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, "vendor"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "main.leia"), []byte("require(\"example.com/lib\")\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	listOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "list", "--json", tmp)
	var listReport struct {
		SchemaVersion   int           `json:"schema_version"`
		OK              bool          `json:"ok"`
		RequireCount    int           `json:"require_count"`
		ReplaceCount    int           `json:"replace_count"`
		CollectionCount int           `json:"collection_count"`
		DiagnosticCount int           `json:"diagnostic_count"`
		Requires        []interface{} `json:"requires"`
		Replaces        []interface{} `json:"replaces"`
		Collections     []interface{} `json:"collections"`
		Diagnostics     []interface{} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(listOut), &listReport); err != nil {
		t.Fatalf("mod list JSON failed to decode: %v\n%s", err, listOut)
	}
	assertJSONFieldsPresentAndNonNull(t, listOut, "mod list JSON", "requires", "replaces", "collections", "diagnostics")
	if listReport.SchemaVersion != 1 || !listReport.OK || listReport.RequireCount != len(listReport.Requires) || listReport.RequireCount != 1 || listReport.ReplaceCount != len(listReport.Replaces) || listReport.ReplaceCount != 1 || listReport.CollectionCount != len(listReport.Collections) || listReport.CollectionCount != 1 || listReport.DiagnosticCount != len(listReport.Diagnostics) || listReport.DiagnosticCount != 0 {
		t.Fatalf("mod list JSON = %+v, want counted module metadata", listReport)
	}

	capabilityOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "capability", "--json", tmp)
	var capabilityReport struct {
		SchemaVersion   int           `json:"schema_version"`
		OK              bool          `json:"ok"`
		CapabilityCount int           `json:"capability_count"`
		ModuleCount     int           `json:"module_count"`
		DiagnosticCount int           `json:"diagnostic_count"`
		Capabilities    []string      `json:"capabilities"`
		Modules         []interface{} `json:"modules"`
		Diagnostics     []interface{} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(capabilityOut), &capabilityReport); err != nil {
		t.Fatalf("mod capability JSON failed to decode: %v\n%s", err, capabilityOut)
	}
	assertJSONFieldsPresentAndNonNull(t, capabilityOut, "mod capability JSON", "capabilities", "modules", "matrix", "diagnostics")
	if capabilityReport.SchemaVersion != 1 || !capabilityReport.OK || capabilityReport.CapabilityCount != len(capabilityReport.Capabilities) || capabilityReport.CapabilityCount != 2 || capabilityReport.ModuleCount != len(capabilityReport.Modules) || capabilityReport.ModuleCount != 2 || capabilityReport.DiagnosticCount != len(capabilityReport.Diagnostics) || capabilityReport.DiagnosticCount != 0 {
		t.Fatalf("mod capability JSON = %+v, want counted capability matrix", capabilityReport)
	}

	lockOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "lock", "--json", tmp)
	var lockReport struct {
		SchemaVersion   int           `json:"schema_version"`
		OK              bool          `json:"ok"`
		EntryCount      int           `json:"entry_count"`
		DiagnosticCount int           `json:"diagnostic_count"`
		Entries         []interface{} `json:"entries"`
		Diagnostics     []interface{} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(lockOut), &lockReport); err != nil {
		t.Fatalf("mod lock JSON failed to decode: %v\n%s", err, lockOut)
	}
	assertJSONFieldsPresentAndNonNull(t, lockOut, "mod lock JSON", "entries", "diagnostics")
	if lockReport.SchemaVersion != 1 || !lockReport.OK || lockReport.EntryCount != len(lockReport.Entries) || lockReport.EntryCount == 0 || lockReport.DiagnosticCount != len(lockReport.Diagnostics) || lockReport.DiagnosticCount != 0 {
		t.Fatalf("mod lock JSON = %+v, want counted lock entries", lockReport)
	}

	tidyOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "tidy", "--json", "--dir", tmp)
	var tidyReport struct {
		SchemaVersion   int           `json:"schema_version"`
		OK              bool          `json:"ok"`
		RemovedCount    int           `json:"removed_count"`
		MissingCount    int           `json:"missing_count"`
		DiagnosticCount int           `json:"diagnostic_count"`
		Removed         []interface{} `json:"removed"`
		Missing         []interface{} `json:"missing"`
		Diagnostics     []interface{} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(tidyOut), &tidyReport); err != nil {
		t.Fatalf("mod tidy JSON failed to decode: %v\n%s", err, tidyOut)
	}
	assertJSONFieldsPresentAndNonNull(t, tidyOut, "mod tidy JSON", "removed", "missing", "diagnostics")
	if tidyReport.SchemaVersion != 1 || !tidyReport.OK || tidyReport.RemovedCount != len(tidyReport.Removed) || tidyReport.MissingCount != len(tidyReport.Missing) || tidyReport.DiagnosticCount != len(tidyReport.Diagnostics) || tidyReport.MissingCount != 0 || tidyReport.DiagnosticCount != 0 {
		t.Fatalf("mod tidy JSON = %+v, want counted tidy result", tidyReport)
	}

	verifyOut := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "mod", "verify", "--json", tmp)
	var verifyReport struct {
		SchemaVersion   int           `json:"schema_version"`
		OK              bool          `json:"ok"`
		DiagnosticCount int           `json:"diagnostic_count"`
		Diagnostics     []interface{} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(verifyOut), &verifyReport); err != nil {
		t.Fatalf("mod verify JSON failed to decode: %v\n%s", err, verifyOut)
	}
	assertJSONFieldsPresentAndNonNull(t, verifyOut, "mod verify JSON", "diagnostics")
	if verifyReport.SchemaVersion != 1 || !verifyReport.OK || verifyReport.DiagnosticCount != len(verifyReport.Diagnostics) || verifyReport.DiagnosticCount != 0 {
		t.Fatalf("mod verify JSON = %+v, want counted top-level diagnostics", verifyReport)
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
		"a := [1, 2, 3, 4, 5, 6, 7, 8, 6]",
		"x := sum(a)",
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
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, high-throughput in-memory data runtime, and first-class extensible domain dialects.",
		"x := sum(a)",
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
		"want 42.0 fallback without host LLM provider",
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
		"Leia is an efficient, embeddable scripting language for Go, combining a LuaJIT-class execution model, high-throughput in-memory data runtime, and first-class extensible domain dialects.",
		"x := sum(a)",
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
			dirs:       []string{"`examples/dialects/`", "`examples/web/`"},
			docTerms:   []string{"shell/process literals", "data-format fixtures", "fullstack_project"},
			cliIDs:     []string{"repo-dialects-shell_filesystem", "repo-web-serve_dialect_app", "repo-web-fullstack_project-main"},
			featureIDs: []string{"tagged_dialect_syntax", "spreadsheet_dialects"},
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
			docTerms:   []string{"dense arrays", "matrices", "vectors", "SoA", "SQLite `db.frame`"},
			cliIDs:     []string{"repo-data_processing-data_oriented-soa_kernels", "repo-data_processing-data_oriented-dense_matrix_vec_kernels", "repo-performance-execution_modes_matrix"},
			featureIDs: []string{"matrix_dense_arrays"},
			docRefs:    []string{"docs/reference/data-oriented/index.md", "docs/reference/scientific/index.md"},
		},
		{
			capability: "CLI tooling",
			dirs:       []string{"`examples/tooling/`", "`examples/testing/`"},
			docTerms:   []string{"release evidence", "diagnostics", "`leia test`"},
			cliIDs:     []string{"repo-tooling-release_evidence_pipeline", "repo-testing-jsonl_workflow_test"},
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
			},
			exampleIDs: []string{
				"repo-dialects-shell_filesystem",
				"repo-web-serve_dialect_app",
				"repo-web-fullstack_project-main",
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
			promise:      "high-throughput in-memory data runtime",
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
	case strings.HasPrefix(ref, "benchmarks/") && (strings.HasSuffix(ref, ".leia") || strings.HasSuffix(ref, "_test.go")):
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

func releaseMatrixZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func releaseMatrixTarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := []byte(files[name])
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func assertJSONFieldsPresentAndNonNull(t *testing.T, data, label string, fields ...string) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatalf("%s failed to decode as raw JSON: %v\n%s", label, err, data)
	}
	for _, field := range fields {
		value, ok := raw[field]
		if !ok {
			t.Fatalf("%s missing JSON field %q in %s", label, field, data)
		}
		if strings.TrimSpace(string(value)) == "null" {
			t.Fatalf("%s field %q is null in %s", label, field, data)
		}
	}
}

func assertNestedJSONFieldPresentAndNonNull(t *testing.T, data, label string, path ...string) {
	t.Helper()
	values := nestedJSONFields(t, data, label, path...)
	for _, value := range values {
		if strings.TrimSpace(string(value)) == "null" {
			t.Fatalf("%s field %s is null in %s", label, strings.Join(path, "."), data)
		}
	}
}

func assertNestedJSONNumberFieldPresent(t *testing.T, data, label string, path ...string) {
	t.Helper()
	values := nestedJSONFields(t, data, label, path...)
	for _, value := range values {
		var number json.Number
		if err := json.Unmarshal(value, &number); err != nil {
			t.Fatalf("%s field %s must be a JSON number: %v\n%s", label, strings.Join(path, "."), err, data)
		}
		if _, err := number.Int64(); err != nil {
			t.Fatalf("%s field %s must be an integer JSON number: %v\n%s", label, strings.Join(path, "."), err, data)
		}
	}
}

func nestedJSONField(t *testing.T, data, label string, path ...string) json.RawMessage {
	t.Helper()
	values := nestedJSONFields(t, data, label, path...)
	if len(values) != 1 {
		t.Fatalf("%s path %s resolved to %d values, want 1\n%s", label, strings.Join(path, "."), len(values), data)
	}
	return values[0]
}

func nestedJSONFields(t *testing.T, data, label string, path ...string) []json.RawMessage {
	t.Helper()
	if len(path) == 0 {
		t.Fatalf("%s nested JSON assertion needs a path", label)
	}
	current := []json.RawMessage{[]byte(data)}
	for _, field := range path {
		arrayField := strings.HasSuffix(field, "[]")
		objectField := strings.TrimSuffix(field, "[]")
		var next []json.RawMessage
		for _, item := range current {
			var raw map[string]json.RawMessage
			if err := json.Unmarshal(item, &raw); err != nil {
				t.Fatalf("%s failed to decode object while checking %s: %v\n%s", label, strings.Join(path, "."), err, data)
			}
			value, ok := raw[objectField]
			if !ok {
				t.Fatalf("%s missing JSON field %q while checking %s in %s", label, objectField, strings.Join(path, "."), data)
			}
			if strings.TrimSpace(string(value)) == "null" {
				t.Fatalf("%s field %q is null while checking %s in %s", label, objectField, strings.Join(path, "."), data)
			}
			if !arrayField {
				next = append(next, value)
				continue
			}
			var array []json.RawMessage
			if err := json.Unmarshal(value, &array); err != nil {
				t.Fatalf("%s field %q must be an array while checking %s: %v\n%s", label, objectField, strings.Join(path, "."), err, data)
			}
			if len(array) == 0 {
				t.Fatalf("%s field %q must be a non-empty array while checking %s in %s", label, objectField, strings.Join(path, "."), data)
			}
			next = append(next, array...)
		}
		current = next
	}
	return current
}

type releaseReportRegistryEntry struct {
	StatusField          string
	SchemaVersion        int
	ScalarFields         []string
	CountFields          []string
	CollectionFields     []string
	CollectionItemFields []string
}

type releaseReportSmokeCase struct {
	reportCommand string
	args          []string
	scalars       []string
	counts        []string
	fields        []string
	itemFields    []string
	matches       []releaseReportCountMatch
	wantFailure   bool
}

type releaseReportCountMatch struct {
	countPath      string
	collectionPath string
}

func assertReleaseReportRegistrySmoke(t *testing.T, root string, registry map[string]releaseReportRegistryEntry, tc releaseReportSmokeCase) {
	t.Helper()
	t.Run(tc.reportCommand, func(t *testing.T) {
		declared, ok := registry[tc.reportCommand]
		if !ok {
			t.Fatalf("capabilities report registry missing %q", tc.reportCommand)
		}
		for _, count := range tc.counts {
			if !stringSliceContains(declared.CountFields, count) {
				t.Fatalf("capabilities report %q count_fields = %#v, want %q", tc.reportCommand, declared.CountFields, count)
			}
		}
		for _, field := range tc.fields {
			if !stringSliceContains(declared.CollectionFields, field) {
				t.Fatalf("capabilities report %q collection_fields = %#v, want %q", tc.reportCommand, declared.CollectionFields, field)
			}
		}
		for _, field := range tc.itemFields {
			if !stringSliceContains(declared.CollectionItemFields, field) {
				t.Fatalf("capabilities report %q collection_item_fields = %#v, want %q", tc.reportCommand, declared.CollectionItemFields, field)
			}
		}
		var out string
		if tc.wantFailure {
			result := runCommandResult(root, 60*time.Second, tc.args[0], tc.args[1:]...)
			if result.err == nil {
				t.Fatalf("%s unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", tc.reportCommand, result.stdout, result.stderr)
			}
			out = result.stdout
		} else {
			out = runCommand(t, root, 60*time.Second, tc.args[0], tc.args[1:]...)
		}
		assertReleaseReportSchemaVersion(t, out, tc.reportCommand, declared.SchemaVersion)
		if declared.StatusField != "" {
			assertReleaseReportStatusField(t, out, tc.reportCommand, declared.StatusField)
		}
		for _, scalar := range tc.scalars {
			if !stringSliceContains(declared.ScalarFields, scalar) {
				t.Fatalf("capabilities report %q scalar_fields = %#v, want %q", tc.reportCommand, declared.ScalarFields, scalar)
			}
			assertNestedJSONFieldPresentAndNonNull(t, out, tc.reportCommand, strings.Split(scalar, ".")...)
		}
		for _, count := range tc.counts {
			assertNestedJSONNumberFieldPresent(t, out, tc.reportCommand, strings.Split(count, ".")...)
		}
		for _, field := range tc.fields {
			assertNestedJSONFieldPresentAndNonNull(t, out, tc.reportCommand, strings.Split(field, ".")...)
		}
		for _, field := range tc.itemFields {
			assertNestedJSONFieldPresentAndNonNull(t, out, tc.reportCommand, strings.Split(field, ".")...)
		}
		for _, match := range tc.matches {
			assertNestedJSONCountMatchesCollection(t, out, tc.reportCommand, strings.Split(match.countPath, "."), strings.Split(match.collectionPath, "."))
		}
	})
}

func assertReleaseReportSchemaVersion(t *testing.T, data, label string, want int) {
	t.Helper()
	if want <= 0 {
		t.Fatalf("%s capabilities report registry must advertise positive schema_version, got %d", label, want)
	}
	value := nestedJSONField(t, data, label, "schema_version")
	var got int
	if err := json.Unmarshal(value, &got); err != nil {
		t.Fatalf("%s schema_version must be a JSON integer: %v\n%s", label, err, data)
	}
	if got != want {
		t.Fatalf("%s schema_version = %d, want registry schema_version %d\n%s", label, got, want, data)
	}
}

func assertNestedJSONCountMatchesCollection(t *testing.T, data, label string, countPath, collectionPath []string) {
	t.Helper()
	counts := nestedJSONFields(t, data, label, countPath...)
	collections := nestedJSONFields(t, data, label, collectionPath...)
	if len(counts) != len(collections) {
		t.Fatalf("%s count path %s resolved to %d values but collection path %s resolved to %d values\n%s", label, strings.Join(countPath, "."), len(counts), strings.Join(collectionPath, "."), len(collections), data)
	}
	for i := range counts {
		var count int
		if err := json.Unmarshal(counts[i], &count); err != nil {
			t.Fatalf("%s count field %s must be a JSON integer: %v\n%s", label, strings.Join(countPath, "."), err, data)
		}
		var collection []json.RawMessage
		if err := json.Unmarshal(collections[i], &collection); err != nil {
			t.Fatalf("%s collection field %s must be a JSON array: %v\n%s", label, strings.Join(collectionPath, "."), err, data)
		}
		if count != len(collection) {
			t.Fatalf("%s count field %s[%d] = %d, want len(%s[%d]) = %d\n%s", label, strings.Join(countPath, "."), i, count, strings.Join(collectionPath, "."), i, len(collection), data)
		}
	}
}

func assertReleaseReportStatusField(t *testing.T, data, label, field string) {
	t.Helper()
	path := strings.Split(field, ".")
	value := nestedJSONField(t, data, label, path...)
	switch field {
	case "ok":
		var ok bool
		if err := json.Unmarshal(value, &ok); err != nil {
			t.Fatalf("%s status_field %q must be a JSON bool: %v\n%s", label, field, err, data)
		}
	case "status":
		var status string
		if err := json.Unmarshal(value, &status); err != nil {
			t.Fatalf("%s status_field %q must be a JSON string: %v\n%s", label, field, err, data)
		}
		if strings.TrimSpace(status) == "" {
			t.Fatalf("%s status_field %q must be non-empty in %s", label, field, data)
		}
	default:
		assertNestedJSONFieldPresentAndNonNull(t, data, label, path...)
	}
}

func releaseReportRegistry(t *testing.T, root string) map[string]releaseReportRegistryEntry {
	t.Helper()
	out := runCommand(t, root, 30*time.Second, "go", "run", "./cmd/leia", "capabilities", "--json")
	var payload struct {
		Tooling struct {
			Reports []struct {
				Command              string   `json:"command"`
				StatusField          string   `json:"status_field"`
				SchemaVersion        int      `json:"schema_version"`
				ScalarFields         []string `json:"scalar_fields"`
				CountFields          []string `json:"count_fields"`
				CollectionFields     []string `json:"collection_fields"`
				CollectionItemFields []string `json:"collection_item_fields"`
			} `json:"reports"`
		} `json:"tooling"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("capabilities JSON failed to decode: %v\n%s", err, out)
	}
	registry := map[string]releaseReportRegistryEntry{}
	for _, report := range payload.Tooling.Reports {
		registry[report.Command] = releaseReportRegistryEntry{
			StatusField:          report.StatusField,
			SchemaVersion:        report.SchemaVersion,
			ScalarFields:         report.ScalarFields,
			CountFields:          report.CountFields,
			CollectionFields:     report.CollectionFields,
			CollectionItemFields: report.CollectionItemFields,
		}
	}
	return registry
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
