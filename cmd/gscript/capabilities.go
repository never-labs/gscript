package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/never-labs/gscript/internal/stdlib/catalog"
)

type cliCapabilities struct {
	SchemaVersion int                    `json:"schema_version"`
	Platform      cliPlatformCapability  `json:"platform"`
	Execution     cliExecutionCapability `json:"execution"`
	Commands      []string               `json:"commands"`
	StdlibModules []string               `json:"stdlib_modules"`
	StdlibLayers  []cliStdlibLayer       `json:"stdlib_layers"`
	LLM           cliLLMCapability       `json:"llm"`
	Tooling       cliToolingCapability   `json:"tooling"`
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
	Capabilities []string `json:"capabilities,omitempty"`
	SafeDefault  bool     `json:"safe_default,omitempty"`
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
	Formatter cliFormatterCapability `json:"formatter"`
	Linter    cliLinterCapability    `json:"linter"`
	Test      cliTestCapability      `json:"test"`
	Config    cliConfigCapability    `json:"config"`
}

type cliFormatterCapability struct {
	Stdin     bool     `json:"stdin"`
	Check     bool     `json:"check"`
	Write     bool     `json:"write"`
	Formats   []string `json:"formats"`
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
	SeedEnv      string   `json:"seed_env"`
}

func runCapabilitiesCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print capabilities as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript capabilities [--json]")
		return 2
	}
	caps := buildCapabilities()
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(caps); err != nil {
			fmt.Fprintf(errw, "gscript capabilities: write json: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(outw, "platform: %s/%s\n", caps.Platform.GOOS, caps.Platform.GOARCH)
	fmt.Fprintf(outw, "jit: %t\n", caps.Execution.JIT)
	fmt.Fprintf(outw, "llm: %t (%s)\n", caps.LLM.Enabled, strings.Join(caps.LLM.Syntax, ", "))
	fmt.Fprintf(outw, "commands: %s\n", strings.Join(caps.Commands, ", "))
	fmt.Fprintf(outw, "stdlib modules: %d\n", len(caps.StdlibModules))
	return 0
}

func buildCapabilities() cliCapabilities {
	modules := catalog.ModuleNames()
	sort.Strings(modules)
	return cliCapabilities{
		SchemaVersion: 1,
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
		Commands:      cliCommandNames(),
		StdlibModules: modules,
		StdlibLayers:  buildStdlibLayerCapabilities(),
		LLM: cliLLMCapability{
			Enabled: true,
			Syntax: []string{
				"models",
				"tool",
				"agent defaults",
				"agent",
				"direct_agent_tools",
				"turn",
				"messages",
				"messages_bare_expr",
				"budget",
				"toolof",
			},
			ToolMetadata: []string{
				"doc_comment",
				"gscript:requires",
				"gscript:param",
			},
			StaticValidation: []string{
				"duplicate_agent_defaults",
				"tool_requires",
				"tool_param_docs",
				"static_tool_capabilities",
				"agent_defaults_merge",
			},
			RuntimePrimitives: []string{
				"llm.models",
				"llm.tool",
				"llm.agent",
				"llm.turn",
				"llm.messages",
				"llm.budget",
				"llm.toolof",
				"llm.agent_as_tool",
				"llm.validate_output",
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
				Stability: "whitespace-normalizer",
			},
			Linter: cliLinterCapability{
				Formats: []string{"text", "json", "sarif"},
				Codes:   []string{"GS0001", "GS1001"},
			},
			Test: cliTestCapability{
				GoldenStdout: true,
				GoldenModes:  []string{"auto", "require", "ignore", "update"},
				Directory:    true,
				List:         true,
				SeedEnv:      "GSCRIPT_TEST_SEED",
			},
			Config: cliConfigCapability{
				FileName: "gscript.toml",
				Sections: []string{
					"project",
					"tool.fmt",
					"tool.lint",
					"tool.test",
				},
				Formats: []string{"text", "json"},
			},
		},
	}
}

func buildStdlibLayerCapabilities() []cliStdlibLayer {
	layerNames := catalog.Layers()
	layers := make([]cliStdlibLayer, 0, len(layerNames))
	for _, name := range layerNames {
		modules := make([]cliStdlibModule, 0)
		for _, module := range catalog.ModulesForLayer(name) {
			modules = append(modules, cliStdlibModule{
				Name:         module.Name,
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
