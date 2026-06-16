package main

import (
	"bytes"
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
	SchemaVersion int               `json:"schema_version"`
	OK            bool              `json:"ok"`
	StepCount     int               `json:"step_count"`
	FailedCount   int               `json:"failed_count"`
	SkippedCount  int               `json:"skipped_count"`
	Steps         []checkStepReport `json:"steps"`
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
	quick := fs.Bool("quick", false, "run the fast local gate: fmt, lint, and test only")
	full := fs.Bool("full", false, "run the full local gate")
	noFmt := fs.Bool("no-fmt", false, "skip formatter check")
	noLint := fs.Bool("no-lint", false, "skip lint")
	noTest := fs.Bool("no-test", false, "skip tests")
	noManifest := fs.Bool("no-manifest", false, "skip test and benchmark manifest coverage check")
	noDocs := fs.Bool("no-docs", false, "skip docs check")
	noEditor := fs.Bool("no-editor", false, "skip editor asset check")
	noExamples := fs.Bool("no-examples", false, "skip repository example project check")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if *quick && *full {
		fmt.Fprintln(errw, "leia check: --quick and --full are mutually exclusive")
		return 2
	}
	if *quick {
		*noManifest = true
		*noDocs = true
		*noEditor = true
		*noExamples = true
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{"."}
	}
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: leia check [--json] [--quick|--full] [--no-fmt] [--no-lint] [--no-test] [--no-manifest] [--no-docs] [--no-editor] [--no-examples] [path-or-dir]")
		return 2
	}

	path := paths[0]
	toolPath := checkToolingPath(path)
	report := checkReport{SchemaVersion: 1, OK: true}
	runStep := func(name string, skipped bool, fn func() int) {
		step := checkStepReport{Name: name, Skipped: skipped}
		if skipped {
			step.OK = true
			report.Steps = append(report.Steps, step)
			report.StepCount++
			report.SkippedCount++
			return
		}
		step.ExitCode = fn()
		step.OK = step.ExitCode == 0
		if !step.OK {
			report.OK = false
			report.FailedCount++
		}
		report.Steps = append(report.Steps, step)
		report.StepCount++
	}

	runStep("fmt", *noFmt, func() int {
		return runFmtCommand([]string{"--check", toolPath}, io.Discard, errw)
	})
	runStep("lint", *noLint, func() int {
		return runLintCommand([]string{toolPath}, io.Discard, errw)
	})
	runStep("test", *noTest, func() int {
		return runTestCommand([]string{toolPath}, cliRunOptions{UseVM: false}, io.Discard, errw)
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
	runStep("editor", *noEditor, func() int {
		editorOut := outw
		if *jsonOut {
			editorOut = errw
		}
		return runEditorCheck(editorOut, errw)
	})
	runStep("examples", *noExamples, func() int {
		examplesOut := outw
		if *jsonOut {
			examplesOut = errw
		}
		return runExamplesCheckCommand([]string{"--jobs=6"}, examplesOut, errw)
	})

	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia check: write json: %v\n", err)
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

func checkToolingPath(path string) string {
	root, ok := currentLeiaRepoRoot()
	if !ok {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	if filepath.Clean(abs) != root {
		return path
	}
	smoke := filepath.Join(root, "tests", "smoke", "01_basic.leia")
	if _, err := os.Stat(smoke); err != nil {
		return path
	}
	return smoke
}

func currentLeiaRepoRoot() (string, bool) {
	root, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		goMod := filepath.Join(root, "go.mod")
		if data, err := os.ReadFile(goMod); err == nil {
			if bytes.Contains(data, []byte("module github.com/never-labs/leia\n")) {
				return root, true
			}
			return "", false
		}
		parent := filepath.Dir(root)
		if parent == root {
			return "", false
		}
		root = parent
	}
}

func runManifestCheck(outw, errw io.Writer) int {
	return runManifestCheckRoots([]string{"tests", "benchmarks"}, outw, errw)
}

func runManifestCheckRoots(roots []string, outw, errw io.Writer) int {
	script, err := findScriptFromCWD(filepath.Join("tests", "manifest.py"))
	if err != nil {
		fmt.Fprintf(errw, "leia check: %v\n", err)
		return 1
	}
	python := os.Getenv("LEIA_CHECK_PYTHON")
	if python == "" {
		python = "python3"
	}
	cmdArgs := append([]string{script, "check"}, roots...)
	cmd := checkExecCommand(python, cmdArgs...)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	return runExternalCommand(cmd, "leia check", errw)
}

func runDocsCheck(outw, errw io.Writer) int {
	script, err := findScriptFromCWD(filepath.Join("scripts", "docs_check.sh"))
	if err != nil {
		fmt.Fprintf(errw, "leia check: %v\n", err)
		return 1
	}
	cmd := checkExecCommand("bash", script)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	return runExternalCommand(cmd, "leia check", errw)
}

func runEditorCheck(outw, errw io.Writer) int {
	script, err := findScriptFromCWD(filepath.Join("scripts", "editor_check.sh"))
	if err != nil {
		fmt.Fprintf(errw, "leia check: %v\n", err)
		return 1
	}
	cmd := checkExecCommand("bash", script)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	return runExternalCommand(cmd, "leia check", errw)
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
