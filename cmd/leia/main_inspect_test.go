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
	path := filepath.Join(dir, "ok.leia")
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
	path := filepath.Join(dir, "fn.leia")
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

func TestInspectBytecodeDumpsJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bytecode_json.leia")
	src := `func add(a, b) {
    return a + b
}
print(add(1, 2))
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectCommand([]string{"bytecode", "--json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInspectCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report inspectBytecodeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON bytecode report: %v; stdout = %q", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Source != path || report.SelectedProto != "<main>" || !report.Recursive {
		t.Fatalf("report = %+v, want schema v1 recursive main report", report)
	}
	if report.Proto.DisplayName != "<main>" || report.Proto.InstructionCount == 0 || report.Proto.ChildProtoCount != 1 || len(report.Proto.Children) != 1 {
		t.Fatalf("main proto = %+v, want one child and bytecode metadata", report.Proto)
	}
	if report.Proto.Tier1.Reason == "" || report.Proto.Tier2.Reason == "" {
		t.Fatalf("jit decisions missing reasons: %+v %+v", report.Proto.Tier1, report.Proto.Tier2)
	}
	if child := report.Proto.Children[0]; child.DisplayName != "add" || child.NumParams != 2 || !strings.Contains(child.Disassembly, "RETURN") {
		t.Fatalf("child proto = %+v, want add function disassembly", child)
	}
}

func TestInspectBytecodeDumpsNamedProtoJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fn_json.leia")
	src := `func add(a, b) {
    return a + b
}
print(add(1, 2))
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runInspectCommand([]string{"bytecode", "--json", "--proto", "add", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runInspectCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report inspectBytecodeReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON bytecode report: %v; stdout = %q", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.SelectedProto != "add" || report.Recursive {
		t.Fatalf("report = %+v, want selected non-recursive add report", report)
	}
	if report.Proto.DisplayName != "add" || report.Proto.NumParams != 2 || report.Proto.ChildProtoCount != 0 || len(report.Proto.Children) != 0 {
		t.Fatalf("proto = %+v, want selected add proto only", report.Proto)
	}
	if strings.Contains(report.Proto.Disassembly, "=== <main>") || !strings.Contains(report.Proto.Disassembly, "RETURN") {
		t.Fatalf("disassembly = %q, want named proto disassembly only", report.Proto.Disassembly)
	}
}

func TestInspectDirectivesDumpsFileDirectives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "directives.leia")
	src := `//leia:build linux, darwin
//leia:test integration slow
//leia:cap docs.read,net.client
//leia:feature llm
//@leia:build ignored
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
	path := filepath.Join(dir, "directives_json.leia")
	src := `//leia:cap fs.read, net.client
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
