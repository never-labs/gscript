package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type cliEnvReport struct {
	SchemaVersion int                 `json:"schema_version"`
	Version       cliVersionReport    `json:"version"`
	Capabilities  cliCapabilities     `json:"capabilities"`
	WorkingDir    string              `json:"working_dir"`
	Project       cliEnvProjectReport `json:"project"`
	Paths         cliEnvPathsReport   `json:"paths"`
}

type cliEnvProjectReport struct {
	Found bool   `json:"found"`
	Root  string `json:"root,omitempty"`
	Path  string `json:"path,omitempty"`
	Name  string `json:"name,omitempty"`
}

type cliEnvPathsReport struct {
	UserCacheDir  string `json:"user_cache_dir,omitempty"`
	UserConfigDir string `json:"user_config_dir,omitempty"`
}

func runEnvCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print environment as JSON")
	start := fs.String("path", ".", "path used for project config discovery")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia env [--json] [--path PATH]")
		return 2
	}
	report := buildEnvReport(*start)
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia env: write json: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(outw, "version: %s\n", report.Version.Version)
	fmt.Fprintf(outw, "go: %s %s/%s\n", report.Version.GoVersion, report.Version.GOOS, report.Version.GOARCH)
	fmt.Fprintf(outw, "working dir: %s\n", report.WorkingDir)
	if report.Project.Found {
		fmt.Fprintf(outw, "project root: %s\n", report.Project.Root)
		fmt.Fprintf(outw, "project config: %s\n", report.Project.Path)
		if report.Project.Name != "" {
			fmt.Fprintf(outw, "project name: %s\n", report.Project.Name)
		}
	} else {
		fmt.Fprintln(outw, "project config: not found")
	}
	fmt.Fprintf(outw, "jit: %t\n", report.Capabilities.Execution.JIT)
	if report.Paths.UserCacheDir != "" {
		fmt.Fprintf(outw, "cache dir: %s\n", report.Paths.UserCacheDir)
	}
	if report.Paths.UserConfigDir != "" {
		fmt.Fprintf(outw, "config dir: %s\n", report.Paths.UserConfigDir)
	}
	return 0
}

func buildEnvReport(start string) cliEnvReport {
	wd, _ := os.Getwd()
	cacheDir, _ := os.UserCacheDir()
	configDir, _ := os.UserConfigDir()
	report := cliEnvReport{
		SchemaVersion: 1,
		Version:       buildVersionReport(),
		Capabilities:  buildCapabilities(),
		WorkingDir:    wd,
		Paths: cliEnvPathsReport{
			UserCacheDir:  cliEnvJoinDir(cacheDir),
			UserConfigDir: cliEnvJoinDir(configDir),
		},
	}
	if cfg, err := loadCLIProjectConfig(start); err == nil && cfg.Found {
		report.Project = cliEnvProjectReport{
			Found: true,
			Root:  cfg.Root,
			Path:  cfg.Path,
		}
		if cfg.Config != nil {
			report.Project.Name = cfg.Config.Project.Name
		}
	}
	return report
}

func cliEnvJoinDir(base string) string {
	if base == "" {
		return ""
	}
	return filepath.Join(base, "leia")
}
