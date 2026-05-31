package main

import (
	"os"
	goruntime "runtime"
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

	os.Exit(runLegacyCommand(os.Stdout, os.Stderr))
}
