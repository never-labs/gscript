package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/never-labs/leia/internal/tooling/benchdisc"
)

type benchDiagConfig struct {
	Benches      []string
	Groups       []string
	AllGroups    bool
	OutDir       string
	Timeout      string
	Runs         int
	Warmup       int
	Scale        []string
	ScaleProfile string
	NoTiming     bool
	PPROF        bool
	WarmDump     bool
}

func runBenchDiagnoseCommand(args []string, outw, errw io.Writer) int {
	cfg, err := parseBenchDiagnoseArgs(args, errw)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	root, err := qReportRepoRoot()
	if err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 1
	}
	specs, err := benchSelectForDiag(root, cfg.Groups, cfg.Benches, cfg.AllGroups)
	if err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 2
	}
	outDir := cfg.OutDir
	if outDir == "" {
		outDir, err = os.MkdirTemp("", "leia_diagnose_")
		if err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
			return 1
		}
	} else if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 1
	}
	tempDir, err := os.MkdirTemp("", "leia-bench-diagnose-")
	if err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tempDir)
	leiaBin := ""
	timeout, err := parseBenchGoTimeout(cfg.Timeout)
	if err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 2
	}
	if !cfg.NoTiming {
		leiaBin = filepath.Join(tempDir, "leia-current")
		if err := benchBuildLeia(root, leiaBin, "build failed in {root} with exit {exit_code}", errw); err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
			return 1
		}
	}
	rows := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		artifactDir := filepath.Join(outDir, spec.Group+"__"+spec.Name)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
			return 1
		}
		scale, err := benchGoScaleForSpec(root, spec, benchGoHarnessConfig{Scale: cfg.Scale, ScaleProfile: cfg.ScaleProfile})
		if err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %s: %v\n", spec.ID(), err)
			return 2
		}
		runSpec, err := benchGoPrepareScaledSpec(spec, filepath.Join(tempDir, "scaled", spec.Group+"__"+spec.Name), scale)
		if err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %s: %v\n", spec.ID(), err)
			return 2
		}
		summary := filepath.Join(artifactDir, "summary.raw.txt")
		raw := filepath.Join(artifactDir, "run.raw.txt")
		diag := benchRunDiagnoseSpec(runSpec, leiaBin, timeout, cfg)
		if err := os.WriteFile(raw, []byte(diag.RawOutput), 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
			return 1
		}
		if err := os.WriteFile(summary, []byte(benchDiagnoseSummaryText(spec, diag)), 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
			return 1
		}
		rows = append(rows, map[string]any{
			"benchmark":             spec.Name,
			"group":                 spec.Group,
			"script":                spec.LeiaRel(),
			"status":                diag.Status,
			"time_seconds":          diag.TimeSeconds,
			"t2_attempted":          diag.T2Attempted,
			"t2_compiled":           diag.T2Entered,
			"t2_entered":            diag.T2Entered,
			"t2_failed":             diag.T2Failed,
			"exit_total":            diag.ExitTotal,
			"top_exit":              nil,
			"work_action":           "collect-runtime-evidence",
			"work_target":           spec.ID(),
			"work_proto":            "",
			"work_priority":         0,
			"readiness":             benchDiagnoseReadiness(diag),
			"runtime_summary":       map[string]any{},
			"tier2_call_summary":    map[string]any{},
			"pprof_runs":            0,
			"pprof_script_repeat":   0,
			"pprof_samples_seconds": 0,
			"pprof_effective":       cfg.PPROF,
			"scale":                 scale,
			"artifact_dir":          artifactDir,
			"artifacts": map[string]string{
				"summary_raw_txt": summary,
				"run_raw_txt":     raw,
			},
		})
	}
	payload := map[string]any{"out_dir": outDir, "benchmarks": rows}
	jsonPath := filepath.Join(outDir, "diagnostics.json")
	mdPath := filepath.Join(outDir, "diagnostics.md")
	if err := writeJSONFile(jsonPath, payload); err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 1
	}
	if err := os.WriteFile(mdPath, []byte(benchDiagnoseMarkdown(rows)), 0o644); err != nil {
		fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
		return 1
	}
	fmt.Fprintf(outw, "Wrote diagnostics: %s\n", jsonPath)
	fmt.Fprintf(outw, "Wrote markdown:    %s\n", mdPath)
	return 0
}

func parseBenchDiagnoseArgs(args []string, errw io.Writer) (benchDiagConfig, error) {
	cfg := benchDiagConfig{Groups: []string{"numeric", "recursion", "table", "calls", "string", "app", "control"}, Timeout: "120", Runs: 5, Warmup: 1, ScaleProfile: "none"}
	fs := flag.NewFlagSet("bench diagnose", flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.Var((*benchStringList)(&cfg.Benches), "bench", "Benchmark selector; repeatable.")
	fs.Var((*benchStringList)(&cfg.Groups), "group", "Benchmark group; repeatable.")
	fs.BoolVar(&cfg.AllGroups, "all-groups", false, "Run all groups.")
	fs.StringVar(&cfg.OutDir, "out-dir", "", "Output directory.")
	fs.StringVar(&cfg.Timeout, "timeout", cfg.Timeout, "Timeout seconds.")
	fs.IntVar(&cfg.Runs, "runs", cfg.Runs, "Timing measured runs.")
	fs.IntVar(&cfg.Warmup, "warmup", cfg.Warmup, "Timing warmup runs.")
	fs.Var((*benchStringList)(&cfg.Scale), "scale", "Scale override; repeatable.")
	fs.Var((*benchStringList)(&cfg.Scale), "param", "Scale override alias.")
	fs.StringVar(&cfg.ScaleProfile, "scale-profile", cfg.ScaleProfile, "Scale profile.")
	fs.BoolVar(&cfg.NoTiming, "no-timing", false, "Skip timing compare.")
	fs.BoolVar(&cfg.PPROF, "pprof", false, "Collect pprof evidence when supported.")
	fs.BoolVar(&cfg.WarmDump, "warm-dump", false, "Collect warm addr map evidence when supported.")
	var pprofMin float64
	var pprofMax int
	fs.Float64Var(&pprofMin, "pprof-min-samples-ms", 50, "Accepted for compatibility.")
	fs.IntVar(&pprofMax, "pprof-max-runs", 8, "Accepted for compatibility.")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(errw, "leia bench diagnose: unexpected argument %q\n", fs.Arg(0))
		return cfg, flag.ErrHelp
	}
	return cfg, nil
}

type benchDiagnoseRun struct {
	Status      string
	TimeSeconds *float64
	T2Attempted int
	T2Entered   int
	T2Failed    int
	ExitTotal   int
	WallSeconds float64
	RawOutput   string
}

func benchRunDiagnoseSpec(spec benchdisc.Benchmark, leiaBin string, timeout time.Duration, cfg benchDiagConfig) benchDiagnoseRun {
	if cfg.NoTiming {
		return benchDiagnoseRun{Status: "skipped", RawOutput: "diagnose timing skipped by --no-timing\n"}
	}
	cmd, err := benchBenchmarkModeCommand("default", leiaBin, spec.Leia, "", "")
	if err != nil {
		return benchDiagnoseRun{Status: "error", RawOutput: err.Error() + "\n"}
	}
	if cmd.Unavailable != "" {
		return benchDiagnoseRun{Status: cmd.Unavailable, RawOutput: "input unavailable\n"}
	}
	for i := 0; i < cfg.Warmup; i++ {
		_ = benchRunTextCommand(cmd.Args, timeout, cmd.Env)
	}
	runs := cfg.Runs
	if runs < 1 {
		runs = 1
	}
	values := []float64{}
	last := benchTextCommandResult{Status: "missing"}
	for i := 0; i < runs; i++ {
		last = benchRunTextCommand(cmd.Args, timeout, cmd.Env)
		if seconds := benchParseTime(last.Output); seconds != nil {
			values = append(values, *seconds)
		}
		if last.Status != "ok" {
			break
		}
	}
	status := last.Status
	var seconds *float64
	if len(values) > 0 {
		stats := benchGoComputeStats(values)
		seconds = stats.Median
		if status == "ok" && len(values) != runs {
			status = "partial"
		}
	}
	return benchDiagnoseRun{
		Status:      status,
		TimeSeconds: seconds,
		T2Attempted: benchParseCounter(benchT2AttemptedRE, last.Output),
		T2Entered:   benchParseCounter(benchT2EnteredRE, last.Output),
		T2Failed:    benchParseCounter(benchT2FailedRE, last.Output),
		ExitTotal:   benchParseCounter(benchExitTotalRE, last.Output),
		WallSeconds: last.WallSeconds,
		RawOutput:   last.Output,
	}
}

func benchDiagnoseSummaryText(spec benchdisc.Benchmark, diag benchDiagnoseRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "benchmark: %s\n", spec.ID())
	fmt.Fprintf(&b, "status: %s\n", diag.Status)
	fmt.Fprintf(&b, "time_seconds: %s\n", benchDiagFormatSeconds(diag.TimeSeconds))
	fmt.Fprintf(&b, "wall_seconds: %.6f\n", diag.WallSeconds)
	fmt.Fprintf(&b, "tier2_attempted: %d\n", diag.T2Attempted)
	fmt.Fprintf(&b, "tier2_entered: %d\n", diag.T2Entered)
	fmt.Fprintf(&b, "tier2_failed: %d\n", diag.T2Failed)
	fmt.Fprintf(&b, "exit_total: %d\n", diag.ExitTotal)
	if tail := benchOutputTail(diag.RawOutput, 20); tail != "" {
		b.WriteString("\noutput_tail:\n")
		b.WriteString(tail)
		b.WriteByte('\n')
	}
	return b.String()
}

func benchDiagnoseReadiness(diag benchDiagnoseRun) string {
	if diag.Status != "ok" {
		return "needs-attention"
	}
	if diag.T2Failed > 0 || diag.ExitTotal > 0 {
		return "has-runtime-evidence"
	}
	return "baseline"
}

type benchTriageConfig struct {
	Benches          []string
	Modes            []string
	Runs             string
	Warmup           string
	Timeout          string
	MinSampleSeconds string
	MaxRepeat        string
	TimeSource       string
	Scale            []string
	ScaleProfile     string
	Diag             bool
	PPROF            bool
	MemProfile       string
	RuntimeStats     string
	SpecState        string
	NoSpecState      bool
	WarmDump         bool
	OutDir           string
}

func runBenchTriageCommand(args []string, outw, errw io.Writer) int {
	cfg, err := parseBenchTriageArgs(args, errw)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(cfg.Benches) == 0 {
		fmt.Fprintln(errw, "leia bench triage: at least one --bench is required")
		return 2
	}
	root, err := qReportRepoRoot()
	if err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	if _, err := benchSelectForDiag(root, benchdisc.DomainGroups, cfg.Benches, false); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 2
	}
	outDir := cfg.OutDir
	if outDir == "" {
		outDir = filepath.Join(os.TempDir(), "leia-triage")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	timingJSON := filepath.Join(outDir, "timing.json")
	timingMD := filepath.Join(outDir, "timing.md")
	compareArgs := benchTriageCompareArgs(cfg, timingJSON, timingMD)
	if code := runBenchGoHarness("compare", compareArgs, io.Discard, errw); code != 0 {
		fmt.Fprintf(errw, "leia bench triage: timing compare failed with code %d\n", code)
		return code
	}
	timing, err := benchTriageRowsFromCompare(timingJSON)
	if err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	payload := map[string]any{
		"timing":             timing,
		"bottlenecks":        []any{},
		"recommendations":    benchTriageRecommendations(timing),
		"artifacts":          map[string]any{"timing_json": timingJSON, "timing_md": timingMD},
		"exit_summary":       map[string]any{"total": 0, "by_code": map[string]any{}, "by_reason": map[string]any{}, "top_sites": []any{}, "statuses": map[string]any{}},
		"pprof_summary":      map[string]any{},
		"pcmap_summary":      map[string]any{},
		"memprofile_summary": map[string]any{},
		"runtime_stats":      map[string]any{},
		"speculation_state": map[string]any{
			"status": "not_collected",
		},
	}
	jsonPath := filepath.Join(outDir, "triage.json")
	mdPath := filepath.Join(outDir, "triage.md")
	if err := writeJSONFile(jsonPath, payload); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	md := benchTriageMarkdown(timing)
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	_, _ = io.WriteString(outw, md)
	return 0
}

func parseBenchTriageArgs(args []string, errw io.Writer) (benchTriageConfig, error) {
	cfg := benchTriageConfig{Modes: []string{"default"}, Runs: "5", Warmup: "1", Timeout: "120", MinSampleSeconds: "0.100", MaxRepeat: "128", TimeSource: "auto", ScaleProfile: "none", OutDir: filepath.Join(os.TempDir(), "leia-triage")}
	fs := flag.NewFlagSet("bench triage", flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.Var((*benchStringList)(&cfg.Benches), "bench", "Benchmark selector; repeatable.")
	fs.Var((*benchStringList)(&cfg.Modes), "mode", "Execution mode; repeatable.")
	fs.StringVar(&cfg.Runs, "runs", cfg.Runs, "Timing runs.")
	fs.StringVar(&cfg.Warmup, "warmup", cfg.Warmup, "Timing warmup.")
	fs.StringVar(&cfg.Timeout, "timeout", cfg.Timeout, "Timeout seconds.")
	fs.StringVar(&cfg.MinSampleSeconds, "min-sample-seconds", cfg.MinSampleSeconds, "Minimum sample seconds.")
	fs.StringVar(&cfg.MaxRepeat, "max-repeat", cfg.MaxRepeat, "Maximum repeat.")
	fs.StringVar(&cfg.TimeSource, "time-source", cfg.TimeSource, "Time source.")
	fs.Var((*benchStringList)(&cfg.Scale), "scale", "Scale override.")
	fs.Var((*benchStringList)(&cfg.Scale), "param", "Scale override alias.")
	fs.StringVar(&cfg.ScaleProfile, "scale-profile", cfg.ScaleProfile, "Scale profile.")
	fs.BoolVar(&cfg.Diag, "diag", false, "Collect diagnose artifacts.")
	fs.BoolVar(&cfg.PPROF, "pprof", false, "Collect pprof evidence.")
	fs.StringVar(&cfg.MemProfile, "memprofile", "", "Collect memory profile.")
	fs.StringVar(&cfg.RuntimeStats, "runtime-stats", "", "Runtime stats input.")
	fs.StringVar(&cfg.SpecState, "spec-state", "", "Speculation state input.")
	fs.BoolVar(&cfg.NoSpecState, "no-spec-state", false, "Skip speculation state.")
	fs.BoolVar(&cfg.WarmDump, "warm-dump", false, "Collect warm dump evidence.")
	fs.StringVar(&cfg.OutDir, "out-dir", cfg.OutDir, "Output directory.")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(errw, "leia bench triage: unexpected argument %q\n", fs.Arg(0))
		return cfg, flag.ErrHelp
	}
	return cfg, nil
}

func benchTriageCompareArgs(cfg benchTriageConfig, timingJSON, timingMD string) []string {
	args := []string{
		"--runs", cfg.Runs,
		"--warmup", cfg.Warmup,
		"--timeout", cfg.Timeout,
		"--min-sample-seconds", cfg.MinSampleSeconds,
		"--max-repeat", cfg.MaxRepeat,
		"--time-source", cfg.TimeSource,
		"--head-ref", "HEAD",
		"--json", timingJSON,
		"--markdown", timingMD,
	}
	for _, mode := range cfg.Modes {
		args = append(args, "--mode", mode)
	}
	for _, bench := range cfg.Benches {
		args = append(args, "--bench", bench)
	}
	for _, scale := range cfg.Scale {
		args = append(args, "--scale", scale)
	}
	if cfg.ScaleProfile != "" && cfg.ScaleProfile != "none" {
		args = append(args, "--scale-profile", cfg.ScaleProfile)
	}
	return args
}

func benchTriageRowsFromCompare(path string) ([]map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	rows := []map[string]any{}
	for _, result := range report.Results {
		benchmark := fmt.Sprint(result["benchmark"])
		group := fmt.Sprint(result["group"])
		if group != "" && group != "<nil>" {
			benchmark = group + "/" + benchmark
		}
		scale := map[string]any{}
		if raw, ok := result["scale"].(map[string]any); ok {
			scale = raw
		}
		modes, _ := result["modes"].(map[string]any)
		for mode, rawMode := range modes {
			modeRow, _ := rawMode.(map[string]any)
			current := benchTriageSubject(modeRow["current"])
			head := benchTriageSubject(modeRow["head"])
			luajit := benchTriageSubject(modeRow["luajit"])
			rows = append(rows, map[string]any{
				"benchmark":    benchmark,
				"scale":        scale,
				"mode":         mode,
				"current":      current["seconds"],
				"head":         head["seconds"],
				"luajit":       luajit["seconds"],
				"cur_head":     benchTriageRatio(current["seconds"], head["seconds"]),
				"cur_luajit":   benchTriageRatio(current["seconds"], luajit["seconds"]),
				"status":       map[string]any{"current": current["status"], "head": head["status"], "luajit": luajit["status"]},
				"source":       current["source"],
				"repeat":       current["repeat"],
				"t2_attempted": current["t2_attempted"],
				"t2_entered":   current["t2_entered"],
				"t2_failed":    current["t2_failed"],
				"exits":        current["exit_total"],
				"ci95":         current["ci95"],
				"note":         benchTriageTimingNote(current, head, luajit),
			})
		}
	}
	return rows, nil
}

func benchTriageSubject(raw any) map[string]any {
	subject, _ := raw.(map[string]any)
	stats, _ := subject["stats"].(map[string]any)
	return map[string]any{
		"seconds":      stats["median"],
		"status":       subject["status"],
		"source":       subject["source"],
		"repeat":       subject["repeat"],
		"t2_attempted": subject["t2_attempted"],
		"t2_entered":   subject["t2_entered"],
		"t2_failed":    subject["t2_failed"],
		"exit_total":   subject["exit_total"],
		"ci95":         stats["ci95_half_width_pct"],
	}
}

func benchTriageRatio(a, b any) any {
	left, okLeft := a.(float64)
	right, okRight := b.(float64)
	if !okLeft || !okRight || right == 0 {
		return nil
	}
	return left / right
}

func benchTriageTimingNote(current, head, luajit map[string]any) string {
	curStatus := fmt.Sprint(current["status"])
	headStatus := fmt.Sprint(head["status"])
	luaStatus := fmt.Sprint(luajit["status"])
	if curStatus != "ok" || headStatus != "ok" {
		return "current/head timing needs attention"
	}
	if luaStatus != "ok" && luaStatus != "<nil>" {
		return "LuaJIT reference unavailable or failed"
	}
	return "calibrated timing captured"
}

func benchTriageRecommendations(timing []map[string]any) []string {
	recs := []string{}
	for _, row := range timing {
		if note, _ := row["note"].(string); note != "" && note != "calibrated timing captured" {
			recs = append(recs, fmt.Sprintf("%s: %s", row["benchmark"], note))
		}
	}
	if len(recs) == 0 {
		return []string{"Use timing.json for detailed current/head/LuaJIT ratios; run leia bench diagnose when runtime artifacts are needed."}
	}
	return recs
}

func benchSelectForDiag(root string, groups, benches []string, allGroups bool) ([]benchdisc.Benchmark, error) {
	selectedGroups := groups
	if allGroups {
		selectedGroups = benchdisc.DomainGroups
	}
	if len(selectedGroups) == 0 {
		selectedGroups = benchdisc.DomainGroups
	}
	selectedGroups = benchGroupsForSelectors(selectedGroups, benches)
	specs, err := benchdisc.Discover(root, selectedGroups)
	if err != nil {
		return nil, err
	}
	return benchdisc.SelectSpecs(specs, benches)
}

func benchGroupsForSelectors(groups, selectors []string) []string {
	out := append([]string(nil), groups...)
	seen := map[string]bool{}
	for _, group := range out {
		seen[group] = true
	}
	for _, selector := range selectors {
		id, ok := benchdisc.BenchmarkIDFromSelector(selector, benchdisc.DomainGroups)
		if !ok {
			continue
		}
		group, _, _ := strings.Cut(id, "/")
		if group != "" && !seen[group] {
			out = append(out, group)
			seen[group] = true
		}
	}
	return out
}

func writeJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func benchDiagnoseMarkdown(rows []map[string]any) string {
	var b strings.Builder
	b.WriteString("# Benchmark Diagnostics\n\n")
	b.WriteString("| Benchmark | Time | T2 a/c/e | Exits | Top Exit | Work Target | Runtime Hot Counters | CPU Profile | Artifacts |\n")
	b.WriteString("| --- | ---: | --- | ---: | --- | --- | --- | --- | --- |\n")
	for _, row := range rows {
		name := fmt.Sprintf("%s/%s", row["group"], row["benchmark"])
		tier2 := fmt.Sprintf("%v/%v/%v", row["t2_attempted"], row["t2_compiled"], row["t2_failed"])
		b.WriteString(benchMarkdownRow(name, benchDiagFormatSeconds(row["time_seconds"]), tier2, row["exit_total"], "-", row["work_target"], "-", "-", "`summary.raw.txt`, `run.raw.txt`"))
		b.WriteByte('\n')
	}
	return b.String()
}

func benchTriageMarkdown(timing []map[string]any) string {
	var b strings.Builder
	b.WriteString("# Performance Triage\n\n")
	b.WriteString("## Optimization Priorities\n\nNo high-confidence bottleneck has been collected yet.\n\n")
	b.WriteString("## Timing\n\n")
	b.WriteString("| Benchmark | Mode | Current | HEAD | LuaJIT | T2 | Exits | Note |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, row := range timing {
		tier2 := fmt.Sprintf("%v/%v/%v", row["t2_attempted"], row["t2_entered"], row["t2_failed"])
		b.WriteString(benchMarkdownRow(row["benchmark"], row["mode"], benchDiagFormatSeconds(row["current"]), benchDiagFormatSeconds(row["head"]), benchDiagFormatSeconds(row["luajit"]), tier2, row["exits"], row["note"]))
		b.WriteByte('\n')
	}
	b.WriteString("\n## Artifacts\n\nGenerated by `leia bench triage`.\n\n## Artifact Status\n\nNo external artifacts were requested.\n")
	return b.String()
}

func benchDiagFormatSeconds(value any) string {
	switch v := value.(type) {
	case float64:
		return fmt.Sprintf("%.6fs", v)
	case *float64:
		if v == nil {
			return "-"
		}
		return fmt.Sprintf("%.6fs", *v)
	default:
		return "-"
	}
}
