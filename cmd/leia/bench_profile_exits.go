package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var benchProfileExitDefaults = []string{
	"recursion/fib_recursive",
	"recursion/ackermann",
	"recursion/mutual_recursion",
	"table/sort",
	"table/table_array_access",
	"table/table_field_access",
	"string/string_bench",
	"numeric/matmul",
	"numeric/spectral_norm",
	"recursion/fibonacci_iterative",
}

var benchProfileGroups = []string{"numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control", "precision"}
var benchProfileTimeRE = regexp.MustCompile(`(?m)^Time:\s*([0-9]+(?:\.[0-9]+)?)s\b`)

type benchProfileExitResult struct {
	Benchmark  string         `json:"benchmark"`
	Status     string         `json:"status"`
	Seconds    *float64       `json:"seconds,omitempty"`
	Stats      map[string]any `json:"stats,omitempty"`
	ExitCode   int            `json:"exit_code,omitempty"`
	Error      string         `json:"error,omitempty"`
	OutputTail string         `json:"output_tail,omitempty"`
}

func runBenchProfileExitsCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench profile-exits", flag.ContinueOnError)
	fs.SetOutput(errw)
	benches := benchProfileStringList{}
	mode := fs.String("mode", "default", "execution mode: default or no_filter")
	timeout := fs.Int("timeout", 60, "per-benchmark timeout in seconds")
	jsonPath := fs.String("json", "", "write raw profile JSON")
	markdownPath := fs.String("markdown", "", "write Markdown summary")
	top := fs.Int("top", 20, "number of aggregate reasons and sites to show")
	fs.Var(&benches, "bench", "benchmark name or group/name; repeatable")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *mode != "default" && *mode != "no_filter" {
		fmt.Fprintf(errw, "leia bench profile-exits: unknown mode %q (want default or no_filter)\n", *mode)
		return 2
	}
	if *timeout <= 0 {
		fmt.Fprintln(errw, "leia bench profile-exits: --timeout must be positive")
		return 2
	}
	if *top < 0 {
		fmt.Fprintln(errw, "leia bench profile-exits: --top must be non-negative")
		return 2
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia bench profile-exits: %v\n", err)
		return 1
	}
	selected := []string(benches)
	if len(selected) == 0 {
		selected = append([]string(nil), benchProfileExitDefaults...)
	}

	tempdir, err := os.MkdirTemp("", "leia_exit_profile_")
	if err != nil {
		fmt.Fprintf(errw, "leia bench profile-exits: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tempdir)
	leia := filepath.Join(tempdir, "leia")
	if err := benchProfileBuildCLI(root, leia); err != nil {
		fmt.Fprintf(errw, "leia bench profile-exits: %v\n", err)
		return 1
	}
	results := make([]benchProfileExitResult, 0, len(selected))
	for _, bench := range selected {
		results = append(results, benchProfileRunOne(root, leia, bench, *mode, time.Duration(*timeout)*time.Second))
	}

	payload := map[string]any{"mode": *mode, "benchmarks": selected, "results": results, "summary": benchProfileSummary(results)}
	if *jsonPath != "" {
		if err := benchProfileWriteJSONFile(*jsonPath, payload); err != nil {
			fmt.Fprintf(errw, "leia bench profile-exits: %v\n", err)
			return 1
		}
	}
	report := benchProfileMarkdown(results, *top)
	if *markdownPath != "" {
		if err := os.MkdirAll(filepath.Dir(*markdownPath), 0o755); err != nil {
			fmt.Fprintf(errw, "leia bench profile-exits: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*markdownPath, []byte(report), 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench profile-exits: %v\n", err)
			return 1
		}
		return 0
	}
	fmt.Fprint(outw, report)
	return 0
}

type benchProfileStringList []string

func (f *benchProfileStringList) String() string { return strings.Join(*f, ",") }

func (f *benchProfileStringList) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func benchProfileBuildCLI(root, out string) error {
	cmd := benchExecCommand("go", "build", "-o", out, "./cmd/leia")
	cmd.Dir = root
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build CLI: %w\n%s", err, output.String())
	}
	return nil
}

func benchProfileRunOne(root, leia, bench, mode string, timeout time.Duration) benchProfileExitResult {
	id, path, ok := benchProfileResolveBenchmark(root, bench)
	if !ok {
		return benchProfileExitResult{Benchmark: bench, Status: "missing"}
	}
	cmd := benchExecCommand(leia, "-jit", "-jit-stats", "-exit-stats-json", path)
	cmd.Dir = root
	env := os.Environ()
	if cmd.Env != nil {
		env = append([]string(nil), cmd.Env...)
	}
	if mode == "no_filter" {
		env = append(env, "LEIA_TIER2_NO_FILTER=1")
	}
	cmd.Env = env
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := benchProfileRunCommand(cmd, timeout); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return benchProfileExitResult{Benchmark: id, Status: "timeout", OutputTail: tailString(output.String(), 1000)}
		}
		exitCode := 1
		type exitCoder interface{ ExitCode() int }
		var ec exitCoder
		if errors.As(err, &ec) {
			exitCode = ec.ExitCode()
		}
		return benchProfileExitResult{Benchmark: id, Status: "error", ExitCode: exitCode, OutputTail: tailString(output.String(), 1000)}
	}
	stats, err := benchProfileExtractExitJSON(output.String())
	if err != nil {
		return benchProfileExitResult{Benchmark: id, Status: "parse_error", Error: err.Error(), OutputTail: tailString(output.String(), 1000)}
	}
	seconds := benchProfileParseTime(output.String())
	return benchProfileExitResult{Benchmark: id, Status: "ok", Seconds: seconds, Stats: stats}
}

func benchProfileRunCommand(cmd *exec.Cmd, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		return ctx.Err()
	}
}

func benchProfileWriteJSONFile(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func benchProfileResolveBenchmark(root, bench string) (string, string, bool) {
	groups := benchProfileGroups
	name := bench
	if strings.Contains(bench, "/") {
		parts := strings.SplitN(bench, "/", 2)
		groups = []string{parts[0]}
		name = parts[1]
	}
	for _, group := range groups {
		path := filepath.Join(root, "benchmarks", group, name+".leia")
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return group + "/" + name, path, true
		}
	}
	return "", "", false
}

func benchProfileExtractExitJSON(output string) (map[string]any, error) {
	marker := "{\n  \"total\":"
	start := strings.LastIndex(output, marker)
	if start < 0 {
		return nil, errors.New("no exit-stats JSON object found")
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(output[start:]), &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func benchProfileParseTime(output string) *float64 {
	match := benchProfileTimeRE.FindStringSubmatch(output)
	if match == nil {
		return nil
	}
	value, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return nil
	}
	return &value
}

func benchProfileMarkdown(results []benchProfileExitResult, top int) string {
	byCode := map[string]int{}
	byReason := map[string]int{}
	var sites []benchProfileSiteRow
	var b strings.Builder
	b.WriteString("# Tier 2 Exit Profile\n\n")
	b.WriteString("| Benchmark | Time | Total exits | By exit code |\n")
	b.WriteString("|---|---:|---:|---|\n")
	for _, row := range results {
		if row.Status != "ok" {
			fmt.Fprintf(&b, "| %s | %s | - | - |\n", row.Benchmark, row.Status)
			continue
		}
		for name, count := range benchProfileAnyMap(row.Stats["by_exit_code"]) {
			byCode[name] += benchDebugToInt(count)
		}
		for _, siteAny := range benchProfileAnySlice(row.Stats["sites"]) {
			site := benchProfileAnyMap(siteAny)
			count := benchDebugToInt(site["count"])
			reason := benchDebugStringDefault(site["reason"], "")
			byReason[reason] += count
			sites = append(sites, benchProfileSiteRow{
				Count:     count,
				Benchmark: row.Benchmark,
				Proto:     benchDebugStringDefault(site["proto"], ""),
				ExitName:  benchDebugStringDefault(site["exit_name"], ""),
				PC:        benchDebugToInt(site["pc"]),
				OpID:      benchDebugToInt(site["op_id"]),
				Reason:    reason,
			})
		}
		seconds := "-"
		if row.Seconds != nil {
			seconds = fmt.Sprintf("%.3fs", *row.Seconds)
		}
		fmt.Fprintf(&b, "| %s | %s | %d | %s |\n", row.Benchmark, seconds, benchDebugToInt(row.Stats["total"]), benchProfileCodeText(row.Stats["by_exit_code"]))
	}
	b.WriteString("\n## Aggregate By Exit Code\n\n")
	if len(byCode) == 0 {
		b.WriteString("_No exit-code data collected._\n")
	} else {
		b.WriteString("| Exit code | Count |\n")
		b.WriteString("|---|---:|\n")
		for _, item := range benchProfileSortedCounts(byCode, 0) {
			fmt.Fprintf(&b, "| %s | %d |\n", item.Name, item.Count)
		}
	}
	b.WriteString("\n## Aggregate By Reason\n\n")
	if len(byReason) == 0 {
		b.WriteString("_No exit reason data collected._\n")
	} else {
		b.WriteString("| Reason | Count |\n")
		b.WriteString("|---|---:|\n")
		for _, item := range benchProfileSortedCounts(byReason, top) {
			fmt.Fprintf(&b, "| %s | %d |\n", item.Name, item.Count)
		}
	}
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].Count == sites[j].Count {
			return sites[i].Benchmark > sites[j].Benchmark
		}
		return sites[i].Count > sites[j].Count
	})
	b.WriteString("\n## Top Sites\n\n")
	if len(sites) == 0 || top == 0 {
		b.WriteString("_No exit sites collected._\n")
	} else {
		b.WriteString("| Count | Benchmark | Proto | Exit | PC | OpID | Reason |\n")
		b.WriteString("|---:|---|---|---|---:|---:|---|\n")
		for i, site := range sites {
			if i >= top {
				break
			}
			fmt.Fprintf(&b, "| %d | %s | %s | %s | %d | %d | %s |\n", site.Count, site.Benchmark, site.Proto, site.ExitName, site.PC, site.OpID, site.Reason)
		}
	}
	return b.String()
}

func benchProfileSummary(results []benchProfileExitResult) map[string]any {
	statuses := map[string]int{}
	totalExits := 0
	okRows := 0
	for _, row := range results {
		statuses[row.Status]++
		if row.Status != "ok" {
			continue
		}
		okRows++
		totalExits += benchDebugToInt(row.Stats["total"])
	}
	return map[string]any{
		"benchmarks":  len(results),
		"ok":          okRows,
		"total_exits": totalExits,
		"statuses":    benchDebugSortedIntMap(statuses),
	}
}

type benchProfileSiteRow struct {
	Count     int
	Benchmark string
	Proto     string
	ExitName  string
	PC        int
	OpID      int
	Reason    string
}

type benchProfileCount struct {
	Name  string
	Count int
}

func benchProfileSortedCounts(counts map[string]int, limit int) []benchProfileCount {
	items := make([]benchProfileCount, 0, len(counts))
	for name, count := range counts {
		items = append(items, benchProfileCount{Name: name, Count: count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].Name < items[j].Name
		}
		return items[i].Count > items[j].Count
	})
	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func benchProfileCodeText(value any) string {
	codes := benchProfileAnyMap(value)
	if len(codes) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(codes))
	for key := range codes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, benchDebugToInt(codes[key])))
	}
	return strings.Join(parts, ", ")
}

func benchProfileAnyMap(value any) map[string]any {
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func benchProfileAnySlice(value any) []any {
	if out, ok := value.([]any); ok {
		return out
	}
	return nil
}

func tailString(value string, max int) string {
	if len(value) <= max {
		return value
	}
	return value[len(value)-max:]
}
