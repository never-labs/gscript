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
	dialectDoc, err := os.ReadFile(filepath.Join(dir, "dialects.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(cliDoc, []byte("`run`")) || !bytes.Contains(cliDoc, []byte("`doc`")) {
		t.Fatalf("cli.md = %q, want command reference", string(cliDoc))
	}
	if !bytes.Contains(stdlibDoc, []byte("`json`")) || !bytes.Contains(stdlibDoc, []byte("JSON encode/decode")) || !bytes.Contains(stdlibDoc, []byte("Safe default")) || !bytes.Contains(stdlibDoc, []byte("## Default Imports")) || !bytes.Contains(stdlibDoc, []byte("| `mat` | `linalg.matrix` |")) {
		t.Fatalf("stdlib.md = %q, want stdlib inventory", string(stdlibDoc))
	}
	if !bytes.Contains(dialectDoc, []byte("`sh`")) || !bytes.Contains(dialectDoc, []byte("`agent`")) || !bytes.Contains(dialectDoc, []byte("Built-In Dialects")) || !bytes.Contains(dialectDoc, []byte("Standard-library helpers such as `q.sql` and `q.query`")) {
		t.Fatalf("dialects.md = %q, want dialect reference", string(dialectDoc))
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
	dialectPath := filepath.Join(dir, "reference", "dialects", "index.md")
	if _, err := os.Stat(cliPath); err != nil {
		t.Fatalf("missing site cli doc: %v", err)
	}
	stdlibDoc, err := os.ReadFile(stdlibPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(stdlibDoc, []byte("generated from the standard-library metadata")) || !bytes.Contains(stdlibDoc, []byte("## Default Imports")) {
		t.Fatalf("stdlib site doc = %q, want generated stdlib inventory", string(stdlibDoc))
	}
	dialectDoc, err := os.ReadFile(dialectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(dialectDoc, []byte("The registry table is generated from the current `leia` binary dialect registry")) {
		t.Fatalf("dialect site doc = %q, want generated dialect reference", string(dialectDoc))
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
		{
			path: filepath.Join("docs", "reference", "dialects", "index.md"),
			want: generateDialectReferenceMarkdown(),
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
	if cliRef.SchemaVersion != 1 || cliRef.CommandCount != len(cliRef.Commands) || len(cliRef.Commands) == 0 || cliRef.Commands[0].Usage == "" {
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
	if stdlibRef.SchemaVersion != 1 || stdlibRef.LayerCount != len(stdlibRef.Layers) || stdlibRef.DefaultCount != len(stdlibRef.DefaultImports) || len(stdlibRef.Layers) == 0 || len(stdlibRef.Layers[0].Modules) == 0 || !docDefaultImportsContain(stdlibRef.DefaultImports, "mat", "linalg", "matrix") {
		t.Fatalf("stdlib json = %#v, want versioned stdlib inventory", stdlibRef)
	}
	dialectDoc, err := os.ReadFile(filepath.Join(dir, "reference", "dialects", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var dialectRef docDialectReference
	if err := json.Unmarshal(dialectDoc, &dialectRef); err != nil {
		t.Fatalf("decode dialect json: %v", err)
	}
	if dialectRef.SchemaVersion != 1 || dialectRef.DialectCount != len(dialectRef.Dialects) || len(dialectRef.Dialects) == 0 {
		t.Fatalf("dialect json = %#v, want versioned dialect reference", dialectRef)
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
	if bundle.SchemaVersion != 1 || bundle.CLI.CommandCount != len(bundle.CLI.Commands) || len(bundle.CLI.Commands) == 0 || bundle.Stdlib.LayerCount != len(bundle.Stdlib.Layers) || bundle.Stdlib.DefaultCount != len(bundle.Stdlib.DefaultImports) || len(bundle.Stdlib.Layers) == 0 || !docDefaultImportsContain(bundle.Stdlib.DefaultImports, "sqrt", "math", "sqrt") || bundle.Dialects.DialectCount != len(bundle.Dialects.Dialects) || len(bundle.Dialects.Dialects) == 0 {
		t.Fatalf("bundle = %#v, want CLI and stdlib references", bundle)
	}
}

func docDefaultImportsContain(imports []cliDefaultImport, name, module, member string) bool {
	for _, item := range imports {
		if item.Name == name && item.Module == module && item.Member == member {
			return true
		}
	}
	return false
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

func TestDocCheckJSONDispatchesDocsScriptWithJSON(t *testing.T) {
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
	code := runDocCommand([]string{"check", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 2 || !strings.HasSuffix(gotArgs[0], filepath.Join("scripts", "docs_check.sh")) || gotArgs[1] != "--json" {
		t.Fatalf("args = %#v, want scripts/docs_check.sh --json", gotArgs)
	}
	if !strings.Contains(stdout.String(), "doc helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}
