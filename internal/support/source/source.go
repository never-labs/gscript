// Package source provides shared source-file discovery and syntax checks for
// Leia tooling.
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

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
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return Parse(filename, src)
}

func Parse(filename string, src []byte) error {
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}
	if _, err := parser.New(tokens).Parse(); err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	return nil
}
