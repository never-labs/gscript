package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
)

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
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: gscript check [--json] [--no-fmt] [--no-lint] [--no-test] <path-or-dir>")
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
