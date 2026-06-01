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
		{Name: "bench", Usage: "usage: leia bench [--manifest-check|--quick|--full|--guard|BENCH|compare|strict|diagnose] [benchmark-harness-flags...]", Summary: "Run benchmark and benchmark-diagnostic harnesses.", Run: runBenchCommand},
		{Name: "capabilities", Usage: "usage: leia capabilities [--json]", Summary: "Report binary capabilities, stdlib modules, and supported tooling formats.", Run: runCapabilitiesCommand},
		{Name: "check", Usage: "usage: leia check [--json] [--no-fmt] [--no-lint] [--no-test] [--no-manifest] [--no-docs] [path-or-dir]", Summary: "Run formatter, linter, manifest, tests, and docs checks as one local gate.", Run: runCheckCommand},
		{Name: "ci", Usage: "usage: leia ci [smoke|pr|perf|release] [--list] [--no-luajit]", Summary: "Run canonical local CI profiles.", Run: runCICommand},
		{Name: "config", Usage: "usage: leia config [--json] [path]", Summary: "Discover and validate project configuration.", Run: runConfigCommand},
		{Name: "diag", Usage: "usage: leia diag [dump|bundle] [diagnostic-flags...]", Summary: "Run production diagnostic dump and bundle tools.", Run: runDiagCommand},
		{Name: "diagnose", Usage: "usage: leia diagnose <benchmark> [diagnose-flags...]", Summary: "Collect benchmark timing, exit, and Tier 2 diagnostics.", Run: runDiagnoseCommand},
		{Name: "doc", Usage: "usage: leia doc [generate|check] [flags]", Summary: "Generate reference docs or validate repository docs.", Run: runDocCommand},
		{Name: "eval", Usage: "usage: leia eval [--vm] [--jit=true|false] <source> [args...]", Summary: "Execute source passed on the command line.", Run: runEvalCommand},
		{Name: "evaluate", Usage: "usage: leia evaluate [--json|--format=text] [path-or-dir...]", Summary: "Run evaluate blocks and emit an agent evaluation report.", Run: runEvaluateCommand},
		{Name: "env", Usage: "usage: leia env [--json] [--path PATH]", Summary: "Report toolchain, project, cache, and platform environment.", Run: runEnvCommand},
		{Name: "fmt", Usage: "usage: leia fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]", Summary: "Normalize source formatting.", Run: runFmtCommand},
		{Name: "help", Usage: "usage: leia help [command]", Summary: "Show command help.", Run: runHelpCommand},
		{Name: "inspect", Usage: "usage: leia inspect bytecode [--proto NAME] <file.leia>\n       leia inspect directives [--json] <file.leia>", Summary: "Inspect compiled artifacts and file directives.", Run: runInspectCommand},
		{Name: "lint", Usage: "usage: leia lint [--format=text|json|sarif] <path-or-dir> [...]", Summary: "Report source diagnostics.", Run: runLintCommand},
		{Name: "mod", Usage: "usage: leia mod [init|add|tidy|check|download|vendor|lock|list|graph|explain|capability|gomod|verify] [flags]", Summary: "Manage local module metadata and require graphs.", Run: runModCommand},
		{Name: "playground", Usage: "usage: leia playground [--addr ADDR] [--timeout DURATION] [--max-source-bytes N] [--max-steps N]", Summary: "Serve the local backend-powered Leia playground.", Run: runPlaygroundCommand},
		{Name: "repl", Usage: "usage: leia repl", Summary: "Start the interactive shell.", Run: runREPLCommand},
		{Name: "run", Usage: "usage: leia run [--vm] [--jit=true|false] [--mod=readonly|vendor|mod] <file.leia> [args...]", Summary: "Run a script file.", Run: runRunCommand},
		{Name: "test", Usage: "usage: leia test [--manifest-check] [--format=text|json] [--golden=auto|require|ignore|update] [--list] [--seed SEED] [path-or-dir]", Summary: "Run Leia test files and stdout goldens.", Run: runDefaultTestCommand},
		{Name: "version", Usage: "usage: leia version [--json]", Summary: "Report binary version and build metadata.", Run: runVersionCommand},
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
