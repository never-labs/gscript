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

type benchSubmitRow struct {
	Name          string
	Current       *float64
	LuaJIT        *float64
	CurrentStatus string
	LuaJITStatus  string
	CurrentSource string
	LuaJITSource  string
}

type benchSubmitViolation struct {
	Kind  string
	Name  string
	Value string
	Limit string
}

func runBenchSubmitGuardCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench submit-guard", flag.ContinueOnError)
	fs.SetOutput(errw)
	baselinePath := fs.String("baseline", "", "previous accepted timing_compare JSON")
	mode := fs.String("mode", "default", "timing mode")
	ratioThreshold := fs.Float64("ratio-threshold", 0.8, "maximum current/LuaJIT ratio")
	regressionTolerance := fs.Float64("regression-tolerance", 0.03, "maximum current/baseline regression")
	candidatePath := ""
	parseArgs := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		candidatePath = args[0]
		parseArgs = args[1:]
	}
	if err := fs.Parse(parseArgs); err != nil {
		return 2
	}
	if candidatePath == "" && fs.NArg() == 1 {
		candidatePath = fs.Arg(0)
	}
	if candidatePath == "" || fs.NArg() > 1 {
		fmt.Fprintln(errw, "usage: leia bench submit-guard <candidate.json> [--baseline FILE] [--mode MODE] [--ratio-threshold N] [--regression-tolerance N]")
		return 2
	}
	candidate, err := benchSubmitLoadRows(candidatePath, *mode)
	if err != nil {
		fmt.Fprintf(errw, "leia bench submit-guard: %v\n", err)
		return 1
	}
	var baseline map[string]benchSubmitRow
	if *baselinePath != "" {
		baseline, err = benchSubmitLoadRows(*baselinePath, *mode)
		if err != nil {
			fmt.Fprintf(errw, "leia bench submit-guard: %v\n", err)
			return 1
		}
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia bench submit-guard: %v\n", err)
		return 1
	}
	required := benchSubmitLuaJITRequired(filepath.Join(root, "benchmarks", "manifest.json"))
	violations := benchSubmitCheckRows(candidate, baseline, required, *ratioThreshold, *regressionTolerance)
	fmt.Fprint(outw, benchSubmitSummary(candidate, violations))
	if len(violations) > 0 {
		return 1
	}
	return 0
}

func benchSubmitLoadRows(path, mode string) (map[string]benchSubmitRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, err
	}
	rawRows, ok := payload["results"].([]any)
	if !ok {
		return nil, fmt.Errorf("performance JSON must contain a list-valued 'results'")
	}
	rows := map[string]benchSubmitRow{}
	for _, raw := range rawRows {
		rowMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		row, ok := benchSubmitTimingCompareRow(rowMap, mode)
		if !ok {
			row, ok = benchSubmitFlatGuardRow(rowMap)
		}
		if ok && row.Name != "" {
			rows[row.Name] = row
		}
	}
	return rows, nil
}

func benchSubmitTimingCompareRow(row map[string]any, mode string) (benchSubmitRow, bool) {
	modes, ok := row["modes"].(map[string]any)
	if !ok {
		return benchSubmitRow{}, false
	}
	modeRow, ok := modes[mode].(map[string]any)
	if !ok {
		return benchSubmitRow{}, false
	}
	current, _ := modeRow["current"].(map[string]any)
	luajit, _ := modeRow["luajit"].(map[string]any)
	group := benchDebugStringDefault(row["group"], "")
	bench := benchDebugStringDefault(row["benchmark"], "")
	name := bench
	if group != "" && !strings.HasPrefix(bench, group+"/") {
		name = group + "/" + bench
	}
	return benchSubmitRow{
		Name:          name,
		Current:       benchSubmitSubjectSeconds(current),
		LuaJIT:        benchSubmitSubjectSeconds(luajit),
		CurrentStatus: benchDebugStringDefault(current["status"], "missing"),
		LuaJITStatus:  benchDebugStringDefault(luajit["status"], "missing"),
		CurrentSource: benchDebugStringDefault(current["source"], ""),
		LuaJITSource:  benchDebugStringDefault(luajit["source"], ""),
	}, true
}

func benchSubmitFlatGuardRow(row map[string]any) (benchSubmitRow, bool) {
	bench, ok := row["benchmark"].(string)
	if !ok {
		return benchSubmitRow{}, false
	}
	current, _ := row["default"].(map[string]any)
	luajit, _ := row["luajit"].(map[string]any)
	return benchSubmitRow{
		Name:          bench,
		Current:       benchSubmitSubjectSeconds(current),
		LuaJIT:        benchSubmitSubjectSeconds(luajit),
		CurrentStatus: benchDebugStringDefault(current["status"], "missing"),
		LuaJITStatus:  benchDebugStringDefault(luajit["status"], "missing"),
		CurrentSource: benchDebugStringDefault(current["source"], ""),
		LuaJITSource:  benchDebugStringDefault(luajit["source"], ""),
	}, true
}

func benchSubmitSubjectSeconds(subject map[string]any) *float64 {
	if stats, ok := subject["stats"].(map[string]any); ok {
		if n := benchDebugNumber(stats["median"]); n != nil {
			return n
		}
	}
	return benchDebugNumber(subject["seconds"])
}

func benchSubmitLuaJITRequired(manifestPath string) map[string]bool {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil
	}
	required := map[string]bool{}
	for _, item := range benchProfileAnySlice(payload["workloads"]) {
		workload := benchProfileAnyMap(item)
		id, ok := workload["id"].(string)
		if ok && workload["comparison_reference"] != nil {
			required[id] = true
		}
	}
	return required
}

func benchSubmitCheckRows(candidate, baseline map[string]benchSubmitRow, required map[string]bool, ratioThreshold, regressionTolerance float64) []benchSubmitViolation {
	violations := make([]benchSubmitViolation, 0)
	names := make([]string, 0, len(candidate))
	for name := range candidate {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		row := candidate[name]
		requireLuaJIT := required == nil || required[name]
		if requireLuaJIT && !row.hasTimedLuaJITPair() {
			violations = append(violations, benchSubmitViolation{"missing", name, fmt.Sprintf("current=%s luajit=%s", row.CurrentStatus, row.LuaJITStatus), "timed current+luajit"})
		} else if requireLuaJIT && row.hasComparableLuaJITTiming() {
			if ratio := row.ratio(); ratio != nil && *ratio > ratioThreshold {
				violations = append(violations, benchSubmitViolation{"luajit", name, fmt.Sprintf("%.3fx", *ratio), fmt.Sprintf("<=%.3fx", ratioThreshold)})
			}
		}
		if baseline == nil {
			continue
		}
		base, ok := baseline[name]
		if !ok || row.Current == nil || base.Current == nil || *base.Current <= 0 || !row.hasComparableCurrentTiming(base) {
			continue
		}
		change := *row.Current / *base.Current - 1
		if change > regressionTolerance {
			violations = append(violations, benchSubmitViolation{"regression", name, fmt.Sprintf("+%.2f%%", change*100), fmt.Sprintf("<=%.2f%%", regressionTolerance*100)})
		}
	}
	return violations
}

func benchSubmitSummary(rows map[string]benchSubmitRow, violations []benchSubmitViolation) string {
	worst := make([]benchSubmitRow, 0, len(rows))
	for _, row := range rows {
		if row.hasComparableLuaJITTiming() && row.ratio() != nil {
			worst = append(worst, row)
		}
	}
	sort.SliceStable(worst, func(i, j int) bool {
		return *worst[i].ratio() > *worst[j].ratio()
	})
	if len(worst) > 12 {
		worst = worst[:12]
	}
	var b strings.Builder
	b.WriteString("Worst current/LuaJIT ratios:\n")
	b.WriteString("Benchmark                          Current     LuaJIT    Cur/LJ\n")
	b.WriteString("----------------------------------------------------------------\n")
	for _, row := range worst {
		fmt.Fprintf(&b, "%-34s %8.6fs %8.6fs %7.3fx\n", row.Name, *row.Current, *row.LuaJIT, *row.ratio())
	}
	if len(violations) > 0 {
		b.WriteString("\nGuard violations:\n")
		b.WriteString("Kind        Benchmark                          Value       Limit\n")
		b.WriteString("----------------------------------------------------------------\n")
		for _, item := range violations {
			fmt.Fprintf(&b, "%-11s %-34s %10s %10s\n", item.Kind, item.Name, item.Value, item.Limit)
		}
	} else {
		b.WriteString("\nGuard passed.\n")
	}
	return b.String()
}

func (r benchSubmitRow) ratio() *float64 {
	if !r.hasComparableLuaJITTiming() {
		return nil
	}
	value := *r.Current / *r.LuaJIT
	return &value
}

func (r benchSubmitRow) hasTimedLuaJITPair() bool {
	return r.Current != nil && r.LuaJIT != nil && *r.LuaJIT > 0
}

func (r benchSubmitRow) hasComparableLuaJITTiming() bool {
	if !r.hasTimedLuaJITPair() {
		return false
	}
	return r.CurrentSource == "" || r.LuaJITSource == "" || r.CurrentSource == r.LuaJITSource
}

func (r benchSubmitRow) hasComparableCurrentTiming(other benchSubmitRow) bool {
	if r.Current == nil || other.Current == nil {
		return false
	}
	return r.CurrentSource == "" || other.CurrentSource == "" || r.CurrentSource == other.CurrentSource
}
