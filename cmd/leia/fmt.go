package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	toolsource "github.com/never-labs/leia/internal/support/source"
	toolformat "github.com/never-labs/leia/internal/tooling/format"
)

func runFmtCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(errw)
	check := fs.Bool("check", false, "check whether files are formatted without writing")
	write := fs.Bool("write", false, "write formatted files in place")
	jsonOut := fs.Bool("json", false, "print a machine-readable formatter report")
	stdinFileName := fs.String("stdin-file-name", "", "read source from stdin and use this filename for diagnostics")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	paths := fs.Args()
	if len(paths) == 0 && *stdinFileName == "" {
		fmt.Fprintln(errw, fmtUsage)
		return 2
	}
	if *check && *write {
		fmt.Fprintln(errw, "leia fmt: --check and --write are mutually exclusive")
		return 2
	}
	if *stdinFileName != "" {
		if len(paths) != 0 {
			fmt.Fprintln(errw, "leia fmt: --stdin-file-name cannot be used with path arguments")
			return 2
		}
		if *write {
			fmt.Fprintln(errw, "leia fmt: --stdin-file-name cannot be used with --write")
			return 2
		}
		if *jsonOut && !*check {
			fmt.Fprintln(errw, "leia fmt: --json with --stdin-file-name requires --check")
			return 2
		}
		return runFmtStdin(*stdinFileName, *check, *jsonOut, outw, errw)
	}

	writeFiles := *write || !*check
	report := fmtReport{SchemaVersion: 1, OK: true, Mode: fmtMode(*check, writeFiles)}
	ok := true
	for _, path := range paths {
		files, err := toolsource.Files(path)
		if err != nil {
			if !*jsonOut {
				fmt.Fprintf(errw, "%s: %v\n", path, err)
			}
			report.OK = false
			report.ErrorCount++
			report.Files = append(report.Files, fmtFileReport{Path: path, Error: err.Error()})
			ok = false
			continue
		}
		for _, filename := range files {
			changed, err := formatFile(filename, writeFiles)
			item := fmtFileReport{Path: filename, Changed: changed, Written: writeFiles && changed}
			if err != nil {
				if !*jsonOut {
					fmt.Fprintf(errw, "%s: %v\n", filename, err)
				}
				report.OK = false
				report.ErrorCount++
				item.Error = err.Error()
				report.Files = append(report.Files, item)
				ok = false
				continue
			}
			report.FileCount++
			if changed {
				report.ChangedCount++
			}
			report.Files = append(report.Files, item)
			if *check && changed {
				if !*jsonOut {
					fmt.Fprintf(errw, "%s: not formatted\n", filename)
				}
				report.OK = false
				ok = false
			}
			if writeFiles && changed && !*jsonOut {
				fmt.Fprintln(outw, filename)
			}
		}
	}
	if *jsonOut {
		if err := writeFmtReport(outw, report); err != nil {
			fmt.Fprintf(errw, "leia fmt: write json: %v\n", err)
			return 1
		}
	}
	if !ok {
		return 1
	}
	return 0
}

const fmtUsage = "usage: leia fmt [--check] [--write] [--json] [--stdin-file-name FILE] <path-or-dir> [...]"

type fmtReport struct {
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Mode          string          `json:"mode"`
	Stdin         bool            `json:"stdin"`
	FileCount     int             `json:"file_count"`
	ChangedCount  int             `json:"changed_count"`
	ErrorCount    int             `json:"error_count"`
	Files         []fmtFileReport `json:"files"`
}

type fmtFileReport struct {
	Path    string `json:"path"`
	Changed bool   `json:"changed"`
	Written bool   `json:"written"`
	Error   string `json:"error,omitempty"`
}

func fmtMode(check, writeFiles bool) string {
	if check {
		return "check"
	}
	if writeFiles {
		return "write"
	}
	return "format"
}

func writeFmtReport(w io.Writer, report fmtReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func runFmtStdin(filename string, check bool, jsonOut bool, outw, errw io.Writer) int {
	src, err := io.ReadAll(cliStdin)
	if err != nil {
		if !jsonOut {
			fmt.Fprintf(errw, "%s: %v\n", filename, err)
		}
		if jsonOut {
			_ = writeFmtReport(outw, fmtReport{
				SchemaVersion: 1,
				OK:            false,
				Mode:          "check",
				Stdin:         true,
				ErrorCount:    1,
				Files:         []fmtFileReport{{Path: filename, Error: err.Error()}},
			})
		}
		return 1
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		if !jsonOut {
			fmt.Fprintf(errw, "%s: %v\n", filename, err)
		}
		if jsonOut {
			_ = writeFmtReport(outw, fmtReport{
				SchemaVersion: 1,
				OK:            false,
				Mode:          "check",
				Stdin:         true,
				ErrorCount:    1,
				Files:         []fmtFileReport{{Path: filename, Error: err.Error()}},
			})
		}
		return 1
	}
	if check {
		changed := !bytes.Equal(src, formatted)
		if jsonOut {
			report := fmtReport{
				SchemaVersion: 1,
				OK:            !changed,
				Mode:          "check",
				Stdin:         true,
				FileCount:     1,
				Files:         []fmtFileReport{{Path: filename, Changed: changed}},
			}
			if changed {
				report.ChangedCount = 1
			}
			if err := writeFmtReport(outw, report); err != nil {
				fmt.Fprintf(errw, "leia fmt: write json: %v\n", err)
				return 1
			}
		}
		if changed {
			if !jsonOut {
				fmt.Fprintf(errw, "%s: not formatted\n", filename)
			}
			return 1
		}
		return 0
	}
	if _, err := outw.Write(formatted); err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	return 0
}

func formatFile(filename string, write bool) (bool, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		return false, err
	}
	changed := !bytes.Equal(src, formatted)
	if write && changed {
		if err := os.WriteFile(filename, formatted, 0644); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func formatSource(filename string, src []byte) ([]byte, error) {
	return toolformat.Source(filename, src)
}
