package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/never-labs/leia/internal/tooling/lsp"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, outw, errw io.Writer) int {
	if len(args) > 0 {
		switch args[0] {
		case "help", "-h", "--help":
			fmt.Fprintln(outw, "usage: leia-lsp")
			fmt.Fprintln(outw, "Run the Leia Language Server Protocol endpoint over stdio.")
			return 0
		default:
			fmt.Fprintln(errw, "usage: leia-lsp")
			return 2
		}
	}
	if err := lsp.NewServer().Run(context.Background(), in, outw); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(errw, "leia-lsp: %v\n", err)
		return 1
	}
	return 0
}
