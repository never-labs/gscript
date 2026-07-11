package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var benchExecCommand = exec.Command

func runBenchCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 1 && args[0] == "--manifest-check" {
		return runManifestCheckRoots([]string{"benchmarks"}, outw, errw)
	}
	if len(args) == 0 {
		return runBenchCompareCommand([]string{"--bench", "control/sieve", "--runs", "1", "--warmup", "0", "--timeout", "60"}, outw, errw)
	}
	if profileMode, profileArgs, rest, ok := benchProfileArgs(args); ok {
		harnessArgs := append(profileArgs, rest...)
		if profileMode == "strict" {
			return runBenchStrictCommand(harnessArgs, outw, errw)
		}
		return runBenchCompareCommand(harnessArgs, outw, errw)
	}
	mode := args[0]
	harnessArgs := args[1:]
	if mode == "timing" {
		mode = "compare"
	}
	if len(mode) > 0 && mode[0] == '-' {
		mode = "compare"
		harnessArgs = args
	} else if isBenchmarkSelector(mode) {
		mode = "compare"
		harnessArgs = append([]string{"--bench", args[0]}, args[1:]...)
	}
	if mode == "compare" {
		if profileMode, profileArgs, rest, ok := benchProfileArgs(harnessArgs); ok && profileMode == "compare" {
			harnessArgs = append(profileArgs, rest...)
		}
		return runBenchCompareCommand(harnessArgs, outw, errw)
	}
	if mode == "strict" {
		return runBenchStrictCommand(harnessArgs, outw, errw)
	}
	if mode == "diagnose" {
		return runBenchDiagnoseCommand(harnessArgs, outw, errw)
	}
	if mode == "triage" {
		return runBenchTriageCommand(harnessArgs, outw, errw)
	}
	if mode == "audit" {
		return runBenchAuditCommand(harnessArgs, outw, errw)
	}
	if mode == "rank-luajit-gaps" || mode == "rank-luajit" {
		return runBenchRankLuaJITGapsCommand(harnessArgs, outw, errw)
	}
	if mode == "debug-artifact" {
		return runBenchDebugArtifactCommand(harnessArgs, outw, errw)
	}
	if mode == "gate-validate" {
		return runBenchGateValidateCommand(harnessArgs, outw, errw)
	}
	if mode == "coverage" {
		return runBenchCoverageCommand(harnessArgs, outw, errw)
	}
	if mode == "profile-exits" || mode == "exits" {
		return runBenchProfileExitsCommand(harnessArgs, outw, errw)
	}
	if mode == "validate-lua-refs" || mode == "lua-refs" {
		return runBenchValidateLuaRefsCommand(harnessArgs, outw, errw)
	}
	if mode == "submit-guard" {
		return runBenchSubmitGuardCommand(harnessArgs, outw, errw)
	}
	if mode == "jit-addr-map" {
		return runBenchJITAddrMapCommand(harnessArgs, outw, errw)
	}
	if mode == "regression-guard" {
		return runBenchRegressionGuardCommand(harnessArgs, outw, errw)
	}
	fmt.Fprintf(errw, "leia bench: unknown bench mode %q (want compare, strict, diagnose, triage, audit, rank-luajit-gaps, debug-artifact, gate-validate, coverage, profile-exits, validate-lua-refs, submit-guard, jit-addr-map, or regression-guard)\n", mode)
	return 2
}

func runBenchShellScript(script string, args []string, outw, errw io.Writer) int {
	path, err := findBenchmarkScript(script)
	if err != nil {
		fmt.Fprintf(errw, "leia bench: %v\n", err)
		return 1
	}
	cmd := benchExecCommand("bash", append([]string{path}, args...)...)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(path))
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(errw, "leia bench: %v\n", err)
		return 1
	}
	return 0
}

func runDiagnoseCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia diagnose <benchmark> [diagnose-flags...]")
		return 2
	}
	return runBenchDiagnoseCommand(normalizeBenchmarkSelectorArgs(args), outw, errw)
}

func benchProfileArgs(args []string) (string, []string, []string, bool) {
	if len(args) == 0 {
		return "", nil, nil, false
	}
	switch args[0] {
	case "--quick":
		return "compare", []string{"--bench", "control/sieve", "--bench", "table/table_array_access", "--runs", "1", "--warmup", "0", "--timeout", "60"}, args[1:], true
	case "--full":
		return "compare", []string{"--all-groups"}, args[1:], true
	case "--guard":
		return "strict", []string{"--bench", "control/sieve", "--bench", "table/table_array_access", "--runs", "1", "--warmup", "0", "--timeout", "60"}, args[1:], true
	default:
		return "", nil, nil, false
	}
}

func normalizeBenchmarkSelectorArgs(args []string) []string {
	if len(args) == 0 || len(args[0]) == 0 || args[0][0] == '-' {
		return args
	}
	return append([]string{"--bench", args[0]}, args[1:]...)
}

func isBenchmarkSelector(arg string) bool {
	if arg == "" || arg[0] == '-' {
		return false
	}
	switch arg {
	case "audit", "rank-luajit-gaps", "rank-luajit", "debug-artifact", "gate-validate", "coverage", "profile-exits", "exits", "validate-lua-refs", "lua-refs", "submit-guard", "jit-addr-map", "regression-guard", "compare", "timing", "strict", "diagnose", "triage", "help":
		return false
	default:
		return true
	}
}

func findBenchmarkScript(name string) (string, error) {
	if name == "" {
		return "", errors.New("usage: leia bench [--quick|--full|--guard|BENCH|compare|strict|diagnose|triage] [benchmark-harness-flags...]")
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "benchmarks", name)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find benchmarks/%s from %s", name, dir)
		}
		dir = parent
	}
}
