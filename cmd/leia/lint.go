package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"

	toolsource "github.com/never-labs/leia/internal/support/source"
)

type lintDiagnostic struct {
	File     string `json:"file"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`

	text string
}

var lintDiagnosticPositionRE = regexp.MustCompile(`(?:^|[^0-9])([0-9]+):([0-9]+)(?:[^0-9]|$)`)

func newLintDiagnostic(file, code, severity string, err error) lintDiagnostic {
	diagnostic := lintDiagnostic{
		File:     file,
		Code:     code,
		Severity: severity,
		Message:  err.Error(),
	}
	diagnostic.Line, diagnostic.Column = parseLintDiagnosticPosition(diagnostic.Message)
	return diagnostic
}

func parseLintDiagnosticPosition(message string) (int, int) {
	match := lintDiagnosticPositionRE.FindStringSubmatch(message)
	if match == nil {
		return 0, 0
	}
	line, lineErr := strconv.Atoi(match[1])
	column, columnErr := strconv.Atoi(match[2])
	if lineErr != nil || columnErr != nil {
		return 0, 0
	}
	return line, column
}

func runLintCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(errw)
	format := fs.String("format", "text", "output format: text, json, or sarif")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(errw, "usage: leia lint [--format=text|json|sarif] <path-or-dir> [...]")
		return 2
	}
	if !flagWasSet(fs, "format") {
		config, diagnostics, err := loadOptionalCLIProjectConfig(paths[0])
		if err != nil {
			printCLIConfigDiagnostics(errw, paths[0], diagnostics)
			return 2
		}
		if config != nil && config.Tool.Lint.Format != "" {
			*format = config.Tool.Lint.Format
		}
	}
	if *format != "text" && *format != "json" && *format != "sarif" {
		fmt.Fprintf(errw, "leia lint: unsupported --format %q (want text, json, or sarif)\n", *format)
		return 2
	}

	diagnostics := []lintDiagnostic{}
	for _, path := range paths {
		files, err := toolsource.Files(path)
		if err != nil {
			diagnostic := newLintDiagnostic(path, "LEIA0001", "error", err)
			diagnostic.text = fmt.Sprintf("%s: %v", path, err)
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		for _, filename := range files {
			if err := toolsource.ParseFile(filename); err != nil {
				diagnostics = append(diagnostics, newLintDiagnostic(filename, "LEIA1001", "error", err))
			}
		}
	}

	if *format == "json" {
		if err := json.NewEncoder(outw).Encode(diagnostics); err != nil {
			fmt.Fprintf(errw, "leia lint: write json: %v\n", err)
			return 1
		}
	} else if *format == "sarif" {
		if err := writeLintSARIF(outw, diagnostics); err != nil {
			fmt.Fprintf(errw, "leia lint: write sarif: %v\n", err)
			return 1
		}
	} else {
		for _, diagnostic := range diagnostics {
			if diagnostic.text != "" {
				fmt.Fprintln(errw, diagnostic.text)
				continue
			}
			fmt.Fprintf(errw, "%s: %s %s: %s\n", diagnostic.File, diagnostic.Code, diagnostic.Severity, diagnostic.Message)
		}
	}

	if len(diagnostics) > 0 {
		return 1
	}
	return 0
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	ShortDescription sarifText          `json:"shortDescription"`
	DefaultConfig    sarifDefaultConfig `json:"defaultConfiguration"`
}

type sarifDefaultConfig struct {
	Level string `json:"level"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

func writeLintSARIF(w io.Writer, diagnostics []lintDiagnostic) error {
	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name: "leia lint",
				Rules: []sarifRule{
					{
						ID:               "LEIA0001",
						Name:             "file-discovery",
						ShortDescription: sarifText{Text: "File discovery failed"},
						DefaultConfig:    sarifDefaultConfig{Level: "error"},
					},
					{
						ID:               "LEIA1001",
						Name:             "syntax",
						ShortDescription: sarifText{Text: "Lexer or parser error"},
						DefaultConfig:    sarifDefaultConfig{Level: "error"},
					},
				},
			}},
			Results: make([]sarifResult, 0, len(diagnostics)),
		}},
	}
	for _, diagnostic := range diagnostics {
		result := sarifResult{
			RuleID:  diagnostic.Code,
			Level:   diagnostic.Severity,
			Message: sarifText{Text: diagnostic.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: filepath.ToSlash(diagnostic.File)},
					Region: sarifRegion{
						StartLine:   diagnostic.Line,
						StartColumn: diagnostic.Column,
					},
				},
			}},
		}
		log.Runs[0].Results = append(log.Runs[0].Results, result)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}
