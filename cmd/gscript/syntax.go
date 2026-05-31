package main

import (
	"fmt"
	"os"

	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/parser"
)

func parseGScriptFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return parseGScriptSource(filename, src)
}

func parseGScriptSource(filename string, src []byte) error {
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}
	if _, err := parser.New(tokens).Parse(); err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	return nil
}
