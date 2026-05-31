package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectBytecodeDumpsMainProto(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\nprint(x)\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectCommand([]string{"bytecode", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInspectCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "=== <main>") || !strings.Contains(out, "LOAD") {
		t.Fatalf("stdout = %q, want main bytecode dump", out)
	}
}

func TestInspectBytecodeDumpsNamedProto(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fn.gs")
	src := `func add(a, b) {
    return a + b
}
print(add(1, 2))
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectCommand([]string{"bytecode", "--proto", "add", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInspectCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "=== <main>") || !strings.Contains(out, "RETURN") {
		t.Fatalf("stdout = %q, want named proto disassembly only", out)
	}
}

func TestInspectDirectivesDumpsFileDirectives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directives.gs")
	src := `//gscript:build linux, darwin
//gscript:test integration slow
//gscript:cap docs.read,net.client
//gscript:feature llm
//@gscript:build ignored
func main() {}
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectCommand([]string{"directives", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInspectCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"1:1 build linux, darwin",
		"2:1 test integration slow",
		"3:1 cap docs.read,net.client",
		"4:1 feature llm",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, "ignored") {
		t.Fatalf("stdout = %q, want @ syntax ignored", out)
	}
}

func TestInspectDirectivesDumpsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directives_json.gs")
	src := `//gscript:cap fs.read, net.client
print("ok")
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectCommand([]string{"directives", "--json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInspectCommand code = %d, stderr = %q", code, stderr.String())
	}
	var directives []inspectFileDirective
	if err := json.Unmarshal(stdout.Bytes(), &directives); err != nil {
		t.Fatalf("stdout is not JSON directives: %v; stdout = %q", err, stdout.String())
	}
	if len(directives) != 1 {
		t.Fatalf("directives = %#v, want one", directives)
	}
	if got := directives[0]; got.Kind != "cap" || got.Line != 1 || got.Column != 1 || len(got.Args) != 2 || got.Args[0] != "fs.read" || got.Args[1] != "net.client" {
		t.Fatalf("directive = %#v, want cap fs.read net.client at 1:1", got)
	}
}
