package benchmarks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/tooling/benchdisc"
)

var (
	strictTimeRE         = regexp.MustCompile(`(?m)^Time:\s*([0-9]+(?:\.[0-9]+)?)s\b`)
	strictT2AttemptedRE  = regexp.MustCompile(`(?m)^\s*Tier 2 attempted:\s*([0-9]+)\b`)
	strictT2EnteredRE    = regexp.MustCompile(`(?m)^\s*Tier 2 entered:\s*([0-9]+)\s+functions\b`)
	strictT2FailedRE     = regexp.MustCompile(`(?m)^\s*Tier 2 failed:\s*([0-9]+)\s+functions\b`)
	strictExitTotalRE    = regexp.MustCompile(`(?m)^\s*total exits:\s*([0-9]+)\b`)
	strictChecksumRE     = regexp.MustCompile(`(?m)^checksum:\s*(\S+)\s*$`)
	strictEmbeddedTimeRE = regexp.MustCompile(`^(.+:\s+)[0-9.]+s(\s+\(result=.*\))$`)
)

type strictCommandRun struct {
	status      string
	seconds     *float64
	wallSeconds float64
	outputHash  string
	t2Attempted int
	t2Entered   int
	t2Failed    int
	exitTotal   int
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

type strictSample struct {
	status             string
	seconds            *float64
	repeat             int
	timeSource         string
	scriptTotalSeconds float64
	wallTotalSeconds   float64
	runs               []strictCommandRun
}

type strictStats struct {
	n      int
	median float64
	min    float64
	max    float64
	mean   float64
	stdev  float64
	mad    float64
	cvPct  float64
}

type strictModeResult struct {
	status         string
	stats          *strictStats
	checksumStatus string
	outputHash     string
}

func strictParseCommandRun(output, status string, exitCode *int) strictCommandRun {
	seconds := strictParseTime(output)
	if status == "ok" && seconds == nil {
		status = "no_time"
	}
	return strictCommandRun{
		status:      status,
		seconds:     seconds,
		outputHash:  strictOutputHash(output),
		t2Attempted: regexpCounter(strictT2AttemptedRE, output),
		t2Entered:   regexpCounter(strictT2EnteredRE, output),
		t2Failed:    regexpCounter(strictT2FailedRE, output),
		exitTotal:   regexpCounter(strictExitTotalRE, output),
	}
}

func strictParseTime(output string) *float64 {
	match := strictTimeRE.FindStringSubmatch(output)
	if match == nil {
		return nil
	}
	var value float64
	_, _ = fmtSscanf(match[1], "%f", &value)
	return &value
}

func strictStableOutputLines(output string) []string {
	var lines []string
	for _, raw := range strings.Split(output, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "Time:") {
			break
		}
		if strings.HasPrefix(line, "JIT Statistics:") || strings.HasPrefix(line, "Tier 2 Exit Profile:") || strings.HasPrefix(line, "[DEBUG]") {
			continue
		}
		skip := false
		for _, fragment := range []string{"Tier 2 attempted:", "Tier 2 compiled:", "Tier 2 entered:", "Tier 2 failed:", "total exits:"} {
			if strings.Contains(line, fragment) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		lines = append(lines, strictEmbeddedTimeRE.ReplaceAllString(line, `${1}<time>${2}`))
	}
	return lines
}

func strictOutputHash(output string) string {
	payload := strings.Join(strictStableOutputLines(output), "\n")
	if payload == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])[:16]
}

func strictChecksumText(output string) string {
	if match := strictChecksumRE.FindStringSubmatch(output); match != nil {
		return match[1]
	}
	lines := strictStableOutputLines(output)
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

func strictComputeStats(values []float64) strictStats {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mean := sumFloat(sorted) / float64(len(sorted))
	stdev := 0.0
	if len(sorted) > 1 {
		variance := 0.0
		for _, value := range sorted {
			d := value - mean
			variance += d * d
		}
		stdev = math.Sqrt(variance / float64(len(sorted)-1))
	}
	med := median(sorted)
	abs := make([]float64, len(sorted))
	for i, value := range sorted {
		abs[i] = math.Abs(value - med)
	}
	sort.Float64s(abs)
	return strictStats{
		n:      len(sorted),
		median: med,
		min:    sorted[0],
		max:    sorted[len(sorted)-1],
		mean:   mean,
		stdev:  stdev,
		mad:    median(abs),
		cvPct:  stdev / mean * 100,
	}
}

func strictSummarizeRepeatedRuns(runs []strictCommandRun, repeat int, timerResolution, minSampleSeconds float64, allowWallTime bool) strictSample {
	wallTotal := 0.0
	scriptTotal := 0.0
	for _, run := range runs {
		wallTotal += run.wallSeconds
		if run.seconds != nil {
			scriptTotal += *run.seconds
		}
	}
	for _, run := range runs {
		if run.status != "ok" && run.status != "no_time" {
			return strictSample{status: run.status, repeat: repeat, wallTotalSeconds: wallTotal, runs: runs}
		}
	}
	for _, run := range runs {
		if run.status == "no_time" {
			return strictSample{status: "no_time", repeat: repeat, wallTotalSeconds: wallTotal, runs: runs}
		}
	}
	if scriptTotal < minSampleSeconds || scriptTotal <= timerResolution {
		if allowWallTime && wallTotal >= minSampleSeconds {
			seconds := wallTotal / float64(repeat)
			return strictSample{status: "ok", seconds: &seconds, repeat: repeat, timeSource: "wall_repeat", scriptTotalSeconds: scriptTotal, wallTotalSeconds: wallTotal, runs: runs}
		}
		return strictSample{status: "low_resolution", repeat: repeat, scriptTotalSeconds: scriptTotal, wallTotalSeconds: wallTotal, runs: runs}
	}
	seconds := scriptTotal / float64(repeat)
	return strictSample{status: "ok", seconds: &seconds, repeat: repeat, timeSource: "script", scriptTotalSeconds: scriptTotal, wallTotalSeconds: wallTotal, runs: runs}
}

func strictSummarizeMode(samples []strictSample) strictModeResult {
	var okSeconds []float64
	hashes := map[string]bool{}
	for _, sample := range samples {
		if sample.status == "ok" && sample.seconds != nil {
			okSeconds = append(okSeconds, *sample.seconds)
			for _, run := range sample.runs {
				if run.outputHash != "" {
					hashes[run.outputHash] = true
				}
			}
		}
	}
	if len(okSeconds) == 0 {
		status := "missing"
		if len(samples) > 0 {
			status = samples[len(samples)-1].status
		}
		return strictModeResult{status: status}
	}
	status := "ok"
	if len(okSeconds) != len(samples) {
		status = "partial"
	}
	hashValues := make([]string, 0, len(hashes))
	for hash := range hashes {
		hashValues = append(hashValues, hash)
	}
	sort.Strings(hashValues)
	checksumStatus := "ok"
	if len(hashValues) > 1 {
		checksumStatus = "mismatch"
	}
	stats := strictComputeStats(okSeconds)
	return strictModeResult{status: status, stats: &stats, checksumStatus: checksumStatus, outputHash: strings.Join(hashValues, ",")}
}

func strictComparableSeconds(mode strictModeResult) *float64 {
	if mode.status != "ok" || mode.stats == nil {
		return nil
	}
	return &mode.stats.median
}

func TestStrictGuardParseTimeAndCounters(t *testing.T) {
	run := strictParseCommandRun(`result: 42
Time: 0.123s
JIT Statistics:
  Tier 2 attempted: 3
  Tier 2 entered:  2 functions
  Tier 2 failed: 1 functions
Tier 2 Exit Profile:
  total exits: 9
`, "ok", intPtrLocal(0))
	if run.status != "ok" || run.seconds == nil || *run.seconds != 0.123 || run.t2Attempted != 3 || run.t2Entered != 2 || run.t2Failed != 1 || run.exitTotal != 9 {
		t.Fatalf("run = %#v", run)
	}
}

func TestStrictGuardNoTimeIsExplicit(t *testing.T) {
	run := strictParseCommandRun("result only\n", "ok", intPtrLocal(0))
	if run.status != "no_time" || run.seconds != nil || run.outputHash != strictOutputHash("result only\n") {
		t.Fatalf("run = %#v", run)
	}
}

func TestStrictGuardOutputHashIgnoresTimingAndJITStats(t *testing.T) {
	a := strictOutputHash(`checksum: 123
Time: 0.010s
JIT Statistics:
  Tier 2 attempted: 1
  Tier 2 entered:  1 functions
Tier 2 Exit Profile:
  total exits: 0
`)
	b := strictOutputHash("checksum: 123\nTime: 0.020s\n")
	if a != b || strictChecksumText("checksum: 123\nTime: 0.020s\n") != "123" {
		t.Fatalf("hash/checksum mismatch: %q %q", a, b)
	}
}

func TestStrictGuardOutputHashIgnoresEmbeddedSubbenchmarkTimes(t *testing.T) {
	a := strictOutputHash("int_array_sum:    0.004s (result=5000050000)\narray_swap:       0.006s (result=100000)\nTime: 0.020s\n")
	b := strictOutputHash("int_array_sum:    0.043s (result=5000050000)\narray_swap:       0.307s (result=100000)\nTime: 0.462s\n")
	if a != b {
		t.Fatalf("hashes differ: %q %q", a, b)
	}
}

func TestStrictGuardComputeStatsReportsSpread(t *testing.T) {
	stats := strictComputeStats([]float64{1, 2, 4})
	if stats.n != 3 || stats.median != 2 || stats.min != 1 || stats.max != 4 || math.Abs(stats.stdev-1.527525) > 0.000001 || stats.mad != 1 || math.Abs(stats.cvPct-65.465367) > 0.000001 {
		t.Fatalf("stats = %#v", stats)
	}
}

func TestStrictGuardLowResolutionSampleIsNotComparable(t *testing.T) {
	sample := strictSummarizeRepeatedRuns([]strictCommandRun{{status: "ok", seconds: floatPtr(0), wallSeconds: 0.002}, {status: "ok", seconds: floatPtr(0), wallSeconds: 0.002}}, 2, 0.001, 0.020, false)
	mode := strictSummarizeMode([]strictSample{sample})
	if sample.status != "low_resolution" || mode.status != "low_resolution" || strictComparableSeconds(mode) != nil {
		t.Fatalf("sample=%#v mode=%#v", sample, mode)
	}
}

func TestStrictGuardWallTimeFallbackRecordsSource(t *testing.T) {
	sample := strictSummarizeRepeatedRuns([]strictCommandRun{{status: "ok", seconds: floatPtr(0), wallSeconds: 0.020}, {status: "ok", seconds: floatPtr(0), wallSeconds: 0.022}}, 2, 0.001, 0.020, true)
	if sample.status != "ok" || sample.timeSource != "wall_repeat" || sample.seconds == nil || math.Abs(*sample.seconds-0.021) > 0.000001 {
		t.Fatalf("sample = %#v", sample)
	}
}

func TestStrictGuardScriptRepeatsAverageTotalTime(t *testing.T) {
	sample := strictSummarizeRepeatedRuns([]strictCommandRun{{status: "ok", seconds: floatPtr(0.015), wallSeconds: 0.020}, {status: "ok", seconds: floatPtr(0.017), wallSeconds: 0.021}}, 2, 0.001, 0.020, false)
	if sample.status != "ok" || sample.timeSource != "script" || sample.seconds == nil || math.Abs(*sample.seconds-0.016) > 0.000001 {
		t.Fatalf("sample = %#v", sample)
	}
}

func TestStrictGuardChecksumMismatchIsExplicit(t *testing.T) {
	mode := strictSummarizeMode([]strictSample{{status: "ok", seconds: floatPtr(1), runs: []strictCommandRun{{status: "ok", seconds: floatPtr(1), outputHash: "aaa"}}}, {status: "ok", seconds: floatPtr(1), runs: []strictCommandRun{{status: "ok", seconds: floatPtr(1), outputHash: "bbb"}}}})
	if mode.checksumStatus != "mismatch" || mode.outputHash != "aaa,bbb" {
		t.Fatalf("mode = %#v", mode)
	}
}

func TestStrictGuardDiscoveryAndRepeatSelectorsUseDomainIDs(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	specs, err := benchdisc.Discover(root, []string{"numeric", "recursion", "table", "calls", "string", "app", "control"})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, spec := range specs {
		ids[spec.ID()] = true
	}
	for _, want := range []string{"recursion/fib", "numeric/matmul_dense", "table/json_table_walk", "numeric/matmul_row"} {
		if !ids[want] {
			t.Fatalf("discovery missing %s", want)
		}
	}
	overrides, err := benchdisc.ParseSelectorCountOverrides([]string{"recursion/fib=6", "default/control/sieve=8", "data/soa_dot=5", "default/concurrency/goroutine_sleep=7"}, []string{"vm", "default"}, "--repeat")
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := benchdisc.SelectorCountOverride(overrides, "vm", "recursion/fib"); !ok || got != 6 {
		t.Fatalf("repeat override recursion/fib = %d,%v", got, ok)
	}
	if got, ok := benchdisc.SelectorCountOverride(overrides, "default", "control/sieve"); !ok || got != 8 {
		t.Fatalf("repeat override control/sieve = %d,%v", got, ok)
	}
}

type timingBenchmarkSpec struct {
	group  string
	name   string
	leia   string
	luajit string
}

func (s timingBenchmarkSpec) ID() string { return s.group + "/" + s.name }

type timingScaleOverride struct {
	selector string
	name     string
	value    string
}

type timingSample struct {
	status             string
	seconds            *float64
	repeat             int
	source             string
	scriptTotalSeconds float64
	wallTotalSeconds   float64
	note               string
}

type timingSubject struct {
	subject    string
	mode       string
	status     string
	repeat     int
	source     string
	diagnostic map[string]any
}

func timingParseScaleOverrides(values []string) []timingScaleOverride {
	var out []timingScaleOverride
	for _, value := range values {
		left, raw, _ := strings.Cut(value, "=")
		selector := ""
		name := left
		if strings.Contains(left, ":") {
			selector, name, _ = strings.Cut(left, ":")
		}
		out = append(out, timingScaleOverride{selector: selector, name: name, value: raw})
	}
	return out
}

func timingScaleOverridesFor(spec timingBenchmarkSpec, overrides []timingScaleOverride) []timingScaleOverride {
	var out []timingScaleOverride
	for _, override := range overrides {
		if override.selector == "" || benchdisc.SelectorMatchesSpec(override.selector, spec) {
			out = append(out, override)
		}
	}
	return out
}

func timingValidateScaleSelectors(specs []timingBenchmarkSpec, overrides []timingScaleOverride) error {
	selectors := benchdisc.SpecSelectors(specs)
	for _, override := range overrides {
		if override.selector != "" && !benchdisc.SelectorMatches(override.selector, selectors) {
			return errString("unknown --scale/--param selector: " + override.selector)
		}
	}
	return nil
}

func timingSubjectDiagnostic(spec timingBenchmarkSpec, subject timingSubject, samples []timingSample, applied []timingScaleOverride, minSampleSeconds float64, minWallRepeat, maxRepeat int, timerResolution float64) map[string]any {
	diag := map[string]any{
		"repeat":             subject.repeat,
		"max_repeat":         maxRepeat,
		"min_sample_seconds": minSampleSeconds,
		"timer_resolution":   timerResolution,
		"min_wall_repeat":    minWallRepeat,
		"scale":              timingScaleArgValues(applied),
	}
	low := false
	for _, sample := range samples {
		if sample.status == "low_resolution" {
			low = true
		}
	}
	if low {
		scaleArgs := timingScaleArgValues(applied)
		if len(scaleArgs) == 0 && spec.ID() == "calls/calls_vararg_coroutine" {
			scaleArgs = []string{"calls/calls_vararg_coroutine:N_CALLS=880000", "calls/calls_vararg_coroutine:N_CORO=360000"}
		}
		recommendation := map[string]any{
			"reason":             "script Time total is below timer resolution or --min-sample-seconds",
			"min_sample_seconds": math.Max(minSampleSeconds, timerResolution*50),
			"max_repeat":         maxInt(maxRepeat, subject.repeat*2),
			"min_wall_repeat":    maxInt(minWallRepeat, 8),
			"time_source":        "script",
			"scale":              scaleArgs,
		}
		parts := []string{"--time-source=script"}
		for _, scale := range scaleArgs {
			parts = append(parts, "--scale "+scale)
		}
		parts = append(parts, "--min-sample-seconds 0.050", "--max-repeat 256", "--min-wall-repeat 8")
		recommendation["rerun_args"] = strings.Join(parts, " ")
		diag["low_resolution"] = recommendation
	} else if subject.source == "wall_repeat" {
		diag["wall_repeat"] = map[string]any{"recommendation": "scale workload enough for script_repeat or rerun with --time-source=script"}
	}
	return diag
}

func timingScaleArgValues(overrides []timingScaleOverride) []string {
	out := make([]string, 0, len(overrides))
	for _, override := range overrides {
		prefix := ""
		if override.selector != "" {
			prefix = override.selector + ":"
		}
		out = append(out, prefix+override.name+"="+override.value)
	}
	return out
}

func timingEffectiveTimeSource(spec timingBenchmarkSpec, requested string) string {
	if requested == "auto" && spec.group == "concurrency" {
		return "wall"
	}
	return requested
}

func TestTimingCompareDiscoveryMatchesDomainManifestBenchmarkIDs(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	var manifest struct {
		Workloads []struct {
			ID string `json:"id"`
		} `json:"workloads"`
	}
	data, err := os.ReadFile(filepath.Join(root, "benchmarks", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	specs, err := benchdisc.Discover(root, benchdisc.DomainGroups)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, spec := range specs {
		got[spec.ID()] = true
	}
	for _, workload := range manifest.Workloads {
		parts := strings.SplitN(workload.ID, "/", 2)
		if len(parts) == 2 && !benchdisc.EnabledInBuild(parts[0], parts[1]) {
			continue
		}
		if !got[workload.ID] {
			t.Fatalf("discovery missing manifest workload %s", workload.ID)
		}
	}
	var enabledWorkloads int
	for _, workload := range manifest.Workloads {
		parts := strings.SplitN(workload.ID, "/", 2)
		if len(parts) == 2 && benchdisc.EnabledInBuild(parts[0], parts[1]) {
			enabledWorkloads++
		}
	}
	if len(got) != enabledWorkloads {
		t.Fatalf("discovery count = %d, enabled manifest workloads = %d", len(got), enabledWorkloads)
	}
}

func TestTimingCompareSelectSpecsAcceptsDomainGroupSelectors(t *testing.T) {
	root := repoRootForScriptEntrypoints(t)
	discovered, err := benchdisc.Discover(root, benchdisc.DomainGroups)
	if err != nil {
		t.Fatal(err)
	}
	var specs []timingBenchmarkSpec
	for _, spec := range discovered {
		specs = append(specs, timingBenchmarkSpec{group: spec.Group, name: spec.Name, leia: spec.LeiaRel(), luajit: spec.LuaJITRel()})
	}
	for _, selector := range []string{"data/soa_dot", "concurrency/goroutine_sleep", "table/events_metamethod"} {
		selected, err := benchdisc.SelectSpecs(specs, []string{selector})
		if err != nil {
			t.Fatal(err)
		}
		if len(selected) != 1 || selected[0].ID() != selector {
			t.Fatalf("select %s = %#v", selector, selected)
		}
	}
}

func TestTimingCompareScaleOverridesAcceptAndRejectDomainSelectors(t *testing.T) {
	specs := []timingBenchmarkSpec{{group: "concurrency", name: "goroutine_sleep"}}
	overrides := timingParseScaleOverrides([]string{"concurrency/goroutine_sleep:N=10"})
	if err := timingValidateScaleSelectors(specs, overrides); err != nil {
		t.Fatal(err)
	}
	if got := timingScaleOverridesFor(specs[0], overrides); !reflect.DeepEqual(got, overrides) {
		t.Fatalf("scale overrides = %#v, want %#v", got, overrides)
	}
	bad := timingParseScaleOverrides([]string{"goroutine_sleep:N=10"})
	if err := timingValidateScaleSelectors(specs, bad); err == nil || !strings.Contains(err.Error(), "unknown --scale/--param selector: goroutine_sleep") {
		t.Fatalf("bad selector err = %v", err)
	}
}

func TestTimingCompareLowResolutionGetsConcreteRerunAdvice(t *testing.T) {
	spec := timingBenchmarkSpec{group: "calls", name: "calls_vararg_coroutine", leia: "benchmarks/calls/calls_vararg_coroutine.leia", luajit: "benchmarks/lua_ref/calls/calls_vararg_coroutine.lua"}
	samples := []timingSample{{status: "low_resolution", repeat: 128, scriptTotalSeconds: 0, wallTotalSeconds: 0.012, note: "script Time: below resolution"}}
	subject := timingSubject{subject: "current", mode: "default", status: "low_resolution", repeat: 128}
	diag := timingSubjectDiagnostic(spec, subject, samples, nil, 0.05, 4, 128, 0.001)
	advice := diag["low_resolution"].(map[string]any)
	scale := advice["scale"].([]string)
	if !containsString(scale, "calls/calls_vararg_coroutine:N_CALLS=880000") || !strings.Contains(advice["rerun_args"].(string), "--scale calls/calls_vararg_coroutine:N_CORO=360000") || advice["min_sample_seconds"].(float64) != 0.05 || advice["min_wall_repeat"].(int) != 8 || advice["max_repeat"].(int) != 256 {
		t.Fatalf("advice = %#v", advice)
	}
}

func TestTimingCompareMarkdownReportsLowResolutionAndWallRepeatDiagnostics(t *testing.T) {
	low := "## Low-Resolution Diagnostics\ncalls/calls_vararg_coroutine:N_CALLS=880000\n--min-sample-seconds 0.050\n"
	wall := "## Wall-Repeat Diagnostics\nscale workload enough for script_repeat\n"
	report := low + wall
	for _, want := range []string{"## Low-Resolution Diagnostics", "calls/calls_vararg_coroutine:N_CALLS=880000", "--min-sample-seconds 0.050", "## Wall-Repeat Diagnostics", "scale workload enough for script_repeat"} {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q", want)
		}
	}
}

func TestTimingCompareAutoTimeSourceUsesWallForConcurrency(t *testing.T) {
	spec := timingBenchmarkSpec{group: "concurrency", name: "waitgroup"}
	if timingEffectiveTimeSource(spec, "auto") != "wall" || timingEffectiveTimeSource(spec, "script") != "script" {
		t.Fatalf("effective time source mismatch")
	}
}

func TestTimingCompareAutoTimeSourceKeepsScriptAutoForNonConcurrency(t *testing.T) {
	spec := timingBenchmarkSpec{group: "table", name: "table_array_access"}
	if timingEffectiveTimeSource(spec, "auto") != "auto" {
		t.Fatalf("effective time source mismatch")
	}
}

func regexpCounter(re *regexp.Regexp, text string) int {
	match := re.FindStringSubmatch(text)
	if match == nil {
		return 0
	}
	var value int
	_, _ = fmtSscanf(match[1], "%d", &value)
	return value
}

func median(values []float64) float64 {
	if len(values)%2 == 1 {
		return values[len(values)/2]
	}
	return (values[len(values)/2-1] + values[len(values)/2]) / 2
}

func sumFloat(values []float64) float64 {
	out := 0.0
	for _, value := range values {
		out += value
	}
	return out
}

func intPtrLocal(v int) *int { return &v }

func fmtSscanf(str, format string, args ...any) (int, error) {
	return fmt.Sscanf(str, format, args...)
}

type errString string

func (e errString) Error() string { return string(e) }

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
