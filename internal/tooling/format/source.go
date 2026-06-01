// Package format provides shared Leia source formatting for CLI and editor
// tooling.
package format

import (
	"bytes"
	"fmt"

	"github.com/never-labs/leia/internal/lexer"
	toolsource "github.com/never-labs/leia/internal/support/source"
)

// Source formats a Leia source buffer after validating it with the shared
// lexer/parser syntax check.
func Source(filename string, src []byte) ([]byte, error) {
	if err := toolsource.Parse(filename, src); err != nil {
		return nil, err
	}

	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}

	normalized := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	lines := bytes.Split(normalized, []byte("\n"))
	for i := range lines {
		lines[i] = bytes.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []byte("\n"), nil
	}
	// The formatter is deliberately parse-backed, not AST-printed: parsing is
	// a safety gate, then token positions drive only newline/trailing-space
	// normalization and brace indentation. Intra-line expression spacing,
	// table/config field alignment, and comment attachment stay untouched until
	// an AST printer can round-trip comments and original source boundaries.
	indentFormatted := formatLineIndentation(lines, tokens)
	return append(bytes.Join(indentFormatted, []byte("\n")), '\n'), nil
}

func formatLineIndentation(lines [][]byte, tokens []lexer.Token) [][]byte {
	lineStats := map[int]formatLineStats{}
	for _, tok := range tokens {
		if tok.Type == lexer.TOKEN_EOF {
			continue
		}
		stats := lineStats[tok.Line]
		if !stats.HasToken {
			stats.HasToken = true
			stats.StartsWithClosingBrace = tok.Type == lexer.TOKEN_RBRACE
		}
		switch tok.Type {
		case lexer.TOKEN_LBRACE:
			stats.OpenBraces++
		case lexer.TOKEN_RBRACE:
			stats.CloseBraces++
		}
		lineStats[tok.Line] = stats
	}

	out := make([][]byte, len(lines))
	indent := 0
	for i, line := range lines {
		lineNo := i + 1
		stats := lineStats[lineNo]
		lineIndent := indent
		if stats.StartsWithClosingBrace && lineIndent > 0 {
			lineIndent--
		}

		trimmed := bytes.TrimLeft(line, " \t")
		if len(line) == 0 {
			out[i] = nil
		} else {
			if !stats.HasToken && !bytes.HasPrefix(trimmed, []byte("//")) {
				out[i] = line
			} else {
				out[i] = append(bytes.Repeat([]byte(" "), lineIndent*4), trimmed...)
			}
		}

		indent += stats.OpenBraces - stats.CloseBraces
		if indent < 0 {
			indent = 0
		}
	}
	return out
}

type formatLineStats struct {
	HasToken               bool
	StartsWithClosingBrace bool
	OpenBraces             int
	CloseBraces            int
}
