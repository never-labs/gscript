package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestTestFilesSingleFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := testFiles(path)
	if err != nil {
		t.Fatalf("testFiles err = %v", err)
	}
	if len(files) != 1 || files[0] != path {
		t.Fatalf("testFiles = %#v, want [%q]", files, path)
	}
}

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
	if caps.Platform.GOOS != goruntime.GOOS || caps.Platform.GOARCH != goruntime.GOARCH {
		t.Fatalf("platform = %s/%s, want %s/%s", caps.Platform.GOOS, caps.Platform.GOARCH, goruntime.GOOS, goruntime.GOARCH)
	}
	if !caps.Execution.Interpreter || !caps.Execution.BytecodeVM {
		t.Fatalf("execution capabilities = %+v, want interpreter and bytecode VM", caps.Execution)
	}
	if len(caps.StdlibModules) == 0 {
		t.Fatal("stdlib_modules is empty")
	}
	for _, want := range []string{"base", "host", "ai", "data", "vendor", "compat"} {
		if !capabilitiesHaveStdlibLayer(caps.StdlibLayers, want) {
			t.Fatalf("stdlib_layers = %#v, want layer %q", caps.StdlibLayers, want)
		}
	}
	if !capabilitiesHaveStdlibModule(caps.StdlibLayers, "ai", "llm") || !capabilitiesHaveStdlibModule(caps.StdlibLayers, "host", "fs") || !capabilitiesHaveStdlibModule(caps.StdlibLayers, "data", "soa") {
		t.Fatalf("stdlib_layers = %#v, want ai/llm, host/fs, and data/soa", caps.StdlibLayers)
	}
	for _, want := range []string{"agent", "tool", "turn", "messages_bare_expr", "direct_agent_tools", "toolof"} {
		if !caps.AINative.Enabled || !containsString(caps.AINative.Syntax, want) {
			t.Fatalf("ai_native syntax = %#v, want %q", caps.AINative.Syntax, want)
		}
	}
	if !containsString(caps.AINative.ToolMetadata, "gscript:requires") || !containsString(caps.AINative.StaticValidation, "static_tool_capabilities") || !containsString(caps.AINative.Tooling, "lint-sarif") {
		t.Fatalf("ai_native capabilities = %+v, want metadata, static validation, and tooling entries", caps.AINative)
	}
	for _, want := range []string{"llm.toolof", "llm.agent_as_tool", "llm.validate_output", "msg.assistant_call", "msg.tool_result", "history.find", "history.find_all", "history.last", "history.append"} {
		if !containsString(caps.AINative.RuntimePrimitives, want) {
			t.Fatalf("ai_native runtime primitives = %#v, want %q", caps.AINative.RuntimePrimitives, want)
		}
	}
	if !containsString(caps.Commands, "bench") || !containsString(caps.Commands, "capabilities") || !containsString(caps.Commands, "check") || !containsString(caps.Commands, "ci") || !containsString(caps.Commands, "config") || !containsString(caps.Commands, "diag") || !containsString(caps.Commands, "doc") || !containsString(caps.Commands, "env") || !containsString(caps.Commands, "eval") || !containsString(caps.Commands, "fmt") || !containsString(caps.Commands, "help") || !containsString(caps.Commands, "inspect") || !containsString(caps.Commands, "lint") || !containsString(caps.Commands, "mod") || !containsString(caps.Commands, "repl") || !containsString(caps.Commands, "run") || !containsString(caps.Commands, "test") || !containsString(caps.Commands, "version") {
		t.Fatalf("commands = %#v, want core command set", caps.Commands)
	}
	if !containsString(caps.Tooling.Linter.Formats, "json") || !containsString(caps.Tooling.Linter.Formats, "sarif") || !containsString(caps.Tooling.Linter.Codes, "GS1001") {
		t.Fatalf("linter capabilities = %+v, want json and GS1001", caps.Tooling.Linter)
	}
	if !caps.Tooling.Test.GoldenStdout || !caps.Tooling.Test.Directory || !caps.Tooling.Test.List || caps.Tooling.Test.SeedEnv != "GSCRIPT_TEST_SEED" || !containsString(caps.Tooling.Test.GoldenModes, "update") {
		t.Fatalf("test capabilities = %+v, want golden stdout modes, directory, list, and seed env", caps.Tooling.Test)
	}
	if caps.Tooling.Config.FileName != "gscript.toml" || !containsString(caps.Tooling.Config.Formats, "json") {
		t.Fatalf("config capabilities = %+v, want gscript.toml/json", caps.Tooling.Config)
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

func TestCapabilitiesRejectsExtraArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCapabilitiesCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runCapabilitiesCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: gscript capabilities") {
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
	if report.SchemaVersion != 1 || report.Version == "" || report.GoVersion == "" || report.GOOS != goruntime.GOOS || report.GOARCH != goruntime.GOARCH {
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
	if !strings.Contains(stderr.String(), "usage: gscript version") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestEnvCommandJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[project]\nname = \"demo\"\n"), 0644); err != nil {
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
	if report.SchemaVersion != 1 || report.Version.GoVersion == "" || report.WorkingDir == "" {
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
	if !strings.Contains(stderr.String(), "usage: gscript env") {
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
	for _, want := range []string{"usage: gscript <command>", "run", "version", "help"} {
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
	if !strings.Contains(out, "usage: gscript run") || !strings.Contains(out, "Run a script file") {
		t.Fatalf("stdout = %q, want run usage", out)
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

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GSCRIPT_TEST_HELPER") == "" {
		return
	}
	switch os.Getenv("GSCRIPT_TEST_HELPER") {
	case "bench":
		_, _ = os.Stdout.WriteString("bench helper ok\n")
		os.Exit(0)
	case "ci":
		_, _ = os.Stdout.WriteString("ci helper ok\n")
		os.Exit(0)
	case "diag":
		_, _ = os.Stdout.WriteString("diag helper ok\n")
		os.Exit(0)
	case "doc":
		_, _ = os.Stdout.WriteString("doc helper ok\n")
		os.Exit(0)
	case "docs":
		_, _ = os.Stdout.WriteString("docs helper ok\n")
		os.Exit(0)
	case "manifest":
		_, _ = os.Stdout.WriteString("manifest helper ok\n")
		os.Exit(0)
	default:
		os.Exit(2)
	}
}

func testHelperCommand(t *testing.T, helper string) (string, []string) {
	t.Helper()
	args := []string{"-test.run=TestHelperProcess", "--"}
	t.Setenv("GSCRIPT_TEST_HELPER", helper)
	return os.Args[0], args
}

func TestConfigCommandJSONDiscoversAndParsesProjectConfig(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "cmd", "tool")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatal(err)
	}
	config := `[project]
name = "demo"
version = "0.1.0"

[tool.fmt]
indent_width = 4
line_width = 100

[tool.lint]
format = "sarif"

[tool.test]
format = "json"
`
	if err := os.WriteFile(filepath.Join(root, "gscript.toml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", nested}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runConfigCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if !report.Found || report.Root != root || report.Path != filepath.Join(root, "gscript.toml") {
		t.Fatalf("report location = %+v, want discovered root %s", report, root)
	}
	if report.Config == nil {
		t.Fatal("config is nil, want parsed config")
	}
	if report.Config.Project.Name != "demo" || report.Config.Project.Version != "0.1.0" {
		t.Fatalf("project = %+v, want demo 0.1.0", report.Config.Project)
	}
	if report.Config.Tool.Format.IndentWidth != 4 || report.Config.Tool.Format.LineWidth != 100 {
		t.Fatalf("fmt config = %+v, want 4/100", report.Config.Tool.Format)
	}
	if report.Config.Tool.Lint.Format != "sarif" || report.Config.Tool.Test.Format != "json" {
		t.Fatalf("tool config = %+v, want sarif/json", report.Config.Tool)
	}
}

func TestConfigCommandReportsMissingConfig(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty for JSON mode", stderr.String())
	}
	var report cliConfigReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON config report: %v; stdout = %q", err, stdout.String())
	}
	if report.Found || len(report.Diagnostics) != 1 || report.Diagnostics[0].Code != "GS9001" {
		t.Fatalf("report = %+v, want not found diagnostic", report)
	}
}

func TestConfigCommandReportsParseErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.test]\nformat = \"xml\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runConfigCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runConfigCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "GS9002") || !strings.Contains(stderr.String(), "tool.test.format") {
		t.Fatalf("stderr = %q, want config parse diagnostic", stderr.String())
	}
}

func TestCICommandListsProfiles(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"smoke", "--list"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCICommand code = %d, stderr = %q", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"go test", "tests/manifest.py", "github.com/never-labs/gscript", "gscript check", "tests/smoke/01_basic.gs", "worktree_audit.sh"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout = %q, want %q", out, want)
		}
	}
}

func TestCICommandRunsProfileCommands(t *testing.T) {
	oldCIExecCommand := ciExecCommand
	t.Cleanup(func() { ciExecCommand = oldCIExecCommand })
	var commands []string
	ciExecCommand = func(name string, args ...string) *exec.Cmd {
		commands = append(commands, shellJoin(append([]string{name}, args...)))
		helper, helperArgs := testHelperCommand(t, "ci")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"perf", "--no-luajit"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCICommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(commands) != 1 || !strings.Contains(commands[0], "performance_gate.sh") || !strings.Contains(commands[0], "--no-luajit") {
		t.Fatalf("commands = %#v, want performance gate with --no-luajit", commands)
	}
	if !strings.Contains(stdout.String(), "ci helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestCICommandRejectsUnknownProfile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runCICommand([]string{"nightly"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runCICommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown ci profile") {
		t.Fatalf("stderr = %q, want unknown profile", stderr.String())
	}
}

func TestCheckCommandJSONRunsEnabledSteps(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		helper, helperArgs := testHelperCommand(t, "docs")
		return exec.Command(helper, helperArgs...)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".gs")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runCheckCommand([]string{"--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runCheckCommand code = %d, stderr = %q", code, stderr.String())
	}
	var report checkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON check report: %v; stdout = %q", err, stdout.String())
	}
	if !report.OK || len(report.Steps) != 5 {
		t.Fatalf("report = %+v, want five passing steps", report)
	}
	for _, step := range report.Steps {
		if !step.OK || step.Skipped || step.ExitCode != 0 {
			t.Fatalf("step = %+v, want passing non-skipped step", step)
		}
	}
}

func TestCheckCommandReportsFailureAndSkips(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(path, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runCheckCommand([]string{"--json", "--no-test", "--no-manifest", "--no-docs", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runCheckCommand code = %d, want 1", code)
	}
	var report checkReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("stdout is not JSON check report: %v; stdout = %q", err, stdout.String())
	}
	if report.OK || len(report.Steps) != 5 {
		t.Fatalf("report = %+v, want failed report with five steps", report)
	}
	if !report.Steps[2].Skipped || !report.Steps[2].OK {
		t.Fatalf("test step = %+v, want skipped ok", report.Steps[2])
	}
	if !report.Steps[3].Skipped || !report.Steps[3].OK {
		t.Fatalf("manifest step = %+v, want skipped ok", report.Steps[3])
	}
	if !report.Steps[4].Skipped || !report.Steps[4].OK {
		t.Fatalf("docs step = %+v, want skipped ok", report.Steps[4])
	}
}

func TestTestCommandManifestCheckDispatchesTestsManifest(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	var gotName string
	var gotArgs []string
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "manifest")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--manifest-check"}, cliRunOptions{}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "python3" {
		t.Fatalf("python command = %q, want python3", gotName)
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("tests", "manifest.py")) || gotArgs[1] != "check" || gotArgs[2] != "tests" {
		t.Fatalf("args = %#v, want tests/manifest.py check tests", gotArgs)
	}
	if !strings.Contains(stdout.String(), "manifest helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestBenchCommandManifestCheckDispatchesBenchmarksManifest(t *testing.T) {
	oldCheckExecCommand := checkExecCommand
	t.Cleanup(func() { checkExecCommand = oldCheckExecCommand })
	var gotArgs []string
	checkExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "manifest")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"--manifest-check"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("tests", "manifest.py")) || gotArgs[1] != "check" || gotArgs[2] != "benchmarks" {
		t.Fatalf("args = %#v, want tests/manifest.py check benchmarks", gotArgs)
	}
	if !strings.Contains(stdout.String(), "manifest helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestBenchCommandDispatchesCompareHarness(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotName string
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"compare", "--bench", "numeric/mandelbrot"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "python3" {
		t.Fatalf("python command = %q, want python3", gotName)
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "numeric/mandelbrot" {
		t.Fatalf("args = %#v, want timing_compare.py --bench numeric/mandelbrot", gotArgs)
	}
	if !strings.Contains(stdout.String(), "bench helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestBenchCommandDefaultsToQuickCompare(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand(nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 9 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "control/sieve" || gotArgs[3] != "--runs" || gotArgs[4] != "1" || gotArgs[5] != "--warmup" || gotArgs[6] != "0" || gotArgs[7] != "--timeout" || gotArgs[8] != "60" {
		t.Fatalf("args = %#v, want timing_compare.py quick control/sieve profile", gotArgs)
	}
}

func TestBenchCommandDispatchesBenchmarkSelector(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"table/table_field_access", "--runs", "2"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 5 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "timing_compare.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "table/table_field_access" || gotArgs[3] != "--runs" || gotArgs[4] != "2" {
		t.Fatalf("args = %#v, want timing_compare.py --bench table/table_field_access --runs 2", gotArgs)
	}
}

func TestBenchCommandDispatchesProfiles(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var calls [][]string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		calls = append(calls, append([]string{name}, args...))
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	if code := runBenchCommand([]string{"--quick"}, &stdout, &stderr); code != 0 {
		t.Fatalf("quick code = %d, stderr = %q", code, stderr.String())
	}
	if code := runBenchCommand([]string{"--full"}, &stdout, &stderr); code != 0 {
		t.Fatalf("full code = %d, stderr = %q", code, stderr.String())
	}
	if code := runBenchCommand([]string{"--guard"}, &stdout, &stderr); code != 0 {
		t.Fatalf("guard code = %d, stderr = %q", code, stderr.String())
	}
	if len(calls) != 3 {
		t.Fatalf("calls = %#v, want three profile dispatches", calls)
	}
	if !strings.HasSuffix(calls[0][1], filepath.Join("benchmarks", "timing_compare.py")) || !containsString(calls[0], "table/table_array_access") {
		t.Fatalf("quick call = %#v, want timing quick profile", calls[0])
	}
	if !strings.HasSuffix(calls[1][1], filepath.Join("benchmarks", "timing_compare.py")) || !containsString(calls[1], "--all-groups") {
		t.Fatalf("full call = %#v, want timing full profile", calls[1])
	}
	if !strings.HasSuffix(calls[2][1], filepath.Join("benchmarks", "strict_guard.py")) || !containsString(calls[2], "control/sieve") {
		t.Fatalf("guard call = %#v, want strict guard profile", calls[2])
	}
}

func TestBenchCommandDispatchesStrictHarness(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"strict", "--group", "table"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 3 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "strict_guard.py")) || gotArgs[1] != "--group" || gotArgs[2] != "table" {
		t.Fatalf("args = %#v, want strict_guard.py --group table", gotArgs)
	}
}

func TestBenchCommandDispatchesDiagnoseHarness(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runBenchCommand([]string{"diagnose", "--bench", "control/sieve", "--no-timing"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runBenchCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 4 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "diagnose.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "control/sieve" || gotArgs[3] != "--no-timing" {
		t.Fatalf("args = %#v, want diagnose.py --bench control/sieve --no-timing", gotArgs)
	}
}

func TestDiagCommandDispatchesDumpScript(t *testing.T) {
	oldDiagExecCommand := diagExecCommand
	t.Cleanup(func() { diagExecCommand = oldDiagExecCommand })
	var gotName string
	var gotArgs []string
	diagExecCommand = func(name string, args ...string) *exec.Cmd {
		gotName = name
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "diag")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDiagCommand([]string{"dump", "control/sieve"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDiagCommand code = %d, stderr = %q", code, stderr.String())
	}
	if gotName != "bash" {
		t.Fatalf("command = %q, want bash", gotName)
	}
	if len(gotArgs) != 2 || !strings.HasSuffix(gotArgs[0], filepath.Join("scripts", "diag.sh")) || gotArgs[1] != "control/sieve" {
		t.Fatalf("args = %#v, want scripts/diag.sh control/sieve", gotArgs)
	}
	if !strings.Contains(stdout.String(), "diag helper ok") {
		t.Fatalf("stdout = %q, want helper output", stdout.String())
	}
}

func TestDiagCommandDispatchesBundleScript(t *testing.T) {
	oldDiagExecCommand := diagExecCommand
	t.Cleanup(func() { diagExecCommand = oldDiagExecCommand })
	var gotArgs []string
	diagExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "diag")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDiagCommand([]string{"bundle", "--skip-benchmarks"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDiagCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 2 || !strings.HasSuffix(gotArgs[0], filepath.Join("scripts", "diagnostics_bundle.sh")) || gotArgs[1] != "--skip-benchmarks" {
		t.Fatalf("args = %#v, want diagnostics_bundle.sh --skip-benchmarks", gotArgs)
	}
}

func TestDiagCommandRejectsUnknownMode(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runDiagCommand([]string{"unknown"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runDiagCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unknown diag mode") {
		t.Fatalf("stderr = %q, want unknown mode", stderr.String())
	}
}

func TestDiagnoseCommandDispatchesBenchmarkSelector(t *testing.T) {
	oldBenchExecCommand := benchExecCommand
	t.Cleanup(func() { benchExecCommand = oldBenchExecCommand })
	var gotArgs []string
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		gotArgs = append([]string(nil), args...)
		helper, helperArgs := testHelperCommand(t, "bench")
		return exec.Command(helper, helperArgs...)
	}

	var stdout, stderr bytes.Buffer
	code := runDiagnoseCommand([]string{"table/table_field_access", "--no-timing"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runDiagnoseCommand code = %d, stderr = %q", code, stderr.String())
	}
	if len(gotArgs) != 4 || !strings.HasSuffix(gotArgs[0], filepath.Join("benchmarks", "diagnose.py")) || gotArgs[1] != "--bench" || gotArgs[2] != "table/table_field_access" || gotArgs[3] != "--no-timing" {
		t.Fatalf("args = %#v, want diagnose.py --bench table/table_field_access --no-timing", gotArgs)
	}
}

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
//gscript:feature ai-native
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
		"4:1 feature ai-native",
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
	if !bytes.Contains(stdlibDoc, []byte("`json`")) {
		t.Fatalf("stdlib.md = %q, want stdlib inventory", string(stdlibDoc))
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

func TestModInitGraphAndVerify(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gs"), []byte(`helper := require("pkg.helper")
jsonMod := require("json")
`), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"init", "--name", "demo", "--dir", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod init code = %d, stderr = %q", code, stderr.String())
	}
	manifestPath := filepath.Join(dir, "gscript.mod.json")
	if !strings.Contains(stdout.String(), manifestPath) {
		t.Fatalf("stdout = %q, want manifest path", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"graph", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod graph code = %d, stderr = %q", code, stderr.String())
	}
	var graph modGraphReport
	if err := json.Unmarshal(stdout.Bytes(), &graph); err != nil {
		t.Fatalf("stdout is not JSON graph: %v; stdout = %q", err, stdout.String())
	}
	if len(graph.Files) != 1 || !containsString(graph.Files[0].Requires, "pkg.helper") || !containsString(graph.Files[0].Requires, "json") {
		t.Fatalf("graph = %+v, want static requires", graph)
	}

	stdout.Reset()
	stderr.Reset()
	code = runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("mod verify code = %d, stderr = %q", code, stderr.String())
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if !verify.OK || verify.Manifest != manifestPath {
		t.Fatalf("verify = %+v, want ok manifest", verify)
	}
}

func TestModVerifyReportsMissingManifest(t *testing.T) {
	dir := t.TempDir()
	var stdout, stderr bytes.Buffer
	code := runModCommand([]string{"verify", "--json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("mod verify code = %d, want 1", code)
	}
	var verify modVerifyReport
	if err := json.Unmarshal(stdout.Bytes(), &verify); err != nil {
		t.Fatalf("stdout is not JSON verify report: %v; stdout = %q", err, stdout.String())
	}
	if verify.OK || len(verify.Diagnostics) == 0 || verify.Diagnostics[0].Code != "GS9103" {
		t.Fatalf("verify = %+v, want missing manifest diagnostic", verify)
	}
}

func TestRunCommandExecutesFileWithArgs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "args.gs")
	src := `assert(arg[0] == "` + path + `")
assert(arg[1] == "one")
assert(arg[2] == "two")
`
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runRunCommand([]string{"--vm", path, "one", "two"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runRunCommand code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunCommandReportsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runRunCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runRunCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: gscript run") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestEvalCommandExecutesSourceWithArgs(t *testing.T) {
	src := `assert(arg[0] == "<eval>")
assert(arg[1] == "one")
assert(arg[2] == "two")
`
	var stdout, stderr bytes.Buffer
	code := runEvalCommand([]string{"--vm", src, "one", "two"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runEvalCommand code = %d, stderr = %q", code, stderr.String())
	}
}

func TestRunPublicPathPredicateKeepsDiagnosticsInternal(t *testing.T) {
	if !canUsePublicRunPath(cliRunOptions{UseJIT: true}) {
		t.Fatal("plain JIT run should use public path")
	}
	diagnosticCases := []cliRunOptions{
		{ShowJITStats: true},
		{JIT: jitCLIOptions{TimelinePath: "timeline.jsonl"}},
		{JIT: jitCLIOptions{WarmDumpDir: "warm"}},
		{JIT: jitCLIOptions{ShowExitStats: true}},
		{JIT: jitCLIOptions{ShowExitStatsJSON: true}},
		{JIT: jitCLIOptions{ShowTier2PerfStats: true}},
		{JIT: jitCLIOptions{ShowTier2PerfStatsJSON: true}},
		{JIT: jitCLIOptions{ShowTier2SpecStateJSON: true}},
		{JIT: jitCLIOptions{ShowTier2SpecWorklistJSON: true}},
		{JIT: jitCLIOptions{ShowCoroutineStats: true}},
		{JIT: jitCLIOptions{ShowPathStats: true}},
		{JIT: jitCLIOptions{ShowPathStatsJSON: true}},
	}
	for _, opts := range diagnosticCases {
		if canUsePublicRunPath(opts) {
			t.Fatalf("diagnostic options should require internal path: %+v", opts)
		}
	}
}

func TestEvalCommandReportsUsage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runEvalCommand(nil, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runEvalCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: gscript eval") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestREPLCommandRejectsArgs(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runREPLCommand([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runREPLCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "usage: gscript repl") {
		t.Fatalf("stderr = %q, want usage", stderr.String())
	}
}

func TestTestFilesDirectoryCollectsGSFilesSorted(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "b.gs"),
		filepath.Join(dir, "a.gs"),
		filepath.Join(dir, "nested", "c.gs"),
	}
	for _, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "ignore.txt"), []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := testFiles(dir)
	if err != nil {
		t.Fatalf("testFiles err = %v", err)
	}
	want := []string{paths[1], paths[0], paths[2]}
	if len(files) != len(want) {
		t.Fatalf("testFiles len = %d, want %d: %#v", len(files), len(want), files)
	}
	for i := range want {
		if files[i] != want[i] {
			t.Fatalf("testFiles = %#v, want %#v", files, want)
		}
	}
}

func TestRunTestCommandDefaultsToCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
	if err := os.WriteFile(filepath.Join(dir, "ok.gs"), []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json"}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.Total != 1 || result.Passed != 1 {
		t.Fatalf("result = %+v, want one passing default-directory test", result)
	}
}

func TestRunTestsReportsFailingFile(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if runTests(dir, cliRunOptions{UseVM: false}, &stderr) {
		t.Fatal("runTests succeeded, want failure")
	}
	out := stderr.String()
	if !strings.Contains(out, badPath) {
		t.Fatalf("stderr = %q, want failing filename %q", out, badPath)
	}
	if !strings.Contains(out, "parse error") {
		t.Fatalf("stderr = %q, want parse error", out)
	}
}

func TestRunTestsComparesGoldenStdout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("print(\"hello\", \"world\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ok.out"), []byte("hello\tworld\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if !runTests(path, cliRunOptions{UseVM: false}, &stderr) {
		t.Fatalf("runTests failed, stderr = %q", stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTestsReportsGoldenStdoutMismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	golden := filepath.Join(dir, "bad.out")
	if err := os.WriteFile(path, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(golden, []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	if runTests(path, cliRunOptions{UseVM: false}, &stderr) {
		t.Fatal("runTests succeeded, want failure")
	}
	out := stderr.String()
	for _, want := range []string{path, golden, "stdout mismatch", "expected:\nexpected\n", "got:\nactual\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr = %q, want %q", out, want)
		}
	}
}

func TestRunTestCommandJSONReportsResults(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(okPath, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(okPath, ".gs")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.Total != 1 || result.Passed != 1 || result.Failed != 0 || result.GoldenMode != "auto" {
		t.Fatalf("result = %+v, want one passing test", result)
	}
	if len(result.Files) != 1 || result.Files[0].File != okPath || !result.Files[0].OK {
		t.Fatalf("files = %+v, want passing %s", result.Files, okPath)
	}
}

func TestRunTestCommandGoldenRequireReportsMissingGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.gs")
	golden := strings.TrimSuffix(path, ".gs") + ".out"
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--golden=require", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runTestCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if result.OK || result.GoldenMode != "require" || len(result.Files) != 1 || result.Files[0].Golden != golden || !strings.Contains(result.Files[0].Error, "missing golden") {
		t.Fatalf("result = %+v, want missing golden failure for %s", result, golden)
	}
}

func TestRunTestCommandGoldenIgnoreSkipsComparison(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(path, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".gs")+".out", []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--golden=ignore", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.GoldenMode != "ignore" || len(result.Files) != 1 || result.Files[0].Expected != "" || result.Files[0].Actual != "" {
		t.Fatalf("result = %+v, want ignored golden mismatch", result)
	}
}

func TestRunTestCommandGoldenUpdateWritesGolden(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	golden := strings.TrimSuffix(path, ".gs") + ".out"
	if err := os.WriteFile(path, []byte("print(\"new\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(golden, []byte("old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--golden=update", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	got, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new\n" {
		t.Fatalf("golden = %q, want updated stdout", got)
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.GoldenMode != "update" || len(result.Files) != 1 || result.Files[0].Golden != golden {
		t.Fatalf("result = %+v, want update golden result", result)
	}
}

func TestRunTestCommandListsFiles(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.gs")
	b := filepath.Join(dir, "b.gs")
	if err := os.WriteFile(a, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("x := 2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--list", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); !strings.Contains(got, a) || !strings.Contains(got, b) {
		t.Fatalf("stdout = %q, want listed files %q and %q", got, a, b)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunTestCommandSeedIsVisibleToScripts(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "seed.gs")
	if err := os.WriteFile(path, []byte(`print(os.getenv("GSCRIPT_TEST_SEED"))
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".gs")+".out", []byte("odin\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", "--seed", "odin", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if result.Seed != "odin" || !result.OK {
		t.Fatalf("result = %+v, want seed and passing test", result)
	}
	if got := os.Getenv("GSCRIPT_TEST_SEED"); got == "odin" {
		t.Fatal("GSCRIPT_TEST_SEED leaked after test run")
	}
}

func TestRunTestCommandUsesConfiguredFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.test]\nformat = \"json\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("print(\"ok\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".gs")+".out", []byte("ok\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runTestCommand code = %d, stderr = %q", code, stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not configured JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if !result.OK || result.Total != 1 {
		t.Fatalf("result = %+v, want one passing configured JSON test", result)
	}
}

func TestRunTestCommandJSONReportsFailures(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("print(\"actual\")\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(badPath, ".gs")+".out", []byte("expected\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=json", dir}, cliRunOptions{UseVM: false}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runTestCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var result testRunResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("stdout is not JSON test result: %v; stdout = %q", err, stdout.String())
	}
	if result.OK || result.Total != 1 || result.Passed != 0 || result.Failed != 1 {
		t.Fatalf("result = %+v, want one failing test", result)
	}
	if len(result.Files) != 1 || result.Files[0].Error != "" || result.Files[0].Expected != "expected\n" || result.Files[0].Actual != "actual\n" {
		t.Fatalf("file result = %+v, want stdout mismatch payload", result.Files)
	}
}

func TestLintCommandUsesConfiguredFormat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gscript.toml"), []byte("[tool.lint]\nformat = \"json\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(path, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want diagnostics failure", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty in configured JSON mode", stderr.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not configured JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 || diagnostics[0].File != path || diagnostics[0].Code != "GS1001" {
		t.Fatalf("diagnostics = %+v, want configured JSON parse diagnostic for %s", diagnostics, path)
	}
}

func TestRunTestCommandRejectsUnsupportedFormat(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runTestCommand([]string{"--format=xml", "x.gs"}, cliRunOptions{}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runTestCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported --format") {
		t.Fatalf("stderr = %q, want unsupported format", stderr.String())
	}
}

func TestFmtCheckReportsUnformattedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "needs_fmt.gs")
	original := []byte("x := 1 \t\r\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), path+": not formatted") {
		t.Fatalf("stderr = %q, want not formatted diagnostic", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed during --check: %q", string(got))
	}
}

func TestFmtWritesWhitespaceNormalization(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "needs_fmt.gs")
	if err := os.WriteFile(path, []byte("x := 1  \n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), path) {
		t.Fatalf("stdout = %q, want formatted filename", stdout.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "x := 1\n" {
		t.Fatalf("formatted file = %q, want %q", string(got), "x := 1\n")
	}
}

func TestFmtStdinWritesFormattedSourceToStdout(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1  \r\n\r\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got, want := stdout.String(), "x := 1\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinCheckReportsUnformattedSource(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1 \t\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", "--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "scratch.gs: not formatted") {
		t.Fatalf("stderr = %q, want not formatted diagnostic", stderr.String())
	}
}

func TestFmtStdinCheckAcceptsFormattedSource(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--check", "--stdin-file-name", "scratch.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestCLIAINativeAnthropicCompatibleRequestsKeepPrompts(t *testing.T) {
	type anthropicRequest struct {
		Model    string `json:"model"`
		System   string `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content any    `json:"content"`
		} `json:"messages"`
	}

	var (
		mu       sync.Mutex
		requests []anthropicRequest
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var req anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		mu.Lock()
		requests = append(requests, req)
		requestCount := len(requests)
		mu.Unlock()
		text := "MEMORY_STORED"
		switch requestCount {
		case 2:
			text = "project=ORCHID;owner=ADA"
		case 3:
			text = `{"project":"ORCHID","owner":"ADA","remembered":true,"meta":{"source":"history"}}`
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"content":[{"type":"text","text":%q}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`, text)
	}))
	defer server.Close()

	source := `
models {
    default: "glm-smoke"
    "glm-smoke": {
        protocol: "anthropic_compatible"
        base_url: os.getenv("GSCRIPT_GLM_BASE_URL")
        api_key: os.getenv("GSCRIPT_GLM_API_KEY")
        provider_model: os.getenv("GSCRIPT_GLM_MODEL")
    }
}

history := messages {
    system: "You are a deterministic memory smoke-test assistant."
    user: "Store this memory: project codename is ORCHID and owner is ADA. Reply exactly: MEMORY_STORED"
}

stored, err := turn {
    messages: history
    max_tokens: 32
    temperature: 0
}
if err != nil {
    return
}

history[#history + 1] = msg.assistant(stored.text)
history[#history + 1] = msg.user("Using only the stored memory, reply exactly: project=ORCHID;owner=ADA")

recalled, err := turn {
    messages: history
    max_tokens: 48
    temperature: 0
}
if err != nil {
    return
}

extractor := agent(summary) {
    model: "glm-smoke"
    system: "Return only compact JSON."
    user: "Convert this memory recall into JSON. Recall: " .. summary
    output: {
        project: "ORCHID"
        owner: "ADA"
        remembered: true
        meta: {source: "history"}
    }
    max_tokens: 96
    temperature: 0
}

extracted, err := extractor(recalled.text)
project := extracted.value.project
`

	cases := []struct {
		name string
		run  func(*runtime.Interpreter, string) error
	}{
		{name: "interpreter", run: func(interp *runtime.Interpreter, src string) error {
			return runString(interp, src)
		}},
		{name: "bytecode", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, false, false, jitCLIOptions{})
		}},
	}
	if goruntime.GOOS == "darwin" && goruntime.GOARCH == "arm64" {
		cases = append(cases, struct {
			name string
			run  func(*runtime.Interpreter, string) error
		}{name: "jit", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, true, false, jitCLIOptions{})
		}})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			requests = nil
			mu.Unlock()
			t.Setenv("GSCRIPT_GLM_BASE_URL", server.URL)
			t.Setenv("GSCRIPT_GLM_API_KEY", "test-key")
			t.Setenv("GSCRIPT_GLM_MODEL", "mock-glm")
			interp := runtime.New()
			installCLILLMProviderFactory(interp)
			if err := tc.run(interp, source); err != nil {
				t.Fatalf("run: %v", err)
			}
			mu.Lock()
			gotRequests := append([]anthropicRequest(nil), requests...)
			mu.Unlock()
			if len(gotRequests) != 3 {
				t.Fatalf("requests = %d, want 3: %#v", len(gotRequests), gotRequests)
			}
			for i, req := range gotRequests {
				if req.Model != "mock-glm" {
					t.Fatalf("request %d model = %q, want mock-glm", i+1, req.Model)
				}
				if strings.TrimSpace(req.System) == "" {
					t.Fatalf("request %d system prompt is empty: %#v", i+1, req)
				}
				if len(req.Messages) == 0 {
					t.Fatalf("request %d messages empty: %#v", i+1, req)
				}
				if req.Messages[0].Role != "user" || strings.TrimSpace(fmt.Sprint(req.Messages[0].Content)) == "" {
					t.Fatalf("request %d first user message empty: %#v", i+1, req.Messages)
				}
			}
		})
	}
}

func TestFmtStdinAINativeIndentation(t *testing.T) {
	src := `tool lookup(query) {
return "found:" .. query, nil
}
models {
default: "fast"
fast: {provider_model: "mock-fast"}
}
agent defaults {
model: "fast"
tools: [lookup]
budget: {turns: 2, calls: 4, tokens: 1000, time: 30s}
}
agent researcher(topic) {
system: "Use the tool."
user: topic
tools: [lookup]
} flow {
history := messages {
system: system
user: topic
}
result, err := turn {
messages: history
tools: tools
model: model
}
return result, err
}
answer := agent(q) {
user: q
}
budget { turns: 1 } {
direct, direct_err := turn {
messages: messages { user: "one-shot" }
}
_ = direct
_ = direct_err
}
`
	want := `tool lookup(query) {
    return "found:" .. query, nil
}
models {
    default: "fast"
    fast: {provider_model: "mock-fast"}
}
agent defaults {
    model: "fast"
    tools: [lookup]
    budget: {turns: 2, calls: 4, tokens: 1000, time: 30s}
}
agent researcher(topic) {
    system: "Use the tool."
    user: topic
    tools: [lookup]
} flow {
    history := messages {
        system: system
        user: topic
    }
    result, err := turn {
        messages: history
        tools: tools
        model: model
    }
    return result, err
}
answer := agent(q) {
    user: q
}
budget { turns: 1 } {
    direct, direct_err := turn {
        messages: messages { user: "one-shot" }
    }
    _ = direct
    _ = direct_err
}
`

	oldStdin := cliStdin
	cliStdin = strings.NewReader(src)
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "ai_native.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtStdinPreservesCommentOnlyLines(t *testing.T) {
	src := `agent sample() {
// keep this note

user: "hello"
} flow {
if true {
// nested
print("ok")
}
}
`
	want := `agent sample() {
    // keep this note

    user: "hello"
} flow {
    if true {
        // nested
        print("ok")
    }
}
`

	oldStdin := cliStdin
	cliStdin = strings.NewReader(src)
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "comments.gs"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runFmtCommand code = %d, stderr = %q", code, stderr.String())
	}
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestFmtPreservesIntraLineFormattingBoundary(t *testing.T) {
	src := `// lookup searches project docs.
//gscript:requires docs.read
tool lookup(query) {
return "found:"..query,nil
}
models {
short: "x"
longer_key : {provider_model:"mock-fast"}
}
cfg := {short:1, longer_key : 2}
total:=1+  2
`
	want := `// lookup searches project docs.
//gscript:requires docs.read
tool lookup(query) {
    return "found:"..query,nil
}
models {
    short: "x"
    longer_key : {provider_model:"mock-fast"}
}
cfg := {short:1, longer_key : 2}
total:=1+  2
`

	formatted, err := formatSource("boundary.gs", []byte(src))
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	if got := string(formatted); got != want {
		t.Fatalf("formatted = %q, want %q", got, want)
	}
}

func aiNativeToolchainCoverageSource() []byte {
	return []byte(`// lookup searches project docs.
//gscript:requires docs.read
//gscript:param query search query
tool lookup(query) {
    return "found:" .. query, nil
}

models {
    default: "fast"
    fast: {provider_model: "mock-fast"}
}

agent extractor(topic) {
    model: "fast"
    system: "Return JSON."
    user: topic
    output: {summary: "example"}
}

delegate := toolof(extractor, {
    name: "delegate"
    description: "Delegate extraction."
})

agent supervisor(topic) {
    model: "fast"
    tools: [extractor, delegate, lookup]
    user: topic
} flow {
    call := {id: "call_1", tool: "lookup", args: {query: topic}}
    msgs := messages {
        system: system
        user: topic
        msg.assistant_call(call)
        msg.tool_result("call_1", {summary: "docs"})
    }
    tool_msg, tool_idx := history.find(msgs, {role: "tool"})
    assistant_msg, assistant_idx := history.last(msgs, {role: "assistant"})
    all_users := history.find_all(msgs, {role: "user"})
    history.append(msgs, msg.user("Summarize."))
    ok, ok_msg := llm.validate_output({summary: "docs"}, {summary: "example"})
    _ = tool_msg
    _ = tool_idx
    _ = assistant_msg
    _ = assistant_idx
    _ = all_users
    _ = ok
    _ = ok_msg
    return turn {
        messages: msgs
        tools: tools
        model: model
    }
}

answer, answer_err := supervisor("gscript")
_ = answer
_ = answer_err
`)
}

func TestFmtAINativeSyntaxCoverage(t *testing.T) {
	formatted, err := formatSource("ai_native.gs", aiNativeToolchainCoverageSource())
	if err != nil {
		t.Fatalf("formatSource: %v", err)
	}
	for _, want := range []string{
		"tools: [extractor, delegate, lookup]",
		"msg.assistant_call(call)",
		"msg.tool_result(\"call_1\", {summary: \"docs\"})",
		"history.find(msgs, {role: \"tool\"})",
		"history.find_all(msgs, {role: \"user\"})",
		"llm.validate_output({summary: \"docs\"}, {summary: \"example\"})",
	} {
		if !strings.Contains(string(formatted), want) {
			t.Fatalf("formatted AI-native source missing %q:\n%s", want, formatted)
		}
	}
	if strings.Contains(string(formatted), "}  \n") {
		t.Fatalf("formatted source still contains trailing spaces: %q", string(formatted))
	}
	if !strings.HasSuffix(string(formatted), "\n") {
		t.Fatalf("formatted source does not end with newline: %q", string(formatted))
	}
	formattedAgain, err := formatSource("ai_native.gs", formatted)
	if err != nil {
		t.Fatalf("format formatted source: %v", err)
	}
	if !bytes.Equal(formattedAgain, formatted) {
		t.Fatalf("AI-native formatting is not idempotent:\nonce:\n%s\ntwice:\n%s", formatted, formattedAgain)
	}
}

func TestLintAINativeSyntaxCoverage(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ai_native.gs")
	src := aiNativeToolchainCoverageSource()
	if err := os.WriteFile(path, src, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runLintCommand code = %d, stderr = %q, stdout = %q", code, stderr.String(), stdout.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want empty", diagnostics)
	}
}

func TestSoADirectAccessRunsInInterpreterAndVM(t *testing.T) {
	source := `
points := soa.zip({
    x: []f64{1, 2, 3},
    y: []f64{10, 20, 30},
    id: []i64{101, 102, 103},
})
xcol := points.x
row := points[2]
row.x = 42
points[2] = row
points.y = []f64{100, 200, 300}
points.z = []i64{7, 8, 9}
assert(xcol[2] == 42)
assert(points.x[2] == 42)
assert(points["x"][3] == 3)
assert(points[2].x == 42)
assert(points.y[3] == 300)
assert(points.z[1] == 7)
assert(points.missing == nil)
`
	for _, tc := range []struct {
		name string
		run  func(*runtime.Interpreter, string) error
	}{
		{name: "interpreter", run: runString},
		{name: "bytecode", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, false, false, jitCLIOptions{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interp := runtime.New()
			if err := tc.run(interp, source); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}

func TestFmtStdinRejectsPathArguments(t *testing.T) {
	oldStdin := cliStdin
	cliStdin = strings.NewReader("x := 1\n")
	defer func() { cliStdin = oldStdin }()

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{"--stdin-file-name", "scratch.gs", "file.gs"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runFmtCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "--stdin-file-name cannot be used with path arguments") {
		t.Fatalf("stderr = %q, want stdin/path diagnostic", stderr.String())
	}
}

func TestFmtRefusesSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.gs")
	original := []byte("func {\n")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runFmtCommand([]string{path}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runFmtCommand code = %d, want 1", code)
	}
	if !strings.Contains(stderr.String(), "parse error") {
		t.Fatalf("stderr = %q, want parse error", stderr.String())
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("file changed after parse failure: %q", string(got))
	}
}

func TestLintReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	out := stderr.String()
	for _, want := range []string{badPath, "GS1001", "parse error"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stderr = %q, want %q", out, want)
		}
	}
	if strings.Contains(out, okPath) {
		t.Fatalf("stderr = %q, did not want clean file", out)
	}
}

func TestLintJSONReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.gs")
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(okPath, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.File != badPath {
		t.Fatalf("diagnostic file = %q, want %q", got.File, badPath)
	}
	if got.Code != "GS1001" {
		t.Fatalf("diagnostic code = %q, want GS1001", got.Code)
	}
	if got.Severity != "error" {
		t.Fatalf("diagnostic severity = %q, want error", got.Severity)
	}
	if !strings.Contains(got.Message, "parse error") {
		t.Fatalf("diagnostic message = %q, want parse error", got.Message)
	}
	if got.Line != 1 {
		t.Fatalf("diagnostic line = %d, want 1", got.Line)
	}
	if got.Column != 6 {
		t.Fatalf("diagnostic column = %d, want 6", got.Column)
	}
}

func TestLintJSONReportsLexerErrorPosition(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("\"unterminated\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=json", badPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics len = %d, want 1: %#v", len(diagnostics), diagnostics)
	}
	got := diagnostics[0]
	if got.File != badPath {
		t.Fatalf("diagnostic file = %q, want %q", got.File, badPath)
	}
	if got.Code != "GS1001" {
		t.Fatalf("diagnostic code = %q, want GS1001", got.Code)
	}
	if !strings.Contains(got.Message, "lexer error") {
		t.Fatalf("diagnostic message = %q, want lexer error", got.Message)
	}
	if got.Line != 1 {
		t.Fatalf("diagnostic line = %d, want 1", got.Line)
	}
	if got.Column != 1 {
		t.Fatalf("diagnostic column = %d, want 1", got.Column)
	}
}

func TestLintJSONReportsEmptyDiagnosticsOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format", "json", path}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runLintCommand code = %d, want 0; stderr = %q", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var diagnostics []lintDiagnostic
	if err := json.Unmarshal(stdout.Bytes(), &diagnostics); err != nil {
		t.Fatalf("stdout is not JSON diagnostics: %v; stdout = %q", err, stdout.String())
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, want empty", diagnostics)
	}
}

func TestLintSARIFReportsSyntaxErrors(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.gs")
	if err := os.WriteFile(badPath, []byte("func {\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=sarif", badPath}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("runLintCommand code = %d, want 1", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}

	var log sarifLog
	if err := json.Unmarshal(stdout.Bytes(), &log); err != nil {
		t.Fatalf("stdout is not SARIF JSON: %v; stdout = %q", err, stdout.String())
	}
	if log.Version != "2.1.0" || len(log.Runs) != 1 {
		t.Fatalf("SARIF version/runs = %q/%d, want 2.1.0/1", log.Version, len(log.Runs))
	}
	if log.Runs[0].Tool.Driver.Name != "gscript lint" {
		t.Fatalf("tool name = %q, want gscript lint", log.Runs[0].Tool.Driver.Name)
	}
	if len(log.Runs[0].Results) != 1 {
		t.Fatalf("results len = %d, want 1", len(log.Runs[0].Results))
	}
	result := log.Runs[0].Results[0]
	if result.RuleID != "GS1001" || result.Level != "error" {
		t.Fatalf("result = %+v, want GS1001 error", result)
	}
	if !strings.Contains(result.Message.Text, "parse error") {
		t.Fatalf("message = %q, want parse error", result.Message.Text)
	}
	if len(result.Locations) != 1 || result.Locations[0].PhysicalLocation.ArtifactLocation.URI != filepath.ToSlash(badPath) {
		t.Fatalf("locations = %+v, want %s", result.Locations, filepath.ToSlash(badPath))
	}
}

func TestLintRejectsUnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.gs")
	if err := os.WriteFile(path, []byte("x := 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	code := runLintCommand([]string{"--format=xml", path}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("runLintCommand code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "unsupported --format") {
		t.Fatalf("stderr = %q, want unsupported format diagnostic", stderr.String())
	}
}
