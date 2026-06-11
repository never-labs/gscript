package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/llm"
)

type cliExample struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Section   string `json:"section"`
	Path      string `json:"path"`
	Runnable  bool   `json:"runnable"`
	Checkable bool   `json:"checkable,omitempty"`
	Runner    string `json:"runner,omitempty"`
	Requires  string `json:"requires,omitempty"`
}

type cliExampleCheckResult struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Status   string `json:"status"`
	Duration string `json:"duration,omitempty"`
	Requires string `json:"requires,omitempty"`
	Error    string `json:"error,omitempty"`
}

var cliExampleTestRunnerMu sync.Mutex

func runExamplesCommand(args []string, outw, errw io.Writer) int {
	if len(args) == 0 {
		return runExamplesListCommand(nil, outw, errw)
	}
	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(errw, "usage: leia examples [list|show|run|check] [--json] [ID-or-path]")
		return 0
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
		if !example.Runnable && example.Checkable {
			status = "check"
		} else if !example.Runnable {
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
	if example.Runner != "" && example.Runner != "playground" {
		return runCLIExampleRunner(example, path, 2_000_000, outw, errw)
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
		cli := cliExample{
			ID:       example.ID,
			Title:    example.Title,
			Section:  example.Section,
			Path:     example.Summary,
			Runnable: example.Runnable,
			Runner:   "playground",
			Requires: example.Requires,
		}
		applyCLIExampleRunner(&cli)
		examples = append(examples, cli)
	}
	examples = append(examples, cliCuratedProjectExamples()...)
	sort.Slice(examples, func(i, j int) bool {
		if examples[i].Section != examples[j].Section {
			return examples[i].Section < examples[j].Section
		}
		return examples[i].ID < examples[j].ID
	})
	return examples, nil
}

func cliCuratedProjectExamples() []cliExample {
	return []cliExample{
		{
			ID:        "repo-embedding-go-doc-examples",
			Title:     "Go Embedding Doc Examples",
			Section:   "Embedding",
			Path:      "examples/embedding/embedding_test.go",
			Runnable:  true,
			Checkable: true,
			Runner:    "go-test",
		},
		{
			ID:        "repo-embedding-hot_reload_project",
			Title:     "Embedding Hot Reload Project",
			Section:   "Embedding",
			Path:      "examples/embedding/hot_reload_project/hot_reload_project_test.go",
			Runnable:  true,
			Checkable: true,
			Runner:    "go-test",
		},
	}
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
		if ok {
			selected = append(selected, example)
			continue
		}
		matches := cliExamplesInDirectory(all, selector)
		if len(matches) == 0 {
			return nil, fmt.Errorf("example %q not found", selector)
		}
		selected = append(selected, matches...)
	}
	return selected, nil
}

func cliExamplesInDirectory(examples []cliExample, selector string) []cliExample {
	dir := normalizeCLIExampleSelector(selector)
	if dir == "" {
		return nil
	}
	var matches []cliExample
	for _, example := range examples {
		path := normalizeCLIExampleSelector(example.Path)
		if strings.HasPrefix(path, dir+"/") {
			matches = append(matches, example)
		}
	}
	return matches
}

func normalizeCLIExampleSelector(selector string) string {
	normalized := filepath.ToSlash(selector)
	normalized = strings.TrimPrefix(normalized, "./")
	normalized = strings.TrimPrefix(normalized, "examples/")
	normalized = strings.Trim(normalized, "/")
	if normalized == "." {
		return ""
	}
	return normalized
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
	if !example.Runnable && !example.Checkable {
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
		if moduleRoot, ok := cliExampleModuleRoot(path); ok {
			if code := runModCommand([]string{"verify", "--json", moduleRoot}, io.Discard, &stderr); code != 0 {
				done <- runResult{code: code, stdout: stdout.String(), stderr: "module verify failed: " + strings.TrimSpace(stderr.String())}
				return
			}
		}
		code := runCLIExampleRunner(example, path, maxSteps, &stdout, &stderr)
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

func cliExampleModuleRoot(path string) (string, bool) {
	root := filepath.Dir(playgroundExamplesRoot())
	examplesRoot := filepath.Join(root, "examples")
	dir := path
	if info, err := os.Stat(dir); err == nil && !info.IsDir() {
		dir = filepath.Dir(dir)
	}
	for {
		if rel, err := filepath.Rel(examplesRoot, dir); err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
			return "", false
		}
		if info, err := os.Stat(filepath.Join(dir, "leia.mod")); err == nil && !info.IsDir() {
			return dir, true
		}
		if filepath.Clean(dir) == filepath.Clean(examplesRoot) {
			return "", false
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

func applyCLIExampleRunner(example *cliExample) {
	switch {
	case strings.Contains(example.Path, "/evaluate/"):
		if cliExampleCompanionRecordsExist(example.Path) {
			example.Runnable = true
			example.Checkable = true
			example.Runner = "evaluate-replay"
			example.Requires = ""
			return
		}
		example.Runnable = true
		example.Checkable = true
		example.Runner = "evaluate"
		example.Requires = ""
		return
	case strings.Contains(example.Path, "/ai/coding_agent_replay.leia"),
		strings.Contains(example.Path, "/ai/coding_agent_project/"),
		strings.Contains(example.Path, "/ai/tagged_agent_workflow.leia"),
		strings.Contains(example.Path, "/ai/general_agent_workflow.leia"),
		strings.Contains(example.Path, "/ai/record_replay_trace_project.leia"),
		strings.Contains(example.Path, "/workflow/support_triage_replay.leia"):
		if cliExampleCompanionRecordsExist(example.Path) {
			example.Runnable = true
			example.Checkable = true
			example.Runner = "llm-replay"
			example.Requires = ""
			return
		}
		example.Runnable = true
		example.Checkable = true
		example.Runner = "llm-mock"
		example.Requires = ""
		return
	case strings.Contains(example.Path, "/macos/package_managed/"),
		strings.Contains(example.Path, "/tooling/package_manager_workflow/"),
		strings.Contains(example.Path, "/ui/package_managed/"):
		example.Checkable = true
		example.Runner = "mod-check"
		example.Requires = "package manifest check"
		return
	case strings.Contains(example.Path, "/testing/"):
		example.Checkable = true
		example.Runner = "test-dir"
		example.Requires = "leia test CLI"
		return
	case strings.Contains(example.Path, "/web/hello_server.leia"),
		strings.Contains(example.Path, "/web/webserver.leia"):
		example.Runnable = true
		example.Checkable = true
		example.Runner = "web-loopback"
		example.Requires = ""
		return
	case strings.Contains(example.Path, "/web/tiny_app.leia"),
		strings.Contains(example.Path, "/web/fullstack_project/"),
		strings.Contains(example.Path, "/web/tiny_fullstack_app.leia"),
		strings.Contains(example.Path, "/web/serve_dialect_app.leia"),
		strings.Contains(example.Path, "/web/route_workbench.leia"),
		strings.Contains(example.Path, "/tooling/release_gate_project/"),
		strings.Contains(example.Path, "/concurrency/pipeline_project/"),
		strings.Contains(example.Path, "/concurrency/context_process.leia"),
		strings.Contains(example.Path, "/concurrency/goroutine_errors.leia"),
		strings.Contains(example.Path, "/data/db_q_frame_project/"),
		strings.Contains(example.Path, "/dialects/shell_filesystem.leia"):
		example.Runnable = true
		example.Checkable = true
		if strings.Contains(example.Path, "/tooling/release_gate_project/") {
			example.Runner = "release-gate-project"
		} else {
			example.Runner = "host-vm"
		}
		example.Requires = ""
		return
	case strings.Contains(example.Path, "/data_processing/data_oriented/particle_integration.leia"),
		strings.Contains(example.Path, "/game_engine/game_of_life.leia"):
		example.Runnable = true
		example.Checkable = true
		example.Runner = "host-vm-high"
		example.Requires = ""
		return
	case strings.Contains(example.Path, "/llm/") && cliExampleLLMMockFriendly(example.Path):
		example.Runnable = true
		example.Checkable = true
		example.Runner = "llm-mock"
		example.Requires = ""
		return
	}
	if example.Runner == "" {
		example.Runner = "playground"
	}
}

func runCLIExampleRunner(example cliExample, path string, maxSteps int64, stdout, stderr io.Writer) int {
	switch example.Runner {
	case "", "playground":
		return runPlaygroundExecCommand([]string{"--mode=bytecode", "--max-steps=" + fmt.Sprint(maxSteps), path}, stdout, stderr)
	case "evaluate":
		return runEvaluateCommand([]string{"--json", path}, stdout, stderr)
	case "evaluate-replay":
		return runEvaluateCommand([]string{"--json", "--replay", cliExampleCompanionRecordsPath(path), path}, stdout, stderr)
	case "llm-replay":
		return runCLIExampleLLMReplay(path, stderr)
	case "test":
		return runTestCommand([]string{"--json", "--golden=require", path}, cliRunOptions{UseVM: true}, stdout, stderr)
	case "test-dir":
		cliExampleTestRunnerMu.Lock()
		defer cliExampleTestRunnerMu.Unlock()
		return runTestCommand([]string{"--json", "--golden=require", filepath.Dir(path)}, cliRunOptions{UseVM: true}, stdout, stderr)
	case "mod-check":
		return runModCommand([]string{"check", "--json", filepath.Dir(path)}, stdout, stderr)
	case "web-loopback":
		return runCLIExampleWebLoopback(example, path, maxSteps, stdout, stderr)
	case "host-vm":
		return runCLIExampleHostVM(path, maxSteps, stdout, stderr)
	case "host-vm-high":
		return runCLIExampleHostVM(path, maxSteps*64, stdout, stderr)
	case "llm-mock":
		return runCLIExampleLLMMock(path, maxSteps, stdout, stderr)
	case "release-gate-project":
		return runCLIExampleReleaseGateProject(path, maxSteps, stdout, stderr)
	case "go-test":
		return runCLIExampleGoTest(example, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown example runner %q for %s\n", example.Runner, example.ID)
		return 1
	}
}

func runCLIExampleGoTest(example cliExample, stdout, stderr io.Writer) int {
	root := filepath.Dir(playgroundExamplesRoot())
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pkgDir := filepath.ToSlash(filepath.Dir(example.Path))
	if pkgDir == "." || pkgDir == "" {
		fmt.Fprintf(stderr, "run %s: go-test runner needs a package path\n", example.ID)
		return 1
	}
	args := []string{"test", "./" + pkgDir, "-count=1"}
	if strings.Contains(filepath.Base(example.Path), "embedding_test.go") {
		args = append(args, "-run", "Example")
	}
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = root
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			fmt.Fprintf(stderr, "run %s: timed out\n", example.ID)
			return 1
		}
		fmt.Fprintf(stderr, "run %s: %v\n", example.ID, err)
		return 1
	}
	return 0
}

func runCLIExampleHostVM(path string, maxSteps int64, stdout, stderr io.Writer) int {
	vm := leia.New(
		leia.WithLibs(leia.LibAll),
		leia.WithVM(),
		leia.WithMaxSteps(maxSteps),
		leia.WithMaxNativeCalls(100_000),
		leia.WithMaxGoroutines(128),
		leia.WithMaxChannelCapacity(1024),
		leia.WithMaxHostResultBytes(1<<20),
		leia.WithNetworkAccess(true),
		leia.WithProcessExecution(true),
		leia.WithProcessShell(true),
		leia.WithDebugAccess(true),
		leia.WithPrint(func(args ...interface{}) {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = fmt.Sprint(arg)
			}
			fmt.Fprintln(stdout, strings.Join(parts, "\t"))
		}),
	)
	if err := vm.ExecFile(path); err != nil {
		fmt.Fprintf(stderr, "run host example: %v\n", err)
		return 1
	}
	return 0
}

func runCLIExampleWebLoopback(example cliExample, path string, maxSteps int64, stdout, stderr io.Writer) int {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read web example: %v\n", err)
		return 1
	}
	testSrc, err := cliExampleWebLoopbackSource(example.Path, string(src))
	if err != nil {
		fmt.Fprintf(stderr, "prepare web loopback example: %v\n", err)
		return 1
	}
	vm := leia.New(
		leia.WithLibs(leia.LibAll),
		leia.WithVM(),
		leia.WithMaxSteps(maxSteps),
		leia.WithMaxNativeCalls(100_000),
		leia.WithMaxGoroutines(128),
		leia.WithMaxChannelCapacity(1024),
		leia.WithMaxHostResultBytes(1<<20),
		leia.WithNetworkAccess(true),
		leia.WithPrint(func(args ...interface{}) {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = fmt.Sprint(arg)
			}
			fmt.Fprintln(stdout, strings.Join(parts, "\t"))
		}),
	)
	if err := vm.Exec(testSrc); err != nil {
		fmt.Fprintf(stderr, "run web loopback example: %v\n", err)
		return 1
	}
	return 0
}

func cliExampleWebLoopbackSource(path, src string) (string, error) {
	switch {
	case strings.Contains(path, "/web/hello_server.leia"):
		const oldListen = `http.listen(":8080", func(req, res) {`
		if !strings.Contains(src, oldListen) {
			return "", fmt.Errorf("hello_server listen shape changed")
		}
		src = strings.Replace(src, oldListen, `__leia_example_server := http.listen("127.0.0.1:0", func(req, res) {`, 1)
		src = strings.TrimRight(src, " \t\r\n")
		if !strings.HasSuffix(src, "})") {
			return "", fmt.Errorf("hello_server handler close shape changed")
		}
		src = strings.TrimSuffix(src, "})") + `}, {background: true})

__leia_example_resp := http.get(__leia_example_server.url .. "/loopback")
assert(__leia_example_resp.status == 200)
assert(__leia_example_resp.body == "Hello from Leia!\nMethod: GET\nPath: /loopback\n")
__leia_example_closed, __leia_example_close_err := __leia_example_server.close()
assert(__leia_example_closed == true)
assert(__leia_example_close_err == nil)
__leia_example_waited, __leia_example_wait_err := __leia_example_server.wait()
assert(__leia_example_waited == true)
assert(__leia_example_wait_err == nil)
`
		return src, nil
	case strings.Contains(path, "/web/webserver.leia"):
		const oldListen = `router.listen(":9988")`
		if !strings.Contains(src, oldListen) {
			return "", fmt.Errorf("webserver listen shape changed")
		}
		src = strings.Replace(src, oldListen, `__leia_example_server := router.listen("127.0.0.1:0", {background: true})`, 1)
		src += `

__leia_example_root := http.get(__leia_example_server.url .. "/")
assert(__leia_example_root.status == 200)
assert(string.find(__leia_example_root.body, "<h1>Welcome to Leia Web Server!</h1>", 1, true) != nil)

__leia_example_hello := http.get(__leia_example_server.url .. "/hello?name=Ada")
assert(__leia_example_hello.status == 200)
assert(__leia_example_hello.body == "Hello, Ada!")

__leia_example_counter1 := http.get(__leia_example_server.url .. "/counter")
__leia_example_counter2 := http.get(__leia_example_server.url .. "/counter")
assert(__leia_example_counter1.body == "Counter: 1")
assert(__leia_example_counter2.body == "Counter: 2")

__leia_example_json := http.get(__leia_example_server.url .. "/json")
assert(__leia_example_json.status == 200)
assert(string.find(__leia_example_json.body, "\"language\":\"Leia\"", 1, true) != nil)
assert(string.find(__leia_example_json.body, "\"counter\":2", 1, true) != nil)

__leia_example_echo := http.get(__leia_example_server.url .. "/echo?msg=loop")
assert(__leia_example_echo.body == "Echo: loop")

__leia_example_fib := http.get(__leia_example_server.url .. "/fib?n=7")
assert(__leia_example_fib.body == "fib(7) = 13")

__leia_example_closed, __leia_example_close_err := __leia_example_server.shutdown()
assert(__leia_example_closed == true)
assert(__leia_example_close_err == nil)
__leia_example_waited, __leia_example_wait_err := __leia_example_server.wait()
assert(__leia_example_waited == true)
assert(__leia_example_wait_err == nil)
`
		return src, nil
	default:
		return "", fmt.Errorf("unsupported web loopback example %s", path)
	}
}

func runCLIExampleReleaseGateProject(path string, maxSteps int64, stdout, stderr io.Writer) int {
	vm := leia.New(
		leia.WithLibs(leia.LibAll),
		leia.WithVM(),
		leia.WithMaxSteps(maxSteps*4),
		leia.WithMaxNativeCalls(100_000),
		leia.WithMaxGoroutines(128),
		leia.WithMaxChannelCapacity(1024),
		leia.WithMaxHostResultBytes(1<<20),
		leia.WithFilesystem(true),
		leia.WithNetworkAccess(true),
		leia.WithProcessExecution(true),
		leia.WithProcessShell(true),
		leia.WithLLMProvider(cliExampleMockLLMProvider{}),
		leia.WithPrint(func(args ...interface{}) {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = fmt.Sprint(arg)
			}
			fmt.Fprintln(stdout, strings.Join(parts, "\t"))
		}),
	)
	if err := vm.ExecFile(path); err != nil {
		fmt.Fprintf(stderr, "run release gate project example: %v\n", err)
		return 1
	}
	return 0
}

func cliExampleLLMMockFriendly(path string) bool {
	name := filepath.Base(path)
	return name != "glm_smoke.leia" && name != "glm_direct_agent_tools.leia"
}

func runCLIExampleLLMMock(path string, maxSteps int64, stdout, stderr io.Writer) int {
	vm := leia.New(
		leia.WithLibs(leia.LibAll),
		leia.WithVM(),
		leia.WithMaxSteps(maxSteps),
		leia.WithMaxNativeCalls(100_000),
		leia.WithMaxGoroutines(128),
		leia.WithMaxChannelCapacity(1024),
		leia.WithMaxHostResultBytes(1<<20),
		leia.WithLLMProvider(cliExampleMockLLMProvider{}),
		leia.WithPrint(func(args ...interface{}) {
			parts := make([]string, len(args))
			for i, arg := range args {
				parts[i] = fmt.Sprint(arg)
			}
			fmt.Fprintln(stdout, strings.Join(parts, "\t"))
		}),
	)
	if err := vm.ExecFile(path); err != nil {
		fmt.Fprintf(stderr, "run mock llm example: %v\n", err)
		return 1
	}
	return 0
}

type cliExampleMockLLMProvider struct{}

func (cliExampleMockLLMProvider) Turn(_ context.Context, req llm.TurnRequest) (llm.TurnResult, error) {
	switch req.Model {
	case "mock-fast":
		return llm.TurnResult{Status: "final_answer", Text: "Direct llm.turn keeps examples simple."}, nil
	case "mock-stream":
		return llm.TurnResult{Status: "final_answer", Text: "hello stream"}, nil
	}
	if len(req.Messages) > 0 && req.Messages[len(req.Messages)-1].Role == "tool" {
		return cliExampleMockFinalAfterTool(req), nil
	}
	if len(req.Tools) > 0 {
		tool := req.Tools[0]
		return llm.TurnResult{
			Status: "tool_calls",
			Calls: []llm.ToolCall{{
				ID:   "call_" + tool.Name + "_1",
				Tool: tool.Name,
				Args: cliExampleMockToolArgs(tool),
			}},
		}, nil
	}
	if req.ResponseFormat != nil || strings.Contains(strings.ToLower(fmt.Sprint(req.ResponseFormat)), "summary") {
		return llm.TurnResult{Status: "final_answer", Text: `{"summary":"mock evidence accepted","confidence":1,"risk":"low"}`}, nil
	}
	return llm.TurnResult{Status: "final_answer", Text: cliExampleMockText(req)}, nil
}

func (p cliExampleMockLLMProvider) StreamTurn(ctx context.Context, req llm.TurnRequest, sink llm.StreamSink) (llm.TurnResult, error) {
	res, err := p.Turn(ctx, req)
	if err != nil || sink == nil {
		return res, err
	}
	tokens := []string{"hello", " ", "stream"}
	if res.Text != "hello stream" {
		tokens = strings.Fields(res.Text)
	}
	for _, token := range tokens {
		if err := sink(llm.StreamEvent{Type: "token", Token: token, Text: token}); err != nil {
			return llm.TurnResult{}, err
		}
	}
	return res, nil
}

func cliExampleMockToolArgs(tool llm.Tool) map[string]any {
	args := make(map[string]any, len(tool.Params))
	for _, param := range tool.Params {
		switch param {
		case "domain":
			args[param] = "pci"
		case "control":
			args[param] = "refunds"
		case "service":
			args[param] = "checkout"
		case "severity":
			args[param] = "sev2"
		case "symptom":
			args[param] = "p95 latency spike"
		case "topic":
			args[param] = "agent history"
		case "question":
			args[param] = "Can refunds bypass PCI review?"
		default:
			args[param] = "mock"
		}
	}
	return args
}

func cliExampleMockFinalAfterTool(req llm.TurnRequest) llm.TurnResult {
	if req.ResponseFormat != nil || strings.Contains(strings.ToLower(fmt.Sprint(req.ResponseFormat)), "summary") {
		return llm.TurnResult{Status: "final_answer", Text: `{"summary":"mock policy evidence","confidence":1,"risk":"low"}`}
	}
	switch req.Model {
	case "mock-manual":
		return llm.TurnResult{Status: "final_answer", Text: "Manual history used local documentation evidence."}
	case "mock-rich-agent":
		return llm.TurnResult{Status: "final_answer", Text: "Checkout sev2 requires payment queue triage."}
	case "mock-planner":
		return llm.TurnResult{Status: "final_answer", Text: "Checkout latency is elevated; follow runbook and watch error rate."}
	default:
		return llm.TurnResult{Status: "final_answer", Text: "Mock tool evidence reviewed."}
	}
}

func cliExampleMockText(req llm.TurnRequest) string {
	switch req.Model {
	case "mock-prompt":
		return "Leia agent history keeps tagged prompt messages searchable."
	case "mock-audit":
		return "Delegated answer audited with mock evidence."
	case "mock-ops":
		return "Checkout incidents are owned by the operations responder."
	case "mock-planner":
		return "Checkout latency is elevated; follow runbook and watch error rate."
	default:
		if strings.Contains(cliExampleMockRequestText(req), "audit delegated answer") {
			return "Delegated answer audited with mock evidence."
		}
		return "Mock LLM example completed."
	}
}

func cliExampleMockRequestText(req llm.TurnRequest) string {
	var b strings.Builder
	for _, msg := range req.Messages {
		b.WriteString(strings.ToLower(msg.Text))
		b.WriteByte('\n')
	}
	return b.String()
}

func runCLIExampleLLMReplay(path string, stderr io.Writer) int {
	records, err := llm.LoadRecords(cliExampleCompanionRecordsPath(path))
	if err != nil {
		fmt.Fprintf(stderr, "load replay records: %v\n", err)
		return 1
	}
	vm := leia.New(
		leia.WithLibs(leia.LibAll),
		leia.WithLLMReplay(records),
		leia.WithPrint(func(args ...interface{}) {}),
		leia.WithVM(),
	)
	if err := vm.ExecFile(path); err != nil {
		fmt.Fprintf(stderr, "run replay example: %v\n", err)
		return 1
	}
	return 0
}

func cliExampleCompanionRecordsExist(path string) bool {
	info, err := os.Stat(cliExampleCompanionRecordsPath(filepath.Join(filepath.Dir(playgroundExamplesRoot()), filepath.FromSlash(path))))
	return err == nil && !info.IsDir()
}

func cliExampleCompanionRecordsPath(sourcePath string) string {
	return strings.TrimSuffix(sourcePath, filepath.Ext(sourcePath)) + ".records.json"
}

func boolFlagString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}
