package architecture_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
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

func TestBuiltinDialectRegistryStaysModular(t *testing.T) {
	bindRoot := filepath.Join(repoRoot(t), "internal", "stdlib", "bind")
	expectedCalls := []string{
		"registerDialectShellFS",
		"registerDialectText",
		"registerDialectProtocol",
		"registerDialectProtocolNetwork",
		"registerDialectWeb",
		"registerDialectData",
		"registerDialectDatabase",
		"registerDialectAI",
	}
	expectedTagsByFile := map[string][]string{
		"dialect_shell_fs.go":         {"sh", "cmd", "shellwords", "glob", "path"},
		"dialect_text.go":             {"re", "regexp", "json", "jsonptr", "jsonl", "csv", "tsv", "mdtable", "markdown", "md", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "yaml", "yml", "semver", "duration", "timestamp", "rfc3339", "tap", "junit", "xml", "template"},
		"dialect_yaml.go":             nil,
		"dialect_protocol.go":         {"url", "html_escape", "html", "urlquery", "form", "urlform", "urlpath", "mime", "mailaddr", "emailaddr", "headers", "http_headers", "cookie", "cookies", "httpmsg", "sse", "multipart", "jwt"},
		"dialect_protocol_network.go": {"ipaddr", "cidr", "hostport"},
		"dialect_web.go":              {"serve"},
		"dialect_data.go":             {"base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary", "q", "pem", "xlsx", "excel"},
		"dialect_xlsx.go":             nil,
		"dialect_database.go":         {"sql"},
		"dialect_ai.go":               {"prompt", "quote", "model", "turn", "tool", "agent"},
	}
	expectedProjectImportsByFile := map[string][]string{
		"dialect_shell_fs.go": {
			"github.com/never-labs/leia/internal/stdlib/lib/path",
			"github.com/never-labs/leia/internal/support",
			"github.com/never-labs/leia/internal/support/dialect",
		},
		"dialect_text.go": {
			"github.com/never-labs/leia/internal/runtime",
			"github.com/never-labs/leia/internal/stdlib/lib/csv",
			"github.com/never-labs/leia/internal/stdlib/lib/encoding",
			"github.com/never-labs/leia/internal/support/dialect",
		},
		"dialect_yaml.go": nil,
		"dialect_protocol.go": {
			"github.com/never-labs/leia/internal/support/dialect",
		},
		"dialect_protocol_network.go": nil,
		"dialect_web.go":              nil,
		"dialect_database.go":         nil,
		"dialect_data.go": {
			"github.com/never-labs/leia/internal/stdlib/lib/base64",
			"github.com/never-labs/leia/internal/stdlib/lib/compress",
			"github.com/never-labs/leia/internal/stdlib/lib/encoding",
			"github.com/never-labs/leia/internal/stdlib/lib/hash",
			"github.com/never-labs/leia/internal/stdlib/lib/uuid",
			"github.com/never-labs/leia/internal/support/binaryfmt",
		},
		"dialect_xlsx.go": nil,
		"dialect_ai.go":   nil,
	}

	dialectFile := parseGoFile(t, filepath.Join(bindRoot, "dialect.go"))
	actualCalls := directCallsInFunc(dialectFile, "BuildDialect", "registerDialect")
	if strings.Join(actualCalls, ",") != strings.Join(expectedCalls, ",") {
		t.Fatalf("BuildDialect registers dialect modules %v, want %v", actualCalls, expectedCalls)
	}

	entries, err := os.ReadDir(bindRoot)
	if err != nil {
		t.Fatalf("read internal/stdlib/bind: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, "dialect_") || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if _, ok := expectedTagsByFile[name]; !ok {
			t.Fatalf("internal/stdlib/bind/%s is not an approved builtin dialect module; keep larger domains outside the builtin registry", name)
		}
	}

	seen := map[string]string{}
	for fileName, expectedTags := range expectedTagsByFile {
		path := filepath.Join(bindRoot, fileName)
		actualProjectImports := projectImports(parseImports(t, path))
		expectedProjectImports := expectedProjectImportsByFile[fileName]
		if strings.Join(actualProjectImports, ",") != strings.Join(expectedProjectImports, ",") {
			t.Fatalf("%s imports project packages %v, want %v; keep builtin dialects on small substrate packages only", relativeToRoot(t, path), actualProjectImports, expectedProjectImports)
		}
		actualTags := dialectTagsRegisteredByFile(t, path)
		if strings.Join(actualTags, ",") != strings.Join(expectedTags, ",") {
			t.Fatalf("%s registers dialect tags %v, want %v", relativeToRoot(t, path), actualTags, expectedTags)
		}
		for _, tag := range actualTags {
			if previous := seen[tag]; previous != "" {
				t.Fatalf("dialect tag %q registered by both %s and %s", tag, previous, fileName)
			}
			seen[tag] = fileName
		}
	}
	featureMatrixTags := loadFeatureMatrixBuiltinDialectTags(t)
	var missingFromMatrix, extraInMatrix []string
	for tag := range seen {
		if !featureMatrixTags[tag] {
			missingFromMatrix = append(missingFromMatrix, tag)
		}
	}
	for tag := range featureMatrixTags {
		if seen[tag] == "" {
			extraInMatrix = append(extraInMatrix, tag)
		}
	}
	if len(missingFromMatrix) > 0 || len(extraInMatrix) > 0 {
		sort.Strings(missingFromMatrix)
		sort.Strings(extraInMatrix)
		t.Fatalf("builtin dialect tags must stay aligned with tests/feature_matrix.json tagged_dialect_syntax.builtin_dialect_tags; missing from matrix=%v extra in matrix=%v", missingFromMatrix, extraInMatrix)
	}
	exampleGate := readArchitectureFile(t, filepath.Join(repoRoot(t), "cmd", "leia", "main_examples_test.go"))
	for _, snippet := range []string{
		"TestRunCommandDialectExamplesCoverApprovedBuiltinTags",
		"approvedBuiltinDialectTags",
		"collectDialectExampleTags",
	} {
		if !strings.Contains(exampleGate, snippet) {
			t.Fatalf("cmd/leia/main_examples_test.go must keep builtin dialect release/example gate snippet %q", snippet)
		}
	}

	forEachGoFile(t, bindRoot, func(path string) {
		if strings.HasSuffix(path, "_test.go") {
			return
		}
		fileName := filepath.Base(path)
		tags := dialectTagsRegisteredByFile(t, path)
		if len(tags) > 0 {
			if _, ok := expectedTagsByFile[fileName]; !ok {
				t.Fatalf("%s registers builtin dialect tags %v outside an approved dialect module", relativeToRoot(t, path), tags)
			}
		}
		for _, fn := range funcsAcceptingDialectRegisterFunc(t, path) {
			if !stringInSlice(expectedCalls, fn) || expectedDialectFileForFunc(fn) != fileName {
				t.Fatalf("%s defines %s with dialectRegisterFunc; builtin dialect registration must stay in approved modules", relativeToRoot(t, path), fn)
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
	file := parseGoFileMode(t, path, parser.ImportsOnly)
	imports := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		imports = append(imports, strings.Trim(spec.Path.Value, `"`))
	}
	return imports
}

func parseGoFile(t *testing.T, path string) *ast.File {
	t.Helper()
	return parseGoFileMode(t, path, 0)
}

func parseGoFileMode(t *testing.T, path string, mode parser.Mode) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, mode)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

func directCallsInFunc(file *ast.File, funcName, calleePrefix string) []string {
	var calls []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName || fn.Body == nil {
			continue
		}
		for _, stmt := range fn.Body.List {
			exprStmt, ok := stmt.(*ast.ExprStmt)
			if !ok {
				continue
			}
			call, ok := exprStmt.X.(*ast.CallExpr)
			if !ok {
				continue
			}
			callee, ok := call.Fun.(*ast.Ident)
			if ok && strings.HasPrefix(callee.Name, calleePrefix) {
				calls = append(calls, callee.Name)
			}
		}
	}
	return calls
}

func dialectTagsRegisteredByFile(t *testing.T, path string) []string {
	t.Helper()
	file := parseGoFile(t, path)
	var tags []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		callee, ok := call.Fun.(*ast.Ident)
		if !ok || callee.Name != "register" || len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.CompositeLit)
		if !ok {
			if filepath.Base(path) == "dialect.go" {
				return true
			}
			t.Fatalf("%s has dialect register call without literal tag list", relativeToRoot(t, path))
		}
		for _, elt := range lit.Elts {
			tag, ok := stringLiteralValue(elt)
			if !ok {
				t.Fatalf("%s has dialect register call with non-literal tag", relativeToRoot(t, path))
			}
			tags = append(tags, tag)
		}
		return true
	})
	return tags
}

func funcsAcceptingDialectRegisterFunc(t *testing.T, path string) []string {
	t.Helper()
	file := parseGoFile(t, path)
	var funcs []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type.Params == nil {
			continue
		}
		for _, field := range fn.Type.Params.List {
			ident, ok := field.Type.(*ast.Ident)
			if ok && ident.Name == "dialectRegisterFunc" {
				funcs = append(funcs, fn.Name.Name)
				break
			}
		}
	}
	return funcs
}

func expectedDialectFileForFunc(funcName string) string {
	switch funcName {
	case "registerDialectShellFS":
		return "dialect_shell_fs.go"
	case "registerDialectText":
		return "dialect_text.go"
	case "registerDialectProtocol":
		return "dialect_protocol.go"
	case "registerDialectProtocolNetwork":
		return "dialect_protocol_network.go"
	case "registerDialectWeb":
		return "dialect_web.go"
	case "registerDialectData":
		return "dialect_data.go"
	case "registerDialectDatabase":
		return "dialect_database.go"
	case "registerDialectAI":
		return "dialect_ai.go"
	default:
		return ""
	}
}

func projectImports(imports []string) []string {
	var project []string
	for _, importPath := range imports {
		if strings.HasPrefix(importPath, "github.com/never-labs/leia/") {
			project = append(project, importPath)
		}
	}
	return project
}

func loadFeatureMatrixBuiltinDialectTags(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), "tests", "feature_matrix.json"))
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}
	var matrix struct {
		Features []struct {
			ID                 string   `json:"id"`
			BuiltinDialectTags []string `json:"builtin_dialect_tags"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}
	for _, feature := range matrix.Features {
		if feature.ID != "tagged_dialect_syntax" {
			continue
		}
		tags := make(map[string]bool, len(feature.BuiltinDialectTags))
		for _, tag := range feature.BuiltinDialectTags {
			if tag == "" {
				t.Fatal("tagged_dialect_syntax.builtin_dialect_tags must not contain empty tags")
			}
			if tags[tag] {
				t.Fatalf("tagged_dialect_syntax.builtin_dialect_tags contains duplicate tag %q", tag)
			}
			tags[tag] = true
		}
		if len(tags) == 0 {
			t.Fatal("tagged_dialect_syntax.builtin_dialect_tags must list approved builtin dialect tags")
		}
		return tags
	}
	t.Fatal("feature_matrix.json missing tagged_dialect_syntax feature")
	return nil
}

func readArchitectureFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func stringInSlice(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func stringLiteralValue(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	return strings.Trim(lit.Value, `"`), true
}

func relativeToRoot(t *testing.T, path string) string {
	t.Helper()
	rel, err := filepath.Rel(repoRoot(t), path)
	if err != nil {
		return path
	}
	return rel
}
