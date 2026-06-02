package lsp

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
)

var diagnosticPositionRE = regexp.MustCompile(`(?:^|[^0-9])([0-9]+):([0-9]+)(?:[^0-9]|$)`)

type diagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

func syntaxDiagnostics(src string) []diagnostic {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return []diagnostic{newSyntaxDiagnostic("LEIA1001", fmt.Errorf("lexer error: %w", err))}
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return []diagnostic{newSyntaxDiagnostic("LEIA1001", fmt.Errorf("parse error: %w", err))}
	}
	if err := ast.ValidateLLM(prog); err != nil {
		return []diagnostic{newSyntaxDiagnostic("LEIA2001", err)}
	}
	return nil
}

func newSyntaxDiagnostic(code string, err error) diagnostic {
	line, col := parseDiagnosticPosition(err.Error())
	start := positionFromOneBased(line, col)
	end := start
	end.Character++
	return diagnostic{
		Range:    lspRange{Start: start, End: end},
		Severity: 1,
		Code:     code,
		Source:   "leia",
		Message:  err.Error(),
	}
}

func parseDiagnosticPosition(message string) (int, int) {
	match := diagnosticPositionRE.FindStringSubmatch(message)
	if match == nil {
		return 1, 1
	}
	line, lineErr := strconv.Atoi(match[1])
	col, colErr := strconv.Atoi(match[2])
	if lineErr != nil || colErr != nil || line < 1 || col < 1 {
		return 1, 1
	}
	return line, col
}

func positionFromOneBased(line, col int) position {
	if line < 1 {
		line = 1
	}
	if col < 1 {
		col = 1
	}
	return position{Line: line - 1, Character: col - 1}
}
