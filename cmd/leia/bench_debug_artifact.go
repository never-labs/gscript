package main

import (
	"encoding/json"
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

const benchDebugArtifactSchemaVersion = 1

var (
	benchDebugTimeRE       = regexp.MustCompile(`Time:\s*([0-9]+(?:\.[0-9]+)?)s\b`)
	benchDebugNumberLineRE = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-/]+)\s*[:=]\s*([0-9]+(?:\.[0-9]+)?)\s*$`)
	benchDebugPerfRowRE    = regexp.MustCompile(`^\s*([A-Za-z0-9_.\-/]+):\s*count=(\d+)\s+total=(\d+)ns\s+avg=(\d+)ns\s*$`)
)

type benchDebugArtifactArgs struct {
	BenchmarkJSON    []string
	ExitStats        string
	RuntimePathStats string
	PerfStats        string
	SpecState        string
	WarmDump         string
	Label            string
	Out              string
}

func runBenchDebugArtifactCommand(args []string, outw, errw io.Writer) int {
	var opts benchDebugArtifactArgs
	fs := flag.NewFlagSet("bench debug-artifact", flag.ContinueOnError)
	fs.SetOutput(errw)
	fs.Func("benchmark-json", "timing_compare/strict_guard/regression_guard JSON; repeatable", func(value string) error {
		opts.BenchmarkJSON = append(opts.BenchmarkJSON, value)
		return nil
	})
	fs.StringVar(&opts.ExitStats, "exit-stats", "", "profile_exits JSON, raw -exit-stats-json output, or embedded JSON")
	fs.StringVar(&opts.RuntimePathStats, "runtime-path-stats", "", "raw -runtime-path-stats[-json] output")
	fs.StringVar(&opts.PerfStats, "perf-stats", "", "raw -tier2-perf-stats[-json] output")
	fs.StringVar(&opts.SpecState, "spec-state", "", "raw -tier2-spec-state-json output")
	fs.StringVar(&opts.WarmDump, "warm-dump", "", "directory produced by -jit-dump-warm")
	fs.StringVar(&opts.Label, "label", "", "free-form artifact label")
	fs.StringVar(&opts.Out, "out", "", "write artifact JSON to this path; stdout when omitted")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia bench debug-artifact [--benchmark-json FILE ...] [--exit-stats FILE] [--runtime-path-stats FILE] [--perf-stats FILE] [--spec-state FILE] [--warm-dump DIR] [--label TEXT] [--out FILE]")
		return 2
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia bench debug-artifact: %v\n", err)
		return 1
	}
	artifact := buildBenchDebugArtifact(opts, root)
	body, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		fmt.Fprintf(errw, "leia bench debug-artifact: %v\n", err)
		return 1
	}
	body = append(body, '\n')
	if opts.Out != "" {
		if err := os.MkdirAll(filepath.Dir(opts.Out), 0o755); err != nil && filepath.Dir(opts.Out) != "." {
			fmt.Fprintf(errw, "leia bench debug-artifact: %v\n", err)
			return 1
		}
		if err := os.WriteFile(opts.Out, body, 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench debug-artifact: %v\n", err)
			return 1
		}
		return 0
	}
	if _, err := outw.Write(body); err != nil {
		fmt.Fprintf(errw, "leia bench debug-artifact: %v\n", err)
		return 1
	}
	return 0
}

func buildBenchDebugArtifact(args benchDebugArtifactArgs, root string) map[string]any {
	inputs := map[string]any{}
	for i, path := range args.BenchmarkJSON {
		inputs[fmt.Sprintf("benchmark_json[%d]", i)] = benchDebugInputStatus(path, "benchmark_json")
	}
	inputs["exit_stats"] = benchDebugInputStatus(args.ExitStats, "exit_stats")
	inputs["runtime_path_stats"] = benchDebugInputStatus(args.RuntimePathStats, "runtime_path_stats")
	inputs["tier2_perf_stats"] = benchDebugInputStatus(args.PerfStats, "tier2_perf_stats")
	inputs["tier2_speculation_state"] = benchDebugInputStatus(args.SpecState, "tier2_speculation_state")
	inputs["warm_dump"] = benchDebugInputStatus(args.WarmDump, "warm_dump")

	rows := benchDebugNormalizeBenchmarkOutputs(args.BenchmarkJSON)
	exitStats := benchDebugSummarizeExitStats(args.ExitStats)
	runtimeStats := benchDebugSummarizeRuntimeStats(args.RuntimePathStats)
	perfStats := benchDebugSummarizePerfStats(args.PerfStats)
	specState := benchDebugSummarizeSpeculationState(args.SpecState)
	warmDump := benchDebugSummarizeWarmDump(args.WarmDump)
	benchmarkSummary := benchDebugSummarizeBenchmarks(rows)
	tiering := map[string]any{
		"t2_attempted":       benchDebugSumInt(rows, "t2_attempted"),
		"t2_entered":         benchDebugSumInt(rows, "t2_entered"),
		"t2_failed":          benchDebugSumInt(rows, "t2_failed"),
		"warm_dump_compiled": benchDebugToInt(warmDump["compiled"]),
		"warm_dump_entered":  benchDebugToInt(warmDump["entered"]),
	}
	gates := benchDebugSummarizeGates(rows, exitStats, warmDump)
	profiles := map[string]any{
		"warm_dump_protos":     benchDebugToInt(warmDump["protos"]),
		"warm_dump_insn_count": benchDebugToInt(warmDump["insn_count"]),
		"warm_dump_code_bytes": benchDebugToInt(warmDump["code_bytes"]),
		"pcmap_functions":      benchDebugToInt(warmDump["pcmap_functions"]),
		"pcmap_ranges":         benchDebugToInt(warmDump["pcmap_ranges"]),
	}
	return map[string]any{
		"schema_version":    benchDebugArtifactSchemaVersion,
		"generated_at":      time.Now().UTC().Format(time.RFC3339Nano),
		"label":             args.Label,
		"metadata":          map[string]any{"repo": root, "commit": benchDebugGitCommit(root)},
		"inputs":            inputs,
		"benchmark_summary": benchmarkSummary,
		"benchmarks":        rows,
		"timing":            map[string]any{"summary": benchmarkSummary, "rows": rows},
		"tiering":           tiering,
		"gates":             gates,
		"exits":             exitStats,
		"runtime_paths":     runtimeStats,
		"specialization":    specState,
		"profiles":          profiles,
		"debug": map[string]any{
			"exit_stats":              exitStats,
			"runtime_path_stats":      runtimeStats,
			"tier2_perf_stats":        perfStats,
			"tier2_speculation_state": specState,
			"warm_dump":               warmDump,
		},
	}
}

func benchDebugInputStatus(path, kind string) map[string]any {
	if path == "" {
		return map[string]any{"kind": kind, "path": nil, "status": "not-provided"}
	}
	info, err := os.Stat(path)
	if err != nil {
		return map[string]any{"kind": kind, "path": path, "status": "missing"}
	}
	var size any
	if info.Mode().IsRegular() {
		size = info.Size()
	}
	return map[string]any{"kind": kind, "path": path, "status": "ok", "bytes": size}
}

func benchDebugReadText(path string) (string, bool) {
	if path == "" {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func benchDebugReadJSONOrEmbedded(path string) any {
	text, ok := benchDebugReadText(path)
	if !ok {
		return nil
	}
	var value any
	if err := json.Unmarshal([]byte(text), &value); err == nil {
		return value
	}
	dec := json.NewDecoder(strings.NewReader(text))
	var last any
	for {
		if err := dec.Decode(&value); err != nil {
			break
		}
		last = value
	}
	if last != nil {
		return last
	}
	for i, ch := range text {
		if ch != '{' && ch != '[' {
			continue
		}
		dec := json.NewDecoder(strings.NewReader(text[i:]))
		if err := dec.Decode(&value); err == nil {
			last = value
		}
	}
	return last
}

func benchDebugNormalizeBenchmarkOutputs(paths []string) []map[string]any {
	rows := []map[string]any{}
	for _, path := range paths {
		data, ok := benchDebugReadJSONOrEmbedded(path).(map[string]any)
		if !ok {
			continue
		}
		switch results := data["results"].(type) {
		case []any:
			rows = append(rows, benchDebugNormalizeResultList(results, path)...)
		case map[string]any:
			for benchmark, modesAny := range results {
				modes, ok := modesAny.(map[string]any)
				if !ok {
					continue
				}
				for mode, value := range modes {
					seconds := benchDebugSecondsFromValue(value)
					status := "unknown"
					timeSource := ""
					if seconds != nil {
						status = "ok"
						timeSource = "script"
					}
					if mode == "jit" {
						mode = "default"
					}
					rows = append(rows, map[string]any{"source": path, "group": "unknown", "benchmark": benchmark, "mode": mode, "subject": "current", "status": status, "seconds": seconds, "time_source": timeSource, "repeat": nil, "t2_attempted": 0, "t2_entered": 0, "t2_failed": 0, "exit_total": 0})
				}
			}
		}
	}
	return rows
}

func benchDebugNormalizeResultList(results []any, source string) []map[string]any {
	var rows []map[string]any
	for _, item := range results {
		result, ok := item.(map[string]any)
		if !ok {
			continue
		}
		group := benchDebugStringDefault(result["group"], "unknown")
		benchmark := benchDebugStringDefault(result["benchmark"], benchDebugStringDefault(result["name"], ""))
		if benchmark == "" {
			continue
		}
		if modes, ok := result["modes"].(map[string]any); ok {
			for mode, subjectsAny := range modes {
				subjects, ok := subjectsAny.(map[string]any)
				if !ok {
					continue
				}
				for subject, row := range subjects {
					rows = append(rows, benchDebugNormalizedRow(source, group, benchmark, mode, subject, row))
				}
			}
			continue
		}
		for mode, row := range result {
			if mode == "benchmark" || mode == "name" || mode == "group" || mode == "scale" || mode == "samples" {
				continue
			}
			if _, ok := row.(map[string]any); ok {
				rows = append(rows, benchDebugNormalizedRow(source, group, benchmark, mode, "current", row))
			}
		}
	}
	return rows
}

func benchDebugNormalizedRow(source, group, benchmark, mode, subject string, rowAny any) map[string]any {
	row, _ := rowAny.(map[string]any)
	stats, _ := row["stats"].(map[string]any)
	seconds := benchDebugNumber(row["seconds"])
	if seconds == nil {
		seconds = benchDebugNumber(stats["median"])
	}
	return map[string]any{
		"source":       source,
		"group":        group,
		"benchmark":    benchmark,
		"mode":         mode,
		"subject":      subject,
		"status":       benchDebugStringDefault(row["status"], "unknown"),
		"seconds":      seconds,
		"time_source":  benchDebugStringDefault(row["source"], benchDebugStringDefault(row["time_source"], "")),
		"repeat":       row["repeat"],
		"t2_attempted": benchDebugToInt(row["t2_attempted"]),
		"t2_entered":   benchDebugToInt(row["t2_entered"]),
		"t2_failed":    benchDebugToInt(row["t2_failed"]),
		"exit_total":   benchDebugToInt(row["exit_total"]) + benchDebugToInt(row["exits"]),
	}
}

func benchDebugSummarizeBenchmarks(rows []map[string]any) map[string]any {
	statuses := map[string]int{}
	benches := map[string]bool{}
	totalExits := 0
	totalEntered := 0
	for _, row := range rows {
		statuses[benchDebugStringDefault(row["status"], "unknown")]++
		benches[fmt.Sprint(row["group"])+"\x00"+fmt.Sprint(row["benchmark"])] = true
		totalExits += benchDebugToInt(row["exit_total"])
		totalEntered += benchDebugToInt(row["t2_entered"])
	}
	return map[string]any{"rows": len(rows), "benchmarks": len(benches), "statuses": benchDebugSortedIntMap(statuses), "total_exits": totalExits, "total_t2_entered": totalEntered}
}

func benchDebugSummarizeExitStats(path string) map[string]any {
	data := benchDebugReadJSONOrEmbedded(path)
	if data == nil {
		return map[string]any{"status": "missing", "total": 0, "by_exit_code": map[string]int{}, "top_sites": []any{}}
	}
	if root, ok := data.(map[string]any); ok {
		if results, ok := root["results"].([]any); ok {
			byCode := map[string]int{}
			statuses := map[string]int{}
			var sites []map[string]any
			for _, item := range results {
				row, ok := item.(map[string]any)
				if !ok {
					continue
				}
				statuses[benchDebugStringDefault(row["status"], "unknown")]++
				stats, _ := row["stats"].(map[string]any)
				if codes, ok := stats["by_exit_code"].(map[string]any); ok {
					for key, value := range codes {
						byCode[key] += benchDebugToInt(value)
					}
				}
				if rawSites, ok := stats["sites"].([]any); ok {
					for _, siteAny := range rawSites {
						if site, ok := siteAny.(map[string]any); ok {
							cp := benchDebugCopyMap(site)
							if _, exists := cp["benchmark"]; !exists {
								cp["benchmark"] = row["benchmark"]
							}
							sites = append(sites, cp)
						}
					}
				}
			}
			sort.SliceStable(sites, func(i, j int) bool { return benchDebugToInt(sites[i]["count"]) > benchDebugToInt(sites[j]["count"]) })
			topSites := make([]any, 0, min(len(sites), 20))
			for i := 0; i < len(sites) && i < 20; i++ {
				topSites = append(topSites, sites[i])
			}
			total := 0
			for _, value := range byCode {
				total += value
			}
			return map[string]any{"status": "ok", "total": total, "by_exit_code": benchDebugSortedIntMap(byCode), "top_sites": topSites, "result_statuses": benchDebugSortedIntMap(statuses)}
		}
		if rawSites, ok := root["sites"].([]any); ok {
			if len(rawSites) > 20 {
				rawSites = rawSites[:20]
			}
			return map[string]any{"status": "ok", "total": benchDebugToInt(root["total"]), "by_exit_code": root["by_exit_code"], "top_sites": rawSites}
		}
	}
	return map[string]any{"status": "parse_error", "total": 0, "by_exit_code": map[string]int{}, "top_sites": []any{}}
}

func benchDebugSummarizeRuntimeStats(path string) map[string]any {
	data := benchDebugReadJSONOrEmbedded(path)
	if data != nil {
		switch data.(type) {
		case map[string]any, []any:
			return map[string]any{"status": "ok", "source": "json", "numbers": benchDebugFlattenNumbers(data, "")}
		}
	}
	if _, err := os.Stat(path); err == nil {
		return map[string]any{"status": "ok", "source": "text", "numbers": benchDebugParseNumberText(path)}
	}
	return map[string]any{"status": "missing", "source": "missing", "numbers": map[string]float64{}}
}

func benchDebugSummarizePerfStats(path string) map[string]any {
	data := benchDebugReadJSONOrEmbedded(path)
	if root, ok := data.(map[string]any); ok {
		rows, _ := root["rows"].([]any)
		return benchDebugSummarizePerfRows(rows, root["enabled"], "json")
	}
	if text, ok := benchDebugReadText(path); ok {
		var rows []any
		var enabled any
		for _, line := range strings.Split(text, "\n") {
			stripped := strings.TrimSpace(line)
			if strings.HasPrefix(stripped, "enabled:") {
				enabled = strings.TrimSpace(strings.SplitN(stripped, ":", 2)[1]) == "true"
			}
			if match := benchDebugPerfRowRE.FindStringSubmatch(line); match != nil {
				rows = append(rows, map[string]any{"name": match[1], "count": benchDebugAtoi(match[2]), "nanos": benchDebugAtoi(match[3]), "avg_nanos": benchDebugAtoi(match[4])})
			}
		}
		return benchDebugSummarizePerfRows(rows, enabled, "text")
	}
	return map[string]any{"status": "missing", "source": "missing", "enabled": false, "rows": []any{}, "total_nanos": 0}
}

func benchDebugSummarizePerfRows(rows []any, enabled any, source string) map[string]any {
	clean := make([]map[string]any, 0, len(rows))
	for _, rowAny := range rows {
		if row, ok := rowAny.(map[string]any); ok {
			clean = append(clean, row)
		}
	}
	sort.SliceStable(clean, func(i, j int) bool { return benchDebugToInt(clean[i]["nanos"]) > benchDebugToInt(clean[j]["nanos"]) })
	outRows := make([]any, len(clean))
	totalNanos := 0
	totalCount := 0
	for i, row := range clean {
		outRows[i] = row
		totalNanos += benchDebugToInt(row["nanos"])
		totalCount += benchDebugToInt(row["count"])
	}
	return map[string]any{"status": "ok", "source": source, "enabled": benchDebugBool(enabled), "rows": outRows, "total_nanos": totalNanos, "total_count": totalCount}
}

func benchDebugSummarizeSpeculationState(path string) map[string]any {
	data := benchDebugReadJSONOrEmbedded(path)
	states, ok := data.([]any)
	if !ok {
		status := "missing"
		if _, err := os.Stat(path); err == nil {
			status = "parse_error"
		}
		return map[string]any{"status": status, "protos": 0, "compiled": 0, "failed": 0, "suppressed": 0, "states": []any{}}
	}
	suppressedKinds := map[string]int{}
	compiled := 0
	failed := 0
	suppressed := 0
	guardFailures := 0
	var stateRows []any
	for _, stateAny := range states {
		row, ok := stateAny.(map[string]any)
		if !ok {
			continue
		}
		stateRows = append(stateRows, row)
		if benchDebugBool(row["compiled"]) {
			compiled++
		}
		if benchDebugBool(row["failed"]) {
			failed++
		}
		suppressed += benchDebugToInt(row["suppressed_count"])
		if kinds, ok := row["suppressed_kinds"].(map[string]any); ok {
			for kind, count := range kinds {
				suppressedKinds[kind] += benchDebugToInt(count)
			}
		}
		if failures, ok := row["guard_failures"].(map[string]any); ok {
			for _, count := range failures {
				guardFailures += benchDebugToInt(count)
			}
		}
	}
	return map[string]any{"status": "ok", "protos": len(stateRows), "compiled": compiled, "failed": failed, "suppressed": suppressed, "suppressed_kinds": benchDebugSortedIntMap(suppressedKinds), "guard_failures": guardFailures, "states": stateRows}
}

func benchDebugSummarizeWarmDump(path string) map[string]any {
	if path == "" {
		return map[string]any{"status": "missing", "path": nil}
	}
	if _, err := os.Stat(path); err != nil {
		return map[string]any{"status": "missing", "path": path}
	}
	manifest, ok := benchDebugReadJSONOrEmbedded(filepath.Join(path, "manifest.json")).(map[string]any)
	if !ok {
		return map[string]any{"status": "parse_error", "path": path}
	}
	pcmap, _ := benchDebugReadJSONOrEmbedded(filepath.Join(path, "pcmap.json")).(map[string]any)
	protos, _ := manifest["protos"].([]any)
	statuses := map[string]int{}
	fileKinds := map[string]int{}
	compiled := 0
	entered := 0
	codeBytes := 0
	insnCount := 0
	for _, protoAny := range protos {
		proto, ok := protoAny.(map[string]any)
		if !ok {
			continue
		}
		statuses[benchDebugStringDefault(proto["status"], "unknown")]++
		if benchDebugBool(proto["compiled"]) {
			compiled++
		}
		if benchDebugBool(proto["entered"]) {
			entered++
		}
		codeBytes += benchDebugToInt(proto["code_bytes"])
		insnCount += benchDebugToInt(proto["insn_count"])
		if files, ok := proto["files"].(map[string]any); ok {
			for kind := range files {
				fileKinds[kind]++
			}
		}
	}
	functions, _ := pcmap["functions"].([]any)
	pcmapRanges := 0
	for _, fnAny := range functions {
		if fn, ok := fnAny.(map[string]any); ok {
			if ranges, ok := fn["ranges"].([]any); ok {
				pcmapRanges += len(ranges)
			}
		}
	}
	return map[string]any{"status": "ok", "path": path, "proto_filter": benchDebugStringDefault(manifest["proto_filter"], ""), "protos": len(protos), "statuses": benchDebugSortedIntMap(statuses), "compiled": compiled, "entered": entered, "code_bytes": codeBytes, "insn_count": insnCount, "file_kinds": benchDebugSortedIntMap(fileKinds), "pcmap_functions": len(functions), "pcmap_ranges": pcmapRanges}
}

func benchDebugSummarizeGates(rows []map[string]any, exitStats map[string]any, warmDump map[string]any) map[string]any {
	reasons := map[string]int{}
	for _, row := range rows {
		status := benchDebugStringDefault(row["status"], "")
		if status != "" && status != "ok" {
			reasons[status]++
		}
	}
	if sites, ok := exitStats["top_sites"].([]any); ok {
		for _, siteAny := range sites {
			site, ok := siteAny.(map[string]any)
			if !ok {
				continue
			}
			reason := benchDebugStringDefault(site["reason"], benchDebugStringDefault(site["kind"], benchDebugStringDefault(site["exit"], "")))
			if reason != "" {
				count := benchDebugToInt(site["count"])
				if count == 0 {
					count = 1
				}
				reasons[reason] += count
			}
		}
	}
	if statuses, ok := warmDump["statuses"].(map[string]any); ok {
		for status, count := range statuses {
			if status != "entered" {
				reasons["warm_dump:"+status] += benchDebugToInt(count)
			}
		}
	}
	return map[string]any{"schema": "gate-summary-v1", "reason_counts": benchDebugSortedIntMap(reasons)}
}

func benchDebugFlattenNumbers(data any, prefix string) map[string]float64 {
	out := map[string]float64{}
	switch value := data.(type) {
	case map[string]any:
		for key, child := range value {
			childPrefix := key
			if prefix != "" {
				childPrefix = prefix + "." + key
			}
			for k, v := range benchDebugFlattenNumbers(child, childPrefix) {
				out[k] = v
			}
		}
	case []any:
		for i, child := range value {
			for k, v := range benchDebugFlattenNumbers(child, fmt.Sprintf("%s[%d]", prefix, i)) {
				out[k] = v
			}
		}
	default:
		if n := benchDebugNumber(value); n != nil {
			out[prefix] = *n
		}
	}
	return out
}

func benchDebugParseNumberText(path string) map[string]float64 {
	text, _ := benchDebugReadText(path)
	numbers := map[string]float64{}
	type scopePart struct {
		Indent int
		Name   string
	}
	var scope []scopePart
	for _, line := range strings.Split(text, "\n") {
		stripped := strings.TrimSpace(line)
		if stripped == "" {
			continue
		}
		if stripped == "Runtime Path Statistics:" || stripped == "Tier 2 Performance Diagnostics:" || stripped == "Tier 2 Exit Profile:" {
			scope = nil
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		for len(scope) > 0 && indent <= scope[len(scope)-1].Indent {
			scope = scope[:len(scope)-1]
		}
		if strings.HasSuffix(stripped, ":") && !benchDebugNumberLineRE.MatchString(stripped) {
			scope = append(scope, scopePart{Indent: indent, Name: strings.TrimSuffix(stripped, ":")})
			continue
		}
		if match := benchDebugNumberLineRE.FindStringSubmatch(line); match != nil {
			parts := make([]string, 0, len(scope)+1)
			for _, item := range scope {
				parts = append(parts, item.Name)
			}
			parts = append(parts, match[1])
			numbers[strings.Join(parts, ".")] = benchDebugAtof(match[2])
		}
	}
	return numbers
}

func benchDebugSecondsFromValue(value any) *float64 {
	if n := benchDebugNumber(value); n != nil {
		return n
	}
	if text, ok := value.(string); ok {
		if match := benchDebugTimeRE.FindStringSubmatch(text); match != nil {
			value := benchDebugAtof(match[1])
			return &value
		}
	}
	return nil
}

func benchDebugGitCommit(root string) any {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	return strings.TrimSpace(string(out))
}

func benchDebugSumInt(rows []map[string]any, key string) int {
	total := 0
	for _, row := range rows {
		total += benchDebugToInt(row[key])
	}
	return total
}

func benchDebugNumber(value any) *float64 {
	switch v := value.(type) {
	case float64:
		return &v
	case int:
		f := float64(v)
		return &f
	}
	return nil
}

func benchDebugToInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return 0
}

func benchDebugBool(value any) bool {
	if v, ok := value.(bool); ok {
		return v
	}
	return false
}

func benchDebugStringDefault(value any, fallback string) string {
	if v, ok := value.(string); ok {
		return v
	}
	return fallback
}

func benchDebugCopyMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func benchDebugSortedIntMap(in map[string]int) map[string]any {
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(keys))
	for _, key := range keys {
		out[key] = in[key]
	}
	return out
}

func benchDebugAtoi(value string) int {
	out, _ := strconv.Atoi(value)
	return out
}

func benchDebugAtof(value string) float64 {
	out, _ := strconv.ParseFloat(value, 64)
	return out
}
