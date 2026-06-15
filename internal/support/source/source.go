// Package source provides shared source-file discovery and syntax checks for
// Leia tooling.
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/parser"
)

func Files(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".leia" {
			return nil, fmt.Errorf("file must have .leia extension")
		}
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".leia" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func ParseFile(filename string) error {
	_, err := ParseFileProgram(filename)
	return err
}

func ParseFileProgram(filename string) (*ast.Program, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return ParseProgram(filename, src)
}

func Parse(filename string, src []byte) error {
	_, err := ParseProgram(filename, src)
	return err
}

func ParseProgram(filename string, src []byte) (*ast.Program, error) {
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return nil, fmt.Errorf("lexer error: %w", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return prog, nil
}
