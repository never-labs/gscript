package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/never-labs/gscript/internal/runtime"
)

func runREPLCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("repl", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript repl")
		return 2
	}
	interp := newCLIInterpreter()
	interp.SetArgs("<repl>", nil)
	_ = outw
	runREPL(interp)
	return 0
}

func runREPL(interp *runtime.Interpreter) {
	fmt.Println("GScript REPL (type 'exit' to quit)")
	scanner := bufio.NewScanner(os.Stdin)
	buf := ""

	for {
		if buf == "" {
			fmt.Print("> ")
		} else {
			fmt.Print(">> ")
		}

		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if line == "exit" || line == "quit" {
			break
		}

		buf += line + "\n"

		// Try to execute
		err := runString(interp, buf)
		if err != nil {
			// Show error and reset buffer
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		buf = ""
	}
}
