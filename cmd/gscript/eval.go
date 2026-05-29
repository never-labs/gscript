package main

import (
	"flag"
	"fmt"
	"io"

	"github.com/gscript/gscript/internal/runtime"
)

func runEvalCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(errw)
	useVM := fs.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := fs.Bool("jit", true, "use bytecode VM with JIT compilation")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(errw, "usage: gscript eval [--vm] [--jit=true|false] <source> [args...]")
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

	interp := runtime.New()
	interp.SetArgs("<eval>", rest[1:])
	var err error
	if *useVM {
		err = runStringVM(interp, rest[0], *useJIT, false, jitCLIOptions{})
	} else {
		err = runString(interp, rest[0])
	}
	if err != nil {
		if code, ok := processExitCode(err); ok {
			return code
		}
		fmt.Fprintf(errw, "error: %v\n", err)
		return 1
	}
	_ = outw
	return 0
}
