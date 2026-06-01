package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/catalog"
)

func TestInternalStdlibLayerStaysBelowRuntime(t *testing.T) {
	root := repoRoot(t)
	stdlibRoot := filepath.Join(root, "internal", "stdlib")
	libRoot := filepath.Join(stdlibRoot, "lib")

	for _, module := range []string{"catalog", "lib/llm", "lib/table", "lib/soa", "lib/fs", "lib/http", "bind", "install"} {
		dir := filepath.Join(stdlibRoot, filepath.FromSlash(module))
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
	forEachGoFile(t, libRoot, func(path string) {
		for _, importPath := range parseImports(t, path) {
			if importPath == "github.com/never-labs/leia/internal/runtime" ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/runtime/") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/vm") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/jit") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/methodjit") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/stdlib/bind") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/stdlib/install") {
				t.Fatalf("%s imports %s; internal/stdlib/lib must stay pure and below runtime bindings", relativeToRoot(t, path), importPath)
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
			if importPath == "github.com/never-labs/leia/llm/openai" ||
				importPath == "github.com/never-labs/leia/llm/anthropic" ||
				importPath == "github.com/never-labs/leia/llm/command" {
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
			if importPath == "github.com/never-labs/leia/internal/stdlib" ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/stdlib/") {
				t.Fatalf("%s imports %s; runtime must depend on shared substrates or stdlib bindings, not stdlib implementation packages", relativeToRoot(t, path), importPath)
			}
		}
	})
}

func TestInternalStdlibDirsRepresentCatalogModules(t *testing.T) {
	stdlibRoot := filepath.Join(repoRoot(t), "internal", "stdlib")
	libRoot := filepath.Join(stdlibRoot, "lib")
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
	allowedRoot := map[string]bool{
		"catalog": true,
		"lib":     true,
		"bind":    true,
		"install": true,
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		if !allowedRoot[dir] {
			t.Fatalf("internal/stdlib/%s is not an approved stdlib layer; expected catalog, lib, bind, or install", dir)
		}
	}

	entries, err = os.ReadDir(libRoot)
	if err != nil {
		t.Fatalf("read internal/stdlib/lib: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		module := dir
		if mapped := dirToModule[dir]; mapped != "" {
			module = mapped
		}
		if !moduleNames[module] {
			t.Fatalf("internal/stdlib/lib/%s is not a catalog stdlib module; shared substrates belong in neutral internal packages", dir)
		}
	}
}

func TestStdlibBindOwnsRuntimeAdapterContracts(t *testing.T) {
	bindRoot := filepath.Join(repoRoot(t), "internal", "stdlib", "bind")
	installRoot := filepath.Join(repoRoot(t), "internal", "stdlib", "install")

	forEachGoFile(t, bindRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, importPath := range parseImports(t, path) {
			if strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/vm") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/jit") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/methodjit") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/stdlib/install") {
				t.Fatalf("%s imports %s; stdlib/bind may adapt runtime and lib packages only", relativeToRoot(t, path), importPath)
			}
		}
	})
	forEachGoFile(t, installRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		for _, importPath := range parseImports(t, path) {
			if strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/vm") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/jit") ||
				strings.HasPrefix(importPath, "github.com/never-labs/leia/internal/methodjit") {
				t.Fatalf("%s imports %s; stdlib/install should only compose runtime and stdlib/bind", relativeToRoot(t, path), importPath)
			}
		}
	})
}

func TestInternalSupportPackagesStayGrouped(t *testing.T) {
	root := repoRoot(t)
	for _, name := range []string{"binaryfmt", "hostpath", "modresolve", "source", "stringlib"} {
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
		"modpkg":    true,
		"nanbox":    true,
		"parser":    true,
		"runtime":   true,
		"stdlib":    true,
		"support":   true,
		"testutil":  true,
		"tooling":   true,
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
