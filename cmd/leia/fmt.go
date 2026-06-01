package main

import (
	"bytes"
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
	stdinFileName := fs.String("stdin-file-name", "", "read source from stdin and use this filename for diagnostics")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	paths := fs.Args()
	if len(paths) == 0 && *stdinFileName == "" {
		fmt.Fprintln(errw, "usage: leia fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]")
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
		return runFmtStdin(*stdinFileName, *check, outw, errw)
	}

	writeFiles := *write || !*check
	ok := true
	for _, path := range paths {
		files, err := toolsource.Files(path)
		if err != nil {
			fmt.Fprintf(errw, "%s: %v\n", path, err)
			ok = false
			continue
		}
		for _, filename := range files {
			changed, err := formatFile(filename, writeFiles)
			if err != nil {
				fmt.Fprintf(errw, "%s: %v\n", filename, err)
				ok = false
				continue
			}
			if *check && changed {
				fmt.Fprintf(errw, "%s: not formatted\n", filename)
				ok = false
			}
			if writeFiles && changed {
				fmt.Fprintln(outw, filename)
			}
		}
	}
	if !ok {
		return 1
	}
	return 0
}

func runFmtStdin(filename string, check bool, outw, errw io.Writer) int {
	src, err := io.ReadAll(cliStdin)
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	if check {
		if !bytes.Equal(src, formatted) {
			fmt.Fprintf(errw, "%s: not formatted\n", filename)
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
