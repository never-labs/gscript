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
	if !bytes.Contains(dialectDoc, []byte("`sh`")) || !bytes.Contains(dialectDoc, []byte("`agent`")) || !bytes.Contains(dialectDoc, []byte("Built-In Dialects")) || !bytes.Contains(dialectDoc, []byte("data-format")) {
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
	if bundle.SchemaVersion != 1 || bundle.Status != "pass" || bundle.CLI.CommandCount != len(bundle.CLI.Commands) || len(bundle.CLI.Commands) == 0 || bundle.Stdlib.LayerCount != len(bundle.Stdlib.Layers) || bundle.Stdlib.DefaultCount != len(bundle.Stdlib.DefaultImports) || len(bundle.Stdlib.Layers) == 0 || !docDefaultImportsContain(bundle.Stdlib.DefaultImports, "sqrt", "math", "sqrt") || bundle.Dialects.DialectCount != len(bundle.Dialects.Dialects) || len(bundle.Dialects.Dialects) == 0 {
		t.Fatalf("bundle = %#v, want CLI and stdlib references", bundle)
	}
}

func TestDocSpecPreviewRendersPublishableHTML(t *testing.T) {
	dir := t.TempDir()
	for _, item := range []struct {
		name string
		body string
	}{
		{
			name: "index.md",
			body: "---\nlayout: spec\ntitle: Spec\n---\n# Intro\nSee [Notation](notation.md), [Grammar](grammar.ebnf), and [Source section](source.md#comments).\n<!-- hidden generator marker -->\n```leia\nprint(\"ok\")\n```\n",
		},
		{name: "notation.md", body: "# Notation\n"},
		{name: "source.md", body: "# Source\n"},
		{name: "lexical.md", body: "# Lexical\n"},
		{name: "declarations.md", body: "# Declarations\n"},
		{name: "values.md", body: "# Values\n"},
		{name: "expressions.md", body: "# Expressions\n"},
		{name: "statements.md", body: "# Statements\n"},
		{name: "functions.md", body: "# Functions\n"},
		{name: "tables.md", body: "# Tables\n"},
		{name: "concurrency.md", body: "# Concurrency\n"},
		{name: "dialects.md", body: "# Dialects\n"},
		{name: "q-dialect.md", body: "# q\n"},
		{name: "ai-dialect.md", body: "# AI\n"},
		{name: "modules.md", body: "# Modules\n"},
		{name: "errors.md", body: "# Errors\n"},
		{name: "implementation.md", body: "# Implementation\n"},
		{name: "grammar.ebnf", body: "expr = ident ;\n"},
	} {
		if err := os.WriteFile(filepath.Join(dir, item.name), []byte(item.body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	htmlText, err := renderSpecPreview(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"layout: spec", "hidden generator marker", `href="#"`} {
		if strings.Contains(htmlText, bad) {
			t.Fatalf("spec preview contains %q:\n%s", bad, htmlText)
		}
	}
	for _, want := range []string{`href="#notation"`, `href="#grammar-appendix"`, `href="#source-code-representation-comments"`, `language-leia leia-code`} {
		if !strings.Contains(htmlText, want) {
			t.Fatalf("spec preview missing %q:\n%s", want, htmlText)
		}
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
	if !strings.HasSuffix(gotName, filepath.Join("scripts", "run.sh")) {
		t.Fatalf("command = %q, want scripts/run.sh", gotName)
	}
	if len(gotArgs) != 1 || gotArgs[0] != "docs" {
		t.Fatalf("args = %#v, want docs", gotArgs)
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
	if !strings.HasSuffix(gotName, filepath.Join("scripts", "run.sh")) {
		t.Fatalf("command = %q, want scripts/run.sh", gotName)
	}
	if len(gotArgs) != 2 || gotArgs[0] != "docs" || gotArgs[1] != "--json" {
		t.Fatalf("args = %#v, want docs --json", gotArgs)
	}
	if !strings.Contains(stdout.String(), "doc helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestDocSiteCheckReportsRenderedSite(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	siteDir := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(filepath.Join(siteDir, "guide"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "style.css"), []byte("body{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(`<!doctype html><html><head><link rel="stylesheet" href="/style.css"></head><body><h1 id="top">Leia</h1><a href="/guide/">Guide</a><a href="#top">Top</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "guide", "index.html"), []byte(`<!doctype html><html><body><h1 id="intro">Guide</h1><a href="/index.html#top">Home</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"site-check", "--site-dir", siteDir, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDocCommand site-check code = %d, stderr = %q", code, stderr.String())
	}
	var report docSiteReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("site-check JSON failed to decode: %v\n%s", err, stdout.String())
	}
	if report.Status != "pass" || report.HTMLFileCount != 2 || report.LocalLinkCount != 3 || report.AssetRefCount != 1 || report.FragmentCheckCount != 2 || report.FailureCount != 0 || len(report.FailureDetails) != 0 {
		t.Fatalf("site-check report = %+v, want passing rendered-site report", report)
	}
}

func TestDocSiteCheckReportsBrokenSite(t *testing.T) {
	root := repoRootForBoundaryTest(t)
	siteDir := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(siteDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte(`<!doctype html><html><body><h1 id="top">Leia</h1><a href="/missing.html">Missing</a><a href="#absent">Bad anchor</a></body></html>`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	var stdout, stderr bytes.Buffer
	code := runDocCommand([]string{"site-check", "--site-dir", siteDir, "--json"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runDocCommand site-check code = %d, want 1; stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report docSiteReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("site-check JSON failed to decode: %v\n%s", err, stdout.String())
	}
	if report.Status != "issues" || report.FailureKindCount != 2 || report.FailureCount != 2 || !containsString(report.FailureKinds, "missing_target") || !containsString(report.FailureKinds, "missing_anchor") {
		t.Fatalf("site-check broken report = %+v, want missing target and anchor", report)
	}
}
