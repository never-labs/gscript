package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/never-labs/gscript/internal/modpkg"
)

type modGraphReport = modpkg.GraphReport
type modVerifyReport = modpkg.VerifyReport
type modTidyReport = modpkg.TidyReport
type modExplainReport = modpkg.ExplainReport

func runModCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: gscript mod [init|add|tidy|check|lock|graph|explain|verify] [flags]")
		return 2
	}
	switch args[0] {
	case "init":
		return runModInitCommand(args[1:], outw, errw)
	case "add":
		return runModAddCommand(args[1:], outw, errw)
	case "tidy":
		return runModTidyCommand(args[1:], outw, errw)
	case "check":
		return runModVerifyCommand(args[1:], outw, errw)
	case "lock":
		return runModLockCommand(args[1:], outw, errw)
	case "graph":
		return runModGraphCommand(args[1:], outw, errw)
	case "explain":
		return runModExplainCommand(args[1:], outw, errw)
	case "verify":
		return runModVerifyCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: gscript mod [init|add|tidy|check|lock|graph|explain|verify] [flags]")
		return 0
	default:
		fmt.Fprintf(errw, "gscript mod: unknown mode %q (want init, add, tidy, check, lock, graph, explain, or verify)\n", args[0])
		return 2
	}
}

func runModInitCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod init", flag.ContinueOnError)
	fs.SetOutput(errw)
	module := fs.String("module", "", "module path")
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript mod init [--module PATH] [--dir DIR]")
		return 2
	}
	path, err := modpkg.Init(modpkg.InitOptions{Module: *module, Dir: *dir})
	if err != nil {
		fmt.Fprintf(errw, "gscript mod init: %v\n", err)
		return 1
	}
	fmt.Fprintln(outw, path)
	return 0
}

func runModAddCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod add", flag.ContinueOnError)
	fs.SetOutput(errw)
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	targets := fs.Args()
	if len(targets) == 0 {
		fmt.Fprintln(errw, "usage: gscript mod add [--dir DIR] PATH@VERSION [...]")
		return 2
	}
	path, err := modpkg.AddRequirements(*dir, targets)
	if err != nil {
		fmt.Fprintf(errw, "gscript mod add: %v\n", err)
		return 1
	}
	fmt.Fprintln(outw, path)
	return 0
}

func runModTidyCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod tidy", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print tidy report as JSON")
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript mod tidy [--json] [--dir DIR]")
		return 2
	}
	report := modpkg.Tidy(*dir)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript mod tidy: %v\n", err)
			return 1
		}
	} else {
		for _, removed := range report.Removed {
			fmt.Fprintf(outw, "removed %s\n", removed)
		}
		for _, missing := range report.Missing {
			fmt.Fprintf(errw, "missing require %s\n", missing)
		}
		for _, diag := range report.Diagnostics {
			fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
		}
		if report.OK {
			fmt.Fprintf(outw, "ok: %s\n", report.Manifest)
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}

func runModLockCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod lock", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print lock report as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: gscript mod lock [--json] [path]")
		return 2
	}
	report := modpkg.Lock(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript mod lock: %v\n", err)
			return 1
		}
	} else {
		if report.OK {
			fmt.Fprintf(outw, "ok: %s\n", report.Sum)
		}
		for _, diag := range report.Diagnostics {
			if diag.File != "" {
				fmt.Fprintf(errw, "%s: %s %s: %s\n", diag.File, diag.Severity, diag.Code, diag.Message)
			} else {
				fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
			}
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}

func runModGraphCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod graph", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print module graph as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: gscript mod graph [--json] [path]")
		return 2
	}
	report, err := modpkg.Graph(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			fmt.Fprintf(errw, "gscript mod graph: %v\n", encErr)
			return 1
		}
		if err != nil {
			return 1
		}
		return 0
	}
	for _, file := range report.Files {
		for _, req := range file.Requires {
			fmt.Fprintf(outw, "%s -> %s\n", file.File, req)
		}
	}
	for _, diag := range report.Diagnostics {
		fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
	}
	if err != nil {
		return 1
	}
	return 0
}

func runModExplainCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod explain", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print resolution as JSON")
	dir := fs.String("dir", ".", "project directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintln(errw, "usage: gscript mod explain [--json] [--dir DIR] MODULE")
		return 2
	}
	report := modpkg.Explain(*dir, fs.Args()[0])
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript mod explain: %v\n", err)
			return 1
		}
	} else {
		if report.OK {
			fmt.Fprintf(outw, "%s -> %s", report.Module, report.Kind)
			if report.File != "" {
				fmt.Fprintf(outw, " %s", report.File)
			}
			fmt.Fprintln(outw)
		}
		for _, diag := range report.Diagnostics {
			if diag.File != "" {
				fmt.Fprintf(errw, "%s: %s %s: %s\n", diag.File, diag.Severity, diag.Code, diag.Message)
			} else {
				fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
			}
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}

func runModVerifyCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod verify", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print verification as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: gscript mod verify [--json] [path]")
		return 2
	}
	report := modpkg.Verify(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "gscript mod verify: %v\n", err)
			return 1
		}
	} else {
		if report.OK {
			fmt.Fprintf(outw, "ok: %s\n", report.Manifest)
		}
		for _, diag := range report.Diagnostics {
			if diag.File != "" {
				fmt.Fprintf(errw, "%s: %s %s: %s\n", diag.File, diag.Severity, diag.Code, diag.Message)
			} else {
				fmt.Fprintf(errw, "%s %s: %s\n", diag.Severity, diag.Code, diag.Message)
			}
		}
	}
	if !report.OK {
		return 1
	}
	return 0
}
