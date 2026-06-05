package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type cliExample struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Section  string `json:"section"`
	Path     string `json:"path"`
	Runnable bool   `json:"runnable"`
	Requires string `json:"requires,omitempty"`
}

func runExamplesCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		return runExamplesListCommand(nil, outw, errw)
	}
	if strings.HasPrefix(args[0], "-") {
		return runExamplesListCommand(args, outw, errw)
	}
	switch args[0] {
	case "list":
		return runExamplesListCommand(args[1:], outw, errw)
	case "run":
		return runExamplesRunCommand(args[1:], outw, errw)
	case "show":
		return runExamplesShowCommand(args[1:], outw, errw)
	case "-h", "--help", "help":
		fmt.Fprintln(errw, "usage: leia examples [list|run|show] [--json] [ID-or-path]")
		return 0
	default:
		fmt.Fprintf(errw, "leia examples: unknown subcommand %q\n", args[0])
		fmt.Fprintln(errw, "usage: leia examples [list|run|show] [--json] [ID-or-path]")
		return 2
	}
}

func runExamplesListCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("examples list", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print examples as JSON")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia examples list [--json]")
		return 2
	}
	examples, err := cliRepositoryExamples()
	if err != nil {
		fmt.Fprintf(errw, "leia examples: %v\n", err)
		return 1
	}
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			SchemaVersion int          `json:"schema_version"`
			Examples      []cliExample `json:"examples"`
		}{SchemaVersion: 1, Examples: examples}); err != nil {
			fmt.Fprintf(errw, "leia examples: write json: %v\n", err)
			return 1
		}
		return 0
	}
	for _, example := range examples {
		status := "run"
		if !example.Runnable {
			status = "manual"
		}
		if example.Requires != "" {
			fmt.Fprintf(outw, "%-48s %-8s %-18s %s (%s)\n", example.ID, status, example.Section, example.Path, example.Requires)
		} else {
			fmt.Fprintf(outw, "%-48s %-8s %-18s %s\n", example.ID, status, example.Section, example.Path)
		}
	}
	return 0
}

func runExamplesShowCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("examples show", flag.ContinueOnError)
	fs.SetOutput(errw)
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintln(errw, "usage: leia examples show ID-or-path")
		return 2
	}
	example, path, ok, err := resolveCLIExample(fs.Args()[0])
	if err != nil {
		fmt.Fprintf(errw, "leia examples: %v\n", err)
		return 1
	}
	if !ok {
		fmt.Fprintf(errw, "leia examples: example %q not found\n", fs.Args()[0])
		return 1
	}
	fmt.Fprintf(outw, "id: %s\n", example.ID)
	fmt.Fprintf(outw, "title: %s\n", example.Title)
	fmt.Fprintf(outw, "section: %s\n", example.Section)
	fmt.Fprintf(outw, "path: %s\n", example.Path)
	fmt.Fprintf(outw, "runnable: %t\n", example.Runnable)
	if example.Requires != "" {
		fmt.Fprintf(outw, "requires: %s\n", example.Requires)
	}
	fmt.Fprintln(outw)
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(errw, "leia examples: read %s: %v\n", path, err)
		return 1
	}
	_, _ = outw.Write(src)
	return 0
}

func runExamplesRunCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("examples run", flag.ContinueOnError)
	fs.SetOutput(errw)
	useVM := fs.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := fs.Bool("jit", true, "use bytecode VM with JIT compilation")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintln(errw, "usage: leia examples run [--vm] [--jit=true|false] ID-or-path")
		return 2
	}
	example, path, ok, err := resolveCLIExample(fs.Args()[0])
	if err != nil {
		fmt.Fprintf(errw, "leia examples: %v\n", err)
		return 1
	}
	if !ok {
		fmt.Fprintf(errw, "leia examples: example %q not found\n", fs.Args()[0])
		return 1
	}
	if !example.Runnable {
		fmt.Fprintf(errw, "leia examples: %s is a manual example", example.ID)
		if example.Requires != "" {
			fmt.Fprintf(errw, " (%s)", example.Requires)
		}
		fmt.Fprintln(errw)
		return 1
	}
	resolveVMJITFlags(fs, useVM, useJIT)
	return runRunCommand([]string{"--vm=" + boolFlagString(*useVM), "--jit=" + boolFlagString(*useJIT), path}, outw, errw)
}

func cliRepositoryExamples() ([]cliExample, error) {
	playgroundExamples, err := playgroundRepositoryExamples(playgroundExamplesRoot())
	if err != nil {
		return nil, err
	}
	examples := make([]cliExample, 0, len(playgroundExamples))
	for _, example := range playgroundExamples {
		examples = append(examples, cliExample{
			ID:       example.ID,
			Title:    example.Title,
			Section:  example.Section,
			Path:     example.Summary,
			Runnable: example.Runnable,
			Requires: example.Requires,
		})
	}
	sort.Slice(examples, func(i, j int) bool {
		if examples[i].Section != examples[j].Section {
			return examples[i].Section < examples[j].Section
		}
		return examples[i].ID < examples[j].ID
	})
	return examples, nil
}

func resolveCLIExample(selector string) (cliExample, string, bool, error) {
	examples, err := cliRepositoryExamples()
	if err != nil {
		return cliExample{}, "", false, err
	}
	root := filepath.Dir(playgroundExamplesRoot())
	normalized := filepath.ToSlash(selector)
	for _, example := range examples {
		if selector == example.ID || normalized == example.Path || normalized == strings.TrimPrefix(example.Path, "examples/") {
			return example, filepath.Join(root, filepath.FromSlash(example.Path)), true, nil
		}
	}
	return cliExample{}, "", false, nil
}

func boolFlagString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
