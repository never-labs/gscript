package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCommandJSONDiscoversAndParsesProjectConfig(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cmd", "tool")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	config := `[project]
name = "demo"
version = "0.1.0"

[tool.fmt]
indent_width = 4
line_width = 100

[tool.lint]
format = "sarif"

[tool.test]
format = "json"
`
	if err := os.WriteFile(filepath.Join(root, "leia.toml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", nested}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runConfigCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	assertConfigJSONFieldsPresentAndNonNull(t, stdout.String(), "diagnostics")
	if !report.Found || report.Root != root || report.Path != filepath.Join(root, "leia.toml") {
		t.Fatalf("report location = %+v, want discovered root %s", report, root)
	}
	if !report.OK || report.DiagnosticCount != 0 || len(report.Diagnostics) != 0 {
		t.Fatalf("report diagnostics = %+v, want ok config with no diagnostics", report)
	}
	if report.Config == nil {
		t.Fatal("config is nil, want parsed config")
	}
	if report.Config.Project.Name != "demo" || report.Config.Project.Version != "0.1.0" {
		t.Fatalf("project = %+v, want demo 0.1.0", report.Config.Project)
	}
	if report.Config.Tool.Format.IndentWidth != 4 || report.Config.Tool.Format.LineWidth != 100 {
		t.Fatalf("fmt config = %+v, want 4/100", report.Config.Tool.Format)
	}
	if report.Config.Tool.Lint.Format != "sarif" || report.Config.Tool.Test.Format != "json" {
		t.Fatalf("tool config = %+v, want sarif/json", report.Config.Tool)
	}
}

func assertConfigJSONFieldsPresentAndNonNull(t *testing.T, data string, fields ...string) {
	t.Helper()
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		t.Fatalf("config JSON failed to decode as raw object: %v\n%s", err, data)
	}
	for _, field := range fields {
		value, ok := raw[field]
		if !ok {
			t.Fatalf("config JSON missing field %q in %s", field, data)
		}
		if strings.TrimSpace(string(value)) == "null" {
			t.Fatalf("config JSON field %q is null in %s", field, data)
		}
	}
}

func TestConfigCommandReportsMissingConfig(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty for JSON mode", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if report.OK || report.Found || report.DiagnosticCount != 1 || len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "LEIA9001" {
		t.Fatalf("report = %+v, want not found diagnostic", report)
	}
}

func TestConfigCommandJSONReportsWarningsWithoutFailing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "leia.toml"), []byte("[project]\nname = \"demo\"\n\n[unknown]\nvalue = \"ok\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runConfigCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if !report.OK || !report.Found || report.DiagnosticCount != 2 || len(report.Diagnostics) != 2 {
		t.Fatalf("report = %+v, want ok config with two warnings", report)
	}
	for _, diag := range report.Diagnostics {
		if diag.Severity != "warning" {
			t.Fatalf("diagnostic = %+v, want warning", diag)
		}
	}
}

func TestConfigCommandReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "leia.toml"), []byte("[tool.test]\nformat = \"xml\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "LEIA9002") || !strings.Contains(stderr.String(), "tool.test.format") {
		t.Fatalf("stderr = %q, want config parse diagnostic", stderr.String())
	}
}

func TestConfigCommandJSONReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "leia.toml"), []byte("[tool.test]\nformat = \"xml\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in JSON mode", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if report.OK || !report.Found || report.DiagnosticCount != 1 || len(report.Diagnostics) != 1 || report.Diagnostics[0].Severity != "error" || report.Diagnostics[0].Code != "LEIA9002" {
		t.Fatalf("report = %+v, want parse error diagnostic", report)
	}
}
