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
		if os.Args[1] == "__playground_exec" {
			os.Exit(runPlaygroundExecCommand(os.Args[2:], os.Stdout, os.Stderr))
		}
		if spec, ok := lookupCLICommand(os.Args[1]); ok {
			os.Exit(spec.Run(os.Args[2:], os.Stdout, os.Stderr))
		}
	}

	os.Exit(runDefaultCommand(os.Stdout, os.Stderr))
}
