package main

import (
	"archive/tar"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/never-labs/leia/internal/tooling/benchdisc"
)

var benchGoHarnessModes = []string{"default", "vm", "no_filter"}

type benchGoHarnessConfig struct {
	Mode                  string
	BenchSelectors        []string
	Groups                []string
	Modes                 []string
	Runs                  int
	Warmup                int
	Timeout               time.Duration
	MinSampleSeconds      float64
	TimerResolution       float64
	MaxRepeat             int
	MinWallRepeat         int
	TimeSource            string
	NoWallFallback        bool
	NoLuaJIT              bool
	AllGroups             bool
	DryRun                bool
	Jobs                  int
	JSONPath              string
	MarkdownPath          string
	HeadRef               string
	Progress              bool
	Sort                  string
	ScaleProfile          string
	Scale                 []string
	SameWorkload          bool
	TimeoutRaw            string
	SuspiciousVMSpeedup   float64
	SuspiciousLuaJITRatio float64
	RelatedConfirmRatio   float64
}

type benchGoStats struct {
	N                int      `json:"n"`
	Median           *float64 `json:"median,omitempty"`
	Mean             *float64 `json:"mean,omitempty"`
	Min              *float64 `json:"min,omitempty"`
	Max              *float64 `json:"max,omitempty"`
	Stdev            *float64 `json:"stdev,omitempty"`
	MAD              *float64 `json:"mad,omitempty"`
	CVPct            *float64 `json:"cv_pct,omitempty"`
	CI95Low          *float64 `json:"ci95_low,omitempty"`
	CI95High         *float64 `json:"ci95_high,omitempty"`
	CI95HalfWidth    *float64 `json:"ci95_half_width,omitempty"`
	CI95HalfWidthPct *float64 `json:"ci95_half_width_pct,omitempty"`
}

type benchGoSample struct {
	Status             string   `json:"status"`
	Seconds            *float64 `json:"seconds,omitempty"`
	Repeat             int      `json:"repeat"`
	Source             string   `json:"source,omitempty"`
	ScriptTotalSeconds *float64 `json:"script_total_seconds,omitempty"`
	WallTotalSeconds   float64  `json:"wall_total_seconds"`
	T2Attempted        int      `json:"t2_attempted"`
	T2Entered          int      `json:"t2_entered"`
	T2Failed           int      `json:"t2_failed"`
	ExitTotal          int      `json:"exit_total"`
	Note               string   `json:"note,omitempty"`
}

type benchGoSubjectResult struct {
	Subject        string          `json:"subject,omitempty"`
	Mode           string          `json:"mode,omitempty"`
	Status         string          `json:"status"`
	Repeat         int             `json:"repeat"`
	Source         string          `json:"source,omitempty"`
	Stats          benchGoStats    `json:"stats"`
	Samples        []benchGoSample `json:"samples,omitempty"`
	Warmups        []benchGoSample `json:"warmups,omitempty"`
	OutputHash     string          `json:"output_hash,omitempty"`
	ChecksumText   string          `json:"checksum_text,omitempty"`
	ChecksumStatus string          `json:"checksum_status,omitempty"`
	T2Attempted    int             `json:"t2_attempted"`
	T2Entered      int             `json:"t2_entered"`
	T2Failed       int             `json:"t2_failed"`
	ExitTotal      int             `json:"exit_total"`
	Note           string          `json:"note,omitempty"`
	Diagnostic     map[string]any  `json:"diagnostic,omitempty"`
}

type benchGoBenchmarkResult struct {
	Benchmark string                                     `json:"benchmark"`
	Group     string                                     `json:"group"`
	Modes     map[string]map[string]benchGoSubjectResult `json:"modes,omitempty"`
	Strict    map[string]benchGoSubjectResult            `json:"-"`
	Base      string                                     `json:"base,omitempty"`
	Scale     map[string]string                          `json:"scale,omitempty"`
}

type benchGoReport struct {
	SchemaVersion    int                `json:"schema_version"`
	Mode             string             `json:"mode"`
	Modes            []string           `json:"modes"`
	Results          []map[string]any   `json:"results"`
	Timestamp        string             `json:"timestamp,omitempty"`
	GeneratedAt      string             `json:"generated_at"`
	DurationSeconds  float64            `json:"duration_seconds"`
	HeadRef          string             `json:"head_ref,omitempty"`
	Groups           []string           `json:"groups,omitempty"`
	Benchmarks       []string           `json:"benchmarks,omitempty"`
	Runs             int                `json:"runs,omitempty"`
	Warmup           int                `json:"warmup,omitempty"`
	TimeoutSeconds   float64            `json:"timeout_seconds,omitempty"`
	MinSampleSeconds float64            `json:"min_sample_seconds,omitempty"`
	TimerResolution  float64            `json:"timer_resolution,omitempty"`
	MaxRepeat        int                `json:"max_repeat,omitempty"`
	MinWallRepeat    int                `json:"min_wall_repeat,omitempty"`
	WallFallback     bool               `json:"wall_fallback"`
	TimeSource       string             `json:"time_source,omitempty"`
	Sort             string             `json:"sort,omitempty"`
	ScaleProfile     string             `json:"scale_profile,omitempty"`
	Scale            []string           `json:"scale,omitempty"`
	StrictThresholds map[string]float64 `json:"strict_thresholds,omitempty"`
	Platform         map[string]string  `json:"platform,omitempty"`
	Notes            []string           `json:"notes,omitempty"`
}

func runBenchCompareCommand(args []string, outw, errw io.Writer) int {
	return runBenchGoHarness("compare", args, outw, errw)
}

func runBenchStrictCommand(args []string, outw, errw io.Writer) int {
	return runBenchGoHarness("strict", args, outw, errw)
}

func runBenchGoHarness(mode string, args []string, outw, errw io.Writer) int {
	started := time.Now()
	cfg, err := parseBenchGoHarnessConfig(mode, args, errw)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
		return 2
	}
	root, err := benchRepoRoot()
	if err != nil {
		fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
		return 1
	}
	cfg.Groups = benchGroupsForSelectors(cfg.Groups, cfg.BenchSelectors)
	specs, err := benchdisc.Discover(root, cfg.Groups)
	if err != nil {
		fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
		return 2
	}
	specs, err = benchdisc.SelectSpecs(specs, cfg.BenchSelectors)
	if err != nil {
		fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
		return 2
	}
	if len(specs) == 0 {
		fmt.Fprintf(errw, "leia bench %s: no benchmarks selected\n", mode)
		return 2
	}

	tempDir, err := os.MkdirTemp("", "leia-bench-"+mode+"-")
	if err != nil {
		fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
		return 1
	}
	defer os.RemoveAll(tempDir)

	leiaBin := ""
	if !cfg.DryRun {
		leiaBin = filepath.Join(tempDir, "leia-current")
		if err := benchBuildLeia(root, leiaBin, "build failed in {root} with exit {exit_code}", errw); err != nil {
			fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
			return 1
		}
	}
	headRoot := ""
	headLeiaBin := ""
	if mode == "compare" && cfg.HeadRef != "" && !cfg.DryRun {
		headRoot = filepath.Join(tempDir, "head-src")
		if err := benchExportGitRef(root, cfg.HeadRef, headRoot); err != nil {
			fmt.Fprintf(errw, "leia bench %s: export --head-ref %s: %v\n", mode, cfg.HeadRef, err)
			return 1
		}
		headLeiaBin = filepath.Join(tempDir, "leia-head")
		if err := benchBuildLeia(headRoot, headLeiaBin, "head build failed in {root} with exit {exit_code}", errw); err != nil {
			fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
			return 1
		}
	}
	luajitBin := ""
	if !cfg.NoLuaJIT {
		luajitBin = findExecutable("luajit")
	}

	results, err := runBenchGoSpecs(mode, root, tempDir, leiaBin, headRoot, headLeiaBin, luajitBin, specs, cfg, errw)
	if err != nil {
		fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
		return 2
	}
	results = sortBenchGoResults(results, cfg)

	report := buildBenchGoReport(cfg, results, time.Since(started).Seconds())
	if cfg.JSONPath != "" {
		if err := writeBenchGoJSON(cfg.JSONPath, report); err != nil {
			fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
			return 1
		}
		fmt.Fprintf(outw, "wrote %s\n", cfg.JSONPath)
	}
	if cfg.MarkdownPath != "" {
		if err := writeBenchGoMarkdown(cfg.MarkdownPath, cfg, results); err != nil {
			fmt.Fprintf(errw, "leia bench %s: %v\n", mode, err)
			return 1
		}
		fmt.Fprintf(outw, "wrote %s\n", cfg.MarkdownPath)
	}
	if cfg.JSONPath == "" && cfg.MarkdownPath == "" {
		_, _ = io.WriteString(outw, benchGoMarkdown(cfg, results))
	}
	return 0
}

func parseBenchGoHarnessConfig(mode string, args []string, errw io.Writer) (benchGoHarnessConfig, error) {
	defaultGroups := []string{"numeric", "recursion", "table", "calls", "string", "app", "control"}
	defaultModes := []string{"default"}
	if mode == "strict" {
		defaultModes = []string{"vm", "default", "no_filter"}
	}
	cfg := benchGoHarnessConfig{
		Mode:                  mode,
		Groups:                append([]string(nil), defaultGroups...),
		Runs:                  3,
		Warmup:                1,
		Timeout:               60 * time.Second,
		MinSampleSeconds:      0.020,
		TimerResolution:       0.001,
		MaxRepeat:             128,
		MinWallRepeat:         4,
		TimeSource:            "auto",
		NoWallFallback:        mode == "strict",
		Jobs:                  1,
		Sort:                  "name",
		SuspiciousVMSpeedup:   2.0,
		SuspiciousLuaJITRatio: 0.75,
		RelatedConfirmRatio:   0.95,
	}
	var explicitGroups benchStringList
	var explicitModes benchStringList
	var repeatOverrides benchStringList
	var allowWallTime bool
	fs := flag.NewFlagSet("bench "+mode, flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.Var((*benchStringList)(&cfg.BenchSelectors), "bench", "Benchmark selector; repeatable.")
	fs.Var(&explicitGroups, "group", "Benchmark group; repeatable.")
	fs.Var(&explicitModes, "mode", "Execution mode; repeatable.")
	fs.IntVar(&cfg.Runs, "runs", cfg.Runs, "Measured runs.")
	fs.IntVar(&cfg.Warmup, "warmup", cfg.Warmup, "Warmup runs.")
	cfg.TimeoutRaw = "60"
	fs.StringVar(&cfg.TimeoutRaw, "timeout", cfg.TimeoutRaw, "Per command timeout in seconds or Go duration syntax.")
	fs.Float64Var(&cfg.MinSampleSeconds, "min-sample-seconds", cfg.MinSampleSeconds, "Minimum sample seconds.")
	fs.Float64Var(&cfg.TimerResolution, "timer-resolution", cfg.TimerResolution, "Timer resolution.")
	fs.IntVar(&cfg.MaxRepeat, "max-repeat", cfg.MaxRepeat, "Maximum repeat calibration.")
	fs.IntVar(&cfg.MinWallRepeat, "min-wall-repeat", cfg.MinWallRepeat, "Minimum wall repeat count.")
	fs.StringVar(&cfg.TimeSource, "time-source", cfg.TimeSource, "Time source: auto, script, or wall.")
	fs.BoolVar(&cfg.NoWallFallback, "no-wall-fallback", cfg.NoWallFallback, "Disable wall fallback.")
	fs.BoolVar(&allowWallTime, "allow-wall-time", false, "Allow wall fallback for strict compatibility.")
	fs.BoolVar(&cfg.NoLuaJIT, "no-luajit", false, "Skip LuaJIT.")
	fs.BoolVar(&cfg.AllGroups, "all-groups", false, "Run all groups.")
	fs.BoolVar(&cfg.DryRun, "dry-run", false, "Write selected benchmark metadata without running commands.")
	fs.BoolVar(&cfg.SameWorkload, "same-workload", false, "Run the current benchmark scripts against the head binary.")
	fs.IntVar(&cfg.Jobs, "jobs", cfg.Jobs, "Parallel benchmark jobs.")
	fs.StringVar(&cfg.JSONPath, "json", "", "JSON report path.")
	fs.StringVar(&cfg.MarkdownPath, "markdown", "", "Markdown report path.")
	fs.StringVar(&cfg.HeadRef, "head-ref", "", "HEAD/reference name for current-vs-old comparison.")
	fs.BoolVar(&cfg.Progress, "progress", false, "Print progress.")
	fs.StringVar(&cfg.Sort, "sort", cfg.Sort, "Sort order: name, luajit-gap, current, or head.")
	fs.StringVar(&cfg.ScaleProfile, "scale-profile", "", "Scale profile.")
	fs.Var((*benchStringList)(&cfg.Scale), "scale", "Scale override.")
	fs.Var((*benchStringList)(&cfg.Scale), "param", "Scale override alias.")
	fs.Var(&repeatOverrides, "repeat", "Scale override alias retained for older strict invocations.")
	fs.IntVar(&cfg.Runs, "measured", cfg.Runs, "Measured runs alias for strict compatibility.")
	fs.Float64Var(&cfg.SuspiciousVMSpeedup, "suspicious-vm-speedup", cfg.SuspiciousVMSpeedup, "Warn when VM/default median ratio exceeds this threshold.")
	fs.Float64Var(&cfg.SuspiciousLuaJITRatio, "suspicious-luajit-ratio", cfg.SuspiciousLuaJITRatio, "Warn when default/LuaJIT median ratio exceeds this threshold.")
	fs.Float64Var(&cfg.RelatedConfirmRatio, "related-confirm-ratio", cfg.RelatedConfirmRatio, "Warn when related strict modes agree below this ratio.")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(errw, "leia bench %s: unexpected argument %q\n", mode, fs.Arg(0))
		return cfg, flag.ErrHelp
	}
	if allowWallTime {
		cfg.NoWallFallback = false
	}
	if len(repeatOverrides) > 0 {
		cfg.Scale = append(cfg.Scale, repeatOverrides...)
	}
	if len(explicitGroups) > 0 {
		cfg.Groups = append([]string(nil), explicitGroups...)
	}
	if len(explicitModes) > 0 {
		cfg.Modes = append([]string(nil), explicitModes...)
	} else {
		cfg.Modes = append([]string(nil), defaultModes...)
	}
	if cfg.AllGroups {
		cfg.Groups = append([]string(nil), benchdisc.DomainGroups...)
	}
	if len(cfg.Groups) == 0 {
		cfg.Groups = append([]string(nil), benchdisc.DomainGroups...)
	}
	for _, runMode := range cfg.Modes {
		if runMode != "vm" && runMode != "default" && runMode != "no_filter" && runMode != "luajit" {
			return cfg, fmt.Errorf("unknown --mode %q", runMode)
		}
	}
	if cfg.Runs < 1 {
		cfg.Runs = 1
	}
	if cfg.MaxRepeat < 1 {
		cfg.MaxRepeat = 1
	}
	timeout, err := parseBenchGoTimeout(cfg.TimeoutRaw)
	if err != nil {
		return cfg, err
	}
	cfg.Timeout = timeout
	if cfg.Timeout <= 0 {
		cfg.Timeout = 60 * time.Second
	}
	if cfg.TimeSource != "auto" && cfg.TimeSource != "script" && cfg.TimeSource != "wall" {
		return cfg, fmt.Errorf("unknown --time-source %q", cfg.TimeSource)
	}
	if !benchGoValidSort(cfg.Sort) {
		return cfg, fmt.Errorf("unknown --sort %q", cfg.Sort)
	}
	if cfg.Jobs < 1 {
		return cfg, fmt.Errorf("--jobs must be >= 1")
	}
	if cfg.SuspiciousVMSpeedup <= 0 {
		return cfg, fmt.Errorf("--suspicious-vm-speedup must be > 0")
	}
	if cfg.SuspiciousLuaJITRatio <= 0 {
		return cfg, fmt.Errorf("--suspicious-luajit-ratio must be > 0")
	}
	if cfg.RelatedConfirmRatio <= 0 || cfg.RelatedConfirmRatio > 1 {
		return cfg, fmt.Errorf("--related-confirm-ratio must be > 0 and <= 1")
	}
	return cfg, nil
}

func benchGoValidSort(value string) bool {
	switch value {
	case "", "name", "luajit-gap", "current", "head":
		return true
	default:
		return false
	}
}

func sortBenchGoResults(rows []benchGoBenchmarkResult, cfg benchGoHarnessConfig) []benchGoBenchmarkResult {
	out := append([]benchGoBenchmarkResult(nil), rows...)
	sortKey := cfg.Sort
	if sortKey == "" {
		sortKey = "name"
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		switch sortKey {
		case "luajit-gap":
			leftRatio, leftOK := benchGoWorstLuaJITRatio(left, cfg)
			rightRatio, rightOK := benchGoWorstLuaJITRatio(right, cfg)
			if leftOK != rightOK {
				return leftOK
			}
			if leftOK && rightOK && leftRatio != rightRatio {
				return leftRatio > rightRatio
			}
		case "current", "head":
			leftSeconds, leftOK := benchGoWorstSubjectMedian(left, cfg, sortKey)
			rightSeconds, rightOK := benchGoWorstSubjectMedian(right, cfg, sortKey)
			if leftOK != rightOK {
				return leftOK
			}
			if leftOK && rightOK && leftSeconds != rightSeconds {
				return leftSeconds > rightSeconds
			}
		}
		return benchGoRowID(left) < benchGoRowID(right)
	})
	return out
}

func benchGoWorstLuaJITRatio(row benchGoBenchmarkResult, cfg benchGoHarnessConfig) (float64, bool) {
	var worst float64
	ok := false
	for _, mode := range cfg.Modes {
		modeRow := row.Modes[mode]
		current, currentOK := modeRow["current"]
		luajit, luaOK := modeRow["luajit"]
		if !currentOK || !luaOK || current.Stats.Median == nil || luajit.Stats.Median == nil || *luajit.Stats.Median <= 0 {
			continue
		}
		ratio := *current.Stats.Median / *luajit.Stats.Median
		if !ok || ratio > worst {
			worst = ratio
			ok = true
		}
	}
	return worst, ok
}

func benchGoWorstSubjectMedian(row benchGoBenchmarkResult, cfg benchGoHarnessConfig, subject string) (float64, bool) {
	var worst float64
	ok := false
	for _, mode := range cfg.Modes {
		modeRow := row.Modes[mode]
		result, exists := modeRow[subject]
		if !exists || result.Stats.Median == nil {
			continue
		}
		if !ok || *result.Stats.Median > worst {
			worst = *result.Stats.Median
			ok = true
		}
	}
	return worst, ok
}

func benchGoRowID(row benchGoBenchmarkResult) string {
	return row.Group + "/" + row.Benchmark
}

func annotateBenchGoStrictResult(row *benchGoBenchmarkResult, cfg benchGoHarnessConfig) {
	warnings := []map[string]any{}
	if ratio, ok := benchGoStrictRatio(row.Strict, "vm", "default"); ok && ratio > cfg.SuspiciousVMSpeedup {
		warnings = append(warnings, map[string]any{
			"kind":      "suspicious-vm-speedup",
			"ratio":     ratio,
			"threshold": cfg.SuspiciousVMSpeedup,
			"message":   fmt.Sprintf("vm/default median ratio %.3fx exceeds %.3fx", ratio, cfg.SuspiciousVMSpeedup),
		})
	}
	if ratio, ok := benchGoStrictRatio(row.Strict, "default", "luajit"); ok && ratio > cfg.SuspiciousLuaJITRatio {
		warnings = append(warnings, map[string]any{
			"kind":      "suspicious-luajit-ratio",
			"ratio":     ratio,
			"threshold": cfg.SuspiciousLuaJITRatio,
			"message":   fmt.Sprintf("default/LuaJIT median ratio %.3fx exceeds %.3fx", ratio, cfg.SuspiciousLuaJITRatio),
		})
	}
	if ratio, ok := benchGoStrictAgreement(row.Strict, "default", "no_filter"); ok && ratio < cfg.RelatedConfirmRatio {
		warnings = append(warnings, map[string]any{
			"kind":      "related-confirm-ratio",
			"ratio":     ratio,
			"threshold": cfg.RelatedConfirmRatio,
			"message":   fmt.Sprintf("default/no_filter median agreement %.3fx below %.3fx", ratio, cfg.RelatedConfirmRatio),
		})
	}
	if len(warnings) == 0 {
		return
	}
	subject := row.Strict["default"]
	subject.Note = appendNote(subject.Note, fmt.Sprintf("%d strict warning(s)", len(warnings)))
	subject.Diagnostic = map[string]any{"warnings": warnings}
	row.Strict["default"] = subject
}

func benchGoStrictRatio(rows map[string]benchGoSubjectResult, numerator, denominator string) (float64, bool) {
	left, leftOK := rows[numerator]
	right, rightOK := rows[denominator]
	if !leftOK || !rightOK || left.Stats.Median == nil || right.Stats.Median == nil || *right.Stats.Median <= 0 {
		return 0, false
	}
	return *left.Stats.Median / *right.Stats.Median, true
}

func benchGoStrictAgreement(rows map[string]benchGoSubjectResult, leftName, rightName string) (float64, bool) {
	left, leftOK := rows[leftName]
	right, rightOK := rows[rightName]
	if !leftOK || !rightOK || left.Stats.Median == nil || right.Stats.Median == nil || *left.Stats.Median <= 0 || *right.Stats.Median <= 0 {
		return 0, false
	}
	leftSeconds := *left.Stats.Median
	rightSeconds := *right.Stats.Median
	if leftSeconds < rightSeconds {
		return leftSeconds / rightSeconds, true
	}
	return rightSeconds / leftSeconds, true
}

type benchGoSpecRunResult struct {
	index  int
	result benchGoBenchmarkResult
	err    error
}

func runBenchGoSpecs(mode, root, tempDir, leiaBin, headRoot, headLeiaBin, luajitBin string, specs []benchdisc.Benchmark, cfg benchGoHarnessConfig, errw io.Writer) ([]benchGoBenchmarkResult, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	jobs := cfg.Jobs
	if jobs > len(specs) {
		jobs = len(specs)
	}
	if jobs < 1 {
		jobs = 1
	}
	results := make([]benchGoBenchmarkResult, len(specs))
	work := make(chan int)
	done := make(chan benchGoSpecRunResult, len(specs))
	var progressMu sync.Mutex
	worker := func() {
		for idx := range work {
			spec := specs[idx]
			if cfg.Progress {
				progressMu.Lock()
				fmt.Fprintf(errw, "bench %s\n", spec.ID())
				progressMu.Unlock()
			}
			result, err := runBenchGoSpec(mode, root, tempDir, leiaBin, headRoot, headLeiaBin, luajitBin, spec, cfg)
			done <- benchGoSpecRunResult{index: idx, result: result, err: err}
		}
	}
	var wg sync.WaitGroup
	wg.Add(jobs)
	for i := 0; i < jobs; i++ {
		go func() {
			defer wg.Done()
			worker()
		}()
	}
	go func() {
		for i := range specs {
			work <- i
		}
		close(work)
		wg.Wait()
		close(done)
	}()
	var firstErr error
	for item := range done {
		if item.err != nil && firstErr == nil {
			firstErr = item.err
		}
		results[item.index] = item.result
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return results, nil
}

func runBenchGoSpec(mode, root, tempDir, leiaBin, headRoot, headLeiaBin, luajitBin string, spec benchdisc.Benchmark, cfg benchGoHarnessConfig) (benchGoBenchmarkResult, error) {
	scale, err := benchGoScaleForSpec(root, spec, cfg)
	if err != nil {
		return benchGoBenchmarkResult{}, fmt.Errorf("%s: %w", spec.ID(), err)
	}
	result := benchGoBenchmarkResult{
		Benchmark: spec.Name,
		Group:     spec.Group,
		Base:      spec.Base,
		Modes:     map[string]map[string]benchGoSubjectResult{},
		Strict:    map[string]benchGoSubjectResult{},
		Scale:     scale,
	}
	if cfg.DryRun {
		for _, runMode := range cfg.Modes {
			current := runBenchGoSubject("current", runMode, root, leiaBin, luajitBin, spec, cfg)
			if mode == "strict" {
				result.Strict[runMode] = current
				continue
			}
			head := current
			head.Subject = "head"
			modeRow := map[string]benchGoSubjectResult{
				"current": current,
				"head":    head,
			}
			if !cfg.NoLuaJIT {
				modeRow["luajit"] = runBenchGoSubject("luajit", runMode, root, leiaBin, luajitBin, spec, cfg)
			}
			result.Modes[runMode] = modeRow
		}
		return result, nil
	}
	currentSpec, err := benchGoPrepareScaledSpec(spec, filepath.Join(tempDir, "scaled", "current", spec.Group+"__"+spec.Name), scale)
	if err != nil {
		return benchGoBenchmarkResult{}, fmt.Errorf("%s: %w", spec.ID(), err)
	}
	if mode == "strict" {
		for _, runMode := range cfg.Modes {
			subject := runBenchGoSubject("current", runMode, root, leiaBin, luajitBin, currentSpec, cfg)
			result.Strict[runMode] = subject
		}
		annotateBenchGoStrictResult(&result, cfg)
		return result, nil
	}
	for _, runMode := range cfg.Modes {
		current := runBenchGoSubject("current", runMode, root, leiaBin, luajitBin, currentSpec, cfg)
		head := current
		head.Subject = "head"
		if cfg.HeadRef != "" {
			headSpec := currentSpec
			if !cfg.SameWorkload {
				headSpec = benchGoHeadSpec(spec, headRoot, false)
				headSpec, err = benchGoPrepareScaledSpec(headSpec, filepath.Join(tempDir, "scaled", "head", spec.Group+"__"+spec.Name), scale)
				if err != nil {
					return benchGoBenchmarkResult{}, fmt.Errorf("%s head: %w", spec.ID(), err)
				}
			}
			head = runBenchGoSubject("head", runMode, headRoot, headLeiaBin, luajitBin, headSpec, cfg)
		}
		modeRow := map[string]benchGoSubjectResult{
			"current": current,
			"head":    head,
		}
		if !cfg.NoLuaJIT {
			modeRow["luajit"] = runBenchGoSubject("luajit", runMode, root, leiaBin, luajitBin, spec, cfg)
		}
		result.Modes[runMode] = modeRow
	}
	return result, nil
}

func benchExportGitRef(root, ref, dest string) error {
	if ref == "" {
		return errors.New("empty git ref")
	}
	cmd := benchExecCommand("git", "archive", "--format=tar", ref)
	cmd.Dir = root
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	readErr := benchUntar(pipe, dest)
	waitErr := cmd.Wait()
	if readErr != nil {
		return readErr
	}
	if waitErr != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			return fmt.Errorf("%w: %s", waitErr, detail)
		}
		return waitErr
	}
	return nil
}

func benchUntar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	cleanDest := filepath.Clean(dest)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
			return fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		target := filepath.Join(cleanDest, name)
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive target %q", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			mode := hdr.FileInfo().Mode()
			if mode == 0 {
				mode = 0o644
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr)
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if hdr.Linkname == "" {
				return fmt.Errorf("empty symlink target for %q", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		default:
			// Git archives should not need devices or other special entries for benchmarks.
		}
	}
}

func benchGoHeadSpec(spec benchdisc.Benchmark, headRoot string, sameWorkload bool) benchdisc.Benchmark {
	head := spec
	if sameWorkload {
		head.Leia = spec.Leia
		head.LuaJIT = spec.LuaJIT
		return head
	}
	head.Leia = filepath.Join(headRoot, filepath.FromSlash(spec.LeiaRel()))
	if spec.LuaJIT != "" {
		head.LuaJIT = filepath.Join(headRoot, filepath.FromSlash(spec.LuaJITRel()))
	}
	return head
}

func benchGoScaleForSpec(root string, spec benchdisc.Benchmark, cfg benchGoHarnessConfig) (map[string]string, error) {
	scale := map[string]string{}
	if cfg.ScaleProfile != "" && cfg.ScaleProfile != "none" {
		profileScale, err := benchGoScaleProfileForSpec(root, spec, cfg.ScaleProfile)
		if err != nil {
			return nil, err
		}
		for key, value := range profileScale {
			scale[key] = value
		}
	}
	for _, raw := range cfg.Scale {
		selector, assignments, ok := strings.Cut(raw, ":")
		if !ok {
			assignments = selector
			selector = ""
		}
		if selector != "" && !benchGoScaleSelectorMatches(selector, spec) {
			continue
		}
		parsed, err := benchGoParseScaleAssignments(assignments)
		if err != nil {
			return nil, fmt.Errorf("invalid --scale %q: %w", raw, err)
		}
		for key, value := range parsed {
			scale[key] = value
		}
	}
	if len(scale) == 0 {
		return nil, nil
	}
	return scale, nil
}

func benchGoScaleProfileForSpec(root string, spec benchdisc.Benchmark, profile string) (map[string]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "benchmarks", "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest struct {
		Workloads []struct {
			ID               string                    `json:"id"`
			RecommendedScale map[string]map[string]any `json:"recommended_scale"`
			Params           map[string]any            `json:"params"`
		} `json:"workloads"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	for _, workload := range manifest.Workloads {
		if workload.ID != spec.ID() {
			continue
		}
		raw := workload.RecommendedScale[profile]
		out := map[string]string{}
		for key, value := range raw {
			text := fmt.Sprint(value)
			if err := benchGoValidateScaleValue(text); err != nil {
				return nil, fmt.Errorf("manifest scale %s.%s: %w", spec.ID(), key, err)
			}
			out[key] = text
		}
		if len(out) == 0 {
			return nil, nil
		}
		return out, nil
	}
	return nil, nil
}

func benchGoScaleSelectorMatches(selector string, spec benchdisc.Benchmark) bool {
	selector = strings.TrimPrefix(strings.TrimSuffix(selector, ".leia"), "benchmarks/")
	return selector == spec.ID() || selector == spec.Name || selector == spec.Group+"/"+spec.Name
}

func benchGoParseScaleAssignments(raw string) (map[string]string, error) {
	out := map[string]string{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("expected KEY=VALUE")
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if !benchGoScaleNameRE.MatchString(key) {
			return nil, fmt.Errorf("invalid key %q", key)
		}
		if err := benchGoValidateScaleValue(value); err != nil {
			return nil, err
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no assignments")
	}
	return out, nil
}

var (
	benchGoScaleNameRE  = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	benchGoScaleValueRE = regexp.MustCompile(`^-?[0-9]+(?:\.[0-9]+)?$`)
)

func benchGoValidateScaleValue(value string) error {
	if !benchGoScaleValueRE.MatchString(value) {
		return fmt.Errorf("invalid value %q; scale values must be numeric literals", value)
	}
	return nil
}

func benchGoPrepareScaledSpec(spec benchdisc.Benchmark, outDir string, scale map[string]string) (benchdisc.Benchmark, error) {
	if len(scale) == 0 {
		return spec, nil
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return spec, err
	}
	scaled := spec
	leiaPath, err := benchGoWriteScaledSource(spec.Leia, filepath.Join(outDir, filepath.Base(spec.Leia)), scale, "leia")
	if err != nil {
		return spec, err
	}
	scaled.Leia = leiaPath
	if spec.LuaJIT != "" && benchFileExists(spec.LuaJIT) {
		luaPath, err := benchGoWriteScaledSource(spec.LuaJIT, filepath.Join(outDir, filepath.Base(spec.LuaJIT)), scale, "lua")
		if err != nil {
			return spec, err
		}
		scaled.LuaJIT = luaPath
	}
	return scaled, nil
}

func benchGoWriteScaledSource(src, dest string, scale map[string]string, syntax string) (string, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	text := string(data)
	for _, key := range benchGoSortedScaleKeys(scale) {
		next, ok := benchGoReplaceScaleConstant(text, key, scale[key], syntax)
		if !ok {
			return "", fmt.Errorf("%s has no top-level %s constant for scale override", src, key)
		}
		text = next
	}
	if err := os.WriteFile(dest, []byte(text), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func benchGoSortedScaleKeys(scale map[string]string) []string {
	keys := make([]string, 0, len(scale))
	for key := range scale {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func benchGoReplaceScaleConstant(text, key, value, syntax string) (string, bool) {
	quoted := regexp.QuoteMeta(key)
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?m)^(\s*` + quoted + `\s*:=\s*)[^\r\n]+`),
	}
	if syntax == "lua" {
		patterns = []*regexp.Regexp{
			regexp.MustCompile(`(?m)^(\s*local\s+` + quoted + `\s*=\s*)[^\r\n]+`),
			regexp.MustCompile(`(?m)^(\s*` + quoted + `\s*=\s*)[^\r\n]+`),
		}
	}
	for _, pattern := range patterns {
		if pattern.MatchString(text) {
			return pattern.ReplaceAllString(text, "${1}"+value), true
		}
	}
	return text, false
}

func parseBenchGoTimeout(raw string) (time.Duration, error) {
	if raw == "" {
		return 60 * time.Second, nil
	}
	if duration, err := time.ParseDuration(raw); err == nil {
		return duration, nil
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid --timeout %q", raw)
	}
	return time.Duration(value * float64(time.Second)), nil
}

type benchStringList []string

func (l *benchStringList) String() string { return strings.Join(*l, ",") }
func (l *benchStringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

func runBenchGoSubject(subject, runMode, root, leiaBin, luajitBin string, spec benchdisc.Benchmark, cfg benchGoHarnessConfig) benchGoSubjectResult {
	if cfg.DryRun {
		return benchGoSubjectResult{Subject: subject, Mode: runMode, Status: "dry_run", Repeat: 1, Note: "dry run"}
	}
	modeForCommand := runMode
	if subject == "luajit" {
		modeForCommand = "luajit"
	}
	cmd, err := benchBenchmarkModeCommand(modeForCommand, leiaBin, spec.Leia, luajitBin, spec.LuaJIT)
	if err != nil {
		return benchGoSubjectResult{Subject: subject, Mode: runMode, Status: "error", Repeat: 1, Note: err.Error()}
	}
	if cmd.Unavailable != "" {
		return benchGoSubjectResult{Subject: subject, Mode: runMode, Status: cmd.Unavailable, Repeat: 1, Note: "input unavailable"}
	}
	timeSource := cfg.TimeSource
	if timeSource == "auto" && spec.Group == "concurrency" {
		timeSource = "wall"
	}
	repeat, calibration := calibrateBenchGoRepeat(cmd, cfg, timeSource)
	for i := 0; i < cfg.Warmup; i++ {
		_ = runBenchGoSample(cmd, repeat, cfg, timeSource)
	}
	samples := []benchGoSample{calibration}
	if cfg.Runs > 0 {
		samples = samples[:0]
	}
	for i := 0; i < cfg.Runs; i++ {
		samples = append(samples, runBenchGoSample(cmd, repeat, cfg, timeSource))
	}
	return summarizeBenchGoSubject(subject, runMode, samples, repeat)
}

func calibrateBenchGoRepeat(cmd benchModeCommand, cfg benchGoHarnessConfig, timeSource string) (int, benchGoSample) {
	repeat := 1
	last := benchGoSample{Status: "missing", Repeat: repeat}
	for repeat <= cfg.MaxRepeat {
		last = runBenchGoSample(cmd, repeat, cfg, timeSource)
		if benchGoSampleBigEnough(last, cfg.MinSampleSeconds, cfg.MinWallRepeat) || last.Status == "error" || last.Status == "timeout" || last.Status == "no_time" {
			return repeat, last
		}
		repeat *= 2
	}
	return cfg.MaxRepeat, last
}

func runBenchGoSample(cmd benchModeCommand, repeat int, cfg benchGoHarnessConfig, timeSource string) benchGoSample {
	var scriptTotal float64
	var wallTotal float64
	var badStatus string
	var noTime bool
	var t2Attempted, t2Entered, t2Failed, exitTotal int
	for i := 0; i < repeat; i++ {
		run := benchRunTextCommand(cmd.Args, cfg.Timeout, cmd.Env)
		wallTotal += run.WallSeconds
		if run.Status != "ok" {
			badStatus = run.Status
		}
		seconds := benchParseTime(run.Output)
		if run.Status == "ok" && seconds == nil {
			noTime = true
		}
		if seconds != nil {
			scriptTotal += *seconds
		}
		t2Attempted += benchParseCounter(benchT2AttemptedRE, run.Output)
		t2Entered += benchParseCounter(benchT2EnteredRE, run.Output)
		t2Failed += benchParseCounter(benchT2FailedRE, run.Output)
		exitTotal += benchParseCounter(benchExitTotalRE, run.Output)
	}
	counters := benchGoSample{
		T2Attempted: t2Attempted,
		T2Entered:   t2Entered,
		T2Failed:    t2Failed,
		ExitTotal:   exitTotal,
	}
	if badStatus != "" {
		counters.Status = badStatus
		counters.Repeat = repeat
		counters.WallTotalSeconds = wallTotal
		counters.Note = fmt.Sprintf("%d repeated command(s) include failure", repeat)
		return counters
	}
	if timeSource == "wall" {
		seconds := wallTotal / float64(repeat)
		counters.Status = "ok"
		counters.Seconds = &seconds
		counters.Repeat = repeat
		counters.Source = "wall_hr"
		counters.WallTotalSeconds = wallTotal
		counters.Note = "used command wall time"
		return counters
	}
	if noTime {
		counters.Status = "no_time"
		counters.Repeat = repeat
		counters.WallTotalSeconds = wallTotal
		counters.Note = "no Time: line in command output"
		return counters
	}
	if scriptTotal > cfg.TimerResolution && scriptTotal >= cfg.MinSampleSeconds {
		seconds := scriptTotal / float64(repeat)
		total := scriptTotal
		counters.Status = "ok"
		counters.Seconds = &seconds
		counters.Repeat = repeat
		counters.Source = "script_repeat"
		counters.ScriptTotalSeconds = &total
		counters.WallTotalSeconds = wallTotal
		return counters
	}
	if !cfg.NoWallFallback && wallTotal >= cfg.MinSampleSeconds {
		seconds := wallTotal / float64(repeat)
		total := scriptTotal
		counters.Status = "ok"
		counters.Seconds = &seconds
		counters.Repeat = repeat
		counters.Source = "wall_repeat"
		counters.ScriptTotalSeconds = &total
		counters.WallTotalSeconds = wallTotal
		counters.Note = "script Time below resolution; used command wall time"
		return counters
	}
	total := scriptTotal
	counters.Status = "low_resolution"
	counters.Repeat = repeat
	counters.ScriptTotalSeconds = &total
	counters.WallTotalSeconds = wallTotal
	counters.Note = "script Time below resolution"
	return counters
}

var (
	benchT2AttemptedRE = regexp.MustCompile(`(?m)^\s*Tier 2 attempted:\s*([0-9]+)\b`)
	benchT2EnteredRE   = regexp.MustCompile(`(?m)^\s*Tier 2 entered:\s*([0-9]+)\s+functions\b`)
	benchT2FailedRE    = regexp.MustCompile(`(?m)^\s*Tier 2 failed:\s*([0-9]+)\s+functions\b`)
	benchExitTotalRE   = regexp.MustCompile(`(?m)^\s*total exits:\s*([0-9]+)\b`)
)

func benchGoSampleBigEnough(sample benchGoSample, minSampleSeconds float64, minWallRepeat int) bool {
	if sample.Status != "ok" {
		return false
	}
	if sample.Source == "script_repeat" && sample.ScriptTotalSeconds != nil {
		return *sample.ScriptTotalSeconds >= minSampleSeconds
	}
	if sample.Source == "wall_repeat" || sample.Source == "wall_hr" {
		return sample.WallTotalSeconds >= minSampleSeconds && sample.Repeat >= minWallRepeat
	}
	return false
}

func summarizeBenchGoSubject(subject, runMode string, samples []benchGoSample, repeat int) benchGoSubjectResult {
	values := []float64{}
	type timedSample struct {
		seconds float64
		sample  benchGoSample
	}
	okSamples := []timedSample{}
	sourceSet := map[string]bool{}
	status := "missing"
	for _, sample := range samples {
		if sample.Status != "" {
			status = sample.Status
		}
		if sample.Status == "ok" && sample.Seconds != nil {
			values = append(values, *sample.Seconds)
			okSamples = append(okSamples, timedSample{seconds: *sample.Seconds, sample: sample})
			if sample.Source != "" {
				sourceSet[sample.Source] = true
			}
		}
	}
	if len(values) > 0 {
		status = "ok"
		if len(values) != len(samples) {
			status = "partial"
		}
	}
	sources := make([]string, 0, len(sourceSet))
	for source := range sourceSet {
		sources = append(sources, source)
	}
	sort.Strings(sources)
	counterSource := benchGoSample{}
	if len(okSamples) > 0 {
		sort.Slice(okSamples, func(i, j int) bool {
			return okSamples[i].seconds < okSamples[j].seconds
		})
		counterSource = okSamples[len(okSamples)/2].sample
	} else if len(samples) > 0 {
		counterSource = samples[len(samples)-1]
	}
	return benchGoSubjectResult{
		Subject:     subject,
		Mode:        runMode,
		Status:      status,
		Repeat:      repeat,
		Source:      strings.Join(sources, ","),
		Stats:       benchGoComputeStats(values),
		Samples:     samples,
		T2Attempted: counterSource.T2Attempted,
		T2Entered:   counterSource.T2Entered,
		T2Failed:    counterSource.T2Failed,
		ExitTotal:   counterSource.ExitTotal,
	}
}

func benchGoComputeStats(values []float64) benchGoStats {
	stats := benchGoStats{N: len(values)}
	if len(values) == 0 {
		return stats
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}
	sum := 0.0
	for _, value := range sorted {
		sum += value
	}
	mean := sum / float64(len(sorted))
	stdev := 0.0
	if len(sorted) > 1 {
		var variance float64
		for _, value := range sorted {
			d := value - mean
			variance += d * d
		}
		stdev = math.Sqrt(variance / float64(len(sorted)-1))
	}
	cv := 0.0
	if mean > 0 {
		cv = stdev / mean * 100
	}
	stats.Median = &median
	stats.Mean = &mean
	stats.Min = &sorted[0]
	stats.Max = &sorted[len(sorted)-1]
	stats.Stdev = &stdev
	stats.CVPct = &cv
	return stats
}

func buildBenchGoReport(cfg benchGoHarnessConfig, rows []benchGoBenchmarkResult, durationSeconds float64) benchGoReport {
	results := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{
			"benchmark": row.Benchmark,
			"group":     row.Group,
		}
		if row.Base != "" {
			item["base"] = row.Base
		}
		if len(row.Scale) > 0 {
			item["scale"] = row.Scale
		}
		if cfg.Mode == "strict" {
			item["modes"] = row.Strict
		} else {
			item["modes"] = row.Modes
		}
		results = append(results, item)
	}
	notes := []string{}
	benchmarks := make([]string, 0, len(rows))
	for _, row := range rows {
		benchmarks = append(benchmarks, row.Group+"/"+row.Benchmark)
	}
	generated := time.Now().UTC().Format(time.RFC3339)
	return benchGoReport{
		SchemaVersion:    1,
		Mode:             cfg.Mode,
		Modes:            cfg.Modes,
		Results:          results,
		Timestamp:        generated,
		GeneratedAt:      generated,
		DurationSeconds:  durationSeconds,
		HeadRef:          cfg.HeadRef,
		Groups:           cfg.Groups,
		Benchmarks:       benchmarks,
		Runs:             cfg.Runs,
		Warmup:           cfg.Warmup,
		TimeoutSeconds:   cfg.Timeout.Seconds(),
		MinSampleSeconds: cfg.MinSampleSeconds,
		TimerResolution:  cfg.TimerResolution,
		MaxRepeat:        cfg.MaxRepeat,
		MinWallRepeat:    cfg.MinWallRepeat,
		WallFallback:     !cfg.NoWallFallback,
		TimeSource:       cfg.TimeSource,
		Sort:             cfg.Sort,
		ScaleProfile:     cfg.ScaleProfile,
		Scale:            cfg.Scale,
		StrictThresholds: benchGoStrictThresholds(cfg),
		Platform: map[string]string{
			"go":      runtime.Version(),
			"machine": runtime.GOARCH,
			"system":  runtime.GOOS,
		},
		Notes: notes,
	}
}

func benchGoStrictThresholds(cfg benchGoHarnessConfig) map[string]float64 {
	if cfg.Mode != "strict" {
		return nil
	}
	return map[string]float64{
		"suspicious_vm_speedup":   cfg.SuspiciousVMSpeedup,
		"suspicious_luajit_ratio": cfg.SuspiciousLuaJITRatio,
		"related_confirm_ratio":   cfg.RelatedConfirmRatio,
	}
}

func writeBenchGoJSON(path string, report benchGoReport) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func writeBenchGoMarkdown(path string, cfg benchGoHarnessConfig, rows []benchGoBenchmarkResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(benchGoMarkdown(cfg, rows)), 0o644)
}

func benchGoMarkdown(cfg benchGoHarnessConfig, rows []benchGoBenchmarkResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Leia Bench %s Report\n\n", cfg.Mode)
	if cfg.Mode == "strict" {
		b.WriteString("| Benchmark | Mode | Status | Median | Source | T2 | Exits | Checksum | Note |\n")
		b.WriteString("| --- | --- | --- | ---: | --- | ---: | ---: | --- | --- |\n")
		for _, row := range rows {
			for _, mode := range cfg.Modes {
				subject := row.Strict[mode]
				fmt.Fprintf(&b, "| %s/%s | %s | %s | %s | %s | %s | %d | %s | %s |\n", row.Group, row.Benchmark, mode, subject.Status, benchGoSeconds(subject.Stats.Median), subject.Source, benchGoT2(subject), subject.ExitTotal, "ok", benchGoMarkdownCell(subject.Note))
			}
		}
		return b.String()
	}
	b.WriteString("| Benchmark | Mode | Current | HEAD | LuaJIT | Source | T2 | Exits |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | --- | ---: | ---: |\n")
	for _, row := range rows {
		for _, mode := range cfg.Modes {
			modeRow := row.Modes[mode]
			current := modeRow["current"]
			head := modeRow["head"]
			luajit := modeRow["luajit"]
			fmt.Fprintf(&b, "| %s/%s | %s | %s | %s | %s | %s | %s | %d |\n", row.Group, row.Benchmark, mode, benchGoSeconds(current.Stats.Median), benchGoSeconds(head.Stats.Median), benchGoSeconds(luajit.Stats.Median), current.Source, benchGoT2(current), current.ExitTotal)
		}
	}
	return b.String()
}

func benchGoMarkdownCell(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return strings.ReplaceAll(value, "|", "\\|")
}

func benchGoT2(subject benchGoSubjectResult) string {
	return fmt.Sprintf("%d/%d/%d", subject.T2Attempted, subject.T2Entered, subject.T2Failed)
}

func benchGoSeconds(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.6fs", *value)
}

func appendNote(existing, note string) string {
	if existing == "" {
		return note
	}
	return existing + "; " + note
}

func findExecutable(name string) string {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		candidate := filepath.Join(dir, name)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() && info.Mode()&0o111 != 0 {
			return candidate
		}
	}
	return ""
}
