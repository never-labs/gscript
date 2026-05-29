package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var benchExecCommand = exec.Command

func runBenchCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: gscript bench [compare|strict|diagnose] [benchmark-harness-flags...]")
		return 2
	}
	mode := args[0]
	harnessArgs := args[1:]
	if mode == "timing" {
		mode = "compare"
	}
	if len(mode) > 0 && mode[0] == '-' {
		mode = "compare"
		harnessArgs = args
	}

	script, err := benchScriptForMode(mode)
	if err != nil {
		fmt.Fprintf(errw, "gscript bench: %v\n", err)
		return 2
	}
	path, err := findBenchmarkScript(script)
	if err != nil {
		fmt.Fprintf(errw, "gscript bench: %v\n", err)
		return 1
	}
	python := os.Getenv("GSCRIPT_BENCH_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmdArgs := append([]string{path}, harnessArgs...)
	cmd := benchExecCommand(python, cmdArgs...)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(path))
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(errw, "gscript bench: %v\n", err)
		return 1
	}
	return 0
}

func benchScriptForMode(mode string) (string, error) {
	switch mode {
	case "compare":
		return "timing_compare.py", nil
	case "strict":
		return "strict_guard.py", nil
	case "diagnose":
		return "diagnose.py", nil
	case "help", "-h", "--help":
		return "", flag.ErrHelp
	default:
		return "", fmt.Errorf("unknown bench mode %q (want compare, strict, or diagnose)", mode)
	}
}

func findBenchmarkScript(name string) (string, error) {
	if name == "" {
		return "", errors.New("usage: gscript bench [compare|strict|diagnose] [benchmark-harness-flags...]")
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
