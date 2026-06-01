package evaluate

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	stdlibinstall "github.com/never-labs/leia/internal/stdlib/install"
)

const SchemaVersion = 1

type Options struct {
	Paths []string
}

type Report struct {
	SchemaVersion int       `json:"schema_version"`
	Phase         string    `json:"phase"`
	Status        string    `json:"status"`
	Summary       Summary   `json:"summary"`
	Inputs        []Input   `json:"inputs"`
	Cases         []Case    `json:"cases"`
	Findings      []Finding `json:"findings"`
	Notes         []string  `json:"notes"`
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
	CaseID     string      `json:"case_id"`
	Name       string      `json:"name"`
	SourcePath string      `json:"source_path"`
	Range      SourceRange `json:"range"`
	Status     string      `json:"status"`
}

type SourceRange struct {
	StartLine   int `json:"start_line"`
	StartColumn int `json:"start_column"`
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
		Inputs:        []Input{},
		Cases:         []Case{},
		Findings:      []Finding{},
		Notes: []string{
			"evaluate runs each evaluate block body as ordinary Leia code; provider scoring, golden updates, and workflow orchestration are reserved for later phases.",
		},
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
					c := parsed.Case
					if err := executeCase(file, prog, parsed); err != nil {
						c.Status = "failed"
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
	return report, nil
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

func executeCase(path string, prog *ast.Program, c parsedCase) error {
	if prog == nil || c.Body == nil {
		return nil
	}
	interp := runtime.NewCore()
	stdlibinstall.Install(interp)
	if abs, err := filepath.Abs(path); err == nil {
		interp.SetScriptDir(filepath.Dir(abs))
	}
	interp.SetArgs(path, nil)
	return interp.Exec(&ast.Program{
		Stmts:          caseProgramStmts(prog.Stmts, c.Body.Stmts),
		FileDirectives: append([]ast.FileDirective(nil), prog.FileDirectives...),
	})
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

func FormatText(report Report) string {
	return fmt.Sprintf("evaluate: %s (%d files, %d parsed, %d cases, %d agents, %d tools, %d todos)\n",
		report.Status,
		report.Summary.Files,
		report.Summary.ParsedFiles,
		report.Summary.EvaluateBlocks,
		report.Summary.Agents,
		report.Summary.Tools,
		report.Summary.TODOs,
	)
}
