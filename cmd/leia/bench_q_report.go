package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const qReportMinTrustedGoBaselineNS = 100.0

var (
	qReportBenchRE       = regexp.MustCompile(`^(Benchmark[^\s]+)\s+(\d+)\s+([0-9.]+)\s+ns/op(?:\s+(.*))?$`)
	qReportBenchNoNSRE   = regexp.MustCompile(`^(Benchmark[^\s]+)\s+(\d+)\s+(.*)$`)
	qReportCPUSuffixRE   = regexp.MustCompile(`-\d+$`)
	qReportFallbackLine  = regexp.MustCompile(`q_pipeline_fallback_report\s+(.*)$`)
	qReportMilestoneKeys = map[string]bool{
		"max_leia_go_ratio":                    true,
		"max_leia_jit_go_ratio":                true,
		"max_leia_realdata_go_ratio":           true,
		"min_typed_hit_pct":                    true,
		"max_typed_fallbacks_op":               true,
		"max_pipeline_fallback_shapes":         true,
		"max_allocs_op":                        true,
		"min_runtime_jit_backend_benchmarks":   true,
		"min_runtime_array_bridge_benchmarks":  true,
		"min_runtime_backend_route_benchmarks": true,
		"min_runtime_backend_route_hits_op":    true,
		"max_runtime_backend_route_errors_op":  true,
		"min_q_eval_family_cases":              true,
		"min_q_session_planned_op_exit_op":     true,
	}
)

type qReportCommandResult struct {
	Label                string   `json:"label"`
	Cmd                  []string `json:"cmd"`
	ExitCode             int      `json:"exit_code"`
	Output               string   `json:"output"`
	ParsedBenchmarkCount int      `json:"parsed_benchmark_count"`
}

type qReportBenchRow struct {
	Name       string             `json:"name"`
	Iterations int                `json:"iterations"`
	NsOp       float64            `json:"ns_op"`
	Metrics    map[string]float64 `json:"metrics"`
}

type qReportGatePolicy struct {
	MaxLeiaGoRatio                     float64 `json:"max_leia_go_ratio"`
	MinTypedHitPct                     float64 `json:"min_typed_hit_pct"`
	MaxTypedFallbacksOp                float64 `json:"max_typed_fallbacks_op"`
	MaxPipelineFallbackShapes          float64 `json:"max_pipeline_fallback_shapes"`
	MaxAllocsOp                        float64 `json:"max_allocs_op"`
	MaxLeiaJITGoRatio                  float64 `json:"max_leia_jit_go_ratio"`
	MaxLeiaRealDataGoRatio             float64 `json:"max_leia_realdata_go_ratio"`
	MaxJITTypedErrorsOp                float64 `json:"max_jit_typed_errors_op"`
	MaxJITBackendSlowRoutePct          float64 `json:"max_jit_backend_slow_route_pct"`
	MinRuntimeDirectBridgeSharePct     float64 `json:"min_runtime_direct_bridge_share_pct"`
	MaxRuntimeAllocsPerDirectCall      float64 `json:"max_runtime_allocs_per_direct_call"`
	MinQArrayBridgeBulkHitPct          float64 `json:"min_q_array_bridge_bulk_hit_pct"`
	MaxQArrayBridgeFallbacksOp         float64 `json:"max_q_array_bridge_fallbacks_op"`
	MinRuntimeTypedPrimitiveBenchmarks int     `json:"min_runtime_typed_primitive_benchmarks"`
	MinRuntimeJITBackendBenchmarks     int     `json:"min_runtime_jit_backend_benchmarks"`
	MinRuntimeArrayBridgeBenchmarks    int     `json:"min_runtime_array_bridge_benchmarks"`
	MinRuntimeBridgeBenchmarkCount     int     `json:"min_runtime_bridge_benchmark_count"`
	MinQArrayBridgeRowsOp              float64 `json:"min_q_array_bridge_rows_op"`
	MaxQArrayBridgeAvgAllocsOp         float64 `json:"max_q_array_bridge_avg_allocs_op"`
	MaxQArrayBridgeMaxAllocsOp         float64 `json:"max_q_array_bridge_max_allocs_op"`
	MinRuntimeBackendRouteBenchmarks   int     `json:"min_runtime_backend_route_benchmarks"`
	MinRuntimeBackendRouteHitsOp       float64 `json:"min_runtime_backend_route_hits_op"`
	MaxRuntimeBackendRouteErrorsOp     float64 `json:"max_runtime_backend_route_errors_op"`
	MinQEvalFamilyCases                int     `json:"min_q_eval_family_cases"`
	MinQSessionPlannedOpExitOp         float64 `json:"min_q_session_planned_op_exit_op"`
}

type qReportGateCheck struct {
	Signal    string   `json:"signal"`
	Benchmark string   `json:"benchmark"`
	Value     *float64 `json:"value"`
	Threshold string   `json:"threshold"`
	Status    string   `json:"status"`
	Note      string   `json:"note"`
}

type qReportRatioRow struct {
	Scenario    string   `json:"scenario"`
	Numerator   string   `json:"numerator"`
	Denominator string   `json:"denominator"`
	Ratio       *float64 `json:"ratio"`
	Note        string   `json:"note"`
}

type qReportCurrentVsOldRow struct {
	Benchmark      string   `json:"benchmark"`
	Mode           string   `json:"mode"`
	CurrentSeconds *float64 `json:"current_seconds"`
	OldSeconds     *float64 `json:"old_seconds"`
	Ratio          *float64 `json:"ratio"`
	Source         string   `json:"source"`
}

type qReportFallbackTopRow struct {
	Category      string `json:"category"`
	PipelineShape string `json:"pipeline_shape"`
	Kernel        string `json:"kernel"`
	Reason        string `json:"reason"`
	Outcome       string `json:"outcome"`
	Count         int    `json:"count"`
}

type qReportConfig struct {
	Benchtime         string
	JSONPath          string
	MarkdownPath      string
	FromOutput        []string
	TimingJSON        []string
	Check             bool
	RatioBaselinePath string
	FallbackTopN      int
	Policy            qReportGatePolicy
	SeenFlags         map[string]bool
}

func runBenchQReportCommand(args []string, outw, errw io.Writer) int {
	cfg, err := parseQReportArgs(args, errw)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	root, err := qReportRepoRoot()
	if err != nil {
		fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
		return 1
	}
	if baseline := qReportLoadJSON(filepath.Join(root, cfg.RatioBaselinePath)); baseline != nil {
		qReportApplyMilestoneCaps(&cfg.Policy, baseline, cfg.SeenFlags)
	}

	rows := map[string]qReportBenchRow{}
	var commands []qReportCommandResult
	var fallbackRows []qReportFallbackTopRow
	if len(cfg.FromOutput) > 0 {
		for _, path := range cfg.FromOutput {
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
				return 1
			}
			parsed := qReportParseGoBenchmarks(string(data))
			qReportMergeRows(rows, parsed)
			fallbackRows = append(fallbackRows, qReportParseFallbackReports(string(data))...)
			commands = append(commands, qReportCommandResult{
				Label:                "from-output:" + path,
				Cmd:                  []string{"cat", path},
				ExitCode:             0,
				Output:               string(data),
				ParsedBenchmarkCount: len(parsed),
			})
		}
	} else {
		commands, rows, fallbackRows = qReportRunBenchmarks(root, cfg.Benchtime)
	}

	var currentVsOld []qReportCurrentVsOldRow
	for _, path := range cfg.TimingJSON {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
			return 1
		}
		items, err := qReportParseTimingJSON(data)
		if err != nil {
			fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
			return 1
		}
		currentVsOld = append(currentVsOld, items...)
	}

	sort.Slice(fallbackRows, func(i, j int) bool {
		a, b := fallbackRows[i], fallbackRows[j]
		if a.Count != b.Count {
			return a.Count > b.Count
		}
		return strings.Join([]string{a.Category, a.PipelineShape, a.Kernel, a.Reason, a.Outcome}, "\x00") <
			strings.Join([]string{b.Category, b.PipelineShape, b.Kernel, b.Reason, b.Outcome}, "\x00")
	})
	if cfg.FallbackTopN >= 0 && len(fallbackRows) > cfg.FallbackTopN {
		fallbackRows = fallbackRows[:cfg.FallbackTopN]
	}

	var checks []qReportGateCheck
	if cfg.Check {
		checks = qReportBuildGateChecks(rows, cfg.Policy)
	}
	payload := qReportBuildPayload(commands, rows, currentVsOld, checks, fallbackRows, cfg.Policy)
	if err := os.MkdirAll(filepath.Dir(cfg.JSONPath), 0o755); err != nil {
		fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
		return 1
	}
	if err := os.MkdirAll(filepath.Dir(cfg.MarkdownPath), 0o755); err != nil {
		fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
		return 1
	}
	jsonData, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
		return 1
	}
	jsonData = append(jsonData, '\n')
	if err := os.WriteFile(cfg.JSONPath, jsonData, 0o644); err != nil {
		fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
		return 1
	}
	if err := os.WriteFile(cfg.MarkdownPath, []byte(qReportMarkdown(rows, commands, currentVsOld, checks, fallbackRows)), 0o644); err != nil {
		fmt.Fprintf(errw, "leia bench q-report: %v\n", err)
		return 1
	}
	for _, command := range commands {
		if command.ExitCode != 0 {
			fmt.Fprintf(errw, "%s exited %d; parsed %d benchmark rows into the report\n", command.Label, command.ExitCode, command.ParsedBenchmarkCount)
			if command.ParsedBenchmarkCount == 0 {
				fmt.Fprintln(errw, command.Output)
				return command.ExitCode
			}
		}
	}
	fmt.Fprintf(outw, "wrote %s\n", cfg.MarkdownPath)
	fmt.Fprintf(outw, "wrote %s\n", cfg.JSONPath)
	if cfg.Check && qReportGateFailed(checks) {
		fmt.Fprintf(errw, "q performance gate failed: %d checks failed\n", qReportFailureCount(checks))
		return 2
	}
	return 0
}

func parseQReportArgs(args []string, errw io.Writer) (qReportConfig, error) {
	policy := qReportGatePolicy{
		MaxLeiaGoRatio:                     5,
		MinTypedHitPct:                     95,
		MaxTypedFallbacksOp:                0,
		MaxPipelineFallbackShapes:          0,
		MaxAllocsOp:                        64,
		MaxLeiaJITGoRatio:                  5,
		MaxLeiaRealDataGoRatio:             60,
		MaxJITTypedErrorsOp:                0,
		MaxJITBackendSlowRoutePct:          0,
		MinRuntimeDirectBridgeSharePct:     95,
		MaxRuntimeAllocsPerDirectCall:      32,
		MinQArrayBridgeBulkHitPct:          95,
		MaxQArrayBridgeFallbacksOp:         0,
		MinRuntimeTypedPrimitiveBenchmarks: 1,
		MinRuntimeJITBackendBenchmarks:     1,
		MinRuntimeArrayBridgeBenchmarks:    1,
		MinRuntimeBridgeBenchmarkCount:     3,
		MinQArrayBridgeRowsOp:              1,
		MaxQArrayBridgeAvgAllocsOp:         64,
		MaxQArrayBridgeMaxAllocsOp:         64,
		MinRuntimeBackendRouteBenchmarks:   1,
		MinRuntimeBackendRouteHitsOp:       1,
		MaxRuntimeBackendRouteErrorsOp:     0,
		MinQEvalFamilyCases:                1,
		MinQSessionPlannedOpExitOp:         0.9,
	}
	cfg := qReportConfig{
		Benchtime:         "100x",
		JSONPath:          filepath.Join("benchmarks", "data", "q_perf_report_latest.json"),
		MarkdownPath:      filepath.Join("benchmarks", "data", "q_perf_report_latest.md"),
		RatioBaselinePath: filepath.Join("benchmarks", "data", "qeval_go_ratio_baseline.json"),
		FallbackTopN:      20,
		Policy:            policy,
		SeenFlags:         qReportSeenFlags(args),
	}
	fs := flag.NewFlagSet("bench q-report", flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.StringVar(&cfg.Benchtime, "benchtime", cfg.Benchtime, "Go benchmark benchtime.")
	fs.StringVar(&cfg.JSONPath, "json", cfg.JSONPath, "JSON report path.")
	fs.StringVar(&cfg.MarkdownPath, "markdown", cfg.MarkdownPath, "Markdown report path.")
	fs.Var((*qReportStringList)(&cfg.FromOutput), "from-output", "Parse existing go test output; repeatable.")
	fs.Var((*qReportStringList)(&cfg.TimingJSON), "timing-json", "Include timing_compare JSON; repeatable.")
	fs.BoolVar(&cfg.Check, "check", false, "Fail if gate checks fail.")
	fs.StringVar(&cfg.RatioBaselinePath, "ratio-baseline", cfg.RatioBaselinePath, "Ratio baseline JSON with milestone caps.")
	fs.IntVar(&cfg.FallbackTopN, "fallback-top-n", cfg.FallbackTopN, "Maximum fallback rows to include; negative keeps all.")
	fs.Float64Var(&cfg.Policy.MaxLeiaGoRatio, "max-leia-go-ratio", cfg.Policy.MaxLeiaGoRatio, "")
	fs.Float64Var(&cfg.Policy.MaxLeiaJITGoRatio, "max-leia-jit-go-ratio", cfg.Policy.MaxLeiaJITGoRatio, "")
	fs.Float64Var(&cfg.Policy.MaxLeiaRealDataGoRatio, "max-leia-realdata-go-ratio", cfg.Policy.MaxLeiaRealDataGoRatio, "")
	fs.Float64Var(&cfg.Policy.MinTypedHitPct, "min-typed-hit-pct", cfg.Policy.MinTypedHitPct, "")
	fs.Float64Var(&cfg.Policy.MaxTypedFallbacksOp, "max-typed-fallbacks-op", cfg.Policy.MaxTypedFallbacksOp, "")
	fs.Float64Var(&cfg.Policy.MaxPipelineFallbackShapes, "max-pipeline-fallback-shapes", cfg.Policy.MaxPipelineFallbackShapes, "")
	fs.Float64Var(&cfg.Policy.MaxAllocsOp, "max-allocs-op", cfg.Policy.MaxAllocsOp, "")
	fs.Float64Var(&cfg.Policy.MaxJITTypedErrorsOp, "max-jit-typed-errors-op", cfg.Policy.MaxJITTypedErrorsOp, "")
	fs.Float64Var(&cfg.Policy.MaxJITBackendSlowRoutePct, "max-jit-backend-slow-route-pct", cfg.Policy.MaxJITBackendSlowRoutePct, "")
	fs.Float64Var(&cfg.Policy.MinRuntimeDirectBridgeSharePct, "min-runtime-direct-bridge-share-pct", cfg.Policy.MinRuntimeDirectBridgeSharePct, "")
	fs.Float64Var(&cfg.Policy.MaxRuntimeAllocsPerDirectCall, "max-runtime-allocs-per-direct-call", cfg.Policy.MaxRuntimeAllocsPerDirectCall, "")
	fs.Float64Var(&cfg.Policy.MinQArrayBridgeBulkHitPct, "min-q-array-bridge-bulk-hit-pct", cfg.Policy.MinQArrayBridgeBulkHitPct, "")
	fs.Float64Var(&cfg.Policy.MaxQArrayBridgeFallbacksOp, "max-q-array-bridge-fallbacks-op", cfg.Policy.MaxQArrayBridgeFallbacksOp, "")
	fs.IntVar(&cfg.Policy.MinRuntimeTypedPrimitiveBenchmarks, "min-runtime-typed-primitive-benchmarks", cfg.Policy.MinRuntimeTypedPrimitiveBenchmarks, "")
	fs.IntVar(&cfg.Policy.MinRuntimeJITBackendBenchmarks, "min-runtime-jit-backend-benchmarks", cfg.Policy.MinRuntimeJITBackendBenchmarks, "")
	fs.IntVar(&cfg.Policy.MinRuntimeArrayBridgeBenchmarks, "min-runtime-array-bridge-benchmarks", cfg.Policy.MinRuntimeArrayBridgeBenchmarks, "")
	fs.IntVar(&cfg.Policy.MinRuntimeBridgeBenchmarkCount, "min-runtime-bridge-benchmark-count", cfg.Policy.MinRuntimeBridgeBenchmarkCount, "")
	fs.Float64Var(&cfg.Policy.MinQArrayBridgeRowsOp, "min-q-array-bridge-rows-op", cfg.Policy.MinQArrayBridgeRowsOp, "")
	fs.Float64Var(&cfg.Policy.MaxQArrayBridgeAvgAllocsOp, "max-q-array-bridge-avg-allocs-op", cfg.Policy.MaxQArrayBridgeAvgAllocsOp, "")
	fs.Float64Var(&cfg.Policy.MaxQArrayBridgeMaxAllocsOp, "max-q-array-bridge-max-allocs-op", cfg.Policy.MaxQArrayBridgeMaxAllocsOp, "")
	fs.IntVar(&cfg.Policy.MinRuntimeBackendRouteBenchmarks, "min-runtime-backend-route-benchmarks", cfg.Policy.MinRuntimeBackendRouteBenchmarks, "")
	fs.Float64Var(&cfg.Policy.MinRuntimeBackendRouteHitsOp, "min-runtime-backend-route-hits-op", cfg.Policy.MinRuntimeBackendRouteHitsOp, "")
	fs.Float64Var(&cfg.Policy.MaxRuntimeBackendRouteErrorsOp, "max-runtime-backend-route-errors-op", cfg.Policy.MaxRuntimeBackendRouteErrorsOp, "")
	fs.IntVar(&cfg.Policy.MinQEvalFamilyCases, "min-q-eval-family-cases", cfg.Policy.MinQEvalFamilyCases, "")
	fs.Float64Var(&cfg.Policy.MinQSessionPlannedOpExitOp, "min-q-session-planned-op-exit-op", cfg.Policy.MinQSessionPlannedOpExitOp, "")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(errw, "leia bench q-report: unexpected argument %q\n", fs.Arg(0))
		return cfg, flag.ErrHelp
	}
	return cfg, nil
}

type qReportStringList []string

func (f *qReportStringList) String() string { return strings.Join(*f, ",") }
func (f *qReportStringList) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func qReportSeenFlags(args []string) map[string]bool {
	seen := map[string]bool{}
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		key := strings.TrimPrefix(arg, "--")
		if idx := strings.IndexByte(key, '='); idx >= 0 {
			key = key[:idx]
		}
		seen[strings.ReplaceAll(key, "-", "_")] = true
	}
	return seen
}

func qReportRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			if info, benchErr := os.Stat(filepath.Join(dir, "benchmarks")); benchErr == nil && info.IsDir() {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root from %s", dir)
		}
		dir = parent
	}
}

func qReportParseGoBenchmarks(output string) map[string]qReportBenchRow {
	rows := map[string]qReportBenchRow{}
	for _, line := range strings.Split(output, "\n") {
		stripped := strings.TrimSpace(line)
		if match := qReportBenchRE.FindStringSubmatch(stripped); match != nil {
			name := qReportNormalizeBenchName(match[1])
			iterations, _ := strconv.Atoi(match[2])
			nsOp, _ := strconv.ParseFloat(match[3], 64)
			rows[name] = qReportBenchRow{Name: name, Iterations: iterations, NsOp: nsOp, Metrics: qReportParseMetricPairs(match[4])}
			continue
		}
		match := qReportBenchNoNSRE.FindStringSubmatch(stripped)
		if match == nil || !strings.HasPrefix(match[1], "Benchmark") {
			continue
		}
		name := qReportNormalizeBenchName(match[1])
		iterations, _ := strconv.Atoi(match[2])
		rows[name] = qReportBenchRow{Name: name, Iterations: iterations, NsOp: 0, Metrics: qReportParseMetricPairs(match[3])}
	}
	return rows
}

func qReportNormalizeBenchName(raw string) string {
	return qReportCPUSuffixRE.ReplaceAllString(raw, "")
}

func qReportParseMetricPairs(text string) map[string]float64 {
	fields := strings.Fields(text)
	metrics := map[string]float64{}
	for i := 0; i+1 < len(fields); {
		value, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			i++
			continue
		}
		metrics[fields[i+1]] = value
		i += 2
	}
	return metrics
}

func qReportParseFallbackReports(output string) []qReportFallbackTopRow {
	counts := map[string]int{}
	rowsByKey := map[string]qReportFallbackTopRow{}
	for _, line := range strings.Split(output, "\n") {
		match := qReportFallbackLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		fields := qReportParseKeyValue(match[1])
		if rank := fields["rank"]; rank == "" || rank == "none" {
			continue
		}
		count, err := strconv.Atoi(fields["count"])
		if err != nil {
			continue
		}
		row := qReportFallbackTopRow{
			Category:      qReportDefault(fields["category"], "unknown"),
			PipelineShape: qReportDefault(fields["pipeline_shape"], "unknown"),
			Kernel:        qReportDefault(fields["kernel"], "unknown"),
			Reason:        qReportDefault(fields["reason"], "unknown"),
			Outcome:       qReportDefault(fields["outcome"], "unknown"),
		}
		key := strings.Join([]string{row.Category, row.PipelineShape, row.Kernel, row.Reason, row.Outcome}, "\x00")
		counts[key] += count
		rowsByKey[key] = row
	}
	out := make([]qReportFallbackTopRow, 0, len(rowsByKey))
	for key, row := range rowsByKey {
		row.Count = counts[key]
		out = append(out, row)
	}
	return out
}

func qReportParseKeyValue(text string) map[string]string {
	out := map[string]string{}
	for _, token := range strings.Fields(text) {
		key, value, ok := strings.Cut(token, "=")
		if ok {
			out[key] = value
		}
	}
	return out
}

func qReportRunBenchmarks(root, benchtime string) ([]qReportCommandResult, map[string]qReportBenchRow, []qReportFallbackTopRow) {
	specs := []struct {
		label string
		args  []string
	}{
		{"qsql-bind-native", []string{"test", "./internal/stdlib/bind", "-run", "^$", "-bench", `BenchmarkQSQL(Bind|DataRuntime|NativeGo)`, "-benchmem", "-benchtime=" + benchtime}},
		{"qeval-native", []string{"test", "./benchmarks", "-run", "^$", "-bench", `Benchmark(QEvalVector(ResultCacheWarm|Cold|GoBaseline)|QSessionEvalVectorWarmExecution|QEvalJITScriptWarm|QEvalRealData(Warm|GoBaseline))`, "-benchmem", "-benchtime=" + benchtime}},
		{"qjit-typed-runtime-callpath", []string{"test", "./internal/methodjit", "-run", "^$", "-bench", "BenchmarkQEvalPipelineNativeExitCallpath/CodegenNativeExit", "-benchmem", "-benchtime=" + benchtime}},
		{"qjit-array-runtime-bridge", []string{"test", "./internal/methodjit", "-run", "^$", "-bench", "BenchmarkQEvalPipelineArrayRuntimeBridge/Bulk", "-benchmem", "-benchtime=" + benchtime}},
		{"qjit-backend-route", []string{"test", "./internal/methodjit", "-run", "^$", "-bench", "BenchmarkQFrameVectorMethodJITRoute", "-benchmem", "-benchtime=" + benchtime}},
	}
	rows := map[string]qReportBenchRow{}
	var commands []qReportCommandResult
	var fallbackRows []qReportFallbackTopRow
	for _, spec := range specs {
		cmd := qReportRunGoCommand(root, spec.args)
		cmd.Label = spec.label
		parsed := qReportParseGoBenchmarks(cmd.Output)
		cmd.ParsedBenchmarkCount = len(parsed)
		qReportMergeRows(rows, parsed)
		fallbackRows = append(fallbackRows, qReportParseFallbackReports(cmd.Output)...)
		commands = append(commands, cmd)
	}
	return commands, rows, fallbackRows
}

func qReportRunGoCommand(root string, args []string) qReportCommandResult {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := benchExecCommand("go", args...)
	cmd.Dir = root
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := qReportRunWithContext(ctx, cmd)
	exitCode := 0
	if err != nil {
		exitCode = 1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			output.WriteString("\nq-report benchmark command timed out\n")
		}
	}
	return qReportCommandResult{Cmd: append([]string{"go"}, args...), ExitCode: exitCode, Output: output.String()}
}

func qReportRunWithContext(ctx context.Context, cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-done
		return ctx.Err()
	}
}

func qReportMergeRows(dst, src map[string]qReportBenchRow) {
	for name, row := range src {
		dst[name] = row
	}
}

func qReportBuildPayload(commands []qReportCommandResult, rows map[string]qReportBenchRow, currentVsOld []qReportCurrentVsOldRow, checks []qReportGateCheck, fallbackRows []qReportFallbackTopRow, policy qReportGatePolicy) map[string]any {
	return map[string]any{
		"commands":                          commands,
		"benchmarks":                        qReportSortedBenchMap(rows),
		"coverage":                          qReportBuildCoverage(rows, currentVsOld),
		"current_vs_old":                    currentVsOld,
		"runtime_metrics":                   qReportRuntimeMetricRows(rows),
		"jit_route_summary":                 qReportJITRouteSummary(rows),
		"runtime_observability_summary":     qReportRuntimeObservabilitySummary(rows),
		"runtime_health_summary":            qReportRuntimeHealthSummary(rows),
		"runtime_bridge_efficiency_summary": qReportRuntimeBridgeEfficiencySummary(rows),
		"runtime_array_bridge_summary":      qReportRuntimeArrayBridgeSummary(rows),
		"runtime_backend_route_summary":     qReportRuntimeBackendRouteSummary(rows),
		"pipeline_category_metrics":         qReportPipelineCategoryMetrics(rows),
		"pipeline_fallback_top":             fallbackRows,
		"qsql_benchmark_coverage":           qReportQSQLCoverage(rows),
		"q_eval_compute_coverage":           qReportQEvalComputeCoverage(rows),
		"q_eval_family_coverage":            qReportQEvalFamilyCoverage(rows),
		"q_eval_case_diagnostics":           qReportQEvalCaseDiagnostics(rows),
		"q_eval_realdata":                   qReportRealDataRows(rows),
		"ratios":                            qReportBuildRatios(rows),
		"fallback_shape_summary":            qReportFallbackShapeSummary(rows),
		"gate_policy":                       policy,
		"gate":                              checks,
	}
}

func qReportSortedBenchMap(rows map[string]qReportBenchRow) map[string]qReportBenchRow {
	out := map[string]qReportBenchRow{}
	for _, name := range qReportSortedNames(rows) {
		out[name] = rows[name]
	}
	return out
}

func qReportSortedNames(rows map[string]qReportBenchRow) []string {
	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func qReportRuntimeMetricRows(rows map[string]qReportBenchRow) []map[string]any {
	keys := []string{
		"B/op", "allocs/op", "kernel_hit_pct", "fallbacks/op",
		"typed_kernel_hit_pct", "typed_kernel_attempts/op", "typed_kernel_hits/op", "typed_kernel_fallbacks/op", "typed_kernel_errors/op", "typed_pipeline_shapes", "typed_pipeline_fallback_shapes",
		"data_runtime_hit_pct", "data_runtime_attempts/op", "data_runtime_hits/op", "data_runtime_fallbacks/op", "data_runtime_errors/op", "data_runtime_pipeline_shapes",
		"linalg_vector_attempts/op", "linalg_vector_hits/op", "linalg_vector_fallbacks/op", "linalg_vector_errors/op", "linalg_matrix_attempts/op", "linalg_matrix_hits/op", "linalg_matrix_fallbacks/op", "linalg_matrix_errors/op",
		"jit_typed_direct_return/op", "jit_typed_native_exit/op", "jit_typed_op_exit/op", "jit_typed_kernel_success/op", "jit_typed_kernel_errors/op", "jit_typed_pipeline_shapes",
		"q_session_planned_op_exit/op", "q_session_shell_fallback/op", "q_session_eval_errors/op", "q_session_backend_shapes",
		"q_array_bridge_bulk_hits/op", "q_array_bridge_fallbacks/op", "q_array_bridge_errors/op", "q_array_bridge_rows/op",
		"runtime_primitive_hits/op", "runtime_primitive_errors/op", "frame_runtime_primitive_hits/op", "frame_runtime_primitive_errors/op", "vector_runtime_primitive_hits/op", "vector_runtime_primitive_errors/op",
		"methodjit_frame_runtime_success/op", "methodjit_frame_runtime_errors/op", "methodjit_frame_runtime_direct_helper/op", "methodjit_frame_runtime_native_exit/op", "methodjit_frame_runtime_op_exit/op",
		"methodjit_vector_runtime_success/op", "methodjit_vector_runtime_errors/op", "methodjit_vector_runtime_direct_helper/op", "methodjit_vector_runtime_native_exit/op", "methodjit_vector_runtime_op_exit/op",
	}
	out := make([]map[string]any, 0, len(rows))
	for _, name := range qReportSortedNames(rows) {
		row := rows[name]
		item := map[string]any{"benchmark": name, "ns_op": row.NsOp}
		for _, key := range keys {
			item[qReportJSONMetricKey(key)] = qReportMetricValue(row, key)
		}
		out = append(out, item)
	}
	return out
}

func qReportJSONMetricKey(key string) string {
	switch key {
	case "B/op":
		return "bytes_op"
	case "allocs/op":
		return "allocs_op"
	}
	return strings.ReplaceAll(strings.ReplaceAll(key, "/", "_"), "-", "_")
}

func qReportMetricValue(row qReportBenchRow, key string) any {
	value, ok := row.Metrics[key]
	if !ok {
		return nil
	}
	return value
}

func qReportMetric(row qReportBenchRow, key string) *float64 {
	value, ok := row.Metrics[key]
	if !ok {
		return nil
	}
	return &value
}

func qReportMetricOrZero(row qReportBenchRow, key string) float64 {
	if value, ok := row.Metrics[key]; ok {
		return value
	}
	return 0
}

func qReportBuildRatios(rows map[string]qReportBenchRow) []qReportRatioRow {
	var out []qReportRatioRow
	add := func(scenario, numerator, denominator, note string) {
		out = append(out, qReportRatioRow{Scenario: scenario, Numerator: numerator, Denominator: denominator, Ratio: qReportRatio(rows, numerator, denominator), Note: note})
	}
	add("qSQL select/filter/project", "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLNativeGoSelectWhereProject", "")
	add("qSQL select/filter/project warm-vs-cold", "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject", "")
	for _, caseName := range qReportCases(rows, "BenchmarkQSQLBindMatrixWarm") {
		add("qSQL matrix "+caseName+" warm vs Go", "BenchmarkQSQLBindMatrixWarm/"+caseName, "BenchmarkQSQLNativeGoMatrix/"+caseName, "")
		add("qSQL matrix "+caseName+" cold vs warm", "BenchmarkQSQLBindMatrixCold/"+caseName, "BenchmarkQSQLBindMatrixWarm/"+caseName, "")
	}
	for _, caseName := range qReportCases(rows, "BenchmarkQSessionEvalVectorWarmExecution") {
		session := "BenchmarkQSessionEvalVectorWarmExecution/" + caseName
		goName := "BenchmarkQEvalVectorGoBaseline/" + caseName
		goRatio, note := qReportTrustedRatio(rows, session, goName)
		out = append(out, qReportRatioRow{"q.eval " + caseName + " session execution vs Go", session, goName, goRatio, note})
		add("q.eval "+caseName+" result-cache warm vs session execution", "BenchmarkQEvalVectorResultCacheWarm/"+caseName, session, "warm result-cache hits are not recomputation")
		add("q.eval "+caseName+" cold vs session execution", "BenchmarkQEvalVectorCold/"+caseName, session, "")
	}
	for _, caseName := range qReportCases(rows, "BenchmarkQEvalJITScriptWarm") {
		num := "BenchmarkQEvalJITScriptWarm/" + caseName
		goName := "BenchmarkQEvalVectorGoBaseline/" + caseName
		value, note := qReportTrustedRatio(rows, num, goName)
		if value != nil {
			note = "JIT-compiled q script warm execution vs hand-written Go"
		}
		out = append(out, qReportRatioRow{"q.eval " + caseName + " JIT script warm vs Go", num, goName, value, note})
	}
	for _, caseName := range qReportCases(rows, "BenchmarkQEvalVMScriptWarm") {
		num := "BenchmarkQEvalVMScriptWarm/" + caseName
		goName := "BenchmarkQEvalVectorGoBaseline/" + caseName
		value, note := qReportTrustedRatio(rows, num, goName)
		if value != nil {
			note = "VM script warm execution vs hand-written Go; attribution only, not gated"
		}
		out = append(out, qReportRatioRow{"q.eval " + caseName + " VM script warm vs Go", num, goName, value, note})
	}
	for _, caseName := range qReportCases(rows, "BenchmarkQEvalRealDataWarm") {
		num := "BenchmarkQEvalRealDataWarm/" + caseName
		goName := "BenchmarkQEvalRealDataGoBaseline/" + caseName
		value, note := qReportTrustedRatio(rows, num, goName)
		if value != nil {
			note = "env-injected dense columns; closed forms cannot fire"
		}
		out = append(out, qReportRatioRow{"q.eval realdata " + caseName + " warm vs Go", num, goName, value, note})
	}
	return out
}

func qReportRatio(rows map[string]qReportBenchRow, numerator, denominator string) *float64 {
	left, lok := rows[numerator]
	right, rok := rows[denominator]
	if !lok || !rok || right.NsOp == 0 {
		return nil
	}
	value := left.NsOp / right.NsOp
	return &value
}

func qReportTrustedRatio(rows map[string]qReportBenchRow, numerator, denominator string) (*float64, string) {
	goRow, ok := rows[denominator]
	if !ok {
		return nil, "missing Go baseline"
	}
	if goRow.NsOp < qReportMinTrustedGoBaselineNS {
		return nil, fmt.Sprintf("Go baseline is %g ns/op, below %g ns/op; treat as correctness oracle or constant-folded baseline, not a performance denominator", goRow.NsOp, qReportMinTrustedGoBaselineNS)
	}
	return qReportRatio(rows, numerator, denominator), "session eval bypasses q.eval result cache and measures repeated execution"
}

func qReportCases(rows map[string]qReportBenchRow, prefix string) []string {
	prefix += "/"
	seen := map[string]bool{}
	for name := range rows {
		if strings.HasPrefix(name, prefix) {
			seen[strings.TrimPrefix(name, prefix)] = true
		}
	}
	cases := make([]string, 0, len(seen))
	for name := range seen {
		cases = append(cases, name)
	}
	sort.Strings(cases)
	return cases
}

func qReportBuildGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	checks = append(checks, qReportRatioGateChecks(rows, policy)...)
	checks = append(checks, qReportRuntimeGateChecks(rows, policy)...)
	checks = append(checks, qReportObservabilityGateChecks(rows, policy)...)
	checks = append(checks, qReportRuntimeHealthGateChecks(rows, policy)...)
	checks = append(checks, qReportBridgeGateChecks(rows, policy)...)
	checks = append(checks, qReportArrayBridgeGateChecks(rows, policy)...)
	checks = append(checks, qReportBackendRouteGateChecks(rows, policy)...)
	checks = append(checks, qReportRuntimeContractGateChecks(rows, policy)...)
	checks = append(checks, qReportFamilyCoverageGateChecks(rows, policy)...)
	return checks
}

func qReportRatioGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	for _, ratio := range qReportBuildRatios(rows) {
		if !strings.Contains(ratio.Denominator, "Go") || strings.HasPrefix(ratio.Numerator, "BenchmarkQEvalVMScriptWarm/") {
			continue
		}
		signal := "leia_go_ratio"
		cap := policy.MaxLeiaGoRatio
		if strings.HasPrefix(ratio.Numerator, "BenchmarkQEvalJITScriptWarm/") {
			signal = "leia_jit_go_ratio"
			cap = policy.MaxLeiaJITGoRatio
		}
		if strings.HasPrefix(ratio.Numerator, "BenchmarkQEvalRealDataWarm/") {
			signal = "leia_realdata_go_ratio"
			cap = policy.MaxLeiaRealDataGoRatio
		}
		status := "skip"
		if ratio.Ratio != nil {
			status = qReportPassFail(*ratio.Ratio <= cap)
		}
		checks = append(checks, qReportGateCheck{Signal: signal, Benchmark: ratio.Numerator, Value: ratio.Ratio, Threshold: fmt.Sprintf("<= %g", cap), Status: status, Note: ratio.Note})
	}
	return checks
}

func qReportRuntimeGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	for _, name := range qReportSortedNames(rows) {
		row := rows[name]
		if strings.HasPrefix(name, "BenchmarkQEvalRealDataWarm/") || strings.HasPrefix(name, "BenchmarkQEvalRealDataGoBaseline/") {
			continue
		}
		if value := qReportMetric(row, "typed_kernel_hit_pct"); value != nil {
			checks = append(checks, qReportGateCheck{"typed_hit_pct", name, value, fmt.Sprintf(">= %g", policy.MinTypedHitPct), qReportPassFail(*value >= policy.MinTypedHitPct), ""})
		}
		fallback := qReportMetric(row, "typed_kernel_fallbacks/op")
		if fallback == nil {
			fallback = qReportMetric(row, "fallbacks/op")
		}
		if fallback != nil {
			checks = append(checks, qReportGateCheck{"fallbacks_op", name, fallback, fmt.Sprintf("<= %g", policy.MaxTypedFallbacksOp), qReportPassFail(*fallback <= policy.MaxTypedFallbacksOp), ""})
		}
		if value := qReportMetric(row, "typed_pipeline_fallback_shapes"); value != nil {
			checks = append(checks, qReportGateCheck{"pipeline_fallback_shapes", name, value, fmt.Sprintf("<= %g", policy.MaxPipelineFallbackShapes), qReportPassFail(*value <= policy.MaxPipelineFallbackShapes), ""})
		}
		if value := qReportMetric(row, "allocs/op"); value != nil {
			checks = append(checks, qReportGateCheck{"allocs_op", name, value, fmt.Sprintf("<= %g", policy.MaxAllocsOp), qReportPassFail(*value <= policy.MaxAllocsOp), ""})
		}
		if value := qReportMetric(row, "jit_typed_kernel_errors/op"); value != nil {
			checks = append(checks, qReportGateCheck{"jit_typed_errors_op", name, value, fmt.Sprintf("<= %g", policy.MaxJITTypedErrorsOp), qReportPassFail(*value <= policy.MaxJITTypedErrorsOp), ""})
		}
		if value := qReportMetric(row, "q_session_shell_fallback/op"); value != nil {
			checks = append(checks, qReportGateCheck{"q_session_shell_fallback_op", name, value, fmt.Sprintf("<= %g", policy.MaxTypedFallbacksOp), qReportPassFail(*value <= policy.MaxTypedFallbacksOp), ""})
		}
		if value := qReportMetric(row, "q_session_eval_errors/op"); value != nil {
			checks = append(checks, qReportGateCheck{"q_session_eval_errors_op", name, value, fmt.Sprintf("<= %g", policy.MaxJITTypedErrorsOp), qReportPassFail(*value <= policy.MaxJITTypedErrorsOp), ""})
		}
		if strings.HasPrefix(name, "BenchmarkQEvalJITScriptWarm/") {
			present := qReportHasMetrics(row, "q_session_planned_op_exit/op", "q_session_shell_fallback/op", "q_session_eval_errors/op", "q_session_backend_shapes")
			value := 0.0
			if present {
				value = 1
			}
			checks = append(checks, qReportGateCheck{"q_session_route_metrics_present", name, &value, "present", qReportPassFail(present), ""})
			if planned := qReportMetric(row, "q_session_planned_op_exit/op"); planned != nil {
				checks = append(checks, qReportGateCheck{"q_session_planned_op_exit_op", name, planned, fmt.Sprintf(">= %g", policy.MinQSessionPlannedOpExitOp), qReportPassFail(*planned >= policy.MinQSessionPlannedOpExitOp), ""})
			}
			if shapes := qReportMetric(row, "q_session_backend_shapes"); shapes != nil {
				checks = append(checks, qReportGateCheck{"q_session_backend_shapes", name, shapes, ">= 1", qReportPassFail(*shapes >= 1), ""})
			}
		}
	}
	return checks
}

func qReportHasMetrics(row qReportBenchRow, keys ...string) bool {
	for _, key := range keys {
		if _, ok := row.Metrics[key]; !ok {
			return false
		}
	}
	return true
}

func qReportObservabilityGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	for _, item := range qReportRuntimeObservabilitySummary(rows) {
		layer := qReportString(item["layer"])
		switch layer {
		case "typed_primitive":
			if value := qReportFloatPtr(item["hit_pct"]); value != nil {
				checks = append(checks, qReportGateCheck{"typed_primitive_hit_pct", layer, value, fmt.Sprintf(">= %g", policy.MinTypedHitPct), qReportPassFail(*value >= policy.MinTypedHitPct), qReportString(item["note"])})
			}
			if value := qReportFloatPtr(item["fallbacks_op"]); value != nil {
				checks = append(checks, qReportGateCheck{"typed_primitive_fallbacks_op", layer, value, fmt.Sprintf("<= %g", policy.MaxTypedFallbacksOp), qReportPassFail(*value <= policy.MaxTypedFallbacksOp), qReportString(item["note"])})
			}
		case "unified_pipeline":
			if value := qReportFloatPtr(item["fallback_shapes"]); value != nil {
				checks = append(checks, qReportGateCheck{"unified_pipeline_fallback_shapes", layer, value, fmt.Sprintf("<= %g", policy.MaxPipelineFallbackShapes), qReportPassFail(*value <= policy.MaxPipelineFallbackShapes), qReportString(item["note"])})
			}
		case "jit_backend":
			if value := qReportFloatPtr(item["errors_op"]); value != nil {
				checks = append(checks, qReportGateCheck{"jit_backend_errors_op", layer, value, fmt.Sprintf("<= %g", policy.MaxJITTypedErrorsOp), qReportPassFail(*value <= policy.MaxJITTypedErrorsOp), qReportString(item["note"])})
			}
			if value := qReportFloatPtr(item["slow_route_pct"]); value != nil {
				checks = append(checks, qReportGateCheck{"jit_backend_slow_route_pct", layer, value, fmt.Sprintf("<= %g", policy.MaxJITBackendSlowRoutePct), qReportPassFail(*value <= policy.MaxJITBackendSlowRoutePct), qReportString(item["note"])})
			}
		}
	}
	return checks
}

func qReportRuntimeHealthGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	for _, item := range qReportRuntimeHealthSummary(rows) {
		scope := qReportString(item["scope"])
		for _, spec := range []struct {
			signal    string
			key       string
			threshold float64
			op        string
		}{
			{"runtime_health_typed_fallbacks_op", "typed_fallbacks_op", policy.MaxTypedFallbacksOp, "<="},
			{"runtime_health_typed_errors_op", "typed_errors_op", policy.MaxJITTypedErrorsOp, "<="},
			{"runtime_health_pipeline_fallback_shapes", "pipeline_fallback_shapes", policy.MaxPipelineFallbackShapes, "<="},
			{"runtime_health_max_allocs_op", "max_allocs_op", policy.MaxAllocsOp, "<="},
			{"runtime_health_jit_slow_route_pct", "jit_slow_route_pct", policy.MaxJITBackendSlowRoutePct, "<="},
		} {
			value := qReportFloatPtr(item[spec.key])
			if value == nil {
				continue
			}
			checks = append(checks, qReportGateCheck{spec.signal, scope, value, fmt.Sprintf("%s %g", spec.op, spec.threshold), qReportPassFail(*value <= spec.threshold), qReportString(item["note"])})
		}
	}
	return checks
}

func qReportBridgeGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	for _, item := range qReportRuntimeBridgeEfficiencySummary(rows) {
		scope := qReportString(item["scope"])
		if value := qReportFloatPtr(item["direct_call_share_pct"]); value != nil {
			checks = append(checks, qReportGateCheck{"runtime_bridge_direct_call_share_pct", scope, value, fmt.Sprintf(">= %g", policy.MinRuntimeDirectBridgeSharePct), qReportPassFail(*value >= policy.MinRuntimeDirectBridgeSharePct), qReportString(item["note"])})
		}
		if value := qReportFloatPtr(item["allocs_per_direct_call"]); value != nil {
			checks = append(checks, qReportGateCheck{"runtime_bridge_allocs_per_direct_call", scope, value, fmt.Sprintf("<= %g", policy.MaxRuntimeAllocsPerDirectCall), qReportPassFail(*value <= policy.MaxRuntimeAllocsPerDirectCall), qReportString(item["note"])})
		}
	}
	return checks
}

func qReportArrayBridgeGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	for _, item := range qReportRuntimeArrayBridgeSummary(rows) {
		scope := qReportString(item["scope"])
		if value := qReportFloatPtr(item["bulk_hit_pct"]); value != nil {
			checks = append(checks, qReportGateCheck{"q_array_bridge_bulk_hit_pct", scope, value, fmt.Sprintf(">= %g", policy.MinQArrayBridgeBulkHitPct), qReportPassFail(*value >= policy.MinQArrayBridgeBulkHitPct), qReportString(item["note"])})
		}
		for _, spec := range []struct {
			signal    string
			key       string
			threshold float64
			op        string
		}{
			{"q_array_bridge_fallbacks_op", "fallbacks_op", policy.MaxQArrayBridgeFallbacksOp, "<="},
			{"q_array_bridge_rows_op", "rows_op", policy.MinQArrayBridgeRowsOp, ">="},
			{"q_array_bridge_avg_allocs_op", "avg_allocs_op", policy.MaxQArrayBridgeAvgAllocsOp, "<="},
			{"q_array_bridge_max_allocs_op", "max_allocs_op", policy.MaxQArrayBridgeMaxAllocsOp, "<="},
		} {
			value := qReportFloatPtr(item[spec.key])
			if value == nil {
				continue
			}
			pass := *value <= spec.threshold
			if spec.op == ">=" {
				pass = *value >= spec.threshold
			}
			checks = append(checks, qReportGateCheck{spec.signal, scope, value, fmt.Sprintf("%s %g", spec.op, spec.threshold), qReportPassFail(pass), qReportString(item["note"])})
		}
	}
	return checks
}

func qReportBackendRouteGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	summary := qReportRuntimeBackendRouteSummary(rows)
	if len(summary) == 0 {
		zero := 0.0
		scope := "runtime_primitive_registry_and_frame_vector_routes"
		return []qReportGateCheck{
			{"runtime_backend_route_benchmarks", scope, &zero, fmt.Sprintf(">= %d", policy.MinRuntimeBackendRouteBenchmarks), qReportPassFail(policy.MinRuntimeBackendRouteBenchmarks <= 0), "runtime primitive registry or MethodJIT frame/vector route counters must be present so backend statistics cannot silently disappear"},
			{"runtime_backend_route_hits_op", scope, &zero, fmt.Sprintf(">= %g", policy.MinRuntimeBackendRouteHitsOp), qReportPassFail(policy.MinRuntimeBackendRouteHitsOp <= 0), "backend route counters are missing"},
		}
	}
	var checks []qReportGateCheck
	for _, item := range summary {
		scope := qReportString(item["scope"])
		benchCount := qReportFloatPtr(item["benchmark_count"])
		hits := qReportFloatPtr(item["hits_op"])
		errorsOp := qReportFloatPtr(item["errors_op"])
		checks = append(checks, qReportGateCheck{"runtime_backend_route_benchmarks", scope, benchCount, fmt.Sprintf(">= %d", policy.MinRuntimeBackendRouteBenchmarks), qReportPassFail(benchCount != nil && *benchCount >= float64(policy.MinRuntimeBackendRouteBenchmarks)), qReportString(item["note"])})
		checks = append(checks, qReportGateCheck{"runtime_backend_route_hits_op", scope, hits, fmt.Sprintf(">= %g", policy.MinRuntimeBackendRouteHitsOp), qReportPassFail(hits != nil && *hits >= policy.MinRuntimeBackendRouteHitsOp), qReportString(item["note"])})
		checks = append(checks, qReportGateCheck{"runtime_backend_route_errors_op", scope, errorsOp, fmt.Sprintf("<= %g", policy.MaxRuntimeBackendRouteErrorsOp), qReportPassFail(errorsOp != nil && *errorsOp <= policy.MaxRuntimeBackendRouteErrorsOp), qReportString(item["note"])})
	}
	return checks
}

func qReportRuntimeContractGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	obs := map[string]map[string]any{}
	for _, item := range qReportRuntimeObservabilitySummary(rows) {
		obs[qReportString(item["layer"])] = item
	}
	bridgeCount := 0.0
	if bridge := qReportRuntimeBridgeEfficiencySummary(rows); len(bridge) > 0 {
		bridgeCount = qReportFloat(bridge[0]["benchmark_count"])
	}
	reqs := []struct {
		signal    string
		benchmark string
		value     float64
		threshold float64
		note      string
	}{
		{"runtime_contract_typed_primitive_benchmarks", "typed_primitive", qReportFloat(obs["typed_primitive"]["benchmark_count"]), float64(policy.MinRuntimeTypedPrimitiveBenchmarks), "typed primitive counters must be present so typed kernel hit/fallback rates cannot silently disappear"},
		{"runtime_contract_jit_backend_benchmarks", "jit_backend", qReportFloat(obs["jit_backend"]["benchmark_count"]), float64(policy.MinRuntimeJITBackendBenchmarks), "JIT route counters must be present so direct-return versus slow exits remain observable"},
		{"runtime_contract_array_bridge_benchmarks", "methodjit_array_bridge", qReportFloat(obs["methodjit_array_bridge"]["benchmark_count"]), float64(policy.MinRuntimeArrayBridgeBenchmarks), "array bridge counters must be present so bulk export regressions cannot be hidden"},
		{"runtime_contract_bridge_benchmark_count", "typed_runtime_and_jit_backend", bridgeCount, float64(policy.MinRuntimeBridgeBenchmarkCount), "runtime bridge efficiency should aggregate typed primitive, JIT backend, and array bridge rows"},
	}
	var checks []qReportGateCheck
	for _, req := range reqs {
		value := req.value
		checks = append(checks, qReportGateCheck{req.signal, req.benchmark, &value, fmt.Sprintf(">= %g", req.threshold), qReportPassFail(value >= req.threshold), req.note})
	}
	return checks
}

func qReportFamilyCoverageGateChecks(rows map[string]qReportBenchRow, policy qReportGatePolicy) []qReportGateCheck {
	var checks []qReportGateCheck
	threshold := float64(policy.MinQEvalFamilyCases)
	for _, item := range qReportQEvalFamilyCoverage(rows) {
		family := qReportString(item["family"])
		session := qReportFloat(item["session_case_count"])
		goMatched := qReportFloat(item["matched_go_baseline_count"])
		jitMatched := qReportFloat(item["matched_jit_case_count"])
		checks = append(checks, qReportGateCheck{"q_eval_family_session_cases", family, &session, fmt.Sprintf(">= %g", threshold), qReportPassFail(session >= threshold), qReportString(item["note"])})
		checks = append(checks, qReportGateCheck{"q_eval_family_go_baseline_cases", family, &goMatched, fmt.Sprintf(">= %g", threshold), qReportPassFail(goMatched >= threshold), "same-case hand-written Go baseline rows are required"})
		checks = append(checks, qReportGateCheck{"q_eval_family_jit_cases", family, &jitMatched, fmt.Sprintf(">= %g", threshold), qReportPassFail(jitMatched >= threshold), "same-case BenchmarkQEvalJITScriptWarm rows are required"})
	}
	return checks
}

func qReportRuntimeObservabilitySummary(rows map[string]qReportBenchRow) []map[string]any {
	var out []map[string]any
	qsqlRows := qReportRowsWithAny(rows, "kernel_hit_pct", "fallbacks/op")
	if len(qsqlRows) > 0 {
		out = append(out, map[string]any{"layer": "qsql_kernel", "benchmark_count": len(qsqlRows), "fallbacks_op": qReportSumMetric(qsqlRows, "fallbacks/op"), "hit_pct": qReportAverageMetric(qsqlRows, "kernel_hit_pct"), "note": "qSQL bind/runtime kernel metrics emitted directly by qSQL benchmarks"})
	}
	typedRows := qReportRowsWithAny(rows, "typed_kernel_attempts/op")
	if len(typedRows) > 0 {
		attempts := qReportSumMetric(typedRows, "typed_kernel_attempts/op")
		hits := qReportSumMetric(typedRows, "typed_kernel_hits/op")
		out = append(out, map[string]any{"layer": "typed_primitive", "benchmark_count": len(typedRows), "attempts_op": attempts, "hits_op": hits, "fallbacks_op": qReportSumMetric(typedRows, "typed_kernel_fallbacks/op"), "errors_op": qReportSumMetric(typedRows, "typed_kernel_errors/op"), "hit_pct": qReportPercent(hits, attempts), "note": "ordinary q typed primitive dispatch across session-execution benchmarks"})
	}
	pipelineRows := qReportRowsWithAny(rows, "typed_pipeline_shapes")
	if len(pipelineRows) > 0 {
		shapes := qReportSumMetric(pipelineRows, "typed_pipeline_shapes")
		fallbacks := qReportSumMetric(pipelineRows, "typed_pipeline_fallback_shapes")
		out = append(out, map[string]any{"layer": "unified_pipeline", "benchmark_count": len(pipelineRows), "hit_pct": qReportPercent(shapes-fallbacks, shapes), "shapes": shapes, "fallback_shapes": fallbacks, "note": "recognized q expression pipeline shapes and shapes that still fell back"})
	}
	jitRows := qReportRowsWithAny(rows, "jit_typed_direct_return/op", "jit_typed_native_exit/op", "jit_typed_op_exit/op", "jit_typed_kernel_success/op", "jit_typed_kernel_errors/op", "q_session_planned_op_exit/op", "q_session_shell_fallback/op", "q_session_eval_errors/op")
	if len(jitRows) > 0 {
		direct := qReportSumMetric(jitRows, "jit_typed_direct_return/op")
		native := qReportSumMetric(jitRows, "jit_typed_native_exit/op")
		opExit := qReportSumMetric(jitRows, "jit_typed_op_exit/op")
		success := qReportSumMetric(jitRows, "jit_typed_kernel_success/op")
		errorsOp := qReportSumMetric(jitRows, "jit_typed_kernel_errors/op")
		sessionPlanned := qReportSumMetric(jitRows, "q_session_planned_op_exit/op")
		sessionShell := qReportSumMetric(jitRows, "q_session_shell_fallback/op")
		sessionErrors := qReportSumMetric(jitRows, "q_session_eval_errors/op")
		kernelTotal := success + errorsOp
		if kernelTotal == 0 && sessionPlanned+sessionShell+sessionErrors > 0 {
			kernelTotal = sessionPlanned + sessionShell + sessionErrors
			success = sessionPlanned
			errorsOp = sessionErrors
		}
		slowPct := qReportSlowRoutePct(direct, native, opExit, sessionPlanned, sessionShell, sessionErrors)
		out = append(out, map[string]any{"layer": "jit_backend", "benchmark_count": len(jitRows), "attempts_op": qReportNilIfZero(kernelTotal), "hits_op": success, "errors_op": errorsOp, "hit_pct": qReportPercent(success, kernelTotal), "shapes": qReportSumMetric(jitRows, "jit_typed_pipeline_shapes") + qReportSumMetric(jitRows, "q_session_backend_shapes"), "direct_return_op": direct + sessionPlanned, "native_exit_op": native, "op_exit_op": opExit + sessionShell, "slow_route_pct": slowPct, "note": "JIT typed backend route split; session planned exits are the steady q session hot route"})
	}
	arrayRows := qReportRowsWithAny(rows, "q_array_bridge_bulk_hits/op", "q_array_bridge_fallbacks/op", "q_array_bridge_errors/op")
	if len(arrayRows) > 0 {
		bulk := qReportSumMetric(arrayRows, "q_array_bridge_bulk_hits/op")
		fallbacks := qReportSumMetric(arrayRows, "q_array_bridge_fallbacks/op")
		errorsOp := qReportSumMetric(arrayRows, "q_array_bridge_errors/op")
		attempts := bulk + fallbacks + errorsOp
		out = append(out, map[string]any{"layer": "methodjit_array_bridge", "benchmark_count": len(arrayRows), "attempts_op": attempts, "hits_op": bulk, "fallbacks_op": fallbacks, "errors_op": errorsOp, "hit_pct": qReportPercent(bulk, attempts), "shapes": qReportSumMetric(arrayRows, "q_array_bridge_rows/op"), "note": "MethodJIT q pipeline data.Array to runtime.Value bridge; hits use bulk typed export"})
	}
	return out
}

func qReportRuntimeHealthSummary(rows map[string]qReportBenchRow) []map[string]any {
	healthRows := qReportRowsWithAny(rows, "typed_kernel_attempts/op", "typed_pipeline_shapes", "jit_typed_direct_return/op", "jit_typed_native_exit/op", "jit_typed_op_exit/op", "jit_typed_kernel_success/op", "jit_typed_kernel_errors/op", "q_session_planned_op_exit/op", "q_session_shell_fallback/op", "q_session_eval_errors/op", "q_array_bridge_bulk_hits/op", "q_array_bridge_fallbacks/op", "q_array_bridge_errors/op")
	if len(healthRows) == 0 {
		return nil
	}
	allocs := qReportMetricValues(healthRows, "allocs/op")
	direct := qReportSumMetric(healthRows, "jit_typed_direct_return/op")
	native := qReportSumMetric(healthRows, "jit_typed_native_exit/op")
	opExit := qReportSumMetric(healthRows, "jit_typed_op_exit/op")
	sessionPlanned := qReportSumMetric(healthRows, "q_session_planned_op_exit/op")
	sessionShell := qReportSumMetric(healthRows, "q_session_shell_fallback/op")
	sessionErrors := qReportSumMetric(healthRows, "q_session_eval_errors/op")
	return []map[string]any{{
		"scope":                    "q_runtime_hotpath",
		"benchmark_count":          len(healthRows),
		"avg_allocs_op":            qReportAverage(allocs),
		"max_allocs_op":            qReportMax(allocs),
		"typed_fallbacks_op":       qReportSumMetric(healthRows, "typed_kernel_fallbacks/op"),
		"typed_errors_op":          qReportSumMetric(healthRows, "typed_kernel_errors/op") + qReportSumMetric(healthRows, "jit_typed_kernel_errors/op") + qReportSumMetric(healthRows, "q_session_eval_errors/op"),
		"pipeline_fallback_shapes": qReportSumMetric(healthRows, "typed_pipeline_fallback_shapes"),
		"jit_direct_return_op":     direct + sessionPlanned,
		"jit_native_exit_op":       native,
		"jit_op_exit_op":           opExit + sessionShell,
		"jit_slow_route_pct":       qReportSlowRoutePct(direct, native, opExit, sessionPlanned, sessionShell, sessionErrors),
		"note":                     "combined health of typed primitive fallback, pipeline fallback, JIT route split, and allocation pressure",
	}}
}

func qReportRuntimeBridgeEfficiencySummary(rows map[string]qReportBenchRow) []map[string]any {
	bridgeRows := qReportRowsWithAny(rows, "typed_kernel_attempts/op", "jit_typed_direct_return/op", "jit_typed_native_exit/op", "jit_typed_op_exit/op", "jit_typed_kernel_success/op", "jit_typed_kernel_errors/op", "q_session_planned_op_exit/op", "q_session_shell_fallback/op", "q_session_eval_errors/op", "q_array_bridge_bulk_hits/op", "q_array_bridge_fallbacks/op", "q_array_bridge_errors/op")
	if len(bridgeRows) == 0 {
		return nil
	}
	direct := qReportSumMetric(bridgeRows, "typed_kernel_hits/op") + qReportSumMetric(bridgeRows, "jit_typed_direct_return/op") + qReportSumMetric(bridgeRows, "q_session_planned_op_exit/op") + qReportSumMetric(bridgeRows, "q_array_bridge_bulk_hits/op")
	slow := qReportSumMetric(bridgeRows, "typed_kernel_fallbacks/op") + qReportSumMetric(bridgeRows, "typed_kernel_errors/op") + qReportSumMetric(bridgeRows, "jit_typed_native_exit/op") + qReportSumMetric(bridgeRows, "jit_typed_op_exit/op") + qReportSumMetric(bridgeRows, "jit_typed_kernel_errors/op") + qReportSumMetric(bridgeRows, "q_session_shell_fallback/op") + qReportSumMetric(bridgeRows, "q_session_eval_errors/op") + qReportSumMetric(bridgeRows, "q_array_bridge_fallbacks/op") + qReportSumMetric(bridgeRows, "q_array_bridge_errors/op")
	avgAllocs := qReportAverage(qReportMetricValues(bridgeRows, "allocs/op"))
	var allocsPerDirect any
	if avgAllocs != nil && direct > 0 {
		allocsPerDirect = qReportFloat(avgAllocs) / direct
	}
	return []map[string]any{{"scope": "typed_runtime_and_jit_backend", "benchmark_count": len(bridgeRows), "direct_calls_op": direct, "slow_bridge_calls_op": slow, "direct_call_share_pct": qReportPercent(direct, direct+slow), "avg_allocs_op": avgAllocs, "allocs_per_direct_call": allocsPerDirect, "note": "direct calls combine typed primitive hits, JIT direct returns, and q session planned exits; slow bridge calls combine typed fallback/error, JIT native/op exits, session shell fallback, and errors"}}
}

func qReportRuntimeArrayBridgeSummary(rows map[string]qReportBenchRow) []map[string]any {
	arrayRows := qReportRowsWithAny(rows, "q_array_bridge_bulk_hits/op", "q_array_bridge_fallbacks/op", "q_array_bridge_errors/op")
	if len(arrayRows) == 0 {
		return nil
	}
	bulk := qReportSumMetric(arrayRows, "q_array_bridge_bulk_hits/op")
	fallbacks := qReportSumMetric(arrayRows, "q_array_bridge_fallbacks/op")
	errorsOp := qReportSumMetric(arrayRows, "q_array_bridge_errors/op")
	attempts := bulk + fallbacks + errorsOp
	allocs := qReportMetricValues(arrayRows, "allocs/op")
	return []map[string]any{{"scope": "methodjit_array_bridge", "benchmark_count": len(arrayRows), "attempts_op": attempts, "bulk_hits_op": bulk, "fallbacks_op": fallbacks, "errors_op": errorsOp, "bulk_hit_pct": qReportPercent(bulk, attempts), "rows_op": qReportSumMetric(arrayRows, "q_array_bridge_rows/op"), "avg_allocs_op": qReportAverage(allocs), "max_allocs_op": qReportMax(allocs), "note": "MethodJIT q array bridge route split; bulk hits avoid row-wise Array.At fallback"}}
}

func qReportRuntimeBackendRouteSummary(rows map[string]qReportBenchRow) []map[string]any {
	routeRows := qReportRowsWithAny(rows, "runtime_primitive_hits/op", "runtime_primitive_errors/op", "frame_runtime_primitive_hits/op", "frame_runtime_primitive_errors/op", "vector_runtime_primitive_hits/op", "vector_runtime_primitive_errors/op", "methodjit_frame_runtime_success/op", "methodjit_frame_runtime_errors/op", "methodjit_frame_runtime_direct_helper/op", "methodjit_frame_runtime_native_exit/op", "methodjit_frame_runtime_op_exit/op", "methodjit_vector_runtime_success/op", "methodjit_vector_runtime_errors/op", "methodjit_vector_runtime_direct_helper/op", "methodjit_vector_runtime_native_exit/op", "methodjit_vector_runtime_op_exit/op")
	if len(routeRows) == 0 {
		return nil
	}
	registryRows := qReportRowsWithAny(qReportRowsMap(routeRows), "runtime_primitive_hits/op", "runtime_primitive_errors/op", "frame_runtime_primitive_hits/op", "frame_runtime_primitive_errors/op", "vector_runtime_primitive_hits/op", "vector_runtime_primitive_errors/op")
	frameVectorRows := qReportRowsWithAny(qReportRowsMap(routeRows), "methodjit_frame_runtime_success/op", "methodjit_frame_runtime_errors/op", "methodjit_frame_runtime_direct_helper/op", "methodjit_frame_runtime_native_exit/op", "methodjit_frame_runtime_op_exit/op", "methodjit_vector_runtime_success/op", "methodjit_vector_runtime_errors/op", "methodjit_vector_runtime_direct_helper/op", "methodjit_vector_runtime_native_exit/op", "methodjit_vector_runtime_op_exit/op")
	hits := qReportSumMetric(routeRows, "runtime_primitive_hits/op") + qReportSumMetric(routeRows, "frame_runtime_primitive_hits/op") + qReportSumMetric(routeRows, "vector_runtime_primitive_hits/op") + qReportSumMetric(routeRows, "methodjit_frame_runtime_success/op") + qReportSumMetric(routeRows, "methodjit_vector_runtime_success/op")
	errorsOp := qReportSumMetric(routeRows, "runtime_primitive_errors/op") + qReportSumMetric(routeRows, "frame_runtime_primitive_errors/op") + qReportSumMetric(routeRows, "vector_runtime_primitive_errors/op") + qReportSumMetric(routeRows, "methodjit_frame_runtime_errors/op") + qReportSumMetric(routeRows, "methodjit_vector_runtime_errors/op")
	return []map[string]any{{"scope": "runtime_primitive_registry_and_frame_vector_routes", "benchmark_count": len(routeRows), "registry_benchmark_count": len(registryRows), "methodjit_frame_vector_benchmark_count": len(frameVectorRows), "hits_op": hits, "errors_op": errorsOp, "direct_helper_op": qReportSumMetric(routeRows, "methodjit_frame_runtime_direct_helper/op") + qReportSumMetric(routeRows, "methodjit_vector_runtime_direct_helper/op"), "native_exit_op": qReportSumMetric(routeRows, "methodjit_frame_runtime_native_exit/op") + qReportSumMetric(routeRows, "methodjit_vector_runtime_native_exit/op"), "op_exit_op": qReportSumMetric(routeRows, "methodjit_frame_runtime_op_exit/op") + qReportSumMetric(routeRows, "methodjit_vector_runtime_op_exit/op"), "hit_pct": qReportPercent(hits, hits+errorsOp), "note": "VM primitive registry plus MethodJIT frame/vector typed-runtime route counters; presence is a contract that backend route stats did not silently disappear"}}
}

func qReportRowsMap(rows []qReportBenchRow) map[string]qReportBenchRow {
	out := map[string]qReportBenchRow{}
	for _, row := range rows {
		out[row.Name] = row
	}
	return out
}

func qReportJITRouteSummary(rows map[string]qReportBenchRow) []map[string]any {
	routes := []struct {
		name string
		key  string
	}{
		{"direct_return", "jit_typed_direct_return/op"},
		{"native_exit", "jit_typed_native_exit/op"},
		{"op_exit", "jit_typed_op_exit/op"},
		{"success", "jit_typed_kernel_success/op"},
		{"error", "jit_typed_kernel_errors/op"},
	}
	totals := map[string]float64{}
	counts := map[string]int{}
	for _, row := range rows {
		for _, route := range routes {
			if value, ok := row.Metrics[route.key]; ok {
				totals[route.name] += value
				counts[route.name]++
			}
		}
	}
	routeTotal := totals["direct_return"] + totals["native_exit"] + totals["op_exit"]
	out := make([]map[string]any, 0, len(routes))
	for _, route := range routes {
		share := 0.0
		if routeTotal > 0 && (route.name == "direct_return" || route.name == "native_exit" || route.name == "op_exit") {
			share = 100 * totals[route.name] / routeTotal
		}
		out = append(out, map[string]any{"route": route.name, "calls_per_op": totals[route.name], "share_pct": share, "benchmark_count": counts[route.name]})
	}
	return out
}

func qReportPipelineCategoryMetrics(rows map[string]qReportBenchRow) []map[string]any {
	grouped := map[string][]qReportBenchRow{}
	for _, row := range rows {
		if !strings.HasPrefix(row.Name, "BenchmarkQSessionEvalVectorWarmExecution/") {
			continue
		}
		for key, value := range row.Metrics {
			if strings.HasPrefix(key, "q_pipeline_category_") && value > 0 {
				category := strings.TrimPrefix(key, "q_pipeline_category_")
				grouped[category] = append(grouped[category], row)
			}
		}
	}
	categories := make([]string, 0, len(grouped))
	for category := range grouped {
		categories = append(categories, category)
	}
	sort.Strings(categories)
	out := make([]map[string]any, 0, len(categories))
	for _, category := range categories {
		items := grouped[category]
		out = append(out, map[string]any{"category": category, "benchmark_count": len(items), "avg_ns_op": qReportAverageNs(items), "avg_bytes_op": qReportAverage(qReportMetricValues(items, "B/op")), "avg_allocs_op": qReportAverage(qReportMetricValues(items, "allocs/op")), "avg_typed_hit_pct": qReportAverage(qReportMetricValues(items, "typed_kernel_hit_pct")), "total_fallbacks_op": qReportSumMetric(items, "typed_kernel_fallbacks/op") + qReportSumMetric(items, "fallbacks/op"), "total_fallback_shapes": qReportSumMetric(items, "typed_pipeline_fallback_shapes")})
	}
	return out
}

func qReportFallbackShapeSummary(rows map[string]qReportBenchRow) []map[string]any {
	var out []map[string]any
	for _, name := range qReportSortedNames(rows) {
		row := rows[name]
		if qReportMetricOrZero(row, "typed_pipeline_fallback_shapes") > 0 || qReportMetricOrZero(row, "typed_kernel_fallbacks/op") > 0 || qReportMetricOrZero(row, "fallbacks/op") > 0 {
			out = append(out, map[string]any{"benchmark": name, "ns_op": row.NsOp, "bytes_op": qReportMetricValue(row, "B/op"), "allocs_op": qReportMetricValue(row, "allocs/op"), "fallbacks_op": qReportMetricValue(row, "fallbacks/op"), "typed_kernel_fallbacks_op": qReportMetricValue(row, "typed_kernel_fallbacks/op"), "typed_pipeline_fallback_shapes": qReportMetricValue(row, "typed_pipeline_fallback_shapes")})
		}
	}
	return out
}

func qReportQSQLCoverage(rows map[string]qReportBenchRow) map[string]any {
	leia, native, data := 0, 0, 0
	for name := range rows {
		switch {
		case strings.HasPrefix(name, "BenchmarkQSQLNativeGo"):
			native++
		case strings.HasPrefix(name, "BenchmarkQSQLDataRuntime"):
			data++
		case strings.HasPrefix(name, "BenchmarkQSQL"):
			leia++
		}
	}
	expected := []string{
		"BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject",
		"BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject",
		"BenchmarkQSQLBindFastArg2WarmCacheSelectWhereProject",
		"BenchmarkQSQLBindRunSQLWarmCacheGroupByAggregate",
		"BenchmarkQSQLBindRunSQLWarmCacheJoin",
		"BenchmarkQSQLBindRunSQLColdCacheJoin",
	}
	var missing []string
	for _, name := range expected {
		if _, ok := rows[name]; !ok {
			missing = append(missing, name)
		}
	}
	return map[string]any{"leia_case_count": leia, "native_go_case_count": native, "data_runtime_case_count": data, "expected_case_count": len(expected), "matched_expected_count": len(expected) - len(missing), "missing_expected": missing}
}

func qReportQEvalComputeCoverage(rows map[string]qReportBenchRow) map[string]any {
	session := qReportCases(rows, "BenchmarkQSessionEvalVectorWarmExecution")
	goCases := qReportCases(rows, "BenchmarkQEvalVectorGoBaseline")
	warm := qReportCases(rows, "BenchmarkQEvalVectorResultCacheWarm")
	cold := qReportCases(rows, "BenchmarkQEvalVectorCold")
	sessionSet := qReportSet(session)
	goSet := qReportSet(goCases)
	warmSet := qReportSet(warm)
	coldSet := qReportSet(cold)
	return map[string]any{
		"session_case_count":              len(session),
		"go_baseline_case_count":          len(goCases),
		"trusted_go_baseline_count":       qReportTrustedGoCount(rows),
		"untrusted_go_baseline_count":     len(goCases) - qReportTrustedGoCount(rows),
		"result_cache_warm_case_count":    len(warm),
		"cold_case_count":                 len(cold),
		"matched_go_baseline_count":       qReportIntersectionCount(sessionSet, goSet),
		"matched_result_cache_warm_count": qReportIntersectionCount(sessionSet, warmSet),
		"matched_cold_count":              qReportIntersectionCount(sessionSet, coldSet),
		"missing_go_baseline":             qReportMissing(session, goSet),
		"missing_result_cache_warm":       qReportMissing(session, warmSet),
		"missing_cold":                    qReportMissing(session, coldSet),
		"orphan_go_baseline":              qReportMissing(goCases, sessionSet),
		"untrusted_go_baselines":          qReportUntrustedGoBaselines(rows),
	}
}

func qReportQEvalFamilyCoverage(rows map[string]qReportBenchRow) []map[string]any {
	defs := []struct {
		family string
		note   string
		match  func(string, qReportBenchRow) bool
	}{
		{"ordinary_list_adverb", "session rows carrying q_pipeline_category_ordinary_list_adverb", func(caseName string, row qReportBenchRow) bool {
			return qReportMetricOrZero(row, "q_pipeline_category_ordinary_list_adverb") > 0
		}},
		{"type_matrix", "TypeMatrix* benchmark cases across typed/null/promotion matrix rows", func(caseName string, row qReportBenchRow) bool { return strings.HasPrefix(caseName, "TypeMatrix") }},
		{"complex_combo", "Combo* benchmark cases covering depth, mixed-type, nested-adverb, dict/table, and apply/index combinations", func(caseName string, row qReportBenchRow) bool { return strings.HasPrefix(caseName, "Combo") }},
	}
	var out []map[string]any
	goSet := qReportSet(qReportCases(rows, "BenchmarkQEvalVectorGoBaseline"))
	jitSet := qReportSet(qReportCases(rows, "BenchmarkQEvalJITScriptWarm"))
	for _, def := range defs {
		var session []string
		for _, caseName := range qReportCases(rows, "BenchmarkQSessionEvalVectorWarmExecution") {
			row := rows["BenchmarkQSessionEvalVectorWarmExecution/"+caseName]
			if def.match(caseName, row) {
				session = append(session, caseName)
			}
		}
		out = append(out, map[string]any{"family": def.family, "session_case_count": len(session), "go_baseline_case_count": len(goSet), "jit_case_count": len(jitSet), "matched_go_baseline_count": qReportIntersectionCount(qReportSet(session), goSet), "matched_jit_case_count": qReportIntersectionCount(qReportSet(session), jitSet), "missing_go_baseline": qReportMissing(session, goSet), "missing_jit_case": qReportMissing(session, jitSet), "note": def.note})
	}
	return out
}

func qReportQEvalCaseDiagnostics(rows map[string]qReportBenchRow) []map[string]any {
	cases := qReportUnionCases(rows, "BenchmarkQSessionEvalVectorWarmExecution", "BenchmarkQEvalVectorGoBaseline", "BenchmarkQEvalVectorResultCacheWarm", "BenchmarkQEvalVectorCold", "BenchmarkQEvalJITScriptWarm", "BenchmarkQEvalVMScriptWarm")
	out := make([]map[string]any, 0, len(cases))
	for _, caseName := range cases {
		session := rows["BenchmarkQSessionEvalVectorWarmExecution/"+caseName]
		goRow := rows["BenchmarkQEvalVectorGoBaseline/"+caseName]
		jit := rows["BenchmarkQEvalJITScriptWarm/"+caseName]
		trusted := goRow.Name != "" && goRow.NsOp >= qReportMinTrustedGoBaselineNS
		pressure := "healthy_or_ratio_only"
		switch {
		case goRow.Name == "":
			pressure = "missing_go_baseline"
		case !trusted:
			pressure = "untrusted_go_baseline"
		case session.Name == "":
			pressure = "missing_session_warm"
		case qReportMetricOrZero(session, "typed_kernel_errors/op") > 0:
			pressure = "typed_errors"
		case qReportMetricOrZero(session, "typed_kernel_fallbacks/op") > 0 || qReportMetricOrZero(session, "typed_pipeline_fallback_shapes") > 0:
			pressure = "typed_fallback"
		case qReportMetricOrZero(jit, "q_session_eval_errors/op") > 0 || qReportMetricOrZero(jit, "jit_typed_kernel_errors/op") > 0:
			pressure = "jit_backend_errors"
		case qReportMetricOrZero(jit, "q_session_shell_fallback/op") > 0:
			pressure = "jit_slow_route"
		case qReportMetricOrZero(session, "allocs/op") > 64:
			pressure = "alloc_pressure"
		}
		out = append(out, map[string]any{"case": caseName, "primary_pressure": pressure, "go_baseline_ns_op": qReportNilRowNs(goRow), "trusted_go_baseline": trusted, "session_ns_op": qReportNilRowNs(session), "session_go_ratio": qReportSafeRatio(qReportNilRowNs(session), qReportNilRowNs(goRow), trusted), "jit_warm_ns_op": qReportNilRowNs(jit), "jit_go_ratio": qReportSafeRatio(qReportNilRowNs(jit), qReportNilRowNs(goRow), trusted), "typed_hit_pct": qReportMetricValue(session, "typed_kernel_hit_pct"), "typed_fallbacks_op": qReportMetricValue(session, "typed_kernel_fallbacks/op"), "typed_pipeline_fallback_shapes": qReportMetricValue(session, "typed_pipeline_fallback_shapes"), "q_session_planned_op_exit_op": qReportMetricValue(jit, "q_session_planned_op_exit/op"), "q_session_shell_fallback_op": qReportMetricValue(jit, "q_session_shell_fallback/op"), "q_session_eval_errors_op": qReportMetricValue(jit, "q_session_eval_errors/op")})
	}
	return out
}

func qReportRealDataRows(rows map[string]qReportBenchRow) []map[string]any {
	cases := qReportUnionCases(rows, "BenchmarkQEvalRealDataWarm", "BenchmarkQEvalRealDataGoBaseline")
	out := make([]map[string]any, 0, len(cases))
	for _, caseName := range cases {
		warm := rows["BenchmarkQEvalRealDataWarm/"+caseName]
		goRow := rows["BenchmarkQEvalRealDataGoBaseline/"+caseName]
		trusted := goRow.Name != "" && goRow.NsOp >= qReportMinTrustedGoBaselineNS
		out = append(out, map[string]any{"case": caseName, "warm_ns_op": qReportNilRowNs(warm), "warm_allocs_op": qReportMetricValue(warm, "allocs/op"), "go_ns_op": qReportNilRowNs(goRow), "trusted_go_baseline": trusted, "realdata_go_ratio": qReportSafeRatio(qReportNilRowNs(warm), qReportNilRowNs(goRow), trusted), "note": "env-injected dense columns; closed forms cannot fire"})
	}
	return out
}

func qReportBuildCoverage(rows map[string]qReportBenchRow, currentVsOld []qReportCurrentVsOldRow) []map[string]string {
	qsqlNames := qReportNamesWithPrefix(rows, "BenchmarkQSQL")
	qevalNames := append(qReportNamesWithPrefix(rows, "BenchmarkQEval"), qReportNamesWithPrefix(rows, "BenchmarkQSessionEval")...)
	qevalKernel := qReportMetricPresent(rows, qevalNames, "typed_kernel_hit_pct") || qReportMetricPresent(rows, qevalNames, "kernel_hit_pct")
	qevalFallback := qReportMetricPresent(rows, qevalNames, "typed_kernel_fallbacks/op") || qReportMetricPresent(rows, qevalNames, "fallbacks/op")
	return []map[string]string{
		{"signal": "current Leia vs old Leia", "qSQL": qReportCovered(len(currentVsOld) > 0), "q.eval": qReportCovered(len(currentVsOld) > 0), "gap": qReportGap(len(currentVsOld) > 0, "provide --timing-json from leia bench compare or q_columnar_suite JSON output")},
		{"signal": "current Leia vs hand-written Go", "qSQL": qReportCovered(qReportRatio(rows, "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLNativeGoSelectWhereProject") != nil), "q.eval": qReportCovered(len(qReportCases(rows, "BenchmarkQEvalVectorGoBaseline")) > 0), "gap": ""},
		{"signal": "warm run vs cold run", "qSQL": qReportCovered(qReportRatio(rows, "BenchmarkQSQLBindRunSQLWarmCacheSelectWhereProject", "BenchmarkQSQLBindRunSQLColdCacheSelectWhereProject") != nil), "q.eval": qReportCovered(len(qReportCases(rows, "BenchmarkQEvalVectorCold")) > 0), "gap": "qSQL cold coverage currently only exists for select/filter/project"},
		{"signal": "typed kernel hit rate", "qSQL": qReportCovered(qReportMetricPresent(rows, qsqlNames, "kernel_hit_pct")), "q.eval": qReportCovered(qevalKernel), "gap": qReportGap(qevalKernel, "q.eval typed kernel execution is visible through q.cache_stats, but q.eval benchmarks do not yet emit per-op typed kernel metrics")},
		{"signal": "fallback rate", "qSQL": qReportCovered(qReportMetricPresent(rows, qsqlNames, "fallbacks/op")), "q.eval": qReportCovered(qevalFallback), "gap": qReportGap(qevalFallback, "q.eval benchmarks do not yet emit per-op fallbacks/op; q.cache_stats has execution counters only")},
		{"signal": "allocs/op", "qSQL": qReportCovered(qReportMetricPresent(rows, qsqlNames, "allocs/op")), "q.eval": qReportCovered(qReportMetricPresent(rows, qevalNames, "allocs/op")), "gap": ""},
	}
}

func qReportMarkdown(rows map[string]qReportBenchRow, commands []qReportCommandResult, currentVsOld []qReportCurrentVsOldRow, checks []qReportGateCheck, fallbackRows []qReportFallbackTopRow) string {
	var b strings.Builder
	b.WriteString("# q Performance Completeness Report\n\n## Commands\n\n")
	for _, command := range commands {
		status := "ok"
		if command.ExitCode != 0 {
			status = fmt.Sprintf("exit %d", command.ExitCode)
		}
		fmt.Fprintf(&b, "- `%s` (%s): `%s`\n", command.Label, status, strings.Join(command.Cmd, " "))
	}
	b.WriteString("\n## Coverage Matrix\n\n| Signal | qSQL | q.eval / ordinary q | Gap |\n|---|---|---|---|\n")
	for _, item := range qReportBuildCoverage(rows, currentVsOld) {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", item["signal"], item["qSQL"], item["q.eval"], item["gap"])
	}
	b.WriteString("\n## Gate Summary\n\n| Status | Signal | Benchmark | Value | Threshold | Note |\n|---|---|---|---:|---:|---|\n")
	if len(checks) == 0 {
		b.WriteString("| not-run | missing | missing | missing | run with `--check` to enforce thresholds |  |\n")
	} else {
		for _, check := range checks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", check.Status, check.Signal, check.Benchmark, qReportFormatValue(check.Value, 3), check.Threshold, check.Note)
		}
	}
	b.WriteString("\n## Ratios\n\n| Scenario | Leia benchmark | Denominator | Ratio | Note |\n|---|---|---|---:|---|\n")
	for _, item := range qReportBuildRatios(rows) {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s |\n", item.Scenario, item.Numerator, item.Denominator, qReportFormatRatio(item.Ratio), item.Note)
	}
	b.WriteString("\n## Runtime Metrics\n\n| Benchmark | ns/op | B/op | allocs/op | typed hit pct | fallbacks/op | pipeline fallback shapes |\n|---|---:|---:|---:|---:|---:|---:|\n")
	for _, name := range qReportSortedNames(rows) {
		row := rows[name]
		fmt.Fprintf(&b, "| %s | %.0f | %s | %s | %s | %s | %s |\n", name, row.NsOp, qReportFormatAny(qReportMetricValue(row, "B/op"), 0), qReportFormatAny(qReportMetricValue(row, "allocs/op"), 0), qReportFormatAny(qReportMetricValue(row, "typed_kernel_hit_pct"), 1), qReportFormatAny(qReportMetricValue(row, "typed_kernel_fallbacks/op"), 3), qReportFormatAny(qReportMetricValue(row, "typed_pipeline_fallback_shapes"), 0))
	}
	b.WriteString("\n## Runtime Observability Summary\n\n| Layer | Benchmarks | attempts/op | hits/op | fallbacks/op | errors/op | hit pct | Note |\n|---|---:|---:|---:|---:|---:|---:|---|\n")
	for _, item := range qReportRuntimeObservabilitySummary(rows) {
		fmt.Fprintf(&b, "| %s | %.0f | %s | %s | %s | %s | %s | %s |\n", qReportString(item["layer"]), qReportFloat(item["benchmark_count"]), qReportFormatAny(item["attempts_op"], 3), qReportFormatAny(item["hits_op"], 3), qReportFormatAny(item["fallbacks_op"], 3), qReportFormatAny(item["errors_op"], 3), qReportFormatAny(item["hit_pct"], 1), qReportString(item["note"]))
	}
	b.WriteString("\n## Pipeline Fallback Top-N\n\n| Category | Pipeline shape | Kernel | Reason | Outcome | Count |\n|---|---|---|---|---|---:|\n")
	if len(fallbackRows) == 0 {
		b.WriteString("| missing | missing | missing | missing | missing | 0 |\n")
	} else {
		for _, row := range fallbackRows {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %d |\n", row.Category, row.PipelineShape, row.Kernel, row.Reason, row.Outcome, row.Count)
		}
	}
	b.WriteString("\n## Required Follow-up Gaps\n\n- Pass `--timing-json` from `leia bench compare` / `q_columnar_suite.sh --json ...` when this report is used for current-vs-old decisions.\n")
	return b.String()
}

func qReportParseTimingJSON(data []byte) ([]qReportCurrentVsOldRow, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	var out []qReportCurrentVsOldRow
	for _, result := range qReportAnySlice(payload["results"]) {
		item := qReportAnyMap(result)
		group := qReportString(item["group"])
		bench := qReportString(item["benchmark"])
		name := bench
		if group != "" && bench != "" {
			name = group + "/" + bench
		} else if name == "" {
			name = group
		}
		for mode, subjectsAny := range qReportAnyMap(item["modes"]) {
			subjects := qReportAnyMap(subjectsAny)
			current := qReportAnyMap(subjects["current"])
			old := qReportAnyMap(subjects["head"])
			curS := qReportSubjectSeconds(current)
			oldS := qReportSubjectSeconds(old)
			out = append(out, qReportCurrentVsOldRow{Benchmark: name, Mode: mode, CurrentSeconds: curS, OldSeconds: oldS, Ratio: qReportSafeRatio(curS, oldS, true), Source: strings.Join(qReportNonEmpty(qReportString(current["source"]), qReportString(old["source"])), "/")})
		}
	}
	return out, nil
}

func qReportSubjectSeconds(subject map[string]any) *float64 {
	if stats := qReportAnyMap(subject["stats"]); stats != nil {
		if value := qReportFloatPtr(stats["median"]); value != nil {
			return value
		}
	}
	return qReportFloatPtr(subject["seconds"])
}

func qReportLoadJSON(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	return payload
}

func qReportApplyMilestoneCaps(policy *qReportGatePolicy, baseline map[string]any, seen map[string]bool) {
	caps := qReportAnyMap(baseline["milestone_caps"])
	for key := range qReportMilestoneKeys {
		if seen[key] {
			continue
		}
		value, ok := caps[key]
		if !ok {
			continue
		}
		switch key {
		case "max_leia_go_ratio":
			policy.MaxLeiaGoRatio = qReportFloat(value)
		case "max_leia_jit_go_ratio":
			policy.MaxLeiaJITGoRatio = qReportFloat(value)
		case "max_leia_realdata_go_ratio":
			policy.MaxLeiaRealDataGoRatio = qReportFloat(value)
		case "min_typed_hit_pct":
			policy.MinTypedHitPct = qReportFloat(value)
		case "max_typed_fallbacks_op":
			policy.MaxTypedFallbacksOp = qReportFloat(value)
		case "max_pipeline_fallback_shapes":
			policy.MaxPipelineFallbackShapes = qReportFloat(value)
		case "max_allocs_op":
			policy.MaxAllocsOp = qReportFloat(value)
		case "min_runtime_jit_backend_benchmarks":
			policy.MinRuntimeJITBackendBenchmarks = int(qReportFloat(value))
		case "min_runtime_array_bridge_benchmarks":
			policy.MinRuntimeArrayBridgeBenchmarks = int(qReportFloat(value))
		case "min_runtime_backend_route_benchmarks":
			policy.MinRuntimeBackendRouteBenchmarks = int(qReportFloat(value))
		case "min_runtime_backend_route_hits_op":
			policy.MinRuntimeBackendRouteHitsOp = qReportFloat(value)
		case "max_runtime_backend_route_errors_op":
			policy.MaxRuntimeBackendRouteErrorsOp = qReportFloat(value)
		case "min_q_eval_family_cases":
			policy.MinQEvalFamilyCases = int(qReportFloat(value))
		case "min_q_session_planned_op_exit_op":
			policy.MinQSessionPlannedOpExitOp = qReportFloat(value)
		}
	}
}

func qReportRowsWithAny(rows map[string]qReportBenchRow, keys ...string) []qReportBenchRow {
	var out []qReportBenchRow
	for _, name := range qReportSortedNames(rows) {
		row := rows[name]
		for _, key := range keys {
			if _, ok := row.Metrics[key]; ok {
				out = append(out, row)
				break
			}
		}
	}
	return out
}

func qReportSumMetric(rows []qReportBenchRow, key string) float64 {
	total := 0.0
	for _, row := range rows {
		total += qReportMetricOrZero(row, key)
	}
	return total
}

func qReportAverageMetric(rows []qReportBenchRow, key string) any {
	return qReportAverage(qReportMetricValues(rows, key))
}

func qReportMetricValues(rows []qReportBenchRow, key string) []float64 {
	var values []float64
	for _, row := range rows {
		if value, ok := row.Metrics[key]; ok {
			values = append(values, value)
		}
	}
	return values
}

func qReportAverageNs(rows []qReportBenchRow) any {
	if len(rows) == 0 {
		return nil
	}
	total := 0.0
	for _, row := range rows {
		total += row.NsOp
	}
	return total / float64(len(rows))
}

func qReportAverage(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func qReportMax(values []float64) any {
	if len(values) == 0 {
		return nil
	}
	max := values[0]
	for _, value := range values[1:] {
		if value > max {
			max = value
		}
	}
	return max
}

func qReportPercent(numerator, denominator float64) any {
	if denominator <= 0 {
		return nil
	}
	return 100 * numerator / denominator
}

func qReportSlowRoutePct(direct, native, opExit, sessionPlanned, sessionShell, sessionErrors float64) any {
	if sessionPlanned+sessionShell+sessionErrors > 0 {
		return 100 * (sessionShell + sessionErrors) / (sessionPlanned + sessionShell + sessionErrors)
	}
	if direct+native+opExit > 0 {
		return 100 * (native + opExit) / (direct + native + opExit)
	}
	return nil
}

func qReportNilIfZero(value float64) any {
	if value == 0 {
		return nil
	}
	return value
}

func qReportGateFailed(checks []qReportGateCheck) bool {
	return qReportFailureCount(checks) > 0
}

func qReportFailureCount(checks []qReportGateCheck) int {
	count := 0
	for _, check := range checks {
		if check.Status == "fail" {
			count++
		}
	}
	return count
}

func qReportPassFail(pass bool) string {
	if pass {
		return "pass"
	}
	return "fail"
}

func qReportNamesWithPrefix(rows map[string]qReportBenchRow, prefix string) []string {
	var names []string
	for name := range rows {
		if strings.HasPrefix(name, prefix) {
			names = append(names, name)
		}
	}
	return names
}

func qReportMetricPresent(rows map[string]qReportBenchRow, names []string, metric string) bool {
	for _, name := range names {
		if _, ok := rows[name].Metrics[metric]; ok {
			return true
		}
	}
	return false
}

func qReportCovered(ok bool) string {
	if ok {
		return "covered"
	}
	return "missing"
}

func qReportGap(ok bool, gap string) string {
	if ok {
		return ""
	}
	return gap
}

func qReportSet(items []string) map[string]bool {
	out := map[string]bool{}
	for _, item := range items {
		out[item] = true
	}
	return out
}

func qReportIntersectionCount(left, right map[string]bool) int {
	count := 0
	for item := range left {
		if right[item] {
			count++
		}
	}
	return count
}

func qReportMissing(items []string, present map[string]bool) []string {
	var missing []string
	for _, item := range items {
		if !present[item] {
			missing = append(missing, item)
		}
	}
	return missing
}

func qReportTrustedGoCount(rows map[string]qReportBenchRow) int {
	count := 0
	for _, caseName := range qReportCases(rows, "BenchmarkQEvalVectorGoBaseline") {
		if rows["BenchmarkQEvalVectorGoBaseline/"+caseName].NsOp >= qReportMinTrustedGoBaselineNS {
			count++
		}
	}
	return count
}

func qReportUntrustedGoBaselines(rows map[string]qReportBenchRow) []string {
	var out []string
	for _, caseName := range qReportCases(rows, "BenchmarkQEvalVectorGoBaseline") {
		name := "BenchmarkQEvalVectorGoBaseline/" + caseName
		if rows[name].NsOp < qReportMinTrustedGoBaselineNS {
			out = append(out, caseName)
		}
	}
	return out
}

func qReportUnionCases(rows map[string]qReportBenchRow, prefixes ...string) []string {
	seen := map[string]bool{}
	for _, prefix := range prefixes {
		for _, caseName := range qReportCases(rows, prefix) {
			seen[caseName] = true
		}
	}
	out := make([]string, 0, len(seen))
	for caseName := range seen {
		out = append(out, caseName)
	}
	sort.Strings(out)
	return out
}

func qReportNilRowNs(row qReportBenchRow) any {
	if row.Name == "" {
		return nil
	}
	return row.NsOp
}

func qReportSafeRatio(numeratorAny, denominatorAny any, trusted bool) *float64 {
	if !trusted {
		return nil
	}
	numerator := qReportFloatPtr(numeratorAny)
	denominator := qReportFloatPtr(denominatorAny)
	if numerator == nil || denominator == nil || *denominator == 0 {
		return nil
	}
	value := *numerator / *denominator
	return &value
}

func qReportAnyMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func qReportAnySlice(value any) []any {
	if typed, ok := value.([]any); ok {
		return typed
	}
	return nil
}

func qReportFloatPtr(value any) *float64 {
	switch typed := value.(type) {
	case nil:
		return nil
	case float64:
		return &typed
	case *float64:
		return typed
	case float32:
		value := float64(typed)
		return &value
	case int:
		value := float64(typed)
		return &value
	case int64:
		value := float64(typed)
		return &value
	case json.Number:
		value, err := typed.Float64()
		if err == nil {
			return &value
		}
	case string:
		value, err := strconv.ParseFloat(typed, 64)
		if err == nil {
			return &value
		}
	}
	return nil
}

func qReportFloat(value any) float64 {
	if ptr := qReportFloatPtr(value); ptr != nil {
		return *ptr
	}
	return 0
}

func qReportString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func qReportDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func qReportNonEmpty(values ...string) []string {
	var out []string
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func qReportFormatValue(value *float64, digits int) string {
	if value == nil {
		return "missing"
	}
	return fmt.Sprintf("%.*f", digits, *value)
}

func qReportFormatRatio(value *float64) string {
	if value == nil {
		return "missing"
	}
	return fmt.Sprintf("%.3fx", *value)
}

func qReportFormatAny(value any, digits int) string {
	if ptr := qReportFloatPtr(value); ptr != nil {
		if math.IsNaN(*ptr) || math.IsInf(*ptr, 0) {
			return "missing"
		}
		return fmt.Sprintf("%.*f", digits, *ptr)
	}
	return "missing"
}
