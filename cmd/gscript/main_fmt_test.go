package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFmtCheckReportsUnformattedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "needs_fmt.gs")
	original := []byte("x := 1 \t\r\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), path+": not formatted") {
		t.Fatalf("stderr = %q, want not formatted diagnostic", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed during --check: %q", string(got))
	}
}

func TestFmtWritesWhitespaceNormalization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "needs_fmt.gs")
	if err := os.WriteFile(path, []byte("x := 1  \n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("stdout = %q, want formatted filename", stdout.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x := 1\n" {
		t.Fatalf("formatted file = %q, want %q", string(got), "x := 1\n")
	}
}

func TestFmtStdinWritesFormattedSourceToStdout(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1  \r\n\r\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "x := 1\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinCheckReportsUnformattedSource(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1 \t\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", "--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "scratch.gs: not formatted") {
		t.Fatalf("stderr = %q, want not formatted diagnostic", stderr.String())
	}
}

func TestFmtStdinCheckAcceptsFormattedSource(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", "--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
