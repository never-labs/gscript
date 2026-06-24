package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
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
	PPROFMinMS   float64
	PPROFMaxRuns int
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
	if !cfg.NoTiming || cfg.WarmDump {
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
		warmDump := benchCollectDiagnoseWarmDump(runSpec, leiaBin, artifactDir, timeout, cfg)
		artifacts := map[string]string{
			"summary_raw_txt": summary,
			"run_raw_txt":     raw,
		}
		for pathName, pathValue := range warmDump.Artifacts {
			artifacts[pathName] = pathValue
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
			"runtime_summary":       benchDiagnoseRuntimeSummary(diag),
			"tier2_call_summary":    benchDiagnoseTier2Summary(diag),
			"pprof_requested":       cfg.PPROF,
			"pprof_min_samples_ms":  cfg.PPROFMinMS,
			"pprof_max_runs":        cfg.PPROFMaxRuns,
			"pprof_runs":            0,
			"pprof_script_repeat":   0,
			"pprof_samples_seconds": 0,
			"pprof_effective":       false,
			"pprof_summary": benchDiagnoseEvidenceSummary(
				cfg.PPROF,
				false,
				"pprof collection is not yet wired for bench diagnose",
				map[string]any{"min_samples_ms": cfg.PPROFMinMS, "max_runs": cfg.PPROFMaxRuns},
			),
			"warm_dump_requested": cfg.WarmDump,
			"warm_dump_effective": warmDump.Effective,
			"warm_dump_summary":   warmDump.Summary,
			"scale":               scale,
			"artifact_dir":        artifactDir,
			"artifacts":           artifacts,
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
	cfg := benchDiagConfig{Groups: []string{"numeric", "recursion", "table", "calls", "string", "app", "control"}, Timeout: "120", Runs: 5, Warmup: 1, ScaleProfile: "none", PPROFMinMS: 50, PPROFMaxRuns: 8}
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
	fs.Float64Var(&cfg.PPROFMinMS, "pprof-min-samples-ms", cfg.PPROFMinMS, "Minimum requested CPU profile sample milliseconds.")
	fs.IntVar(&cfg.PPROFMaxRuns, "pprof-max-runs", cfg.PPROFMaxRuns, "Maximum requested CPU profile collection runs.")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(errw, "leia bench diagnose: unexpected argument %q\n", fs.Arg(0))
		return cfg, flag.ErrHelp
	}
	if cfg.PPROFMinMS < 0 {
		return cfg, fmt.Errorf("--pprof-min-samples-ms must be >= 0")
	}
	if cfg.PPROFMaxRuns < 1 {
		return cfg, fmt.Errorf("--pprof-max-runs must be >= 1")
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

type benchDiagnoseWarmDump struct {
	Effective bool
	Summary   map[string]any
	Artifacts map[string]string
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

func benchCollectDiagnoseWarmDump(spec benchdisc.Benchmark, leiaBin, artifactDir string, timeout time.Duration, cfg benchDiagConfig) benchDiagnoseWarmDump {
	if !cfg.WarmDump {
		return benchDiagnoseWarmDump{Summary: benchDiagnoseEvidenceSummary(false, false, "warm dump not requested", nil), Artifacts: map[string]string{}}
	}
	warmDir := filepath.Join(artifactDir, "warm-dump")
	rawPath := filepath.Join(artifactDir, "warm-dump.raw.txt")
	if leiaBin == "" {
		return benchDiagnoseWarmDump{
			Summary:   benchDiagnoseEvidenceSummary(true, false, "leia binary unavailable for warm dump", map[string]any{"dir": warmDir}),
			Artifacts: map[string]string{"warm_dump_dir": warmDir, "warm_dump_raw_txt": rawPath},
		}
	}
	cmd, err := benchBenchmarkModeCommand("default", leiaBin, spec.Leia, "", "")
	if err != nil {
		return benchDiagnoseWarmDump{
			Summary:   benchDiagnoseEvidenceSummary(true, false, err.Error(), map[string]any{"dir": warmDir}),
			Artifacts: map[string]string{"warm_dump_dir": warmDir, "warm_dump_raw_txt": rawPath},
		}
	}
	if cmd.Unavailable != "" {
		return benchDiagnoseWarmDump{
			Summary:   benchDiagnoseEvidenceSummary(true, false, "input unavailable: "+cmd.Unavailable, map[string]any{"dir": warmDir}),
			Artifacts: map[string]string{"warm_dump_dir": warmDir, "warm_dump_raw_txt": rawPath},
		}
	}
	args := benchDiagnoseWarmDumpArgs(cmd.Args, warmDir)
	run := benchRunTextCommand(args, timeout, cmd.Env)
	_ = os.WriteFile(rawPath, []byte(run.Output), 0o644)
	summary := benchDiagnoseWarmDumpSummary(warmDir, run)
	return benchDiagnoseWarmDump{
		Effective: benchDiagnoseBool(summary["effective"]),
		Summary:   summary,
		Artifacts: benchDiagnoseWarmDumpArtifacts(warmDir, rawPath),
	}
}

func benchDiagnoseWarmDumpArgs(args []string, warmDir string) []string {
	if len(args) <= 1 {
		return append(append([]string(nil), args...), "-jit-dump-warm", warmDir)
	}
	out := make([]string, 0, len(args)+2)
	out = append(out, args[:len(args)-1]...)
	out = append(out, "-jit-dump-warm", warmDir)
	out = append(out, args[len(args)-1])
	return out
}

func benchDiagnoseWarmDumpSummary(warmDir string, run benchTextCommandResult) map[string]any {
	files := benchDiagnoseWarmDumpFiles(warmDir)
	manifest := filepath.Join(warmDir, "manifest.json")
	pcmap := filepath.Join(warmDir, "pcmap.json")
	jitSymbols := filepath.Join(warmDir, "jit-symbols.txt")
	effective := run.Status == "ok" && len(files) > 0
	reason := ""
	if !effective {
		reason = "warm dump command did not produce artifacts"
		if run.Status != "ok" {
			reason = "warm dump command status: " + run.Status
		}
	}
	summary := benchDiagnoseEvidenceSummary(true, effective, reason, map[string]any{
		"dir":                warmDir,
		"files":              files,
		"file_count":         len(files),
		"manifest_exists":    fileExists(manifest),
		"pcmap_exists":       fileExists(pcmap),
		"jit_symbols_exists": fileExists(jitSymbols),
		"command_status":     run.Status,
		"wall_seconds":       run.WallSeconds,
	})
	if run.ExitCode != nil {
		summary["exit_code"] = *run.ExitCode
	}
	return summary
}

func benchDiagnoseWarmDumpFiles(warmDir string) []string {
	var files []string
	_ = filepath.WalkDir(warmDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(warmDir, path)
		if relErr == nil {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func benchDiagnoseWarmDumpArtifacts(warmDir, rawPath string) map[string]string {
	artifacts := map[string]string{
		"warm_dump_dir":     warmDir,
		"warm_dump_raw_txt": rawPath,
	}
	for key, name := range map[string]string{
		"warm_dump_manifest_json": "manifest.json",
		"warm_dump_pcmap_json":    "pcmap.json",
		"warm_dump_jit_symbols":   "jit-symbols.txt",
	} {
		path := filepath.Join(warmDir, name)
		if fileExists(path) {
			artifacts[key] = path
		}
	}
	return artifacts
}

func benchDiagnoseBool(value any) bool {
	out, _ := value.(bool)
	return out
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

func benchDiagnoseRuntimeSummary(diag benchDiagnoseRun) map[string]any {
	return map[string]any{
		"status":       diag.Status,
		"readiness":    benchDiagnoseReadiness(diag),
		"exit_total":   diag.ExitTotal,
		"has_exits":    diag.ExitTotal > 0,
		"wall_seconds": diag.WallSeconds,
	}
}

func benchDiagnoseTier2Summary(diag benchDiagnoseRun) map[string]any {
	summary := map[string]any{
		"attempted": diag.T2Attempted,
		"entered":   diag.T2Entered,
		"failed":    diag.T2Failed,
	}
	if diag.T2Attempted > 0 {
		summary["entered_pct"] = float64(diag.T2Entered) / float64(diag.T2Attempted) * 100
		summary["failed_pct"] = float64(diag.T2Failed) / float64(diag.T2Attempted) * 100
	} else {
		summary["entered_pct"] = 0.0
		summary["failed_pct"] = 0.0
	}
	return summary
}

func benchDiagnoseEvidenceSummary(requested, effective bool, reason string, params map[string]any) map[string]any {
	status := "not_requested"
	if requested {
		status = "not_collected"
	}
	if effective {
		status = "ok"
		reason = ""
	}
	summary := map[string]any{
		"requested": requested,
		"effective": effective,
		"status":    status,
		"reason":    reason,
	}
	if len(params) > 0 {
		summary["params"] = params
	}
	return summary
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
	bottlenecks := benchTriageBottlenecks(timing)
	recommendations := benchTriageRecommendations(timing)
	artifacts := map[string]any{
		"timing_json": map[string]any{"path": timingJSON, "status": "ok", "note": "compare JSON report"},
		"timing_md":   map[string]any{"path": timingMD, "status": "ok", "note": "compare Markdown report"},
	}
	payload := map[string]any{
		"timing":             timing,
		"bottlenecks":        bottlenecks,
		"recommendations":    recommendations,
		"artifacts":          artifacts,
		"exit_summary":       benchTriageExitSummary(timing),
		"pprof_summary":      benchTriageNotCollectedSummary("pass --pprof after selecting a benchmark that needs CPU profile evidence"),
		"pcmap_summary":      benchTriageNotCollectedSummary("pass --warm-dump with pprof evidence when JIT PC mapping is needed"),
		"memprofile_summary": benchTriageNotCollectedSummary("pass --memprofile when allocation evidence is needed"),
		"runtime_stats":      benchTriageNotCollectedSummary("pass --runtime-stats or run diagnose for runtime counter evidence"),
		"speculation_state": map[string]any{
			"status": "not_collected",
			"reason": "pass --spec-state when Tier 2 speculation state is needed",
		},
	}
	jsonPath := filepath.Join(outDir, "triage.json")
	mdPath := filepath.Join(outDir, "triage.md")
	if err := writeJSONFile(jsonPath, payload); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	md := benchTriageMarkdown(timing, bottlenecks, recommendations, artifacts)
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
		return []string{"Use timing.json for detailed ratios; run leia bench diagnose when per-run raw output is needed."}
	}
	return recs
}

func benchTriageExitSummary(timing []map[string]any) map[string]any {
	total := 0
	statuses := map[string]int{}
	for _, row := range timing {
		if exits, ok := benchTriageInt(row["exits"]); ok {
			total += exits
		}
		if statusMap, ok := row["status"].(map[string]any); ok {
			for subject, status := range statusMap {
				key := subject + ":" + fmt.Sprint(status)
				statuses[key]++
			}
		}
	}
	return map[string]any{
		"total":     total,
		"by_code":   map[string]any{},
		"by_reason": map[string]any{},
		"top_sites": []any{},
		"statuses":  statuses,
	}
}

func benchTriageNotCollectedSummary(reason string) map[string]any {
	return map[string]any{
		"status": "not_collected",
		"reason": reason,
	}
}

func benchTriageBottlenecks(timing []map[string]any) []map[string]any {
	out := []map[string]any{}
	add := func(row map[string]any, category, priority, confidence, evidence, recommendation string) {
		out = append(out, map[string]any{
			"benchmark":      row["benchmark"],
			"mode":           row["mode"],
			"category":       category,
			"priority":       priority,
			"confidence":     confidence,
			"evidence":       evidence,
			"recommendation": recommendation,
		})
	}
	for _, row := range timing {
		note, _ := row["note"].(string)
		if note != "" && note != "calibrated timing captured" {
			add(row, "timing-status", "P1", "high", note, "open timing.json and fix failed or unavailable timing subjects")
		}
		if ratio, ok := benchTriageFloat(row["cur_head"]); ok && ratio > 1.10 {
			add(row, "current-head-regression", "P1", "medium", fmt.Sprintf("current/head %.3fx", ratio), "profile current and clean HEAD for this benchmark")
		}
		if ratio, ok := benchTriageFloat(row["cur_luajit"]); ok && ratio > 0.85 {
			add(row, "luajit-gap", "P2", "medium", fmt.Sprintf("current/LuaJIT %.3fx", ratio), "rank LuaJIT gaps and inspect JIT/runtime exits")
		}
		if failed, ok := benchTriageInt(row["t2_failed"]); ok && failed > 0 {
			add(row, "tier2-failed", "P1", "high", fmt.Sprintf("t2_failed=%d", failed), "run bench diagnose and inspect Tier 2 failure reasons")
		}
		if exits, ok := benchTriageInt(row["exits"]); ok && exits > 0 {
			add(row, "runtime-exits", "P2", "high", fmt.Sprintf("exits=%d", exits), "run bench profile-exits or diagnose for top exit sites")
		}
	}
	return out
}

func benchTriageFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func benchTriageInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
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
		b.WriteString(benchMarkdownRow(name, benchDiagFormatSeconds(row["time_seconds"]), tier2, row["exit_total"], "-", row["work_target"], benchDiagnoseRuntimeText(row), benchDiagnoseEvidenceText(row["pprof_summary"]), benchDiagnoseArtifactText(row)))
		b.WriteByte('\n')
	}
	return b.String()
}

func benchDiagnoseRuntimeText(row map[string]any) string {
	runtimeSummary, _ := row["runtime_summary"].(map[string]any)
	if len(runtimeSummary) == 0 {
		return "-"
	}
	return fmt.Sprintf("readiness=%v, exits=%v", runtimeSummary["readiness"], runtimeSummary["exit_total"])
}

func benchDiagnoseEvidenceText(value any) string {
	summary, _ := value.(map[string]any)
	if len(summary) == 0 {
		return "-"
	}
	status := fmt.Sprint(summary["status"])
	if status == "" || status == "<nil>" {
		return "-"
	}
	if reason := strings.TrimSpace(fmt.Sprint(summary["reason"])); reason != "" && reason != "<nil>" {
		return status + ": " + reason + benchDiagnoseEvidenceParamsText(summary["params"])
	}
	return status + benchDiagnoseEvidenceParamsText(summary["params"])
}

func benchDiagnoseEvidenceParamsText(value any) string {
	params, _ := value.(map[string]any)
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", key, params[key]))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

func benchDiagnoseArtifactText(row map[string]any) string {
	items := []string{"`summary.raw.txt`", "`run.raw.txt`"}
	if text := benchDiagnoseEvidenceText(row["pprof_summary"]); text != "-" {
		items = append(items, "pprof "+text)
	}
	if text := benchDiagnoseEvidenceText(row["warm_dump_summary"]); text != "-" {
		items = append(items, "warm-dump "+text)
	}
	return strings.Join(items, ", ")
}

func benchTriageMarkdown(timing []map[string]any, bottlenecks []map[string]any, recommendations []string, artifacts map[string]any) string {
	var b strings.Builder
	b.WriteString("# Performance Triage\n\n")
	b.WriteString("## Optimization Priorities\n\n")
	if len(bottlenecks) == 0 {
		b.WriteString("No high-confidence bottleneck was detected from timing counters.\n\n")
	} else {
		b.WriteString("| Priority | Benchmark | Mode | Category | Confidence | Evidence | Next step |\n")
		b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
		for _, item := range bottlenecks {
			b.WriteString(benchMarkdownRow(item["priority"], item["benchmark"], item["mode"], item["category"], item["confidence"], item["evidence"], item["recommendation"]))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}
	b.WriteString("## Recommendations\n\n")
	for _, rec := range recommendations {
		fmt.Fprintf(&b, "- %s\n", rec)
	}
	b.WriteByte('\n')
	b.WriteString("## Timing\n\n")
	b.WriteString("| Benchmark | Mode | Current | HEAD | LuaJIT | T2 | Exits | Note |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | --- |\n")
	for _, row := range timing {
		tier2 := fmt.Sprintf("%v/%v/%v", row["t2_attempted"], row["t2_entered"], row["t2_failed"])
		b.WriteString(benchMarkdownRow(row["benchmark"], row["mode"], benchDiagFormatSeconds(row["current"]), benchDiagFormatSeconds(row["head"]), benchDiagFormatSeconds(row["luajit"]), tier2, row["exits"], row["note"]))
		b.WriteByte('\n')
	}
	b.WriteString("\n## Artifacts\n\nGenerated by `leia bench triage`.\n\n")
	b.WriteString("| Artifact | Status | Path | Note |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	keys := make([]string, 0, len(artifacts))
	for key := range artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item, _ := artifacts[key].(map[string]any)
		b.WriteString(benchMarkdownRow(key, item["status"], item["path"], item["note"]))
		b.WriteByte('\n')
	}
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
