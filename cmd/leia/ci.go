package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
)

var ciExecCommand = exec.Command
var ciReleaseVersionRE = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$`)

const (
	ciSmokeScriptPath    = "tests/smoke/01_basic.leia"
	ciExpectedModulePath = "github.com/never-labs/leia"
)

type ciCommand struct {
	Name string   `json:"name"`
	Args []string `json:"args"`
}

type ciPlanReport struct {
	SchemaVersion  int             `json:"schema_version"`
	Status         string          `json:"status"`
	Profile        string          `json:"profile"`
	ListOnly       bool            `json:"list_only"`
	NoLuaJIT       bool            `json:"no_luajit"`
	ReleaseVersion string          `json:"release_version,omitempty"`
	CommandCount   int             `json:"command_count"`
	Commands       []ciPlanCommand `json:"commands"`
}

type ciPlanCommand struct {
	Name     string   `json:"name"`
	Args     []string `json:"args"`
	ArgCount int      `json:"arg_count"`
	Command  string   `json:"command"`
}

func runCICommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, ciUsage)
		return 2
	}
	if args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		fmt.Fprintln(outw, ciUsage)
		return 0
	}
	profile := args[0]
	fs := flag.NewFlagSet("ci "+profile, flag.ContinueOnError)
	fs.SetOutput(errw)
	listOnly := fs.Bool("list", false, "print commands without running them")
	jsonOut := fs.Bool("json", false, "print command plan as JSON; valid with --list")
	noLuaJIT := fs.Bool("no-luajit", false, "skip LuaJIT where supported")
	releaseVersion := fs.String("release-version", "", "release tag to validate with the release profile")
	if code, done := parseCLIFlags(fs, args[1:]); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, ciUsage)
		return 2
	}
	if *jsonOut && !*listOnly {
		fmt.Fprintln(errw, "leia ci: --json is only supported with --list")
		return 2
	}
	if profile == "release" && *noLuaJIT {
		fmt.Fprintln(errw, "leia ci: release profile requires LuaJIT evidence; use pr or perf --no-luajit for non-release checks")
		return 2
	}
	if profile != "release" && *releaseVersion != "" {
		fmt.Fprintln(errw, "leia ci: --release-version is only valid with the release profile")
		return 2
	}
	if profile == "release" && *releaseVersion != "" && !*listOnly && !ciReleaseVersionRE.MatchString(*releaseVersion) {
		fmt.Fprintf(errw, "leia ci: --release-version must match vMAJOR.MINOR.PATCH or prerelease: %s\n", *releaseVersion)
		return 2
	}
	commands, err := ciProfileCommands(profile, *noLuaJIT, *releaseVersion)
	if err != nil {
		fmt.Fprintf(errw, "leia ci: %v\n", err)
		return 2
	}
	if *listOnly {
		if *jsonOut {
			if err := json.NewEncoder(outw).Encode(ciPlan(profile, *noLuaJIT, *releaseVersion, commands)); err != nil {
				fmt.Fprintf(errw, "leia ci: write JSON plan: %v\n", err)
				return 1
			}
			return 0
		}
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

const ciUsage = "usage: leia ci [smoke|pr|perf|release] [--list] [--json] [--no-luajit] [--release-version VERSION]"

func ciPlan(profile string, noLuaJIT bool, releaseVersion string, commands []ciCommand) ciPlanReport {
	planCommands := make([]ciPlanCommand, 0, len(commands))
	for _, cmd := range commands {
		planCommands = append(planCommands, ciPlanCommand{
			Name:     cmd.Name,
			Args:     append([]string(nil), cmd.Args...),
			ArgCount: len(cmd.Args),
			Command:  shellJoin(cmd.Args),
		})
	}
	return ciPlanReport{
		SchemaVersion:  1,
		Status:         "pass",
		Profile:        profile,
		ListOnly:       true,
		NoLuaJIT:       noLuaJIT,
		ReleaseVersion: releaseVersion,
		CommandCount:   len(planCommands),
		Commands:       planCommands,
	}
}

func ciProfileCommands(profile string, noLuaJIT bool, releaseVersion string) ([]ciCommand, error) {
	switch profile {
	case "smoke":
		return []ciCommand{
			{Name: "Go smoke tests", Args: []string{"go", "test", "./cmd/leia", "./cmd/leia-lsp", ".", "./internal/runtime", "./internal/vm", "./internal/tooling/lsp", "-count=1"}},
			{Name: "Manifest coverage", Args: manifestCoverageCommand()},
			{Name: "Module path gate", Args: modulePathGateCommand()},
			{Name: "Tooling check", Args: []string{"go", "run", "./cmd/leia", "check", "--no-test", "--no-docs", "--no-editor", ciSmokeScriptPath}},
			{Name: "Worktree audit", Args: []string{"scripts/run.sh", "worktree"}},
		}, nil
	case "pr":
		return []ciCommand{
			{Name: "All Go tests", Args: []string{"go", "test", "./...", "-count=1"}},
			{Name: "Manifest coverage", Args: manifestCoverageCommand()},
			{Name: "Module path gate", Args: modulePathGateCommand()},
			{Name: "Example projects check", Args: []string{"go", "run", "./cmd/leia", "examples", "check", "--jobs=6"}},
			{Name: "Docs check", Args: []string{"scripts/run.sh", "docs"}},
			{Name: "Performance smoke", Args: appendNoLuaJIT([]string{"scripts/run.sh", "perf", "--smoke"}, noLuaJIT)},
		}, nil
	case "perf":
		return []ciCommand{
			{Name: "Performance gate", Args: appendNoLuaJIT([]string{"scripts/run.sh", "perf", "--full"}, noLuaJIT)},
		}, nil
	case "release":
		args := []string{"scripts/run.sh", "production", "--full", "--release-profile"}
		if releaseVersion != "" {
			args = append(args, "--release-version", releaseVersion)
		}
		return []ciCommand{
			{Name: "Production check", Args: args},
		}, nil
	default:
		return nil, fmt.Errorf("unknown ci profile %q (want smoke, pr, perf, or release)", profile)
	}
}

func manifestCoverageCommand() []string {
	return []string{"scripts/run.sh", "manifest-check", "tests", "benchmarks"}
}

func modulePathGateCommand() []string {
	return []string{"scripts/run.sh", "module-path", ciExpectedModulePath}
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
