package main

import (
	"flag"
	"fmt"
	"io"

	leia "github.com/never-labs/leia"
)

func runRunCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(errw)
	useVM := fs.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := fs.Bool("jit", true, "use bytecode VM with JIT compilation")
	modMode := fs.String("mod", string(leia.ModuleModeMod), "module mode: readonly, vendor, or mod")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	moduleMode := leia.ModuleMode(*modMode)
	if !leia.ValidModuleMode(moduleMode) {
		fmt.Fprintf(errw, "leia run: invalid --mod=%q (want readonly, vendor, or mod)\n", *modMode)
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(errw, "usage: leia run [--vm] [--jit=true|false] [--mod=readonly|vendor|mod] <file.leia> [args...]")
		return 2
	}

	resolveVMJITFlags(fs, useVM, useJIT)

	filename := paths[0]
	if err := runPublicScriptFile(filename, paths[1:], cliRunOptions{UseVM: *useVM, UseJIT: *useJIT, ModuleMode: moduleMode}); err != nil {
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
	vm := leia.New(publicRunOptions(opts, filename, args)...)
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

func publicRunOptions(opts cliRunOptions, script string, args []string) []leia.Option {
	leiaOpts := append([]leia.Option{}, leia.ModuleOptionsForScriptMode(script, opts.ModuleMode)...)
	leiaOpts = append(leiaOpts, leia.WithArgs(script, args...))
	if opts.UseJIT {
		leiaOpts = append(leiaOpts, leia.WithJIT())
	} else if opts.UseVM {
		leiaOpts = append(leiaOpts, leia.WithVM())
	}
	return leiaOpts
}
