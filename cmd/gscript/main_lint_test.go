package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintCommandUsesConfiguredFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.lint]\nformat = \"json\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(path, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want diagnostics failure", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in configured JSON mode", stderr.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not configured JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 || diagnostics[0].File != path || diagnostics[0].Code != "GS1001" {
		t.Fatalf("diagnostics = %+v, want configured JSON parse diagnostic for %s", diagnostics, path)
	}
}

func TestLintLLMSyntaxCoverage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "llm.gs")
	src := llmToolchainCoverageSource()
	if err := os.WriteFile(path, src, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runLintCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want empty", diagnostics)
	}
}

func TestLintReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	out := stderr.String()
	for _, want := range []string{badPath, "GS1001", "parse error"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, okPath) {
		t.Fatalf("stderr = %q, did not want clean file", out)
	}
}

func TestLintJSONReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.File != badPath {
		t.Fatalf("diagnostic file = %q, want %q", got.File, badPath)
	}
	if got.Code != "GS1001" {
		t.Fatalf("diagnostic code = %q, want GS1001", got.Code)
	}
	if got.Severity != "error" {
		t.Fatalf("diagnostic severity = %q, want error", got.Severity)
	}
	if !strings.Contains(got.Message, "parse error") {
		t.Fatalf("diagnostic message = %q, want parse error", got.Message)
	}
	if got.Line != 1 {
		t.Fatalf("diagnostic line = %d, want 1", got.Line)
	}
	if got.Column != 6 {
		t.Fatalf("diagnostic column = %d, want 6", got.Column)
	}
}

func TestLintJSONReportsLexerErrorPosition(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("\"unterminated\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", badPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.File != badPath {
		t.Fatalf("diagnostic file = %q, want %q", got.File, badPath)
	}
	if got.Code != "GS1001" {
		t.Fatalf("diagnostic code = %q, want GS1001", got.Code)
	}
	if !strings.Contains(got.Message, "lexer error") {
		t.Fatalf("diagnostic message = %q, want lexer error", got.Message)
	}
	if got.Line != 1 {
		t.Fatalf("diagnostic line = %d, want 1", got.Line)
	}
	if got.Column != 1 {
		t.Fatalf("diagnostic column = %d, want 1", got.Column)
	}
}

func TestLintJSONReportsEmptyDiagnosticsOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format", "json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runLintCommand code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want empty", diagnostics)
	}
}

func TestLintSARIFReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=sarif", badPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var log sarifLog
	if err := json.Unmarshal(stdout.Bytes(), &log); err != nil {
		t.Fatalf("stdout is not SARIF JSON: %v; stdout = %q", err, stdout.String())
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("SARIF version/runs = %q/%d, want 2.1.0/1", log.Version, len(log.Runs))
	}
	if log.Runs[0].Tool.Driver.Name != "gscript lint" {
		t.Fatalf("tool name = %q, want gscript lint", log.Runs[0].Tool.Driver.Name)
	}
	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(log.Runs[0].Results))
	}
	result := log.Runs[0].Results[0]
	if result.RuleID != "GS1001" || result.Level != "error" {
		t.Fatalf("result = %+v, want GS1001 error", result)
	}
	if !strings.Contains(result.Message.Text, "parse error") {
		t.Fatalf("message = %q, want parse error", result.Message.Text)
	}
	if len(result.Locations) != 1 || result.Locations[0].PhysicalLocation.ArtifactLocation.URI != filepath.ToSlash(badPath) {
		t.Fatalf("locations = %+v, want %s", result.Locations, filepath.ToSlash(badPath))
	}
}

func TestLintRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=xml", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runLintCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported --format") {
		t.Fatalf("stderr = %q, want unsupported format diagnostic", stderr.String())
	}
}
