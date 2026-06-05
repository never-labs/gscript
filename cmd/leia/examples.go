package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type cliExample struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Section  string `json:"section"`
	Path     string `json:"path"`
	Runnable bool   `json:"runnable"`
	Requires string `json:"requires,omitempty"`
}

type cliExampleCheckResult struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	Duration string `json:"duration,omitempty"`
	Requires string `json:"requires,omitempty"`
	Error    string `json:"error,omitempty"`
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
	case "check":
		return runExamplesCheckCommand(args[1:], outw, errw)
	case "run":
		return runExamplesRunCommand(args[1:], outw, errw)
	case "show":
		return runExamplesShowCommand(args[1:], outw, errw)
	case "-h", "--help", "help":
		fmt.Fprintln(errw, "usage: leia examples [list|show|run|check] [--json] [ID-or-path]")
		return 0
	default:
		fmt.Fprintf(errw, "leia examples: unknown subcommand %q\n", args[0])
		fmt.Fprintln(errw, "usage: leia examples [list|show|run|check] [--json] [ID-or-path]")
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

func runExamplesCheckCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("examples check", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print check results as JSON")
	jobs := fs.Int("jobs", 1, "number of runnable examples to check in parallel")
	maxSteps := fs.Int64("max-steps", 2_000_000, "maximum VM or interpreter steps per example")
	timeout := fs.Duration("timeout", 30*time.Second, "wall-clock timeout per runnable example")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if *jobs < 1 {
		fmt.Fprintln(errw, "leia examples check: --jobs must be >= 1")
		return 2
	}
	if *maxSteps < 1 {
		fmt.Fprintln(errw, "leia examples check: --max-steps must be >= 1")
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(errw, "leia examples check: --timeout must be positive")
		return 2
	}
	selectors := fs.Args()
	examples, err := selectedCLIExamples(selectors)
	if err != nil {
		fmt.Fprintf(errw, "leia examples: %v\n", err)
		return 1
	}
	results := checkCLIExamples(examples, *jobs, *maxSteps, *timeout)
	failed := 0
	runnable := 0
	skipped := 0
	for _, result := range results {
		switch result.Status {
		case "ok":
			runnable++
		case "skipped":
			skipped++
		default:
			failed++
		}
	}
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			SchemaVersion int                     `json:"schema_version"`
			OK            bool                    `json:"ok"`
			Runnable      int                     `json:"runnable"`
			Skipped       int                     `json:"skipped"`
			Failed        int                     `json:"failed"`
			Results       []cliExampleCheckResult `json:"results"`
		}{
			SchemaVersion: 1,
			OK:            failed == 0,
			Runnable:      runnable,
			Skipped:       skipped,
			Failed:        failed,
			Results:       results,
		}); err != nil {
			fmt.Fprintf(errw, "leia examples check: write json: %v\n", err)
			return 1
		}
	} else {
		for _, result := range results {
			switch result.Status {
			case "ok":
				fmt.Fprintf(outw, "ok      %-48s %s\n", result.ID, result.Duration)
			case "skipped":
				fmt.Fprintf(outw, "skip    %-48s %s\n", result.ID, result.Requires)
			default:
				fmt.Fprintf(outw, "fail    %-48s %s\n", result.ID, result.Error)
			}
		}
		fmt.Fprintf(outw, "examples: %d ok, %d skipped, %d failed\n", runnable, skipped, failed)
	}
	if failed != 0 {
		return 1
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

func selectedCLIExamples(selectors []string) ([]cliExample, error) {
	all, err := cliRepositoryExamples()
	if err != nil {
		return nil, err
	}
	if len(selectors) == 0 {
		return all, nil
	}
	selected := make([]cliExample, 0, len(selectors))
	for _, selector := range selectors {
		example, _, ok, err := resolveCLIExample(selector)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("example %q not found", selector)
		}
		selected = append(selected, example)
	}
	return selected, nil
}

func checkCLIExamples(examples []cliExample, jobs int, maxSteps int64, timeout time.Duration) []cliExampleCheckResult {
	type indexedExample struct {
		index   int
		example cliExample
	}
	results := make([]cliExampleCheckResult, len(examples))
	work := make(chan indexedExample)
	var wg sync.WaitGroup
	workerCount := jobs
	if workerCount > len(examples) {
		workerCount = len(examples)
	}
	if workerCount < 1 {
		workerCount = 1
	}
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				results[item.index] = checkCLIExample(item.example, maxSteps, timeout)
			}
		}()
	}
	for index, example := range examples {
		work <- indexedExample{index: index, example: example}
	}
	close(work)
	wg.Wait()
	return results
}

func checkCLIExample(example cliExample, maxSteps int64, timeout time.Duration) cliExampleCheckResult {
	result := cliExampleCheckResult{
		ID:       example.ID,
		Path:     example.Path,
		Status:   "ok",
		Requires: example.Requires,
	}
	if !example.Runnable {
		result.Status = "skipped"
		return result
	}
	path := filepath.Join(filepath.Dir(playgroundExamplesRoot()), filepath.FromSlash(example.Path))
	started := time.Now()
	type runResult struct {
		code   int
		stdout string
		stderr string
	}
	done := make(chan runResult, 1)
	go func() {
		var stdout, stderr bytes.Buffer
		code := runPlaygroundExecCommand([]string{"--mode=bytecode", "--max-steps=" + fmt.Sprint(maxSteps), path}, &stdout, &stderr)
		done <- runResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
	}()
	var run runResult
	select {
	case run = <-done:
	case <-time.After(timeout):
		result.Duration = time.Since(started).Round(time.Millisecond).String()
		result.Status = "failed"
		result.Error = fmt.Sprintf("timed out after %s", timeout)
		return result
	}
	result.Duration = time.Since(started).Round(time.Millisecond).String()
	if run.code != 0 {
		result.Status = "failed"
		result.Error = strings.TrimSpace(run.stderr)
		if result.Error == "" {
			result.Error = strings.TrimSpace(run.stdout)
		}
		if result.Error == "" {
			result.Error = fmt.Sprintf("exit code %d", run.code)
		}
	}
	return result
}

func boolFlagString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
