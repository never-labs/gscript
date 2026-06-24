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
)

type benchAuditRow struct {
	Name           string
	DefaultSeconds *float64
	LuaJITSeconds  *float64
	LuaJITStatus   string
	ExitTotal      int
}

func runBenchAuditCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench audit", flag.ContinueOnError)
	fs.SetOutput(errw)
	markdown := fs.String("markdown", "", "write markdown report to this path")
	lowResolutionCutoff := fs.Float64("low-resolution-cutoff", 0.001, "flag default timings at or below this cutoff")
	exitCutoff := fs.Int("exit-cutoff", 20, "flag default-mode exit totals at or above this cutoff")
	parseArgs, jsonFile := normalizeBenchAuditArgs(args)
	if err := fs.Parse(parseArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 || jsonFile == "" {
		fmt.Fprintln(errw, "usage: leia bench audit <guard.json> [--markdown FILE] [--low-resolution-cutoff N] [--exit-cutoff N]")
		return 2
	}
	rows, err := loadBenchAuditRows(jsonFile)
	if err != nil {
		fmt.Fprintf(errw, "leia bench audit: %v\n", err)
		return 1
	}
	report := benchAuditMarkdown(rows, *lowResolutionCutoff, *exitCutoff)
	if *markdown != "" {
		if err := os.MkdirAll(filepath.Dir(*markdown), 0o755); err != nil && filepath.Dir(*markdown) != "." {
			fmt.Fprintf(errw, "leia bench audit: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*markdown, []byte(report), 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench audit: %v\n", err)
			return 1
		}
		return 0
	}
	if _, err := io.WriteString(outw, report); err != nil {
		fmt.Fprintf(errw, "leia bench audit: %v\n", err)
		return 1
	}
	return 0
}

func normalizeBenchAuditArgs(args []string) ([]string, string) {
	parseArgs := make([]string, 0, len(args))
	jsonFile := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			parseArgs = append(parseArgs, arg)
			if arg != "-h" && arg != "--help" && !strings.Contains(arg, "=") && i+1 < len(args) {
				parseArgs = append(parseArgs, args[i+1])
				i++
			}
			continue
		}
		if jsonFile == "" {
			jsonFile = arg
			continue
		}
		parseArgs = append(parseArgs, arg)
	}
	return parseArgs, jsonFile
}

func loadBenchAuditRows(path string) ([]benchAuditRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	results, ok := payload["results"].([]any)
	if !ok {
		return nil, fmt.Errorf("guard JSON must contain a list-valued 'results'")
	}
	rows := make([]benchAuditRow, 0, len(results))
	for _, item := range results {
		rowMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		defaultMode := benchAuditSubject(rowMap, "default", "current")
		luajitMode := benchAuditSubject(rowMap, "default", "luajit")
		rows = append(rows, benchAuditRow{
			Name:           benchAuditName(rowMap),
			DefaultSeconds: benchAuditSeconds(defaultMode),
			LuaJITSeconds:  benchAuditSeconds(luajitMode),
			LuaJITStatus:   benchAuditStringDefault(luajitMode["status"], "missing"),
			ExitTotal:      benchAuditInt(defaultMode["exit_total"]),
		})
	}
	return rows, nil
}

func benchAuditMarkdown(rows []benchAuditRow, lowResolutionCutoff float64, exitCutoff int) string {
	confirmed := make([]benchAuditRow, 0)
	unresolved := make([]benchAuditRow, 0)
	lowResolution := make([]benchAuditRow, 0)
	exitHeavy := make([]benchAuditRow, 0)
	for _, row := range rows {
		if ratio, ok := row.JITLuaJITRatio(); ok && ratio < 1.0 {
			confirmed = append(confirmed, row)
		}
		if !row.HasLuaJITTime() {
			unresolved = append(unresolved, row)
		}
		if row.DefaultSeconds != nil && *row.DefaultSeconds <= lowResolutionCutoff {
			lowResolution = append(lowResolution, row)
		}
		if row.ExitTotal >= exitCutoff {
			exitHeavy = append(exitHeavy, row)
		}
	}
	sort.SliceStable(confirmed, func(i, j int) bool {
		left, _ := confirmed[i].JITLuaJITRatio()
		right, _ := confirmed[j].JITLuaJITRatio()
		return left < right
	})
	sort.SliceStable(exitHeavy, func(i, j int) bool {
		return exitHeavy[i].ExitTotal > exitHeavy[j].ExitTotal
	})

	var b bytes.Buffer
	fmt.Fprintln(&b, "# Benchmark Audit")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Confirmed LuaJIT Comparisons")
	if len(confirmed) == 0 {
		fmt.Fprintln(&b, "_No benchmark has a parseable LuaJIT timing where JIT is faster._")
	} else {
		fmt.Fprintln(&b, "| Benchmark | JIT | LuaJIT | JIT/LuaJIT |")
		fmt.Fprintln(&b, "|---|---:|---:|---:|")
		for _, row := range confirmed {
			ratio, _ := row.JITLuaJITRatio()
			fmt.Fprintf(&b, "| %s | %.3fs | %.3fs | %.2fx |\n", row.Name, *row.DefaultSeconds, *row.LuaJITSeconds, ratio)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Unresolved Comparisons")
	if len(unresolved) == 0 {
		fmt.Fprintln(&b, "_Every benchmark has a parseable LuaJIT timing._")
	} else {
		fmt.Fprintln(&b, "| Benchmark | LuaJIT status | JIT |")
		fmt.Fprintln(&b, "|---|---:|---:|")
		for _, row := range unresolved {
			jit := "-"
			if row.DefaultSeconds != nil {
				jit = fmt.Sprintf("%.3fs", *row.DefaultSeconds)
			}
			fmt.Fprintf(&b, "| %s | %s | %s |\n", row.Name, row.LuaJITStatus, jit)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Low-Resolution Measurements")
	if len(lowResolution) == 0 {
		fmt.Fprintln(&b, "_No default JIT result is below the low-resolution cutoff._")
	} else {
		fmt.Fprintln(&b, "| Benchmark | JIT | Reason |")
		fmt.Fprintln(&b, "|---|---:|---|")
		for _, row := range lowResolution {
			fmt.Fprintf(&b, "| %s | %.3fs | Needs calibrated repeats or ns/op bench |\n", row.Name, *row.DefaultSeconds)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Exit-Heavy Benchmarks")
	if len(exitHeavy) == 0 {
		fmt.Fprintln(&b, "_No benchmark exceeded the exit cutoff._")
	} else {
		fmt.Fprintln(&b, "| Benchmark | Exits |")
		fmt.Fprintln(&b, "|---|---:|")
		for _, row := range exitHeavy {
			fmt.Fprintf(&b, "| %s | %d |\n", row.Name, row.ExitTotal)
		}
	}
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Recommended Next Tests")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "- Add or fix LuaJIT references for every unresolved comparison.")
	fmt.Fprintln(&b, "- Use high-repeat or ns/op measurement for low-resolution rows before claiming a win.")
	fmt.Fprintln(&b, "- Add renamed and parameter-varied related workloads for benchmarks accelerated by whole-call kernels.")
	fmt.Fprintln(&b, "- Investigate exit-heavy rows even when wall time is already competitive.")
	return b.String()
}

func (r benchAuditRow) HasLuaJITTime() bool {
	return r.LuaJITSeconds != nil && *r.LuaJITSeconds > 0
}

func (r benchAuditRow) JITLuaJITRatio() (float64, bool) {
	if r.DefaultSeconds == nil || !r.HasLuaJITTime() {
		return 0, false
	}
	return *r.DefaultSeconds / *r.LuaJITSeconds, true
}

func benchAuditMode(row map[string]any, name string) map[string]any {
	mode, _ := row[name].(map[string]any)
	if mode == nil {
		return map[string]any{}
	}
	return mode
}

func benchAuditSubject(row map[string]any, mode, subject string) map[string]any {
	if modes, ok := row["modes"].(map[string]any); ok {
		if modeRow, ok := modes[mode].(map[string]any); ok {
			if subjectRow, ok := modeRow[subject].(map[string]any); ok {
				return subjectRow
			}
		}
	}
	if subject == "current" {
		return benchAuditMode(row, mode)
	}
	return benchAuditMode(row, subject)
}

func benchAuditName(row map[string]any) string {
	bench := fmt.Sprint(row["benchmark"])
	group := benchAuditStringDefault(row["group"], "")
	if group != "" && !strings.HasPrefix(bench, group+"/") {
		return group + "/" + bench
	}
	return bench
}

func benchAuditSeconds(row map[string]any) *float64 {
	if stats := benchAuditMode(row, "stats"); len(stats) > 0 {
		if seconds := benchAuditFloatPtr(stats["median"]); seconds != nil {
			return seconds
		}
	}
	return benchAuditFloatPtr(row["seconds"])
}

func benchAuditFloatPtr(value any) *float64 {
	if v, ok := value.(float64); ok {
		return &v
	}
	return nil
}

func benchAuditInt(value any) int {
	if v, ok := value.(float64); ok {
		return int(v)
	}
	return 0
}

func benchAuditStringDefault(value any, fallback string) string {
	if v, ok := value.(string); ok {
		return v
	}
	return fallback
}
