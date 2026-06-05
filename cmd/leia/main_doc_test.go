package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDocGenerateWritesReferenceFiles(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"generate", "--output", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand code = %d, stderr = %q", code, stderr.String())
	}
	cliDoc, err := os.ReadFile(filepath.Join(dir, "cli.md"))
	if err != nil {
		t.Fatal(err)
	}
	stdlibDoc, err := os.ReadFile(filepath.Join(dir, "stdlib.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(cliDoc, []byte("`run`")) || !bytes.Contains(cliDoc, []byte("`doc`")) {
		t.Fatalf("cli.md = %q, want command reference", string(cliDoc))
	}
	if !bytes.Contains(stdlibDoc, []byte("`json`")) || !bytes.Contains(stdlibDoc, []byte("JSON encode/decode")) || !bytes.Contains(stdlibDoc, []byte("Safe default")) {
		t.Fatalf("stdlib.md = %q, want stdlib inventory", string(stdlibDoc))
	}
}

func TestDocGenerateWritesSiteLayout(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"generate", "--layout", "site", "--output", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand code = %d, stderr = %q", code, stderr.String())
	}
	cliPath := filepath.Join(dir, "reference", "cli", "index.md")
	stdlibPath := filepath.Join(dir, "reference", "stdlib", "index.md")
	if _, err := os.Stat(cliPath); err != nil {
		t.Fatalf("missing site cli doc: %v", err)
	}
	stdlibDoc, err := os.ReadFile(stdlibPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdlibDoc, []byte("Generated from `internal/stdlib/catalog`")) {
		t.Fatalf("stdlib site doc = %q, want generated stdlib inventory", string(stdlibDoc))
	}
}

func TestCheckedInReferenceDocsStayGenerated(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	for _, item := range []struct {
		path string
		want []byte
	}{
		{
			path: filepath.Join("docs", "reference", "cli", "index.md"),
			want: generateCLIReferenceMarkdown(),
		},
		{
			path: filepath.Join("docs", "reference", "stdlib", "index.md"),
			want: generateStdlibInventoryMarkdown(),
		},
	} {
		got, err := os.ReadFile(filepath.Join(root, item.path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, item.want) {
			t.Fatalf("%s is stale; run `go run ./cmd/leia doc generate --layout site --output docs`", item.path)
		}
	}
}

func TestDocGenerateWritesJSONReferenceFiles(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"generate", "--format", "json", "--layout", "site", "--output", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand code = %d, stderr = %q", code, stderr.String())
	}
	cliDoc, err := os.ReadFile(filepath.Join(dir, "reference", "cli", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cliRef docCLIReference
	if err := json.Unmarshal(cliDoc, &cliRef); err != nil {
		t.Fatalf("decode cli json: %v", err)
	}
	if cliRef.SchemaVersion != 1 || len(cliRef.Commands) == 0 || cliRef.Commands[0].Usage == "" {
		t.Fatalf("cli json = %#v, want versioned command reference with usage", cliRef)
	}
	stdlibDoc, err := os.ReadFile(filepath.Join(dir, "reference", "stdlib", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var stdlibRef docStdlibInventory
	if err := json.Unmarshal(stdlibDoc, &stdlibRef); err != nil {
		t.Fatalf("decode stdlib json: %v", err)
	}
	if stdlibRef.SchemaVersion != 1 || len(stdlibRef.Layers) == 0 || len(stdlibRef.Layers[0].Modules) == 0 {
		t.Fatalf("stdlib json = %#v, want versioned stdlib inventory", stdlibRef)
	}
}

func TestDocGenerateWritesCombinedJSONToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"generate", "--format", "json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand code = %d, stderr = %q", code, stderr.String())
	}
	var bundle docReferenceBundle
	if err := json.Unmarshal(stdout.Bytes(), &bundle); err != nil {
		t.Fatalf("decode combined json: %v", err)
	}
	if bundle.SchemaVersion != 1 || len(bundle.CLI.Commands) == 0 || len(bundle.Stdlib.Layers) == 0 {
		t.Fatalf("bundle = %#v, want CLI and stdlib references", bundle)
	}
}

func TestDocHelpFlagsExitSuccessfully(t *testing.T) {
	for _, args := range [][]string{
		{"help"},
		{"generate", "--help"},
		{"check", "--help"},
	} {
		var stdout, stderr bytes.Buffer
		code := runDocCommand(args, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("runDocCommand(%v) code = %d, stderr = %q", args, code, stderr.String())
		}
		if stdout.Len() == 0 && stderr.Len() == 0 {
			t.Fatalf("runDocCommand(%v) produced no help output", args)
		}
	}
}

func TestDocCheckDispatchesDocsScript(t *testing.T) {
	oldDocExecCommand := docExecCommand
	t.Cleanup(func() { docExecCommand = oldDocExecCommand })
	var gotName string
	var gotArgs []string
	docExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "doc")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 1 || !strings.HasSuffix(gotArgs[0], filepath.Join("scripts", "docs_check.sh")) {
		t.Fatalf("args = %#v, want scripts/docs_check.sh", gotArgs)
	}
	if !strings.Contains(stdout.String(), "doc helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}
