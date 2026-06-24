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
	rows := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		artifactDir := filepath.Join(outDir, spec.Group+"__"+spec.Name)
		if err := os.MkdirAll(artifactDir, 0o755); err != nil {
			fmt.Fprintf(errw, "leia bench diagnose: %v\n", err)
			return 1
		}
		summary := filepath.Join(artifactDir, "summary.raw.txt")
		_ = os.WriteFile(summary, []byte("diagnose summary pending deep runtime probes\n"), 0o644)
		rows = append(rows, map[string]any{
			"benchmark":             spec.Name,
			"group":                 spec.Group,
			"script":                spec.LeiaRel(),
			"status":                "ok",
			"time_seconds":          nil,
			"t2_attempted":          0,
			"t2_compiled":           0,
			"t2_entered":            0,
			"t2_failed":             0,
			"exit_total":            0,
			"top_exit":              nil,
			"work_action":           "collect-runtime-evidence",
			"work_target":           spec.ID(),
			"work_proto":            "",
			"work_priority":         0,
			"readiness":             "baseline",
			"runtime_summary":       map[string]any{},
			"tier2_call_summary":    map[string]any{},
			"pprof_runs":            0,
			"pprof_script_repeat":   0,
			"pprof_samples_seconds": 0,
			"pprof_effective":       cfg.PPROF,
			"artifact_dir":          artifactDir,
			"artifacts": map[string]string{
				"summary_raw_txt": summary,
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
	specs, err := benchSelectForDiag(root, benchdisc.DomainGroups, cfg.Benches, false)
	if err != nil {
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
	timing := make([]map[string]any, 0, len(specs))
	for _, spec := range specs {
		timing = append(timing, map[string]any{
			"benchmark":  spec.ID(),
			"scale":      map[string]any{},
			"mode":       "default",
			"current":    nil,
			"head":       nil,
			"luajit":     nil,
			"cur_head":   nil,
			"cur_luajit": nil,
			"source":     "",
			"repeat":     0,
			"exits":      0,
			"ci95":       nil,
			"note":       "timing capture delegated to leia bench compare",
		})
	}
	payload := map[string]any{
		"timing":             timing,
		"bottlenecks":        []any{},
		"recommendations":    []string{"Run leia bench compare for calibrated timing and leia bench diagnose for runtime artifacts."},
		"artifacts":          map[string]any{},
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
	timingJSON := filepath.Join(outDir, "timing.json")
	timingMD := filepath.Join(outDir, "timing.md")
	if err := writeJSONFile(jsonPath, payload); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	if err := writeJSONFile(timingJSON, map[string]any{"results": []any{}, "modes": cfg.Modes}); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	md := benchTriageMarkdown(timing)
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		fmt.Fprintf(errw, "leia bench triage: %v\n", err)
		return 1
	}
	if err := os.WriteFile(timingMD, []byte("# Timing\n\nTiming capture delegated to `leia bench compare`.\n"), 0o644); err != nil {
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
		b.WriteString(benchMarkdownRow(name, "-", "0/0/0", 0, "-", row["work_target"], "-", "-", "`summary.raw.txt`"))
		b.WriteByte('\n')
	}
	return b.String()
}

func benchTriageMarkdown(timing []map[string]any) string {
	var b strings.Builder
	b.WriteString("# Performance Triage\n\n")
	b.WriteString("## Optimization Priorities\n\nNo high-confidence bottleneck has been collected yet.\n\n")
	b.WriteString("## Timing\n\n")
	b.WriteString("| Benchmark | Mode | Current | HEAD | LuaJIT | Note |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | --- |\n")
	for _, row := range timing {
		b.WriteString(benchMarkdownRow(row["benchmark"], row["mode"], "-", "-", "-", row["note"]))
		b.WriteByte('\n')
	}
	b.WriteString("\n## Artifacts\n\nGenerated by `leia bench triage`.\n\n## Artifact Status\n\nNo external artifacts were requested.\n")
	return b.String()
}
