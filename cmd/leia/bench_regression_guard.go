package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var benchRegressionDefaults = []string{
	"recursion/fib", "recursion/fib_recursive", "control/sieve", "numeric/mandelbrot",
	"recursion/ackermann", "numeric/matmul", "numeric/spectral_norm", "numeric/nbody",
	"numeric/fannkuch", "table/sort", "numeric/sum_primes", "recursion/mutual_recursion",
	"calls/method_dispatch", "calls/closure_bench", "string/string_bench", "recursion/binary_trees",
	"table/table_field_access", "table/table_array_access", "calls/coroutine_bench",
	"recursion/fibonacci_iterative", "numeric/math_intensive", "calls/object_creation",
}

var (
	benchRegressionT2AttemptedRE = regexp.MustCompile(`(?m)^\s*Tier 2 attempted:\s*([0-9]+)\b`)
	benchRegressionT2EnteredRE   = regexp.MustCompile(`(?m)^\s*Tier 2 entered:\s*([0-9]+)\s+functions\b`)
	benchRegressionT2FailedRE    = regexp.MustCompile(`(?m)^\s*Tier 2 failed:\s*([0-9]+)\s+functions\b`)
	benchRegressionExitTotalRE   = regexp.MustCompile(`(?m)^\s*total exits:\s*([0-9]+)\b`)
)

type benchRegressionSample struct {
	Status      string   `json:"status"`
	Seconds     *float64 `json:"seconds,omitempty"`
	ExitCode    *int     `json:"exit_code,omitempty"`
	OutputTail  string   `json:"output_tail,omitempty"`
	T2Attempted int      `json:"t2_attempted"`
	T2Entered   int      `json:"t2_entered"`
	T2Failed    int      `json:"t2_failed"`
	ExitTotal   int      `json:"exit_total"`
}

type benchRegressionMode struct {
	Status      string                  `json:"status"`
	Seconds     *float64                `json:"seconds,omitempty"`
	Samples     []benchRegressionSample `json:"samples,omitempty"`
	T2Attempted int                     `json:"t2_attempted"`
	T2Entered   int                     `json:"t2_entered"`
	T2Failed    int                     `json:"t2_failed"`
	ExitTotal   int                     `json:"exit_total"`
}

type benchRegressionResult struct {
	Benchmark       string               `json:"benchmark"`
	VM              *benchRegressionMode `json:"vm,omitempty"`
	Default         *benchRegressionMode `json:"default,omitempty"`
	NoFilter        *benchRegressionMode `json:"no_filter,omitempty"`
	LuaJIT          *benchRegressionMode `json:"luajit,omitempty"`
	BaselineSeconds *float64             `json:"baseline_seconds,omitempty"`
	RegressionPct   *float64             `json:"regression_pct,omitempty"`
	Regression      bool                 `json:"regression"`
}

func runBenchRegressionGuardCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench regression-guard", flag.ContinueOnError)
	fs.SetOutput(errw)
	benches := benchProfileStringList{}
	runs := fs.Int("runs", 3, "samples per benchmark/mode")
	fs.IntVar(runs, "count", 3, "alias for --runs")
	timeout := fs.Int("timeout", 60, "timeout per sample in seconds")
	threshold := fs.Float64("threshold", 10.0, "regression threshold percent")
	baselinePath := fs.String("baseline", filepath.Join("benchmarks", "data", "baseline.json"), "baseline JSON")
	jsonPath := fs.String("json", "", "write machine-readable results")
	csvPath := fs.String("csv", "", "write flat CSV summary")
	markdownPath := fs.String("markdown", "", "write Markdown summary")
	noLuaJIT := fs.Bool("no-luajit", false, "skip LuaJIT even when installed")
	keepBin := fs.Bool("keep-bin", false, "keep temporary Leia binary")
	fs.Var(&benches, "bench", "benchmark to run; repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *runs <= 0 {
		fmt.Fprintln(errw, "leia bench regression-guard: --runs must be > 0")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(errw, "leia bench regression-guard: --timeout must be > 0")
		return 2
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
		return 1
	}
	selected := []string(benches)
	if len(selected) == 0 {
		selected = append([]string(nil), benchRegressionDefaults...)
	}
	baseline := benchRegressionLoadBaseline(absRepoPath(root, *baselinePath))
	tempdir, err := os.MkdirTemp("", "leia_bench_guard_")
	if err != nil {
		fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
		return 1
	}
	leia := filepath.Join(tempdir, "leia")
	if !*keepBin {
		defer os.RemoveAll(tempdir)
	}
	if err := benchProfileBuildCLI(root, leia); err != nil {
		fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
		return 1
	}
	luajit := ""
	if !*noLuaJIT {
		if found, ok := lookupExecutable("luajit"); ok {
			luajit = found
		}
	}
	started := time.Now()
	results := make([]benchRegressionResult, 0, len(selected))
	for _, bench := range selected {
		id, leiaScript, _ := benchRegressionResolve(root, bench)
		row := benchRegressionResult{Benchmark: id}
		row.VM = benchRegressionRunMode("vm", root, leia, luajit, leiaScript, bench, *runs, *timeout)
		row.Default = benchRegressionRunMode("default", root, leia, luajit, leiaScript, bench, *runs, *timeout)
		row.NoFilter = benchRegressionRunMode("no_filter", root, leia, luajit, leiaScript, bench, *runs, *timeout)
		row.LuaJIT = benchRegressionRunMode("luajit", root, leia, luajit, leiaScript, bench, *runs, *timeout)
		if base, ok := baseline[id]; ok {
			row.BaselineSeconds = &base
			if row.Default != nil && row.Default.Seconds != nil && base > 0 {
				pct := (*row.Default.Seconds/base - 1) * 100
				row.RegressionPct = &pct
				row.Regression = pct > *threshold
			}
		}
		results = append(results, row)
	}
	benchRegressionPrintTable(outw, results, *threshold)
	payload := map[string]any{
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"duration_seconds": time.Since(started).Seconds(),
		"commit":           benchDebugGitCommit(root),
		"platform": map[string]any{
			"luajit": luajit,
		},
		"runs":            *runs,
		"timeout_seconds": *timeout,
		"threshold_pct":   *threshold,
		"baseline":        *baselinePath,
		"results":         results,
	}
	if *jsonPath != "" {
		if err := benchProfileWriteJSONFile(absRepoPath(root, *jsonPath), payload); err != nil {
			fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
			return 1
		}
		fmt.Fprintf(outw, "Wrote JSON: %s\n", absRepoPath(root, *jsonPath))
	}
	if *csvPath != "" {
		if err := benchRegressionWriteCSV(absRepoPath(root, *csvPath), results); err != nil {
			fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
			return 1
		}
		fmt.Fprintf(outw, "Wrote CSV: %s\n", absRepoPath(root, *csvPath))
	}
	if *markdownPath != "" {
		if err := os.MkdirAll(filepath.Dir(absRepoPath(root, *markdownPath)), 0o755); err != nil {
			fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
			return 1
		}
		if err := os.WriteFile(absRepoPath(root, *markdownPath), []byte(benchRegressionMarkdown(results, *threshold)), 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench regression-guard: %v\n", err)
			return 1
		}
		fmt.Fprintf(outw, "Wrote Markdown: %s\n", absRepoPath(root, *markdownPath))
	}
	if *keepBin {
		fmt.Fprintf(outw, "Kept leia binary: %s\n", leia)
	}
	for _, row := range results {
		if row.Regression {
			return 1
		}
	}
	return 0
}

func benchRegressionResolve(root, bench string) (string, string, string) {
	id, leiaPath, ok := benchProfileResolveBenchmark(root, bench)
	if !ok {
		return bench, filepath.Join(root, "benchmarks", "__missing__", bench+".leia"), ""
	}
	group, name, _ := strings.Cut(id, "/")
	lua := filepath.Join(root, "benchmarks", "lua_ref", group, name+".lua")
	if !fileExists(lua) {
		lua = ""
	}
	return id, leiaPath, lua
}

func benchRegressionRunMode(mode, root, leia, luajit, leiaScript, bench string, runs, timeout int) *benchRegressionMode {
	_, _, luaScript := benchRegressionResolve(root, bench)
	var cmd []string
	env := os.Environ()
	switch mode {
	case "vm":
		if !fileExists(leiaScript) {
			return &benchRegressionMode{Status: "missing"}
		}
		cmd = []string{leia, "-vm", leiaScript}
	case "default":
		if !fileExists(leiaScript) {
			return &benchRegressionMode{Status: "missing"}
		}
		cmd = []string{leia, "-jit", "-jit-stats", "-exit-stats", leiaScript}
	case "no_filter":
		if !fileExists(leiaScript) {
			return &benchRegressionMode{Status: "missing"}
		}
		env = append(env, "LEIA_TIER2_NO_FILTER=1")
		cmd = []string{leia, "-jit", "-jit-stats", "-exit-stats", leiaScript}
	case "luajit":
		if luajit == "" {
			return &benchRegressionMode{Status: "skipped"}
		}
		if luaScript == "" {
			return &benchRegressionMode{Status: "missing"}
		}
		cmd = []string{luajit, luaScript}
	default:
		return &benchRegressionMode{Status: "missing"}
	}
	samples := make([]benchRegressionSample, 0, runs)
	for i := 0; i < runs; i++ {
		samples = append(samples, benchRegressionRunCommand(cmd, env, time.Duration(timeout)*time.Second))
	}
	return benchRegressionSummarizeSamples(samples)
}

func benchRegressionRunCommand(cmdArgs []string, env []string, timeout time.Duration) benchRegressionSample {
	cmd := benchExecCommand(cmdArgs[0], cmdArgs[1:]...)
	cmd.Env = env
	var output strings.Builder
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := benchProfileRunCommand(cmd, timeout)
	text := output.String()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return benchRegressionParseSample(text+"\nTIMEOUT after "+fmt.Sprintf("%.0fs", timeout.Seconds()), "timeout", nil)
		}
		exitCode := 1
		type exitCoder interface{ ExitCode() int }
		var ec exitCoder
		if errors.As(err, &ec) {
			exitCode = ec.ExitCode()
		}
		return benchRegressionParseSample(text, "error", &exitCode)
	}
	sample := benchRegressionParseSample(text, "ok", intPtr(0))
	if sample.Seconds == nil {
		sample.Status = "no_time"
	}
	return sample
}

func benchRegressionParseSample(output, status string, exitCode *int) benchRegressionSample {
	seconds := benchProfileParseTime(output)
	if status != "ok" {
		seconds = nil
	}
	return benchRegressionSample{
		Status: status, Seconds: seconds, ExitCode: exitCode, OutputTail: lastNonEmptyLines(output, 8),
		T2Attempted: benchRegressionParseCounter(benchRegressionT2AttemptedRE, output),
		T2Entered:   benchRegressionParseCounter(benchRegressionT2EnteredRE, output),
		T2Failed:    benchRegressionParseCounter(benchRegressionT2FailedRE, output),
		ExitTotal:   benchRegressionParseCounter(benchRegressionExitTotalRE, output),
	}
}

func benchRegressionSummarizeSamples(samples []benchRegressionSample) *benchRegressionMode {
	okSeconds := make([]float64, 0)
	okSamples := make([]benchRegressionSample, 0)
	for _, sample := range samples {
		if sample.Status == "ok" && sample.Seconds != nil {
			okSeconds = append(okSeconds, *sample.Seconds)
			okSamples = append(okSamples, sample)
		}
	}
	status := "ok"
	if len(okSamples) == 0 {
		status = "missing"
		if len(samples) > 0 {
			status = samples[len(samples)-1].Status
		}
	} else if len(okSamples) != len(samples) {
		status = "partial"
	}
	var seconds *float64
	if len(okSeconds) > 0 {
		sort.Float64s(okSeconds)
		median := okSeconds[len(okSeconds)/2]
		if len(okSeconds)%2 == 0 {
			median = (okSeconds[len(okSeconds)/2-1] + okSeconds[len(okSeconds)/2]) / 2
		}
		seconds = &median
	}
	source := benchRegressionSample{Status: "missing"}
	if len(okSamples) > 0 {
		source = okSamples[len(okSamples)/2]
	} else if len(samples) > 0 {
		source = samples[len(samples)-1]
	}
	return &benchRegressionMode{Status: status, Seconds: seconds, Samples: samples, T2Attempted: source.T2Attempted, T2Entered: source.T2Entered, T2Failed: source.T2Failed, ExitTotal: source.ExitTotal}
}

func benchRegressionLoadBaseline(path string) map[string]float64 {
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]float64{}
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return map[string]float64{}
	}
	out := map[string]float64{}
	switch results := payload["results"].(type) {
	case map[string]any:
		for name, raw := range results {
			row := benchProfileAnyMap(raw)
			if sec, ok := benchRegressionParseSeconds(row["jit"]); ok {
				out[name] = sec
			}
		}
	case []any:
		for _, raw := range results {
			row := benchProfileAnyMap(raw)
			name := benchDebugStringDefault(row["benchmark"], "")
			defaultRow := benchProfileAnyMap(row["default"])
			if name != "" {
				if sec, ok := benchRegressionParseSeconds(defaultRow["seconds"]); ok {
					out[name] = sec
				}
			}
		}
	}
	return out
}

func benchRegressionParseSeconds(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	switch text {
	case "", "ERROR", "FAILED", "HANG", "N/A", "SKIP", "TIMEOUT", "n/a":
		return 0, false
	}
	if strings.HasPrefix(text, "Time:") {
		text = strings.TrimSpace(strings.TrimPrefix(text, "Time:"))
	}
	mult := 1.0
	if strings.HasSuffix(text, "us") {
		mult, text = 1e-6, strings.TrimSuffix(text, "us")
	} else if strings.HasSuffix(text, "ms") {
		mult, text = 1e-3, strings.TrimSuffix(text, "ms")
	} else if strings.HasSuffix(text, "s") {
		text = strings.TrimSuffix(text, "s")
	}
	n, err := strconv.ParseFloat(text, 64)
	return n * mult, err == nil
}

func benchRegressionPrintTable(outw io.Writer, results []benchRegressionResult, threshold float64) {
	header := fmt.Sprintf("%-22s %9s %9s %9s %9s %8s %8s %9s %11s %7s %9s", "Benchmark", "VM", "Default", "NoFilter", "LuaJIT", "JIT/VM", "JIT/LJ", "Baseline", "T2 a/e/f", "Exits", "Regress")
	fmt.Fprintln(outw, header)
	fmt.Fprintln(outw, strings.Repeat("-", len(header)))
	regressions := 0
	for _, row := range results {
		vm, def, noFilter, luajit := modeOrMissing(row.VM), modeOrMissing(row.Default), modeOrMissing(row.NoFilter), modeOrMissing(row.LuaJIT)
		marker := "-"
		if row.RegressionPct != nil {
			marker = fmt.Sprintf("%+.1f%%", *row.RegressionPct)
		}
		if row.Regression {
			marker = "REG " + marker
			regressions++
		}
		fmt.Fprintf(outw, "%-22s %9s %9s %9s %9s %8s %8s %9s %11s %7d %9s\n",
			row.Benchmark, benchRegressionFmtSeconds(vm.Seconds, vm.Status), benchRegressionFmtSeconds(def.Seconds, def.Status),
			benchRegressionFmtSeconds(noFilter.Seconds, noFilter.Status), benchRegressionFmtSeconds(luajit.Seconds, luajit.Status),
			benchRegressionRatioText(vm.Seconds, def.Seconds), benchRegressionRatioText(def.Seconds, luajit.Seconds),
			benchRegressionFmtSeconds(row.BaselineSeconds, ""), fmt.Sprintf("%d/%d/%d", def.T2Attempted, def.T2Entered, def.T2Failed), def.ExitTotal, marker)
	}
	fmt.Fprintln(outw)
	fmt.Fprintf(outw, "Regression threshold: >%.1f%% slower than baseline default JIT\n", threshold)
	fmt.Fprintf(outw, "Regressions: %d\n", regressions)
}

func benchRegressionMarkdown(results []benchRegressionResult, threshold float64) string {
	var b strings.Builder
	b.WriteString("| Benchmark | VM | Default JIT | NoFilter | LuaJIT | JIT/VM | JIT/LJ | Baseline | Regress | T2 a/e/f | Exits |\n")
	b.WriteString("|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	regressions := 0
	for _, row := range results {
		vm, def, noFilter, luajit := modeOrMissing(row.VM), modeOrMissing(row.Default), modeOrMissing(row.NoFilter), modeOrMissing(row.LuaJIT)
		marker := benchRegressionFmtPct(row.RegressionPct)
		if row.Regression {
			marker = "REG " + marker
			regressions++
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %d/%d/%d | %d |\n",
			row.Benchmark, benchRegressionFmtSeconds(vm.Seconds, vm.Status), benchRegressionFmtSeconds(def.Seconds, def.Status),
			benchRegressionFmtSeconds(noFilter.Seconds, noFilter.Status), benchRegressionFmtSeconds(luajit.Seconds, luajit.Status),
			benchRegressionRatioText(vm.Seconds, def.Seconds), benchRegressionRatioText(def.Seconds, luajit.Seconds),
			benchRegressionFmtSeconds(row.BaselineSeconds, ""), marker, def.T2Attempted, def.T2Entered, def.T2Failed, def.ExitTotal)
	}
	fmt.Fprintf(&b, "\nRegression threshold: >%.1f%% slower than baseline default JIT.\nRegressions: %d\n", threshold, regressions)
	return b.String()
}

func benchRegressionWriteCSV(path string, results []benchRegressionResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	header := []string{"benchmark", "vm_seconds", "default_seconds", "no_filter_seconds", "luajit_seconds", "jit_vm_speedup", "jit_luajit_ratio", "baseline_seconds", "regression_pct", "regression", "default_status", "t2_attempted", "t2_entered", "t2_failed", "exit_total"}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, row := range results {
		vm, def, noFilter, luajit := modeOrMissing(row.VM), modeOrMissing(row.Default), modeOrMissing(row.NoFilter), modeOrMissing(row.LuaJIT)
		record := []string{row.Benchmark, floatPtrCSV(vm.Seconds), floatPtrCSV(def.Seconds), floatPtrCSV(noFilter.Seconds), floatPtrCSV(luajit.Seconds), floatPtrCSV(benchRegressionRatio(vm.Seconds, def.Seconds)), floatPtrCSV(benchRegressionRatio(def.Seconds, luajit.Seconds)), floatPtrCSV(row.BaselineSeconds), floatPtrCSV(row.RegressionPct), fmt.Sprint(row.Regression), def.Status, fmt.Sprint(def.T2Attempted), fmt.Sprint(def.T2Entered), fmt.Sprint(def.T2Failed), fmt.Sprint(def.ExitTotal)}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func modeOrMissing(mode *benchRegressionMode) *benchRegressionMode {
	if mode == nil {
		return &benchRegressionMode{Status: "missing"}
	}
	return mode
}

func benchRegressionFmtSeconds(value *float64, status string) string {
	if value == nil {
		if status != "" {
			return status
		}
		return "-"
	}
	return fmt.Sprintf("%.3fs", *value)
}

func benchRegressionRatio(numer, denom *float64) *float64 {
	if numer == nil || denom == nil || *denom == 0 {
		return nil
	}
	value := *numer / *denom
	return &value
}

func benchRegressionRatioText(numer, denom *float64) string {
	value := benchRegressionRatio(numer, denom)
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2fx", *value)
}

func benchRegressionFmtPct(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%+.1f%%", *value)
}

func benchRegressionParseCounter(re *regexp.Regexp, output string) int {
	match := re.FindStringSubmatch(output)
	if match == nil {
		return 0
	}
	n, _ := strconv.Atoi(match[1])
	return n
}

func absRepoPath(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}

func floatPtrCSV(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func intPtr(value int) *int { return &value }
