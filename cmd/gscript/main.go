package main

import (
	"flag"
	"fmt"
	"os"
	goruntime "runtime"
	"runtime/pprof"

	gscript "github.com/never-labs/gscript/gscript"
	"github.com/never-labs/gscript/internal/runtime"
)

func init() {
	goruntime.LockOSThread() // Required for GLFW/OpenGL on macOS
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bench":
			os.Exit(runBenchCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "capabilities":
			os.Exit(runCapabilitiesCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "check":
			os.Exit(runCheckCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "ci":
			os.Exit(runCICommand(os.Args[2:], os.Stdout, os.Stderr))
		case "config":
			os.Exit(runConfigCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "diag":
			os.Exit(runDiagCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "diagnose":
			os.Exit(runDiagnoseCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "doc":
			os.Exit(runDocCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "eval":
			os.Exit(runEvalCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "env":
			os.Exit(runEnvCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "fmt":
			os.Exit(runFmtCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "help":
			os.Exit(runHelpCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "inspect":
			os.Exit(runInspectCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "lint":
			os.Exit(runLintCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "mod":
			os.Exit(runModCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "repl":
			os.Exit(runREPLCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "run":
			os.Exit(runRunCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "version":
			os.Exit(runVersionCommand(os.Args[2:], os.Stdout, os.Stderr))
		}
	}

	// Flags
	eval := flag.String("e", "", "execute string")
	useVM := flag.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := flag.Bool("jit", true, "use bytecode VM with JIT compilation (default)")
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to file")
	memprofile := flag.String("memprofile", "", "write memory profile to file")
	jitStats := flag.Bool("jit-stats", false, "print JIT tier statistics after execution")
	jitTimeline := flag.String("jit-timeline", "", "write production JIT event timeline to file ('-' for stderr)")
	jitTimelineFormat := flag.String("jit-timeline-format", "jsonl", "JIT timeline format: jsonl or json")
	jitDumpWarm := flag.String("jit-dump-warm", "", "write warm production Tier 2 diagnostic dump to directory")
	jitDumpProto := flag.String("jit-dump-proto", "", "limit -jit-dump-warm to a proto name")
	exitStats := flag.Bool("exit-stats", false, "print Tier 2 exit/deopt profile after execution")
	exitStatsJSON := flag.Bool("exit-stats-json", false, "print Tier 2 exit/deopt profile as JSON after execution")
	tier2PerfStats := flag.Bool("tier2-perf-stats", false, "print opt-in Tier 2 protocol/timing diagnostics after execution")
	tier2PerfStatsJSON := flag.Bool("tier2-perf-stats-json", false, "print opt-in Tier 2 protocol/timing diagnostics as JSON after execution")
	tier2SpecStateJSON := flag.Bool("tier2-spec-state-json", false, "print Tier 2 specialization/version state as JSON after execution")
	tier2SpecWorklistJSON := flag.Bool("tier2-spec-worklist-json", false, "print prioritized Tier 2 self-driving worklist as JSON after execution")
	jitOpAudit := flag.Bool("jit-op-audit", false, "print MethodJIT op audit matrix and exit")
	jitOpAuditJSON := flag.Bool("jit-op-audit-json", false, "print MethodJIT op audit matrix as JSON and exit")
	coroutineStats := flag.Bool("coroutine-stats", false, "print VM coroutine runtime statistics after execution")
	pathStats := flag.Bool("runtime-path-stats", false, "print runtime path counters after execution")
	pathStatsJSON := flag.Bool("runtime-path-stats-json", false, "print runtime path counters as JSON after execution")
	flag.Parse()

	if *jitOpAudit {
		if err := cliPrintMethodJITOpAudit(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *jitOpAuditJSON {
		if err := cliPrintMethodJITOpAuditJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not create memory profile: %v\n", err)
				return
			}
			defer f.Close()
			goruntime.GC()
			pprof.WriteHeapProfile(f)
		}()
	}

	resolveVMJITFlags(flag.CommandLine, useVM, useJIT)

	runOpts := cliRunOptions{
		UseVM:        *useVM,
		UseJIT:       *useJIT,
		ShowJITStats: *jitStats,
		JIT: jitCLIOptions{
			TimelinePath:              *jitTimeline,
			TimelineFormat:            *jitTimelineFormat,
			WarmDumpDir:               *jitDumpWarm,
			WarmDumpProto:             *jitDumpProto,
			ShowExitStats:             *exitStats,
			ShowExitStatsJSON:         *exitStatsJSON,
			ShowTier2PerfStats:        *tier2PerfStats,
			ShowTier2PerfStatsJSON:    *tier2PerfStatsJSON,
			ShowTier2SpecStateJSON:    *tier2SpecStateJSON,
			ShowTier2SpecWorklistJSON: *tier2SpecWorklistJSON,
			ShowCoroutineStats:        *coroutineStats,
			ShowPathStats:             *pathStats,
			ShowPathStatsJSON:         *pathStatsJSON,
		},
	}

	args := flag.Args()
	if len(args) > 0 && args[0] == "test" {
		os.Exit(runTestCommand(args[1:], runOpts, os.Stdout, os.Stderr))
		return
	}

	if *eval != "" {
		if canUsePublicRunPath(runOpts) {
			vm := gscript.New(publicRunOptions(runOpts, "<eval>", flag.Args())...)
			prog, err := gscript.Compile(*eval, gscript.WithSourceName("<eval>"))
			if err == nil {
				err = vm.Run(prog)
			}
			if err != nil {
				if code, ok := processExitCode(err); ok {
					os.Exit(code)
				}
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		interp := runtime.New()
		installCLILLMProviderFactory(interp)
		interp.SetArgs("<eval>", flag.Args())
		if err := runStringVM(interp, *eval, runOpts.UseJIT, runOpts.ShowJITStats, runOpts.JIT); err != nil {
			if code, ok := processExitCode(err); ok {
				os.Exit(code)
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(args) == 0 {
		// REPL mode
		interp := runtime.New()
		installCLILLMProviderFactory(interp)
		interp.SetArgs("<repl>", nil)
		runREPL(interp)
		return
	}

	// Execute file
	filename := args[0]
	var err error
	if canUsePublicRunPath(runOpts) {
		err = runPublicScriptFile(filename, args[1:], runOpts)
	} else {
		interp := runtime.New()
		installCLILLMProviderFactory(interp)
		err = runScriptFile(interp, filename, args[1:], runOpts)
	}
	if err != nil {
		if code, ok := processExitCode(err); ok {
			os.Exit(code)
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", filename, err)
		os.Exit(1)
	}
}
