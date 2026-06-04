package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/internal/modpkg"
	"github.com/never-labs/leia/internal/stdlib/catalog"
	"github.com/never-labs/leia/llm"
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

func TestRunCommandDialectExamples(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dialectDir := filepath.Join(root, "examples", "dialects")
	entries, err := os.ReadDir(dialectDir)
	if err != nil {
		t.Fatal(err)
	}
	var examples []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
			continue
		}
		examples = append(examples, entry.Name())
	}
	if len(examples) == 0 {
		t.Fatal("examples/dialects contains no runnable .leia examples")
	}
	for _, name := range examples {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dialectDir, name)

			var stdout, stderr bytes.Buffer
			code := runRunCommand([]string{"--vm", path}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
			}
		})
	}
}

func TestRunCommandDialectExamplesCoverApprovedBuiltinTags(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	matrixTags := loadFeatureMatrixBuiltinDialectTags(t, root)
	exampleTags := collectDialectExampleTags(t, root)
	var missing []string
	for _, tag := range approvedBuiltinDialectTags() {
		if !matrixTags[tag] && !exampleTags[tag] {
			missing = append(missing, tag)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("approved builtin dialect tags must be listed in tests/feature_matrix.json or covered by runnable dialect examples; missing %s", strings.Join(missing, ", "))
	}

	for _, tag := range []string{"sh", "cmd", "glob", "path", "json", "csv", "url", "base64", "prompt", "quote"} {
		if !exampleTags[tag] {
			t.Fatalf("runnable dialect examples must keep representative builtin tag %q covered", tag)
		}
	}
}

func TestPackageManagedUIExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "ui", "package_managed")

	for _, forbidden := range []string{"ui", "dom", "widget", "widgets", "canvas"} {
		if _, ok := catalog.Module(forbidden); ok {
			t.Fatalf("stdlib catalog contains UI-shaped module %q; UI runtimes should stay package-managed", forbidden)
		}
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"check", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod check code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK {
		t.Fatalf("verify = %+v, want package-managed UI example manifest to verify", verify)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"list", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod list code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var list modListReport
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("stdout is not JSON list report: %v; stdout = %q", err, stdout.String())
	}
	if !list.OK || !containsResolvedRequire(list.Requires, "github.com/never-labs/leia-ui/raylib", "v0.1.0") {
		t.Fatalf("list = %+v, want external Leia UI runtime requirement", list)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"gomod", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod gomod code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if got := stdout.String(); !strings.Contains(got, "github.com/gen2brain/raylib-go/raylib v0.55.1") {
		t.Fatalf("generated go.mod = %q, want native UI adapter dependency", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"capability", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod capability code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var caps modCapabilityReport
	if err := json.Unmarshal(stdout.Bytes(), &caps); err != nil {
		t.Fatalf("stdout is not JSON capability report: %v; stdout = %q", err, stdout.String())
	}
	if !caps.OK {
		t.Fatalf("capability report = %+v, want package-managed UI capabilities to report", caps)
	}
	if !moduleHasCapabilities(caps.Modules, "example.com/leia/examples/ui/package-managed", "ui.input", "ui.window") {
		t.Fatalf("capability modules = %+v, want UI domain capabilities on the example manifest", caps.Modules)
	}
	if !moduleHasPath(caps.Modules, "github.com/never-labs/leia-ui/raylib") {
		t.Fatalf("capability modules = %+v, want external package-managed UI runtime module", caps.Modules)
	}
}

func TestPackageManagedDatabaseExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "database", "package_managed")

	for _, forbidden := range []string{"database", "db", "sql", "sqlite", "postgres", "mysql"} {
		if _, ok := catalog.Module(forbidden); ok {
			t.Fatalf("stdlib catalog contains database-shaped module %q; database runtimes should stay package-managed", forbidden)
		}
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"check", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod check code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK {
		t.Fatalf("verify = %+v, want package-managed database example manifest to verify", verify)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"list", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod list code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var list modListReport
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("stdout is not JSON list report: %v; stdout = %q", err, stdout.String())
	}
	if !list.OK || !containsResolvedRequire(list.Requires, "github.com/never-labs/leia-db/sqlite", "v0.1.0") {
		t.Fatalf("list = %+v, want external Leia database runtime requirement", list)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"gomod", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod gomod code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if got := stdout.String(); !strings.Contains(got, "modernc.org/sqlite v1.38.2") {
		t.Fatalf("generated go.mod = %q, want native SQLite adapter dependency", got)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"capability", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod capability code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var caps modCapabilityReport
	if err := json.Unmarshal(stdout.Bytes(), &caps); err != nil {
		t.Fatalf("stdout is not JSON capability report: %v; stdout = %q", err, stdout.String())
	}
	if !caps.OK {
		t.Fatalf("capability report = %+v, want package-managed database capabilities to report", caps)
	}
	if !moduleHasCapabilities(caps.Modules, "example.com/leia/examples/database/package-managed", "db.open", "db.query") {
		t.Fatalf("capability modules = %+v, want database domain capabilities on the example manifest", caps.Modules)
	}
	if !moduleHasPath(caps.Modules, "github.com/never-labs/leia-db/sqlite") {
		t.Fatalf("capability modules = %+v, want external package-managed database runtime module", caps.Modules)
	}
}

func TestTestingJSONLWorkflowExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "testing")

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--json", "--golden=require", dir}, cliRunOptions{UseVM: true}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("test command code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON test report: %v; stdout = %q", err, stdout.String())
	}
	if !report.OK || report.Total != 2 || report.Passed != 2 {
		t.Fatalf("report = %+v, want two passing JSONL testing examples", report)
	}
	if !testReportHasPassingFile(report, "jsonl_workflow_test.leia") {
		t.Fatalf("files = %+v, want jsonl_workflow_test.leia to pass", report.Files)
	}
	if !testReportHasPassingFile(report, "jsonl_golden_eval_replay_test.leia") {
		t.Fatalf("files = %+v, want jsonl_golden_eval_replay_test.leia to pass", report.Files)
	}
}

func TestWorkflowReplayExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "workflow")
	records, err := llm.LoadRecords(filepath.Join(dir, "support_triage_replay.records.json"))
	if err != nil {
		t.Fatalf("load replay records: %v", err)
	}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibLLM),
		leia.WithLLMReplay(records),
		leia.WithVM(),
	)
	if err := vm.ExecFile(filepath.Join(dir, "support_triage_replay.leia")); err != nil {
		t.Fatalf("run replay workflow example: %v", err)
	}
	got, err := vm.Get("classification")
	if err != nil || got != "refund/escalate" {
		t.Fatalf("classification = %#v err=%v, want refund/escalate", got, err)
	}
}

func TestWorkflowEvaluateListExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "workflow")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--list", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate command code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report struct {
		Status  string `json:"status"`
		Summary struct {
			Files       int `json:"files"`
			ParsedFiles int `json:"parsed_files"`
		} `json:"summary"`
		Inputs []struct {
			Path   string `json:"path"`
			Status string `json:"status"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if report.Status != "ok" || report.Summary.Files != 1 || report.Summary.ParsedFiles != 1 {
		t.Fatalf("report = %+v, want one parsed workflow file with ok status", report)
	}
	if !evaluateReportHasOKInput(report.Inputs, "support_triage_replay.leia") {
		t.Fatalf("inputs = %+v, want support_triage_replay.leia ok", report.Inputs)
	}
}

func approvedBuiltinDialectTags() []string {
	return []string{
		"sh", "cmd", "shellwords", "glob", "path",
		"re", "regexp", "json", "jsonl", "csv", "tsv", "mdtable", "lines", "split", "words", "nums", "numbers", "kv", "env", "ini", "semver", "duration", "tap", "junit", "xml", "template",
		"url", "html_escape", "urlquery", "urlpath", "mime", "headers", "http_headers", "cookie", "cookies", "httpmsg",
		"ipaddr", "cidr", "hostport",
		"base64", "hash", "hex", "base32",
		"prompt", "quote",
	}
}

func loadFeatureMatrixBuiltinDialectTags(t *testing.T, root string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "tests", "feature_matrix.json"))
	if err != nil {
		t.Fatalf("read feature matrix: %v", err)
	}
	var matrix struct {
		Features []struct {
			ID                 string   `json:"id"`
			BuiltinDialectTags []string `json:"builtin_dialect_tags"`
		} `json:"features"`
	}
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatalf("decode feature matrix: %v", err)
	}
	for _, feature := range matrix.Features {
		if feature.ID != "tagged_dialect_syntax" {
			continue
		}
		tags := make(map[string]bool, len(feature.BuiltinDialectTags))
		for _, tag := range feature.BuiltinDialectTags {
			tags[tag] = true
		}
		return tags
	}
	t.Fatal("feature_matrix.json missing tagged_dialect_syntax builtin_dialect_tags")
	return nil
}

func collectDialectExampleTags(t *testing.T, root string) map[string]bool {
	t.Helper()
	approved := map[string]bool{}
	for _, tag := range approvedBuiltinDialectTags() {
		approved[tag] = true
	}
	paths := []string{filepath.Join(root, "examples", "hello", "dialects.leia")}
	dialectDir := filepath.Join(root, "examples", "dialects")
	entries, err := os.ReadDir(dialectDir)
	if err != nil {
		t.Fatalf("read examples/dialects: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
			continue
		}
		paths = append(paths, filepath.Join(dialectDir, entry.Name()))
	}
	sort.Strings(paths)

	evalRe := regexp.MustCompile(`dialect\.eval\("([A-Za-z_][A-Za-z0-9_]*)"`)
	taggedRe := regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)!?\s*(?:` + "`" + `|\{)`)
	shellShortcutRe := regexp.MustCompile(`\$!?\s*` + "`")
	tags := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		source := string(data)
		if shellShortcutRe.MatchString(source) {
			tags["sh"] = true
		}
		for _, match := range evalRe.FindAllStringSubmatch(source, -1) {
			if approved[match[1]] {
				tags[match[1]] = true
			}
		}
		for _, match := range taggedRe.FindAllStringSubmatch(source, -1) {
			if approved[match[1]] {
				tags[match[1]] = true
			}
		}
	}
	return tags
}

func containsResolvedRequire(reqs []modpkg.ListRequire, path, version string) bool {
	for _, req := range reqs {
		if req.Path == path && req.Version == version {
			return true
		}
	}
	return false
}

func moduleHasCapabilities(modules []modpkg.CapabilityModule, path string, capabilities ...string) bool {
	for _, module := range modules {
		if module.Path != path {
			continue
		}
		for _, capability := range capabilities {
			if !containsString(module.Capabilities, capability) {
				return false
			}
		}
		return true
	}
	return false
}

func moduleHasPath(modules []modpkg.CapabilityModule, path string) bool {
	for _, module := range modules {
		if module.Path == path {
			return true
		}
	}
	return false
}

func testReportHasPassingFile(report testRunResult, name string) bool {
	for _, file := range report.Files {
		if filepath.Base(file.File) == name && file.OK {
			return true
		}
	}
	return false
}

func evaluateReportHasOKInput(inputs []struct {
	Path   string `json:"path"`
	Status string `json:"status"`
}, name string) bool {
	for _, input := range inputs {
		if filepath.Base(input.Path) == name && input.Status == "ok" {
			return true
		}
	}
	return false
}
