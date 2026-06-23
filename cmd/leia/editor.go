package main

import (
	"encoding/json"
	"fmt"
	"io"
)

func runEditorCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: leia editor [smoke]")
		return 2
	}
	switch args[0] {
	case "smoke":
		return runEditorSmokeCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: leia editor smoke")
		return 0
	default:
		fmt.Fprintf(errw, "leia editor: unknown mode %q (want smoke)\n", args[0])
		return 2
	}
}

func writeEditorJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
