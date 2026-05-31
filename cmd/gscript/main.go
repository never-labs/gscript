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
		if spec, ok := lookupCLICommand(os.Args[1]); ok {
			os.Exit(spec.Run(os.Args[2:], os.Stdout, os.Stderr))
		}
	}

	os.Exit(runLegacyCommand(os.Stdout, os.Stderr))
}
