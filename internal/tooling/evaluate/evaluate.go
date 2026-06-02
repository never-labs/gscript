package evaluate

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	goruntime "runtime"
	"runtime/debug"
	"sort"
	"strings"
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
	SchemaVersion int         `json:"schema_version"`
	Phase         string      `json:"phase"`
	Status        string      `json:"status"`
	StartedAt     string      `json:"started_at"`
	Runtime       RuntimeInfo `json:"runtime"`
	Summary       Summary     `json:"summary"`
	LLM           *LLMRun     `json:"llm,omitempty"`
	Inputs        []Input     `json:"inputs"`
	Cases         []Case      `json:"cases"`
	Findings      []Finding   `json:"findings"`
	Notes         []string    `json:"notes"`
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
	Files          int `json:"files"`
	ParsedFiles    int `json:"parsed_files"`
	EvaluateBlocks int `json:"evaluate_blocks"`
	Agents         int `json:"agents"`
	Tools          int `json:"tools"`
	Models         int `json:"models"`
	Budgets        int `json:"budgets"`
	TODOs          int `json:"todos"`
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
	DurationMS  int64        `json:"duration_ms,omitempty"`
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

type Finding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
}

type parsedCase struct {
	Case
	Body *ast.BlockStmt
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
						continue
					}
					c := parsed.Case
					c.Assertions = collectAssertions(parsed.Body)
					if opts.ListOnly {
						c.Status = "listed"
						report.Cases = append(report.Cases, c)
						continue
					}
					start := time.Now()
					if err := executeCase(file, prog, parsed, run); err != nil {
						c.Status = "failed"
						c.DurationMS = elapsedMillis(start)
						markAssertions(c.Assertions, "unknown")
						if len(c.Assertions) == 1 || len(c.Assertions) > 0 && strings.Contains(err.Error(), "assert") {
							c.Assertions[0].Status = "failed"
						}
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
		if remaining := run.replayProvider.Remaining(); remaining > 0 {
			report.Status = "failed"
			report.Findings = append(report.Findings, Finding{
				Kind:     "llm_replay_unconsumed",
				Severity: "error",
				Message:  fmt.Sprintf("llm replay left %d unconsumed turn(s)", remaining),
				Path:     opts.LLMReplayPath,
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
	return report, nil
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

func executeCase(path string, prog *ast.Program, c parsedCase, run *runContext) error {
	if prog == nil || c.Body == nil {
		return nil
	}
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)
	if run != nil {
		if run.replayProvider != nil {
			interp.SetLLMProvider(llmbridge.ProviderAdapter(run.replayProvider))
		}
		if run.recorder != nil {
			interp.SetLLMProviderFactory(recordingProviderFactory(run.providerFactory, run.recorder.Record))
		} else if run.providerFactory != nil {
			interp.SetLLMProviderFactory(run.providerFactory)
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		interp.SetScriptDir(filepath.Dir(abs))
	}
	interp.SetArgs(path, nil)
	return interp.Exec(&ast.Program{
		Stmts:          caseProgramStmts(prog.Stmts, c.Body.Stmts),
		FileDirectives: append([]ast.FileDirective(nil), prog.FileDirectives...),
	})
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

func FormatText(report Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "evaluate: %s (%d files, %d parsed, %d cases, %d agents, %d tools, %d todos)\n",
		report.Status,
		report.Summary.Files,
		report.Summary.ParsedFiles,
		report.Summary.EvaluateBlocks,
		report.Summary.Agents,
		report.Summary.Tools,
		report.Summary.TODOs,
	)
	for _, c := range report.Cases {
		fmt.Fprintf(&b, "  %s %s (%s:%d:%d, %dms, %d assertions)\n",
			caseStatusMark(c.Status),
			c.Name,
			c.SourcePath,
			c.Range.StartLine,
			c.Range.StartColumn,
			c.DurationMS,
			len(c.Assertions),
		)
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
