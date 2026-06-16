package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	stdinstall "github.com/never-labs/leia/internal/stdlib/install"
)

func TestCapabilitiesJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCapabilitiesCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var caps cliCapabilities
	if err := json.Unmarshal(stdout.Bytes(), &caps); err != nil {
		t.Fatalf("stdout is not JSON capabilities: %v; stdout = %q", err, stdout.String())
	}
	if caps.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", caps.SchemaVersion)
	}
	if caps.Status != "pass" || caps.CommandCount != len(caps.Commands) || caps.StdlibCount != len(caps.StdlibModules) || caps.DialectCount != len(caps.Dialects) {
		t.Fatalf("capability report counts/status = status %q commands %d/%d stdlib %d/%d dialects %d/%d", caps.Status, caps.CommandCount, len(caps.Commands), caps.StdlibCount, len(caps.StdlibModules), caps.DialectCount, len(caps.Dialects))
	}
	if caps.Platform.GOOS != goruntime.GOOS || caps.Platform.GOARCH != goruntime.GOARCH {
		t.Fatalf("platform = %s/%s, want %s/%s", caps.Platform.GOOS, caps.Platform.GOARCH, goruntime.GOOS, goruntime.GOARCH)
	}
	if !caps.Execution.Interpreter || !caps.Execution.BytecodeVM {
		t.Fatalf("execution capabilities = %+v, want interpreter and bytecode VM", caps.Execution)
	}
	if len(caps.StdlibModules) == 0 {
		t.Fatal("stdlib_modules is empty")
	}
	for _, want := range []string{"base", "host", "llm", "data", "vendor", "compat"} {
		if !capabilitiesHaveStdlibLayer(caps.StdlibLayers, want) {
			t.Fatalf("stdlib_layers = %#v, want layer %q", caps.StdlibLayers, want)
		}
	}
	if !capabilitiesHaveStdlibModule(caps.StdlibLayers, "llm", "llm") || !capabilitiesHaveStdlibModule(caps.StdlibLayers, "host", "fs") || !capabilitiesHaveStdlibModule(caps.StdlibLayers, "data", "soa") {
		t.Fatalf("stdlib_layers = %#v, want llm/llm, host/fs, and data/soa", caps.StdlibLayers)
	}
	for _, tc := range []struct {
		name   string
		module string
		member string
	}{
		{name: "sqrt", module: "math", member: "sqrt"},
		{name: "near", module: "math", member: "near"},
		{name: "mat", module: "linalg", member: "matrix"},
		{name: "eye", module: "linalg", member: "eye"},
		{name: "matmul", module: "linalg", member: "matmul"},
		{name: "mean", module: "stats", member: "mean"},
		{name: "describe", module: "stats", member: "describe"},
		{name: "randn", module: "rand", member: "normal_vec"},
		{name: "sample", module: "rand", member: "sample"},
		{name: "append", module: "table", member: "append"},
	} {
		if !capabilitiesHaveDefaultImport(caps.DefaultImports, tc.name, tc.module, tc.member) {
			t.Fatalf("default_imports = %#v, want %s -> %s.%s", caps.DefaultImports, tc.name, tc.module, tc.member)
		}
	}
	if len(caps.Dialects) == 0 {
		t.Fatal("dialects is empty")
	}
	for _, tc := range []struct {
		name         string
		category     string
		capabilities []string
		eval         bool
		block        bool
	}{
		{name: "sh", category: "host", capabilities: []string{"process.shell"}, eval: true},
		{name: "cmd", category: "host", capabilities: []string{"process.exec", "env.write"}, eval: true},
		{name: "glob", category: "host", capabilities: []string{"fs.read"}, eval: true},
		{name: "env", category: "host", capabilities: []string{"env.read"}, eval: true, block: true},
		{name: "serve", category: "web", capabilities: []string{"net.listen"}, eval: true, block: true},
		{name: "sql", category: "database", eval: true, block: true},
		{name: "q", category: "data", eval: true, block: true},
		{name: "xlsx", category: "data", eval: true},
		{name: "excel", category: "data", eval: true},
		{name: "turn", category: "llm", capabilities: []string{"llm.turn"}, block: true},
		{name: "agent", category: "llm", capabilities: []string{"llm.turn"}, block: true},
	} {
		dialect, ok := capabilitiesDialect(caps.Dialects, tc.name)
		if !ok {
			t.Fatalf("dialects = %#v, want tag %q", caps.Dialects, tc.name)
		}
		if dialect.Category != tc.category || !dialect.Builtin || dialect.Eval != tc.eval || dialect.Block != tc.block {
			t.Fatalf("dialect %q = %+v, want category=%q builtin=true eval=%t block=%t", tc.name, dialect, tc.category, tc.eval, tc.block)
		}
		for _, want := range tc.capabilities {
			if !containsString(dialect.Capabilities, want) {
				t.Fatalf("dialect %q capabilities = %#v, want %q", tc.name, dialect.Capabilities, want)
			}
		}
	}
	for _, want := range []string{"tagged_strings", "tagged_blocks", "shell_strings", "llm_stdlib_calls", "dialect_eval"} {
		if !caps.LLM.Enabled || !containsString(caps.LLM.Syntax, want) {
			t.Fatalf("llm syntax = %#v, want %q", caps.LLM.Syntax, want)
		}
	}
	if !containsString(caps.LLM.ToolMetadata, "leia:requires") || !containsString(caps.LLM.StaticValidation, "static_tool_capabilities") || !containsString(caps.LLM.Tooling, "lint-sarif") {
		t.Fatalf("llm capabilities = %+v, want metadata, static validation, and tooling entries", caps.LLM)
	}
	for _, want := range []string{"llm.register_models", "llm.tool", "llm.agent", "llm.turn", "llm.toolof", "llm.agent_as_tool", "llm.validate_output", "dialect.eval", "msg.assistant_call", "msg.tool_result", "history.find", "history.find_all", "history.last", "history.append"} {
		if !containsString(caps.LLM.RuntimePrimitives, want) {
			t.Fatalf("llm runtime primitives = %#v, want %q", caps.LLM.RuntimePrimitives, want)
		}
	}
	for _, want := range []string{
		"bench",
		"capabilities",
		"check",
		"ci",
		"config",
		"diag",
		"diagnose",
		"doc",
		"env",
		"eval",
		"evaluate",
		"examples",
		"fmt",
		"help",
		"inspect",
		"lint",
		"lsp",
		"mod",
		"playground",
		"repl",
		"run",
		"test",
		"version",
	} {
		if !containsString(caps.Commands, want) {
			t.Fatalf("commands = %#v, want %q", caps.Commands, want)
		}
	}
	if !containsString(caps.Tooling.Linter.Formats, "json") || !containsString(caps.Tooling.Linter.Formats, "sarif") || !containsString(caps.Tooling.Linter.Codes, "LEIA1001") || !containsString(caps.Tooling.Linter.Codes, "LEIA2001") {
		t.Fatalf("linter capabilities = %+v, want json, LEIA1001, and LEIA2001", caps.Tooling.Linter)
	}
	if !caps.Tooling.Formatter.Stdin || !caps.Tooling.Formatter.Check || !caps.Tooling.Formatter.Write || !containsString(caps.Tooling.Formatter.Formats, "source") || !containsString(caps.Tooling.Formatter.Reports, "json") {
		t.Fatalf("formatter capabilities = %+v, want stdin/check/write source formatter with json reports", caps.Tooling.Formatter)
	}
	if !caps.Tooling.Test.GoldenStdout || !caps.Tooling.Test.Directory || !caps.Tooling.Test.List || caps.Tooling.Test.SeedEnv != "LEIA_TEST_SEED" || !containsString(caps.Tooling.Test.GoldenModes, "update") || !containsString(caps.Tooling.Test.Reports, "json") {
		t.Fatalf("test capabilities = %+v, want golden stdout modes, directory, list, json reports, and seed env", caps.Tooling.Test)
	}
	if caps.Tooling.Config.FileName != "leia.toml" || !containsString(caps.Tooling.Config.Formats, "json") {
		t.Fatalf("config capabilities = %+v, want leia.toml/json", caps.Tooling.Config)
	}
	if caps.Tooling.ReportCount != len(caps.Tooling.Reports) {
		t.Fatalf("tooling report_count = %d, want %d", caps.Tooling.ReportCount, len(caps.Tooling.Reports))
	}
	seenReports := map[string]bool{}
	for _, report := range caps.Tooling.Reports {
		if report.Command == "" || report.SchemaVersion <= 0 || !containsString(report.Formats, "json") {
			t.Fatalf("report capability = %+v, want command, json format, and schema version", report)
		}
		if seenReports[report.Command] {
			t.Fatalf("duplicate report capability command %q", report.Command)
		}
		seenReports[report.Command] = true
	}
	for _, want := range []string{
		"leia capabilities --json",
		"leia check --json",
		"leia ci --list --json",
		"leia config --json",
		"leia diag bundle --json",
		"leia doc check --json",
		"leia doc generate --format=json",
		"leia env --json",
		"leia evaluate --json",
		"leia examples check --json",
		"leia examples list --json",
		"leia fmt --json",
		"leia inspect bytecode --json",
		"leia inspect directives --json",
		"leia lint --json",
		"leia mod capability --json",
		"leia mod check --json",
		"leia mod download --json",
		"leia mod explain --json",
		"leia mod gomod --json",
		"leia mod graph --json",
		"leia mod list --json",
		"leia mod lock --json",
		"leia mod tidy --json",
		"leia mod vendor --json",
		"leia mod verify --json",
		"leia test --json",
		"leia test --list --json",
		"leia version --json",
		"scripts/editor_check.sh --json",
		"scripts/install.sh --dry-run --json",
		"scripts/performance_gate.sh --json",
		"scripts/production_check.sh --list --json",
		"scripts/public_release_blockers_check.sh --json",
		"scripts/q_conformance_gate.sh --json",
		"scripts/release_artifacts.sh --dry-run --json",
		"scripts/release_artifacts_check.sh --json",
		"scripts/release_distribution_check.sh --json",
		"scripts/release_notes_check.sh --json",
		"scripts/worktree_audit.sh --json",
	} {
		if !capabilitiesHaveReport(caps.Tooling.Reports, want) {
			t.Fatalf("report capabilities = %#v, want command %q", caps.Tooling.Reports, want)
		}
	}
	envReport := capabilitiesReport(caps.Tooling.Reports, "leia env --json")
	for _, want := range []string{"capabilities.command_count", "capabilities.stdlib_module_count", "capabilities.dialect_count", "capabilities.tooling.report_count"} {
		if envReport == nil || !containsString(envReport.CountFields, want) {
			t.Fatalf("env report capability = %+v, want count field %q", envReport, want)
		}
	}
}

func TestCapabilitiesDefaultImportsStaySyncedWithPrelude(t *testing.T) {
	caps := buildCapabilities()
	aliases := stdinstall.DefaultAliases()
	if len(caps.DefaultImports) != len(aliases) {
		t.Fatalf("default_imports length = %d, want %d", len(caps.DefaultImports), len(aliases))
	}
	seen := map[string]cliDefaultImport{}
	for _, item := range caps.DefaultImports {
		if item.Name == "" || item.Module == "" || item.Member == "" {
			t.Fatalf("default import must include name, module, and member: %+v", item)
		}
		if previous, ok := seen[item.Name]; ok {
			t.Fatalf("duplicate default import %q: %+v and %+v", item.Name, previous, item)
		}
		seen[item.Name] = item
	}
	for _, alias := range aliases {
		got, ok := seen[alias.Name]
		if !ok {
			t.Fatalf("capabilities default_imports missing %s -> %s.%s", alias.Name, alias.Module, alias.Member)
		}
		if got.Module != alias.Module || got.Member != alias.Member {
			t.Fatalf("capabilities default import %s = %s.%s, want %s.%s", alias.Name, got.Module, got.Member, alias.Module, alias.Member)
		}
	}
}

func capabilitiesDialect(dialects []cliDialectCapability, name string) (cliDialectCapability, bool) {
	for _, dialect := range dialects {
		if dialect.Name == name {
			return dialect, true
		}
	}
	return cliDialectCapability{}, false
}

func TestCapabilitiesDialectsCoverFeatureMatrixBuiltinTags(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	matrixTags := loadFeatureMatrixBuiltinDialectTags(t, root)
	caps := buildCapabilities()
	reportTags := make(map[string]bool, len(caps.Dialects))
	for _, dialect := range caps.Dialects {
		if dialect.Name == "" || dialect.Category == "" {
			t.Fatalf("dialect capability = %+v, want name and category", dialect)
		}
		reportTags[dialect.Name] = true
	}
	for tag := range matrixTags {
		if !reportTags[tag] {
			t.Fatalf("capabilities dialects missing feature matrix builtin tag %q", tag)
		}
	}
}

func capabilitiesHaveStdlibLayer(layers []cliStdlibLayer, name string) bool {
	for _, layer := range layers {
		if layer.Name == name {
			return true
		}
	}
	return false
}

func capabilitiesHaveReport(reports []cliReportCapability, command string) bool {
	return capabilitiesReport(reports, command) != nil
}

func capabilitiesReport(reports []cliReportCapability, command string) *cliReportCapability {
	for _, report := range reports {
		if report.Command == command {
			return &report
		}
	}
	return nil
}

func capabilitiesHaveStdlibModule(layers []cliStdlibLayer, layerName, moduleName string) bool {
	for _, layer := range layers {
		if layer.Name != layerName {
			continue
		}
		for _, module := range layer.Modules {
			if module.Name == moduleName {
				return true
			}
		}
	}
	return false
}

func capabilitiesHaveDefaultImport(imports []cliDefaultImport, name, module, member string) bool {
	for _, item := range imports {
		if item.Name == name && item.Module == module && item.Member == member {
			return true
		}
	}
	return false
}

func TestCapabilitiesRejectsExtraArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runCapabilitiesCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia capabilities") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestVersionCommandJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runVersionCommand([]string{"--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runVersionCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report cliVersionReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON version report: %v; stdout = %q", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.Version == "" || report.GoVersion == "" || report.GOOS != goruntime.GOOS || report.GOARCH != goruntime.GOARCH {
		t.Fatalf("report = %+v, want stable version metadata", report)
	}
}

func TestVersionCommandRejectsExtraArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runVersionCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runVersionCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia version") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestEnvCommandJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "leia.toml"), []byte("[project]\nname = \"demo\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runEnvCommand([]string{"--json", "--path", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEnvCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report cliEnvReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON env report: %v; stdout = %q", err, stdout.String())
	}
	if report.SchemaVersion != 1 || report.Status != "pass" || report.Version.Status != "pass" || report.Version.GoVersion == "" || report.WorkingDir == "" {
		t.Fatalf("report = %+v, want stable environment metadata", report)
	}
	if !report.Project.Found || report.Project.Name != "demo" || report.Project.Root != dir {
		t.Fatalf("project = %+v, want discovered demo project at %s", report.Project, dir)
	}
	if !containsString(report.Capabilities.Commands, "env") {
		t.Fatalf("commands = %#v, want env", report.Capabilities.Commands)
	}
}

func TestEnvCommandRejectsExtraArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEnvCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runEnvCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: leia env") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestHelpCommandListsCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runHelpCommand(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runHelpCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"usage: leia <command>", "run", "version", "help"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want %q", out, want)
		}
	}
}

func TestCommandMetadataStaysInSync(t *testing.T) {
	topics := cliHelpTopics()
	commands := cliCommandNames()
	if len(commands) == 0 {
		t.Fatal("cliCommandNames is empty")
	}
	caps := buildCapabilities()
	if len(caps.Commands) != len(commands) {
		t.Fatalf("capability commands = %#v, want %#v", caps.Commands, commands)
	}
	for i, command := range commands {
		if caps.Commands[i] != command {
			t.Fatalf("capability commands = %#v, want %#v", caps.Commands, commands)
		}
		topic := topics[command]
		if topic.Command != command || topic.Usage == "" || topic.Summary == "" {
			t.Fatalf("topic for %q = %+v, want complete metadata", command, topic)
		}
	}
	doc := string(generateCLIReferenceMarkdown())
	if strings.Contains(doc, "No summary available") {
		t.Fatalf("generated CLI reference has missing summary: %q", doc)
	}
	for _, command := range commands {
		if !strings.Contains(doc, "`"+command+"`") {
			t.Fatalf("generated CLI reference missing %q: %q", command, doc)
		}
	}
}

func TestHelpCommandShowsCommandUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runHelpCommand([]string{"run"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runHelpCommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "usage: leia run") || !strings.Contains(out, "Run a script file") {
		t.Fatalf("stdout = %q, want run usage", out)
	}
}

func TestLSPCommandHelpDoesNotStartServer(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runLSPCommand([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runLSPCommand help code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "usage: leia lsp") {
		t.Fatalf("stdout = %q, want lsp usage", stdout.String())
	}
}

func TestHelpCommandRejectsUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runHelpCommand([]string{"missing"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runHelpCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command", stderr.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
