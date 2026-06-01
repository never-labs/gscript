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

type modDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
}

var requireStringRE = regexp.MustCompile(`require\s*\(\s*"([^"]+)"\s*\)`)

func runModCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: gscript mod [init|graph|verify] [flags]")
		return 2
	}
	switch args[0] {
	case "init":
		return runModInitCommand(args[1:], outw, errw)
	case "graph":
		return runModGraphCommand(args[1:], outw, errw)
	case "verify":
		return runModVerifyCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: gscript mod [init|graph|verify] [flags]")
		return 0
	default:
		fmt.Fprintf(errw, "gscript mod: unknown mode %q (want init, graph, or verify)\n", args[0])
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
	report.OK = len(report.Diagnostics) == 0
	return report
}

func readModFile(dir string) (modfile.File, error) {
	path := filepath.Join(dir, modfile.FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return modfile.File{}, err
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
		return file, errors.New(strings.Join(parts, "; "))
	}
	return file, nil
}
