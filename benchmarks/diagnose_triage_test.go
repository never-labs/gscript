package benchmarks

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/tooling/benchdisc"
)

type diagnoseRow struct {
	benchmark          string
	group              string
	script             string
	status             string
	timeSeconds        *float64
	t2Attempted        int
	t2Compiled         int
	t2Entered          int
	exitTotal          int
	topExit            map[string]any
	workAction         string
	workTarget         string
	workProto          string
	workPriority       int
	readiness          string
	runtimeSummary     map[string]any
	tier2CallSummary   map[string]any
	pprofRuns          int
	pprofScriptRepeat  int
	pprofSampleSeconds float64
	pprofEffective     bool
	artifactDir        string
}

func floatPtr(v float64) *float64 { return &v }

func diagnosticTimeText(row diagnoseRow) string {
	if row.timeSeconds == nil {
		return "-"
	}
	return formatSeconds(*row.timeSeconds)
}

func diagnosticTier2Text(row diagnoseRow) string {
	return strings.Join([]string{itoa(row.t2Attempted), itoa(row.t2Compiled), itoa(row.t2Entered)}, "/")
}

func diagnosticTopExitText(row diagnoseRow) string {
	if row.topExit == nil {
		return "-"
	}
	return strings.TrimSpace(toString(row.topExit["exit_name"]) + " " + toString(row.topExit["reason"]) + " pc=" + toString(row.topExit["pc"]) + " count=" + toString(row.topExit["count"]))
}

func diagnosticWorkText(row diagnoseRow) string {
	if row.workAction == "" && row.workTarget == "" {
		return "-"
	}
	return strings.TrimSpace(row.workAction + "/" + row.workTarget + " " + row.workProto + " p=" + itoa(row.workPriority) + " " + row.readiness)
}

var diagnosticRuntimeSummaryKeys = []string{
	"native_fallback",
	"runtime_call",
	"string_format_fast",
	"table_string_get_fast",
	"table_string_set_append",
	"table_string_set_map",
	"table_string_set_promote",
	"string_concat_lazy",
	"string_concat_builder",
}

func diagnosticRuntimeText(row diagnoseRow) string {
	var bits []string
	for _, key := range diagnosticRuntimeSummaryKeys {
		if value, ok := row.runtimeSummary[key]; ok && value != nil && toString(value) != "0" {
			bits = append(bits, key+"="+toString(value))
		}
	}
	var tier2Keys []string
	for key, value := range row.tier2CallSummary {
		if value != nil && toString(value) != "0" {
			tier2Keys = append(tier2Keys, key)
		}
	}
	sort.Strings(tier2Keys)
	for _, key := range tier2Keys {
		bits = append(bits, "tier2_"+key+"="+toString(row.tier2CallSummary[key]))
	}
	if len(bits) == 0 {
		return "-"
	}
	return strings.Join(bits, ", ")
}

func diagnosticPprofText(row diagnoseRow) string {
	if row.pprofRuns == 0 {
		return "-"
	}
	status := "low"
	if row.pprofEffective {
		status = "ok"
	}
	return status + " " + formatSeconds(row.pprofSampleSeconds) + "/" + itoa(row.pprofRuns) + " runs/repeat " + itoa(row.pprofScriptRepeat)
}

func diagnosticMarkdownRow(row diagnoseRow) string {
	return testMarkdownRow(
		row.group+"/"+row.benchmark,
		diagnosticTimeText(row),
		diagnosticTier2Text(row),
		itoa(row.exitTotal),
		diagnosticTopExitText(row),
		diagnosticWorkText(row),
		diagnosticRuntimeText(row),
		diagnosticPprofText(row),
		"`"+row.artifactDir+"`",
	)
}

func TestDiagnoseGroupsForArgsAcceptsDomainGroupsAndSelectors(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	got, err := benchdisc.GroupsForSelection(root, []string{"data"}, []string{"concurrency/goroutine_sleep", "table/events_metamethod"}, false, benchdisc.DomainGroups)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"data", "concurrency", "table"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestDiagnoseSummaryHelpersFormatMissingValues(t *testing.T) {
	row := diagnoseRow{benchmark: "sum", group: "math", script: "benchmarks/math/sum.leia", status: "ok"}
	if diagnosticTimeText(row) != "-" || diagnosticTier2Text(row) != "0/0/0" || diagnosticRuntimeText(row) != "-" || diagnosticPprofText(row) != "-" {
		t.Fatalf("missing row formatting mismatch: %q %q %q %q", diagnosticTimeText(row), diagnosticTier2Text(row), diagnosticRuntimeText(row), diagnosticPprofText(row))
	}
}

func TestDiagnoseSummaryHelpersFormatPresentValues(t *testing.T) {
	row := diagnoseRow{
		benchmark:          "events_metamethod",
		group:              "table",
		script:             "benchmarks/table/events_metamethod.leia",
		status:             "ok",
		timeSeconds:        floatPtr(0.1254),
		t2Attempted:        5,
		t2Compiled:         4,
		t2Entered:          3,
		runtimeSummary:     map[string]any{"native_fallback": 11},
		tier2CallSummary:   map[string]any{"turn": 2},
		pprofRuns:          4,
		pprofScriptRepeat:  8,
		pprofSampleSeconds: 0.321,
		pprofEffective:     true,
	}
	if diagnosticTimeText(row) != "0.125s" || diagnosticTier2Text(row) != "5/4/3" || diagnosticRuntimeText(row) != "native_fallback=11, tier2_turn=2" || diagnosticPprofText(row) != "ok 0.321s/4 runs/repeat 8" {
		t.Fatalf("present row formatting mismatch: %q %q %q %q", diagnosticTimeText(row), diagnosticTier2Text(row), diagnosticRuntimeText(row), diagnosticPprofText(row))
	}
}

func TestDiagnoseMarkdownRowFormatsRuntimeAndProfileBits(t *testing.T) {
	row := diagnoseRow{
		benchmark:          "events_metamethod",
		group:              "table",
		script:             "benchmarks/table/events_metamethod.leia",
		status:             "ok",
		timeSeconds:        floatPtr(0.125),
		t2Attempted:        5,
		t2Compiled:         4,
		t2Entered:          3,
		exitTotal:          2,
		topExit:            map[string]any{"exit_name": "shape", "reason": "guard", "pc": 17, "count": 9},
		workAction:         "compile",
		workTarget:         "loop",
		workProto:          "<main>",
		workPriority:       7,
		readiness:          "ready",
		runtimeSummary:     map[string]any{"native_fallback": 11, "string_format_fast": 3},
		tier2CallSummary:   map[string]any{"turn": 2},
		pprofRuns:          4,
		pprofScriptRepeat:  8,
		pprofSampleSeconds: 0.321,
		pprofEffective:     true,
		artifactDir:        "out/table",
	}
	want := "| table/events_metamethod | 0.125s | 5/4/3 | 2 | shape guard pc=17 count=9 | compile/loop <main> p=7 ready | native_fallback=11, string_format_fast=3, tier2_turn=2 | ok 0.321s/4 runs/repeat 8 | `out/table` |"
	if got := diagnosticMarkdownRow(row); got != want {
		t.Fatalf("diagnosticMarkdownRow = %q\nwant %q", got, want)
	}
}

func TestDiagnoseMarkdownRowFormatsEmptyOptionalFields(t *testing.T) {
	row := diagnoseRow{benchmark: "sum", group: "math", script: "benchmarks/math/sum.leia", status: "ok", artifactDir: "out/math"}
	want := "| math/sum | - | 0/0/0 | 0 | - | - | - | - | `out/math` |"
	if got := diagnosticMarkdownRow(row); got != want {
		t.Fatalf("diagnosticMarkdownRow = %q\nwant %q", got, want)
	}
}

type triageArtifactStatus struct {
	path   string
	status string
	note   string
}

type triageBottleneck struct {
	category       string
	priority       string
	confidence     string
	evidence       []string
	recommendation string
}

func triageBenchIDToPath(root, bench string) (benchdisc.Benchmark, bool) {
	return benchdisc.ResolveScriptIdentity(root, bench, benchdisc.DomainGroups)
}

func triageGroupsForBenches(root string, benches []string) ([]string, error) {
	return benchdisc.GroupsForSelectors(root, []string{}, benches, benchdisc.DomainGroups)
}

func writeTriageReport(out string, rows []map[string]any, timingMD string, summaryJSON string, bottlenecks []triageBottleneck, artifacts map[string]triageArtifactStatus) error {
	lines := []string{
		"# Performance Triage",
		"",
		"## Optimization Priorities",
		"",
		"| Priority | Category | Confidence | Evidence | Next step |",
		"|---|---|---|---|---|",
	}
	for _, item := range bottlenecks {
		evidence := strings.Join(item.evidence, "<br>")
		if evidence == "" {
			evidence = "-"
		}
		lines = append(lines, testMarkdownRow(item.priority, item.category, item.confidence, evidence, item.recommendation))
	}
	lines = append(lines,
		"",
		"## Timing",
		"",
		"| Benchmark | Scale | Mode | Current | HEAD | LuaJIT | Current/HEAD | Current/LuaJIT | Source | Repeat | Exits | CI95 | Verdict |",
		"|---|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---|",
	)
	for _, row := range rows {
		lines = append(lines, testMarkdownRow(
			toString(row["benchmark"]),
			triageFmtScale(row["scale"]),
			toString(row["mode"]),
			triageFmtSeconds(row["current"]),
			triageFmtSeconds(row["head"]),
			triageFmtSeconds(row["luajit"]),
			triageFmtRatio(row["cur_head"]),
			triageFmtRatio(row["cur_luajit"]),
			toString(row["source"]),
			toString(row["repeat"]),
			toString(row["exits"]),
			triageFmtPercent(row["ci95"]),
			triageVerdict(row),
		))
	}
	lines = append(lines, "", "## Artifacts", "", "- summary JSON: `"+summaryJSON+"`", "- timing report: `"+timingMD+"`")
	lines = append(lines, "", "## Artifact Status", "", "| Artifact | Status | Path | Note |", "|---|---|---|---|")
	keys := make([]string, 0, len(artifacts))
	for key := range artifacts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		status := artifacts[key]
		path := status.path
		if path == "" {
			path = "-"
		}
		note := status.note
		if note == "" {
			note = "-"
		}
		lines = append(lines, testMarkdownRow(key, status.status, "`"+path+"`", note))
	}
	return os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func TestTriageBenchScriptPathAcceptsDomainSelectors(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	for _, tc := range []struct {
		selector string
		group    string
		name     string
	}{
		{"data/soa_dot", "data", "soa_dot"},
		{"concurrency/goroutine_sleep", "concurrency", "goroutine_sleep"},
		{"table/events_metamethod", "table", "events_metamethod"},
	} {
		got, ok := triageBenchIDToPath(root, tc.selector)
		if !ok {
			t.Fatalf("%s did not resolve", tc.selector)
		}
		wantPath := filepath.Join(root, "benchmarks", tc.group, tc.name+".leia")
		if got.Group != tc.group || got.Name != tc.name || got.Leia != wantPath {
			t.Fatalf("%s = %#v, want %s/%s %s", tc.selector, got, tc.group, tc.name, wantPath)
		}
	}
}

func TestTriageGroupsForBenchesUsesSharedDomainSelectorResolution(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	got, err := triageGroupsForBenches(root, []string{"table/events_metamethod", "concurrency/goroutine_sleep", "data/soa_dot"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"table", "concurrency", "data"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("groups = %#v, want %#v", got, want)
	}
}

func TestTriageWriteReportUsesSharedMarkdownRowShape(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "triage.md")
	timing := filepath.Join(dir, "timing.md")
	summary := filepath.Join(dir, "triage.json")
	rows := []map[string]any{{
		"benchmark":  "math/sum",
		"scale":      map[string]any{"n": 10},
		"mode":       "default",
		"current":    0.1,
		"head":       0.2,
		"luajit":     0.3,
		"cur_head":   0.5,
		"cur_luajit": 0.333,
		"source":     "script",
		"repeat":     4,
		"exits":      0,
		"ci95":       1.25,
		"note":       "",
	}}
	artifacts := map[string]triageArtifactStatus{
		"timing": {path: timing, status: "ok", note: "ready"},
		"diag":   {status: "not-requested"},
	}
	err := writeTriageReport(out, rows, timing, summary, []triageBottleneck{{
		category:       "runtime-call-heavy",
		priority:       "P2",
		confidence:     "medium",
		evidence:       []string{"runtime call"},
		recommendation: "inline hot path",
	}}, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := string(text)
	for _, want := range []string{
		"| P2 | runtime-call-heavy | medium | runtime call | inline hot path |",
		"| math/sum | n:10 | default | 0.100000s | 0.200000s | 0.300000s |",
		"| diag | not-requested | `-` | - |",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("report missing %q\n%s", want, got)
		}
	}
}

func itoa(v int) string {
	return toString(v)
}

func toString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case int:
		return fmt.Sprintf("%d", value)
	case int64:
		return fmt.Sprintf("%d", value)
	case float64:
		return fmt.Sprintf("%g", value)
	case nil:
		return ""
	default:
		return strings.TrimSpace(strings.Trim(strings.ReplaceAll(strings.TrimPrefix(strings.TrimPrefix(reflect.ValueOf(value).String(), "<"), "&"), "\n", " "), "{}"))
	}
}

func formatSeconds(v float64) string {
	return fmt.Sprintf("%.3f", v) + "s"
}

func triageFmtSeconds(v any) string {
	if value, ok := v.(float64); ok {
		return fmt.Sprintf("%.6f", value) + "s"
	}
	return "-"
}

func triageFmtRatio(v any) string {
	if value, ok := v.(float64); ok {
		return fmt.Sprintf("%.3f", value) + "x"
	}
	return "-"
}

func triageFmtPercent(v any) string {
	if value, ok := v.(float64); ok {
		return fmt.Sprintf("%.2f", value) + "%"
	}
	return "-"
}

func testMarkdownRow(cells ...any) string {
	parts := make([]string, len(cells))
	for i, cell := range cells {
		parts[i] = fmt.Sprint(cell)
	}
	return "| " + strings.Join(parts, " | ") + " |"
}

func triageFmtScale(v any) string {
	m, ok := v.(map[string]any)
	if !ok || len(m) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+":"+toString(m[key]))
	}
	return strings.Join(parts, ",")
}

func triageVerdict(row map[string]any) string {
	if ratio, ok := row["cur_luajit"].(float64); ok && ratio > 3 {
		return "high LuaJIT gap"
	}
	if exits, ok := row["exits"].(int); ok && exits > 0 {
		return "exit-heavy: inspect leia bench profile-exits"
	}
	if ratio, ok := row["cur_luajit"].(float64); ok && ratio < 1 {
		return "faster than LuaJIT on this measurement"
	}
	return "moderate/codegen-runtime investigation"
}
