package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"

	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
	"github.com/gscript/gscript/internal/runtime"
	bytecodevm "github.com/gscript/gscript/internal/vm"
)

// jitStatsReporter is implemented by the platform-specific JIT engine wrapper
// so the CLI can print tier statistics after execution.
type jitStatsReporter interface {
	PrintStats(w *os.File)
	PrintExitStats(w *os.File)
	PrintExitStatsJSON(w *os.File) error
	PrintTier2PerfStats(w *os.File)
	PrintTier2PerfStatsJSON(w *os.File) error
	PrintTier2SpeculationStateJSON(w *os.File) error
	PrintTier2SpeculationWorklistJSON(w *os.File) error
	Close() error
}

type jitCLIOptions struct {
	TimelinePath              string
	TimelineFormat            string
	WarmDumpDir               string
	WarmDumpProto             string
	ShowExitStats             bool
	ShowExitStatsJSON         bool
	ShowTier2PerfStats        bool
	ShowTier2PerfStatsJSON    bool
	ShowTier2SpecStateJSON    bool
	ShowTier2SpecWorklistJSON bool
	ShowCoroutineStats        bool
	ShowPathStats             bool
	ShowPathStatsJSON         bool
}

type jitWarmDumpController interface {
	EnableWarmDump(dir, protoName string) error
	WriteWarmDump(top *bytecodevm.FuncProto) error
}

var cliStdin io.Reader = os.Stdin

// sortStrings is a tiny helper shared with platform files to keep them from
// each importing "sort".
func sortStrings(s []string) { sort.Strings(s) }

func init() {
	goruntime.LockOSThread() // Required for GLFW/OpenGL on macOS
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bench":
			os.Exit(runBenchCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "capabilities":
			os.Exit(runCapabilitiesCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "check":
			os.Exit(runCheckCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "ci":
			os.Exit(runCICommand(os.Args[2:], os.Stdout, os.Stderr))
		case "config":
			os.Exit(runConfigCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "diag":
			os.Exit(runDiagCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "doc":
			os.Exit(runDocCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "eval":
			os.Exit(runEvalCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "env":
			os.Exit(runEnvCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "fmt":
			os.Exit(runFmtCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "help":
			os.Exit(runHelpCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "inspect":
			os.Exit(runInspectCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "lint":
			os.Exit(runLintCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "mod":
			os.Exit(runModCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "repl":
			os.Exit(runREPLCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "run":
			os.Exit(runRunCommand(os.Args[2:], os.Stdout, os.Stderr))
		case "version":
			os.Exit(runVersionCommand(os.Args[2:], os.Stdout, os.Stderr))
		}
	}

	// Flags
	eval := flag.String("e", "", "execute string")
	useVM := flag.Bool("vm", false, "use bytecode VM without JIT")
	useJIT := flag.Bool("jit", true, "use bytecode VM with JIT compilation (default)")
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to file")
	memprofile := flag.String("memprofile", "", "write memory profile to file")
	jitStats := flag.Bool("jit-stats", false, "print JIT tier statistics after execution")
	jitTimeline := flag.String("jit-timeline", "", "write production JIT event timeline to file ('-' for stderr)")
	jitTimelineFormat := flag.String("jit-timeline-format", "jsonl", "JIT timeline format: jsonl or json")
	jitDumpWarm := flag.String("jit-dump-warm", "", "write warm production Tier 2 diagnostic dump to directory")
	jitDumpProto := flag.String("jit-dump-proto", "", "limit -jit-dump-warm to a proto name")
	exitStats := flag.Bool("exit-stats", false, "print Tier 2 exit/deopt profile after execution")
	exitStatsJSON := flag.Bool("exit-stats-json", false, "print Tier 2 exit/deopt profile as JSON after execution")
	tier2PerfStats := flag.Bool("tier2-perf-stats", false, "print opt-in Tier 2 protocol/timing diagnostics after execution")
	tier2PerfStatsJSON := flag.Bool("tier2-perf-stats-json", false, "print opt-in Tier 2 protocol/timing diagnostics as JSON after execution")
	tier2SpecStateJSON := flag.Bool("tier2-spec-state-json", false, "print Tier 2 specialization/version state as JSON after execution")
	tier2SpecWorklistJSON := flag.Bool("tier2-spec-worklist-json", false, "print prioritized Tier 2 self-driving worklist as JSON after execution")
	jitOpAudit := flag.Bool("jit-op-audit", false, "print MethodJIT op audit matrix and exit")
	jitOpAuditJSON := flag.Bool("jit-op-audit-json", false, "print MethodJIT op audit matrix as JSON and exit")
	coroutineStats := flag.Bool("coroutine-stats", false, "print VM coroutine runtime statistics after execution")
	pathStats := flag.Bool("runtime-path-stats", false, "print runtime path counters after execution")
	pathStatsJSON := flag.Bool("runtime-path-stats-json", false, "print runtime path counters as JSON after execution")
	flag.Parse()

	if *jitOpAudit {
		if err := cliPrintMethodJITOpAudit(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *jitOpAuditJSON {
		if err := cliPrintMethodJITOpAuditJSON(os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not create CPU profile: %v\n", err)
			os.Exit(1)
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if *memprofile != "" {
		defer func() {
			f, err := os.Create(*memprofile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not create memory profile: %v\n", err)
				return
			}
			defer f.Close()
			goruntime.GC()
			pprof.WriteHeapProfile(f)
		}()
	}

	// Determine which flags were explicitly set by the user.
	vmExplicit := false
	jitExplicit := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "vm":
			vmExplicit = true
		case "jit":
			jitExplicit = true
		}
	})

	// -vm without -jit means "VM only, no JIT".
	if vmExplicit && !jitExplicit {
		*useJIT = false
	}
	if *useJIT {
		*useVM = true
	}

	runOpts := cliRunOptions{
		UseVM:        *useVM,
		UseJIT:       *useJIT,
		ShowJITStats: *jitStats,
		JIT: jitCLIOptions{
			TimelinePath:              *jitTimeline,
			TimelineFormat:            *jitTimelineFormat,
			WarmDumpDir:               *jitDumpWarm,
			WarmDumpProto:             *jitDumpProto,
			ShowExitStats:             *exitStats,
			ShowExitStatsJSON:         *exitStatsJSON,
			ShowTier2PerfStats:        *tier2PerfStats,
			ShowTier2PerfStatsJSON:    *tier2PerfStatsJSON,
			ShowTier2SpecStateJSON:    *tier2SpecStateJSON,
			ShowTier2SpecWorklistJSON: *tier2SpecWorklistJSON,
			ShowCoroutineStats:        *coroutineStats,
			ShowPathStats:             *pathStats,
			ShowPathStatsJSON:         *pathStatsJSON,
		},
	}

	args := flag.Args()
	if len(args) > 0 && args[0] == "test" {
		os.Exit(runTestCommand(args[1:], runOpts, os.Stdout, os.Stderr))
		return
	}

	interp := runtime.New()

	if *eval != "" {
		interp.SetArgs("<eval>", flag.Args())
		// Execute string
		if err := runString(interp, *eval); err != nil {
			if exit, ok := processExit(err); ok {
				os.Exit(exit.Code)
			}
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(args) == 0 {
		// REPL mode
		interp.SetArgs("<repl>", nil)
		runREPL(interp)
		return
	}

	// Execute file
	filename := args[0]
	if err := runScriptFile(interp, filename, args[1:], runOpts); err != nil {
		if exit, ok := processExit(err); ok {
			os.Exit(exit.Code)
		}
		fmt.Fprintf(os.Stderr, "%s: %v\n", filename, err)
		os.Exit(1)
	}
}

type cliCapabilities struct {
	SchemaVersion int                    `json:"schema_version"`
	Platform      cliPlatformCapability  `json:"platform"`
	Execution     cliExecutionCapability `json:"execution"`
	Commands      []string               `json:"commands"`
	StdlibModules []string               `json:"stdlib_modules"`
	Tooling       cliToolingCapability   `json:"tooling"`
}

type cliPlatformCapability struct {
	GOOS   string `json:"goos"`
	GOARCH string `json:"goarch"`
}

type cliExecutionCapability struct {
	Interpreter bool `json:"interpreter"`
	BytecodeVM  bool `json:"bytecode_vm"`
	JIT         bool `json:"jit"`
	MethodJIT   bool `json:"method_jit"`
}

type cliToolingCapability struct {
	Formatter cliFormatterCapability `json:"formatter"`
	Linter    cliLinterCapability    `json:"linter"`
	Test      cliTestCapability      `json:"test"`
	Config    cliConfigCapability    `json:"config"`
}

type cliFormatterCapability struct {
	Stdin     bool     `json:"stdin"`
	Check     bool     `json:"check"`
	Write     bool     `json:"write"`
	Formats   []string `json:"formats"`
	Stability string   `json:"stability"`
}

type cliLinterCapability struct {
	Formats []string `json:"formats"`
	Codes   []string `json:"codes"`
}

type cliTestCapability struct {
	GoldenStdout bool `json:"golden_stdout"`
	Directory    bool `json:"directory"`
}

func runCapabilitiesCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("capabilities", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonOut := fs.Bool("json", false, "print capabilities as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: gscript capabilities [--json]")
		return 2
	}
	caps := buildCapabilities()
	if *jsonOut {
		enc := json.NewEncoder(outw)
		enc.SetIndent("", "  ")
		if err := enc.Encode(caps); err != nil {
			fmt.Fprintf(errw, "gscript capabilities: write json: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprintf(outw, "platform: %s/%s\n", caps.Platform.GOOS, caps.Platform.GOARCH)
	fmt.Fprintf(outw, "jit: %t\n", caps.Execution.JIT)
	fmt.Fprintf(outw, "commands: %s\n", strings.Join(caps.Commands, ", "))
	fmt.Fprintf(outw, "stdlib modules: %d\n", len(caps.StdlibModules))
	return 0
}

func buildCapabilities() cliCapabilities {
	modules := runtime.StdlibModuleNames()
	sort.Strings(modules)
	return cliCapabilities{
		SchemaVersion: 1,
		Platform: cliPlatformCapability{
			GOOS:   goruntime.GOOS,
			GOARCH: goruntime.GOARCH,
		},
		Execution: cliExecutionCapability{
			Interpreter: true,
			BytecodeVM:  true,
			JIT:         cliJITAvailable(),
			MethodJIT:   cliMethodJITAvailable(),
		},
		Commands:      cliCommandNames(),
		StdlibModules: modules,
		Tooling: cliToolingCapability{
			Formatter: cliFormatterCapability{
				Stdin:     true,
				Check:     true,
				Write:     true,
				Formats:   []string{"source"},
				Stability: "whitespace-normalizer",
			},
			Linter: cliLinterCapability{
				Formats: []string{"text", "json", "sarif"},
				Codes:   []string{"GS0001", "GS1001"},
			},
			Test: cliTestCapability{
				GoldenStdout: true,
				Directory:    true,
			},
			Config: cliConfigCapability{
				FileName: "gscript.toml",
				Sections: []string{
					"project",
					"tool.fmt",
					"tool.lint",
					"tool.test",
				},
				Formats: []string{"text", "json"},
			},
		},
	}
}

func runFmtCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("fmt", flag.ContinueOnError)
	fs.SetOutput(errw)
	check := fs.Bool("check", false, "check whether files are formatted without writing")
	write := fs.Bool("write", false, "write formatted files in place")
	stdinFileName := fs.String("stdin-file-name", "", "read source from stdin and use this filename for diagnostics")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 && *stdinFileName == "" {
		fmt.Fprintln(errw, "usage: gscript fmt [--check] [--write] [--stdin-file-name FILE] <path-or-dir> [...]")
		return 2
	}
	if *check && *write {
		fmt.Fprintln(errw, "gscript fmt: --check and --write are mutually exclusive")
		return 2
	}
	if *stdinFileName != "" {
		if len(paths) != 0 {
			fmt.Fprintln(errw, "gscript fmt: --stdin-file-name cannot be used with path arguments")
			return 2
		}
		if *write {
			fmt.Fprintln(errw, "gscript fmt: --stdin-file-name cannot be used with --write")
			return 2
		}
		return runFmtStdin(*stdinFileName, *check, outw, errw)
	}

	writeFiles := *write || !*check
	ok := true
	for _, path := range paths {
		files, err := gscriptFiles(path)
		if err != nil {
			fmt.Fprintf(errw, "%s: %v\n", path, err)
			ok = false
			continue
		}
		for _, filename := range files {
			changed, err := formatFile(filename, writeFiles)
			if err != nil {
				fmt.Fprintf(errw, "%s: %v\n", filename, err)
				ok = false
				continue
			}
			if *check && changed {
				fmt.Fprintf(errw, "%s: not formatted\n", filename)
				ok = false
			}
			if writeFiles && changed {
				fmt.Fprintln(outw, filename)
			}
		}
	}
	if !ok {
		return 1
	}
	return 0
}

func runFmtStdin(filename string, check bool, outw, errw io.Writer) int {
	src, err := io.ReadAll(cliStdin)
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	if check {
		if !bytes.Equal(src, formatted) {
			fmt.Fprintf(errw, "%s: not formatted\n", filename)
			return 1
		}
		return 0
	}
	if _, err := outw.Write(formatted); err != nil {
		fmt.Fprintf(errw, "%s: %v\n", filename, err)
		return 1
	}
	return 0
}

type lintDiagnostic struct {
	File     string `json:"file"`
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Line     int    `json:"line,omitempty"`
	Column   int    `json:"column,omitempty"`

	text string
}

var lintDiagnosticPositionRE = regexp.MustCompile(`(?:^|[^0-9])([0-9]+):([0-9]+)(?:[^0-9]|$)`)

func newLintDiagnostic(file, code, severity string, err error) lintDiagnostic {
	diagnostic := lintDiagnostic{
		File:     file,
		Code:     code,
		Severity: severity,
		Message:  err.Error(),
	}
	diagnostic.Line, diagnostic.Column = parseLintDiagnosticPosition(diagnostic.Message)
	return diagnostic
}

func parseLintDiagnosticPosition(message string) (int, int) {
	match := lintDiagnosticPositionRE.FindStringSubmatch(message)
	if match == nil {
		return 0, 0
	}
	line, lineErr := strconv.Atoi(match[1])
	column, columnErr := strconv.Atoi(match[2])
	if lineErr != nil || columnErr != nil {
		return 0, 0
	}
	return line, column
}

func runLintCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("lint", flag.ContinueOnError)
	fs.SetOutput(errw)
	format := fs.String("format", "text", "output format: text, json, or sarif")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) == 0 {
		fmt.Fprintln(errw, "usage: gscript lint [--format=text|json|sarif] <path-or-dir> [...]")
		return 2
	}
	if !flagWasSet(fs, "format") {
		config, diagnostics, err := loadOptionalCLIProjectConfig(paths[0])
		if err != nil {
			printCLIConfigDiagnostics(errw, paths[0], diagnostics)
			return 2
		}
		if config != nil && config.Tool.Lint.Format != "" {
			*format = config.Tool.Lint.Format
		}
	}
	if *format != "text" && *format != "json" && *format != "sarif" {
		fmt.Fprintf(errw, "gscript lint: unsupported --format %q (want text, json, or sarif)\n", *format)
		return 2
	}

	diagnostics := []lintDiagnostic{}
	for _, path := range paths {
		files, err := gscriptFiles(path)
		if err != nil {
			diagnostic := newLintDiagnostic(path, "GS0001", "error", err)
			diagnostic.text = fmt.Sprintf("%s: %v", path, err)
			diagnostics = append(diagnostics, diagnostic)
			continue
		}
		for _, filename := range files {
			if err := parseGScriptFile(filename); err != nil {
				diagnostics = append(diagnostics, newLintDiagnostic(filename, "GS1001", "error", err))
			}
		}
	}

	if *format == "json" {
		if err := json.NewEncoder(outw).Encode(diagnostics); err != nil {
			fmt.Fprintf(errw, "gscript lint: write json: %v\n", err)
			return 1
		}
	} else if *format == "sarif" {
		if err := writeLintSARIF(outw, diagnostics); err != nil {
			fmt.Fprintf(errw, "gscript lint: write sarif: %v\n", err)
			return 1
		}
	} else {
		for _, diagnostic := range diagnostics {
			if diagnostic.text != "" {
				fmt.Fprintln(errw, diagnostic.text)
				continue
			}
			fmt.Fprintf(errw, "%s: %s %s: %s\n", diagnostic.File, diagnostic.Code, diagnostic.Severity, diagnostic.Message)
		}
	}

	if len(diagnostics) > 0 {
		return 1
	}
	return 0
}

type sarifLog struct {
	Version string     `json:"version"`
	Schema  string     `json:"$schema"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules"`
}

type sarifRule struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	ShortDescription sarifText          `json:"shortDescription"`
	DefaultConfig    sarifDefaultConfig `json:"defaultConfiguration"`
}

type sarifDefaultConfig struct {
	Level string `json:"level"`
}

type sarifText struct {
	Text string `json:"text"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifText       `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
}

func writeLintSARIF(w io.Writer, diagnostics []lintDiagnostic) error {
	log := sarifLog{
		Version: "2.1.0",
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name: "gscript lint",
				Rules: []sarifRule{
					{
						ID:               "GS0001",
						Name:             "file-discovery",
						ShortDescription: sarifText{Text: "File discovery failed"},
						DefaultConfig:    sarifDefaultConfig{Level: "error"},
					},
					{
						ID:               "GS1001",
						Name:             "syntax",
						ShortDescription: sarifText{Text: "Lexer or parser error"},
						DefaultConfig:    sarifDefaultConfig{Level: "error"},
					},
				},
			}},
			Results: make([]sarifResult, 0, len(diagnostics)),
		}},
	}
	for _, diagnostic := range diagnostics {
		result := sarifResult{
			RuleID:  diagnostic.Code,
			Level:   diagnostic.Severity,
			Message: sarifText{Text: diagnostic.Message},
			Locations: []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: filepath.ToSlash(diagnostic.File)},
					Region: sarifRegion{
						StartLine:   diagnostic.Line,
						StartColumn: diagnostic.Column,
					},
				},
			}},
		}
		log.Runs[0].Results = append(log.Runs[0].Results, result)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(log)
}

func formatFile(filename string, write bool) (bool, error) {
	src, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}
	formatted, err := formatSource(filename, src)
	if err != nil {
		return false, err
	}
	changed := !bytes.Equal(src, formatted)
	if write && changed {
		if err := os.WriteFile(filename, formatted, 0644); err != nil {
			return false, err
		}
	}
	return changed, nil
}

func formatSource(filename string, src []byte) ([]byte, error) {
	if err := parseGScriptSource(filename, src); err != nil {
		return nil, err
	}

	normalized := bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))
	normalized = bytes.ReplaceAll(normalized, []byte("\r"), []byte("\n"))
	lines := bytes.Split(normalized, []byte("\n"))
	for i := range lines {
		lines[i] = bytes.TrimRight(lines[i], " \t")
	}
	for len(lines) > 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return []byte("\n"), nil
	}
	return append(bytes.Join(lines, []byte("\n")), '\n'), nil
}

func parseGScriptFile(filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return parseGScriptSource(filename, src)
}

func parseGScriptSource(filename string, src []byte) error {
	tokens, err := lexer.New(string(src)).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}
	if _, err := parser.New(tokens).Parse(); err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	return nil
}

type cliRunOptions struct {
	UseVM        bool
	UseJIT       bool
	ShowJITStats bool
	JIT          jitCLIOptions
}

func processExit(err error) (*runtime.ProcessExitError, bool) {
	var exit *runtime.ProcessExitError
	if errors.As(err, &exit) {
		return exit, true
	}
	return nil, false
}

func runScriptFile(interp *runtime.Interpreter, filename string, args []string, opts cliRunOptions) error {
	interp.SetArgs(filename, args)

	// Set script directory for require.
	absPath, _ := filepath.Abs(filename)
	interp.SetScriptDir(filepath.Dir(absPath))

	if opts.UseVM {
		return runFileVM(interp, filename, opts.UseJIT, opts.ShowJITStats, opts.JIT)
	}
	return runFile(interp, filename)
}

func runTestCommand(args []string, opts cliRunOptions, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(errw)
	format := fs.String("format", "text", "output format: text or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	paths := fs.Args()
	if len(paths) != 1 {
		fmt.Fprintln(errw, "usage: gscript test [--format=text|json] <path-or-dir>")
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
		fmt.Fprintf(errw, "gscript test: unsupported --format %q (want text or json)\n", *format)
		return 2
	}
	result := runTestsDetailed(paths[0], opts, errw, *format == "text")
	if *format == "json" {
		if err := json.NewEncoder(outw).Encode(result); err != nil {
			fmt.Fprintf(errw, "gscript test: write json: %v\n", err)
			return 1
		}
	}
	if !result.OK {
		return 1
	}
	return 0
}

type testRunResult struct {
	OK     bool             `json:"ok"`
	Total  int              `json:"total"`
	Passed int              `json:"passed"`
	Failed int              `json:"failed"`
	Files  []testFileResult `json:"files"`
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
	return runTestsDetailed(path, opts, errw, true).OK
}

func runTestsDetailed(path string, opts cliRunOptions, errw io.Writer, text bool) testRunResult {
	files, err := testFiles(path)
	if err != nil {
		if text {
			fmt.Fprintf(errw, "%s: %v\n", path, err)
		}
		return testRunResult{
			OK:     false,
			Total:  1,
			Failed: 1,
			Files:  []testFileResult{{File: path, OK: false, Error: err.Error()}},
		}
	}

	result := testRunResult{
		OK:    true,
		Total: len(files),
		Files: make([]testFileResult, 0, len(files)),
	}
	for _, filename := range files {
		fileResult := testFileResult{File: filename, OK: true}
		golden, hasGolden, err := testGoldenOutputFile(filename)
		if hasGolden {
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

		var runErr error
		var stdout []byte
		if hasGolden {
			stdout, runErr = runScriptFileCapturingStdout(filename, opts)
		} else {
			interp := runtime.New()
			runErr = runScriptFile(interp, filename, nil, opts)
		}
		if runErr != nil {
			if exit, isExit := processExit(runErr); isExit && exit.Code == 0 {
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
		if !hasGolden {
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
	return result
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
		interp := runtime.New()
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
	return gscriptFiles(path)
}

func gscriptFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if filepath.Ext(path) != ".gs" {
			return nil, fmt.Errorf("file must have .gs extension")
		}
		return []string{path}, nil
	}

	var files []string
	err = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(p) == ".gs" {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

func runFile(interp *runtime.Interpreter, filename string) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return runString(interp, string(src))
}

func runString(interp *runtime.Interpreter, src string) error {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}

	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}

	return interp.Exec(prog)
}

func runFileVM(interp *runtime.Interpreter, filename string, jit bool, showJITStats bool, jitOpts jitCLIOptions) error {
	src, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return runStringVMWithSource(interp, string(src), filename, jit, showJITStats, jitOpts)
}

func runStringVM(interp *runtime.Interpreter, src string, jit bool, showJITStats bool, jitOpts jitCLIOptions) error {
	return runStringVMWithSource(interp, src, "", jit, showJITStats, jitOpts)
}

func runStringVMWithSource(interp *runtime.Interpreter, src, sourceName string, jit bool, showJITStats bool, jitOpts jitCLIOptions) error {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return fmt.Errorf("lexer error: %w", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return fmt.Errorf("parse error: %w", err)
	}
	proto, err := bytecodevm.Compile(prog)
	if err != nil {
		return fmt.Errorf("compile error: %w", err)
	}
	setProtoSource(proto, sourceName)
	globals := interp.ExportGlobals()
	bvm := bytecodevm.New(globals)
	bvm.SetScriptDir(interp.ScriptDir())
	if jitOpts.ShowCoroutineStats {
		bvm.EnableCoroutineStats()
	}
	var pathStats *runtime.RuntimePathStats
	if jitOpts.ShowPathStats || jitOpts.ShowPathStatsJSON {
		pathStats = runtime.EnableRuntimePathStats()
		defer runtime.DisableRuntimePathStats()
	}
	var reporter jitStatsReporter
	if !jit && jitOpts.TimelinePath != "" {
		return fmt.Errorf("JIT timeline requires JIT to be enabled")
	}
	if !jit && jitOpts.WarmDumpDir != "" {
		return fmt.Errorf("-jit-dump-warm requires -jit")
	}
	var warmDumper jitWarmDumpController
	if jit {
		reporter, err = cliEnableJIT(bvm, jitOpts)
		if err != nil {
			return err
		}
		if reporter != nil {
			warmDumper, _ = reporter.(jitWarmDumpController)
		}
	}
	if jitOpts.WarmDumpDir != "" {
		if warmDumper == nil {
			return fmt.Errorf("-jit-dump-warm requires the darwin/arm64 method JIT")
		}
		if err := warmDumper.EnableWarmDump(jitOpts.WarmDumpDir, jitOpts.WarmDumpProto); err != nil {
			return err
		}
	}
	_, err = bvm.Execute(proto)
	var dumpErr error
	if jitOpts.WarmDumpDir != "" && warmDumper != nil {
		dumpErr = warmDumper.WriteWarmDump(proto)
	}
	var closeErr error
	if reporter != nil {
		closeErr = reporter.Close()
	}
	if showJITStats {
		if reporter != nil {
			reporter.PrintStats(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "JIT Statistics: JIT disabled or unavailable on this platform")
		}
	}
	var statsErr error
	if jitOpts.ShowExitStats {
		if reporter != nil {
			reporter.PrintExitStats(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "Tier 2 Exit Profile: JIT disabled or unavailable on this platform")
		}
	}
	if jitOpts.ShowExitStatsJSON {
		if reporter != nil {
			statsErr = reporter.PrintExitStatsJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowTier2PerfStats {
		if reporter != nil {
			reporter.PrintTier2PerfStats(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, "Tier 2 Performance Diagnostics: JIT disabled or unavailable on this platform")
		}
	}
	if jitOpts.ShowTier2PerfStatsJSON {
		if reporter != nil {
			statsErr = reporter.PrintTier2PerfStatsJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowTier2SpecStateJSON {
		if reporter != nil {
			statsErr = reporter.PrintTier2SpeculationStateJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowTier2SpecWorklistJSON {
		if reporter != nil {
			statsErr = reporter.PrintTier2SpeculationWorklistJSON(os.Stderr)
		} else {
			fmt.Fprintln(os.Stderr, `{"error":"JIT disabled or unavailable on this platform"}`)
		}
	}
	if jitOpts.ShowCoroutineStats {
		printCoroutineStats(os.Stderr, bvm.CoroutineStats())
	}
	if jitOpts.ShowPathStats && pathStats != nil {
		pathStats.WriteText(os.Stderr)
	}
	if jitOpts.ShowPathStatsJSON && pathStats != nil {
		if err := pathStats.WriteJSON(os.Stderr); err != nil && statsErr == nil {
			statsErr = err
		}
	}
	if err != nil {
		if dumpErr != nil {
			return fmt.Errorf("%w; warm dump failed: %v", err, dumpErr)
		}
		if closeErr != nil {
			return fmt.Errorf("%w; JIT close failed: %v", err, closeErr)
		}
		if statsErr != nil {
			return fmt.Errorf("%w; exit stats failed: %v", err, statsErr)
		}
		return err
	}
	if dumpErr != nil {
		return dumpErr
	}
	if closeErr != nil {
		return closeErr
	}
	if statsErr != nil {
		return statsErr
	}
	return nil
}

func setProtoSource(proto *bytecodevm.FuncProto, sourceName string) {
	if proto == nil || sourceName == "" {
		return
	}
	proto.Source = sourceName
	for _, sub := range proto.Protos {
		setProtoSource(sub, sourceName)
	}
}

func printCoroutineStats(w *os.File, stats bytecodevm.CoroutineStatsSnapshot) {
	fmt.Fprintln(w, "Coroutine Statistics:")
	fmt.Fprintf(w, "  created: %d\n", stats.Created)
	fmt.Fprintf(w, "  wrapped: %d\n", stats.Wrapped)
	fmt.Fprintf(w, "  resumes: %d\n", stats.ResumeCalls)
	fmt.Fprintf(w, "  yields: %d\n", stats.YieldCalls)
	fmt.Fprintf(w, "  leaf fast path: %d\n", stats.LeafFastPath)
	fmt.Fprintf(w, "  leaf fallbacks: %d\n", stats.LeafFallbacks)
	fmt.Fprintf(w, "  wrapped generator fast path: %d\n", stats.WrappedGenerator)
	fmt.Fprintf(w, "  goroutine starts: %d\n", stats.GoroutineStarts)
	fmt.Fprintf(w, "  jit continuations: %d\n", stats.JITContinuations)
	fmt.Fprintf(w, "  jit native resumes: %d\n", stats.JITNativeResumes)
	fmt.Fprintf(w, "  jit native yields: %d\n", stats.JITNativeYields)
	fmt.Fprintf(w, "  jit native misses: %d\n", stats.JITNativeMisses)
	fmt.Fprintf(w, "  completed: %d\n", stats.Completed)
	fmt.Fprintf(w, "  resume errors: %d\n", stats.ResumeErrors)
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
