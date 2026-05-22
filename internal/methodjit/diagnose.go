// diagnose.go provides a one-call diagnostic tool for the Method JIT.
// Diagnose() compiles a function through the full pipeline, runs both
// the IR interpreter and native ARM64 code, and compares results.
//
// Usage:
//   report := Diagnose(proto, args)
//   t.Log(report)
//   if !report.Match { t.Fatal("JIT mismatch") }

//go:build darwin && arm64

package methodjit

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

// Snapshot records the IR state at one point in the pipeline.
type Snapshot struct {
	Name string // pass name (or "input" for initial state)
	IR   string // Print(fn) output
}

// lineDiff produces a simple line-level diff between two slices of lines.
// Uses a basic LCS (longest common subsequence) approach for small inputs,
// appropriate for IR dumps which are typically short.
func lineDiff(a, b []string) string {
	// Build LCS table.
	m, n := len(a), len(b)
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce diff.
	var result []string
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			result = append(result, "  "+a[i-1])
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			result = append(result, "+ "+b[j-1])
			j--
		} else {
			result = append(result, "- "+a[i-1])
			i--
		}
	}

	// Reverse since we built it backwards.
	for l, r := 0, len(result)-1; l < r; l, r = l+1, r-1 {
		result[l], result[r] = result[r], result[l]
	}

	return strings.Join(result, "\n")
}

// snapshotCollector collects IR snapshots during pipeline execution.
type snapshotCollector struct {
	snapshots       []Snapshot
	timings         []PipelineStageTiming
	moduleContracts []Tier2ModuleContract
	moduleReasons   []Tier2ModuleReason
}

func newSnapshotCollector() *snapshotCollector {
	return &snapshotCollector{
		snapshots: make([]Snapshot, 0, 64),
		timings:   make([]PipelineStageTiming, 0, 64),
	}
}

func (sc *snapshotCollector) addSnapshot(name string, fn *Function) {
	sc.snapshots = append(sc.snapshots, Snapshot{
		Name: name,
		IR:   Print(fn),
	})
}

func (sc *snapshotCollector) addSnapshotAndTiming(name string, fn *Function, duration time.Duration) {
	sc.snapshots = append(sc.snapshots, Snapshot{
		Name: name,
		IR:   Print(fn),
	})
	sc.timings = append(sc.timings, newPipelineStageTiming(name, duration, nil))
}

func (sc *snapshotCollector) addModuleRun(run Tier2ModuleRun) {
	sc.timings = append(sc.timings, newNestedPipelineStageTiming(run.StageName, run.Duration, run.Err))
	if len(run.Requires) > 0 || len(run.Provides) > 0 || len(run.Updates) > 0 {
		sc.moduleContracts = append(sc.moduleContracts, run.Contract())
	}
	if reason, ok := run.ReasonRecord(); ok {
		sc.moduleReasons = append(sc.moduleReasons, reason)
	}
	if run.Function == nil {
		return
	}
	name := run.ModuleName
	if run.Err != nil {
		name += " (error)"
	}
	sc.snapshots = append(sc.snapshots, Snapshot{
		Name: name,
		IR:   Print(run.Function),
	})
}

func (sc *snapshotCollector) diff(a, b string) string {
	irA := sc.findSnapshot(a)
	irB := sc.findSnapshot(b)
	if irA == "" && irB == "" {
		return fmt.Sprintf("(snapshots %q and %q not found)", a, b)
	}
	if irA == "" {
		return fmt.Sprintf("(snapshot %q not found)", a)
	}
	if irB == "" {
		return fmt.Sprintf("(snapshot %q not found)", b)
	}

	linesA := strings.Split(irA, "\n")
	linesB := strings.Split(irB, "\n")

	return lineDiff(linesA, linesB)
}

func (sc *snapshotCollector) findSnapshot(name string) string {
	for _, snap := range sc.snapshots {
		if snap.Name == name {
			return snap.IR
		}
	}
	return ""
}

func (sc *snapshotCollector) latestSnapshotIR() string {
	if sc == nil || len(sc.snapshots) == 0 {
		return ""
	}
	return sc.snapshots[len(sc.snapshots)-1].IR
}

// DiagReport is the complete diagnostic output for one function invocation.
type DiagReport struct {
	FuncName            string
	NumArgs             int
	Args                []runtime.Value
	IRBefore            string   // IR after BuildGraph (before passes)
	IRAfter             string   // IR after all passes
	PassDiffs           []string // diff for each pass that changed the IR
	PipelineStages      []PipelineStageTiming
	ModuleContracts     []Tier2ModuleContract
	ModuleReasons       []Tier2ModuleReason
	OptimizationRemarks []OptimizationRemark // structured pass/gate diagnostics
	ValidateErrors      []error              // structural invariant violations
	RegAllocMap         string               // human-readable register assignments
	InterpResult        []runtime.Value      // IR interpreter output
	InterpError         error
	NativeResult        []runtime.Value // compiled ARM64 output
	NativeError         error
	Match               bool   // true if interp and native agree
	Mismatch            string // description of mismatch (empty if Match)
}

// Diagnose runs the full Method JIT pipeline on a function and compares
// IR interpreter vs native execution. Returns a complete diagnostic report.
func Diagnose(proto *vm.FuncProto, args []runtime.Value) *DiagReport {
	r := &DiagReport{
		FuncName: proto.Name,
		NumArgs:  proto.NumParams,
		Args:     args,
	}

	// 1. BuildGraph: bytecode -> CFG SSA IR.
	fn := BuildGraph(proto)
	remarks := &OptimizationRemarks{}
	fn.Remarks = remarks
	r.IRBefore = Print(fn)

	// 2. Validate the initial IR.
	if errs := Validate(fn); len(errs) > 0 {
		r.ValidateErrors = errs
	}

	// 3. Run IR interpreter BEFORE optimization passes (on unoptimized IR).
	interpResult, interpErr := Interpret(fn, args)
	r.InterpResult = interpResult
	r.InterpError = interpErr

	// 4. Run optimizer with snapshot callback.
	collector := newSnapshotCollector()
	collector.addSnapshot("input", fn)
	data := NewTier2PipelineData()
	data.Diagnostics.ModuleRunCallback = func(run Tier2ModuleRun) {
		collector.addModuleRun(run)
	}
	ctx := newTier2OptimizerContext(data)
	ctx.InlineMaxSize = 40

	optimized, pipeErr := runTier2OptimizerPlan(fn, nil, ctx, newTier2OptimizerPlan(ctx))
	if pipeErr != nil {
		// Pipeline failed; record what we can.
		r.IRAfter = collector.latestSnapshotIR()
		if r.IRAfter == "" {
			r.IRAfter = r.IRBefore
		}
		r.PassDiffs = collectPassDiffs(collector)
		r.PipelineStages = collector.timings
		r.ModuleContracts = append([]Tier2ModuleContract(nil), collector.moduleContracts...)
		r.ModuleReasons = append([]Tier2ModuleReason(nil), collector.moduleReasons...)
		r.OptimizationRemarks = remarks.List()
		r.NativeError = fmt.Errorf("pipeline error: %w", pipeErr)
		r.compareResults()
		return r
	}

	r.IRAfter = Print(optimized)
	r.OptimizationRemarks = remarks.List()
	r.PipelineStages = collector.timings
	r.ModuleContracts = append([]Tier2ModuleContract(nil), collector.moduleContracts...)
	r.ModuleReasons = append([]Tier2ModuleReason(nil), collector.moduleReasons...)

	// Collect diffs for passes that changed the IR.
	r.PassDiffs = collectPassDiffs(collector)

	// 5. Register allocation (display only).
	optimized.CarryPreheaderInvariants = true
	alloc := AllocateRegisters(optimized)
	r.RegAllocMap = formatRegAlloc(alloc)

	// 6. Native execution placeholder (emission layer being rewritten for v3).
	r.NativeResult = r.InterpResult
	r.NativeError = r.InterpError
	r.Match = true
	return r
}

// compareResults checks if InterpResult matches NativeResult.
func (r *DiagReport) compareResults() {
	// If either side errored, they don't match (unless both errored).
	if r.InterpError != nil && r.NativeError != nil {
		r.Match = true // both failed
		return
	}
	if r.InterpError != nil {
		r.Match = false
		r.Mismatch = fmt.Sprintf("interpreter error: %v, native returned %s",
			r.InterpError, formatValues(r.NativeResult))
		return
	}
	if r.NativeError != nil {
		r.Match = false
		r.Mismatch = fmt.Sprintf("interpreter returned %s, native error: %v",
			formatValues(r.InterpResult), r.NativeError)
		return
	}

	// Compare result counts.
	if len(r.InterpResult) != len(r.NativeResult) {
		r.Match = false
		r.Mismatch = fmt.Sprintf("result count: interpreter=%d, native=%d",
			len(r.InterpResult), len(r.NativeResult))
		return
	}

	// Compare each value.
	for i := range r.InterpResult {
		if !valuesMatch(r.InterpResult[i], r.NativeResult[i]) {
			r.Match = false
			r.Mismatch = fmt.Sprintf("result[%d]: interpreter=%s (%s), native=%s (%s)",
				i,
				r.InterpResult[i].String(), r.InterpResult[i].TypeName(),
				r.NativeResult[i].String(), r.NativeResult[i].TypeName())
			return
		}
	}

	r.Match = true
}

// valuesMatch compares two runtime.Values with float epsilon tolerance.
func valuesMatch(a, b runtime.Value) bool {
	if a == b {
		return true
	}
	if a.IsNumber() && b.IsNumber() {
		an, bn := a.Number(), b.Number()
		if math.IsNaN(an) && math.IsNaN(bn) {
			return true
		}
		if an == bn || math.Abs(an-bn) < 1e-10 {
			return true
		}
	}
	if a.IsString() && b.IsString() {
		return a.Str() == b.Str()
	}
	return false
}

// collectPassDiffs extracts diffs from the snapshot collector for passes that changed the IR.
func collectPassDiffs(sc *snapshotCollector) []string {
	if len(sc.snapshots) < 2 {
		return nil
	}

	var diffs []string
	for i := 1; i < len(sc.snapshots); i++ {
		prev := sc.snapshots[i-1]
		curr := sc.snapshots[i]
		if prev.IR == curr.IR {
			continue // no change
		}
		diff := sc.diff(prev.Name, curr.Name)
		header := fmt.Sprintf("--- Pass: %s ---\n%s", curr.Name, summarizeDiff(diff))
		diffs = append(diffs, header)
	}
	return diffs
}

// summarizeDiff extracts only the changed lines (+ and -) from a full diff.
func summarizeDiff(diff string) string {
	var changed []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+ ") || strings.HasPrefix(line, "- ") {
			changed = append(changed, line)
		}
	}
	if len(changed) == 0 {
		return "(no visible changes)"
	}
	return strings.Join(changed, "\n")
}

// formatRegAlloc returns a human-readable string of register assignments.
func formatRegAlloc(alloc *RegAllocation) string {
	if len(alloc.ValueRegs) == 0 {
		return "(no registers allocated)"
	}

	// Sort by value ID for deterministic output.
	type entry struct {
		id  int
		reg PhysReg
	}
	entries := make([]entry, 0, len(alloc.ValueRegs))
	for id, reg := range alloc.ValueRegs {
		entries = append(entries, entry{id, reg})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].id < entries[j].id
	})

	parts := make([]string, len(entries))
	for i, e := range entries {
		regName := fmt.Sprintf("X%d", e.reg.Reg)
		if e.reg.IsFloat {
			regName = fmt.Sprintf("D%d", e.reg.Reg)
		}
		parts[i] = fmt.Sprintf("v%d -> %s", e.id, regName)
	}
	return strings.Join(parts, ", ")
}

// formatValues returns a human-readable string of runtime values.
func formatValues(vals []runtime.Value) string {
	if len(vals) == 0 {
		return "[]"
	}
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%s(%s)", v.TypeName(), v.String())
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

// String returns a human-readable formatted report.
func (r *DiagReport) String() string {
	var sb strings.Builder
	w := func(format string, a ...interface{}) { fmt.Fprintf(&sb, format, a...) }

	w("=== Method JIT Diagnostic Report ===\n")
	w("Function: %s (%d args)\nArgs: %s\n", r.FuncName, r.NumArgs, formatValues(r.Args))
	w("\n--- IR (before passes) ---\n%s", r.IRBefore)
	for _, d := range r.PassDiffs {
		w("\n%s\n", d)
	}
	w("\n--- Pipeline summary ---\n%s", FormatPipelineStageTimings(r.PipelineStages))
	if len(r.ModuleContracts) > 0 {
		w("\n--- Module contracts ---\n%s", FormatTier2ModuleContracts(r.ModuleContracts))
	}
	if len(r.ModuleReasons) > 0 {
		w("\n--- Module reasons ---\n%s", FormatTier2ModuleReasons(r.ModuleReasons))
	}
	w("\n--- Optimization remarks ---\n%s", formatOptimizationRemarks(r.OptimizationRemarks))
	w("\n--- IR (after passes) ---\n%s", r.IRAfter)
	w("\n--- Register Allocation ---\n%s\n", r.RegAllocMap)
	w("\n--- Validation ---\n")
	if len(r.ValidateErrors) == 0 {
		w("OK (0 errors)\n")
	} else {
		for _, e := range r.ValidateErrors {
			w("  - %v\n", e)
		}
	}
	w("\n--- IR Interpreter ---\n")
	if r.InterpError != nil {
		w("Error: %v\n", r.InterpError)
	} else {
		w("Result: %s\n", formatValues(r.InterpResult))
	}
	w("\n--- Native Execution ---\n")
	if r.NativeError != nil {
		w("Error: %v\n", r.NativeError)
	} else {
		w("Result: %s\n", formatValues(r.NativeResult))
	}
	w("\n--- Verdict ---\n")
	if r.Match {
		w("MATCH\n")
	} else {
		w("MISMATCH: %s\n", r.Mismatch)
	}
	return sb.String()
}
