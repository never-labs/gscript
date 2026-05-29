package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type cliHelpTopic struct {
	Command string
	Usage   string
	Summary string
}

func runHelpCommand(args []string, outw, errw io.Writer) int {
	topics := cliHelpTopics()
	if len(args) > 1 {
		fmt.Fprintln(errw, "usage: gscript help [command]")
		return 2
	}
	if len(args) == 1 {
		topic, ok := topics[args[0]]
		if !ok {
			fmt.Fprintf(errw, "gscript help: unknown command %q\n", args[0])
			return 2
		}
		fmt.Fprintf(outw, "%s\n\n%s\n", topic.Usage, topic.Summary)
		return 0
	}

	fmt.Fprintln(outw, "usage: gscript <command> [args]")
	fmt.Fprintln(outw)
	fmt.Fprintln(outw, "Commands:")
	commands := make([]string, 0, len(topics))
	for command := range topics {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	for _, command := range commands {
		topic := topics[command]
		fmt.Fprintf(outw, "  %-12s %s\n", command, topic.Summary)
	}
	fmt.Fprintln(outw)
	fmt.Fprintln(outw, "Use `gscript help <command>` for command-specific usage.")
	return 0
}

func cliHelpTopics() map[string]cliHelpTopic {
	topics := []cliHelpTopic{
		{Command: "bench", Usage: "usage: gscript bench [compare|strict|diagnose] [benchmark-harness-flags...]", Summary: "Run benchmark and benchmark-diagnostic harnesses."},
		{Command: "capabilities", Usage: "usage: gscript capabilities [--json]", Summary: "Report binary capabilities, stdlib modules, and supported tooling formats."},
		{Command: "check", Usage: "usage: gscript check [--json] [--no-fmt] [--no-lint] [--no-test] [--no-docs] <path-or-dir>", Summary: "Run formatter, linter, tests, and docs checks as one local gate."},
		{Command: "ci", Usage: "usage: gscript ci [smoke|pr|perf|release] [--list] [--no-luajit]", Summary: "Run canonical local CI profiles."},
		{Command: "config", Usage: "usage: gscript config [--json] [path]", Summary: "Discover and validate project configuration."},
		{Command: "diag", Usage: "usage: gscript diag [dump|bundle] [diagnostic-flags...]", Summary: "Run production diagnostic dump and bundle tools."},
		{Command: "doc", Usage: "usage: gscript doc [generate|check] [flags]", Summary: "Generate reference docs or validate repository docs."},
		{Command: "eval", Usage: "usage: gscript eval [--vm] [--jit=true|false] <source> [args...]", Summary: "Execute source passed on the command line."},
		{Command: "env", Usage: "usage: gscript env [--json] [--path PATH]", Summary: "Report toolchain, project, cache, and platform environment."},
		{Command: "fmt", Usage: "usage: gscript fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]", Summary: "Normalize source formatting."},
		{Command: "help", Usage: "usage: gscript help [command]", Summary: "Show command help."},
		{Command: "inspect", Usage: "usage: gscript inspect bytecode [--proto NAME] <file.gs>", Summary: "Inspect compiled artifacts."},
		{Command: "lint", Usage: "usage: gscript lint [--format=text|json|sarif] <path-or-dir> [...]", Summary: "Report source diagnostics."},
		{Command: "mod", Usage: "usage: gscript mod [init|graph|verify] [flags]", Summary: "Manage local module metadata and require graphs."},
		{Command: "repl", Usage: "usage: gscript repl", Summary: "Start the interactive shell."},
		{Command: "run", Usage: "usage: gscript run [--vm] [--jit=true|false] <file.gs> [args...]", Summary: "Run a script file."},
		{Command: "test", Usage: "usage: gscript test [--format=text|json] <path-or-dir>", Summary: "Run GScript test files and stdout goldens."},
		{Command: "version", Usage: "usage: gscript version [--json]", Summary: "Report binary version and build metadata."},
	}
	out := make(map[string]cliHelpTopic, len(topics))
	for _, topic := range topics {
		topic.Summary = strings.TrimSpace(topic.Summary)
		out[topic.Command] = topic
	}
	return out
}

func cliCommandNames() []string {
	topics := cliHelpTopics()
	commands := make([]string, 0, len(topics))
	for command := range topics {
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return commands
}
