package main

import (
	"bytes"
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
)

var (
	jitAddrLocationRE = regexp.MustCompile(`^\s*(\d+):\s+(0x[0-9a-fA-F]+)`)
	jitAddrSampleRE   = regexp.MustCompile(`^\s*(\d+)\s+(\d+):\s+(.+)$`)
	jitAddrSourceRE   = regexp.MustCompile(`:\d+:\d+(?:$|\s)`)
)

type jitAddrRange struct {
	AbsStart   int64  `json:"abs_start"`
	AbsEnd     int64  `json:"abs_end"`
	Proto      string `json:"proto"`
	IRInstr    any    `json:"ir_instr"`
	IROp       string `json:"ir_op"`
	IRType     string `json:"ir_type"`
	Block      any    `json:"block"`
	BytecodePC any    `json:"bytecode_pc"`
	BytecodeOp string `json:"bytecode_op"`
	SourceLine any    `json:"source_line"`
	Pass       string `json:"pass"`
	Symbol     string `json:"symbol"`
}

func runBenchJITAddrMapCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench jit-addr-map", flag.ContinueOnError)
	fs.SetOutput(errw)
	warmDir := fs.String("warm-dir", "", "directory produced by -jit-dump-warm")
	binary := fs.String("binary", "", "binary used to produce --profile")
	profile := fs.String("profile", "", "CPU profile to decode with go tool pprof -raw")
	pprofRaw := fs.String("pprof-raw", "", "precomputed go tool pprof -raw output")
	jsonPath := fs.String("json", "", "write JSON summary")
	pprofFunctionsJSON := fs.String("pprof-functions-json", "", "write pprof-function-like JSON summary")
	top := fs.Int("top", 30, "number of rows to print")
	pcs := benchProfileStringList{}
	fs.Var(&pcs, "pc", "explicit native PC to resolve, e.g. 0x1234")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *warmDir == "" {
		fmt.Fprintln(errw, "usage: leia bench jit-addr-map --warm-dir DIR [--binary BIN --profile CPU.pprof|--pprof-raw RAW|--pc PC] [--json FILE] [--pprof-functions-json FILE] [--top N]")
		return 2
	}
	ranges, err := jitAddrLoadRanges(*warmDir)
	if err != nil {
		fmt.Fprintf(errw, "leia bench jit-addr-map: %v\n", err)
		return 1
	}
	rows := make([]map[string]any, 0)
	for _, pcText := range pcs {
		pc, err := strconv.ParseInt(pcText, 0, 64)
		if err != nil {
			fmt.Fprintf(errw, "leia bench jit-addr-map: invalid --pc %q\n", pcText)
			return 2
		}
		if row := jitAddrFindRange(ranges, pc); row != nil {
			rows = append(rows, jitAddrRangeRow(row, pc))
		}
	}
	stats := jitAddrEmptyProfileStats()
	if *profile != "" || *pprofRaw != "" {
		raw := ""
		if *pprofRaw != "" {
			data, err := os.ReadFile(*pprofRaw)
			if err != nil {
				fmt.Fprintf(errw, "leia bench jit-addr-map: %v\n", err)
				return 1
			}
			raw = string(data)
		} else {
			if *binary == "" {
				fmt.Fprintln(errw, "leia bench jit-addr-map: --profile requires --binary")
				return 2
			}
			raw, err = jitAddrRunPprofRaw(*binary, *profile)
			if err != nil {
				fmt.Fprintf(errw, "leia bench jit-addr-map: %v\n", err)
				return 1
			}
		}
		locations, locationNames, samples := jitAddrParsePprofRaw(raw)
		rows = append(rows, jitAddrSummarize(ranges, locations, samples)...)
		stats = jitAddrProfileStats(ranges, locations, locationNames, samples)
	}
	doc := jitAddrOutputDocument(rows, ranges, stats)
	if *jsonPath != "" {
		if err := benchProfileWriteJSONFile(*jsonPath, doc); err != nil {
			fmt.Fprintf(errw, "leia bench jit-addr-map: %v\n", err)
			return 1
		}
	}
	if *pprofFunctionsJSON != "" {
		if err := benchProfileWriteJSONFile(*pprofFunctionsJSON, jitAddrPprofFunctionSummary(rows)); err != nil {
			fmt.Fprintf(errw, "leia bench jit-addr-map: %v\n", err)
			return 1
		}
	}
	if len(rows) == 0 {
		failure := benchProfileAnyMap(doc["failure"])
		fmt.Fprintf(outw, "No JIT PCs matched warm-dump code ranges: %s\n", benchDebugStringDefault(failure["code"], "unknown"))
		summary, _ := json.Marshal(doc["summary"])
		fmt.Fprintln(outw, string(summary))
		return 0
	}
	fmt.Fprintln(outw, "| Samples | CPU | Proto | IR | Op | Block | BC | Pass | PC |")
	fmt.Fprintln(outw, "|---:|---:|---|---:|---|---:|---:|---|---|")
	for i, row := range rows {
		if i >= *top {
			break
		}
		cpu := "-"
		if nanos := benchDebugNumber(row["cpu_nanos"]); nanos != nil {
			cpu = fmt.Sprintf("%.6fs", *nanos/1e9)
		}
		fmt.Fprintf(outw, "| %v | %s | %v | %v | %v | %v | %v | %v | %v |\n",
			benchValueOr(row["samples"], "-"), cpu, benchValueOr(row["proto"], ""), benchValueOr(row["ir_instr"], ""),
			benchValueOr(row["ir_op"], ""), benchValueOr(row["block"], ""), benchValueOr(row["bytecode_pc"], ""),
			benchValueOr(row["pass"], ""), benchValueOr(row["first_pc"], benchValueOr(row["pc"], "")))
	}
	return 0
}

func jitAddrLoadRanges(warmDir string) ([]jitAddrRange, error) {
	if fileExists(filepath.Join(warmDir, "pcmap.json")) {
		return jitAddrLoadPCMapRanges(filepath.Join(warmDir, "pcmap.json"))
	}
	if fileExists(filepath.Join(warmDir, "jit-symbols.txt")) {
		return jitAddrLoadSymbolRanges(filepath.Join(warmDir, "jit-symbols.txt"))
	}
	data, err := os.ReadFile(filepath.Join(warmDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	ranges := make([]jitAddrRange, 0)
	for _, protoAny := range benchProfileAnySlice(manifest["protos"]) {
		proto := benchProfileAnyMap(protoAny)
		files := benchProfileAnyMap(proto["files"])
		sourceMapName := benchDebugStringDefault(files["sourcemap"], "")
		codeStart, ok := jitAddrIntValue(proto["code_start"])
		if sourceMapName == "" || !ok {
			continue
		}
		sourceData, err := os.ReadFile(filepath.Join(warmDir, sourceMapName))
		if err != nil {
			return nil, err
		}
		var sourceRows []map[string]any
		if err := json.Unmarshal(sourceData, &sourceRows); err != nil {
			return nil, err
		}
		for _, row := range sourceRows {
			relStart, okStart := jitAddrIntValue(row["code_start"])
			relEnd, okEnd := jitAddrIntValue(row["code_end"])
			if !okStart || !okEnd || relStart < 0 || relEnd <= relStart {
				continue
			}
			ranges = append(ranges, jitAddrRange{
				AbsStart: codeStart + relStart, AbsEnd: codeStart + relEnd,
				Proto:   benchDebugStringDefault(proto["name"], benchDebugStringDefault(row["proto"], "")),
				IRInstr: row["ir_instr"], IROp: benchDebugStringDefault(row["ir_op"], ""), IRType: benchDebugStringDefault(row["ir_type"], ""),
				Block: row["block"], BytecodePC: row["bytecode_pc"], BytecodeOp: benchDebugStringDefault(row["bytecode_op"], ""),
				SourceLine: row["source_line"], Pass: benchDebugStringDefault(row["pass"], ""),
			})
		}
	}
	jitAddrSortRanges(ranges)
	return ranges, nil
}

func jitAddrLoadPCMapRanges(path string) ([]jitAddrRange, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var pcmap map[string]any
	if err := json.Unmarshal(data, &pcmap); err != nil {
		return nil, err
	}
	ranges := make([]jitAddrRange, 0)
	for _, fnAny := range benchProfileAnySlice(pcmap["functions"]) {
		fn := benchProfileAnyMap(fnAny)
		for _, rowAny := range benchProfileAnySlice(fn["ranges"]) {
			row := benchProfileAnyMap(rowAny)
			start, okStart := jitAddrIntValue(row["pc_start"])
			end, okEnd := jitAddrIntValue(row["pc_end"])
			if !okStart || !okEnd || end <= start {
				continue
			}
			ranges = append(ranges, jitAddrRange{
				AbsStart: start, AbsEnd: end,
				Proto:   benchDebugStringDefault(row["proto"], benchDebugStringDefault(fn["name"], "")),
				IRInstr: row["ir_instr"], IROp: benchDebugStringDefault(row["ir_op"], ""), IRType: benchDebugStringDefault(row["ir_type"], ""),
				Block: row["block"], BytecodePC: row["bytecode_pc"], BytecodeOp: benchDebugStringDefault(row["bytecode_op"], ""),
				SourceLine: row["source_line"], Pass: benchDebugStringDefault(row["pass"], ""),
			})
		}
	}
	jitAddrSortRanges(ranges)
	return ranges, nil
}

func jitAddrLoadSymbolRanges(path string) ([]jitAddrRange, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	ranges := make([]jitAddrRange, 0)
	for _, line := range strings.Split(string(data), "\n") {
		parts := strings.SplitN(line, " ", 3)
		if len(parts) != 3 {
			continue
		}
		start, err1 := strconv.ParseInt(parts[0], 16, 64)
		size, err2 := strconv.ParseInt(parts[1], 16, 64)
		if err1 != nil || err2 != nil || size <= 0 {
			continue
		}
		symbol := parts[2]
		meta := jitAddrParseSymbolMeta(symbol)
		ranges = append(ranges, jitAddrRange{
			AbsStart: start, AbsEnd: start + size, Proto: stringDefault(meta["proto"], strings.SplitN(symbol, ";", 2)[0]),
			IRInstr: jitAddrParseOptionalInt(meta["ir"]), IROp: meta["op"], IRType: meta["type"], Block: jitAddrParseOptionalInt(meta["block"]),
			BytecodePC: jitAddrParseOptionalInt(meta["bc"]), BytecodeOp: meta["bcop"], SourceLine: jitAddrParseOptionalInt(meta["line"]),
			Pass: meta["pass"], Symbol: symbol,
		})
	}
	jitAddrSortRanges(ranges)
	return ranges, nil
}

func jitAddrParseSymbolMeta(symbol string) map[string]string {
	meta := map[string]string{}
	for _, part := range strings.Split(symbol, ";")[1:] {
		key, value, ok := strings.Cut(part, "=")
		if ok {
			meta[key] = value
		}
	}
	return meta
}

func jitAddrRunPprofRaw(binary, profile string) (string, error) {
	cmd := exec.Command("go", "tool", "pprof", "-raw", binary, profile)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return "", errors.New(out.String())
	}
	return out.String(), nil
}

func jitAddrParsePprofRaw(raw string) (map[int]int64, map[int][]string, []jitAddrSample) {
	locations := map[int]int64{}
	locationNames := map[int][]string{}
	samples := make([]jitAddrSample, 0)
	section := ""
	currentLoc := 0
	for _, line := range strings.Split(raw, "\n") {
		switch line {
		case "Samples:":
			section, currentLoc = "samples", 0
			continue
		case "Locations":
			section, currentLoc = "locations", 0
			continue
		case "Mappings":
			section, currentLoc = "mappings", 0
			continue
		}
		if section == "locations" {
			if match := jitAddrLocationRE.FindStringSubmatch(line); match != nil {
				id, _ := strconv.Atoi(match[1])
				pc, _ := strconv.ParseInt(match[2], 0, 64)
				currentLoc = id
				locations[id] = pc
				if names := jitAddrParseLocationFunctionNames(line); len(names) > 0 {
					locationNames[id] = names
				}
			} else if currentLoc != 0 {
				if names := jitAddrParseLocationFunctionNames(line); len(names) > 0 {
					locationNames[currentLoc] = append(locationNames[currentLoc], names...)
				}
			}
		} else if section == "samples" {
			if match := jitAddrSampleRE.FindStringSubmatch(line); match != nil {
				count, _ := strconv.Atoi(match[1])
				nanos, _ := strconv.Atoi(match[2])
				locs := make([]int, 0)
				for _, locText := range regexp.MustCompile(`\b\d+\b`).FindAllString(match[3], -1) {
					loc, _ := strconv.Atoi(locText)
					locs = append(locs, loc)
				}
				samples = append(samples, jitAddrSample{Count: count, Nanos: nanos, Locations: locs})
			}
		}
	}
	return locations, locationNames, samples
}

type jitAddrSample struct {
	Count     int
	Nanos     int
	Locations []int
}

func jitAddrParseLocationFunctionNames(line string) []string {
	text := strings.TrimSpace(line)
	if text == "" {
		return nil
	}
	if head, _, ok := strings.Cut(text, ":"); ok {
		if _, err := strconv.Atoi(strings.TrimSpace(head)); err == nil {
			parts := strings.Fields(text)
			for i, part := range parts {
				if strings.HasPrefix(part, "0x") || strings.HasPrefix(part, "M=") {
					continue
				}
				if i+1 < len(parts) && jitAddrSourceRE.MatchString(parts[i+1]) {
					return []string{part}
				}
			}
			return nil
		}
	}
	parts := strings.Fields(text)
	if len(parts) >= 2 && jitAddrSourceRE.MatchString(parts[1]) {
		return []string{parts[0]}
	}
	return nil
}

func jitAddrSummarize(ranges []jitAddrRange, locations map[int]int64, samples []jitAddrSample) []map[string]any {
	buckets := map[string]map[string]any{}
	for _, sample := range samples {
		seen := map[string]bool{}
		for _, locID := range sample.Locations {
			pc, ok := locations[locID]
			if !ok {
				continue
			}
			row := jitAddrFindRange(ranges, pc)
			if row == nil {
				continue
			}
			key := fmt.Sprintf("%s/%v/%s/%v/%v/%s", row.Proto, row.IRInstr, row.IROp, row.Block, row.BytecodePC, row.Pass)
			if seen[key] {
				continue
			}
			seen[key] = true
			bucket := buckets[key]
			if bucket == nil {
				bucket = map[string]any{
					"samples": 0, "cpu_nanos": 0, "proto": row.Proto, "ir_instr": row.IRInstr, "ir_op": row.IROp,
					"ir_type": row.IRType, "block": row.Block, "bytecode_pc": row.BytecodePC, "bytecode_op": row.BytecodeOp,
					"source_line": row.SourceLine, "pass": row.Pass, "symbol": row.Symbol, "first_pc": fmt.Sprintf("0x%x", pc),
				}
				buckets[key] = bucket
			}
			bucket["samples"] = benchDebugToInt(bucket["samples"]) + sample.Count
			bucket["cpu_nanos"] = benchDebugToInt(bucket["cpu_nanos"]) + sample.Nanos
		}
	}
	rows := make([]map[string]any, 0, len(buckets))
	for _, row := range buckets {
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return benchDebugToInt(rows[i]["cpu_nanos"]) > benchDebugToInt(rows[j]["cpu_nanos"])
	})
	return rows
}

func jitAddrProfileStats(ranges []jitAddrRange, locations map[int]int64, locationNames map[int][]string, samples []jitAddrSample) map[string]any {
	matchedLocs := map[int]bool{}
	functionCounts := map[string]int{}
	totalSamples, totalNanos, externalSamples, externalNanos, unmatchedSamples, unmatchedNanos := 0, 0, 0, 0, 0, 0
	var pcs []int64
	for _, sample := range samples {
		totalSamples += sample.Count
		totalNanos += sample.Nanos
		sampleMatched, sampleExternal := false, false
		for _, locID := range sample.Locations {
			if pc, ok := locations[locID]; ok {
				pcs = append(pcs, pc)
				if jitAddrFindRange(ranges, pc) != nil {
					matchedLocs[locID] = true
					sampleMatched = true
				}
			}
			for _, name := range locationNames[locID] {
				functionCounts[name] += sample.Count
				if name == "runtime._ExternalCode" {
					sampleExternal = true
				}
			}
		}
		if sampleExternal {
			externalSamples += sample.Count
			externalNanos += sample.Nanos
		}
		if !sampleMatched {
			unmatchedSamples += sample.Count
			unmatchedNanos += sample.Nanos
		}
	}
	minPC, maxPC := "", ""
	if len(pcs) > 0 {
		sort.Slice(pcs, func(i, j int) bool { return pcs[i] < pcs[j] })
		minPC, maxPC = fmt.Sprintf("0x%x", pcs[0]), fmt.Sprintf("0x%x", pcs[len(pcs)-1])
	}
	return map[string]any{
		"profile_samples": totalSamples, "profile_cpu_nanos": totalNanos, "profile_locations": len(locations),
		"matched_locations": len(matchedLocs), "unmatched_samples": unmatchedSamples, "unmatched_cpu_nanos": unmatchedNanos,
		"external_code_samples": externalSamples, "external_code_cpu_nanos": externalNanos, "sampled_pc_min": minPC, "sampled_pc_max": maxPC,
		"top_profile_functions": jitAddrSortedFunctionCounts(functionCounts, 10),
	}
}

func jitAddrOutputDocument(rows []map[string]any, ranges []jitAddrRange, stats map[string]any) map[string]any {
	status := "unmatched"
	if len(rows) > 0 {
		status = "ok"
	}
	summary := jitAddrRangeStats(ranges)
	for k, v := range stats {
		summary[k] = v
	}
	summary["mapped_rows"] = len(rows)
	return map[string]any{"version": 1, "status": status, "failure": jitAddrFailureReason(rows, ranges, stats), "summary": summary, "rows": rows}
}

func jitAddrRangeStats(ranges []jitAddrRange) map[string]any {
	if len(ranges) == 0 {
		return map[string]any{"jit_ranges": 0, "jit_functions": []any{}, "jit_pc_min": "", "jit_pc_max": ""}
	}
	byProto := map[string]int{}
	minPC, maxPC := ranges[0].AbsStart, ranges[0].AbsEnd
	for _, row := range ranges {
		byProto[row.Proto]++
		if row.AbsStart < minPC {
			minPC = row.AbsStart
		}
		if row.AbsEnd > maxPC {
			maxPC = row.AbsEnd
		}
	}
	return map[string]any{"jit_ranges": len(ranges), "jit_functions": jitAddrSortedFunctionCounts(byProto, 0), "jit_pc_min": fmt.Sprintf("0x%x", minPC), "jit_pc_max": fmt.Sprintf("0x%x", maxPC)}
}

func jitAddrFailureReason(rows []map[string]any, ranges []jitAddrRange, stats map[string]any) any {
	if len(rows) > 0 {
		return nil
	}
	if len(ranges) == 0 {
		return map[string]any{"code": "no_warm_jit_ranges", "message": "warm dump did not contain any JIT code ranges; check warm/manifest.json for Tier2 compile status"}
	}
	if benchDebugToInt(stats["profile_samples"]) == 0 {
		return map[string]any{"code": "no_profile_samples", "message": "CPU profile contained no samples to map"}
	}
	if benchDebugToInt(stats["external_code_samples"]) > 0 && benchDebugToInt(stats["matched_locations"]) == 0 {
		return map[string]any{"code": "profile_external_code_without_native_pc", "message": "Go CPU profile sampled runtime._ExternalCode, but the raw profile does not preserve the actual native JIT PC; warm JIT ranges are present but cannot be joined to IR/opcode rows from this profile"}
	}
	return map[string]any{"code": "profile_pcs_outside_warm_jit_ranges", "message": "CPU profile PCs did not fall inside any production-warm JIT code range"}
}

func jitAddrPprofFunctionSummary(rows []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		name := benchDebugStringDefault(row["symbol"], "")
		if name == "" {
			name = fmt.Sprintf("leia_jit::%s;ir=%v;op=%s;bc=%v;bcop=%s;pass=%s", benchDebugStringDefault(row["proto"], ""), row["ir_instr"], benchDebugStringDefault(row["ir_op"], ""), row["bytecode_pc"], benchDebugStringDefault(row["bytecode_op"], ""), benchDebugStringDefault(row["pass"], ""))
		}
		out = append(out, map[string]any{"name": name, "system_name": name, "filename": row["proto"], "start_line": benchDebugToInt(row["source_line"]), "samples": benchDebugToInt(row["samples"]), "cpu_nanos": benchDebugToInt(row["cpu_nanos"]), "first_pc": benchValueOr(row["first_pc"], benchValueOr(row["pc"], ""))})
	}
	return out
}

func jitAddrFindRange(ranges []jitAddrRange, pc int64) *jitAddrRange {
	for i := range ranges {
		if ranges[i].AbsStart <= pc && pc < ranges[i].AbsEnd {
			return &ranges[i]
		}
	}
	return nil
}

func jitAddrRangeRow(row *jitAddrRange, pc int64) map[string]any {
	return map[string]any{"pc": fmt.Sprintf("0x%x", pc), "abs_start": row.AbsStart, "abs_end": row.AbsEnd, "proto": row.Proto, "ir_instr": row.IRInstr, "ir_op": row.IROp, "ir_type": row.IRType, "block": row.Block, "bytecode_pc": row.BytecodePC, "bytecode_op": row.BytecodeOp, "source_line": row.SourceLine, "pass": row.Pass, "symbol": row.Symbol}
}

func jitAddrEmptyProfileStats() map[string]any {
	return map[string]any{"profile_samples": 0, "profile_cpu_nanos": 0, "profile_locations": 0, "matched_locations": 0, "unmatched_samples": 0, "unmatched_cpu_nanos": 0, "external_code_samples": 0, "external_code_cpu_nanos": 0, "sampled_pc_min": "", "sampled_pc_max": "", "top_profile_functions": []any{}}
}

func jitAddrIntValue(value any) (int64, bool) {
	switch v := value.(type) {
	case int:
		return int64(v), true
	case int64:
		return v, true
	case float64:
		return int64(v), true
	case string:
		n, err := strconv.ParseInt(strings.TrimSpace(v), 0, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func jitAddrParseOptionalInt(value string) any {
	if value == "" {
		return nil
	}
	n, err := strconv.ParseInt(value, 0, 64)
	if err != nil {
		return nil
	}
	return n
}

func jitAddrSortRanges(ranges []jitAddrRange) {
	sort.SliceStable(ranges, func(i, j int) bool {
		if ranges[i].AbsStart == ranges[j].AbsStart {
			return ranges[i].AbsEnd < ranges[j].AbsEnd
		}
		return ranges[i].AbsStart < ranges[j].AbsStart
	})
}

func jitAddrSortedFunctionCounts(counts map[string]int, limit int) []map[string]any {
	type item struct {
		name  string
		count int
	}
	items := make([]item, 0, len(counts))
	for name, count := range counts {
		items = append(items, item{name, count})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].name < items[j].name
		}
		return items[i].count > items[j].count
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		out = append(out, map[string]any{"name": item.name, "samples": item.count, "ranges": item.count})
	}
	return out
}

func stringDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func benchValueOr(value any, fallback any) any {
	if value == nil {
		return fallback
	}
	return value
}
