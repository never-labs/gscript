package main

import (
	"flag"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

var ciExecCommand = exec.Command

const (
	ciSmokeScriptPath    = "tests/smoke/01_basic.leia"
	ciExpectedModulePath = "github.com/never-labs/leia"
)

type ciCommand struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

func runCICommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia ci [smoke|pr|perf|release] [--list] [--no-luajit]")
		return 2
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(outw, "usage: leia ci [smoke|pr|perf|release] [--list] [--no-luajit]")
		return 0
	}
	profile := args[0]
	fs := flag.NewFlagSet("ci "+profile, flag.ContinueOnError)
	fs.SetOutput(errw)
	listOnly := fs.Bool("list", false, "print commands without running them")
	noLuaJIT := fs.Bool("no-luajit", false, "skip LuaJIT where supported")
	if code, done := parseCLIFlags(fs, args[1:]); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia ci [smoke|pr|perf|release] [--list] [--no-luajit]")
		return 2
	}
	commands, err := ciProfileCommands(profile, *noLuaJIT)
	if err != nil {
		fmt.Fprintf(errw, "leia ci: %v\n", err)
		return 2
	}
	if *listOnly {
		for _, cmd := range commands {
			fmt.Fprintln(outw, shellJoin(cmd.Args))
		}
		return 0
	}
	for _, spec := range commands {
		if len(spec.Args) == 0 {
			continue
		}
		fmt.Fprintf(outw, "=== %s ===\n%s\n", spec.Name, shellJoin(spec.Args))
		cmd := ciExecCommand(spec.Args[0], spec.Args[1:]...)
		cmd.Stdout = outw
		cmd.Stderr = errw
		if code := runExternalCommand(cmd, "leia ci", errw); code != 0 {
			return code
		}
	}
	return 0
}

func ciProfileCommands(profile string, noLuaJIT bool) ([]ciCommand, error) {
	switch profile {
	case "smoke":
		return []ciCommand{
			{Name: "Go smoke tests", Args: []string{"go", "test", "./cmd/leia", "./cmd/leia-lsp", ".", "./internal/runtime", "./internal/vm", "./internal/tooling/lsp", "-count=1"}},
			{Name: "Manifest coverage", Args: manifestCoverageCommand()},
			{Name: "Module path gate", Args: modulePathGateCommand()},
			{Name: "Tooling check", Args: []string{"go", "run", "./cmd/leia", "check", "--no-test", "--no-docs", "--no-editor", ciSmokeScriptPath}},
			{Name: "Worktree audit", Args: []string{"bash", "scripts/worktree_audit.sh"}},
		}, nil
	case "pr":
		return []ciCommand{
			{Name: "All Go tests", Args: []string{"go", "test", "./...", "-count=1"}},
			{Name: "Manifest coverage", Args: manifestCoverageCommand()},
			{Name: "Module path gate", Args: modulePathGateCommand()},
			{Name: "Example projects check", Args: []string{"go", "run", "./cmd/leia", "examples", "check", "--jobs=6"}},
			{Name: "Docs check", Args: []string{"go", "run", "./cmd/leia", "doc", "check"}},
			{Name: "Performance smoke", Args: appendNoLuaJIT([]string{"bash", "scripts/performance_gate.sh", "--smoke"}, noLuaJIT)},
		}, nil
	case "perf":
		return []ciCommand{
			{Name: "Performance gate", Args: appendNoLuaJIT([]string{"bash", "scripts/performance_gate.sh", "--full"}, noLuaJIT)},
		}, nil
	case "release":
		return []ciCommand{
			{Name: "Module path gate", Args: modulePathGateCommand()},
			{Name: "Performance gate", Args: []string{"bash", "scripts/performance_gate.sh", "--full"}},
			{Name: "Production check", Args: []string{"bash", "scripts/production_check.sh", "--full"}},
			{Name: "Release distribution check", Args: []string{"bash", "scripts/release_distribution_check.sh"}},
			{Name: "Release artifacts check", Args: []string{"bash", "scripts/release_artifacts_check.sh", "--build"}},
		}, nil
	default:
		return nil, fmt.Errorf("unknown ci profile %q (want smoke, pr, perf, or release)", profile)
	}
}

func manifestCoverageCommand() []string {
	return []string{"python3", "tests/manifest.py", "check", "tests", "benchmarks"}
}

func modulePathGateCommand() []string {
	return []string{"bash", "-c", fmt.Sprintf("test \"$(go list -m)\" = %q", ciExpectedModulePath)}
}

func appendNoLuaJIT(args []string, noLuaJIT bool) []string {
	if noLuaJIT {
		return append(args, "--no-luajit")
	}
	return args
}

func shellJoin(args []string) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		if arg == "" || strings.ContainsAny(arg, " \t\n'\"\\$`") {
			parts[i] = "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
		} else {
			parts[i] = arg
		}
	}
	return strings.Join(parts, " ")
}
