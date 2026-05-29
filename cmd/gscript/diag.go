package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

var diagExecCommand = exec.Command

func runDiagCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: gscript diag [dump|bundle] [diagnostic-flags...]")
		return 2
	}
	mode := args[0]
	scriptArgs := args[1:]
	if mode == "help" || mode == "-h" || mode == "--help" {
		fmt.Fprintln(outw, "usage: gscript diag [dump|bundle] [diagnostic-flags...]")
		return 0
	}

	script, err := diagScriptForMode(mode)
	if err != nil {
		fmt.Fprintf(errw, "gscript diag: %v\n", err)
		return 2
	}
	path, err := findScriptFromCWD(script)
	if err != nil {
		fmt.Fprintf(errw, "gscript diag: %v\n", err)
		return 1
	}
	cmd := diagExecCommand("bash", append([]string{path}, scriptArgs...)...)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(path))
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(errw, "gscript diag: %v\n", err)
		return 1
	}
	return 0
}

func diagScriptForMode(mode string) (string, error) {
	switch mode {
	case "dump":
		return filepath.Join("scripts", "diag.sh"), nil
	case "bundle":
		return filepath.Join("scripts", "diagnostics_bundle.sh"), nil
	default:
		return "", fmt.Errorf("unknown diag mode %q (want dump or bundle)", mode)
	}
}
