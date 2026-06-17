// Package modpkg implements local Leia module maintenance.
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

	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/modfile"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/stdlib/catalog"
	"github.com/never-labs/leia/internal/support/modresolve"
	toolsource "github.com/never-labs/leia/internal/support/source"
)

const SumFileName = "leia.sum"

type GraphReport struct {
	SchemaVersion   int          `json:"schema_version"`
	OK              bool         `json:"ok"`
	Root            string       `json:"root"`
	FileCount       int          `json:"file_count"`
	DiagnosticCount int          `json:"diagnostic_count"`
	Files           []GraphFile  `json:"files"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type GraphFile struct {
	File     string   `json:"file"`
	Requires []string `json:"requires,omitempty"`
}

type VerifyReport struct {
	SchemaVersion   int          `json:"schema_version"`
	OK              bool         `json:"ok"`
	Manifest        string       `json:"manifest,omitempty"`
	Graph           GraphReport  `json:"graph"`
	DiagnosticCount int          `json:"diagnostic_count"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type TidyReport struct {
	SchemaVersion   int          `json:"schema_version"`
	OK              bool         `json:"ok"`
	Manifest        string       `json:"manifest,omitempty"`
	RemovedCount    int          `json:"removed_count"`
	MissingCount    int          `json:"missing_count"`
	DiagnosticCount int          `json:"diagnostic_count"`
	Removed         []string     `json:"removed"`
	Missing         []string     `json:"missing"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type ExplainReport struct {
	SchemaVersion   int          `json:"schema_version"`
	OK              bool         `json:"ok"`
	Module          string       `json:"module"`
	Kind            string       `json:"kind,omitempty"`
	Path            string       `json:"path,omitempty"`
	Root            string       `json:"root,omitempty"`
	File            string       `json:"file,omitempty"`
	DiagnosticCount int          `json:"diagnostic_count"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type ListReport struct {
	SchemaVersion   int              `json:"schema_version"`
	OK              bool             `json:"ok"`
	Manifest        string           `json:"manifest,omitempty"`
	Module          string           `json:"module,omitempty"`
	Leia            string           `json:"leia,omitempty"`
	RequireCount    int              `json:"require_count"`
	ReplaceCount    int              `json:"replace_count"`
	CollectionCount int              `json:"collection_count"`
	DiagnosticCount int              `json:"diagnostic_count"`
	Requires        []ListRequire    `json:"requires"`
	Replaces        []ListReplace    `json:"replaces"`
	Collections     []ListCollection `json:"collections"`
	Diagnostics     []Diagnostic     `json:"diagnostics"`
}

type ListRequire struct {
	Path    string `json:"path"`
	Version string `json:"version"`
	Kind    string `json:"kind"`
	Source  string `json:"source,omitempty"`
	File    string `json:"file,omitempty"`
}

type ListReplace struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
	NewPath string `json:"new_path"`
	Local   bool   `json:"local"`
	Root    string `json:"root,omitempty"`
}

type ListCollection struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Root string `json:"root"`
}

type CapabilityReport struct {
	SchemaVersion   int                        `json:"schema_version"`
	OK              bool                       `json:"ok"`
	Manifest        string                     `json:"manifest,omitempty"`
	CapabilityCount int                        `json:"capability_count"`
	ModuleCount     int                        `json:"module_count"`
	DiagnosticCount int                        `json:"diagnostic_count"`
	Capabilities    []string                   `json:"capabilities"`
	Modules         []CapabilityModule         `json:"modules"`
	Matrix          map[string]map[string]bool `json:"matrix"`
	Diagnostics     []Diagnostic               `json:"diagnostics"`
}

type GoModReport struct {
	SchemaVersion   int          `json:"schema_version"`
	OK              bool         `json:"ok"`
	Manifest        string       `json:"manifest,omitempty"`
	GoMod           string       `json:"go_mod,omitempty"`
	Content         string       `json:"content,omitempty"`
	Written         bool         `json:"written,omitempty"`
	DiagnosticCount int          `json:"diagnostic_count"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

type CapabilityModule struct {
	Path         string   `json:"path"`
	Version      string   `json:"version,omitempty"`
	Kind         string   `json:"kind"`
	Root         string   `json:"root,omitempty"`
	Manifest     string   `json:"manifest,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
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

type VerifyOptions struct {
	CacheDir string
}

type SumReport struct {
	SchemaVersion   int          `json:"schema_version"`
	OK              bool         `json:"ok"`
	Sum             string       `json:"sum,omitempty"`
	EntryCount      int          `json:"entry_count"`
	DiagnosticCount int          `json:"diagnostic_count"`
	Entries         []SumEntry   `json:"entries"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
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
	if err := validateModulePath(module); err != nil {
		return filepath.Join(absDir, modfile.FileName), err
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
	data := modfile.Format(modfile.File{Module: module, Leia: "0.1"})
	if err := os.WriteFile(path, data, 0644); err != nil {
		return path, err
	}
	return path, nil
}

func validateModulePath(module string) error {
	if modfile.ValidModulePath(module) {
		return nil
	}
	return errors.New("invalid module path")
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
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		setGraphReportCounts(&report)
		return report, err
	}
	files, err := toolsource.Files(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		setGraphReportCounts(&report)
		return report, err
	}
	excludes := graphExcludeRoots(abs)
	for _, file := range files {
		if isUnderAnyPath(file, excludes) {
			continue
		}
		requires, err := ScanStaticRequires(file)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9102", Message: err.Error(), File: file})
			continue
		}
		rel, relErr := filepath.Rel(abs, file)
		if relErr != nil {
			rel = file
		}
		report.Files = append(report.Files, GraphFile{File: filepath.ToSlash(rel), Requires: requires})
	}
	setGraphReportCounts(&report)
	if len(report.Diagnostics) > 0 {
		return report, errors.New("module graph has diagnostics")
	}
	return report, nil
}

func setGraphReportCounts(report *GraphReport) {
	report.FileCount = len(report.Files)
	report.DiagnosticCount = len(report.Diagnostics)
	report.OK = report.DiagnosticCount == 0
	if report.Files == nil {
		report.Files = []GraphFile{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func graphExcludeRoots(root string) []string {
	manifest, _, err := ReadFileWithPath(root)
	if err != nil {
		return nil
	}
	excludes := []string{filepath.Join(root, "vendor")}
	for _, col := range manifest.Collections {
		excludes = append(excludes, cleanLocalRoot(root, col.Path))
	}
	for _, rep := range manifest.Replace {
		if isLocalPath(rep.NewPath) {
			excludes = append(excludes, cleanLocalRoot(root, rep.NewPath))
		}
	}
	return excludes
}

func isUnderAnyPath(path string, roots []string) bool {
	path = filepath.Clean(path)
	for _, root := range roots {
		root = filepath.Clean(root)
		if root == "." || root == string(os.PathSeparator) {
			continue
		}
		if path == root {
			return true
		}
		rel, err := filepath.Rel(root, path)
		if err == nil && rel != "." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != ".." {
			return true
		}
	}
	return false
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
	case *ast.BlockStmt:
		return collectBlockRequires(out, s)
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
	case *ast.InterpolatedStringExpr:
		for _, part := range e.Parts {
			out = collectExprRequires(out, part.Expr)
		}
		return out
	case *ast.TaggedStringExpr:
		return collectExprRequires(out, e.Body)
	case *ast.TaggedBlockExpr:
		out = collectConfigRequires(out, e.Config)
		return collectBlockRequires(out, e.Body)
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
	return VerifyWithOptions(path, VerifyOptions{})
}

func VerifyWithOptions(path string, opts VerifyOptions) (report VerifyReport) {
	abs, err := filepath.Abs(path)
	report = VerifyReport{SchemaVersion: 1}
	defer setVerifyReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifestPath := filepath.Join(abs, modfile.FileName)
	report.Manifest = manifestPath
	manifest, err := ReadFile(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
		report.Graph, _ = Graph(abs)
		return report
	}
	report.Graph, _ = Graph(abs)
	if strings.TrimSpace(manifest.Module) == "" {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9104", Message: "module is required", File: manifestPath})
	}
	report.Diagnostics = append(report.Diagnostics, report.Graph.Diagnostics...)
	report.Diagnostics = append(report.Diagnostics, verifyDependencies(abs, manifest, report.Graph)...)
	report.Diagnostics = append(report.Diagnostics, verifyTransitiveDependencies(abs, manifest, opts)...)
	report.Diagnostics = append(report.Diagnostics, VerifySumWithOptions(abs, opts)...)
	report.OK = len(report.Diagnostics) == 0
	return report
}

func setVerifyReportCounts(report *VerifyReport) {
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func Tidy(path string) (report TidyReport) {
	abs, err := filepath.Abs(path)
	report = TidyReport{SchemaVersion: 1}
	defer setTidyReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
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
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9105", Message: err.Error(), File: manifestPath})
		}
	}
	report.OK = len(report.Diagnostics) == 0 && len(report.Missing) == 0
	return report
}

func setTidyReportCounts(report *TidyReport) {
	report.RemovedCount = len(report.Removed)
	report.MissingCount = len(report.Missing)
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Removed == nil {
		report.Removed = []string{}
	}
	if report.Missing == nil {
		report.Missing = []string{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func Explain(path, module string) (report ExplainReport) {
	abs, err := filepath.Abs(path)
	report = ExplainReport{SchemaVersion: 1, Module: module}
	defer setExplainReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
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

func setExplainReportCounts(report *ExplainReport) {
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func List(path string) (report ListReport) {
	abs, err := filepath.Abs(path)
	report = ListReport{SchemaVersion: 1}
	defer setListReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
		return report
	}
	report.Module = manifest.Module
	report.Leia = manifest.Leia

	collections := moduleCollections(abs, manifest)
	replaces := moduleReplaces(abs, manifest)
	cacheModules := listVendorModules(abs, manifest)
	if cacheDir, err := ModuleCacheDir(""); err == nil {
		cacheModules = append(cacheModules, listCacheModules(cacheDir, manifest)...)
	}
	for _, req := range manifest.Require {
		result := modresolve.ResolveWithCache(req.Path, collections, replaces, cacheModules, abs)
		item := ListRequire{
			Path:    req.Path,
			Version: req.Version,
			Kind:    result.Kind,
			Source:  result.Path,
			File:    result.File,
		}
		if item.Source == "" && result.Kind == "module" {
			item.Source = req.Path
		}
		report.Requires = append(report.Requires, item)
	}
	for _, rep := range manifest.Replace {
		item := ListReplace{
			Path:    rep.Path,
			Version: rep.Version,
			NewPath: rep.NewPath,
			Local:   isLocalPath(rep.NewPath),
		}
		if item.Local {
			item.Root = cleanLocalRoot(abs, rep.NewPath)
		}
		report.Replaces = append(report.Replaces, item)
	}
	for _, col := range manifest.Collections {
		report.Collections = append(report.Collections, ListCollection{
			Name: col.Name,
			Path: col.Path,
			Root: cleanLocalRoot(abs, col.Path),
		})
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func setListReportCounts(report *ListReport) {
	report.RequireCount = len(report.Requires)
	report.ReplaceCount = len(report.Replaces)
	report.CollectionCount = len(report.Collections)
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Requires == nil {
		report.Requires = []ListRequire{}
	}
	if report.Replaces == nil {
		report.Replaces = []ListReplace{}
	}
	if report.Collections == nil {
		report.Collections = []ListCollection{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func Capability(path string) (report CapabilityReport) {
	abs, err := filepath.Abs(path)
	report = CapabilityReport{SchemaVersion: 1}
	defer setCapabilityReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
		return report
	}
	report.Modules = append(report.Modules, CapabilityModule{
		Path:         manifest.Module,
		Kind:         "main",
		Root:         abs,
		Manifest:     manifestPath,
		Capabilities: sortedStrings(manifest.Capability),
	})

	for _, dep := range dependencyClosure(abs, manifest, "") {
		req := dep.Require
		module := CapabilityModule{Path: req.Path, Version: req.Version, Kind: "module"}
		if dep.Kind == "" || dep.Root == "" {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				Severity: "warning",
				Code:     "LEIA9114",
				Message:  fmt.Sprintf("%s@%s is not available locally; run leia mod download or add a local replace/vendor copy", req.Path, req.Version),
			})
			report.Modules = append(report.Modules, module)
			continue
		}
		module.Kind = dep.Kind
		module.Root = dep.Root
		depManifestPath := filepath.Join(dep.Root, modfile.FileName)
		depManifest, err := ReadFile(dep.Root)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{
				Severity: "warning",
				Code:     "LEIA9115",
				Message:  fmt.Sprintf("%s@%s manifest is unavailable: %v", req.Path, req.Version, err),
				File:     depManifestPath,
			})
			report.Modules = append(report.Modules, module)
			continue
		}
		module.Manifest = depManifestPath
		module.Capabilities = sortedStrings(depManifest.Capability)
		report.Modules = append(report.Modules, module)
	}
	report.Capabilities = capabilityUniverse(report.Modules)
	report.Matrix = capabilityMatrix(report.Modules, report.Capabilities)
	report.OK = !hasErrorDiagnostic(report.Diagnostics)
	return report
}

func setCapabilityReportCounts(report *CapabilityReport) {
	report.CapabilityCount = len(report.Capabilities)
	report.ModuleCount = len(report.Modules)
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Capabilities == nil {
		report.Capabilities = []string{}
	}
	if report.Modules == nil {
		report.Modules = []CapabilityModule{}
	}
	if report.Matrix == nil {
		report.Matrix = map[string]map[string]bool{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func dependencyRoot(root string, manifest modfile.File, req modfile.Require) (string, string, bool) {
	return dependencyRootWithCache(root, manifest, req, "")
}

func dependencyRootWithCache(root string, manifest modfile.File, req modfile.Require, cacheDir string) (string, string, bool) {
	if repRoot, ok := replacementRoot(root, manifest, req); ok {
		return repRoot, "replace", true
	}
	vendorRoot := filepath.Join(root, "vendor", filepath.FromSlash(req.Path+"@"+req.Version))
	if _, err := os.Stat(vendorRoot); err == nil {
		return vendorRoot, "vendor", true
	}
	if cacheDir != "" {
		cacheRoot := cachedRequirementRoot(cacheDir, req.Path, req.Version)
		if _, statErr := os.Stat(cacheRoot); statErr == nil {
			return cacheRoot, "cache", true
		}
		return "", "", false
	}
	if defaultCacheDir, err := ModuleCacheDir(""); err == nil {
		cacheRoot := cachedRequirementRoot(defaultCacheDir, req.Path, req.Version)
		if _, statErr := os.Stat(cacheRoot); statErr == nil {
			return cacheRoot, "cache", true
		}
	}
	return "", "", false
}

func replacementRoot(root string, manifest modfile.File, req modfile.Require) (string, bool) {
	var best modfile.Replace
	for _, rep := range manifest.Replace {
		if rep.Path != req.Path {
			continue
		}
		if rep.Version != "" && rep.Version != req.Version {
			continue
		}
		if !isLocalPath(rep.NewPath) || len(rep.Path) < len(best.Path) {
			continue
		}
		best = rep
	}
	if best.Path == "" {
		return "", false
	}
	repRoot := cleanLocalRoot(root, best.NewPath)
	if filepath.Ext(repRoot) == ".leia" {
		repRoot = filepath.Dir(repRoot)
	}
	return repRoot, true
}

func capabilityUniverse(modules []CapabilityModule) []string {
	seen := map[string]bool{}
	var out []string
	for _, module := range modules {
		for _, cap := range module.Capabilities {
			if seen[cap] {
				continue
			}
			seen[cap] = true
			out = append(out, cap)
		}
	}
	sort.Strings(out)
	return out
}

func capabilityMatrix(modules []CapabilityModule, caps []string) map[string]map[string]bool {
	matrix := make(map[string]map[string]bool, len(modules))
	for _, module := range modules {
		row := make(map[string]bool, len(caps))
		have := map[string]bool{}
		for _, cap := range module.Capabilities {
			have[cap] = true
		}
		for _, cap := range caps {
			row[cap] = have[cap]
		}
		matrix[module.Path] = row
	}
	return matrix
}

func sortedStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func hasErrorDiagnostic(diags []Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity == "error" {
			return true
		}
	}
	return false
}

func listVendorModules(root string, manifest modfile.File) []modresolve.CacheModule {
	modules := make([]modresolve.CacheModule, 0, len(manifest.Require))
	for _, req := range manifest.Require {
		if req.Version == "" {
			continue
		}
		vendorRoot := filepath.Join(root, "vendor", filepath.FromSlash(req.Path+"@"+req.Version))
		if _, err := os.Stat(vendorRoot); err != nil {
			continue
		}
		modules = append(modules, modresolve.CacheModule{Path: req.Path, Version: req.Version, Root: vendorRoot, Kind: "vendor"})
	}
	return modules
}

func listCacheModules(cacheDir string, manifest modfile.File) []modresolve.CacheModule {
	modules := make([]modresolve.CacheModule, 0, len(manifest.Require))
	for _, req := range manifest.Require {
		if req.Version == "" {
			continue
		}
		cacheRoot := cachedRequirementRoot(cacheDir, req.Path, req.Version)
		if _, err := os.Stat(cacheRoot); err != nil {
			continue
		}
		modules = append(modules, modresolve.CacheModule{Path: req.Path, Version: req.Version, Root: cacheRoot, Kind: "cache"})
	}
	return modules
}

func Lock(path string) (report SumReport) {
	abs, err := filepath.Abs(path)
	report = SumReport{SchemaVersion: 1}
	defer setSumReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Sum = filepath.Join(abs, SumFileName)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
		return report
	}
	entries, diags := sumEntries(abs, manifest)
	report.Entries = entries
	report.Diagnostics = append(report.Diagnostics, diags...)
	if len(report.Diagnostics) == 0 {
		if err := writeSumFile(report.Sum, entries); err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9108", Message: err.Error(), File: report.Sum})
		}
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func setSumReportCounts(report *SumReport) {
	report.EntryCount = len(report.Entries)
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Entries == nil {
		report.Entries = []SumEntry{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func VerifySum(path string) []Diagnostic {
	return VerifySumWithOptions(path, VerifyOptions{})
}

func VerifySumWithOptions(path string, opts VerifyOptions) []Diagnostic {
	abs, err := filepath.Abs(path)
	if err != nil {
		return []Diagnostic{{Severity: "error", Code: "LEIA9101", Message: err.Error()}}
	}
	sumPath := filepath.Join(abs, SumFileName)
	if _, err := os.Stat(sumPath); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	if err != nil {
		return []Diagnostic{{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath}}
	}
	want, err := readSumFile(sumPath)
	if err != nil {
		return []Diagnostic{{Severity: "error", Code: "LEIA9108", Message: err.Error(), File: sumPath}}
	}
	got, diags := sumEntriesWithCache(abs, manifest, opts.CacheDir)
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
			out = append(out, Diagnostic{Severity: "error", Code: "LEIA9109", Message: fmt.Sprintf("missing sum entry for %s", entry.Path)})
			continue
		}
		if prev.Hash != entry.Hash {
			out = append(out, Diagnostic{Severity: "error", Code: "LEIA9109", Message: fmt.Sprintf("checksum mismatch for %s", entry.Path)})
		}
	}
	gotMap := map[string]bool{}
	for _, entry := range got {
		gotMap[sumKey(entry)] = true
	}
	for _, entry := range want {
		if entry.Kind == "module" && !gotMap[sumKey(entry)] {
			out = append(out, Diagnostic{Severity: "error", Code: "LEIA9109", Message: fmt.Sprintf("missing cached or vendored module for %s", entry.Path)})
		}
	}
	return out
}

func updateSumFile(path string, updates []SumEntry) error {
	if len(updates) == 0 {
		return nil
	}
	var entries []SumEntry
	if _, err := os.Stat(path); err == nil {
		var readErr error
		entries, readErr = readSumFile(path)
		if readErr != nil {
			return readErr
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	entryMap := make(map[string]SumEntry, len(entries)+len(updates))
	for _, entry := range entries {
		entryMap[sumKey(entry)] = entry
	}
	for _, entry := range updates {
		if prev, ok := entryMap[sumKey(entry)]; ok && prev.Hash != entry.Hash {
			return fmt.Errorf("checksum mismatch for %s", entry.Path)
		}
		entryMap[sumKey(entry)] = entry
	}
	entries = entries[:0]
	for _, entry := range entryMap {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return sumKey(entries[i]) < sumKey(entries[j])
	})
	return writeSumFile(path, entries)
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

func GenerateGoMod(path string, write bool) (report GoModReport) {
	abs, err := filepath.Abs(path)
	report = GoModReport{SchemaVersion: 1}
	defer setGoModReportCounts(&report)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := ReadFileWithPath(abs)
	report.Manifest = manifestPath
	report.GoMod = filepath.Join(abs, "go.mod")
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: manifestPath})
		return report
	}
	content, err := GoModContent(manifest)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9120", Message: err.Error(), File: manifestPath})
		return report
	}
	report.Content = string(content)
	if write {
		if err := os.WriteFile(report.GoMod, content, 0644); err != nil {
			report.Diagnostics = append(report.Diagnostics, Diagnostic{Severity: "error", Code: "LEIA9121", Message: err.Error(), File: report.GoMod})
			return report
		}
		report.Written = true
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func setGoModReportCounts(report *GoModReport) {
	report.DiagnosticCount = len(report.Diagnostics)
	if report.Diagnostics == nil {
		report.Diagnostics = []Diagnostic{}
	}
}

func GoModContent(manifest modfile.File) ([]byte, error) {
	if strings.TrimSpace(manifest.Module) == "" {
		return nil, fmt.Errorf("module is required")
	}
	version := strings.TrimSpace(manifest.Go)
	if version == "" {
		version = "1.25"
	}
	var b strings.Builder
	b.WriteString("// Code generated by leia mod gomod; DO NOT EDIT.\n")
	fmt.Fprintf(&b, "module %s\n\n", manifest.Module)
	fmt.Fprintf(&b, "go %s\n", version)
	if len(manifest.GoRequire) > 0 {
		reqs := append([]modfile.Require(nil), manifest.GoRequire...)
		sort.Slice(reqs, func(i, j int) bool { return reqs[i].Path < reqs[j].Path })
		b.WriteString("\nrequire (\n")
		for _, req := range reqs {
			fmt.Fprintf(&b, "\t%s %s\n", req.Path, req.Version)
		}
		b.WriteString(")\n")
	}
	if len(manifest.GoReplace) > 0 {
		reps := append([]modfile.Replace(nil), manifest.GoReplace...)
		sort.Slice(reps, func(i, j int) bool { return reps[i].Path < reps[j].Path })
		b.WriteByte('\n')
		for _, rep := range reps {
			if rep.Version != "" {
				fmt.Fprintf(&b, "replace %s %s => %s\n", rep.Path, rep.Version, rep.NewPath)
			} else {
				fmt.Fprintf(&b, "replace %s => %s\n", rep.Path, rep.NewPath)
			}
		}
	}
	return []byte(b.String()), nil
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
				Code:     "LEIA9106",
				Message:  fmt.Sprintf("missing require for %s; run leia mod add %s@VERSION", used, used),
			})
		}
	}
	for _, col := range manifest.Collections {
		if err := verifyLocalPath(root, col.Path); err != nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     "LEIA9107",
				Message:  fmt.Sprintf("collection %s path %s: %v", col.Name, col.Path, err),
			})
		}
	}
	for _, rep := range manifest.Replace {
		if isLocalPath(rep.NewPath) {
			if err := verifyLocalPath(root, rep.NewPath); err != nil {
				diags = append(diags, Diagnostic{
					Severity: "error",
					Code:     "LEIA9107",
					Message:  fmt.Sprintf("replace %s path %s: %v", rep.Path, rep.NewPath, err),
				})
			}
		}
	}
	return diags
}

func verifyTransitiveDependencies(root string, manifest modfile.File, opts VerifyOptions) []Diagnostic {
	seen := map[string]bool{}
	return verifyDependencyModules(root, manifest, opts, seen)
}

func verifyDependencyModules(root string, manifest modfile.File, opts VerifyOptions, seen map[string]bool) []Diagnostic {
	var diags []Diagnostic
	for _, req := range manifest.Require {
		depRoot, _, ok := dependencyRootWithCache(root, manifest, req, opts.CacheDir)
		if !ok {
			continue
		}
		depRoot = filepath.Clean(depRoot)
		if seen[depRoot] {
			continue
		}
		seen[depRoot] = true
		depManifest, depManifestPath, err := ReadFileWithPath(depRoot)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			diags = append(diags, Diagnostic{Severity: "error", Code: "LEIA9103", Message: err.Error(), File: depManifestPath})
			continue
		}
		depGraph, graphErr := Graph(depRoot)
		diags = append(diags, depGraph.Diagnostics...)
		diags = append(diags, verifyDependencies(depRoot, depManifest, depGraph)...)
		if graphErr != nil {
			continue
		}
		diags = append(diags, verifyDependencyModules(depRoot, depManifest, opts, seen)...)
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
	return sumEntriesWithCache(root, manifest, "")
}

func sumEntriesWithCache(root string, manifest modfile.File, cacheDir string) ([]SumEntry, []Diagnostic) {
	var entries []SumEntry
	var diags []Diagnostic
	for _, col := range manifest.Collections {
		hash, err := hashModulePath(root, col.Path)
		if err != nil {
			diags = append(diags, Diagnostic{
				Severity: "error",
				Code:     "LEIA9108",
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
				Code:     "LEIA9108",
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
	remoteEntries, remoteDiags := remoteSumEntries(root, manifest, cacheDir)
	entries = append(entries, remoteEntries...)
	diags = append(diags, remoteDiags...)
	sort.Slice(entries, func(i, j int) bool {
		return sumKey(entries[i]) < sumKey(entries[j])
	})
	return entries, diags
}

func remoteSumEntries(root string, manifest modfile.File, cacheDir string) ([]SumEntry, []Diagnostic) {
	requirements := transitiveRequirements(root, manifest, cacheDir)
	hasRemoteRequire := false
	for _, req := range requirements {
		if req.Version != "" {
			hasRemoteRequire = true
			break
		}
	}
	if !hasRemoteRequire {
		return nil, nil
	}
	if cacheDir == "" {
		var err error
		cacheDir, err = ModuleCacheDir("")
		if err != nil {
			return nil, []Diagnostic{{Severity: "error", Code: "LEIA9110", Message: err.Error()}}
		}
	}
	var entries []SumEntry
	var diags []Diagnostic
	for _, req := range requirements {
		if req.Version == "" {
			continue
		}
		roots := remoteModuleRoots(root, cacheDir, req.Path, req.Version)
		if len(roots) == 0 {
			continue
		}
		for _, moduleRoot := range roots {
			hash, err := hashModulePath("", moduleRoot)
			if err != nil {
				diags = append(diags, Diagnostic{
					Severity: "error",
					Code:     "LEIA9108",
					Message:  fmt.Sprintf("module %s@%s path %s: %v", req.Path, req.Version, moduleRoot, err),
					File:     moduleRoot,
				})
				continue
			}
			entries = append(entries, SumEntry{
				Kind:    "module",
				Path:    req.Path,
				Version: req.Version,
				Target:  req.Path + "@" + req.Version,
				Hash:    hash,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return sumKey(entries[i]) < sumKey(entries[j])
	})
	return entries, diags
}

func transitiveRequirements(root string, manifest modfile.File, cacheDir string) []modfile.Require {
	deps := dependencyClosure(root, manifest, cacheDir)
	out := make([]modfile.Require, 0, len(deps))
	for _, dep := range deps {
		out = append(out, dep.Require)
	}
	return out
}

type dependencyRequirement struct {
	Require modfile.Require
	Kind    string
	Root    string
}

func dependencyClosure(root string, manifest modfile.File, cacheDir string) []dependencyRequirement {
	seen := map[string]bool{}
	return appendDependencyClosure(nil, root, manifest, cacheDir, seen)
}

func appendDependencyClosure(out []dependencyRequirement, root string, manifest modfile.File, cacheDir string, seen map[string]bool) []dependencyRequirement {
	for _, req := range manifest.Require {
		key := req.Path + "\x00" + req.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		depRoot, kind, ok := dependencyRootWithCache(root, manifest, req, cacheDir)
		out = append(out, dependencyRequirement{Require: req, Kind: kind, Root: depRoot})
		if !ok {
			continue
		}
		depManifest, _, err := ReadFileWithPath(depRoot)
		if err != nil {
			continue
		}
		out = appendDependencyClosure(out, depRoot, depManifest, cacheDir, seen)
	}
	return out
}

func remoteModuleRoots(root, cacheDir, modulePath, version string) []string {
	var roots []string
	vendorRoot := filepath.Join(root, "vendor", filepath.FromSlash(modulePath+"@"+version))
	if _, err := os.Stat(vendorRoot); err == nil {
		roots = append(roots, vendorRoot)
	}
	cacheRoot := cachedRequirementRoot(cacheDir, modulePath, version)
	if _, err := os.Stat(cacheRoot); err == nil {
		roots = append(roots, cacheRoot)
	}
	return roots
}

func cachedRequirementRoot(cacheDir, modulePath, version string) string {
	if github, ok := parseGitHubModule(modulePath); ok {
		root := filepath.Join(cacheDir, "extract", filepath.FromSlash(github.Repo+"@"+version))
		if github.Subdir != "" {
			root = filepath.Join(root, filepath.FromSlash(github.Subdir))
		}
		return root
	}
	return filepath.Join(cacheDir, "extract", filepath.FromSlash(modulePath+"@"+version))
}

func writeSumFile(path string, entries []SumEntry) error {
	var b strings.Builder
	for _, entry := range entries {
		switch entry.Kind {
		case "collection":
			fmt.Fprintf(&b, "collection %s %s %s\n", entry.Path, entry.Target, entry.Hash)
		case "module":
			fmt.Fprintf(&b, "module %s %s %s %s\n", entry.Path, entry.Version, entry.Target, entry.Hash)
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
		case "module":
			if len(fields) != 5 {
				return nil, fmt.Errorf("%s:%d: module sum entry must be: module PATH VERSION TARGET HASH", path, lineNo+1)
			}
			entries = append(entries, SumEntry{Kind: "module", Path: fields[1], Version: fields[2], Target: fields[3], Hash: fields[4]})
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
		if filepath.Ext(p) != ".leia" && filepath.Base(p) != modfile.FileName {
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
