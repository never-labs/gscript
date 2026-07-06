package benchmarks

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func repoRootForPerformanceGate(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Dir(dir)
}

func readPerformanceGateFile(t *testing.T, root string, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{root}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func performanceGateContains(t *testing.T, text, want string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("text missing %q", want)
	}
}

func performanceGateNotContains(t *testing.T, text, want string) {
	t.Helper()
	if strings.Contains(text, want) {
		t.Fatalf("text unexpectedly contains %q", want)
	}
}

func performanceGateShellArrayValues(t *testing.T, text, name string) []string {
	t.Helper()
	start := strings.Index(text, name+"=(")
	if start < 0 {
		t.Fatalf("shell array %s not found", name)
	}
	end := strings.Index(text[start:], "\n)")
	if end < 0 {
		t.Fatalf("shell array %s has no closing line", name)
	}
	block := text[start : start+end]
	var values []string
	for _, line := range strings.Split(block, "\n")[1:] {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, `"`) && strings.HasSuffix(stripped, `"`) {
			values = append(values, strings.Trim(stripped, `"`))
		}
	}
	return values
}

type performanceGateFeatureMatrix struct {
	Features []struct {
		ID           string `json:"id"`
		SemanticGate struct {
			Refs []string `json:"refs"`
		} `json:"semantic_gate"`
		PerfHotCase struct {
			Refs []string `json:"refs"`
		} `json:"perf_hot_case"`
	} `json:"features"`
}

func performanceGateFeatureCellRefs(t *testing.T, root, featureID, cellName string) []string {
	t.Helper()
	var matrix performanceGateFeatureMatrix
	if err := json.Unmarshal([]byte(readPerformanceGateFile(t, root, "tests", "feature_matrix.json")), &matrix); err != nil {
		t.Fatal(err)
	}
	for _, feature := range matrix.Features {
		if feature.ID != featureID {
			continue
		}
		switch cellName {
		case "semantic_gate":
			return feature.SemanticGate.Refs
		case "perf_hot_case":
			return feature.PerfHotCase.Refs
		default:
			t.Fatalf("unsupported feature matrix cell %q", cellName)
		}
	}
	t.Fatalf("feature_matrix.json missing %s", featureID)
	return nil
}

func performanceGateBenchmarkIDsFromFeatureRefs(t *testing.T, root, featureID, cellName string) []string {
	t.Helper()
	var ids []string
	for _, ref := range performanceGateFeatureCellRefs(t, root, featureID, cellName) {
		if strings.HasPrefix(ref, "benchmarks/") && strings.HasSuffix(ref, ".leia") {
			ids = append(ids, strings.TrimSuffix(strings.TrimPrefix(ref, "benchmarks/"), ".leia"))
		}
	}
	return ids
}

func performanceGateSubject(median any, status, source string, cv float64) map[string]any {
	return map[string]any{
		"status": status,
		"source": source,
		"stats": map[string]any{
			"median": median,
			"cv_pct": cv,
		},
	}
}

func performanceGateTimingPayload(current, head any, opts ...func(map[string]any)) map[string]any {
	payload := map[string]any{
		"modes": []string{"default"},
		"results": []map[string]any{
			{
				"group":     "numeric",
				"benchmark": "hot_loop",
				"modes": map[string]any{
					"default": map[string]any{
						"current": performanceGateSubject(current, "ok", "script_repeat", 2.0),
						"head":    performanceGateSubject(head, "ok", "script_repeat", 2.0),
						"luajit":  performanceGateSubject(2.0, "ok", "script_repeat", 2.0),
					},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(payload)
	}
	return payload
}

func performanceGateWithBenchmarkID(id string) func(map[string]any) {
	return func(payload map[string]any) {
		parts := strings.SplitN(id, "/", 2)
		row := payload["results"].([]map[string]any)[0]
		row["group"] = parts[0]
		if len(parts) == 2 {
			row["benchmark"] = parts[1]
		} else {
			row["benchmark"] = ""
		}
	}
}

func performanceGateWithSubjectStatus(name, status string) func(map[string]any) {
	return func(payload map[string]any) {
		subject := payload["results"].([]map[string]any)[0]["modes"].(map[string]any)["default"].(map[string]any)[name].(map[string]any)
		subject["status"] = status
	}
}

func performanceGateWithLuaJIT(median any) func(map[string]any) {
	return func(payload map[string]any) {
		subject := payload["results"].([]map[string]any)[0]["modes"].(map[string]any)["default"].(map[string]any)["luajit"].(map[string]any)
		subject["stats"].(map[string]any)["median"] = median
	}
}

func performanceGateWithSource(source string) func(map[string]any) {
	return func(payload map[string]any) {
		mode := payload["results"].([]map[string]any)[0]["modes"].(map[string]any)["default"].(map[string]any)
		for _, name := range []string{"current", "head", "luajit"} {
			mode[name].(map[string]any)["source"] = source
		}
	}
}

func runPerformanceGateValidate(t *testing.T, root string, payload map[string]any, args ...string) (string, int) {
	t.Helper()
	td := t.TempDir()
	path := filepath.Join(td, "timing.json")
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmdArgs := append([]string{"scripts/performance_gate.sh", "--validate-only", path}, args...)
	return runPerformanceGateCommand(t, root, "bash", cmdArgs...)
}

func runPerformanceGateCommand(t *testing.T, root, name string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return string(out), exitErr.ExitCode()
		}
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return string(out), 0
}

func decodePerformanceGateReport(t *testing.T, out string) map[string]any {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("invalid JSON report: %v\n%s", err, out)
	}
	return report
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

func sortedMissing(values []string, allowed map[string]bool) []string {
	var missing []string
	for _, value := range values {
		if !allowed[value] {
			missing = append(missing, value)
		}
	}
	sort.Strings(missing)
	return missing
}

type performanceGateManifest struct {
	Cases []struct {
		ID string `json:"id"`
	} `json:"cases"`
	Workloads []struct {
		ID                  string `json:"id"`
		TimeSourceHint      string `json:"time_source_hint"`
		ComparisonReference *struct {
			Kind string `json:"kind"`
			Path string `json:"path"`
		} `json:"comparison_reference"`
	} `json:"workloads"`
}

func readPerformanceGateManifest(t *testing.T, root string) performanceGateManifest {
	t.Helper()
	var manifest performanceGateManifest
	if err := json.Unmarshal([]byte(readPerformanceGateFile(t, root, "benchmarks", "manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestPerformanceGateValidateOnlyAcceptsFlatScriptTimedRow(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.05, 1.00))
	if code != 0 {
		t.Fatalf("performance gate exit %d:\n%s", code, out)
	}
	performanceGateContains(t, out, "Performance gate current/HEAD ranking")
	performanceGateContains(t, out, "Performance gate passed.")
}

func TestPerformanceGateValidateOnlyJSONReportsMachineReadablePass(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.05, 1.00), "--json")
	if code != 0 {
		t.Fatalf("performance gate exit %d:\n%s", code, out)
	}
	report := decodePerformanceGateReport(t, out)
	if report["schema_version"].(float64) != 1 || report["status"] != "pass" || report["validate_only"] != true {
		t.Fatalf("unexpected report header: %#v", report)
	}
	if report["failure_count"].(float64) != 0 || len(report["failures"].([]any)) != 0 {
		t.Fatalf("unexpected failures: %#v", report)
	}
	if report["output_line_count"].(float64) != float64(len(report["output_lines"].([]any))) {
		t.Fatalf("output_line_count mismatch: %#v", report)
	}
	performanceGateContains(t, report["timing_json"].(string), "timing.json")
	var lines []string
	for _, line := range report["output_lines"].([]any) {
		lines = append(lines, line.(string))
	}
	performanceGateContains(t, strings.Join(lines, "\n"), "Performance gate passed.")
}

func TestPerformanceGateValidateOnlyJSONReportsMachineReadableFailure(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.50, 1.00), "--json")
	if code != 1 {
		t.Fatalf("performance gate exit %d, want 1:\n%s", code, out)
	}
	report := decodePerformanceGateReport(t, out)
	if report["schema_version"].(float64) != 1 || report["status"] != "issues" || report["failure_count"].(float64) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	failures := report["failures"].([]any)
	if len(failures) != 1 || failures[0] != "timing validation failed" {
		t.Fatalf("failures = %#v", failures)
	}
	if report["output_line_count"].(float64) != float64(len(report["output_lines"].([]any))) {
		t.Fatalf("output_line_count mismatch: %#v", report)
	}
	var lines []string
	for _, line := range report["output_lines"].([]any) {
		lines = append(lines, line.(string))
	}
	performanceGateContains(t, strings.Join(lines, "\n"), "Performance gate violations")
}

func TestPerformanceGateJSONRequiresValidateOnly(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateCommand(t, root, "bash", "scripts/performance_gate.sh", "--json", "--no-strict", "--no-luajit")
	if code != 2 {
		t.Fatalf("performance gate exit %d, want 2:\n%s", code, out)
	}
	performanceGateContains(t, out, "--json is only supported with --validate-only")
}

func TestPerformanceGateValidateOnlyAcceptsCurrentOnlyNewBenchmark(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.05, nil,
		performanceGateWithBenchmarkID("data/q_query_rollup"),
		performanceGateWithSubjectStatus("head", "missing"),
	))
	if code != 0 {
		t.Fatalf("performance gate exit %d:\n%s", code, out)
	}
	performanceGateContains(t, out, "current_only_new_benchmark")
	performanceGateContains(t, out, "Performance gate passed.")
}

func TestPerformanceGateSmokeProfilesAreExplicitParseableProfiles(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	for _, arg := range []string{"--quick-phase-smoke", "--syntax-smoke"} {
		t.Run(arg, func(t *testing.T) {
			out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.05, 1.00), arg)
			if code != 0 {
				t.Fatalf("performance gate exit %d:\n%s", code, out)
			}
			performanceGateContains(t, out, "Performance gate passed.")
		})
	}
}

func TestPerformanceGateSyntaxSmokeDialectGuardWiring(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	performanceGateContains(t, gate, `"app/dialect_syntax_smoke"`)
	performanceGateNotContains(t, strings.Join(performanceGateShellArrayValues(t, gate, "SYNTAX_SMOKE_BENCHES"), "\n"), "app/dialect_syntax_smoke")
	if got := performanceGateShellArrayValues(t, gate, "SYNTAX_DIALECT_SMOKE_BENCHES"); !reflect.DeepEqual(got, []string{"app/dialect_syntax_smoke"}) {
		t.Fatalf("SYNTAX_DIALECT_SMOKE_BENCHES = %#v", got)
	}
	performanceGateContains(t, gate,
		"if [ \"$PROFILE\" = \"syntax_smoke\" ]; then\n"+
			"        STRICT_CMD+=(--mode vm --mode default --mode no_filter)\n"+
			"    fi")
}

func TestPerformanceGateBuiltinSelectorsAreRegisteredManifestWorkloads(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	manifest := readPerformanceGateManifest(t, root)
	caseIDs := make(map[string]bool)
	for _, c := range manifest.Cases {
		caseIDs[c.ID] = true
	}
	workloadIDs := make(map[string]bool)
	for _, workload := range manifest.Workloads {
		workloadIDs[workload.ID] = true
	}
	var selectors []string
	for _, name := range []string{
		"CORE_BENCHES",
		"SMOKE_BENCHES",
		"SYNTAX_SMOKE_BENCHES",
		"SYNTAX_DIALECT_SMOKE_BENCHES",
		"STRICT_SMOKE_BENCHES",
		"PHASE_SMOKE_BENCHES",
		"FEATURE_SMOKE_BENCHES",
		"STRICT_CORE_BENCHES",
		"STRICT_FEATURE_BENCHES",
	} {
		selectors = append(selectors, performanceGateShellArrayValues(t, gate, name)...)
	}
	if missing := sortedMissing(selectors, caseIDs); len(missing) != 0 {
		t.Fatalf("selectors missing from manifest cases: %v", missing)
	}
	if missing := sortedMissing(selectors, workloadIDs); len(missing) != 0 {
		t.Fatalf("selectors missing from manifest workloads: %v", missing)
	}
}

func TestPerformanceGateFeatureSmokeKeepsDataOrientedDenseAndSOAGate(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	for _, arrayName := range []string{"PHASE_SMOKE_BENCHES", "FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"} {
		values := stringSet(performanceGateShellArrayValues(t, gate, arrayName))
		for _, want := range []string{"numeric/matmul_dense", "data/soa_affine_many", "data/soa_masked_aggregate"} {
			if !values[want] {
				t.Fatalf("%s missing %s", arrayName, want)
			}
		}
	}
	for _, arrayName := range []string{"FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"} {
		values := stringSet(performanceGateShellArrayValues(t, gate, arrayName))
		if !values["data/soa_filter_gather"] {
			t.Fatalf("%s missing data/soa_filter_gather", arrayName)
		}
	}
}

func TestPerformanceGateFeatureSmokeCoversQAnalyticsDataHotRefs(t *testing.T) {
	t.Skip("optional q extension coverage is not part of the core default performance gate")
	root := repoRootForPerformanceGate(t)
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	qHotRefs := stringSet(performanceGateBenchmarkIDsFromFeatureRefs(t, root, "q_analytics_dialect", "perf_hot_case"))
	for _, arrayName := range []string{"FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"} {
		values := stringSet(performanceGateShellArrayValues(t, gate, arrayName))
		for ref := range qHotRefs {
			if !values[ref] {
				t.Fatalf("%s missing q analytics hot ref %s", arrayName, ref)
			}
		}
	}
}

func TestPerformanceGateFeatureSmokeUsesStableSamplingForShortAppWorkloads(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	performanceGateContains(t, gate,
		"--feature-smoke)\n"+
			"            PROFILE=\"feature_smoke\"\n"+
			"            RUNS=2\n"+
			"            WARMUP=1\n"+
			"            TIMEOUT=90\n"+
			"            # Feature smoke includes loopback/http/sqlite/data workloads whose\n"+
			"            # individual script timings are short enough that 0.1s samples can\n"+
			"            # make current-vs-HEAD comparisons fail on measurement noise alone.\n"+
			"            MIN_SAMPLE_SECONDS=0.300\n"+
			"            MAX_REPEAT=256\n"+
			"            # Keep the mixed feature smoke serial by default. These workloads\n"+
			"            # compare current, clean HEAD, and LuaJIT binaries; running several\n"+
			"            # calibrated samples at once measures local CPU contention more\n"+
			"            # than language/runtime performance. A caller can still pass\n"+
			"            # --jobs=N explicitly when using the profile for exploratory runs.\n"+
			"            MAX_JOBS=1")
}

func TestPerformanceGateQAnalyticsFeatureMatrixHotRefsIncludeRunnableQQuerySmoke(t *testing.T) {
	t.Skip("optional q extension coverage is not part of the core default performance gate")
	root := repoRootForPerformanceGate(t)
	manifest := readPerformanceGateManifest(t, root)
	caseIDs := make(map[string]bool)
	for _, c := range manifest.Cases {
		caseIDs[c.ID] = true
	}
	workloads := make(map[string]performanceGateManifestWorkload)
	for _, workload := range manifest.Workloads {
		workloads[workload.ID] = performanceGateManifestWorkload(workload)
	}
	hotRefs := performanceGateBenchmarkIDsFromFeatureRefs(t, root, "q_analytics_dialect", "perf_hot_case")
	if !stringSet(hotRefs)["data/q_query_rollup"] {
		t.Fatal("q analytics hot refs missing data/q_query_rollup")
	}
	if missing := sortedMissing(hotRefs, caseIDs); len(missing) != 0 {
		t.Fatalf("q analytics hot refs missing from cases: %v", missing)
	}
	if missing := sortedMissing(hotRefs, workloadIDSet(workloads)); len(missing) != 0 {
		t.Fatalf("q analytics hot refs missing from workloads: %v", missing)
	}
	for _, benchmarkID := range []string{"data/q_query_rollup", "data/q_operator_pipeline"} {
		workload := workloads[benchmarkID]
		if workload.TimeSourceHint != "script_time_line" {
			t.Fatalf("%s time_source_hint = %q", benchmarkID, workload.TimeSourceHint)
		}
		if workload.ComparisonReference == nil || workload.ComparisonReference.Kind != "lua" {
			t.Fatalf("%s comparison_reference = %#v", benchmarkID, workload.ComparisonReference)
		}
		if _, err := os.Stat(filepath.Join(root, workload.ComparisonReference.Path)); err != nil {
			t.Fatalf("%s comparison reference missing: %v", benchmarkID, err)
		}
	}
}

type performanceGateManifestWorkload struct {
	ID                  string `json:"id"`
	TimeSourceHint      string `json:"time_source_hint"`
	ComparisonReference *struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	} `json:"comparison_reference"`
}

func workloadIDSet(workloads map[string]performanceGateManifestWorkload) map[string]bool {
	set := make(map[string]bool, len(workloads))
	for id := range workloads {
		set[id] = true
	}
	return set
}

func TestPerformanceGateDataOrientedFeatureMatrixHotRefsAreManifestedWithLuaJITRefs(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	manifest := readPerformanceGateManifest(t, root)
	caseIDs := make(map[string]bool)
	for _, c := range manifest.Cases {
		caseIDs[c.ID] = true
	}
	workloads := make(map[string]performanceGateManifestWorkload)
	for _, workload := range manifest.Workloads {
		workloads[workload.ID] = performanceGateManifestWorkload(workload)
	}
	expected := []string{
		"numeric/matmul_dense",
		"numeric/spectral_norm_dense",
		"data/soa_affine_many",
		"data/soa_masked_aggregate",
		"data/soa_filter_gather",
	}
	hotRefs := performanceGateBenchmarkIDsFromFeatureRefs(t, root, "matrix_dense_arrays", "perf_hot_case")
	if !reflect.DeepEqual(hotRefs, expected) {
		t.Fatalf("matrix dense hot refs = %#v, want %#v", hotRefs, expected)
	}
	if missing := sortedMissing(hotRefs, caseIDs); len(missing) != 0 {
		t.Fatalf("data-oriented hot refs missing from cases: %v", missing)
	}
	if missing := sortedMissing(hotRefs, workloadIDSet(workloads)); len(missing) != 0 {
		t.Fatalf("data-oriented hot refs missing from workloads: %v", missing)
	}
	for _, benchmarkID := range hotRefs {
		workload := workloads[benchmarkID]
		if workload.TimeSourceHint != "script_time_line" {
			t.Fatalf("%s time_source_hint = %q", benchmarkID, workload.TimeSourceHint)
		}
		if workload.ComparisonReference == nil || workload.ComparisonReference.Kind != "lua" {
			t.Fatalf("%s comparison_reference = %#v", benchmarkID, workload.ComparisonReference)
		}
		if _, err := os.Stat(filepath.Join(root, workload.ComparisonReference.Path)); err != nil {
			t.Fatalf("%s comparison reference missing: %v", benchmarkID, err)
		}
	}
}

func TestPerformanceGateFullGateSelectorsCoverDataOrientedFeatureRefsWithoutExpandingQuickGate(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	dataHotRefs := stringSet(performanceGateBenchmarkIDsFromFeatureRefs(t, root, "matrix_dense_arrays", "perf_hot_case"))
	fullParts := strings.SplitN(gate, `if [ "$PROFILE" = "full" ]; then`, 2)
	if len(fullParts) != 2 {
		t.Fatal("full profile block not found")
	}
	fullBlock := strings.SplitN(fullParts[1], "elif", 2)[0]
	allGroupsParts := strings.SplitN(gate, "ALL_BENCHMARK_GROUPS=(", 2)
	if len(allGroupsParts) != 2 {
		t.Fatal("ALL_BENCHMARK_GROUPS block not found")
	}
	allGroupsBlock := strings.SplitN(allGroupsParts[1], "\n)", 2)[0]
	performanceGateContains(t, fullBlock, "TIMING_CMD+=(--all-groups)")
	performanceGateContains(t, allGroupsBlock, "numeric")
	performanceGateContains(t, allGroupsBlock, "data")

	phaseValues := stringSet(performanceGateShellArrayValues(t, gate, "PHASE_SMOKE_BENCHES"))
	if intersectionLen(phaseValues, dataHotRefs) >= len(dataHotRefs) {
		t.Fatal("phase smoke unexpectedly covers every data-oriented hot ref")
	}
	for _, want := range []string{"numeric/matmul_dense", "data/soa_affine_many", "data/soa_masked_aggregate"} {
		if !phaseValues[want] {
			t.Fatalf("PHASE_SMOKE_BENCHES missing %s", want)
		}
	}
	for _, arrayName := range []string{"FEATURE_SMOKE_BENCHES", "STRICT_FEATURE_BENCHES"} {
		values := stringSet(performanceGateShellArrayValues(t, gate, arrayName))
		if intersectionLen(values, dataHotRefs) >= len(dataHotRefs) {
			t.Fatalf("%s unexpectedly covers every data-oriented hot ref", arrayName)
		}
		for _, want := range []string{"numeric/matmul_dense", "data/soa_affine_many", "data/soa_masked_aggregate", "data/soa_filter_gather"} {
			if !values[want] {
				t.Fatalf("%s missing %s", arrayName, want)
			}
		}
	}
}

func intersectionLen(a, b map[string]bool) int {
	count := 0
	for value := range a {
		if b[value] {
			count++
		}
	}
	return count
}

func TestPerformanceGateJITFallbackLuaJITContractKeepsGateRefs(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	semanticRefs := stringSet(performanceGateFeatureCellRefs(t, root, "arm64_jit_runtime_fallback", "semantic_gate"))
	perfRefs := stringSet(performanceGateFeatureCellRefs(t, root, "arm64_jit_runtime_fallback", "perf_hot_case"))
	gate := readPerformanceGateFile(t, root, "scripts", "performance_gate.sh")
	for _, ref := range []string{
		"internal/methodjit/semantic_gate_test.go",
		"internal/methodjit/diagnose_test.go",
		"internal/methodjit/exit_resume_check_test.go",
		"scripts/performance_gate.sh",
		"benchmarks/performance_gate_test.go",
		"cmd/leia/main_bench_test.go",
		"benchmarks/manifest.json",
		"docs/reference/performance/index.md",
	} {
		if !semanticRefs[ref] {
			t.Fatalf("arm64_jit_runtime_fallback semantic refs missing %s", ref)
		}
	}
	for _, ref := range []string{
		"benchmarks/numeric/matmul_dense.leia",
		"benchmarks/table/table_field_access.leia",
		"benchmarks/app/mixed_inventory_sim.leia",
	} {
		if !perfRefs[ref] {
			t.Fatalf("arm64_jit_runtime_fallback perf refs missing %s", ref)
		}
	}
	performanceGateContains(t, gate, "validate_luajit_artifact")
	performanceGateContains(t, gate, "--luajit-threshold")
	performanceGateContains(t, gate, "validate_strict_artifact")
}

func TestPerformanceGateHelpDocumentsSyntaxSmokeProfile(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateCommand(t, root, "bash", "scripts/performance_gate.sh", "--help")
	if code != 0 {
		t.Fatalf("performance gate help exit %d:\n%s", code, out)
	}
	performanceGateContains(t, out, "--syntax-smoke")
	performanceGateContains(t, out, "grammar-change hot-path gate")
}

func TestPerformanceGateValidateOnlyRejectsObviousRegression(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.20, 1.00), "--threshold", "0.10")
	if code != 1 {
		t.Fatalf("performance gate exit %d, want 1:\n%s", code, out)
	}
	performanceGateContains(t, out, "Performance gate violations")
	performanceGateContains(t, out, "numeric/hot_loop")
}

func TestPerformanceGateValidateOnlyRejectsLuaJITRatioAboveThreshold(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(0.81, 1.00,
		performanceGateWithBenchmarkID("numeric/matmul_dense"),
		performanceGateWithLuaJIT(1.00),
	), "--enforce-luajit")
	if code != 1 {
		t.Fatalf("performance gate exit %d, want 1:\n%s", code, out)
	}
	performanceGateContains(t, out, "Guard violations")
	performanceGateContains(t, out, "numeric/matmul_dense")
	performanceGateContains(t, out, "luajit")
}

func TestPerformanceGateValidateOnlyRejectsLowResolutionRows(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(nil, 1.00,
		performanceGateWithSubjectStatus("current", "low_resolution"),
	))
	if code != 1 {
		t.Fatalf("performance gate exit %d, want 1:\n%s", code, out)
	}
	performanceGateContains(t, out, "Unreliable timing rows")
	performanceGateContains(t, out, "low_resolution/ok")
}

func TestPerformanceGateWallTimedRowsNeedLargerRegressionToFail(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.20, 1.00,
		performanceGateWithSource("wall_repeat"),
	), "--threshold", "0.10")
	if code != 0 {
		t.Fatalf("performance gate exit %d:\n%s", code, out)
	}
	performanceGateContains(t, out, "wall_timed_startup_noise")

	out, code = runPerformanceGateValidate(t, root, performanceGateTimingPayload(1.40, 1.00,
		performanceGateWithSource("wall_repeat"),
	), "--threshold", "0.10", "--wall-threshold", "0.30")
	if code != 1 {
		t.Fatalf("performance gate exit %d, want 1:\n%s", code, out)
	}
	performanceGateContains(t, out, "wall_regression")
}

func TestPerformanceGateMixedTimingSourcesAreDiagnosticOnly(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	out, code := runPerformanceGateValidate(t, root, performanceGateTimingPayload(5.00, 1.00,
		performanceGateWithSource("script_repeat,wall_repeat"),
	), "--threshold", "0.10", "--wall-threshold", "0.30")
	if code != 0 {
		t.Fatalf("performance gate exit %d:\n%s", code, out)
	}
	performanceGateContains(t, out, "mixed_time_source")
	performanceGateContains(t, out, "wall_timed_startup_noise")
}

func TestPerformanceGateLuaJITSubmitGuardIsReportOnlyUnlessEnforced(t *testing.T) {
	root := repoRootForPerformanceGate(t)
	payload := performanceGateTimingPayload(1.00, 1.00,
		performanceGateWithBenchmarkID("numeric/matmul_dense"),
		performanceGateWithLuaJIT(1.00),
	)
	out, code := runPerformanceGateValidate(t, root, payload)
	if code != 0 {
		t.Fatalf("performance gate exit %d:\n%s", code, out)
	}
	performanceGateContains(t, out, "LuaJIT performance submit guard reported issues; treating as report-only")

	out, code = runPerformanceGateValidate(t, root, payload, "--enforce-luajit")
	if code != 1 {
		t.Fatalf("performance gate exit %d, want 1:\n%s", code, out)
	}
	performanceGateContains(t, out, "Guard violations")
	performanceGateContains(t, out, "numeric/matmul_dense")
}
