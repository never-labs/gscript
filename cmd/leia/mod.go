package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/never-labs/leia/internal/modpkg"
)

type modGraphReport = modpkg.GraphReport
type modVerifyReport = modpkg.VerifyReport
type modTidyReport = modpkg.TidyReport
type modExplainReport = modpkg.ExplainReport
type modListReport = modpkg.ListReport
type modCapabilityReport = modpkg.CapabilityReport
type modDownloadReport = modpkg.DownloadReport
type modVendorReport = modpkg.VendorReport
type modGoModReport = modpkg.GoModReport

func runModCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia mod [init|add|tidy|check|download|vendor|lock|list|graph|explain|capability|gomod|verify] [flags]")
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
	case "download":
		return runModDownloadCommand(args[1:], outw, errw)
	case "vendor":
		return runModVendorCommand(args[1:], outw, errw)
	case "lock":
		return runModLockCommand(args[1:], outw, errw)
	case "list":
		return runModListCommand(args[1:], outw, errw)
	case "graph":
		return runModGraphCommand(args[1:], outw, errw)
	case "explain":
		return runModExplainCommand(args[1:], outw, errw)
	case "capability", "capabilities", "cap":
		return runModCapabilityCommand(args[1:], outw, errw)
	case "gomod", "go-mod":
		return runModGoModCommand(args[1:], outw, errw)
	case "verify":
		return runModVerifyCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: leia mod [init|add|tidy|check|download|vendor|lock|list|graph|explain|capability|gomod|verify] [flags]")
		return 0
	default:
		fmt.Fprintf(errw, "leia mod: unknown mode %q (want init, add, tidy, check, download, vendor, lock, list, graph, explain, capability, gomod, or verify)\n", args[0])
		return 2
	}
}

func runModInitCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod init", flag.ContinueOnError)
	fs.SetOutput(errw)
	module := fs.String("module", "", "module path")
	dir := fs.String("dir", ".", "project directory")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia mod init [--module PATH] [--dir DIR]")
		return 2
	}
	path, err := modpkg.Init(modpkg.InitOptions{Module: *module, Dir: *dir})
	if err != nil {
		fmt.Fprintf(errw, "leia mod init: %v\n", err)
		return 1
	}
	fmt.Fprintln(outw, path)
	return 0
}

func runModAddCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod add", flag.ContinueOnError)
	fs.SetOutput(errw)
	dir := fs.String("dir", ".", "project directory")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	targets := fs.Args()
	if len(targets) == 0 {
		fmt.Fprintln(errw, "usage: leia mod add [--dir DIR] PATH@VERSION [...]")
		return 2
	}
	path, err := modpkg.AddRequirements(*dir, targets)
	if err != nil {
		fmt.Fprintf(errw, "leia mod add: %v\n", err)
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
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia mod tidy [--json] [--dir DIR]")
		return 2
	}
	report := modpkg.Tidy(*dir)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod tidy: %v\n", err)
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
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod lock [--json] [path]")
		return 2
	}
	report := modpkg.Lock(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod lock: %v\n", err)
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

func runModDownloadCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod download", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print download report as JSON")
	cacheDir := fs.String("cache", "", "module cache directory")
	githubBase := fs.String("github-base", "", "GitHub-compatible base URL")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod download [--json] [--cache DIR] [--github-base URL] [path]")
		return 2
	}
	report := modpkg.Download(path, modpkg.DownloadOptions{CacheDir: *cacheDir, GitHubBaseURL: *githubBase})
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod download: %v\n", err)
			return 1
		}
	} else {
		if report.CacheDir != "" {
			fmt.Fprintf(outw, "cache: %s\n", report.CacheDir)
		}
		for _, module := range report.Modules {
			status := "cached"
			if module.Downloaded {
				status = "downloaded"
			}
			extractStatus := "already extracted"
			if module.Extracted {
				extractStatus = "extracted"
			}
			fmt.Fprintf(outw, "%s %s@%s -> %s (%s)\n", status, module.Path, module.Version, module.ExtractDir, extractStatus)
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

func runModVendorCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod vendor", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print vendor report as JSON")
	cacheDir := fs.String("cache", "", "module cache directory")
	vendorDir := fs.String("vendor-dir", "", "vendor directory")
	clear := fs.Bool("clear", false, "remove vendor directory before copying modules")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod vendor [--json] [--cache DIR] [--vendor-dir DIR] [--clear] [path]")
		return 2
	}
	report := modpkg.Vendor(path, modpkg.VendorOptions{CacheDir: *cacheDir, VendorDir: *vendorDir, Clear: *clear})
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod vendor: %v\n", err)
			return 1
		}
	} else {
		if report.VendorDir != "" {
			fmt.Fprintf(outw, "vendor: %s\n", report.VendorDir)
		}
		for _, module := range report.Modules {
			fmt.Fprintf(outw, "copied %s@%s -> %s\n", module.Path, module.Version, module.Target)
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
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod graph [--json] [path]")
		return 2
	}
	report, err := modpkg.Graph(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if encErr := enc.Encode(report); encErr != nil {
			fmt.Fprintf(errw, "leia mod graph: %v\n", encErr)
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

func runModListCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod list", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print module list as JSON")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod list [--json] [path]")
		return 2
	}
	report := modpkg.List(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod list: %v\n", err)
			return 1
		}
	} else {
		if report.Module != "" {
			fmt.Fprintf(outw, "module %s\n", report.Module)
		}
		if report.Leia != "" {
			fmt.Fprintf(outw, "leia %s\n", report.Leia)
		}
		for _, req := range report.Requires {
			fmt.Fprintf(outw, "require %s %s", req.Path, req.Version)
			if req.Kind != "" {
				fmt.Fprintf(outw, " (%s", req.Kind)
				if req.Source != "" {
					fmt.Fprintf(outw, ":%s", req.Source)
				}
				fmt.Fprint(outw, ")")
			}
			fmt.Fprintln(outw)
		}
		for _, rep := range report.Replaces {
			version := ""
			if rep.Version != "" {
				version = " " + rep.Version
			}
			fmt.Fprintf(outw, "replace %s%s => %s\n", rep.Path, version, rep.NewPath)
		}
		for _, col := range report.Collections {
			fmt.Fprintf(outw, "collection %s %s\n", col.Name, col.Path)
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

func runModExplainCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod explain", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print resolution as JSON")
	dir := fs.String("dir", ".", "project directory")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintln(errw, "usage: leia mod explain [--json] [--dir DIR] MODULE")
		return 2
	}
	report := modpkg.Explain(*dir, fs.Args()[0])
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod explain: %v\n", err)
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

func runModCapabilityCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod capability", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print capability matrix as JSON")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod capability [--json] [path]")
		return 2
	}
	report := modpkg.Capability(path)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod capability: %v\n", err)
			return 1
		}
	} else {
		for _, module := range report.Modules {
			version := ""
			if module.Version != "" {
				version = "@" + module.Version
			}
			fmt.Fprintf(outw, "%s%s (%s)", module.Path, version, module.Kind)
			for _, cap := range module.Capabilities {
				fmt.Fprintf(outw, " %s", cap)
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

func runModGoModCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("mod gomod", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print generated go.mod report as JSON")
	write := fs.Bool("write", false, "write generated go.mod next to leia.mod")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod gomod [--json] [--write] [path]")
		return 2
	}
	report := modpkg.GenerateGoMod(path, *write)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod gomod: %v\n", err)
			return 1
		}
	} else {
		if report.OK && !*write {
			fmt.Fprint(outw, report.Content)
		}
		if report.OK && *write {
			fmt.Fprintf(outw, "wrote %s\n", report.GoMod)
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
	cacheDir := fs.String("cache", "", "module cache directory")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	path := "."
	if len(fs.Args()) == 1 {
		path = fs.Args()[0]
	} else if len(fs.Args()) > 1 {
		fmt.Fprintln(errw, "usage: leia mod verify [--json] [--cache DIR] [path]")
		return 2
	}
	report := modpkg.VerifyWithOptions(path, modpkg.VerifyOptions{CacheDir: *cacheDir})
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia mod verify: %v\n", err)
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
