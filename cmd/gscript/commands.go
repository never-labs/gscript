package main

import (
	"io"
	"sort"
	"strings"
)

type cliCommandFunc func(args []string, outw, errw io.Writer) int

type cliCommandSpec struct {
	Name    string
	Usage   string
	Summary string
	Run     cliCommandFunc
}

func cliCommands() []cliCommandSpec {
	return []cliCommandSpec{
		{Name: "bench", Usage: "usage: gscript bench [--manifest-check|--quick|--full|--guard|BENCH|compare|strict|diagnose] [benchmark-harness-flags...]", Summary: "Run benchmark and benchmark-diagnostic harnesses.", Run: runBenchCommand},
		{Name: "capabilities", Usage: "usage: gscript capabilities [--json]", Summary: "Report binary capabilities, stdlib modules, and supported tooling formats.", Run: runCapabilitiesCommand},
		{Name: "check", Usage: "usage: gscript check [--json] [--no-fmt] [--no-lint] [--no-test] [--no-manifest] [--no-docs] [path-or-dir]", Summary: "Run formatter, linter, manifest, tests, and docs checks as one local gate.", Run: runCheckCommand},
		{Name: "ci", Usage: "usage: gscript ci [smoke|pr|perf|release] [--list] [--no-luajit]", Summary: "Run canonical local CI profiles.", Run: runCICommand},
		{Name: "config", Usage: "usage: gscript config [--json] [path]", Summary: "Discover and validate project configuration.", Run: runConfigCommand},
		{Name: "diag", Usage: "usage: gscript diag [dump|bundle] [diagnostic-flags...]", Summary: "Run production diagnostic dump and bundle tools.", Run: runDiagCommand},
		{Name: "diagnose", Usage: "usage: gscript diagnose <benchmark> [diagnose-flags...]", Summary: "Collect benchmark timing, exit, and Tier 2 diagnostics.", Run: runDiagnoseCommand},
		{Name: "doc", Usage: "usage: gscript doc [generate|check] [flags]", Summary: "Generate reference docs or validate repository docs.", Run: runDocCommand},
		{Name: "eval", Usage: "usage: gscript eval [--vm] [--jit=true|false] <source> [args...]", Summary: "Execute source passed on the command line.", Run: runEvalCommand},
		{Name: "env", Usage: "usage: gscript env [--json] [--path PATH]", Summary: "Report toolchain, project, cache, and platform environment.", Run: runEnvCommand},
		{Name: "fmt", Usage: "usage: gscript fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]", Summary: "Normalize source formatting.", Run: runFmtCommand},
		{Name: "help", Usage: "usage: gscript help [command]", Summary: "Show command help.", Run: runHelpCommand},
		{Name: "inspect", Usage: "usage: gscript inspect bytecode [--proto NAME] <file.gs>\n       gscript inspect directives [--json] <file.gs>", Summary: "Inspect compiled artifacts and file directives.", Run: runInspectCommand},
		{Name: "lint", Usage: "usage: gscript lint [--format=text|json|sarif] <path-or-dir> [...]", Summary: "Report source diagnostics.", Run: runLintCommand},
		{Name: "mod", Usage: "usage: gscript mod [init|graph|verify] [flags]", Summary: "Manage local module metadata and require graphs.", Run: runModCommand},
		{Name: "repl", Usage: "usage: gscript repl", Summary: "Start the interactive shell.", Run: runREPLCommand},
		{Name: "run", Usage: "usage: gscript run [--vm] [--jit=true|false] <file.gs> [args...]", Summary: "Run a script file.", Run: runRunCommand},
		{Name: "test", Usage: "usage: gscript test [--manifest-check] [--format=text|json] [--golden=auto|require|ignore|update] [--list] [--seed SEED] [path-or-dir]", Summary: "Run GScript test files and stdout goldens.", Run: runDefaultTestCommand},
		{Name: "version", Usage: "usage: gscript version [--json]", Summary: "Report binary version and build metadata.", Run: runVersionCommand},
	}
}

func lookupCLICommand(name string) (cliCommandSpec, bool) {
	for _, spec := range cliCommands() {
		if spec.Name == name {
			return spec, true
		}
	}
	return cliCommandSpec{}, false
}

func cliCommandNames() []string {
	registry := cliCommands()
	commands := make([]string, 0, len(registry))
	for _, spec := range registry {
		commands = append(commands, spec.Name)
	}
	sort.Strings(commands)
	return commands
}

func cliHelpTopics() map[string]cliHelpTopic {
	registry := cliCommands()
	out := make(map[string]cliHelpTopic, len(registry))
	for _, spec := range registry {
		out[spec.Name] = cliHelpTopic{
			Command: spec.Name,
			Usage:   spec.Usage,
			Summary: strings.TrimSpace(spec.Summary),
		}
	}
	return out
}

func runDefaultTestCommand(args []string, outw, errw io.Writer) int {
	return runTestCommand(args, cliRunOptions{UseVM: true, UseJIT: true}, outw, errw)
}
