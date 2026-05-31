package gscript_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInternalStdlibLayerStaysBelowRuntime(t *testing.T) {
	root := repoRoot(t)
	stdlibRoot := filepath.Join(root, "internal", "stdlib")

	for _, domain := range []string{"ai", "base", "catalog", "data"} {
		dir := filepath.Join(stdlibRoot, domain)
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("internal/stdlib domain %q missing: %v", domain, err)
		}
		if !info.IsDir() {
			t.Fatalf("internal/stdlib domain %q is not a directory", domain)
		}
		if !hasGoFile(t, dir) {
			t.Fatalf("internal/stdlib domain %q has no Go implementation files", domain)
		}
	}

	forEachGoFile(t, stdlibRoot, func(path string) {
		for _, importPath := range parseImports(t, path) {
			if importPath == "github.com/never-labs/gscript/internal/runtime" ||
				strings.HasPrefix(importPath, "github.com/never-labs/gscript/internal/runtime/") {
				t.Fatalf("%s imports %s; internal/stdlib must stay pure and below runtime adapters", relativeToRoot(t, path), importPath)
			}
		}
	})
}

func TestRuntimeStdlibWrappersDoNotImportProviderImplementations(t *testing.T) {
	runtimeRoot := filepath.Join(repoRoot(t), "internal", "runtime")

	forEachGoFile(t, runtimeRoot, func(path string) {
		name := filepath.Base(path)
		if !strings.HasPrefix(name, "stdlib") || strings.HasSuffix(name, "_test.go") {
			return
		}
		for _, importPath := range parseImports(t, path) {
			if importPath == "github.com/never-labs/gscript/llm/openai" ||
				importPath == "github.com/never-labs/gscript/llm/anthropic" ||
				importPath == "github.com/never-labs/gscript/llm/command" {
				t.Fatalf("%s imports provider implementation %s; runtime stdlib wrappers should depend on protocol/adapter surfaces, not hosted provider packages", relativeToRoot(t, path), importPath)
			}
		}
	})
}

func hasGoFile(t *testing.T, dir string) bool {
	t.Helper()
	found := false
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", dir, err)
	}
	return found
}

func forEachGoFile(t *testing.T, root string, fn func(path string)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".go") {
			fn(path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
}

func parseImports(t *testing.T, path string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parse imports for %s: %v", path, err)
	}
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}

func relativeToRoot(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repoRoot(t), path)
	if err != nil {
		return path
	}
	return rel
}
