package main

import (
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
)

var diagExecCommand = exec.Command

func runDiagCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia diag [dump|bundle|summary] [diagnostic-flags...]")
		return 2
	}
	mode := args[0]
	scriptArgs := args[1:]
	if mode == "help" || mode == "-h" || mode == "--help" {
		fmt.Fprintln(outw, "usage: leia diag [dump|bundle|summary] [diagnostic-flags...]")
		return 0
	}
	if mode == "summary" {
		return runDiagSummaryCommand(scriptArgs, outw, errw)
	}

	script, err := diagScriptForMode(mode)
	if err != nil {
		fmt.Fprintf(errw, "leia diag: %v\n", err)
		return 2
	}
	path, err := findScriptFromCWD(script)
	if err != nil {
		fmt.Fprintf(errw, "leia diag: %v\n", err)
		return 1
	}
	cmd := diagExecCommand("bash", append([]string{path}, scriptArgs...)...)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(path))
	return runExternalCommand(cmd, "leia diag", errw)
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
