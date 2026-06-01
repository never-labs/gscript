package gscript_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/gscript/internal/stdlib/catalog"
)

func TestInternalStdlibLayerStaysBelowRuntime(t *testing.T) {
	root := repoRoot(t)
	stdlibRoot := filepath.Join(root, "internal", "stdlib")

	for _, module := range []string{"catalog", "llm", "table", "soa", "fs", "http"} {
		dir := filepath.Join(stdlibRoot, module)
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("internal/stdlib module %q missing: %v", module, err)
		}
		if !info.IsDir() {
			t.Fatalf("internal/stdlib module %q is not a directory", module)
		}
		if !hasGoFile(t, dir) {
			t.Fatalf("internal/stdlib module %q has no Go implementation files", module)
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

func TestRuntimeDoesNotImportHostedProviderImplementations(t *testing.T) {
	runtimeRoot := filepath.Join(repoRoot(t), "internal", "runtime")

	forEachGoFile(t, runtimeRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, importPath := range parseImports(t, path) {
			if importPath == "github.com/never-labs/gscript/llm/openai" ||
				importPath == "github.com/never-labs/gscript/llm/anthropic" ||
				importPath == "github.com/never-labs/gscript/llm/command" {
				t.Fatalf("%s imports provider implementation %s; runtime should depend on protocol/adapter surfaces, not hosted provider packages", relativeToRoot(t, path), importPath)
			}
		}
	})
}

func TestRuntimeDoesNotImportStdlibImplementations(t *testing.T) {
	runtimeRoot := filepath.Join(repoRoot(t), "internal", "runtime")

	forEachGoFile(t, runtimeRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, importPath := range parseImports(t, path) {
			if importPath == "github.com/never-labs/gscript/internal/stdlib" ||
				strings.HasPrefix(importPath, "github.com/never-labs/gscript/internal/stdlib/") {
				t.Fatalf("%s imports %s; runtime must depend on shared substrates or stdlibrt adapters, not stdlib implementation packages", relativeToRoot(t, path), importPath)
			}
		}
	})
}

func TestInternalStdlibDirsRepresentCatalogModules(t *testing.T) {
	stdlibRoot := filepath.Join(repoRoot(t), "internal", "stdlib")
	moduleNames := map[string]bool{}
	for _, module := range catalog.Modules() {
		moduleNames[module.Name] = true
	}
	dirToModule := map[string]string{
		"catalog": "catalog",
		"utf8x":   "utf8",
	}

	entries, err := os.ReadDir(stdlibRoot)
	if err != nil {
		t.Fatalf("read internal/stdlib: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		if dir == "catalog" {
			continue
		}
		module := dir
		if mapped := dirToModule[dir]; mapped != "" {
			module = mapped
		}
		if !moduleNames[module] {
			t.Fatalf("internal/stdlib/%s is not a catalog stdlib module; shared substrates belong in neutral internal packages", dir)
		}
	}
}

func TestStdlibrtModulesDoNotOwnAdapterContracts(t *testing.T) {
	modulesRoot := filepath.Join(repoRoot(t), "internal", "stdlibrt", "modules")

	forEachGoFile(t, modulesRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				name := typeSpec.Name.Name
				if strings.HasSuffix(name, "Options") {
					t.Fatalf("%s defines adapter option type %s; stdlibrt adapter contracts belong in dedicated internal/stdlibrt/* packages", relativeToRoot(t, path), name)
				}
				if strings.HasSuffix(name, "Runtime") {
					if _, ok := typeSpec.Type.(*ast.InterfaceType); ok {
						t.Fatalf("%s defines runtime adapter interface %s; stdlibrt adapter contracts belong in dedicated internal/stdlibrt/* packages", relativeToRoot(t, path), name)
					}
				}
			}
		}
	})
}

func TestStdlibrtKeepsThinContractsInRootPackage(t *testing.T) {
	root := filepath.Join(repoRoot(t), "internal", "stdlibrt")
	allowedSubdirs := map[string]bool{
		"install": true,
		"modules": true,
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read internal/stdlibrt: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !allowedSubdirs[entry.Name()] {
			t.Fatalf("internal/stdlibrt/%s is a thin adapter subpackage; keep adapter contracts in internal/stdlibrt or put real module bindings under modules", entry.Name())
		}
	}
}

func TestInternalSupportPackagesStayGrouped(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"binaryfmt", "debugstate", "filemode", "hostpath", "outputlimit", "stringlib"} {
		if _, err := os.Stat(filepath.Join(root, "internal", name)); !os.IsNotExist(err) {
			t.Fatalf("internal/%s should not be a top-level architecture package; keep shared support code under internal/support/%s", name, name)
		}
		dir := filepath.Join(root, "internal", "support", name)
		info, err := os.Stat(dir)
		if err != nil {
			t.Fatalf("internal/support/%s missing: %v", name, err)
		}
		if !info.IsDir() {
			t.Fatalf("internal/support/%s is not a directory", name)
		}
	}
}

func TestInternalTopLevelPackagesStayArchitectural(t *testing.T) {
	root := filepath.Join(repoRoot(t), "internal")
	allowed := map[string]bool{
		"ast":       true,
		"binding":   true,
		"jit":       true,
		"lexer":     true,
		"llmbridge": true,
		"methodjit": true,
		"modfile":   true,
		"nanbox":    true,
		"parser":    true,
		"runtime":   true,
		"stdlib":    true,
		"stdlibrt":  true,
		"support":   true,
		"testutil":  true,
		"vm":        true,
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read internal: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !allowed[entry.Name()] {
			t.Fatalf("internal/%s is not an approved top-level architecture package; move small helpers under internal/support or tests under internal/testutil", entry.Name())
		}
	}
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
