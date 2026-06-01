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
	if err := lsp.NewServer().Run(context.Background(), os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "leia-lsp: %v\n", err)
		os.Exit(1)
	}
}
