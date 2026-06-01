// Package modpkg implements local GScript module maintenance.
package modpkg

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/never-labs/gscript/internal/ast"
	"github.com/never-labs/gscript/internal/lexer"
	"github.com/never-labs/gscript/internal/modfile"
	"github.com/never-labs/gscript/internal/parser"
	"github.com/never-labs/gscript/internal/stdlib/catalog"
	"github.com/never-labs/gscript/internal/support/modresolve"
)

const SumFileName = "gscript.sum"

type GraphReport struct {
	SchemaVersion int          `json:"schema_version"`
	Root          string       `json:"root"`
	Files         []GraphFile  `json:"files"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

type GraphFile struct {
	File     string   `json:"file"`
	Requires []string `json:"requires,omitempty"`
}

type VerifyReport struct {
	SchemaVersion int          `json:"schema_version"`
	OK            bool         `json:"ok"`
	Manifest      string       `json:"manifest,omitempty"`
	Graph         GraphReport  `json:"graph"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

type TidyReport struct {
	SchemaVersion int          `json:"schema_version"`
	OK            bool         `json:"ok"`
	Manifest      string       `json:"manifest,omitempty"`
	Removed       []string     `json:"removed,omitempty"`
	Missing       []string     `json:"missing,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

type ExplainReport struct {
	SchemaVersion int          `json:"schema_version"`
	OK            bool         `json:"ok"`
	Module        string       `json:"module"`
	Kind          string       `json:"kind,omitempty"`
	Path          string       `json:"path,omitempty"`
	Root          string       `json:"root,omitempty"`
	File          string       `json:"file,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

type Diagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

type InitOptions struct {
	Module string
	Dir    string
}

type SumReport struct {
	SchemaVersion int          `json:"schema_version"`
	OK            bool         `json:"ok"`
	Sum           string       `json:"sum,omitempty"`
	Entries       []SumEntry   `json:"entries,omitempty"`
	Diagnostics   []Diagnostic `json:"diagnostics,omitempty"`
}

type SumEntry struct {
	Kind    string `json:"kind"`
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	Target  string `json:"target"`
	Hash    string `json:"hash"`
}

func Init(opts InitOptions) (string, error) {
	dir := opts.Dir
	if dir == "" {
		dir = "."
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	module := opts.Module
	if module == "" {
		module = filepath.Base(absDir)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(absDir, modfile.FileName)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("%s already exists", path)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return path, err
	}
	data := modfile.Format(modfile.File{Module: module, GS: "0.1"})
	if err := os.WriteFile(path, data, 0644); err != nil {
		return path, err
	}
	return path, nil
}

func AddRequirements(dir string, targets []string) (string, error) {
	manifest, path, err := ReadFileWithPath(dir)
	if err != nil {
		return path, err
	}
	for _, target := range targets {
		req, err := ParseRequireTarget(target)
		if err != nil {
			return path, err
		}
		manifest, err = modfile.AddRequire(manifest, req)
		if err != nil {
			return path, err
		}
	}
	return path, WriteFile(path, manifest)
}

func Graph(path string) (GraphReport, error) {
	abs, err := filepath.Abs(path)
	report := GraphReport{SchemaVersion: 1, Root: abs}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report, err
	}
	files, err := gscriptFiles(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report, err
	}
	for _, file := range files {
		requires, err := ScanStaticRequires(file)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9102", Message: err.Error(), File: file})
			continue
		}
		rel, relErr := filepath.Rel(abs, file)
		if relErr != nil {
			rel = file
		}
		report.Files = append(report.Files, GraphFile{File: filepath.ToSlash(rel), Requires: requires})
	}
	if len(report.Diagnostics) > 0 {
		return report, errors.New("module graph has diagnostics")
	}
	return report, nil
}

func ScanStaticRequires(file string) ([]string, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return nil, err
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var requires []string
	for _, req := range collectStaticRequires(prog) {
		if req == "" || seen[req] {
			continue
		}
		seen[req] = true
		requires = append(requires, req)
	}
	sort.Strings(requires)
	return requires, nil
}

func collectStaticRequires(prog *ast.Program) []string {
	if prog == nil {
		return nil
	}
	var requires []string
	for _, stmt := range prog.Stmts {
		requires = collectStmtRequires(requires, stmt)
	}
	return requires
}

func collectStmtRequires(out []string, stmt ast.Stmt) []string {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return collectExprListRequires(out, s.Values)
	case *ast.DeclareStmt:
		return collectExprListRequires(out, s.Values)
	case *ast.CompoundAssignStmt:
		return collectExprRequires(out, s.Value)
	case *ast.IncDecStmt:
		return collectExprRequires(out, s.Target)
	case *ast.CallStmt:
		return collectExprRequires(out, s.Call)
	case *ast.GoStmt:
		return collectExprRequires(out, s.Call)
	case *ast.DeferStmt:
		return collectExprRequires(out, s.Call)
	case *ast.SendStmt:
		out = collectExprRequires(out, s.Channel)
		return collectExprRequires(out, s.Value)
	case *ast.SelectStmt:
		for _, c := range s.Cases {
			out = collectExprRequires(out, c.Channel)
			out = collectExprRequires(out, c.SendValue)
			out = collectBlockRequires(out, c.Body)
		}
		return collectBlockRequires(out, s.Default)
	case *ast.IfStmt:
		out = collectExprRequires(out, s.Cond)
		out = collectBlockRequires(out, s.Body)
		for _, ei := range s.ElseIfs {
			out = collectExprRequires(out, ei.Cond)
			out = collectBlockRequires(out, ei.Body)
		}
		return collectBlockRequires(out, s.ElseBody)
	case *ast.ForNumStmt:
		out = collectStmtRequires(out, s.Init)
		out = collectExprRequires(out, s.Cond)
		out = collectStmtRequires(out, s.Post)
		return collectBlockRequires(out, s.Body)
	case *ast.ForRangeStmt:
		out = collectExprRequires(out, s.Iter)
		return collectBlockRequires(out, s.Body)
	case *ast.ForStmt:
		out = collectExprRequires(out, s.Cond)
		return collectBlockRequires(out, s.Body)
	case *ast.ReturnStmt:
		return collectExprListRequires(out, s.Values)
	case *ast.FuncDeclStmt:
		return collectBlockRequires(out, s.Body)
	case *ast.ToolDeclStmt:
		return collectBlockRequires(out, s.Body)
	case *ast.AgentDeclStmt:
		out = collectConfigRequires(out, s.Config)
		return collectBlockRequires(out, s.Flow)
	case *ast.AgentDefaultsDeclStmt:
		return collectConfigRequires(out, s.Config)
	case *ast.ModelsDeclStmt:
		return collectConfigRequires(out, s.Config)
	case *ast.BudgetStmt:
		out = collectConfigRequires(out, s.Config)
		return collectBlockRequires(out, s.Body)
	}
	return out
}

func collectBlockRequires(out []string, block *ast.BlockStmt) []string {
	if block == nil {
		return out
	}
	for _, stmt := range block.Stmts {
		out = collectStmtRequires(out, stmt)
	}
	return out
}

func collectExprListRequires(out []string, exprs []ast.Expr) []string {
	for _, expr := range exprs {
		out = collectExprRequires(out, expr)
	}
	return out
}

func collectExprRequires(out []string, expr ast.Expr) []string {
	switch e := expr.(type) {
	case nil:
		return out
	case *ast.BinaryExpr:
		out = collectExprRequires(out, e.Left)
		return collectExprRequires(out, e.Right)
	case *ast.UnaryExpr:
		return collectExprRequires(out, e.Operand)
	case *ast.ParenExpr:
		return collectExprRequires(out, e.Inner)
	case *ast.IndexExpr:
		out = collectExprRequires(out, e.Table)
		return collectExprRequires(out, e.Index)
	case *ast.FieldExpr:
		return collectExprRequires(out, e.Table)
	case *ast.CallExpr:
		if req, ok := staticRequireCall(e); ok {
			out = append(out, req)
		}
		out = collectExprRequires(out, e.Func)
		return collectExprListRequires(out, e.Args)
	case *ast.MethodCallExpr:
		out = collectExprRequires(out, e.Object)
		return collectExprListRequires(out, e.Args)
	case *ast.FuncLitExpr:
		return collectBlockRequires(out, e.Body)
	case *ast.AgentLitExpr:
		out = collectConfigRequires(out, e.Config)
		return collectBlockRequires(out, e.Flow)
	case *ast.TurnExpr:
		return collectConfigRequires(out, e.Config)
	case *ast.MessagesExpr:
		return collectTableFieldsRequires(out, e.Fields)
	case *ast.ListLitExpr:
		return collectExprListRequires(out, e.Values)
	case *ast.TableLitExpr:
		return collectTableFieldsRequires(out, e.Fields)
	case *ast.DenseLitExpr:
		return collectExprListRequires(out, e.Values)
	case *ast.RecvExpr:
		return collectExprRequires(out, e.Channel)
	case *ast.MakeChanExpr:
		return collectExprRequires(out, e.Size)
	}
	return out
}

func collectTableFieldsRequires(out []string, fields []ast.TableField) []string {
	for _, field := range fields {
		out = collectExprRequires(out, field.Key)
		out = collectExprRequires(out, field.Value)
	}
	return out
}

func collectConfigRequires(out []string, fields []ast.ConfigField) []string {
	for _, field := range fields {
		out = collectExprRequires(out, field.Key)
		out = collectExprRequires(out, field.Value)
	}
	return out
}

func staticRequireCall(call *ast.CallExpr) (string, bool) {
	ident, ok := call.Func.(*ast.IdentExpr)
	if !ok || ident.Name != "require" || len(call.Args) == 0 {
		return "", false
	}
	arg, ok := call.Args[0].(*ast.StringLit)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(arg.Value), true
}

func Verify(path string) VerifyReport {
	abs, err := filepath.Abs(path)
	report := VerifyReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifestPath := filepath.Join(abs, modfile.FileName)
	report.Manifest = manifestPath
	manifest, err := ReadFile(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		report.Graph, _ = Graph(abs)
		return report
	}
	report.Graph, _ = Graph(abs)
	if strings.TrimSpace(manifest.Module) == "" {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9104", Message: "module is required", File: manifestPath})
	}
	report.Diagnostics = append(report.Diagnostics, report.Graph.Diagnostics...)
	report.Diagnostics = append(report.Diagnostics, verifyDependencies(abs, manifest, report.Graph)...)
	report.Diagnostics = append(report.Diagnostics, VerifySum(abs)...)
	report.OK = len(report.Diagnostics) == 0
	return report
}

func Tidy(path string) TidyReport {
	abs, err := filepath.Abs(path)
	report := TidyReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		return report
	}
	graph, graphErr := Graph(abs)
	if graphErr != nil {
		report.Diagnostics = append(report.Diagnostics, graph.Diagnostics...)
		return report
	}
	used := ExternalRequires(graph, manifest)
	required := map[string]bool{}
	for _, req := range manifest.Require {
		required[req.Path] = true
	}
	for _, req := range manifest.Require {
		if !usedByAny(req.Path, used) {
			report.Removed = append(report.Removed, req.Path)
			manifest = modfile.DropRequire(manifest, req.Path)
		}
	}
	for _, usedPath := range used {
		if !coveredByRequire(usedPath, required) {
			report.Missing = append(report.Missing, usedPath)
		}
	}
	sort.Strings(report.Removed)
	sort.Strings(report.Missing)
	if len(report.Missing) == 0 {
		if err := WriteFile(manifestPath, manifest); err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9105", Message: err.Error(), File: manifestPath})
		}
	}
	report.OK = len(report.Diagnostics) == 0 && len(report.Missing) == 0
	return report
}

func Explain(path, module string) ExplainReport {
	abs, err := filepath.Abs(path)
	report := ExplainReport{SchemaVersion: 1, Module: module}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		return report
	}
	if isStdlibModule(module) {
		report.OK = true
		report.Kind = "stdlib"
		report.Path = module
		return report
	}
	result := modresolve.Resolve(module, moduleCollections(abs, manifest), moduleReplaces(abs, manifest), abs)
	report.OK = true
	report.Kind = result.Kind
	report.Path = result.Path
	if report.Kind == "module" {
		report.Path = manifest.Module
	}
	report.Root = result.Root
	report.File = result.File
	return report
}

func Lock(path string) SumReport {
	abs, err := filepath.Abs(path)
	report := SumReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Sum = filepath.Join(abs, SumFileName)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		return report
	}
	entries, diags := sumEntries(abs, manifest)
	report.Entries = entries
	report.Diagnostics = append(report.Diagnostics, diags...)
	if len(report.Diagnostics) == 0 {
		if err := writeSumFile(report.Sum, entries); err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "GS9108", Message: err.Error(), File: report.Sum})
		}
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func VerifySum(path string) []Diagnostic {
	abs, err := filepath.Abs(path)
	if err != nil {
		return []Diagnostic{{Severity: "error", Code: "GS9101", Message: err.Error()}}
	}
	sumPath := filepath.Join(abs, SumFileName)
	if _, err := os.Stat(sumPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	if err != nil {
		return []Diagnostic{{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath}}
	}
	want, err := readSumFile(sumPath)
	if err != nil {
		return []Diagnostic{{Severity: "error", Code: "GS9108", Message: err.Error(), File: sumPath}}
	}
	got, diags := sumEntries(abs, manifest)
	if len(diags) > 0 {
		return diags
	}
	wantMap := map[string]SumEntry{}
	for _, entry := range want {
		wantMap[sumKey(entry)] = entry
	}
	var out []Diagnostic
	for _, entry := range got {
		prev, ok := wantMap[sumKey(entry)]
		if !ok {
			out = append(out, Diagnostic{Severity: "error", Code: "GS9109", Message: fmt.Sprintf("missing sum entry for %s", entry.Path)})
			continue
		}
		if prev.Hash != entry.Hash {
			out = append(out, Diagnostic{Severity: "error", Code: "GS9109", Message: fmt.Sprintf("checksum mismatch for %s", entry.Path)})
		}
	}
	return out
}

func ReadFile(dir string) (modfile.File, error) {
	file, _, err := ReadFileWithPath(dir)
	return file, err
}

func ReadFileWithPath(dir string) (modfile.File, string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return modfile.File{}, filepath.Join(dir, modfile.FileName), err
	}
	path := filepath.Join(abs, modfile.FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return modfile.File{}, path, err
	}
	file, diags := modfile.Parse(path, strings.NewReader(string(data)))
	if len(diags) > 0 {
		parts := make([]string, 0, len(diags))
		for _, diag := range diags {
			if diag.Line > 0 {
				parts = append(parts, fmt.Sprintf("line %d: %s", diag.Line, diag.Message))
			} else {
				parts = append(parts, diag.Message)
			}
		}
		return file, path, errors.New(strings.Join(parts, "; "))
	}
	return file, path, nil
}

func WriteFile(path string, file modfile.File) error {
	return os.WriteFile(path, modfile.Format(file), 0644)
}

func ParseRequireTarget(target string) (modfile.Require, error) {
	idx := strings.LastIndex(target, "@")
	if idx <= 0 || idx == len(target)-1 {
		return modfile.Require{}, fmt.Errorf("require target %q must be PATH@VERSION", target)
	}
	return modfile.Require{Path: target[:idx], Version: target[idx+1:]}, nil
}

func ExternalRequires(graph GraphReport, manifest modfile.File) []string {
	stdlib := map[string]bool{}
	for _, name := range catalog.ModuleNames() {
		stdlib[name] = true
	}
	collections := map[string]bool{}
	for _, col := range manifest.Collections {
		collections[col.Name] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, file := range graph.Files {
		for _, req := range file.Requires {
			if !isExternalRequire(req, stdlib, collections) || seen[req] {
				continue
			}
			seen[req] = true
			out = append(out, req)
		}
	}
	sort.Strings(out)
	return out
}

func isExternalRequire(req string, stdlib, collections map[string]bool) bool {
	if req == "" || strings.HasPrefix(req, ".") || stdlib[req] {
		return false
	}
	if idx := strings.Index(req, ":"); idx > 0 && collections[req[:idx]] {
		return false
	}
	return strings.Contains(req, "/") || strings.Contains(req, ":")
}

func usedByAny(modulePath string, used []string) bool {
	for _, req := range used {
		if req == modulePath || strings.HasPrefix(req, modulePath+"/") {
			return true
		}
	}
	return false
}

func coveredByRequire(req string, required map[string]bool) bool {
	for path := range required {
		if req == path || strings.HasPrefix(req, path+"/") {
			return true
		}
	}
	return false
}

func verifyDependencies(root string, manifest modfile.File, graph GraphReport) []Diagnostic {
	var diags []Diagnostic
	required := map[string]bool{}
	for _, req := range manifest.Require {
		required[req.Path] = true
	}
	for _, used := range ExternalRequires(graph, manifest) {
		if !coveredByRequire(used, required) {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     "GS9106",
				Message:  fmt.Sprintf("missing require for %s; run gscript mod add %s@VERSION", used, used),
			})
		}
	}
	for _, col := range manifest.Collections {
		if err := verifyLocalPath(root, col.Path); err != nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     "GS9107",
				Message:  fmt.Sprintf("collection %s path %s: %v", col.Name, col.Path, err),
			})
		}
	}
	for _, rep := range manifest.Replace {
		if isLocalPath(rep.NewPath) {
			if err := verifyLocalPath(root, rep.NewPath); err != nil {
				diags = append(diags, Diagnostic{
					Severity: "error",
					Code:     "GS9107",
					Message:  fmt.Sprintf("replace %s path %s: %v", rep.Path, rep.NewPath, err),
				})
			}
		}
	}
	return diags
}

func isStdlibModule(name string) bool {
	for _, module := range catalog.ModuleNames() {
		if module == name {
			return true
		}
	}
	return false
}

func moduleCollections(root string, manifest modfile.File) []modresolve.Collection {
	collections := make([]modresolve.Collection, 0, len(manifest.Collections))
	for _, col := range manifest.Collections {
		collections = append(collections, modresolve.Collection{Name: col.Name, Root: cleanLocalRoot(root, col.Path)})
	}
	return collections
}

func moduleReplaces(root string, manifest modfile.File) []modresolve.Replace {
	replaces := make([]modresolve.Replace, 0, len(manifest.Replace))
	for _, rep := range manifest.Replace {
		if isLocalPath(rep.NewPath) {
			replaces = append(replaces, modresolve.Replace{Path: rep.Path, Root: cleanLocalRoot(root, rep.NewPath)})
		}
	}
	return replaces
}

func cleanLocalRoot(root, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(root, path))
}

func sumEntries(root string, manifest modfile.File) ([]SumEntry, []Diagnostic) {
	var entries []SumEntry
	var diags []Diagnostic
	for _, col := range manifest.Collections {
		hash, err := hashModulePath(root, col.Path)
		if err != nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     "GS9108",
				Message:  fmt.Sprintf("collection %s path %s: %v", col.Name, col.Path, err),
			})
			continue
		}
		entries = append(entries, SumEntry{
			Kind:   "collection",
			Path:   col.Name,
			Target: filepath.ToSlash(col.Path),
			Hash:   hash,
		})
	}
	for _, rep := range manifest.Replace {
		if !isLocalPath(rep.NewPath) {
			continue
		}
		hash, err := hashModulePath(root, rep.NewPath)
		if err != nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     "GS9108",
				Message:  fmt.Sprintf("replace %s path %s: %v", rep.Path, rep.NewPath, err),
			})
			continue
		}
		entries = append(entries, SumEntry{
			Kind:    "replace",
			Path:    rep.Path,
			Version: rep.Version,
			Target:  filepath.ToSlash(rep.NewPath),
			Hash:    hash,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return sumKey(entries[i]) < sumKey(entries[j])
	})
	return entries, diags
}

func writeSumFile(path string, entries []SumEntry) error {
	var b strings.Builder
	for _, entry := range entries {
		switch entry.Kind {
		case "collection":
			fmt.Fprintf(&b, "collection %s %s %s\n", entry.Path, entry.Target, entry.Hash)
		case "replace":
			version := entry.Version
			if version == "" {
				version = "-"
			}
			fmt.Fprintf(&b, "replace %s %s %s %s\n", entry.Path, version, entry.Target, entry.Hash)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}

func readSumFile(path string) ([]SumEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []SumEntry
	for lineNo, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "collection":
			if len(fields) != 4 {
				return nil, fmt.Errorf("%s:%d: collection sum entry must be: collection NAME TARGET HASH", path, lineNo+1)
			}
			entries = append(entries, SumEntry{Kind: "collection", Path: fields[1], Target: fields[2], Hash: fields[3]})
		case "replace":
			if len(fields) != 5 {
				return nil, fmt.Errorf("%s:%d: replace sum entry must be: replace PATH VERSION TARGET HASH", path, lineNo+1)
			}
			version := fields[2]
			if version == "-" {
				version = ""
			}
			entries = append(entries, SumEntry{Kind: "replace", Path: fields[1], Version: version, Target: fields[3], Hash: fields[4]})
		default:
			return nil, fmt.Errorf("%s:%d: unknown sum entry kind %q", path, lineNo+1, fields[0])
		}
	}
	return entries, nil
}

func sumKey(entry SumEntry) string {
	return entry.Kind + "\x00" + entry.Path + "\x00" + entry.Version + "\x00" + entry.Target
}

func hashModulePath(root, path string) (string, error) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		h.Write([]byte(filepath.ToSlash(filepath.Base(path))))
		h.Write([]byte{0})
		h.Write(data)
		return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
	}
	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".hg" || name == ".svn" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".gs" && filepath.Base(p) != modfile.FileName {
			return nil
		}
		files = append(files, p)
		return nil
	})
	if err != nil {
		return "", err
	}
	sort.Strings(files)
	for _, file := range files {
		rel, err := filepath.Rel(path, file)
		if err != nil {
			return "", err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		h.Write([]byte(filepath.ToSlash(rel)))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0})
	}
	return "h1:" + base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func isLocalPath(path string) bool {
	return strings.HasPrefix(path, ".") || strings.HasPrefix(path, string(os.PathSeparator))
}

func verifyLocalPath(root, path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

func gscriptFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".gs" {
			return nil, fmt.Errorf("file must have .gs extension")
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
		if filepath.Ext(p) == ".gs" {
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
