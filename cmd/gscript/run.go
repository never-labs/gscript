package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/gscript/gscript/internal/runtime"
)

func runRunCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(errw)
	useVM := fs.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := fs.Bool("jit", true, "use bytecode VM with JIT compilation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(errw, "usage: gscript run [--vm] [--jit=true|false] <file.gs> [args...]")
		return 2
	}

	vmExplicit := flagWasSet(fs, "vm")
	jitExplicit := flagWasSet(fs, "jit")
	if vmExplicit && !jitExplicit {
		*useJIT = false
	}
	if *useJIT {
		*useVM = true
	}

	filename := paths[0]
	interp := runtime.New()
	if err := runScriptFile(interp, filename, paths[1:], cliRunOptions{UseVM: *useVM, UseJIT: *useJIT}); err != nil {
		if exit, ok := processExit(err); ok {
			return exit.Code
		}
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	_ = outw
	return 0
}
