package benchmarks

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func repoRootForScriptEntrypoints(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(dir)
}

func readRepoFile(t *testing.T, root string, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func requireContains(t *testing.T, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("text missing %q", want)
	}
}

func requireNotContains(t *testing.T, text, want string) {
	t.Helper()
	if strings.Contains(text, want) {
		t.Fatalf("text unexpectedly contains %q", want)
	}
}

func modulePathForScriptEntrypoints(t *testing.T, root string) string {
	t.Helper()
	for _, line := range strings.Split(readRepoFile(t, root, "go.mod"), "\n") {
		if strings.HasPrefix(line, "module ") {
			return strings.Fields(line)[1]
		}
	}
	t.Fatal("go.mod has no module declaration")
	return ""
}

func runScriptEntrypointCommand(t *testing.T, root string, env []string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = root
	if env != nil {
		cmd.Env = env
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return string(out), 0
}

func TestDiagShellUsesSharedDiscovery(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	diag := readRepoFile(t, root, "scripts", "diag.sh")
	requireContains(t, diag, "import benchmark_discovery as discovery")
	requireContains(t, diag, "discovery.GROUPS")
	requireContains(t, diag, "selector in discovery.GROUPS")
	requireContains(t, diag, "discovery.resolve_script_path(root, selector)")
	requireNotContains(t, diag, "domain_list_for()")
}

func TestBenchmarkShellWrappersExecLeiaCLI(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	requireContains(t, readRepoFile(t, root, "benchmarks", "regression_guard.sh"), `exec go run ./cmd/leia bench regression-guard "$@"`)
	requireContains(t, readRepoFile(t, root, "benchmarks", "strict_guard.sh"), `exec go run ./cmd/leia bench strict "$@"`)
}

func TestQColumnarSuiteWrapsLeiaBenchCompare(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	wrapper := readRepoFile(t, root, "benchmarks", "q_columnar_suite.sh")
	requireContains(t, wrapper, "go run ./cmd/leia bench compare")
	requireContains(t, wrapper, "--no-luajit")
	for _, bench := range []string{
		"data/q_columnar_eval_primitives",
		"data/q_columnar_qsql_filter_project",
		"data/q_columnar_qsql_group_xbar",
		"data/q_columnar_qsql_asof_join",
	} {
		requireContains(t, wrapper, "--bench="+bench)
	}
}

func TestScriptsPerformanceGateWrapsLeiaBenchTools(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	gate := readRepoFile(t, root, "scripts", "performance_gate.sh")
	for _, want := range []string{
		"go run ./cmd/leia bench compare",
		"go run ./cmd/leia bench strict",
		"--progress",
		`--jobs="$JOBS"`,
		"table/table_field_access",
		"--syntax-smoke",
		"SYNTAX_SMOKE_BENCHES=(",
		`PROFILE="syntax_smoke"`,
		"STRICT=0",
		"--quick-phase-smoke",
		`PROFILE="quick_phase_smoke"`,
		"STRICT_SMOKE_BENCHES=(",
		`for bench in "${STRICT_SMOKE_BENCHES[@]}"; do`,
		`if [ "$PROFILE" = "full" ]; then`,
		`for bench in "${STRICT_CORE_BENCHES[@]}"; do`,
	} {
		requireContains(t, gate, want)
	}
}

func TestReleaseScriptsGateCurrentModulePath(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	expected := modulePathForScriptEntrypoints(t, root)
	for _, script := range []string{"production_check.sh", "release_artifacts_check.sh"} {
		text := readRepoFile(t, root, "scripts", script)
		requireContains(t, text, expected)
		requireNotContains(t, text, "github.com/leia/leia")
		requireNotContains(t, text, "github.com/Never-Labs/leia")
	}
}

func TestProductionCheckFullPlanAvoidsGoTestDuplicates(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	text := readRepoFile(t, root, "scripts", "production_check.sh")
	parts := strings.SplitN(text, "build_full_plan() {", 2)
	if len(parts) != 2 {
		t.Fatal("build_full_plan not found")
	}
	fullPlan := strings.SplitN(parts[1], "\n}", 2)[0]
	requireContains(t, fullPlan, `add_go_test "Correctness"`)
	requireContains(t, fullPlan, `add_skip "Feature Matrix" "covered by Correctness`)
	requireContains(t, fullPlan, `add_skip "Release Matrix Metadata" "covered by Correctness`)
	requireNotContains(t, fullPlan, `add_go_test "Feature Matrix"`)
	requireNotContains(t, fullPlan, `add_go_test "Release Matrix Metadata"`)
}

func TestReleaseProfileRequiresReleaseOnlyToolSmokes(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	production := readRepoFile(t, root, "scripts", "production_check.sh")
	distribution := readRepoFile(t, root, "scripts", "release_distribution_check.sh")
	ci := readRepoFile(t, root, "cmd", "leia", "ci.go")
	for _, want := range []string{
		"--release-profile",
		"scripts/run.sh editor --require-tree-sitter",
		"scripts/run.sh release-dist --require-goreleaser",
		"scripts/run.sh release-check",
	} {
		requireContains(t, production, want)
	}
	requireContains(t, distribution, "--require-goreleaser")
	requireContains(t, distribution, "goreleaser CLI is required for release distribution profile")
	requireContains(t, ci, `"--full", "--release-profile"`)
}

func TestProductionReleaseProfileListIncludesRequiredToolFlags(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	out, code := runScriptEntrypointCommand(t, root, nil, "bash", "scripts/production_check.sh", "--full", "--release-profile", "--list")
	if code != 0 {
		t.Fatalf("production_check exit %d:\n%s", code, out)
	}
	for _, want := range []string{
		"Release profile: critical release tool skips are treated as failures.",
		"scripts/run.sh editor --require-tree-sitter",
		"scripts/run.sh release-dist --require-goreleaser",
		"scripts/run.sh release-check",
	} {
		requireContains(t, out, want)
	}
}

func TestReleaseDistributionRequireGoreleaserFailsWithoutCLI(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	env := append(os.Environ(), "PATH=/usr/bin:/bin")
	if _, code := runScriptEntrypointCommand(t, root, env, "bash", "-lc", "command -v goreleaser >/dev/null 2>&1"); code == 0 {
		t.Skip("goreleaser is available on the restricted PATH")
	}
	out, code := runScriptEntrypointCommand(t, root, env, "bash", "scripts/release_distribution_check.sh", "--require-goreleaser")
	if code != 1 {
		t.Fatalf("release_distribution_check exit %d, want 1:\n%s", code, out)
	}
	requireContains(t, out, "goreleaser CLI is required for release distribution profile")
}

func TestScriptsHaveValidTypeSpecificSyntax(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	entries, err := os.ReadDir(filepath.Join(root, "scripts"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	if len(entries) == 0 {
		t.Fatal("scripts directory is empty")
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		path := filepath.Join("scripts", name)
		var args []string
		switch filepath.Ext(name) {
		case ".sh":
			args = []string{"bash", "-n", path}
		case ".leia":
			args = []string{"go", "run", "./cmd/leia", "lint", path}
		default:
			t.Fatalf("unsupported script entrypoint type: %s", name)
		}
		if out, code := runScriptEntrypointCommand(t, root, nil, args...); code != 0 {
			t.Fatalf("%s syntax exit %d:\n%s", name, code, out)
		}
	}
}
