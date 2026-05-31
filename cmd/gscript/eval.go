package main

import (
	"flag"
	"fmt"
	"io"

	gscript "github.com/never-labs/gscript/gscript"
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

	resolveVMJITFlags(fs, useVM, useJIT)

	vm := gscript.New(publicRunOptions(cliRunOptions{UseVM: *useVM, UseJIT: *useJIT}, "<eval>", rest[1:])...)
	prog, err := gscript.Compile(rest[0], gscript.WithSourceName("<eval>"))
	if err != nil {
		if code, ok := processExitCode(err); ok {
			return code
		}
		fmt.Fprintf(errw, "error: %v\n", err)
		return 1
	}
	if err := vm.Run(prog); err != nil {
		if code, ok := processExitCode(err); ok {
			return code
		}
		fmt.Fprintf(errw, "error: %v\n", err)
		return 1
	}
	_ = outw
	return 0
}
