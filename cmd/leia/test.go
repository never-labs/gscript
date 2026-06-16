package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	leia "github.com/never-labs/leia"
	toolsource "github.com/never-labs/leia/internal/support/source"
)

var cliStdoutRedirectMu sync.Mutex

func runTestCommand(args []string, opts cliRunOptions, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "write the test report as JSON")
	format := fs.String("format", "text", "output format: text or json")
	goldenMode := fs.String("golden", "auto", "golden stdout mode: auto, require, ignore, or update")
	listOnly := fs.Bool("list", false, "list matching .leia test files without running them")
	manifestCheck := fs.Bool("manifest-check", false, "check tests/manifest.json against discovered test cases")
	output := fs.String("output", "", "write command output to this file instead of stdout")
	seed := fs.String("seed", "", "set LEIA_TEST_SEED while running tests")
	if code, done := parseCLIFlags(fs, args); done {
		return code
	}
	if *manifestCheck {
		if fs.NArg() != 0 {
			fmt.Fprintln(errw, "usage: leia test --manifest-check")
			return 2
		}
		return runManifestCheckRoots([]string{"tests"}, outw, errw)
	}
	if *jsonOut {
		*format = "json"
	}
	paths := fs.Args()
	if len(paths) == 0 {
		paths = []string{defaultTestPath()}
	}
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: leia test [--json|--format=text|json] [--output FILE] [--golden=auto|require|ignore|update] [--list] [--seed SEED] [path-or-dir]")
		return 2
	}
	if !flagWasSet(fs, "format") {
		config, diagnostics, err := loadOptionalCLIProjectConfig(paths[0])
		if err != nil {
			printCLIConfigDiagnostics(errw, paths[0], diagnostics)
			return 2
		}
		if config != nil && config.Tool.Test.Format != "" {
			*format = config.Tool.Test.Format
		}
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(errw, "leia test: unsupported --format %q (want text or json)\n", *format)
		return 2
	}
	if !validTestGoldenMode(*goldenMode) {
		fmt.Fprintf(errw, "leia test: unsupported --golden %q (want auto, require, ignore, or update)\n", *goldenMode)
		return 2
	}
	if *listOnly {
		files, err := testFiles(paths[0])
		if err != nil {
			fmt.Fprintf(errw, "%s: %v\n", paths[0], err)
			return 1
		}
		if *format == "json" {
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(testListReport{
				SchemaVersion: 1,
				Status:        "pass",
				ListOnly:      true,
				GoldenMode:    *goldenMode,
				FileCount:     len(files),
				Files:         files,
			}); err != nil {
				fmt.Fprintf(errw, "leia test: write json: %v\n", err)
				return 1
			}
			if err := writeCLIOutput(outw, *output, buf.Bytes()); err != nil {
				fmt.Fprintf(errw, "leia test: %v\n", err)
				return 1
			}
			return 0
		}
		var buf bytes.Buffer
		for _, file := range files {
			fmt.Fprintln(&buf, file)
		}
		if err := writeCLIOutput(outw, *output, buf.Bytes()); err != nil {
			fmt.Fprintf(errw, "leia test: %v\n", err)
			return 1
		}
		return 0
	}
	result := runTestsDetailed(paths[0], opts, errw, *format == "text", *seed, *goldenMode)
	if *format == "json" {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(result); err != nil {
			fmt.Fprintf(errw, "leia test: write json: %v\n", err)
			return 1
		}
		if err := writeCLIOutput(outw, *output, buf.Bytes()); err != nil {
			fmt.Fprintf(errw, "leia test: %v\n", err)
			return 1
		}
	}
	if !result.OK {
		return 1
	}
	return 0
}

func defaultTestPath() string {
	if info, err := os.Stat("tests"); err == nil && info.IsDir() {
		return "tests"
	}
	return "."
}

func validTestGoldenMode(mode string) bool {
	switch mode {
	case "auto", "require", "ignore", "update":
		return true
	default:
		return false
	}
}

func writeCLIOutput(outw io.Writer, outputPath string, data []byte) error {
	if outputPath != "" {
		if err := os.WriteFile(outputPath, data, 0600); err != nil {
			return fmt.Errorf("write %s: %w", outputPath, err)
		}
		return nil
	}
	_, err := outw.Write(data)
	return err
}

type testRunResult struct {
	SchemaVersion int              `json:"schema_version"`
	OK            bool             `json:"ok"`
	Status        string           `json:"status"`
	Total         int              `json:"total"`
	Passed        int              `json:"passed"`
	Failed        int              `json:"failed"`
	Seed          string           `json:"seed,omitempty"`
	GoldenMode    string           `json:"golden_mode"`
	Files         []testFileResult `json:"files"`
}

type testListReport struct {
	SchemaVersion int      `json:"schema_version"`
	Status        string   `json:"status"`
	ListOnly      bool     `json:"list_only"`
	GoldenMode    string   `json:"golden_mode"`
	FileCount     int      `json:"file_count"`
	Files         []string `json:"files"`
}

type testFileResult struct {
	File       string `json:"file"`
	OK         bool   `json:"ok"`
	Golden     string `json:"golden,omitempty"`
	Error      string `json:"error,omitempty"`
	Expected   string `json:"expected,omitempty"`
	Actual     string `json:"actual,omitempty"`
	ExitCodeOK bool   `json:"exit_code_ok,omitempty"`
}

func runTests(path string, opts cliRunOptions, errw io.Writer) bool {
	return runTestsDetailed(path, opts, errw, true, "", "auto").OK
}

func runTestsDetailed(path string, opts cliRunOptions, errw io.Writer, text bool, seed string, goldenMode string) testRunResult {
	files, err := testFiles(path)
	if err != nil {
		if text {
			fmt.Fprintf(errw, "%s: %v\n", path, err)
		}
		return testRunResult{
			SchemaVersion: 1,
			OK:            false,
			Status:        "issues",
			Total:         1,
			Failed:        1,
			Files:         []testFileResult{{File: path, OK: false, Error: err.Error()}},
			GoldenMode:    goldenMode,
		}
	}

	result := testRunResult{
		SchemaVersion: 1,
		OK:            true,
		Status:        "pass",
		Total:         len(files),
		Seed:          seed,
		GoldenMode:    goldenMode,
		Files:         make([]testFileResult, 0, len(files)),
	}
	if seed != "" {
		oldSeed, hadSeed := os.LookupEnv("LEIA_TEST_SEED")
		_ = os.Setenv("LEIA_TEST_SEED", seed)
		defer func() {
			if hadSeed {
				_ = os.Setenv("LEIA_TEST_SEED", oldSeed)
			} else {
				_ = os.Unsetenv("LEIA_TEST_SEED")
			}
		}()
	}
	for _, filename := range files {
		fileResult := testFileResult{File: filename, OK: true}
		golden, hasGolden, err := testGoldenOutputFile(filename)
		if hasGolden || goldenMode == "require" || goldenMode == "update" {
			fileResult.Golden = golden
		}
		if err != nil {
			fileResult.OK = false
			fileResult.Error = fmt.Sprintf("stat golden %s: %v", golden, err)
			if text {
				fmt.Fprintf(errw, "%s: %s\n", filename, fileResult.Error)
			}
			result.Files = append(result.Files, fileResult)
			continue
		}
		if goldenMode == "require" && !hasGolden {
			fileResult.OK = false
			fileResult.Error = fmt.Sprintf("missing golden %s", golden)
			if text {
				fmt.Fprintf(errw, "%s: %s\n", filename, fileResult.Error)
			}
			result.Files = append(result.Files, fileResult)
			continue
		}

		var stdout []byte
		var runErr error
		compareGolden := goldenMode == "auto" && hasGolden || goldenMode == "require"
		updateGolden := goldenMode == "update"
		stdout, runErr = runScriptFileCapturingStdout(filename, opts)
		if runErr != nil {
			if code, isExit := processExitCode(runErr); isExit && code == 0 {
				fileResult.ExitCodeOK = true
			} else {
				fileResult.OK = false
				fileResult.Error = runErr.Error()
				if text {
					fmt.Fprintf(errw, "%s: %v\n", filename, runErr)
				}
				result.Files = append(result.Files, fileResult)
				continue
			}
		}
		if updateGolden {
			if err := os.WriteFile(golden, stdout, 0644); err != nil {
				fileResult.OK = false
				fileResult.Error = fmt.Sprintf("write golden %s: %v", golden, err)
				if text {
					fmt.Fprintf(errw, "%s: %s\n", filename, fileResult.Error)
				}
			}
			result.Files = append(result.Files, fileResult)
			continue
		}
		if !compareGolden {
			result.Files = append(result.Files, fileResult)
			continue
		}

		expected, err := os.ReadFile(golden)
		if err != nil {
			fileResult.OK = false
			fileResult.Error = fmt.Sprintf("read golden %s: %v", golden, err)
			if text {
				fmt.Fprintf(errw, "%s: %s\n", filename, fileResult.Error)
			}
			result.Files = append(result.Files, fileResult)
			continue
		}
		if !bytes.Equal(stdout, expected) {
			fileResult.OK = false
			fileResult.Expected = string(expected)
			fileResult.Actual = string(stdout)
			if text {
				fmt.Fprintf(errw, "%s: stdout mismatch with %s\n%s", filename, golden, stdoutDiff(expected, stdout))
			}
		}
		result.Files = append(result.Files, fileResult)
	}
	for _, file := range result.Files {
		if file.OK {
			result.Passed++
		} else {
			result.Failed++
		}
	}
	result.OK = result.Failed == 0
	result.Status = testStatus(result.OK)
	return result
}

func testStatus(ok bool) string {
	if ok {
		return "pass"
	}
	return "issues"
}

func testGoldenOutputFile(filename string) (string, bool, error) {
	golden := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".out"
	_, err := os.Stat(golden)
	if err == nil {
		return golden, true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return golden, false, nil
	}
	return golden, false, fmt.Errorf("stat golden %s: %w", golden, err)
}

func runScriptFileCapturingStdout(filename string, opts cliRunOptions) ([]byte, error) {
	if canUsePublicRunPath(opts) {
		return runPublicScriptFileCapturingPrint(filename, nil, opts)
	}

	cliStdoutRedirectMu.Lock()
	defer cliStdoutRedirectMu.Unlock()

	r, w, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var stdout bytes.Buffer
	copyDone := make(chan error, 1)
	go func() {
		_, err := io.Copy(&stdout, r)
		copyDone <- err
	}()

	oldStdout := os.Stdout
	var runErr error
	func() {
		os.Stdout = w
		defer func() {
			os.Stdout = oldStdout
		}()
		interp := newCLIInterpreter()
		runErr = runScriptFile(interp, filename, nil, opts)
	}()

	closeErr := w.Close()
	copyErr := <-copyDone
	if runErr != nil {
		return stdout.Bytes(), runErr
	}
	if closeErr != nil {
		return stdout.Bytes(), closeErr
	}
	if copyErr != nil {
		return stdout.Bytes(), copyErr
	}
	return stdout.Bytes(), nil
}

func runPublicScriptFileCapturingPrint(filename string, args []string, opts cliRunOptions) ([]byte, error) {
	var stdout bytes.Buffer
	leiaOpts := publicRunOptions(opts, filename, args)
	leiaOpts = append(leiaOpts, leia.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, arg := range args {
			parts[i] = fmt.Sprint(arg)
		}
		fmt.Fprintln(&stdout, strings.Join(parts, "\t"))
	}))
	vm := leia.New(leiaOpts...)
	err := vm.ExecFile(filename)
	return stdout.Bytes(), err
}

func stdoutDiff(expected, got []byte) string {
	want := string(expected)
	have := string(got)
	if len(want) > 400 {
		want = want[:400] + "...(truncated)"
	}
	if len(have) > 400 {
		have = have[:400] + "...(truncated)"
	}
	return fmt.Sprintf("expected:\n%s\ngot:\n%s\n", want, have)
}

func testFiles(path string) ([]string, error) {
	return toolsource.Files(path)
}
