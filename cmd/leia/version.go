package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	goruntime "runtime"
	"runtime/debug"
)

var cliVersion = "dev"

type cliVersionReport struct {
	SchemaVersion int    `json:"schema_version"`
	Status        string `json:"status"`
	Version       string `json:"version"`
	GoVersion     string `json:"go_version"`
	GOOS          string `json:"goos"`
	GOARCH        string `json:"goarch"`
	Revision      string `json:"revision,omitempty"`
	Modified      string `json:"modified,omitempty"`
	Time          string `json:"time,omitempty"`
}

func runVersionCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print version as JSON")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia version [--json]")
		return 2
	}
	report := buildVersionReport()
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			fmt.Fprintf(errw, "leia version: write json: %v\n", err)
			return 1
		}
		return 0
	}
	revision := report.Revision
	if revision == "" {
		revision = "unknown"
	}
	modified := ""
	if report.Modified != "" {
		modified = " modified=" + report.Modified
	}
	fmt.Fprintf(outw, "leia %s %s/%s %s revision=%s%s\n", report.Version, report.GOOS, report.GOARCH, report.GoVersion, revision, modified)
	return 0
}

func buildVersionReport() cliVersionReport {
	report := cliVersionReport{
		SchemaVersion: 1,
		Status:        "pass",
		Version:       cliVersion,
		GoVersion:     goruntime.Version(),
		GOOS:          goruntime.GOOS,
		GOARCH:        goruntime.GOARCH,
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				report.Revision = setting.Value
			case "vcs.modified":
				report.Modified = setting.Value
			case "vcs.time":
				report.Time = setting.Value
			}
		}
	}
	return report
}
