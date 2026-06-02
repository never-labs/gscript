package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/never-labs/leia/internal/tooling/lsp"
)

func runLSPCommand(args []string, outw, errw io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			fmt.Fprintln(outw, "usage: leia lsp")
			return 0
		default:
			fmt.Fprintln(errw, "usage: leia lsp")
			return 2
		}
	}
	if err := lsp.NewServer().Run(context.Background(), os.Stdin, outw); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(errw, "leia lsp: %v\n", err)
		return 1
	}
	return 0
}
