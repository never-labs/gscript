package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/never-labs/gscript/internal/lexer"
	toolsource "github.com/never-labs/gscript/internal/support/source"
)

func runFmtCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(errw)
	check := fs.Bool("check", false, "check whether files are formatted without writing")
	write := fs.Bool("write", false, "write formatted files in place")
	stdinFileName := fs.String("stdin-file-name", "", "read source from stdin and use this filename for diagnostics")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 && *stdinFileName == "" {
		fmt.Fprintln(errw, "usage: gscript fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]")
		return 2
	}
	if *check && *write {
		fmt.Fprintln(errw, "gscript fmt: --check and --write are mutually exclusive")
		return 2
	}
	if *stdinFileName != "" {
		if len(paths) != 0 {
			fmt.Fprintln(errw, "gscript fmt: --stdin-file-name cannot be used with path arguments")
			return 2
		}
		if *write {
			fmt.Fprintln(errw, "gscript fmt: --stdin-file-name cannot be used with --write")
			return 2
		}
		return runFmtStdin(*stdinFileName, *check, outw, errw)
	}

	writeFiles := *write || !*check
	ok := true
	for _, path := range paths {
		files, err := toolsource.Files(path)
		if err != nil {
			fmt.Fprintf(errw, "%s: %v\n", path, err)
			ok = false
			continue
		}
		for _, filename := range files {
			changed, err := formatFile(filename, writeFiles)
			if err != nil {
				fmt.Fprintf(errw, "%s: %v\n", filename, err)
				ok = false
				continue
			}
			if *check && changed {
				fmt.Fprintf(errw, "%s: not formatted\n", filename)
				ok = false
			}
			if writeFiles && changed {
				fmt.Fprintln(outw, filename)
			}
		}
	}
	if !ok {
		return 1
	}
	return 0
}

func runFmtStdin(filename string, check bool, outw, errw io.Writer) int {
	src, err := io.ReadAll(cliStdin)
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	if check {
		if !bytes.Equal(src, formatted) {
			fmt.Fprintf(errw, "%s: not formatted\n", filename)
			return 1
		}
		return 0
	}
	if _, err := outw.Write(formatted); err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	return 0
}

func formatFile(filename string, write bool) (bool, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		return false, err
	}
	changed := !bytes.Equal(src, formatted)
	if write && changed {
		if err := os.WriteFile(filename, formatted, 0644); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func formatSource(filename string, src []byte) ([]byte, error) {
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
