package main

import (
	"flag"
	"fmt"
	"io"

	gscript "github.com/never-labs/gscript"
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

	resolveVMJITFlags(fs, useVM, useJIT)

	filename := paths[0]
	if err := runPublicScriptFile(filename, paths[1:], cliRunOptions{UseVM: *useVM, UseJIT: *useJIT}); err != nil {
		if code, ok := processExitCode(err); ok {
			return code
		}
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	_ = outw
	return 0
}

func runPublicScriptFile(filename string, args []string, opts cliRunOptions) error {
	vm := gscript.New(publicRunOptions(opts, filename, args)...)
	return vm.ExecFile(filename)
}

func canUsePublicRunPath(opts cliRunOptions) bool {
	return !opts.ShowJITStats &&
		opts.JIT.TimelinePath == "" &&
		opts.JIT.WarmDumpDir == "" &&
		!opts.JIT.ShowExitStats &&
		!opts.JIT.ShowExitStatsJSON &&
		!opts.JIT.ShowTier2PerfStats &&
		!opts.JIT.ShowTier2PerfStatsJSON &&
		!opts.JIT.ShowTier2SpecStateJSON &&
		!opts.JIT.ShowTier2SpecWorklistJSON &&
		!opts.JIT.ShowCoroutineStats &&
		!opts.JIT.ShowPathStats &&
		!opts.JIT.ShowPathStatsJSON
}

func publicRunOptions(opts cliRunOptions, script string, args []string) []gscript.Option {
	gsOpts := []gscript.Option{gscript.WithArgs(script, args...)}
	if opts.UseJIT {
		gsOpts = append(gsOpts, gscript.WithJIT())
	} else if opts.UseVM {
		gsOpts = append(gsOpts, gscript.WithVM())
	}
	return gsOpts
}
