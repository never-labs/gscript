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
	Findings      []Finding `json:"findings"`
	Notes         []string  `json:"notes"`
}

type Summary struct {
	Files       int `json:"files"`
	ParsedFiles int `json:"parsed_files"`
	Agents      int `json:"agents"`
	Tools       int `json:"tools"`
	Models      int `json:"models"`
	Budgets     int `json:"budgets"`
	TODOs       int `json:"todos"`
}

type Input struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}

type Finding struct {
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`
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
		Phase:         "syntax-static",
		Status:        "ok",
		Inputs:        []Input{},
		Findings:      []Finding{},
		Notes: []string{
			"evaluate P0 performs syntax-level discovery only; it does not run providers, tools, or agent workflows.",
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
			counts, findings := parseLeia(file, src)
			report.Summary.ParsedFiles += counts.ParsedFiles
			report.Summary.Agents += counts.Agents
			report.Summary.Tools += counts.Tools
			report.Summary.Models += counts.Models
			report.Summary.Budgets += counts.Budgets
			if len(findings) > 0 {
				input.Status = "error"
				report.Status = "failed"
				report.Findings = append(report.Findings, findings...)
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

func parseLeia(path string, src []byte) (Summary, []Finding) {
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return Summary{}, []Finding{{Kind: "lex_error", Severity: "error", Message: err.Error(), Path: path}}
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return Summary{}, []Finding{{Kind: "parse_error", Severity: "error", Message: err.Error(), Path: path}}
	}
	counts := Summary{ParsedFiles: 1}
	countLLMStmts(prog.Stmts, &counts)
	if err := ast.ValidateLLM(prog); err != nil {
		return counts, []Finding{{Kind: "ai_syntax_error", Severity: "error", Message: err.Error(), Path: path}}
	}
	return counts, nil
}

func countLLMStmts(stmts []ast.Stmt, counts *Summary) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AgentDeclStmt:
			counts.Agents++
			if s.Flow != nil {
				countLLMStmts(s.Flow.Stmts, counts)
			}
		case *ast.ToolDeclStmt:
			counts.Tools++
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
		case *ast.ModelsDeclStmt:
			counts.Models++
		case *ast.BudgetStmt:
			counts.Budgets++
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
		case *ast.BlockStmt:
			countLLMStmts(s.Stmts, counts)
		case *ast.FuncDeclStmt:
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
		case *ast.IfStmt:
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
			for _, elseif := range s.ElseIfs {
				if elseif.Body != nil {
					countLLMStmts(elseif.Body.Stmts, counts)
				}
			}
			if s.ElseBody != nil {
				countLLMStmts(s.ElseBody.Stmts, counts)
			}
		case *ast.ForStmt:
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
		case *ast.ForNumStmt:
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
		case *ast.ForRangeStmt:
			if s.Body != nil {
				countLLMStmts(s.Body.Stmts, counts)
			}
		case *ast.SelectStmt:
			for _, c := range s.Cases {
				if c.Body != nil {
					countLLMStmts(c.Body.Stmts, counts)
				}
			}
			if s.Default != nil {
				countLLMStmts(s.Default.Stmts, counts)
			}
		}
	}
}

func FormatText(report Report) string {
	return fmt.Sprintf("evaluate: %s (%d files, %d parsed, %d agents, %d tools, %d todos)\n",
		report.Status,
		report.Summary.Files,
		report.Summary.ParsedFiles,
		report.Summary.Agents,
		report.Summary.Tools,
		report.Summary.TODOs,
	)
}
