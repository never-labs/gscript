package main

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
)

func runExternalCommand(cmd *exec.Cmd, context string, errw io.Writer) int {
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(errw, "%s: %v\n", context, err)
		return 1
	}
	return 0
}
