package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/never-labs/gscript/internal/modfile"
	"github.com/never-labs/gscript/internal/stdlib/catalog"
)

type modGraphReport struct {
	SchemaVersion int             `json:"schema_version"`
	Root          string          `json:"root"`
	Files         []modGraphFile  `json:"files"`
	Diagnostics   []modDiagnostic `json:"diagnostics,omitempty"`
}

type modGraphFile struct {
	File     string   `json:"file"`
	Requires []string `json:"requires,omitempty"`
}

type modVerifyReport struct {
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Manifest      string          `json:"manifest,omitempty"`
	Graph         modGraphReport  `json:"graph"`
	Diagnostics   []modDiagnostic `json:"diagnostics,omitempty"`
}

type modTidyReport struct {
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Manifest      string          `json:"manifest,omitempty"`
	Removed       []string        `json:"removed,omitempty"`
	Missing       []string        `json:"missing,omitempty"`
	Diagnostics   []modDiagnostic `json:"diagnostics,omitempty"`
}

type modDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

var requireStringRE = regexp.MustCompile(`require\s*\(\s*"([^"]+)"\s*\)`)

func runModCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: gscript mod [init|add|tidy|graph|verify] [flags]")
		return 2
	}
	switch args[0] {
	case "init":
		return runModInitCommand(args[1:], outw, errw)
	case "add":
		return runModAddCommand(args[1:], outw, errw)
	case "tidy":
		return runModTidyCommand(args[1:], outw, errw)
	case "graph":
		return runModGraphCommand(args[1:], outw, errw)
	case "verify":
		return runModVerifyCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: gscript mod [init|add|tidy|graph|verify] [flags]")
		return 0
	default:
		fmt.Fprintf(errw, "gscript mod: unknown mode %q (want init, add, tidy, graph, or verify)\n", args[0])
		return 2
	}
}

func runModInitCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod init", flag.ContinueOnError)
	fs.SetOutput(errw)
	module := fs.String("module", "", "module path")
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript mod init [--module PATH] [--dir DIR]")
		return 2
	}
	absDir, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintf(errw, "gscript mod init: %v\n", err)
		return 1
	}
	if *module == "" {
		*module = filepath.Base(absDir)
	}
	if err := os.MkdirAll(absDir, 0755); err != nil {
		fmt.Fprintf(errw, "gscript mod init: %v\n", err)
		return 1
	}
	path := filepath.Join(absDir, modfile.FileName)
	if _, err := os.Stat(path); err == nil {
		fmt.Fprintf(errw, "gscript mod init: %s already exists\n", path)
		return 1
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(errw, "gscript mod init: %v\n", err)
		return 1
	}
	data := modfile.Format(modfile.File{Module: *module, GS: "0.1"})
	if err := os.WriteFile(path, data, 0644); err != nil {
		fmt.Fprintf(errw, "gscript mod init: %v\n", err)
		return 1
	}
	fmt.Fprintln(outw, path)
	return 0
}

func runModAddCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod add", flag.ContinueOnError)
	fs.SetOutput(errw)
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	targets := fs.Args()
	if len(targets) == 0 {
		fmt.Fprintln(errw, "usage: gscript mod add [--dir DIR] PATH@VERSION [...]")
		return 2
	}
	manifest, path, err := readModFileWithPath(*dir)
	if err != nil {
		fmt.Fprintf(errw, "gscript mod add: %v\n", err)
		return 1
	}
	for _, target := range targets {
		req, err := parseRequireTarget(target)
		if err != nil {
			fmt.Fprintf(errw, "gscript mod add: %v\n", err)
			return 2
		}
		manifest, err = modfile.AddRequire(manifest, req)
		if err != nil {
			fmt.Fprintf(errw, "gscript mod add: %v\n", err)
			return 2
		}
	}
	if err := writeModFile(path, manifest); err != nil {
		fmt.Fprintf(errw, "gscript mod add: %v\n", err)
		return 1
	}
	fmt.Fprintln(outw, path)
	return 0
}

func runModTidyCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod tidy", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print tidy report as JSON")
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript mod tidy [--json] [--dir DIR]")
		return 2
	}
	report := tidyModule(*dir)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript mod tidy: %v\n", err)
			return 1
		}
	} else {
		for _, removed := range report.Removed {
			fmt.Fprintf(outw, "removed %s\n", removed)
		}
		for _, missing := range report.Missing {
			fmt.Fprintf(errw, "missing require %s\n", missing)
		}
		for _, diag := range report.Diagnostics {
			fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
		}
		if report.OK {
			fmt.Fprintf(outw, "ok: %s\n", report.Manifest)
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}

func runModGraphCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod graph", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print module graph as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: gscript mod graph [--json] [path]")
		return 2
	}
	report, err := buildModGraph(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			fmt.Fprintf(errw, "gscript mod graph: %v\n", encErr)
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}
	for _, file := range report.Files {
		for _, req := range file.Requires {
			fmt.Fprintf(outw, "%s -> %s\n", file.File, req)
		}
	}
	for _, diag := range report.Diagnostics {
		fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
	}
	if err != nil {
		return 1
	}
	return 0
}

func runModVerifyCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod verify", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print verification as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: gscript mod verify [--json] [path]")
		return 2
	}
	report := verifyModule(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript mod verify: %v\n", err)
			return 1
		}
	} else {
		if report.OK {
			fmt.Fprintf(outw, "ok: %s\n", report.Manifest)
		}
		for _, diag := range report.Diagnostics {
			if diag.File != "" {
				fmt.Fprintf(errw, "%s: %s %s: %s\n", diag.File, diag.Severity, diag.Code, diag.Message)
			} else {
				fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
			}
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}

func buildModGraph(path string) (modGraphReport, error) {
	abs, err := filepath.Abs(path)
	report := modGraphReport{SchemaVersion: 1, Root: abs}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report, err
	}
	files, err := gscriptFiles(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report, err
	}
	for _, file := range files {
		requires, err := scanStaticRequires(file)
		if err != nil {
			report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9102", Message: err.Error(), File: file})
			continue
		}
		rel, relErr := filepath.Rel(abs, file)
		if relErr != nil {
			rel = file
		}
		report.Files = append(report.Files, modGraphFile{File: filepath.ToSlash(rel), Requires: requires})
	}
	if len(report.Diagnostics) > 0 {
		return report, errors.New("module graph has diagnostics")
	}
	return report, nil
}

func scanStaticRequires(file string) ([]string, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var requires []string
	for _, match := range requireStringRE.FindAllStringSubmatch(string(src), -1) {
		req := strings.TrimSpace(match[1])
		if req == "" || seen[req] {
			continue
		}
		seen[req] = true
		requires = append(requires, req)
	}
	sort.Strings(requires)
	return requires, nil
}

func verifyModule(path string) modVerifyReport {
	abs, err := filepath.Abs(path)
	report := modVerifyReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifestPath := filepath.Join(abs, modfile.FileName)
	report.Manifest = manifestPath
	manifest, err := readModFile(abs)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		report.Graph, _ = buildModGraph(abs)
		return report
	}
	report.Graph, _ = buildModGraph(abs)
	if strings.TrimSpace(manifest.Module) == "" {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9104", Message: "module is required", File: manifestPath})
	}
	for _, diag := range report.Graph.Diagnostics {
		report.Diagnostics = append(report.Diagnostics, diag)
	}
	for _, diag := range verifyModDependencies(abs, manifest, report.Graph) {
		report.Diagnostics = append(report.Diagnostics, diag)
	}
	report.OK = len(report.Diagnostics) == 0
	return report
}

func tidyModule(path string) modTidyReport {
	abs, err := filepath.Abs(path)
	report := modTidyReport{SchemaVersion: 1}
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9101", Message: err.Error()})
		return report
	}
	manifest, manifestPath, err := readModFileWithPath(abs)
	report.Manifest = manifestPath
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9103", Message: err.Error(), File: manifestPath})
		return report
	}
	graph, graphErr := buildModGraph(abs)
	if graphErr != nil {
		report.Diagnostics = append(report.Diagnostics, graph.Diagnostics...)
		return report
	}
	used := externalRequires(graph, manifest)
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
		if err := writeModFile(manifestPath, manifest); err != nil {
			report.Diagnostics = append(report.Diagnostics, modDiagnostic{Severity: "error", Code: "GS9105", Message: err.Error(), File: manifestPath})
		}
	}
	report.OK = len(report.Diagnostics) == 0 && len(report.Missing) == 0
	return report
}

func readModFile(dir string) (modfile.File, error) {
	file, _, err := readModFileWithPath(dir)
	return file, err
}

func readModFileWithPath(dir string) (modfile.File, string, error) {
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

func writeModFile(path string, file modfile.File) error {
	return os.WriteFile(path, modfile.Format(file), 0644)
}

func parseRequireTarget(target string) (modfile.Require, error) {
	idx := strings.LastIndex(target, "@")
	if idx <= 0 || idx == len(target)-1 {
		return modfile.Require{}, fmt.Errorf("require target %q must be PATH@VERSION", target)
	}
	return modfile.Require{Path: target[:idx], Version: target[idx+1:]}, nil
}

func externalRequires(graph modGraphReport, manifest modfile.File) []string {
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

func verifyModDependencies(root string, manifest modfile.File, graph modGraphReport) []modDiagnostic {
	var diags []modDiagnostic
	required := map[string]bool{}
	for _, req := range manifest.Require {
		required[req.Path] = true
	}
	for _, used := range externalRequires(graph, manifest) {
		if !coveredByRequire(used, required) {
			diags = append(diags, modDiagnostic{
				Severity: "error",
				Code:     "GS9106",
				Message:  fmt.Sprintf("missing require for %s; run gscript mod add %s@VERSION", used, used),
			})
		}
	}
	for _, col := range manifest.Collections {
		if err := verifyLocalModPath(root, col.Path); err != nil {
			diags = append(diags, modDiagnostic{
				Severity: "error",
				Code:     "GS9107",
				Message:  fmt.Sprintf("collection %s path %s: %v", col.Name, col.Path, err),
			})
		}
	}
	for _, rep := range manifest.Replace {
		if isLocalModPath(rep.NewPath) {
			if err := verifyLocalModPath(root, rep.NewPath); err != nil {
				diags = append(diags, modDiagnostic{
					Severity: "error",
					Code:     "GS9107",
					Message:  fmt.Sprintf("replace %s path %s: %v", rep.Path, rep.NewPath, err),
				})
			}
		}
	}
	return diags
}

func isLocalModPath(path string) bool {
	return strings.HasPrefix(path, ".") || strings.HasPrefix(path, string(os.PathSeparator))
}

func verifyLocalModPath(root, path string) error {
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
