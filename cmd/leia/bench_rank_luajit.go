package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type benchLuaJITGapRow struct {
	Benchmark           string
	VMSeconds           *float64
	DefaultSeconds      *float64
	NoFilterSeconds     *float64
	LuaJITSeconds       *float64
	DefaultLuaJITRatio  float64
	NoFilterLuaJITRatio *float64
	JITVMSpeedup        *float64
	T2Attempted         int
	T2Entered           int
	T2Failed            int
	ExitTotal           int
	VMStatus            string
	DefaultStatus       string
	NoFilterStatus      string
	LuaJITStatus        string
}

func runBenchRankLuaJITGapsCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench rank-luajit-gaps", flag.ContinueOnError)
	fs.SetOutput(errw)
	top := fs.Int("top", 0, "limit rows; 0 means all")
	format := fs.String("format", "markdown", "output format: markdown or csv")
	parseArgs, jsonFile := normalizeBenchPositionalArgs(args, map[string]bool{"-h": false, "--help": false, "--top": true, "--format": true})
	if err := fs.Parse(parseArgs); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 || jsonFile == "" {
		fmt.Fprintln(errw, "usage: leia bench rank-luajit-gaps <regression.json> [--top N] [--format markdown|csv]")
		return 2
	}
	if *format != "markdown" && *format != "csv" {
		fmt.Fprintf(errw, "leia bench rank-luajit-gaps: unknown format %q (want markdown or csv)\n", *format)
		return 2
	}
	rows, err := loadBenchLuaJITGapRows(jsonFile)
	if err != nil {
		fmt.Fprintf(errw, "leia bench rank-luajit-gaps: %v\n", err)
		return 1
	}
	if *top > 0 && *top < len(rows) {
		rows = rows[:*top]
	}
	var output string
	if *format == "csv" {
		output = benchLuaJITGapCSV(rows)
	} else {
		output = benchLuaJITGapMarkdown(rows)
	}
	if _, err := io.WriteString(outw, output); err != nil {
		fmt.Fprintf(errw, "leia bench rank-luajit-gaps: %v\n", err)
		return 1
	}
	return 0
}

func loadBenchLuaJITGapRows(path string) ([]benchLuaJITGapRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	results, _ := payload["results"].([]any)
	rows := make([]benchLuaJITGapRow, 0, len(results))
	for _, item := range results {
		result, ok := item.(map[string]any)
		if !ok {
			continue
		}
		defaultMode := benchAuditMode(result, "default")
		luajitMode := benchAuditMode(result, "luajit")
		defaultSeconds := benchPositiveFloatPtr(defaultMode["seconds"])
		luajitSeconds := benchPositiveFloatPtr(luajitMode["seconds"])
		if defaultSeconds == nil || luajitSeconds == nil {
			continue
		}
		vmMode := benchAuditMode(result, "vm")
		noFilterMode := benchAuditMode(result, "no_filter")
		vmSeconds := benchPositiveFloatPtr(vmMode["seconds"])
		noFilterSeconds := benchPositiveFloatPtr(noFilterMode["seconds"])
		row := benchLuaJITGapRow{
			Benchmark:          fmt.Sprint(result["benchmark"]),
			VMSeconds:          vmSeconds,
			DefaultSeconds:     defaultSeconds,
			NoFilterSeconds:    noFilterSeconds,
			LuaJITSeconds:      luajitSeconds,
			DefaultLuaJITRatio: *defaultSeconds / *luajitSeconds,
			T2Attempted:        benchAuditInt(defaultMode["t2_attempted"]),
			T2Entered:          benchAuditInt(defaultMode["t2_entered"]),
			T2Failed:           benchAuditInt(defaultMode["t2_failed"]),
			ExitTotal:          benchAuditInt(defaultMode["exit_total"]),
			VMStatus:           benchAuditStringDefault(vmMode["status"], "missing"),
			DefaultStatus:      benchAuditStringDefault(defaultMode["status"], "missing"),
			NoFilterStatus:     benchAuditStringDefault(noFilterMode["status"], "missing"),
			LuaJITStatus:       benchAuditStringDefault(luajitMode["status"], "missing"),
		}
		if noFilterSeconds != nil {
			value := *noFilterSeconds / *luajitSeconds
			row.NoFilterLuaJITRatio = &value
		}
		if vmSeconds != nil {
			value := *vmSeconds / *defaultSeconds
			row.JITVMSpeedup = &value
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].DefaultLuaJITRatio > rows[j].DefaultLuaJITRatio
	})
	return rows, nil
}

func benchLuaJITGapMarkdown(rows []benchLuaJITGapRow) string {
	var b bytes.Buffer
	fmt.Fprintln(&b, "| Rank | Benchmark | VM | Default | NoFilter | LuaJIT | Default/LuaJIT | NoFilter/LuaJIT | JIT/VM | T2 a/e/f | Exits |")
	fmt.Fprintln(&b, "|---:|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|")
	for i, row := range rows {
		t2 := fmt.Sprintf("%d/%d/%d", row.T2Attempted, row.T2Entered, row.T2Failed)
		fmt.Fprintf(&b, "| %d | %s | %s | %s | %s | %s | %s | %s | %s | %s | %d |\n",
			i+1,
			row.Benchmark,
			benchFormatSeconds(row.VMSeconds, row.VMStatus),
			benchFormatSeconds(row.DefaultSeconds, row.DefaultStatus),
			benchFormatSeconds(row.NoFilterSeconds, row.NoFilterStatus),
			benchFormatSeconds(row.LuaJITSeconds, row.LuaJITStatus),
			benchFormatRatio(&row.DefaultLuaJITRatio),
			benchFormatRatio(row.NoFilterLuaJITRatio),
			benchFormatRatio(row.JITVMSpeedup),
			t2,
			row.ExitTotal,
		)
	}
	return b.String()
}

func benchLuaJITGapCSV(rows []benchLuaJITGapRow) string {
	var b bytes.Buffer
	writer := csv.NewWriter(&b)
	fields := []string{"rank", "benchmark", "vm_seconds", "default_seconds", "no_filter_seconds", "luajit_seconds", "default_luajit_ratio", "no_filter_luajit_ratio", "jit_vm_speedup", "t2_attempted", "t2_entered", "t2_failed", "exit_total"}
	_ = writer.Write(fields)
	for i, row := range rows {
		_ = writer.Write([]string{
			fmt.Sprint(i + 1),
			row.Benchmark,
			benchCSVFloat(row.VMSeconds),
			benchCSVFloat(row.DefaultSeconds),
			benchCSVFloat(row.NoFilterSeconds),
			benchCSVFloat(row.LuaJITSeconds),
			fmt.Sprint(row.DefaultLuaJITRatio),
			benchCSVFloat(row.NoFilterLuaJITRatio),
			benchCSVFloat(row.JITVMSpeedup),
			fmt.Sprint(row.T2Attempted),
			fmt.Sprint(row.T2Entered),
			fmt.Sprint(row.T2Failed),
			fmt.Sprint(row.ExitTotal),
		})
	}
	writer.Flush()
	return b.String()
}

func normalizeBenchPositionalArgs(args []string, takesValue map[string]bool) ([]string, string) {
	parseArgs := make([]string, 0, len(args))
	jsonFile := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			parseArgs = append(parseArgs, arg)
			if takesValue[arg] && !strings.Contains(arg, "=") && i+1 < len(args) {
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

func benchPositiveFloatPtr(value any) *float64 {
	if v, ok := value.(float64); ok && v > 0 {
		return &v
	}
	return nil
}

func benchFormatSeconds(value *float64, status string) string {
	if value == nil {
		return status
	}
	return fmt.Sprintf("%.3fs", *value)
}

func benchFormatRatio(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2fx", *value)
}

func benchCSVFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(*value)
}
