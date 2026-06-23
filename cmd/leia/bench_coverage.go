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
	"strings"
)

type benchCoverageFamily struct {
	Status     string
	Benchmarks []string
	Note       string
}

var benchCoverageMap = map[string]benchCoverageFamily{
	"api":        {"covered", []string{"table/table_field_access", "table/table_array_access", "table/nextvar_table"}, "raw table helpers and table traversal are covered by hot benchmarks"},
	"big":        {"covered", []string{"table/table_array_access", "calls/object_creation"}, "large table/object construction has benchmark coverage"},
	"bwcoercion": {"covered", []string{"numeric/math_intensive", "string/math_bit_utf8"}, "numeric and string-to-bit32 coercion hot paths are covered"},
	"bitwise":    {"covered", []string{"numeric/math_intensive", "string/math_bit_utf8"}, "numeric and bit32 helper hot paths are covered"},
	"calls":      {"covered", []string{"calls/method_dispatch", "recursion/mutual_recursion", "recursion/fib_recursive", "calls/calls_vararg_coroutine", "calls/call_len_pairs_metamethod"}, "call, method dispatch, recursion, call adjustment, and __call are covered"},
	"closure":    {"covered", []string{"calls/closure_bench", "calls/closure_accumulator", "calls/calls_vararg_coroutine"}, "closure allocation/capture and accumulator hot paths are covered"},
	"code":       {"covered", []string{"numeric/math_intensive", "string/string_bench", "numeric/sum_primes"}, "constant/immediate compiler paths are exercised by numeric, string, and branch-heavy benchmarks"},
	"constructs": {"covered", []string{"numeric/sum_primes", "numeric/fannkuch"}, "loop/control-flow hot paths are covered"},
	"control":    {"covered", []string{"calls/calls_vararg_coroutine", "control/defer_protected"}, "coroutine, protected-call, and defer-style cleanup control paths are covered"},
	"coroutine":  {"covered", []string{"calls/coroutine_bench", "concurrency/producer_consumer_pipeline", "calls/calls_vararg_coroutine"}, "coroutine resume/yield and pipeline paths are covered"},
	"cstack":     {"covered", []string{"string/log_tokenize_format", "string/strings_patterns"}, "pattern stack behavior is covered by string/pattern hot benchmarks"},
	"errors":     {"semantic_only", nil, "error paths are normally cold; track correctness and targeted latency separately"},
	"defer":      {"covered", []string{"control/defer_protected"}, "defer-style cleanup and protected-call unwinding are covered by a hot benchmark"},
	"events":     {"covered", []string{"calls/method_dispatch", "app/actors_dispatch_mutation", "table/events_metamethod", "calls/call_len_pairs_metamethod", "table/table_sort_proxy"}, "method/index dispatch and metamethod hot loops are covered"},
	"files":      {"semantic_only", nil, "IO depends on host filesystem and is not comparable to LuaJIT as a core VM hot path"},
	"gengc":      {"semantic_only", nil, "GC mode controls are semantic/diagnostic checks; allocation pressure is tracked separately"},
	"gc":         {"covered", []string{"calls/object_creation", "recursion/binary_trees", "table/nextvar_table"}, "allocation pressure and table churn are covered; collectgarbage/finalization APIs remain semantic/host behavior"},
	"heavy":      {"covered", []string{"string/string_bench", "string/log_tokenize_format", "string/strings_patterns"}, "string pressure, generated concat, and string growth are covered"},
	"literals":   {"semantic_only", nil, "literal parsing is front-end correctness; not a steady-state runtime hot path"},
	"locals":     {"covered", []string{"recursion/fibonacci_iterative", "numeric/sum_primes"}, "local-slot integer loops are covered"},
	"math":       {"covered", []string{"numeric/math_intensive", "numeric/spectral_norm", "numeric/mandelbrot", "string/math_bit_utf8"}, "integer, float, transcendental, conversion, and loop-heavy math are covered"},
	"nextvar":    {"covered", []string{"table/table_array_access", "table/json_table_walk", "table/nextvar_table", "calls/call_len_pairs_metamethod"}, "array/table traversal and pairs/next mutation-order variants are covered"},
	"oop":        {"covered", []string{"calls/method_dispatch", "calls/object_creation"}, "class-style method dispatch and object construction are covered"},
	"pm":         {"covered", []string{"string/string_bench", "string/log_tokenize_format", "string/strings_patterns"}, "string search/format and Lua pattern capture/gsub/gmatch are covered"},
	"regexp":     {"covered", []string{"string/regexp_random"}, "Go regexp compile/match/split hot paths are covered separately from Lua pattern matching"},
	"sort":       {"covered", []string{"table/sort", "table/sort_mixed_numeric", "table/table_sort_proxy"}, "numeric, mixed, and proxy table sort hot paths are covered"},
	"strings":    {"covered", []string{"string/string_bench", "string/log_tokenize_format", "string/strings_patterns"}, "common string ops plus format/pattern/table.concat edge families are covered"},
	"table":      {"covered", []string{"table/table_field_access", "table/table_array_access", "table/json_table_walk", "table/nextvar_table", "table/table_sort_proxy"}, "field, array, nested walk, mutation traversal, table.move, and proxy table paths are covered"},
	"utf8":       {"covered", []string{"string/string_bench", "string/math_bit_utf8"}, "byte string work and utf8 iterator/validation helpers are covered"},
	"vararg":     {"covered", []string{"calls/closure_bench", "calls/calls_vararg_coroutine"}, "call/closure and select/pack/unpack adjustment hot paths are covered"},
}

var benchCoverageSemanticPrefixes = map[string]bool{
	"all": true, "attrib": true, "base64": true, "binary": true, "bits": true, "bytes": true, "compress": true, "container": true, "crypto": true, "csv": true, "db": true, "debug": true, "defer": true, "encoding": true, "fs": true, "go": true, "goto": true, "http": true, "io": true, "json": true, "log": true, "main": true, "matrix": true, "net": true, "os": true, "process": true, "rand": true, "regexp": true, "time": true, "tracegc": true, "tpack": true, "url": true, "uuid": true, "vec": true, "verybig": true, "xpcall": true,
}

var benchCoverageGroups = []string{"numeric", "recursion", "table", "calls", "string", "concurrency", "data", "app", "control", "precision"}
var benchCoverageHotLoopRE = regexp.MustCompile(`\bfor\b[^\n]*(?:1000|10000|100000|1e4)\b`)

func runBenchCoverageCommand(args []string, outw, errw io.Writer) int {
	fs := flag.NewFlagSet("bench coverage", flag.ContinueOnError)
	fs.SetOutput(errw)
	jsonPath := fs.String("json", "", "write JSON coverage report")
	markdownPath := fs.String("markdown", "", "write Markdown coverage report")
	check := fs.Bool("check", false, "fail when a case family lacks classification or a mapped benchmark is missing")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if len(fs.Args()) != 0 {
		fmt.Fprintln(errw, "usage: leia bench coverage [--check] [--json FILE] [--markdown FILE]")
		return 2
	}
	root, err := findCLIRepoRootFromCWD()
	if err != nil {
		fmt.Fprintf(errw, "leia bench coverage: %v\n", err)
		return 1
	}
	cases := benchCoverageConformanceCases(root)
	known := benchCoverageBenchmarkIDs(root)
	report := benchCoverageMarkdown(cases, known)
	if *markdownPath != "" {
		if err := os.MkdirAll(filepath.Dir(*markdownPath), 0o755); err != nil && filepath.Dir(*markdownPath) != "." {
			fmt.Fprintf(errw, "leia bench coverage: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*markdownPath, []byte(report), 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench coverage: %v\n", err)
			return 1
		}
	} else {
		fmt.Fprintln(outw, report)
	}
	if *jsonPath != "" {
		payload := benchCoverageJSON(cases)
		body, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Fprintf(errw, "leia bench coverage: %v\n", err)
			return 1
		}
		body = append(body, '\n')
		if err := os.MkdirAll(filepath.Dir(*jsonPath), 0o755); err != nil && filepath.Dir(*jsonPath) != "." {
			fmt.Fprintf(errw, "leia bench coverage: %v\n", err)
			return 1
		}
		if err := os.WriteFile(*jsonPath, body, 0o644); err != nil {
			fmt.Fprintf(errw, "leia bench coverage: %v\n", err)
			return 1
		}
	}
	if *check {
		missingFamilies := benchCoverageMissingFamilies(cases)
		missingRefs := benchCoverageMissingRefs(cases, known)
		if len(missingFamilies) > 0 || len(missingRefs) > 0 {
			if len(missingFamilies) > 0 {
				fmt.Fprintln(outw, "missing coverage families: "+strings.Join(missingFamilies, ", "))
			}
			if len(missingRefs) > 0 {
				fmt.Fprintln(outw, "missing benchmark references: "+strings.Join(missingRefs, ", "))
			}
			return 1
		}
	}
	return 0
}

func benchCoverageConformanceCases(root string) map[string][]string {
	cases := map[string][]string{}
	matches, _ := filepath.Glob(filepath.Join(root, "tests", "language", "*.lua"))
	sort.Strings(matches)
	for _, path := range matches {
		stem := strings.TrimSuffix(filepath.Base(path), ".lua")
		prefix := strings.SplitN(stem, "_", 2)[0]
		cases[prefix] = append(cases[prefix], path)
	}
	return cases
}

func benchCoverageBenchmarkIDs(root string) map[string]bool {
	ids := map[string]bool{}
	for _, group := range benchCoverageGroups {
		matches, _ := filepath.Glob(filepath.Join(root, "benchmarks", group, "*.leia"))
		for _, path := range matches {
			ids[group+"/"+strings.TrimSuffix(filepath.Base(path), ".leia")] = true
		}
	}
	return ids
}

func benchCoverageFor(prefix string) benchCoverageFamily {
	if cov, ok := benchCoverageMap[prefix]; ok {
		return cov
	}
	if benchCoverageSemanticPrefixes[prefix] {
		return benchCoverageFamily{"semantic_only", nil, "host integration or short semantic checks; compare correctness unless a hot workload is extracted"}
	}
	return benchCoverageFamily{"missing", nil, "no explicit performance coverage classification yet"}
}

func benchCoverageHotHints(paths []string) []string {
	out := []string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		text := string(data)
		if benchCoverageHotLoopRE.MatchString(text) || len(strings.Split(text, "\n")) >= 80 {
			out = append(out, strings.TrimSuffix(filepath.Base(path), ".lua"))
		}
	}
	return out
}

func benchCoverageMissingRefs(cases map[string][]string, known map[string]bool) []string {
	missing := map[string]bool{}
	for prefix := range cases {
		for _, bench := range benchCoverageFor(prefix).Benchmarks {
			if !known[bench] {
				missing[bench] = true
			}
		}
	}
	return sortedBoolKeys(missing)
}

func benchCoverageMissingFamilies(cases map[string][]string) []string {
	missing := map[string]bool{}
	for prefix := range cases {
		if benchCoverageFor(prefix).Status == "missing" {
			missing[prefix] = true
		}
	}
	return sortedBoolKeys(missing)
}

func benchCoverageMarkdown(cases map[string][]string, known map[string]bool) string {
	prefixes := sortedCaseKeys(cases)
	summary := map[string]int{}
	type row struct {
		prefix, status, benches, hot, note string
		count                              int
	}
	rows := make([]row, 0, len(prefixes))
	for _, prefix := range prefixes {
		cov := benchCoverageFor(prefix)
		summary[cov.Status] += len(cases[prefix])
		hints := benchCoverageHotHints(cases[prefix])
		hot := "-"
		if len(hints) > 0 {
			if len(hints) > 6 {
				hot = strings.Join(hints[:6], ", ") + " ..."
			} else {
				hot = strings.Join(hints, ", ")
			}
		}
		benches := "-"
		if len(cov.Benchmarks) > 0 {
			benches = strings.Join(cov.Benchmarks, ", ")
		}
		rows = append(rows, row{prefix: prefix, count: len(cases[prefix]), status: cov.Status, benches: benches, hot: hot, note: cov.Note})
	}
	var b strings.Builder
	b.WriteString("# Language Conformance Performance Coverage\n\n")
	b.WriteString("This report maps language conformance correctness cases to hot-loop performance coverage.\n")
	b.WriteString("Short semantic cases are not treated as LuaJIT comparisons because process wall time would be dominated by startup noise.\n\n")
	b.WriteString("## Summary\n\n")
	for _, status := range sortedIntKeys(summary) {
		fmt.Fprintf(&b, "- `%s`: %d cases\n", status, summary[status])
	}
	if missing := benchCoverageMissingRefs(cases, known); len(missing) > 0 {
		fmt.Fprintf(&b, "- missing benchmark references in map: %s\n", strings.Join(missing, ", "))
	}
	b.WriteString("\n## Families\n\n")
	b.WriteString("| Family | Cases | Status | Existing Hot Benchmarks | Hot Candidates | Note |\n")
	b.WriteString("|---|---:|---|---|---|---|\n")
	for _, row := range rows {
		fmt.Fprintf(&b, "| `%s` | %d | `%s` | %s | %s | %s |\n", row.prefix, row.count, row.status, row.benches, row.hot, row.note)
	}
	return b.String()
}

func benchCoverageJSON(cases map[string][]string) []map[string]any {
	prefixes := sortedCaseKeys(cases)
	payload := make([]map[string]any, 0, len(prefixes))
	for _, prefix := range prefixes {
		cov := benchCoverageFor(prefix)
		payload = append(payload, map[string]any{
			"family":         prefix,
			"case_count":     len(cases[prefix]),
			"status":         cov.Status,
			"benchmarks":     cov.Benchmarks,
			"hot_candidates": benchCoverageHotHints(cases[prefix]),
			"note":           cov.Note,
		})
	}
	return payload
}

func sortedBoolKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedIntKeys(values map[string]int) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedCaseKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
