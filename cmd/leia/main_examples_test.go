package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/internal/ast"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/modpkg"
	"github.com/never-labs/leia/internal/parser"
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

func TestEvaluateBasicExampleExecutes(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "evaluate", "basic_assert.leia")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate command code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	var report struct {
		Status  string `json:"status"`
		Summary struct {
			EvaluateBlocks int `json:"evaluate_blocks"`
			CasesSelected  int `json:"cases_selected"`
			CasesPassed    int `json:"cases_passed"`
			Assertions     int `json:"assertions"`
		} `json:"summary"`
		Findings []struct {
			Kind string `json:"kind"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
	}
	if report.Status != "ok" || report.Summary.EvaluateBlocks != 1 || report.Summary.CasesSelected != 1 || report.Summary.CasesPassed != 1 || report.Summary.Assertions != 1 {
		t.Fatalf("report = %+v, want one passing evaluate block with one assertion", report)
	}
	if len(report.Findings) != 0 {
		t.Fatalf("findings = %+v, want none", report.Findings)
	}
}

func TestEvaluateReplayExamplesExecute(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "evaluate")
	for _, tc := range []struct {
		name          string
		source        string
		records       string
		replayedTurns int
	}{
		{name: "llm", source: "llm_replay.leia", records: "llm_replay.records.json", replayedTurns: 1},
		{name: "agent", source: "agent_replay.leia", records: "agent_replay.records.json", replayedTurns: 1},
		{name: "multiturn", source: "multiturn_replay.leia", records: "multiturn_replay.records.json", replayedTurns: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := runEvaluateCommand([]string{
				"--json",
				"--replay", filepath.Join(dir, tc.records),
				filepath.Join(dir, tc.source),
			}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("evaluate command code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
			}
			var report struct {
				Status  string `json:"status"`
				Summary struct {
					EvaluateBlocks int `json:"evaluate_blocks"`
					CasesSelected  int `json:"cases_selected"`
					CasesPassed    int `json:"cases_passed"`
					CasesFailed    int `json:"cases_failed"`
				} `json:"summary"`
				LLM *struct {
					ReplayedTurns  int `json:"replayed_turns"`
					RemainingTurns int `json:"remaining_turns"`
				} `json:"llm"`
				Cases []struct {
					Status string `json:"status"`
				} `json:"cases"`
				Findings []struct {
					Kind string `json:"kind"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
			}
			if report.Status != "ok" || report.Summary.EvaluateBlocks != 1 || report.Summary.CasesSelected != 1 || report.Summary.CasesPassed != 1 || report.Summary.CasesFailed != 0 {
				t.Fatalf("report = %+v, want one passing evaluate case", report)
			}
			if len(report.Cases) != 1 || report.Cases[0].Status != "passed" {
				t.Fatalf("cases = %+v, want one passed case", report.Cases)
			}
			if report.LLM == nil || report.LLM.ReplayedTurns != tc.replayedTurns || report.LLM.RemainingTurns != 0 {
				t.Fatalf("llm = %+v, want replayed=%d remaining=0", report.LLM, tc.replayedTurns)
			}
			if len(report.Findings) != 0 {
				t.Fatalf("findings = %+v, want none", report.Findings)
			}
		})
	}
}

func approvedBuiltinDialectTags() []string {
	return []string{
		"sh", "cmd", "shellwords", "glob", "path",
		"re", "regexp", "json", "jsonptr", "jsonl", "csv", "tsv", "mdtable", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "semver", "duration", "tap", "junit", "xml", "template",
		"url", "html_escape", "urlquery", "urlpath", "mime", "headers", "http_headers", "cookie", "cookies", "httpmsg", "sse", "multipart", "jwt",
		"ipaddr", "cidr", "hostport",
		"base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary",
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

	tags := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		tokens, err := lexer.New(string(data)).Tokenize()
		if err != nil {
			t.Fatalf("lex %s: %v", path, err)
		}
		prog, err := parser.New(tokens).Parse()
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		collectDialectTagsFromProgram(prog, approved, tags)
	}
	return tags
}

func collectDialectTagsFromProgram(prog *ast.Program, approved, tags map[string]bool) {
	if prog == nil {
		return
	}
	for _, stmt := range prog.Stmts {
		collectDialectTagsFromStmt(stmt, approved, tags)
	}
}

func collectDialectTagsFromStmt(stmt ast.Stmt, approved, tags map[string]bool) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		for _, expr := range s.Targets {
			collectDialectTagsFromExpr(expr, approved, tags)
		}
		for _, expr := range s.Values {
			collectDialectTagsFromExpr(expr, approved, tags)
		}
	case *ast.DeclareStmt:
		for _, expr := range s.Values {
			collectDialectTagsFromExpr(expr, approved, tags)
		}
	case *ast.CompoundAssignStmt:
		collectDialectTagsFromExpr(s.Target, approved, tags)
		collectDialectTagsFromExpr(s.Value, approved, tags)
	case *ast.IncDecStmt:
		collectDialectTagsFromExpr(s.Target, approved, tags)
	case *ast.CallStmt:
		collectDialectTagsFromExpr(s.Call, approved, tags)
	case *ast.GoStmt:
		collectDialectTagsFromExpr(s.Call, approved, tags)
	case *ast.DeferStmt:
		collectDialectTagsFromExpr(s.Call, approved, tags)
	case *ast.SendStmt:
		collectDialectTagsFromExpr(s.Channel, approved, tags)
		collectDialectTagsFromExpr(s.Value, approved, tags)
	case *ast.SelectStmt:
		for _, clause := range s.Cases {
			collectDialectTagsFromExpr(clause.Channel, approved, tags)
			collectDialectTagsFromExpr(clause.SendValue, approved, tags)
			collectDialectTagsFromBlock(clause.Body, approved, tags)
		}
		collectDialectTagsFromBlock(s.Default, approved, tags)
	case *ast.IfStmt:
		collectDialectTagsFromExpr(s.Cond, approved, tags)
		collectDialectTagsFromBlock(s.Body, approved, tags)
		for _, clause := range s.ElseIfs {
			collectDialectTagsFromExpr(clause.Cond, approved, tags)
			collectDialectTagsFromBlock(clause.Body, approved, tags)
		}
		collectDialectTagsFromBlock(s.ElseBody, approved, tags)
	case *ast.ForNumStmt:
		collectDialectTagsFromStmt(s.Init, approved, tags)
		collectDialectTagsFromExpr(s.Cond, approved, tags)
		collectDialectTagsFromStmt(s.Post, approved, tags)
		collectDialectTagsFromBlock(s.Body, approved, tags)
	case *ast.ForRangeStmt:
		collectDialectTagsFromExpr(s.Iter, approved, tags)
		collectDialectTagsFromBlock(s.Body, approved, tags)
	case *ast.ForStmt:
		collectDialectTagsFromExpr(s.Cond, approved, tags)
		collectDialectTagsFromBlock(s.Body, approved, tags)
	case *ast.ReturnStmt:
		for _, expr := range s.Values {
			collectDialectTagsFromExpr(expr, approved, tags)
		}
	case *ast.FuncDeclStmt:
		collectDialectTagsFromBlock(s.Body, approved, tags)
	case *ast.BlockStmt:
		collectDialectTagsFromBlock(s, approved, tags)
	}
}

func collectDialectTagsFromBlock(block *ast.BlockStmt, approved, tags map[string]bool) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		collectDialectTagsFromStmt(stmt, approved, tags)
	}
}

func collectDialectTagsFromExpr(expr ast.Expr, approved, tags map[string]bool) {
	switch e := expr.(type) {
	case nil:
		return
	case *ast.TaggedStringExpr:
		recordDialectTag(e.Tag, approved, tags)
		collectDialectTagsFromExpr(e.Body, approved, tags)
	case *ast.TaggedBlockExpr:
		recordDialectTag(e.Tag, approved, tags)
		for _, field := range e.Config {
			collectDialectTagsFromExpr(field.Key, approved, tags)
			collectDialectTagsFromExpr(field.Value, approved, tags)
		}
		collectDialectTagsFromBlock(e.Body, approved, tags)
	case *ast.InterpolatedStringExpr:
		for _, part := range e.Parts {
			collectDialectTagsFromExpr(part.Expr, approved, tags)
		}
	case *ast.BinaryExpr:
		collectDialectTagsFromExpr(e.Left, approved, tags)
		collectDialectTagsFromExpr(e.Right, approved, tags)
	case *ast.UnaryExpr:
		collectDialectTagsFromExpr(e.Operand, approved, tags)
	case *ast.ParenExpr:
		collectDialectTagsFromExpr(e.Inner, approved, tags)
	case *ast.IndexExpr:
		collectDialectTagsFromExpr(e.Table, approved, tags)
		collectDialectTagsFromExpr(e.Index, approved, tags)
	case *ast.FieldExpr:
		collectDialectTagsFromExpr(e.Table, approved, tags)
	case *ast.CallExpr:
		if tag, ok := dialectEvalCallTag(e); ok {
			recordDialectTag(tag, approved, tags)
		}
		collectDialectTagsFromExpr(e.Func, approved, tags)
		for _, arg := range e.Args {
			collectDialectTagsFromExpr(arg, approved, tags)
		}
	case *ast.MethodCallExpr:
		collectDialectTagsFromExpr(e.Object, approved, tags)
		for _, arg := range e.Args {
			collectDialectTagsFromExpr(arg, approved, tags)
		}
	case *ast.FuncLitExpr:
		collectDialectTagsFromBlock(e.Body, approved, tags)
	case *ast.ListLitExpr:
		for _, value := range e.Values {
			collectDialectTagsFromExpr(value, approved, tags)
		}
	case *ast.TableLitExpr:
		for _, field := range e.Fields {
			collectDialectTagsFromExpr(field.Key, approved, tags)
			collectDialectTagsFromExpr(field.Value, approved, tags)
		}
	case *ast.DenseLitExpr:
		for _, value := range e.Values {
			collectDialectTagsFromExpr(value, approved, tags)
		}
	case *ast.RecvExpr:
		collectDialectTagsFromExpr(e.Channel, approved, tags)
	case *ast.MakeChanExpr:
		collectDialectTagsFromExpr(e.Size, approved, tags)
	}
}

func dialectEvalCallTag(call *ast.CallExpr) (string, bool) {
	field, ok := call.Func.(*ast.FieldExpr)
	if !ok || (field.Field != "eval" && field.Field != "eval_block" && field.Field != "eval_raw") || len(call.Args) == 0 {
		return "", false
	}
	ident, ok := field.Table.(*ast.IdentExpr)
	if !ok || ident.Name != "dialect" {
		return "", false
	}
	lit, ok := call.Args[0].(*ast.StringLit)
	if !ok {
		return "", false
	}
	return lit.Value, true
}

func recordDialectTag(tag string, approved, tags map[string]bool) {
	if tag == "$" {
		tag = "sh"
	}
	if approved[tag] {
		tags[tag] = true
	}
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
