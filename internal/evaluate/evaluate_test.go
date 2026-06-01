package evaluate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunReportsSyntaxSkeletonAndTODOs(t *testing.T) {
	dir := t.TempDir()
	src := `// TODO: wire real eval fixtures later
models { default: "mock" }

// Echoes input.
//leia:requires none
//leia:param text input text
tool echo(text) {
    return text, nil
}

agent answer(question) {
    model: "mock"
    user: question
    tools: [echo]
}
`
	path := filepath.Join(dir, "agent.leia")
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{dir}})
	if err != nil {
		t.Fatal(err)
	}
	if report.SchemaVersion != SchemaVersion || report.Phase != "syntax-static" || report.Status != "ok" {
		t.Fatalf("report header = %#v", report)
	}
	if report.Summary.Files != 1 || report.Summary.ParsedFiles != 1 {
		t.Fatalf("summary files = %#v", report.Summary)
	}
	if report.Summary.Agents != 1 || report.Summary.Tools != 1 || report.Summary.Models != 1 {
		t.Fatalf("summary LLM counts = %#v", report.Summary)
	}
	if report.Summary.TODOs != 1 {
		t.Fatalf("summary TODOs = %#v", report.Summary)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "todo" {
		t.Fatalf("findings = %#v, want one TODO finding", report.Findings)
	}
}

func TestRunReportsAISyntaxValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.leia")
	if err := os.WriteFile(path, []byte(`tool missing_caps() { return nil, nil }`), 0644); err != nil {
		t.Fatal(err)
	}

	report, err := Run(Options{Paths: []string{path}})
	if err != nil {
		t.Fatal(err)
	}
	if report.Status != "failed" {
		t.Fatalf("status = %q, want failed", report.Status)
	}
	if len(report.Findings) != 1 || report.Findings[0].Kind != "ai_syntax_error" {
		t.Fatalf("findings = %#v, want ai_syntax_error", report.Findings)
	}
}
