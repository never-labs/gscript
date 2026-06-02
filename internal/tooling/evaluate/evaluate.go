package evaluate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/llmbridge"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	stdlibinstall "github.com/never-labs/leia/internal/stdlib/install"
	"github.com/never-labs/leia/llm"
)

const SchemaVersion = 1

type Options struct {
	Paths               []string
	Filter              string
	ListOnly            bool
	LLMRecordPath       string
	LLMReplayPath       string
	LLMUpdateGoldenPath string
	LLMProviderFactory  runtime.LLMProviderFactory
}

type Report struct {
	SchemaVersion int             `json:"schema_version"`
	Phase         string          `json:"phase"`
	Status        string          `json:"status"`
	StartedAt     string          `json:"started_at"`
	Runtime       RuntimeInfo     `json:"runtime"`
	Summary       Summary         `json:"summary"`
	LLM           *LLMRun         `json:"llm,omitempty"`
	Inputs        []Input         `json:"inputs"`
	Cases         []Case          `json:"cases"`
	Metrics       []MetricSummary `json:"metrics,omitempty"`
	Comparison    *Comparison     `json:"comparison,omitempty"`
	Findings      []Finding       `json:"findings"`
	Notes         []string        `json:"notes"`
}

type RuntimeInfo struct {
	LeiaVersion string `json:"leia_version"`
	GoVersion   string `json:"go_version"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	Revision    string `json:"revision,omitempty"`
	Modified    string `json:"modified,omitempty"`
	Time        string `json:"time,omitempty"`
}

type LLMRun struct {
	Mode           string `json:"mode"`
	RecordPath     string `json:"record_path,omitempty"`
	ReplayPath     string `json:"replay_path,omitempty"`
	GoldenUpdated  bool   `json:"golden_updated,omitempty"`
	LoadedTurns    int    `json:"loaded_turns,omitempty"`
	RecordedTurns  int    `json:"recorded_turns,omitempty"`
	ReplayedTurns  int    `json:"replayed_turns,omitempty"`
	RemainingTurns int    `json:"remaining_turns,omitempty"`
}

type Summary struct {
	Files          int     `json:"files"`
	ParsedFiles    int     `json:"parsed_files"`
	EvaluateBlocks int     `json:"evaluate_blocks"`
	CasesSelected  int     `json:"cases_selected"`
	CasesPassed    int     `json:"cases_passed"`
	CasesFailed    int     `json:"cases_failed"`
	CasesListed    int     `json:"cases_listed"`
	CasesSkipped   int     `json:"cases_skipped"`
	Assertions     int     `json:"assertions"`
	DurationMS     int64   `json:"duration_ms"`
	PassRate       float64 `json:"pass_rate"`
	Agents         int     `json:"agents"`
	Tools          int     `json:"tools"`
	Models         int     `json:"models"`
	Budgets        int     `json:"budgets"`
	TODOs          int     `json:"todos"`
}

type Input struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type Case struct {
	CaseID      string       `json:"case_id"`
	Name        string       `json:"name"`
	SourcePath  string       `json:"source_path"`
	Range       SourceRange  `json:"range"`
	Status      string       `json:"status"`
	StartedAt   string       `json:"started_at,omitempty"`
	DurationMS  int64        `json:"duration_ms,omitempty"`
	Metrics     []Metric     `json:"metrics,omitempty"`
	Subcases    []Subcase    `json:"subcases,omitempty"`
	Assertions  []Assertion  `json:"assertions,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type SourceRange struct {
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
}

type Assertion struct {
	ID      string      `json:"id"`
	Status  string      `json:"status"`
	Range   SourceRange `json:"range"`
	Message string      `json:"message,omitempty"`
}

type Diagnostic struct {
	Kind     string      `json:"kind"`
	Severity string      `json:"severity"`
	Message  string      `json:"message"`
	Range    SourceRange `json:"range"`
}

type Metric struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Value any    `json:"value"`
}

type MetricSummary struct {
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Count    int            `json:"count"`
	True     int            `json:"true,omitempty"`
	False    int            `json:"false,omitempty"`
	PassRate float64        `json:"pass_rate,omitempty"`
	Mean     float64        `json:"mean,omitempty"`
	Min      float64        `json:"min,omitempty"`
	Max      float64        `json:"max,omitempty"`
	Values   map[string]int `json:"values,omitempty"`
}

type Comparison struct {
	BaselinePath        string             `json:"baseline_path,omitempty"`
	RegressionThreshold float64            `json:"regression_threshold"`
	Summary             *SummaryComparison `json:"summary,omitempty"`
	Metrics             []MetricComparison `json:"metrics,omitempty"`
}

type SummaryComparison struct {
	BaselinePassRate float64 `json:"baseline_pass_rate"`
	CurrentPassRate  float64 `json:"current_pass_rate"`
	DeltaPassRate    float64 `json:"delta_pass_rate"`
	Regressed        bool    `json:"regressed,omitempty"`
}

type MetricComparison struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Baseline      float64 `json:"baseline,omitempty"`
	Current       float64 `json:"current,omitempty"`
	Delta         float64 `json:"delta,omitempty"`
	BaselineCount int     `json:"baseline_count,omitempty"`
	CurrentCount  int     `json:"current_count,omitempty"`
	Regressed     bool    `json:"regressed,omitempty"`
}

type Subcase struct {
	CaseID      string       `json:"case_id"`
	Status      string       `json:"status"`
	StartedAt   string       `json:"started_at,omitempty"`
	DurationMS  int64        `json:"duration_ms,omitempty"`
	Metrics     []Metric     `json:"metrics,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

type Finding struct {
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	Path     string         `json:"path,omitempty"`
	Line     int            `json:"line,omitempty"`
	Column   int            `json:"column,omitempty"`
	Details  map[string]any `json:"details,omitempty"`
}

type parsedCase struct {
	Case
	Body *ast.BlockStmt
}

type caseEvalData struct {
	Metrics  []Metric
	Subcases []Subcase
}

func (d caseEvalData) Failed() bool {
	for _, subcase := range d.Subcases {
		if subcase.Status == "failed" {
			return true
		}
	}
	return false
}

func Run(opts Options) (Report, error) {
	run, err := newRunContext(opts)
	if err != nil {
		return Report{}, err
	}
	paths := opts.Paths
	if len(paths) == 0 {
		paths = []string{"."}
	}
	files, err := collectFiles(paths)
	if err != nil {
		return Report{}, err
	}
	report := Report{
		SchemaVersion: SchemaVersion,
		Phase:         "runtime-minimal",
		Status:        "ok",
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Runtime:       runtimeInfo(),
		Inputs:        []Input{},
		Cases:         []Case{},
		Findings:      []Finding{},
		Notes: []string{
			"evaluate runs each evaluate block body as ordinary Leia code; provider scoring and workflow orchestration are reserved for later phases.",
		},
	}
	report.LLM = run.report
	if opts.LLMReplayPath != "" {
		report.Notes = append(report.Notes, fmt.Sprintf("llm replay loaded from %s", opts.LLMReplayPath))
	}
	if run != nil && run.recordPath != "" {
		report.Notes = append(report.Notes, fmt.Sprintf("llm turns will be recorded to %s", run.recordPath))
	}
	filter := strings.TrimSpace(opts.Filter)
	if filter != "" {
		report.Notes = append(report.Notes, fmt.Sprintf("filter: %s", filter))
	}
	if opts.ListOnly {
		report.Notes = append(report.Notes, "list mode: evaluate cases are discovered but not executed")
	}
	for _, file := range files {
		input := Input{Path: file, Status: "ok"}
		report.Summary.Files++
		src, err := os.ReadFile(file)
		if err != nil {
			input.Status = "error"
			report.Status = "failed"
			report.Findings = append(report.Findings, Finding{Kind: "io_error", Severity: "error", Message: err.Error(), Path: file})
			report.Inputs = append(report.Inputs, input)
			continue
		}
		report.Findings = append(report.Findings, todoFindings(file, src)...)
		if strings.HasSuffix(file, ".leia") {
			counts, prog, cases, findings := parseLeia(file, src)
			report.Summary.ParsedFiles += counts.ParsedFiles
			report.Summary.EvaluateBlocks += counts.EvaluateBlocks
			report.Summary.Agents += counts.Agents
			report.Summary.Tools += counts.Tools
			report.Summary.Models += counts.Models
			report.Summary.Budgets += counts.Budgets
			if len(findings) > 0 {
				input.Status = "error"
				report.Status = "failed"
				report.Findings = append(report.Findings, findings...)
			} else {
				for _, parsed := range cases {
					if filter != "" && !caseMatchesFilter(parsed.Case, filter) {
						report.Summary.CasesSkipped++
						continue
					}
					c := parsed.Case
					c.Assertions = collectAssertions(parsed.Body)
					report.Summary.CasesSelected++
					if opts.ListOnly {
						c.Status = "listed"
						report.Cases = append(report.Cases, c)
						continue
					}
					start := time.Now()
					c.StartedAt = start.UTC().Format(time.RFC3339)
					caseData, err := executeCase(file, prog, parsed, run)
					c.Metrics = caseData.Metrics
					c.Subcases = caseData.Subcases
					if err != nil {
						c.Status = "failed"
						c.DurationMS = elapsedMillis(start)
						markAssertions(c.Assertions, "unknown")
						markFailedAssertion(c.Assertions, err.Error())
						c.Diagnostics = append(c.Diagnostics, Diagnostic{
							Kind:     "runtime_error",
							Severity: "error",
							Message:  err.Error(),
							Range:    c.Range,
						})
						input.Status = "error"
						report.Status = "failed"
						report.Findings = append(report.Findings, Finding{
							Kind:     "case_runtime_error",
							Severity: "error",
							Message:  err.Error(),
							Path:     file,
							Line:     c.Range.StartLine,
							Column:   c.Range.StartColumn,
						})
					} else if caseData.Failed() {
						c.Status = "failed"
						c.DurationMS = elapsedMillis(start)
						markAssertions(c.Assertions, "passed")
						input.Status = "error"
						report.Status = "failed"
						report.Findings = append(report.Findings, Finding{
							Kind:     "eval_subcase_failure",
							Severity: "error",
							Message:  "one or more eval.case subcases failed",
							Path:     file,
							Line:     c.Range.StartLine,
							Column:   c.Range.StartColumn,
						})
					} else {
						c.Status = "passed"
						c.DurationMS = elapsedMillis(start)
						markAssertions(c.Assertions, "passed")
					}
					report.Cases = append(report.Cases, c)
				}
			}
		}
		report.Inputs = append(report.Inputs, input)
	}
	for _, finding := range report.Findings {
		if finding.Kind == "todo" {
			report.Summary.TODOs++
		}
	}
	if run != nil && run.replayProvider != nil {
		if run.report != nil {
			run.report.ReplayedTurns = run.replayProvider.Consumed()
			run.report.RemainingTurns = run.replayProvider.Remaining()
		}
		for _, finding := range run.replayMonitor.Findings() {
			report.Status = "failed"
			report.Findings = append(report.Findings, finding)
		}
		if remaining := run.replayProvider.Remaining(); remaining > 0 {
			report.Status = "failed"
			report.Findings = append(report.Findings, Finding{
				Kind:     "llm_replay_unconsumed",
				Severity: "error",
				Message:  fmt.Sprintf("llm replay left %d unconsumed turn(s)", remaining),
				Path:     opts.LLMReplayPath,
				Details:  map[string]any{"remaining_turns": remaining},
			})
		}
	}
	if run != nil && run.recorder != nil {
		records := run.recorder.Records()
		if run.report != nil {
			run.report.RecordedTurns = len(records)
		}
		if err := llm.SaveRecords(run.recordPath, records); err != nil {
			return report, err
		}
	}
	finalizeSummary(&report)
	return report, nil
}

func finalizeSummary(report *Report) {
	var executable int
	for _, c := range report.Cases {
		report.Summary.Assertions += len(c.Assertions)
		report.Summary.DurationMS += c.DurationMS
		switch c.Status {
		case "passed":
			report.Summary.CasesPassed++
			executable++
		case "failed":
			report.Summary.CasesFailed++
			executable++
		case "listed":
			report.Summary.CasesListed++
		}
	}
	if executable > 0 {
		report.Summary.PassRate = float64(report.Summary.CasesPassed) / float64(executable)
	}
	report.Metrics = aggregateMetricSummaries(report.Cases)
}

func AttachBaselineComparison(current *Report, baseline Report, baselinePath string, threshold float64) {
	if current == nil {
		return
	}
	comparison := CompareReports(*current, baseline, baselinePath, threshold)
	current.Comparison = &comparison
	if comparison.Summary != nil && comparison.Summary.Regressed {
		current.Status = "failed"
		current.Findings = append(current.Findings, Finding{
			Kind:     "evaluate_regression",
			Severity: "error",
			Message:  fmt.Sprintf("summary pass_rate regressed by %.4g below threshold %.4g", -comparison.Summary.DeltaPassRate, threshold),
			Path:     baselinePath,
			Details: map[string]any{
				"baseline_pass_rate": comparison.Summary.BaselinePassRate,
				"current_pass_rate":  comparison.Summary.CurrentPassRate,
				"delta_pass_rate":    comparison.Summary.DeltaPassRate,
				"threshold":          threshold,
			},
		})
	}
	for _, metric := range comparison.Metrics {
		if !metric.Regressed {
			continue
		}
		current.Status = "failed"
		current.Findings = append(current.Findings, Finding{
			Kind:     "evaluate_metric_regression",
			Severity: "error",
			Message:  fmt.Sprintf("metric %q pass_rate regressed by %.4g below threshold %.4g", metric.Name, -metric.Delta, threshold),
			Path:     baselinePath,
			Details: map[string]any{
				"name":      metric.Name,
				"type":      metric.Type,
				"baseline":  metric.Baseline,
				"current":   metric.Current,
				"delta":     metric.Delta,
				"threshold": threshold,
			},
		})
	}
}

func CompareReports(current, baseline Report, baselinePath string, threshold float64) Comparison {
	comparison := Comparison{
		BaselinePath:        baselinePath,
		RegressionThreshold: threshold,
		Summary: &SummaryComparison{
			BaselinePassRate: baseline.Summary.PassRate,
			CurrentPassRate:  current.Summary.PassRate,
			DeltaPassRate:    current.Summary.PassRate - baseline.Summary.PassRate,
		},
	}
	if comparison.Summary.DeltaPassRate < -threshold {
		comparison.Summary.Regressed = true
	}
	comparison.Metrics = compareMetricSummaries(current.Metrics, baseline.Metrics, threshold)
	return comparison
}

func compareMetricSummaries(current, baseline []MetricSummary, threshold float64) []MetricComparison {
	currentByKey := map[string]MetricSummary{}
	baselineByKey := map[string]MetricSummary{}
	keys := map[string]bool{}
	for _, metric := range current {
		key := metric.Name + "\x00" + metric.Type
		currentByKey[key] = metric
		keys[key] = true
	}
	for _, metric := range baseline {
		key := metric.Name + "\x00" + metric.Type
		baselineByKey[key] = metric
		keys[key] = true
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	out := make([]MetricComparison, 0, len(ordered))
	for _, key := range ordered {
		cur := currentByKey[key]
		base := baselineByKey[key]
		name, typ := splitMetricKey(key)
		item := MetricComparison{
			Name:          name,
			Type:          typ,
			BaselineCount: base.Count,
			CurrentCount:  cur.Count,
		}
		switch typ {
		case "bool":
			item.Baseline = base.PassRate
			item.Current = cur.PassRate
			item.Delta = item.Current - item.Baseline
			item.Regressed = item.Delta < -threshold
		case "number":
			item.Baseline = base.Mean
			item.Current = cur.Mean
			item.Delta = item.Current - item.Baseline
		}
		out = append(out, item)
	}
	return out
}

func splitMetricKey(key string) (string, string) {
	parts := strings.SplitN(key, "\x00", 2)
	if len(parts) != 2 {
		return key, ""
	}
	return parts[0], parts[1]
}

type metricAccumulator struct {
	name       string
	typ        string
	count      int
	trueCount  int
	falseCount int
	sum        float64
	min        float64
	max        float64
	values     map[string]int
}

func aggregateMetricSummaries(cases []Case) []MetricSummary {
	accs := map[string]*metricAccumulator{}
	for _, c := range cases {
		for _, metric := range c.Metrics {
			addMetric(accs, metric)
		}
		for _, subcase := range c.Subcases {
			if subcase.Status == "skipped" {
				continue
			}
			for _, metric := range subcase.Metrics {
				addMetric(accs, metric)
			}
		}
	}
	names := make([]string, 0, len(accs))
	for name := range accs {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]MetricSummary, 0, len(names))
	for _, name := range names {
		acc := accs[name]
		summary := MetricSummary{
			Name:  acc.name,
			Type:  acc.typ,
			Count: acc.count,
		}
		switch acc.typ {
		case "bool":
			summary.True = acc.trueCount
			summary.False = acc.falseCount
			if acc.count > 0 {
				summary.PassRate = float64(acc.trueCount) / float64(acc.count)
			}
		case "number":
			if acc.count > 0 {
				summary.Mean = acc.sum / float64(acc.count)
				summary.Min = acc.min
				summary.Max = acc.max
			}
		case "string":
			summary.Values = acc.values
		}
		out = append(out, summary)
	}
	return out
}

func addMetric(accs map[string]*metricAccumulator, metric Metric) {
	if metric.Name == "" || metric.Type == "nil" {
		return
	}
	key := metric.Name + "\x00" + metric.Type
	acc := accs[key]
	if acc == nil {
		acc = &metricAccumulator{name: metric.Name, typ: metric.Type}
		if metric.Type == "string" {
			acc.values = map[string]int{}
		}
		accs[key] = acc
	}
	switch metric.Type {
	case "bool":
		v, ok := metric.Value.(bool)
		if !ok {
			return
		}
		acc.count++
		if v {
			acc.trueCount++
		} else {
			acc.falseCount++
		}
	case "number":
		v, ok := metricNumber(metric.Value)
		if !ok {
			return
		}
		if acc.count == 0 || v < acc.min {
			acc.min = v
		}
		if acc.count == 0 || v > acc.max {
			acc.max = v
		}
		acc.count++
		acc.sum += v
	case "string":
		v, ok := metric.Value.(string)
		if !ok {
			return
		}
		acc.count++
		acc.values[v]++
	}
}

func metricNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func runtimeInfo() RuntimeInfo {
	info := RuntimeInfo{
		LeiaVersion: "dev",
		GoVersion:   goruntime.Version(),
		GOOS:        goruntime.GOOS,
		GOARCH:      goruntime.GOARCH,
	}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				info.Revision = setting.Value
			case "vcs.modified":
				info.Modified = setting.Value
			case "vcs.time":
				info.Time = setting.Value
			}
		}
	}
	return info
}

func caseMatchesFilter(c Case, filter string) bool {
	if filter == "" {
		return true
	}
	return strings.Contains(c.Name, filter) ||
		strings.Contains(c.CaseID, filter) ||
		strings.Contains(c.SourcePath, filter)
}

type runContext struct {
	recorder        *llm.Recorder
	recordPath      string
	replayProvider  *llm.ReplayProvider
	replayMonitor   *replayMonitor
	providerFactory runtime.LLMProviderFactory
	report          *LLMRun
}

func newRunContext(opts Options) (*runContext, error) {
	modes := 0
	if opts.LLMRecordPath != "" {
		modes++
	}
	if opts.LLMReplayPath != "" {
		modes++
	}
	if opts.LLMUpdateGoldenPath != "" {
		modes++
	}
	if modes > 1 {
		return nil, fmt.Errorf("llm record, replay, and update-golden modes are mutually exclusive")
	}
	run := &runContext{providerFactory: opts.LLMProviderFactory}
	if opts.LLMReplayPath != "" {
		records, err := llm.LoadRecords(opts.LLMReplayPath)
		if err != nil {
			return nil, err
		}
		run.replayProvider = llm.NewReplayProvider(records)
		run.replayMonitor = &replayMonitor{provider: run.replayProvider}
		run.report = &LLMRun{Mode: "replay", ReplayPath: opts.LLMReplayPath, LoadedTurns: len(records)}
	}
	if opts.LLMRecordPath != "" {
		run.recorder = llm.NewRecorder()
		run.recordPath = opts.LLMRecordPath
		run.report = &LLMRun{Mode: "record", RecordPath: opts.LLMRecordPath}
	}
	if opts.LLMUpdateGoldenPath != "" {
		run.recorder = llm.NewRecorder()
		run.recordPath = opts.LLMUpdateGoldenPath
		run.report = &LLMRun{Mode: "update_golden", RecordPath: opts.LLMUpdateGoldenPath, GoldenUpdated: true}
	}
	return run, nil
}

func collectFiles(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	for _, path := range paths {
		if path == "" {
			continue
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if includeFile(path) && !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
			continue
		}
		err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				switch d.Name() {
				case ".git", "vendor", "node_modules":
					return filepath.SkipDir
				}
				return nil
			}
			if includeFile(path) && !seen[path] {
				seen[path] = true
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func includeFile(path string) bool {
	switch filepath.Ext(path) {
	case ".leia", ".md", ".go":
		return true
	default:
		return false
	}
}

func todoFindings(path string, src []byte) []Finding {
	var out []Finding
	scanner := bufio.NewScanner(bytes.NewReader(src))
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if !todoLineMayContainMarker(path, text) {
			continue
		}
		idx := todoMarkerIndex(text)
		if idx < 0 {
			continue
		}
		msg := strings.TrimSpace(text[idx:])
		if msg == "" {
			msg = "TODO"
		}
		out = append(out, Finding{
			Kind:     "todo",
			Severity: "info",
			Message:  msg,
			Path:     path,
			Line:     line,
			Column:   idx + 1,
		})
	}
	return out
}

func todoLineMayContainMarker(path, text string) bool {
	switch filepath.Ext(path) {
	case ".go", ".leia":
		trimmed := strings.TrimSpace(text)
		return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
	default:
		return true
	}
}

func todoMarkerIndex(text string) int {
	for _, marker := range []string{"TODO:", "TODO "} {
		if idx := strings.Index(text, marker); idx >= 0 {
			return idx
		}
	}
	if strings.TrimSpace(text) == "TODO" {
		return strings.Index(text, "TODO")
	}
	return -1
}

func parseLeia(path string, src []byte) (Summary, *ast.Program, []parsedCase, []Finding) {
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return Summary{}, nil, nil, []Finding{{Kind: "lex_error", Severity: "error", Message: err.Error(), Path: path}}
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return Summary{}, nil, nil, []Finding{{Kind: "parse_error", Severity: "error", Message: err.Error(), Path: path}}
	}
	counts := Summary{ParsedFiles: 1}
	var cases []parsedCase
	countLLMStmts(path, prog.Stmts, &counts, &cases)
	if err := ast.ValidateLLM(prog); err != nil {
		return counts, prog, cases, []Finding{{Kind: "ai_syntax_error", Severity: "error", Message: err.Error(), Path: path}}
	}
	return counts, prog, cases, nil
}

func countLLMStmts(path string, stmts []ast.Stmt, counts *Summary, cases *[]parsedCase) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AgentDeclStmt:
			counts.Agents++
			if s.Flow != nil {
				countLLMStmts(path, s.Flow.Stmts, counts, cases)
			}
		case *ast.ToolDeclStmt:
			counts.Tools++
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.ModelsDeclStmt:
			counts.Models++
		case *ast.BudgetStmt:
			counts.Budgets++
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.EvaluateBlockStmt:
			counts.EvaluateBlocks++
			*cases = append(*cases, parsedCase{
				Case: Case{
					CaseID:     fmt.Sprintf("%s:%d:%d", path, s.P.Line, s.P.Column),
					Name:       s.Name,
					SourcePath: path,
					Range: SourceRange{
						StartLine:   s.P.Line,
						StartColumn: s.P.Column,
					},
					Status: "pending",
				},
				Body: s.Body,
			})
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.BlockStmt:
			countLLMStmts(path, s.Stmts, counts, cases)
		case *ast.FuncDeclStmt:
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.IfStmt:
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
			for _, elseif := range s.ElseIfs {
				if elseif.Body != nil {
					countLLMStmts(path, elseif.Body.Stmts, counts, cases)
				}
			}
			if s.ElseBody != nil {
				countLLMStmts(path, s.ElseBody.Stmts, counts, cases)
			}
		case *ast.ForStmt:
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.ForNumStmt:
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.ForRangeStmt:
			if s.Body != nil {
				countLLMStmts(path, s.Body.Stmts, counts, cases)
			}
		case *ast.SelectStmt:
			for _, c := range s.Cases {
				if c.Body != nil {
					countLLMStmts(path, c.Body.Stmts, counts, cases)
				}
			}
			if s.Default != nil {
				countLLMStmts(path, s.Default.Stmts, counts, cases)
			}
		}
	}
}

func executeCase(path string, prog *ast.Program, c parsedCase, run *runContext) (caseEvalData, error) {
	if prog == nil || c.Body == nil {
		return caseEvalData{}, nil
	}
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)
	baseDir := ""
	if abs, err := filepath.Abs(path); err == nil {
		baseDir = filepath.Dir(abs)
	}
	evalState := newEvalCollector(interp.CallFunction, baseDir)
	evalModule := runtime.TableValue(evalState.BuildModule())
	interp.SetGlobal("eval", evalModule)
	interp.SetModule("eval", evalModule)
	if run != nil {
		if run.replayProvider != nil {
			interp.SetLLMProvider(llmbridge.ProviderAdapter(run.replayMonitor))
		}
		if run.recorder != nil {
			interp.SetLLMProviderFactory(recordingProviderFactory(run.providerFactory, run.recorder.Record))
		} else if run.providerFactory != nil {
			interp.SetLLMProviderFactory(run.providerFactory)
		}
	}
	if baseDir != "" {
		interp.SetScriptDir(baseDir)
	}
	interp.SetArgs(path, nil)
	err := interp.Exec(&ast.Program{
		Stmts:          caseProgramStmts(prog.Stmts, c.Body.Stmts),
		FileDirectives: append([]ast.FileDirective(nil), prog.FileDirectives...),
	})
	return evalState.Data(), err
}

type replayMonitor struct {
	provider *llm.ReplayProvider
	mu       sync.Mutex
	errors   []error
}

func (m *replayMonitor) Turn(ctx context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	res, err := m.provider.Turn(ctx, req)
	if err != nil {
		var mismatch *llm.ReplayMismatchError
		var exhausted *llm.ReplayExhaustedError
		if errors.As(err, &mismatch) || errors.As(err, &exhausted) {
			m.mu.Lock()
			m.errors = append(m.errors, err)
			m.mu.Unlock()
		}
	}
	return res, err
}

func (m *replayMonitor) Findings() []Finding {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Finding
	for _, err := range m.errors {
		kind := "llm_replay_error"
		details := map[string]any{}
		var mismatch *llm.ReplayMismatchError
		var exhausted *llm.ReplayExhaustedError
		if errors.As(err, &mismatch) {
			kind = "llm_replay_mismatch"
			details["turn"] = mismatch.Turn
			details["expected"] = replayRequestDetails(mismatch.Expected)
			details["actual"] = replayRequestDetails(mismatch.Actual)
		} else if errors.As(err, &exhausted) {
			kind = "llm_replay_exhausted"
			details["turn"] = exhausted.Turn
		}
		out = append(out, Finding{
			Kind:     kind,
			Severity: "error",
			Message:  err.Error(),
			Details:  details,
		})
	}
	return out
}

func replayRequestDetails(req llm.TurnRequest) map[string]any {
	out := map[string]any{}
	if req.Model != "" {
		out["model"] = req.Model
	}
	if len(req.Messages) > 0 {
		messages := make([]map[string]any, 0, len(req.Messages))
		for _, msg := range req.Messages {
			messages = append(messages, replayMessageDetails(msg))
		}
		out["messages"] = messages
	}
	if len(req.Tools) > 0 {
		tools := make([]map[string]any, 0, len(req.Tools))
		for _, tool := range req.Tools {
			item := map[string]any{"name": tool.Name}
			if tool.Description != "" {
				item["description"] = tool.Description
			}
			if len(tool.Params) > 0 {
				item["params"] = append([]string(nil), tool.Params...)
			}
			if len(tool.Requires) > 0 {
				item["requires"] = append([]string(nil), tool.Requires...)
			}
			if tool.Schema != nil {
				item["schema"] = tool.Schema
			}
			tools = append(tools, item)
		}
		out["tools"] = tools
	}
	if req.ForceTool != "" {
		out["force_tool"] = req.ForceTool
	}
	if req.MaxTokens != 0 {
		out["max_tokens"] = req.MaxTokens
	}
	if req.Temperature != nil {
		out["temperature"] = *req.Temperature
	}
	if req.TopP != nil {
		out["top_p"] = *req.TopP
	}
	if req.ResponseFormat != nil {
		out["response_format"] = req.ResponseFormat
	}
	if req.Stream {
		out["stream"] = true
	}
	if len(req.Stop) > 0 {
		out["stop"] = append([]string(nil), req.Stop...)
	}
	if len(req.Metadata) > 0 {
		metadata := map[string]string{}
		for key, value := range req.Metadata {
			metadata[key] = value
		}
		out["metadata"] = metadata
	}
	return out
}

func replayMessageDetails(msg llm.Message) map[string]any {
	out := map[string]any{"role": msg.Role}
	if msg.Text != "" {
		out["text"] = msg.Text
	}
	if msg.ToolUseID != "" {
		out["tool_use_id"] = msg.ToolUseID
	}
	if msg.Value != nil {
		out["value"] = msg.Value
	}
	if msg.Error != "" {
		out["error"] = msg.Error
	}
	if msg.ToolCall != nil {
		toolCall := map[string]any{
			"id":   msg.ToolCall.ID,
			"tool": msg.ToolCall.Tool,
		}
		if len(msg.ToolCall.Args) > 0 {
			toolCall["args"] = msg.ToolCall.Args
		}
		out["tool_call"] = toolCall
	}
	return out
}

type recordingProvider struct {
	provider runtime.LLMProvider
	sink     llm.RecordSink
}

func recordingProviderFactory(factory runtime.LLMProviderFactory, sink llm.RecordSink) runtime.LLMProviderFactory {
	if factory == nil {
		return nil
	}
	return func(cfg runtime.LLMProviderConfig) (runtime.LLMProvider, error) {
		provider, err := factory(cfg)
		if err != nil || provider == nil || sink == nil {
			return provider, err
		}
		return recordingProvider{provider: provider, sink: sink}, nil
	}
}

func (p recordingProvider) Turn(ctx context.Context, req runtime.LLMTurnRequest) (runtime.LLMTurnResult, error) {
	res, err := p.provider.Turn(ctx, req)
	if p.sink != nil {
		record := llm.Record{
			Request: llmbridge.PublicTurnRequest(req),
			Result:  llmbridge.PublicTurnResult(res),
		}
		if err != nil {
			record.Error = err.Error()
		}
		p.sink(record)
	}
	return res, err
}

func (p recordingProvider) StreamTurn(ctx context.Context, req runtime.LLMTurnRequest, sink runtime.LLMStreamSink) (runtime.LLMTurnResult, error) {
	streaming, ok := p.provider.(runtime.LLMStreamingProvider)
	if !ok {
		return p.Turn(ctx, req)
	}
	res, err := streaming.StreamTurn(ctx, req, sink)
	if p.sink != nil {
		record := llm.Record{
			Request: llmbridge.PublicTurnRequest(req),
			Result:  llmbridge.PublicTurnResult(res),
		}
		if err != nil {
			record.Error = err.Error()
		}
		p.sink(record)
	}
	return res, err
}

func elapsedMillis(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func caseProgramStmts(topLevel []ast.Stmt, body []ast.Stmt) []ast.Stmt {
	stmts := make([]ast.Stmt, 0, len(topLevel)+len(body))
	for _, stmt := range topLevel {
		if _, ok := stmt.(*ast.EvaluateBlockStmt); ok {
			continue
		}
		stmts = append(stmts, stmt)
	}
	stmts = append(stmts, body...)
	return stmts
}

func collectAssertions(body *ast.BlockStmt) []Assertion {
	var out []Assertion
	collectAssertionsInBlock(body, &out)
	return out
}

func collectAssertionsInBlock(body *ast.BlockStmt, out *[]Assertion) {
	if body == nil {
		return
	}
	for _, stmt := range body.Stmts {
		collectAssertionsInStmt(stmt, out)
	}
}

func collectAssertionsInStmt(stmt ast.Stmt, out *[]Assertion) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		collectAssertionsInExprs(s.Targets, out)
		collectAssertionsInExprs(s.Values, out)
	case *ast.DeclareStmt:
		collectAssertionsInExprs(s.Values, out)
	case *ast.CompoundAssignStmt:
		collectAssertionsInExpr(s.Target, out)
		collectAssertionsInExpr(s.Value, out)
	case *ast.IncDecStmt:
		collectAssertionsInExpr(s.Target, out)
	case *ast.CallStmt:
		collectAssertionsInExpr(s.Call, out)
	case *ast.GoStmt:
		collectAssertionsInExpr(s.Call, out)
	case *ast.DeferStmt:
		collectAssertionsInExpr(s.Call, out)
	case *ast.SendStmt:
		collectAssertionsInExpr(s.Channel, out)
		collectAssertionsInExpr(s.Value, out)
	case *ast.SelectStmt:
		for _, c := range s.Cases {
			collectAssertionsInExpr(c.Channel, out)
			collectAssertionsInExpr(c.SendValue, out)
			collectAssertionsInBlock(c.Body, out)
		}
		collectAssertionsInBlock(s.Default, out)
	case *ast.IfStmt:
		collectAssertionsInExpr(s.Cond, out)
		collectAssertionsInBlock(s.Body, out)
		for _, elseif := range s.ElseIfs {
			collectAssertionsInExpr(elseif.Cond, out)
			collectAssertionsInBlock(elseif.Body, out)
		}
		collectAssertionsInBlock(s.ElseBody, out)
	case *ast.ForNumStmt:
		collectAssertionsInStmt(s.Init, out)
		collectAssertionsInExpr(s.Cond, out)
		collectAssertionsInStmt(s.Post, out)
		collectAssertionsInBlock(s.Body, out)
	case *ast.ForRangeStmt:
		collectAssertionsInExpr(s.Iter, out)
		collectAssertionsInBlock(s.Body, out)
	case *ast.ForStmt:
		collectAssertionsInExpr(s.Cond, out)
		collectAssertionsInBlock(s.Body, out)
	case *ast.ReturnStmt:
		collectAssertionsInExprs(s.Values, out)
	case *ast.FuncDeclStmt:
		collectAssertionsInBlock(s.Body, out)
	case *ast.ToolDeclStmt:
		collectAssertionsInBlock(s.Body, out)
	case *ast.AgentDeclStmt:
		collectAssertionsInConfig(s.Config, out)
		collectAssertionsInBlock(s.Flow, out)
	case *ast.AgentDefaultsDeclStmt:
		collectAssertionsInConfig(s.Config, out)
	case *ast.ModelsDeclStmt:
		collectAssertionsInConfig(s.Config, out)
	case *ast.BudgetStmt:
		collectAssertionsInConfig(s.Config, out)
		collectAssertionsInBlock(s.Body, out)
	case *ast.EvaluateBlockStmt:
		collectAssertionsInBlock(s.Body, out)
	}
}

func collectAssertionsInConfig(fields []ast.ConfigField, out *[]Assertion) {
	for _, field := range fields {
		collectAssertionsInExpr(field.Key, out)
		collectAssertionsInExpr(field.Value, out)
	}
}

func collectAssertionsInExprs(exprs []ast.Expr, out *[]Assertion) {
	for _, expr := range exprs {
		collectAssertionsInExpr(expr, out)
	}
}

func collectAssertionsInExpr(expr ast.Expr, out *[]Assertion) {
	switch e := expr.(type) {
	case nil:
		return
	case *ast.BinaryExpr:
		collectAssertionsInExpr(e.Left, out)
		collectAssertionsInExpr(e.Right, out)
	case *ast.UnaryExpr:
		collectAssertionsInExpr(e.Operand, out)
	case *ast.ParenExpr:
		collectAssertionsInExpr(e.Inner, out)
	case *ast.IndexExpr:
		collectAssertionsInExpr(e.Table, out)
		collectAssertionsInExpr(e.Index, out)
	case *ast.FieldExpr:
		collectAssertionsInExpr(e.Table, out)
	case *ast.CallExpr:
		if isAssertCall(e) {
			*out = append(*out, Assertion{
				ID: fmt.Sprintf("assert:%d:%d", e.P.Line, e.P.Column),
				Range: SourceRange{
					StartLine:   e.P.Line,
					StartColumn: e.P.Column,
				},
				Status: "pending",
			})
		}
		collectAssertionsInExpr(e.Func, out)
		collectAssertionsInExprs(e.Args, out)
	case *ast.MethodCallExpr:
		collectAssertionsInExpr(e.Object, out)
		collectAssertionsInExprs(e.Args, out)
	case *ast.FuncLitExpr:
		collectAssertionsInBlock(e.Body, out)
	case *ast.AgentLitExpr:
		collectAssertionsInConfig(e.Config, out)
		collectAssertionsInBlock(e.Flow, out)
	case *ast.TurnExpr:
		collectAssertionsInConfig(e.Config, out)
	case *ast.MessagesExpr:
		collectAssertionsInTableFields(e.Fields, out)
	case *ast.ListLitExpr:
		collectAssertionsInExprs(e.Values, out)
	case *ast.TableLitExpr:
		collectAssertionsInTableFields(e.Fields, out)
	case *ast.DenseLitExpr:
		collectAssertionsInExprs(e.Values, out)
	case *ast.RecvExpr:
		collectAssertionsInExpr(e.Channel, out)
	case *ast.MakeChanExpr:
		collectAssertionsInExpr(e.Size, out)
	}
}

func collectAssertionsInTableFields(fields []ast.TableField, out *[]Assertion) {
	for _, field := range fields {
		collectAssertionsInExpr(field.Key, out)
		collectAssertionsInExpr(field.Value, out)
	}
}

func isAssertCall(call *ast.CallExpr) bool {
	ident, ok := call.Func.(*ast.IdentExpr)
	return ok && ident.Name == "assert"
}

func markAssertions(assertions []Assertion, status string) {
	for i := range assertions {
		assertions[i].Status = status
	}
}

func markFailedAssertion(assertions []Assertion, message string) {
	if len(assertions) == 0 {
		return
	}
	line, column, ok := leadingSourcePosition(message)
	if ok {
		for i := range assertions {
			if assertions[i].Range.StartLine == line && assertions[i].Range.StartColumn == column {
				assertions[i].Status = "failed"
				return
			}
		}
	}
	if len(assertions) == 1 || strings.Contains(message, "assert") {
		assertions[0].Status = "failed"
	}
}

func leadingSourcePosition(message string) (line, column int, ok bool) {
	if _, err := fmt.Sscanf(message, "%d:%d:", &line, &column); err == nil {
		return line, column, true
	}
	return 0, 0, false
}

func FormatText(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "evaluate: %s (%d files, %d parsed, %d selected/%d discovered cases, %d passed, %d failed, %d listed, %.2f pass rate, %dms, %d assertions, %d agents, %d tools, %d todos)\n",
		report.Status,
		report.Summary.Files,
		report.Summary.ParsedFiles,
		report.Summary.CasesSelected,
		report.Summary.EvaluateBlocks,
		report.Summary.CasesPassed,
		report.Summary.CasesFailed,
		report.Summary.CasesListed,
		report.Summary.PassRate,
		report.Summary.DurationMS,
		report.Summary.Assertions,
		report.Summary.Agents,
		report.Summary.Tools,
		report.Summary.TODOs,
	)
	if len(report.Metrics) > 0 {
		fmt.Fprintf(&b, "metrics:\n")
		for _, metric := range report.Metrics {
			switch metric.Type {
			case "bool":
				fmt.Fprintf(&b, "  %s bool pass_rate=%.2f true=%d false=%d count=%d\n", metric.Name, metric.PassRate, metric.True, metric.False, metric.Count)
			case "number":
				fmt.Fprintf(&b, "  %s number mean=%.4g min=%.4g max=%.4g count=%d\n", metric.Name, metric.Mean, metric.Min, metric.Max, metric.Count)
			case "string":
				fmt.Fprintf(&b, "  %s string count=%d values=%s\n", metric.Name, metric.Count, formatMetricValues(metric.Values))
			default:
				fmt.Fprintf(&b, "  %s %s count=%d\n", metric.Name, metric.Type, metric.Count)
			}
		}
	}
	if report.Comparison != nil {
		fmt.Fprintf(&b, "comparison: baseline=%s threshold=%.4g\n", report.Comparison.BaselinePath, report.Comparison.RegressionThreshold)
		if report.Comparison.Summary != nil {
			mark := "ok"
			if report.Comparison.Summary.Regressed {
				mark = "regressed"
			}
			fmt.Fprintf(&b, "  summary pass_rate %.4g -> %.4g (delta %.4g, %s)\n",
				report.Comparison.Summary.BaselinePassRate,
				report.Comparison.Summary.CurrentPassRate,
				report.Comparison.Summary.DeltaPassRate,
				mark,
			)
		}
		for _, metric := range report.Comparison.Metrics {
			if metric.Type != "bool" && metric.Type != "number" {
				continue
			}
			mark := "ok"
			if metric.Regressed {
				mark = "regressed"
			}
			fmt.Fprintf(&b, "  metric %s %s %.4g -> %.4g (delta %.4g, %s)\n",
				metric.Name,
				metric.Type,
				metric.Baseline,
				metric.Current,
				metric.Delta,
				mark,
			)
		}
	}
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "  %s %s (%s:%d:%d, %dms, %d assertions, %d metrics, %d subcases)\n",
			caseStatusMark(c.Status),
			c.Name,
			c.SourcePath,
			c.Range.StartLine,
			c.Range.StartColumn,
			c.DurationMS,
			len(c.Assertions),
			len(c.Metrics),
			len(c.Subcases),
		)
		for _, metric := range c.Metrics {
			fmt.Fprintf(&b, "    metric %s=%v (%s)\n", metric.Name, metric.Value, metric.Type)
		}
		for _, subcase := range c.Subcases {
			fmt.Fprintf(&b, "    %s case %s (%dms, %d metrics)\n", caseStatusMark(subcase.Status), subcase.CaseID, subcase.DurationMS, len(subcase.Metrics))
			for _, metric := range subcase.Metrics {
				fmt.Fprintf(&b, "      metric %s=%v (%s)\n", metric.Name, metric.Value, metric.Type)
			}
			for _, d := range subcase.Diagnostics {
				fmt.Fprintf(&b, "      %s: %s\n", d.Kind, d.Message)
			}
		}
		for _, d := range c.Diagnostics {
			fmt.Fprintf(&b, "    %s: %s\n", d.Kind, d.Message)
		}
	}
	if len(report.Findings) > 0 {
		fmt.Fprintf(&b, "findings:\n")
		for _, f := range report.Findings {
			location := f.Path
			if f.Line > 0 {
				location = fmt.Sprintf("%s:%d:%d", f.Path, f.Line, f.Column)
			}
			fmt.Fprintf(&b, "  %s %s %s: %s\n", f.Severity, f.Kind, location, f.Message)
		}
	}
	return b.String()
}

func formatMetricValues(values map[string]int) string {
	if len(values) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{")
	for i, key := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%s:%d", key, values[key])
	}
	b.WriteString("}")
	return b.String()
}

func caseStatusMark(status string) string {
	switch status {
	case "passed":
		return "PASS"
	case "failed":
		return "FAIL"
	default:
		return strings.ToUpper(status)
	}
}
