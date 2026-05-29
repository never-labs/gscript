package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/gscript/gscript/internal/runtime"
)

func runREPLCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("repl", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript repl")
		return 2
	}
	interp := runtime.New()
	interp.SetArgs("<repl>", nil)
	_ = outw
	runREPL(interp)
	return 0
}
