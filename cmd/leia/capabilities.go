package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/never-labs/leia/internal/stdlib/bind"
	"github.com/never-labs/leia/internal/stdlib/catalog"
	stdinstall "github.com/never-labs/leia/internal/stdlib/install"
)

type cliCapabilities struct {
	SchemaVersion  int                    `json:"schema_version"`
	Status         string                 `json:"status"`
	Platform       cliPlatformCapability  `json:"platform"`
	Execution      cliExecutionCapability `json:"execution"`
	CommandCount   int                    `json:"command_count"`
	StdlibCount    int                    `json:"stdlib_module_count"`
	DialectCount   int                    `json:"dialect_count"`
	Commands       []string               `json:"commands"`
	StdlibModules  []string               `json:"stdlib_modules"`
	StdlibLayers   []cliStdlibLayer       `json:"stdlib_layers"`
	DefaultImports []cliDefaultImport     `json:"default_imports"`
	Dialects       []cliDialectCapability `json:"dialects"`
	LLM            cliLLMCapability       `json:"llm"`
	Tooling        cliToolingCapability   `json:"tooling"`
}

type cliPlatformCapability struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

type cliExecutionCapability struct {
	Interpreter bool `json:"interpreter"`
	BytecodeVM  bool `json:"bytecode_vm"`
	JIT         bool `json:"jit"`
	MethodJIT   bool `json:"method_jit"`
}

type cliStdlibLayer struct {
	Name    string            `json:"name"`
	Modules []cliStdlibModule `json:"modules"`
}

type cliStdlibModule struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities,omitempty"`
	SafeDefault  bool     `json:"safe_default,omitempty"`
}

type cliDefaultImport struct {
	Name   string `json:"name"`
	Module string `json:"module"`
	Member string `json:"member"`
}

type cliDialectCapability struct {
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Capabilities []string `json:"capabilities,omitempty"`
	Builtin      bool     `json:"builtin"`
	Eval         bool     `json:"eval"`
	Block        bool     `json:"block"`
	Aliases      []string `json:"aliases,omitempty"`
}

type cliLLMCapability struct {
	Enabled           bool     `json:"enabled"`
	Syntax            []string `json:"syntax"`
	ToolMetadata      []string `json:"tool_metadata"`
	StaticValidation  []string `json:"static_validation"`
	RuntimePrimitives []string `json:"runtime_primitives"`
	Tooling           []string `json:"tooling"`
}

type cliToolingCapability struct {
	Formatter   cliFormatterCapability `json:"formatter"`
	Linter      cliLinterCapability    `json:"linter"`
	Test        cliTestCapability      `json:"test"`
	Config      cliConfigCapability    `json:"config"`
	ReportCount int                    `json:"report_count"`
	Reports     []cliReportCapability  `json:"reports"`
}

type cliFormatterCapability struct {
	Stdin     bool     `json:"stdin"`
	Check     bool     `json:"check"`
	Write     bool     `json:"write"`
	Formats   []string `json:"formats"`
	Reports   []string `json:"reports"`
	Stability string   `json:"stability"`
}

type cliLinterCapability struct {
	Formats []string `json:"formats"`
	Codes   []string `json:"codes"`
}

type cliTestCapability struct {
	GoldenStdout bool     `json:"golden_stdout"`
	GoldenModes  []string `json:"golden_modes"`
	Directory    bool     `json:"directory"`
	List         bool     `json:"list"`
	Reports      []string `json:"reports"`
	SeedEnv      string   `json:"seed_env"`
}

type cliReportCapability struct {
	Command          string   `json:"command"`
	Formats          []string `json:"formats"`
	SchemaVersion    int      `json:"schema_version"`
	StatusField      string   `json:"status_field,omitempty"`
	CountFields      []string `json:"count_fields,omitempty"`
	CollectionFields []string `json:"collection_fields,omitempty"`
}

func runCapabilitiesCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print capabilities as JSON")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia capabilities [--json]")
		return 2
	}
	caps := buildCapabilities()
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(caps); err != nil {
			fmt.Fprintf(errw, "leia capabilities: write json: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(outw, "platform: %s/%s\n", caps.Platform.GOOS, caps.Platform.GOARCH)
	fmt.Fprintf(outw, "jit: %t\n", caps.Execution.JIT)
	fmt.Fprintf(outw, "llm: %t (%s)\n", caps.LLM.Enabled, strings.Join(caps.LLM.Syntax, ", "))
	fmt.Fprintf(outw, "commands: %s\n", strings.Join(caps.Commands, ", "))
	fmt.Fprintf(outw, "stdlib modules: %d\n", len(caps.StdlibModules))
	fmt.Fprintf(outw, "dialects: %d\n", len(caps.Dialects))
	return 0
}

func buildCapabilities() cliCapabilities {
	modules := catalog.ModuleNames()
	sort.Strings(modules)
	commands := cliCommandNames()
	dialects := buildDialectCapabilities()
	reports := buildReportCapabilities()
	return cliCapabilities{
		SchemaVersion: 1,
		Status:        "pass",
		Platform: cliPlatformCapability{
			GOOS:   goruntime.GOOS,
			GOARCH: goruntime.GOARCH,
		},
		Execution: cliExecutionCapability{
			Interpreter: true,
			BytecodeVM:  true,
			JIT:         cliJITAvailable(),
			MethodJIT:   cliMethodJITAvailable(),
		},
		CommandCount:   len(commands),
		StdlibCount:    len(modules),
		DialectCount:   len(dialects),
		Commands:       commands,
		StdlibModules:  modules,
		StdlibLayers:   buildStdlibLayerCapabilities(),
		DefaultImports: buildDefaultImportCapabilities(),
		Dialects:       dialects,
		LLM: cliLLMCapability{
			Enabled: true,
			Syntax: []string{
				"tagged_strings",
				"tagged_blocks",
				"shell_strings",
				"llm_stdlib_calls",
				"dialect_eval",
			},
			ToolMetadata: []string{
				"doc_comment",
				"leia:requires",
				"leia:param",
			},
			StaticValidation: []string{
				"duplicate_agent_defaults",
				"tool_requires",
				"tool_param_docs",
				"static_tool_capabilities",
				"agent_defaults_merge",
			},
			RuntimePrimitives: []string{
				"llm.register_models",
				"llm.tool",
				"llm.agent",
				"llm.agent_defaults",
				"llm.turn",
				"llm.toolof",
				"llm.agent_as_tool",
				"llm.validate_output",
				"dialect.eval",
				"dialect.eval_block",
				"dialect.eval_raw",
				"msg.system",
				"msg.user",
				"msg.assistant",
				"msg.assistant_call",
				"msg.tool_result",
				"msg.tool_error",
				"history.find",
				"history.find_all",
				"history.last",
				"history.append",
			},
			Tooling: []string{
				"fmt-parse-backed",
				"lint-json",
				"lint-sarif",
			},
		},
		Tooling: cliToolingCapability{
			Formatter: cliFormatterCapability{
				Stdin:     true,
				Check:     true,
				Write:     true,
				Formats:   []string{"source"},
				Reports:   []string{"json"},
				Stability: "whitespace-normalizer",
			},
			Linter: cliLinterCapability{
				Formats: []string{"text", "json", "sarif"},
				Codes:   []string{"LEIA0001", "LEIA1001", "LEIA2001"},
			},
			Test: cliTestCapability{
				GoldenStdout: true,
				GoldenModes:  []string{"auto", "require", "ignore", "update"},
				Directory:    true,
				List:         true,
				Reports:      []string{"json"},
				SeedEnv:      "LEIA_TEST_SEED",
			},
			Config: cliConfigCapability{
				FileName: "leia.toml",
				Sections: []string{
					"project",
					"tool.fmt",
					"tool.lint",
					"tool.test",
				},
				Formats: []string{"text", "json"},
			},
			ReportCount: len(reports),
			Reports:     reports,
		},
	}
}

func buildReportCapabilities() []cliReportCapability {
	return []cliReportCapability{
		{Command: "leia capabilities --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"command_count", "stdlib_module_count", "dialect_count", "tooling.report_count"}, CollectionFields: []string{"commands", "stdlib_modules", "stdlib_layers", "default_imports", "dialects", "tooling.reports"}},
		{Command: "leia check --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"step_count", "failed_count", "skipped_count"}},
		{Command: "leia ci --list --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"command_count"}, CollectionFields: []string{"commands"}},
		{Command: "leia config --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"diagnostic_count"}},
		{Command: "leia diag bundle --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"failure_count"}, CollectionFields: []string{"files"}},
		{Command: "leia doc check --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"failure_count", "counts.markdown_files", "counts.relative_documentation_links", "counts.runnable_spec_examples"}, CollectionFields: []string{"failures"}},
		{Command: "leia doc generate --format=json", Formats: []string{"json"}, SchemaVersion: 1, CountFields: []string{"cli.command_count", "stdlib.layer_count", "stdlib.default_import_count", "dialects.dialect_count"}, CollectionFields: []string{"cli.commands", "stdlib.layers", "stdlib.default_imports", "dialects.dialects"}},
		{Command: "leia env --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"capabilities.command_count", "capabilities.stdlib_module_count", "capabilities.dialect_count", "capabilities.tooling.report_count"}, CollectionFields: []string{"capabilities.commands", "capabilities.stdlib_modules", "capabilities.dialects", "capabilities.tooling.reports"}},
		{Command: "leia evaluate --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"summary", "metrics[].count"}},
		{Command: "leia examples check --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"result_count", "runnable", "skipped", "failed"}, CollectionFields: []string{"results"}},
		{Command: "leia examples list --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"example_count"}, CollectionFields: []string{"examples"}},
		{Command: "leia fmt --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"file_count", "changed_count", "diagnostic_count"}},
		{Command: "leia inspect bytecode --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"proto_count", "proto.instruction_count", "proto.constant_count", "proto.upvalue_count", "proto.child_proto_count"}},
		{Command: "leia inspect directives --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"directive_count"}, CollectionFields: []string{"directives"}},
		{Command: "leia lint --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"diagnostic_count", "error_count", "warning_count"}, CollectionFields: []string{"diagnostics"}},
		{Command: "leia mod capability --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"capability_count", "module_count", "diagnostic_count"}, CollectionFields: []string{"capabilities", "modules", "matrix", "diagnostics"}},
		{Command: "leia mod check --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"graph.file_count", "graph.diagnostic_count"}, CollectionFields: []string{"graph.files", "diagnostics"}},
		{Command: "leia mod download --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"module_count", "diagnostic_count"}, CollectionFields: []string{"modules", "diagnostics"}},
		{Command: "leia mod explain --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"diagnostic_count"}, CollectionFields: []string{"diagnostics"}},
		{Command: "leia mod gomod --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"diagnostic_count"}, CollectionFields: []string{"diagnostics"}},
		{Command: "leia mod graph --json", Formats: []string{"json"}, SchemaVersion: 1, CountFields: []string{"file_count", "diagnostic_count"}, CollectionFields: []string{"files", "diagnostics"}},
		{Command: "leia mod list --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"require_count", "replace_count", "collection_count", "diagnostic_count"}, CollectionFields: []string{"requires", "replaces", "collections", "diagnostics"}},
		{Command: "leia mod lock --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"entry_count", "diagnostic_count"}, CollectionFields: []string{"entries", "diagnostics"}},
		{Command: "leia mod tidy --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"removed_count", "missing_count", "diagnostic_count"}, CollectionFields: []string{"removed", "missing", "diagnostics"}},
		{Command: "leia mod vendor --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"module_count", "diagnostic_count"}, CollectionFields: []string{"modules", "diagnostics"}},
		{Command: "leia mod verify --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "ok", CountFields: []string{"graph.file_count", "graph.diagnostic_count"}, CollectionFields: []string{"graph.files", "diagnostics"}},
		{Command: "leia test --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"total", "passed", "failed"}},
		{Command: "leia test --list --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"file_count"}, CollectionFields: []string{"files"}},
		{Command: "leia version --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status"},
		{Command: "scripts/editor_check.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CollectionFields: []string{"textmate_grammars", "vscode_assets", "tree_sitter_assets", "smoke_tests"}},
		{Command: "scripts/install.sh --dry-run --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"install_count"}, CollectionFields: []string{"binaries", "install_paths"}},
		{Command: "scripts/performance_gate.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"failure_count"}, CollectionFields: []string{"failures", "output_lines"}},
		{Command: "scripts/production_check.sh --list --json", Formats: []string{"json"}, SchemaVersion: 1, CountFields: []string{"run_count", "skip_count", "critical_skip_count"}, CollectionFields: []string{"runnable_checks", "skipped_checks", "release_critical_skips"}},
		{Command: "scripts/public_release_blockers_check.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"blocker_count"}, CollectionFields: []string{"blockers", "blocker_details"}},
		{Command: "scripts/q_conformance_gate.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"language_case_count", "example_case_count", "benchmark_case_count"}, CollectionFields: []string{"language_cases", "example_cases", "benchmark_cases"}},
		{Command: "scripts/release_artifacts.sh --dry-run --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status"},
		{Command: "scripts/release_artifacts_check.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"artifact_count"}, CollectionFields: []string{"artifact_files"}},
		{Command: "scripts/release_distribution_check.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"workflow_count", "install_target_count"}, CollectionFields: []string{"workflow_files", "install_targets"}},
		{Command: "scripts/release_notes_check.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"checked_file_count", "failure_count"}, CollectionFields: []string{"checked_files", "failures"}},
		{Command: "scripts/worktree_audit.sh --json", Formats: []string{"json"}, SchemaVersion: 1, StatusField: "status", CountFields: []string{"finding_count"}, CollectionFields: []string{"findings"}},
	}
}

func buildDefaultImportCapabilities() []cliDefaultImport {
	aliases := stdinstall.DefaultAliases()
	out := make([]cliDefaultImport, len(aliases))
	for i, alias := range aliases {
		out[i] = cliDefaultImport{
			Name:   alias.Name,
			Module: alias.Module,
			Member: alias.Member,
		}
	}
	return out
}

func buildStdlibLayerCapabilities() []cliStdlibLayer {
	layerNames := catalog.Layers()
	layers := make([]cliStdlibLayer, 0, len(layerNames))
	for _, name := range layerNames {
		modules := make([]cliStdlibModule, 0)
		for _, module := range catalog.ModulesForLayer(name) {
			modules = append(modules, cliStdlibModule{
				Name:         module.Name,
				Description:  module.Description,
				Capabilities: append([]string(nil), module.Capabilities...),
				SafeDefault:  module.SafeDefault,
			})
		}
		sort.Slice(modules, func(i, j int) bool {
			return modules[i].Name < modules[j].Name
		})
		layers = append(layers, cliStdlibLayer{Name: name, Modules: modules})
	}
	return layers
}

func buildDialectCapabilities() []cliDialectCapability {
	infos := bind.BuiltinDialectInfos()
	dialects := make([]cliDialectCapability, 0, len(infos))
	for _, info := range infos {
		dialects = append(dialects, cliDialectCapability{
			Name:         info.Name,
			Category:     info.Category,
			Capabilities: append([]string(nil), info.Capabilities...),
			Builtin:      info.Builtin,
			Eval:         info.Eval,
			Block:        info.Block,
			Aliases:      append([]string(nil), info.Aliases...),
		})
	}
	sort.Slice(dialects, func(i, j int) bool {
		return dialects[i].Name < dialects[j].Name
	})
	return dialects
}
