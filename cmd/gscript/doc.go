package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var docExecCommand = exec.Command

func runDocCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(errw, "usage: gscript doc [generate|check] [flags]")
		return 2
	}
	switch args[0] {
	case "generate":
		return runDocGenerateCommand(args[1:], outw, errw)
	case "check":
		return runDocCheckCommand(args[1:], outw, errw)
	case "help", "-h", "--help":
		fmt.Fprintln(outw, "usage: gscript doc [generate|check] [flags]")
		return 0
	default:
		fmt.Fprintf(errw, "gscript doc: unknown mode %q (want generate or check)\n", args[0])
		return 2
	}
}

func runDocGenerateCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("doc generate", flag.ContinueOnError)
	fs.SetOutput(errw)
	outputDir := fs.String("output", "", "write generated docs to a directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript doc generate [--output DIR]")
		return 2
	}

	cliDoc := generateCLIReferenceMarkdown()
	stdlibDoc := generateStdlibInventoryMarkdown()
	if *outputDir == "" {
		if _, err := outw.Write(cliDoc); err != nil {
			fmt.Fprintf(errw, "gscript doc generate: %v\n", err)
			return 1
		}
		if _, err := outw.Write([]byte("\n")); err != nil {
			fmt.Fprintf(errw, "gscript doc generate: %v\n", err)
			return 1
		}
		if _, err := outw.Write(stdlibDoc); err != nil {
			fmt.Fprintf(errw, "gscript doc generate: %v\n", err)
			return 1
		}
		return 0
	}
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(errw, "gscript doc generate: %v\n", err)
		return 1
	}
	files := map[string][]byte{
		"cli.md":    cliDoc,
		"stdlib.md": stdlibDoc,
	}
	for name, content := range files {
		path := filepath.Join(*outputDir, name)
		if err := os.WriteFile(path, content, 0644); err != nil {
			fmt.Fprintf(errw, "gscript doc generate: %v\n", err)
			return 1
		}
		fmt.Fprintln(outw, path)
	}
	return 0
}

func runDocCheckCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("doc check", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript doc check")
		return 2
	}
	script, err := findScriptFromCWD(filepath.Join("scripts", "docs_check.sh"))
	if err != nil {
		fmt.Fprintf(errw, "gscript doc check: %v\n", err)
		return 1
	}
	cmd := docExecCommand("bash", script)
	cmd.Stdout = outw
	cmd.Stderr = errw
	cmd.Dir = filepath.Dir(filepath.Dir(script))
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(errw, "gscript doc check: %v\n", err)
		return 1
	}
	return 0
}

func generateCLIReferenceMarkdown() []byte {
	topics := cliHelpTopics()
	var b bytes.Buffer
	fmt.Fprintln(&b, "# GScript CLI Reference")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Generated from the current `gscript` binary capabilities.")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Command | Summary |")
	fmt.Fprintln(&b, "|---|---|")
	for _, command := range cliCommandNames() {
		fmt.Fprintf(&b, "| `%s` | %s |\n", command, topics[command].Summary)
	}
	return b.Bytes()
}

func generateStdlibInventoryMarkdown() []byte {
	caps := buildCapabilities()
	var b bytes.Buffer
	fmt.Fprintln(&b, "# GScript Standard Library Inventory")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "Generated from the current runtime stdlib registry.")
	fmt.Fprintln(&b)
	for _, module := range caps.StdlibModules {
		fmt.Fprintf(&b, "- `%s`\n", strings.TrimSpace(module))
	}
	return b.Bytes()
}
