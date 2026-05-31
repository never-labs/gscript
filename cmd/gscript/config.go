package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type cliConfigCapability struct {
	FileName string   `json:"file_name"`
	Sections []string `json:"sections"`
	Formats  []string `json:"formats"`
}

type cliConfigReport struct {
	SchemaVersion int                   `json:"schema_version"`
	Found         bool                  `json:"found"`
	Path          string                `json:"path,omitempty"`
	Root          string                `json:"root,omitempty"`
	Config        *cliProjectConfig     `json:"config,omitempty"`
	Diagnostics   []cliConfigDiagnostic `json:"diagnostics,omitempty"`
}

type cliProjectConfig struct {
	Project cliProjectSection `json:"project,omitempty"`
	Tool    cliToolConfig     `json:"tool,omitempty"`
}

type cliProjectSection struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
}

type cliToolConfig struct {
	Format cliFormatConfig `json:"fmt,omitempty"`
	Lint   cliLintConfig   `json:"lint,omitempty"`
	Test   cliTestConfig   `json:"test,omitempty"`
}

type cliFormatConfig struct {
	IndentWidth int `json:"indent_width,omitempty"`
	LineWidth   int `json:"line_width,omitempty"`
}

type cliLintConfig struct {
	Format string `json:"format,omitempty"`
}

type cliTestConfig struct {
	Format string `json:"format,omitempty"`
}

type cliConfigDiagnostic struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
}

func runConfigCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print resolved configuration as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: gscript config [--json] [path]")
		return 2
	}
	start := "."
	if len(fs.Args()) == 1 {
		start = fs.Args()[0]
	}
	report, err := loadCLIProjectConfig(start)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			fmt.Fprintf(errw, "gscript config: write json: %v\n", encErr)
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}
	if err != nil {
		for _, diag := range report.Diagnostics {
			if diag.Line > 0 {
				fmt.Fprintf(errw, "%s:%d: %s %s: %s\n", report.Path, diag.Line, diag.Severity, diag.Code, diag.Message)
			} else {
				fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
			}
		}
		return 1
	}
	fmt.Fprintf(outw, "path: %s\n", report.Path)
	fmt.Fprintf(outw, "root: %s\n", report.Root)
	if report.Config != nil && report.Config.Project.Name != "" {
		fmt.Fprintf(outw, "project: %s\n", report.Config.Project.Name)
	}
	if report.Config != nil && report.Config.Project.Version != "" {
		fmt.Fprintf(outw, "version: %s\n", report.Config.Project.Version)
	}
	for _, diag := range report.Diagnostics {
		if diag.Line > 0 {
			fmt.Fprintf(errw, "%s:%d: %s %s: %s\n", report.Path, diag.Line, diag.Severity, diag.Code, diag.Message)
		} else {
			fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
		}
	}
	return 0
}

func loadCLIProjectConfig(start string) (cliConfigReport, error) {
	report := cliConfigReport{SchemaVersion: 1}
	configPath, root, err := discoverCLIConfig(start)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, cliConfigDiagnostic{
			Severity: "error",
			Code:     "GS9002",
			Message:  err.Error(),
		})
		return report, err
	}
	if configPath == "" {
		err := errors.New("gscript.toml not found")
		report.Diagnostics = append(report.Diagnostics, cliConfigDiagnostic{
			Severity: "error",
			Code:     "GS9001",
			Message:  err.Error(),
		})
		return report, err
	}
	report.Found = true
	report.Path = configPath
	report.Root = root
	src, err := os.ReadFile(configPath)
	if err != nil {
		report.Diagnostics = append(report.Diagnostics, cliConfigDiagnostic{
			Severity: "error",
			Code:     "GS9002",
			Message:  err.Error(),
		})
		return report, err
	}
	config, diags := parseCLIProjectConfig(src)
	report.Config = &config
	report.Diagnostics = diags
	for _, diag := range diags {
		if diag.Severity == "error" {
			return report, errors.New(diag.Message)
		}
	}
	return report, nil
}

func loadOptionalCLIProjectConfig(start string) (*cliProjectConfig, []cliConfigDiagnostic, error) {
	if _, err := os.Stat(start); err != nil {
		return nil, nil, nil
	}
	report, err := loadCLIProjectConfig(start)
	if err != nil {
		if !report.Found && len(report.Diagnostics) == 1 && report.Diagnostics[0].Code == "GS9001" {
			return nil, nil, nil
		}
		return report.Config, report.Diagnostics, err
	}
	return report.Config, report.Diagnostics, nil
}

func printCLIConfigDiagnostics(errw io.Writer, path string, diagnostics []cliConfigDiagnostic) {
	for _, diag := range diagnostics {
		if diag.Line > 0 && path != "" {
			fmt.Fprintf(errw, "%s:%d: %s %s: %s\n", path, diag.Line, diag.Severity, diag.Code, diag.Message)
		} else {
			fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
		}
	}
}

func discoverCLIConfig(start string) (configPath, root string, err error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", "", err
	}
	dir := abs
	if !info.IsDir() {
		dir = filepath.Dir(abs)
	}
	for {
		candidate := filepath.Join(dir, "gscript.toml")
		if st, statErr := os.Stat(candidate); statErr == nil && !st.IsDir() {
			return candidate, dir, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", nil
		}
		dir = parent
	}
}

func parseCLIProjectConfig(src []byte) (cliProjectConfig, []cliConfigDiagnostic) {
	var config cliProjectConfig
	var diags []cliConfigDiagnostic
	section := ""
	scanner := bufio.NewScanner(bytes.NewReader(src))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(stripCLIConfigComment(scanner.Text()))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			switch section {
			case "project", "tool.fmt", "tool.lint", "tool.test":
			default:
				diags = append(diags, cliConfigDiagnostic{
					Severity: "warning",
					Code:     "GS9003",
					Message:  "unknown config section [" + section + "]",
					Line:     lineNo,
				})
			}
			continue
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			diags = append(diags, cliConfigDiagnostic{
				Severity: "error",
				Code:     "GS9002",
				Message:  "expected key = value",
				Line:     lineNo,
			})
			continue
		}
		key = strings.TrimSpace(key)
		raw = strings.TrimSpace(raw)
		if key == "" || raw == "" {
			diags = append(diags, cliConfigDiagnostic{
				Severity: "error",
				Code:     "GS9002",
				Message:  "expected non-empty key and value",
				Line:     lineNo,
			})
			continue
		}
		assignCLIConfigValue(&config, section, key, raw, lineNo, &diags)
	}
	if err := scanner.Err(); err != nil {
		diags = append(diags, cliConfigDiagnostic{
			Severity: "error",
			Code:     "GS9002",
			Message:  err.Error(),
		})
	}
	return config, diags
}

func stripCLIConfigComment(line string) string {
	inString := false
	escaped := false
	for i, r := range line {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && inString {
			escaped = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if r == '#' && !inString {
			return line[:i]
		}
	}
	return line
}

func assignCLIConfigValue(config *cliProjectConfig, section, key, raw string, lineNo int, diags *[]cliConfigDiagnostic) {
	switch section {
	case "project":
		switch key {
		case "name":
			if value, ok := parseCLIConfigString(raw, lineNo, diags); ok {
				config.Project.Name = value
			}
		case "version":
			if value, ok := parseCLIConfigString(raw, lineNo, diags); ok {
				config.Project.Version = value
			}
		default:
			appendUnknownCLIConfigKey(diags, section, key, lineNo)
		}
	case "tool.fmt":
		switch key {
		case "indent_width":
			if value, ok := parseCLIConfigInt(raw, lineNo, diags); ok {
				config.Tool.Format.IndentWidth = value
			}
		case "line_width":
			if value, ok := parseCLIConfigInt(raw, lineNo, diags); ok {
				config.Tool.Format.LineWidth = value
			}
		default:
			appendUnknownCLIConfigKey(diags, section, key, lineNo)
		}
	case "tool.lint":
		switch key {
		case "format":
			if value, ok := parseCLIConfigString(raw, lineNo, diags); ok {
				if value != "text" && value != "json" && value != "sarif" {
					appendInvalidCLIConfigValue(diags, "tool.lint.format must be text, json, or sarif", lineNo)
					return
				}
				config.Tool.Lint.Format = value
			}
		default:
			appendUnknownCLIConfigKey(diags, section, key, lineNo)
		}
	case "tool.test":
		switch key {
		case "format":
			if value, ok := parseCLIConfigString(raw, lineNo, diags); ok {
				if value != "text" && value != "json" {
					appendInvalidCLIConfigValue(diags, "tool.test.format must be text or json", lineNo)
					return
				}
				config.Tool.Test.Format = value
			}
		default:
			appendUnknownCLIConfigKey(diags, section, key, lineNo)
		}
	default:
		appendUnknownCLIConfigKey(diags, section, key, lineNo)
	}
}

func parseCLIConfigString(raw string, lineNo int, diags *[]cliConfigDiagnostic) (string, bool) {
	value, err := strconv.Unquote(raw)
	if err != nil {
		appendInvalidCLIConfigValue(diags, "expected quoted string", lineNo)
		return "", false
	}
	return value, true
}

func parseCLIConfigInt(raw string, lineNo int, diags *[]cliConfigDiagnostic) (int, bool) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		appendInvalidCLIConfigValue(diags, "expected integer", lineNo)
		return 0, false
	}
	if value <= 0 {
		appendInvalidCLIConfigValue(diags, "expected positive integer", lineNo)
		return 0, false
	}
	return value, true
}

func appendUnknownCLIConfigKey(diags *[]cliConfigDiagnostic, section, key string, lineNo int) {
	scope := "top-level"
	if section != "" {
		scope = "[" + section + "]"
	}
	*diags = append(*diags, cliConfigDiagnostic{
		Severity: "warning",
		Code:     "GS9003",
		Message:  "unknown config key " + scope + "." + key,
		Line:     lineNo,
	})
}

func appendInvalidCLIConfigValue(diags *[]cliConfigDiagnostic, message string, lineNo int) {
	*diags = append(*diags, cliConfigDiagnostic{
		Severity: "error",
		Code:     "GS9002",
		Message:  message,
		Line:     lineNo,
	})
}
