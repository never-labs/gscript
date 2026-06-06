package main

import (
	"bytes"
	"encoding/json"
	"fmt"
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
	t.Run("data/q_vector_basics.leia", func(t *testing.T) {
		stdout := execExampleProjectGlobal(root, filepath.Join("examples", "data", "q_vector_basics.leia"), "q_vector_basics_summary")(t)
		if !strings.Contains(stdout, "q vector total=") {
			t.Fatalf("summary = %q, want q vector summary", stdout)
		}
	})
}

func TestPracticalExampleProjects(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	projects := []struct {
		name     string
		run      func(t *testing.T) string
		wantText string
	}{
		{"api-offline-client", execExampleProjectGlobal(root, filepath.Join("examples", "api", "offline_client.leia"), "api_client_summary"), "prj-7 2 tickets"},
		{"data-quality-report", execExampleProjectGlobal(root, filepath.Join("examples", "workflow", "service_quality_report.leia"), "workflow_report_summary"), "services=3 requests=8 errors=3 breaches=2 over_budget=2"},
		{"column-query-analytics", execExampleProjectGlobal(root, filepath.Join("examples", "data_processing", "column_query", "trade_analytics.leia"), "trade_analytics_summary"), "rows=18 channels=4 best_channel=email roas=10.35 top_day=4 top_roas=15.75 risk_segments=4 enterprise_revenue=29100"},
		{"supply-chain-audit", execExampleProjectGlobal(root, filepath.Join("examples", "security", "supply_chain_audit.leia"), "supply_chain_audit_summary"), "release=2026.06.05 deps=6 direct=3 vulns=4 blockers=2 disallowed=1 fixable=3 risk=27"},
		{"vendor-onboarding-audit", execExampleProjectGlobal(root, filepath.Join("examples", "security", "vendor_onboarding_audit.leia"), "vendor_onboarding_audit_summary"), "evidence=9 expired_certs=1 missing_controls=3 open_findings=4 reviewer=security@example.test stale_evidence=3 top_score=185 top_vendor=V-103 vendors=4"},
		{"release-ci-regression", testExampleProject(root, filepath.Join("examples", "testing")), "release_ci_regression_workflow_test.leia"},
		{"static-site-docs-generation", execExampleProjectGlobal(root, filepath.Join("examples", "site", "static_docs_generator.leia"), "site_generation_summary"), "pages=3 published=2 drafts=1 assets=2"},
		{"web-access-log-report", execExampleProjectGlobal(root, filepath.Join("examples", "web", "access_log_report.leia"), "web_access_report_summary"), "routes=4 requests=10 errors=3 slow=3 cache_hits=4 top=/api/orders"},
		{"web-route-workbench", execExampleProjectGlobal(root, filepath.Join("examples", "web", "route_workbench.leia"), "web_route_workbench_summary"), "routes=5 events=6 created=bk-303 updated_stock=8 deleted=bk-202 method_status=405 html=200"},
		{"web-serve-dialect-app", execExampleProjectGlobal(root, filepath.Join("examples", "web", "serve_dialect_app.leia"), "serve_dialect_app_summary"), "serve routes=3 events=4 status=delivered method_status=405 html=200"},
		{"web-tiny-fullstack-app", execExampleProjectGlobal(root, filepath.Join("examples", "web", "tiny_fullstack_app.leia"), "tiny_fullstack_summary"), "tiny_fullstack posts=4 published=2 json=3 form=4 static=200 method=405"},
		{"db-q-frame-project", execExampleProjectGlobal(root, filepath.Join("examples", "data", "db_q_frame_project", "main.leia"), "db_q_frame_summary"), "db_q_frame rows=4 channels=3 best=1 revenue=550 excel_rows=3"},
		{"q-trade-analytics-project", execExampleProjectGlobal(root, filepath.Join("examples", "data", "q_trade_analytics_project", "main.leia"), "q_trade_analytics_summary"), "trades=5 symbols=3 leader=MSFT notional=21360.0 total_size=540 large=3"},
		{"release-gate-project", runCLIExampleByID("repo-tooling-release_gate_project-main"), "release_gate_project checks=6 domains=6 top=performance ms=3770 excel_rows=6 web=200 agent=done shell=release-gate cmd=cmd-ok"},
		{"ai-agent-composition", evaluateReplayExampleProject(root, filepath.Join("examples", "evaluate", "agent_replay.leia"), filepath.Join("examples", "evaluate", "agent_replay.records.json")), "agent consumes replay"},
		{"ai-project-regression", evaluateReplayExampleProject(root, filepath.Join("examples", "evaluate", "project_agent_regression.leia"), filepath.Join("examples", "evaluate", "project_agent_regression.records.json")), "project agent regression consumes replay"},
		{"concurrency-pipeline", execExampleProjectGlobal(root, filepath.Join("examples", "concurrency", "goroutines_channels.leia"), "workers"), "4"},
		{"builtin-database", execExampleProjectGlobal(root, filepath.Join("examples", "database", "package_managed", "main.leia"), "database_summary"), "ledger accounts=5 entries=10 top_account=rev top_total=2510.00 top_project=alpha net=2290.00 uncleared=4 error=constraint"},
		{"package-managed-macos", modCheckExampleProject(root, filepath.Join("examples", "macos", "package_managed")), "github.com/never-labs/leia-macos/automation"},
	}
	for _, project := range projects {
		t.Run(project.name, func(t *testing.T) {
			stdout := project.run(t)
			if project.wantText != "" && !strings.Contains(stdout, project.wantText) {
				t.Fatalf("%s output = %q, want containing %q", project.name, stdout, project.wantText)
			}
			if strings.TrimSpace(stdout) == "" {
				t.Fatalf("%s produced no observable output", project.name)
			}
		})
	}
}

func runCLIExampleByID(id string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runExamplesCommand([]string{"run", id}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("examples run %s code = %d, stderr = %q stdout = %q", id, code, stderr.String(), stdout.String())
		}
		return stdout.String()
	}
}

func execExampleProjectGlobal(root, rel, global string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		vm := leia.New(
			leia.WithLibs(leia.LibAll),
			leia.WithVM(),
		)
		if err := vm.ExecFile(filepath.Join(root, rel)); err != nil {
			t.Fatalf("ExecFile %s: %v", rel, err)
		}
		value, err := vm.Get(global)
		if err != nil {
			t.Fatalf("get %s from %s: %v", global, rel, err)
		}
		return valueString(value)
	}
}

func testExampleProject(root, rel string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runTestCommand([]string{"--json", "--golden=require", filepath.Join(root, rel)}, cliRunOptions{UseVM: true}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("test command %s code = %d, stderr = %q stdout = %q", rel, code, stderr.String(), stdout.String())
		}
		return stdout.String()
	}
}

func evaluateReplayExampleProject(root, rel, recordsRel string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runEvaluateCommand([]string{"--json", "--replay", filepath.Join(root, recordsRel), filepath.Join(root, rel)}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("evaluate replay %s code = %d, stderr = %q stdout = %q", rel, code, stderr.String(), stdout.String())
		}
		return stdout.String()
	}
}

func modCheckExampleProject(root, rel string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		code := runModCommand([]string{"list", "--json", filepath.Join(root, rel)}, &stdout, &stderr)
		if code != 0 {
			t.Fatalf("mod list %s code = %d, stderr = %q stdout = %q", rel, code, stderr.String(), stdout.String())
		}
		return stdout.String()
	}
}

func valueString(value any) string {
	return fmt.Sprint(value)
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

	for _, tag := range []string{"sh", "cmd", "glob", "path", "json", "csv", "url", "html", "base64", "sql", "markdown", "prompt", "quote"} {
		if !exampleTags[tag] {
			t.Fatalf("runnable dialect examples must keep representative builtin tag %q covered", tag)
		}
	}
	if exampleTags["junit"] {
		t.Fatalf("junit is a compat/interop dialect and must not be promoted through runnable workflow examples")
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

func TestBuiltinDatabaseExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "database", "package_managed")
	sourcePath := filepath.Join(dir, "main.leia")

	if _, ok := catalog.Module("db"); !ok {
		t.Fatal("stdlib catalog must include built-in db module")
	}
	for _, forbidden := range []string{"database", "sqlite", "postgres", "mysql"} {
		if _, ok := catalog.Module(forbidden); ok {
			t.Fatalf("stdlib catalog contains external database runtime module %q; non-default database runtimes should stay package-managed", forbidden)
		}
	}
	assertLeiaFileParses(t, sourcePath)
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read database example source: %v", err)
	}
	source := string(sourceBytes)
	for _, want := range []string{
		"conn := db.memory()",
		"conn.exec(",
		"conn.query(",
		"conn.one(",
		"create table accounts",
		"create table ledger_entries",
		"group by",
		"duplicate_account_err",
		"query: \"insert into",
		"params:",
		"//leia:cap db.open,db.exec,db.query,db.one",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("database example source missing %q\nsource:\n%s", want, source)
		}
	}
	for _, forbidden := range []string{`import "github.com/never-labs/leia-db/sqlite"`, `import "database"`, `import "db"`, `import "sqlite"`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("database example source imports obsolete database runtime module %q", forbidden)
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
		t.Fatalf("verify = %+v, want built-in database example manifest to verify", verify)
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
	if !list.OK || containsResolvedRequire(list.Requires, "github.com/never-labs/leia-db/sqlite", "v0.1.0") {
		t.Fatalf("list = %+v, want no external Leia SQLite runtime requirement", list)
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
	if !moduleHasCapabilities(caps.Modules, "example.com/leia/examples/database/package-managed", "db.open", "db.exec", "db.query", "db.one") {
		t.Fatalf("capability modules = %+v, want database domain capabilities on the example manifest", caps.Modules)
	}
}

func TestPackageManagedMacOSAutomationExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "examples", "macos", "package_managed")
	sourcePath := filepath.Join(dir, "main.leia")

	for _, forbidden := range []string{"applescript", "automation", "macos", "osascript"} {
		if _, ok := catalog.Module(forbidden); ok {
			t.Fatalf("stdlib catalog contains macOS automation-shaped module %q; AppleScript automation should stay package-managed", forbidden)
		}
	}
	assertLeiaFileParses(t, sourcePath)
	sourceBytes, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read macOS automation example source: %v", err)
	}
	source := string(sourceBytes)
	for _, want := range []string{
		`import macos "github.com/never-labs/leia-macos/automation"`,
		"macos.applescript(",
		"macos.plan(",
		"clipboard_probe",
		"//leia:cap macos.automation,process.exec",
	} {
		if !strings.Contains(source, want) {
			t.Fatalf("macOS automation example source missing %q\nsource:\n%s", want, source)
		}
	}
	for _, forbidden := range []string{`import "applescript"`, `import "automation"`, `import "macos"`, `import "osascript"`} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("macOS automation example source imports builtin-shaped module %q; AppleScript automation should stay package-managed", forbidden)
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
		t.Fatalf("verify = %+v, want package-managed macOS automation example manifest to verify", verify)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod verify code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK {
		t.Fatalf("verify = %+v, want explicit package-managed macOS automation verify to pass", verify)
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
	if !list.OK || !containsResolvedRequire(list.Requires, "github.com/never-labs/leia-macos/automation", "v0.1.0") {
		t.Fatalf("list = %+v, want external Leia macOS automation runtime requirement", list)
	}
	if len(list.Requires) != 1 || list.Requires[0].Kind != "replace" ||
		len(list.Replaces) != 1 || !list.Replaces[0].Local ||
		!strings.HasSuffix(list.Replaces[0].Root, filepath.Join("package_managed", "adapter")) {
		t.Fatalf("list requires = %+v replaces = %+v, want local replace-backed macOS automation adapter", list.Requires, list.Replaces)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"gomod", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod gomod code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	for _, want := range []string{
		"module example.com/leia/examples/macos/package-managed\n",
		"go 1.25\n",
		"github.com/never-labs/leia-macos/automation v0.1.0",
		"github.com/never-labs/leia v0.0.0-20260601065425-1c9cadbd856f",
		"replace github.com/never-labs/leia-macos/automation v0.1.0 => ./adapter",
	} {
		if got := stdout.String(); !strings.Contains(got, want) {
			t.Fatalf("generated go.mod missing %q in:\n%s", want, got)
		}
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
		t.Fatalf("capability report = %+v, want package-managed macOS automation capabilities to report", caps)
	}
	if !moduleHasCapabilities(caps.Modules, "example.com/leia/examples/macos/package-managed", "macos.automation", "process.exec") {
		t.Fatalf("capability modules = %+v, want macOS automation capabilities on the example manifest", caps.Modules)
	}
	if !moduleHasCapabilities(caps.Modules, "github.com/never-labs/leia-macos/automation", "macos.automation", "process.exec") {
		t.Fatalf("capability modules = %+v, want external package-managed macOS automation runtime module capabilities", caps.Modules)
	}
	if !caps.Matrix["example.com/leia/examples/macos/package-managed"]["macos.automation"] ||
		!caps.Matrix["github.com/never-labs/leia-macos/automation"]["process.exec"] {
		t.Fatalf("capability matrix = %+v, want root and adapter capability summary", caps.Matrix)
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
	if !report.OK || report.Total != 3 || report.Passed != 3 {
		t.Fatalf("report = %+v, want three passing testing examples", report)
	}
	if !testReportHasPassingFile(report, "jsonl_workflow_test.leia") {
		t.Fatalf("files = %+v, want jsonl_workflow_test.leia to pass", report.Files)
	}
	if !testReportHasPassingFile(report, "jsonl_golden_eval_replay_test.leia") {
		t.Fatalf("files = %+v, want jsonl_golden_eval_replay_test.leia to pass", report.Files)
	}
	if !testReportHasPassingFile(report, "release_ci_regression_workflow_test.leia") {
		t.Fatalf("files = %+v, want release_ci_regression_workflow_test.leia to pass", report.Files)
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

func TestWorkflowStatusRollupExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibDialect),
		leia.WithVM(),
	)
	if err := vm.ExecFile(filepath.Join(root, "examples", "workflow", "status_rollup.leia")); err != nil {
		t.Fatalf("run status rollup workflow example: %v", err)
	}
	got, err := vm.Get("workflow_summary")
	if err != nil || got != "wf-10=ok/45ms wf-11=fail/41ms" {
		t.Fatalf("workflow_summary = %#v err=%v, want rollup summary", got, err)
	}
}

func TestWorkflowServiceQualityReportExample(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	vm := leia.New(
		leia.WithLibs(leia.LibString|leia.LibTable|leia.LibArray|leia.LibSoA|leia.LibDialect),
		leia.WithVM(),
	)
	if err := vm.ExecFile(filepath.Join(root, "examples", "workflow", "service_quality_report.leia")); err != nil {
		t.Fatalf("run service quality report workflow example: %v", err)
	}
	got, err := vm.Get("workflow_report_summary")
	if err != nil || got != "services=3 requests=8 errors=3 breaches=2 over_budget=2" {
		t.Fatalf("workflow_report_summary = %#v err=%v, want service quality summary", got, err)
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
	if report.Status != "ok" || report.Summary.Files != 3 || report.Summary.ParsedFiles != 3 {
		t.Fatalf("report = %+v, want three parsed workflow files with ok status", report)
	}
	if !evaluateReportHasOKInput(report.Inputs, "support_triage_replay.leia") {
		t.Fatalf("inputs = %+v, want support_triage_replay.leia ok", report.Inputs)
	}
	if !evaluateReportHasOKInput(report.Inputs, "status_rollup.leia") {
		t.Fatalf("inputs = %+v, want status_rollup.leia ok", report.Inputs)
	}
	if !evaluateReportHasOKInput(report.Inputs, "service_quality_report.leia") {
		t.Fatalf("inputs = %+v, want service_quality_report.leia ok", report.Inputs)
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

func TestEvaluateAdvancedCLIOptionsStayUsable(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "examples", "evaluate", "basic_assert.leia")
	dir := t.TempDir()
	jsonReport := filepath.Join(dir, "eval.json")
	htmlReport := filepath.Join(dir, "eval.html")

	var stdout, stderr bytes.Buffer
	code := runEvaluateCommand([]string{"--json", "--report", jsonReport, "--gate", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --json --report --gate code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty when --report writes JSON", stdout.String())
	}
	reportBytes, err := os.ReadFile(jsonReport)
	if err != nil {
		t.Fatalf("read JSON report: %v", err)
	}
	if !strings.Contains(string(reportBytes), `"status": "ok"`) {
		t.Fatalf("JSON report missing ok status:\n%s", string(reportBytes))
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--format=text", "--filter", "basic", "--parallel=2", "--baseline", jsonReport, "--regression-threshold", "0.05", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate text/filter/baseline code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "evaluate: ok") || !strings.Contains(stdout.String(), "PASS basic assert") {
		t.Fatalf("text report missing expected summary:\n%s", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--format=html", "--output", htmlReport, path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --format=html --output code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	htmlBytes, err := os.ReadFile(htmlReport)
	if err != nil {
		t.Fatalf("read HTML report: %v", err)
	}
	if !strings.Contains(string(htmlBytes), "<!doctype html>") || !strings.Contains(string(htmlBytes), "basic assert") {
		t.Fatalf("HTML report missing expected content:\n%s", string(htmlBytes))
	}

	stdout.Reset()
	stderr.Reset()
	code = runEvaluateCommand([]string{"--compare", jsonReport, jsonReport}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("evaluate --compare code = %d, stderr = %q stdout = %q", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"status": "ok"`) {
		t.Fatalf("compare report missing ok status:\n%s", stdout.String())
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
		streamEvents  int
	}{
		{name: "llm", source: "llm_replay.leia", records: "llm_replay.records.json", replayedTurns: 1},
		{name: "agent", source: "agent_replay.leia", records: "agent_replay.records.json", replayedTurns: 1},
		{name: "multiturn", source: "multiturn_replay.leia", records: "multiturn_replay.records.json", replayedTurns: 2},
		{name: "project-agent-regression", source: "project_agent_regression.leia", records: "project_agent_regression.records.json", replayedTurns: 2, streamEvents: 4},
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
					LLM    *struct {
						StreamEvents int `json:"stream_events"`
					} `json:"llm"`
				} `json:"cases"`
				Findings []struct {
					Kind string `json:"kind"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
				t.Fatalf("stdout is not JSON evaluate report: %v; stdout = %q", err, stdout.String())
			}
			if report.Status != "ok" || report.Summary.EvaluateBlocks < 1 || report.Summary.CasesSelected != report.Summary.EvaluateBlocks || report.Summary.CasesPassed != report.Summary.CasesSelected || report.Summary.CasesFailed != 0 {
				t.Fatalf("report = %+v, want passing evaluate cases", report)
			}
			if len(report.Cases) != report.Summary.CasesSelected {
				t.Fatalf("cases = %+v summary=%+v, want all selected cases reported", report.Cases, report.Summary)
			}
			for _, c := range report.Cases {
				if c.Status != "passed" {
					t.Fatalf("cases = %+v, want all passed", report.Cases)
				}
			}
			if report.LLM == nil || report.LLM.ReplayedTurns != tc.replayedTurns || report.LLM.RemainingTurns != 0 {
				t.Fatalf("llm = %+v, want replayed=%d remaining=0", report.LLM, tc.replayedTurns)
			}
			if tc.streamEvents > 0 {
				if len(report.Cases) == 0 || report.Cases[0].LLM == nil || report.Cases[0].LLM.StreamEvents != tc.streamEvents {
					t.Fatalf("cases = %+v, want first case to report %d stream events", report.Cases, tc.streamEvents)
				}
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
		"re", "regexp", "json", "jsonptr", "jsonl", "csv", "tsv", "mdtable", "markdown", "md", "lines", "split", "words", "nums", "numbers", "kv", "logfmt", "env", "ini", "yaml", "yml", "semver", "duration", "timestamp", "rfc3339", "tap", "junit", "xml", "template",
		"url", "html_escape", "html", "urlquery", "form", "urlform", "urlpath", "mime", "mailaddr", "emailaddr", "headers", "http_headers", "cookie", "cookies", "httpmsg", "sse", "multipart", "jwt",
		"ipaddr", "cidr", "hostport", "serve",
		"base64", "hash", "hex", "base32", "uuid", "gzip", "zlib", "deflate", "binary", "q", "pem", "xlsx", "excel", "sql",
		"prompt", "quote", "model", "turn", "tool", "agent",
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
	paths = append(paths, filepath.Join(root, "examples", "data", "q_vector_basics.leia"))
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

func assertLeiaFileParses(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	tokens, err := lexer.New(string(data)).Tokenize()
	if err != nil {
		t.Fatalf("lex %s: %v", path, err)
	}
	if _, err := parser.New(tokens).Parse(); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
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
