package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"
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
		if !caps.LLM.Enabled || !containsString(caps.LLM.Syntax, want) {
			t.Fatalf("llm syntax = %#v, want %q", caps.LLM.Syntax, want)
		}
	}
	if !containsString(caps.LLM.ToolMetadata, "gscript:requires") || !containsString(caps.LLM.StaticValidation, "static_tool_capabilities") || !containsString(caps.LLM.Tooling, "lint-sarif") {
		t.Fatalf("llm capabilities = %+v, want metadata, static validation, and tooling entries", caps.LLM)
	}
	for _, want := range []string{"llm.toolof", "llm.agent_as_tool", "llm.validate_output", "msg.assistant_call", "msg.tool_result", "history.find", "history.find_all", "history.last", "history.append"} {
		if !containsString(caps.LLM.RuntimePrimitives, want) {
			t.Fatalf("llm runtime primitives = %#v, want %q", caps.LLM.RuntimePrimitives, want)
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
