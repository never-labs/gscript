package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelpDoesNotStartServer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--help"}, strings.NewReader(""), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run help code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "usage: leia-lsp") {
		t.Fatalf("stdout = %q, want usage", stdout.String())
	}
}

func TestUnknownArgumentReportsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--bad"}, strings.NewReader(""), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run unknown code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia-lsp") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}
