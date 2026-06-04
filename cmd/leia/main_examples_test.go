package main

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestRunCommandExampleDialects(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "hello", "dialects.leia")

	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--vm", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
	}
}
