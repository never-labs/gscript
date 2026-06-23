package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type diagSummaryBench struct {
	Label  string
	Domain string
	Name   string
	Dir    string
}

type diagStatsFile struct {
	Protos []diagProtoStats `json:"protos"`
}

type diagProtoStats struct {
	Name          string         `json:"name"`
	SkipReason    string         `json:"skip_reason"`
	InsnCount     int            `json:"insn_count"`
	CodeBytes     int            `json:"code_bytes"`
	InsnHistogram map[string]int `json:"insn_histogram"`
}

func runDiagSummaryCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("diag summary", flag.ContinueOnError)
	fs.SetOutput(errw)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 1 {
		fmt.Fprintln(errw, "usage: leia diag summary <diag-root>")
		return 2
	}
	text, err := buildDiagSummary(fs.Args()[0])
	if err != nil {
		fmt.Fprintf(errw, "leia diag summary: %v\n", err)
		return 1
	}
	fmt.Fprint(outw, text)
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprintln(outw)
	}
	return 0
}

func buildDiagSummary(diagRoot string) (string, error) {
	info, err := os.Stat(diagRoot)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", diagRoot)
	}
	benches, err := discoverDiagSummaryBenches(diagRoot)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	fmt.Fprintln(&b, "# diag/summary.md")
	fmt.Fprintln(&b)
	fmt.Fprintf(&b, "Generated across %d benchmarks.\n", len(benches))
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Insn counts (hottest proto per benchmark)")
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "| Benchmark | Domain | Hottest proto | Insns | Bytes | Load | Store | FP | Branch |")
	fmt.Fprintln(&b, "|-----------|-------|---------------|------:|------:|-----:|------:|---:|-------:|")

	hottest := map[string]diagProtoStats{}
	for _, bench := range benches {
		stats, err := readDiagStats(filepath.Join(bench.Dir, "stats.json"))
		if err != nil {
			return "", err
		}
		hot, ok := hottestProto(stats.Protos)
		if !ok {
			continue
		}
		hottest[bench.Label] = hot
		hist := hot.InsnHistogram
		fmt.Fprintf(
			&b,
			"| %s | %s | %s | %d | %d | %d | %d | %d | %d |\n",
			bench.Name,
			bench.Domain,
			emptyAsQuestion(hot.Name),
			hot.InsnCount,
			hot.CodeBytes,
			hist["load"],
			hist["store"],
			hist["fp"],
			hist["branch"],
		)
	}

	writeDiagDriftSummary(&b)
	fmt.Fprintln(&b)
	fmt.Fprintln(&b, "## Histogram anomalies")
	fmt.Fprintln(&b)
	anomalies := make([]string, 0)
	labels := make([]string, 0, len(hottest))
	for label := range hottest {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		hot := hottest[label]
		hist := hot.InsnHistogram
		loads := hist["load"]
		stores := hist["store"]
		total := hot.InsnCount
		if total == 0 {
			total = 1
		}
		memPct := 100 * float64(loads+stores) / float64(total)
		if memPct > 50 {
			anomalies = append(anomalies, fmt.Sprintf(
				"- **%s/%s**: %d+%d=%d memory ops out of %d insns (%.0f%%) — dominant store/load, suspect allocation/field access.",
				label,
				emptyAsQuestion(hot.Name),
				loads,
				stores,
				loads+stores,
				total,
				memPct,
			))
		}
	}
	if len(anomalies) == 0 {
		fmt.Fprintln(&b, "_no benchmarks above 50% memory-op threshold._")
	} else {
		for _, line := range anomalies {
			fmt.Fprintln(&b, line)
		}
	}
	return b.String(), nil
}

func discoverDiagSummaryBenches(diagRoot string) ([]diagSummaryBench, error) {
	entries, err := os.ReadDir(diagRoot)
	if err != nil {
		return nil, err
	}
	var benches []diagSummaryBench
	for _, top := range entries {
		if !top.IsDir() {
			continue
		}
		topDir := filepath.Join(diagRoot, top.Name())
		if fileExists(filepath.Join(topDir, "stats.json")) {
			benches = append(benches, diagSummaryBench{Label: top.Name(), Domain: "previous_schema", Name: top.Name(), Dir: topDir})
			continue
		}
		subs, err := os.ReadDir(topDir)
		if err != nil {
			return nil, err
		}
		for _, sub := range subs {
			if !sub.IsDir() {
				continue
			}
			subDir := filepath.Join(topDir, sub.Name())
			if fileExists(filepath.Join(subDir, "stats.json")) {
				benches = append(benches, diagSummaryBench{Label: top.Name() + "/" + sub.Name(), Domain: top.Name(), Name: sub.Name(), Dir: subDir})
			}
		}
	}
	sort.Slice(benches, func(i, j int) bool { return benches[i].Label < benches[j].Label })
	return benches, nil
}

func readDiagStats(path string) (diagStatsFile, error) {
	var stats diagStatsFile
	data, err := os.ReadFile(path)
	if err != nil {
		return stats, err
	}
	err = json.Unmarshal(data, &stats)
	return stats, err
}

func hottestProto(protos []diagProtoStats) (diagProtoStats, bool) {
	var hot diagProtoStats
	ok := false
	for _, proto := range protos {
		if proto.SkipReason != "" {
			continue
		}
		if !ok || proto.InsnCount > hot.InsnCount {
			hot = proto
			ok = true
		}
	}
	if hot.InsnHistogram == nil {
		hot.InsnHistogram = map[string]int{}
	}
	return hot, ok
}

func writeDiagDriftSummary(b *strings.Builder) {
	fmt.Fprintln(b)
	fmt.Fprintln(b, "## Drift vs frozen reference.json")
	fmt.Fprintln(b)
	ref, refOK := readBenchmarkTimingJSON(filepath.Join("benchmarks", "data", "reference.json"))
	latest, latestOK := readBenchmarkTimingJSON(filepath.Join("benchmarks", "data", "latest.json"))
	if !refOK || !latestOK {
		if !refOK {
			fmt.Fprintln(b, "_reference.json not found_")
		}
		if !latestOK {
			fmt.Fprintln(b, "_latest.json not found — run benchmarks/strict_guard.py or benchmarks/regression_guard.py first_")
		}
		return
	}
	meta := mapValue(ref["_meta"])
	excluded := map[string]bool{}
	if values, ok := meta["excluded"].([]any); ok {
		for _, value := range values {
			excluded[fmt.Sprint(value)] = true
		}
	}
	flagThreshold := floatFromAny(meta["drift_threshold_flag_pct"], 2.0)
	failThreshold := floatFromAny(meta["drift_threshold_fail_pct"], 5.0)
	refResults := mapValue(ref["results"])
	latestResults := mapValue(latest["results"])
	type drifter struct {
		Name string
		Ref  float64
		New  float64
		Pct  float64
	}
	var drifters []drifter
	for name, rawRef := range refResults {
		if excluded[name] {
			continue
		}
		refTime, ok := parseDiagTime(mapValue(rawRef)["jit"])
		if !ok || refTime <= 0 {
			continue
		}
		latestTime, ok := parseDiagTime(mapValue(latestResults[name])["jit"])
		if !ok {
			continue
		}
		drifters = append(drifters, drifter{Name: name, Ref: refTime, New: latestTime, Pct: (latestTime - refTime) / refTime * 100})
	}
	sort.Slice(drifters, func(i, j int) bool { return drifters[i].Pct > drifters[j].Pct })
	fmt.Fprintf(b, "Flag threshold: %.1f%%. Fail threshold: %.1f%%.\n", flagThreshold, failThreshold)
	fmt.Fprintln(b)
	fmt.Fprintln(b, "| Benchmark | Reference | Latest | Drift | Status |")
	fmt.Fprintln(b, "|-----------|----------:|-------:|------:|:-------|")
	for i, d := range drifters {
		if i >= 10 {
			break
		}
		status := "ok"
		if d.Pct >= failThreshold {
			status = "FAIL"
		} else if d.Pct >= flagThreshold {
			status = "FLAG"
		}
		fmt.Fprintf(b, "| %s | %.3fs | %.3fs | %+.2f%% | %s |\n", d.Name, d.Ref, d.New, d.Pct, status)
	}
}

func readBenchmarkTimingJSON(path string) (map[string]any, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, false
	}
	return out, true
}

var diagTimeRE = regexp.MustCompile(`[\d.]+`)

func parseDiagTime(value any) (float64, bool) {
	if value == nil {
		return 0, false
	}
	match := diagTimeRE.FindString(fmt.Sprint(value))
	if match == "" {
		return 0, false
	}
	parsed, err := strconv.ParseFloat(match, 64)
	return parsed, err == nil
}

func mapValue(value any) map[string]any {
	if out, ok := value.(map[string]any); ok {
		return out
	}
	return map[string]any{}
}

func floatFromAny(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return fallback
	}
}

func emptyAsQuestion(value string) string {
	if value == "" {
		return "?"
	}
	return value
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
