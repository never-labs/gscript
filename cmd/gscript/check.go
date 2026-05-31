package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

var checkExecCommand = exec.Command

type checkReport struct {
	OK    bool              `json:"ok"`
	Steps []checkStepReport `json:"steps"`
}

type checkStepReport struct {
	Name     string `json:"name"`
	OK       bool   `json:"ok"`
	ExitCode int    `json:"exit_code"`
	Skipped  bool   `json:"skipped,omitempty"`
}

func runCheckCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print check results as JSON")
	noFmt := fs.Bool("no-fmt", false, "skip formatter check")
	noLint := fs.Bool("no-lint", false, "skip lint")
	noTest := fs.Bool("no-test", false, "skip tests")
	noManifest := fs.Bool("no-manifest", false, "skip test and benchmark manifest coverage check")
	noDocs := fs.Bool("no-docs", false, "skip docs check")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: gscript check [--json] [--no-fmt] [--no-lint] [--no-test] [--no-manifest] [--no-docs] [path-or-dir]")
		return 2
	}

	path := paths[0]
	report := checkReport{OK: true}
	runStep := func(name string, skipped bool, fn func() int) {
		step := checkStepReport{Name: name, Skipped: skipped}
		if skipped {
			step.OK = true
			report.Steps = append(report.Steps, step)
			return
		}
		step.ExitCode = fn()
		step.OK = step.ExitCode == 0
		if !step.OK {
			report.OK = false
		}
		report.Steps = append(report.Steps, step)
	}

	runStep("fmt", *noFmt, func() int {
		return runFmtCommand([]string{"--check", path}, io.Discard, errw)
	})
	runStep("lint", *noLint, func() int {
		return runLintCommand([]string{path}, io.Discard, errw)
	})
	runStep("test", *noTest, func() int {
		return runTestCommand([]string{path}, cliRunOptions{UseVM: false}, io.Discard, errw)
	})
	runStep("manifest", *noManifest, func() int {
		return runManifestCheck(errw, errw)
	})
	runStep("docs", *noDocs, func() int {
		docsOut := outw
		if *jsonOut {
			docsOut = errw
		}
		return runDocsCheck(docsOut, errw)
	})

	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript check: write json: %v\n", err)
			return 1
		}
	} else {
		for _, step := range report.Steps {
			status := "ok"
			if step.Skipped {
				status = "skipped"
			} else if !step.OK {
				status = "failed"
			}
			fmt.Fprintf(outw, "%s: %s\n", step.Name, status)
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}

func runManifestCheck(outw, errw io.Writer) int {
	return runManifestCheckRoots([]string{"tests", "benchmarks"}, outw, errw)
}

func runManifestCheckRoots(roots []string, outw, errw io.Writer) int {
	script, err := findScriptFromCWD(filepath.Join("tests", "manifest.py"))
	if err != nil {
		fmt.Fprintf(errw, "gscript check: %v\n", err)
		return 1
	}
	python := os.Getenv("GSCRIPT_CHECK_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmdArgs := append([]string{script, "check"}, roots...)
	cmd := checkExecCommand(python, cmdArgs...)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	return runExternalCommand(cmd, "gscript check", errw)
}

func runDocsCheck(outw, errw io.Writer) int {
	script, err := findScriptFromCWD(filepath.Join("scripts", "docs_check.sh"))
	if err != nil {
		fmt.Fprintf(errw, "gscript check: %v\n", err)
		return 1
	}
	cmd := checkExecCommand("bash", script)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	return runExternalCommand(cmd, "gscript check", errw)
}

func findScriptFromCWD(rel string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, rel)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find %s from %s", rel, dir)
		}
		dir = parent
	}
}
