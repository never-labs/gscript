package main

import (
	"flag"
	"fmt"
	"io"

	leia "github.com/never-labs/leia"
)

func runEvalCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("eval", flag.ContinueOnError)
	fs.SetOutput(errw)
	useVM := fs.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := fs.Bool("jit", true, "use bytecode VM with JIT compilation")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	rest := fs.Args()
	if len(rest) == 0 {
		fmt.Fprintln(errw, "usage: leia eval [--vm] [--jit=true|false] <source> [args...]")
		return 2
	}

	resolveVMJITFlags(fs, useVM, useJIT)

	vm := leia.New(publicRunOptions(cliRunOptions{UseVM: *useVM, UseJIT: *useJIT}, "<eval>", rest[1:])...)
	prog, err := leia.Compile(rest[0], leia.WithSourceName("<eval>"))
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
