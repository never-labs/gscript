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

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
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
	moduleFactDiffs []Tier2ModuleFactDiff
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
	if len(run.Requires) > 0 || len(run.Provides) > 0 || len(run.Updates) > 0 || len(run.OptionalReads) > 0 {
		sc.moduleContracts = append(sc.moduleContracts, run.Contract())
	}
	if reason, ok := run.ReasonRecord(); ok {
		sc.moduleReasons = append(sc.moduleReasons, reason)
	}
	if diff, ok := run.FactDiffRecord(); ok {
		sc.moduleFactDiffs = append(sc.moduleFactDiffs, diff)
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
	FuncName                   string
	NumArgs                    int
	Args                       []runtime.Value
	IRBefore                   string   // IR after BuildGraph (before passes)
	IRAfter                    string   // IR after all passes
	PassDiffs                  []string // diff for each pass that changed the IR
	PipelineStages             []PipelineStageTiming
	ModuleContracts            []Tier2ModuleContract
	ModuleReasons              []Tier2ModuleReason
	ModuleFactDiffs            []Tier2ModuleFactDiff
	OptimizationRemarks        []OptimizationRemark     // structured pass/gate diagnostics
	QQueryHotPaths             []QQueryHotPath          // q query primitive pipelines visible in final IR
	QQueryHotPathShapes        map[string]int           // q query primitive pipeline count by shape
	QVectorWhereHotPaths       []QVectorWhereHotPath    // q vector conditional projection pipelines visible in final IR
	QVectorWhereHotPathShapes  map[string]int           // q vector conditional projection count by shape
	QVectorReduceHotPaths      []QVectorReduceHotPath   // q vector aggregate pipelines visible in final IR
	QVectorReduceHotPathShapes map[string]int           // q vector aggregate count by shape
	QVectorRuntimeKernels      []QVectorRuntimeKernel   // q vector primitives carried by typed runtime op-exits
	QVectorRuntimeKernelShapes map[string]int           // q vector typed runtime-kernel count by shape
	QTypedRuntimeKernels       []QFrameSelectColumnSpec // q query primitive pipelines lowered to typed runtime-kernel op-exits
	QTypedRuntimeKernelShapes  map[string]int           // lowered q typed runtime-kernel count by shape
	QQueryFallbacks            map[string]int           // q native lowering fallback count by reason code
	QVectorLoweringFallbacks   map[string]int           // q vector native lowering fallback count by reason code
	QKernelDescriptors         []QKernelDescriptor      // normalized q kernel runtime/fallback observations
	QKernelExecutionStats      []QKernelExecutionStat   // observed q typed-runtime kernel execution outcomes
	QKernelShapeSummary        []QKernelShapeSummary    // source-stable q kernel shape/fallback summary
	ValidateErrors             []error                  // structural invariant violations
	RegAllocMap                string                   // human-readable register assignments
	InterpResult               []runtime.Value          // IR interpreter output on UNOPTIMIZED IR
	InterpError                error
	OptInterpResult            []runtime.Value // IR interpreter output on OPTIMIZED IR
	OptInterpError             error
	NativeResult               []runtime.Value // compiled ARM64 output (OPTIMIZED IR)
	NativeError                error

	// Three-way verdicts. The oracle interprets the unoptimized IR, interprets
	// the optimized IR, and executes the optimized IR as native code. Comparing
	// the three pairwise separates optimizer bugs from backend/codegen bugs.

	// OptimizerMatch compares unoptimized-interp vs optimized-interp. A mismatch
	// here means an optimization PASS changed observable semantics.
	OptimizerMatch    bool
	OptimizerMismatch string

	// BackendMatch compares optimized-interp vs optimized-native. A mismatch
	// here means codegen/backend diverged from the IR it was given.
	BackendMatch    bool
	BackendMismatch string

	// Match is the end-to-end verdict (unoptimized-interp vs native), kept for
	// backward compatibility with existing callers/tests.
	Match    bool   // true if unoptimized-interp and native agree
	Mismatch string // description of mismatch (empty if Match)
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
		r.ModuleFactDiffs = append([]Tier2ModuleFactDiff(nil), collector.moduleFactDiffs...)
		r.OptimizationRemarks = remarks.List()
		r.QQueryHotPaths = DetectQQueryHotPaths(fn)
		r.QQueryHotPathShapes = CountQQueryHotPathShapes(r.QQueryHotPaths)
		r.QVectorWhereHotPaths = DetectQVectorWhereHotPaths(fn)
		r.QVectorWhereHotPathShapes = CountQVectorWhereHotPathShapes(r.QVectorWhereHotPaths)
		r.QVectorReduceHotPaths = DetectQVectorReduceHotPaths(fn)
		r.QVectorReduceHotPathShapes = CountQVectorReduceHotPathShapes(r.QVectorReduceHotPaths)
		r.QVectorRuntimeKernels = DetectQVectorRuntimeKernels(fn)
		r.QVectorRuntimeKernelShapes = CountQVectorRuntimeKernelShapes(r.QVectorRuntimeKernels)
		r.QTypedRuntimeKernels = append([]QFrameSelectColumnSpec(nil), fn.QFrameSelectColumnSpecs...)
		r.QTypedRuntimeKernelShapes = CountQFrameSelectColumnSpecShapes(r.QTypedRuntimeKernels)
		r.QQueryFallbacks = CountQQueryLoweringFallbackReasons(r.OptimizationRemarks)
		r.QVectorLoweringFallbacks = CountQVectorLoweringFallbackReasons(r.OptimizationRemarks)
		r.QKernelDescriptors = BuildQKernelDescriptors(r.QVectorRuntimeKernels, r.QTypedRuntimeKernels, r.OptimizationRemarks)
		r.QKernelShapeSummary = BuildQKernelShapeSummaryFromDescriptors(r.QKernelDescriptors)
		r.NativeError = fmt.Errorf("pipeline error: %w", pipeErr)
		r.compareResults()
		return r
	}

	r.IRAfter = Print(optimized)
	r.OptimizationRemarks = remarks.List()
	r.QQueryHotPaths = DetectQQueryHotPaths(optimized)
	r.QQueryHotPathShapes = CountQQueryHotPathShapes(r.QQueryHotPaths)
	r.QVectorWhereHotPaths = DetectQVectorWhereHotPaths(optimized)
	r.QVectorWhereHotPathShapes = CountQVectorWhereHotPathShapes(r.QVectorWhereHotPaths)
	r.QVectorReduceHotPaths = DetectQVectorReduceHotPaths(optimized)
	r.QVectorReduceHotPathShapes = CountQVectorReduceHotPathShapes(r.QVectorReduceHotPaths)
	r.QVectorRuntimeKernels = DetectQVectorRuntimeKernels(optimized)
	r.QVectorRuntimeKernelShapes = CountQVectorRuntimeKernelShapes(r.QVectorRuntimeKernels)
	r.QTypedRuntimeKernels = append([]QFrameSelectColumnSpec(nil), optimized.QFrameSelectColumnSpecs...)
	r.QTypedRuntimeKernelShapes = CountQFrameSelectColumnSpecShapes(r.QTypedRuntimeKernels)
	r.QQueryFallbacks = CountQQueryLoweringFallbackReasons(r.OptimizationRemarks)
	r.QVectorLoweringFallbacks = CountQVectorLoweringFallbackReasons(r.OptimizationRemarks)
	r.QKernelDescriptors = BuildQKernelDescriptors(r.QVectorRuntimeKernels, r.QTypedRuntimeKernels, r.OptimizationRemarks)
	r.QKernelShapeSummary = BuildQKernelShapeSummaryFromDescriptors(r.QKernelDescriptors)
	r.PipelineStages = collector.timings

	// 4b. Interpret the OPTIMIZED IR. This is the middle of the three-way
	//     oracle: comparing it against the unoptimized-interp result isolates
	//     optimizer bugs, and comparing it against native isolates backend bugs.
	optInterpResult, optInterpErr := Interpret(optimized, args)
	r.OptInterpResult = optInterpResult
	r.OptInterpError = optInterpErr
	r.ModuleContracts = append([]Tier2ModuleContract(nil), collector.moduleContracts...)
	r.ModuleReasons = append([]Tier2ModuleReason(nil), collector.moduleReasons...)
	r.ModuleFactDiffs = append([]Tier2ModuleFactDiff(nil), collector.moduleFactDiffs...)

	// Collect diffs for passes that changed the IR.
	r.PassDiffs = collectPassDiffs(collector)

	// 5. Register allocation.
	optimized.CarryPreheaderInvariants = true
	alloc := AllocateRegisters(optimized)
	r.RegAllocMap = formatRegAlloc(alloc)

	// 6. Compile to native ARM64 and execute, then compare against the IR
	//    interpreter result. The interpreter ran on the UNOPTIMIZED IR and the
	//    native code runs the OPTIMIZED IR, so a match validates the whole
	//    pipeline end-to-end (optimization passes + codegen) in one check.
	cf, compileErr := Compile(optimized, alloc)
	if compileErr != nil {
		r.NativeError = fmt.Errorf("compile error: %w", compileErr)
		r.compareResults()
		return r
	}
	defer cf.Code.Free()
	nativeResult, nativeErr := cf.Execute(args)
	r.NativeResult = nativeResult
	r.NativeError = nativeErr
	r.QKernelExecutionStats = cf.QKernelExecutionStats()
	r.QKernelShapeSummary = BuildQKernelShapeSummaryFromDescriptorsAndExecutionStats(r.QKernelDescriptors, r.QKernelExecutionStats)
	r.compareResults()
	return r
}

// compareResults computes the three pairwise verdicts of the oracle:
//
//   - OptimizerMatch: unoptimized-interp vs optimized-interp (optimizer bugs)
//   - BackendMatch:   optimized-interp vs optimized-native   (codegen bugs)
//   - Match:          unoptimized-interp vs native (end-to-end, backward compat)
//
// Each pairing reuses compareValueLists so the value/error comparison policy is
// shared. When the optimizer failed to produce an optimized IR (pipeline or
// compile error before OptInterp ran), OptInterpResult/OptInterpError are zero;
// in that case the optimizer/backend sub-verdicts only carry meaning insofar as
// the underlying comparison allows, and the end-to-end Match remains the
// authoritative result.
func (r *DiagReport) compareResults() {
	r.OptimizerMatch, r.OptimizerMismatch = compareValueLists(
		r.InterpResult, r.OptInterpResult, r.InterpError, r.OptInterpError,
		"unopt-interp", "opt-interp")
	r.BackendMatch, r.BackendMismatch = compareValueLists(
		r.OptInterpResult, r.NativeResult, r.OptInterpError, r.NativeError,
		"opt-interp", "native")
	r.Match, r.Mismatch = compareValueLists(
		r.InterpResult, r.NativeResult, r.InterpError, r.NativeError,
		"unopt-interp", "native")
}

// compareValueLists compares two execution outcomes (value list + error) and
// returns whether they match plus a human-readable mismatch description.
//
// Error policy: we require the two sides to be in the same error CATEGORY.
// errorCategory classifies an error as nil / "deopt" / "runtime error". Both
// nil is a match (and we go on to compare values). Both non-nil matches ONLY if
// they fall in the same category — this avoids the previous blanket "both
// errored => Match" leniency (which masked a real divergence whenever the two
// engines failed for unrelated reasons) without being as brittle as requiring
// byte-identical error strings (deopt messages legitimately carry differing PC
// / reason text). One side erroring and the other not is always a mismatch.
func compareValueLists(a, b []runtime.Value, aErr, bErr error, aName, bName string) (bool, string) {
	aCat := errorCategory(aErr)
	bCat := errorCategory(bErr)

	if aErr != nil || bErr != nil {
		if aCat != bCat {
			return false, fmt.Sprintf("%s %s, %s %s",
				aName, describeOutcome(a, aErr),
				bName, describeOutcome(b, bErr))
		}
		// Same non-nil category on both sides: treat as matching failures.
		return true, ""
	}

	// Both succeeded: compare the value lists.
	if len(a) != len(b) {
		return false, fmt.Sprintf("result count: %s=%d, %s=%d",
			aName, len(a), bName, len(b))
	}
	for i := range a {
		if !valuesMatch(a[i], b[i]) {
			return false, fmt.Sprintf("result[%d]: %s=%s (%s), %s=%s (%s)",
				i,
				aName, a[i].String(), a[i].TypeName(),
				bName, b[i].String(), b[i].TypeName())
		}
	}
	return true, ""
}

// errorCategory normalizes an error into a coarse category for comparison:
// nil, "deopt", or "runtime error". Deopt is detected via the deopt sentinel /
// message; everything else non-nil is a generic runtime error.
func errorCategory(err error) string {
	if err == nil {
		return ""
	}
	if strings.Contains(strings.ToLower(err.Error()), "deopt") {
		return "deopt"
	}
	return "runtime error"
}

// describeOutcome renders an outcome (error or values) for a mismatch message.
func describeOutcome(vals []runtime.Value, err error) string {
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return fmt.Sprintf("returned %s", formatValues(vals))
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
	if len(r.ModuleFactDiffs) > 0 {
		w("\n--- Module fact diffs ---\n%s", FormatTier2ModuleFactDiffs(r.ModuleFactDiffs))
	}
	w("\n--- Optimization remarks ---\n%s", formatOptimizationRemarks(r.OptimizationRemarks))
	w("\n--- Q query hot paths ---\n%s", formatQQueryHotPaths(r.QQueryHotPaths))
	w("\n--- Q vector conditional hot paths ---\n%s", formatQVectorWhereHotPaths(r.QVectorWhereHotPaths))
	w("\n--- Q typed runtime kernels ---\n%s", formatQFrameSelectColumnSpecs(r.QTypedRuntimeKernels))
	w("\n--- Q typed vector runtime kernels ---\n%s", formatQTypedVectorRuntimeKernelReport(r.QVectorRuntimeKernels))
	w("\n--- Q query fallback reasons ---\n%s", formatQQueryLoweringFallbackReasons(r.QQueryFallbacks))
	w("\n--- Q vector fallback reasons ---\n%s", formatQVectorLoweringFallbackReasons(r.QVectorLoweringFallbacks))
	w("\n--- Q kernel descriptors ---\n%s", formatQKernelDescriptors(r.QKernelDescriptors))
	w("\n--- Q kernel execution stats ---\n%s", formatQKernelExecutionStats(r.QKernelExecutionStats))
	w("\n--- Q kernel shape summary ---\n%s", formatQKernelShapeSummary(r.QKernelShapeSummary))
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
	w("\n--- IR Interpreter (optimized IR) ---\n")
	if r.OptInterpError != nil {
		w("Error: %v\n", r.OptInterpError)
	} else {
		w("Result: %s\n", formatValues(r.OptInterpResult))
	}
	w("\n--- Native Execution ---\n")
	if r.NativeError != nil {
		w("Error: %v\n", r.NativeError)
	} else {
		w("Result: %s\n", formatValues(r.NativeResult))
	}
	w("\n--- Verdict ---\n")
	verdict := func(label string, ok bool, mismatch string) {
		if ok {
			w("%s: MATCH\n", label)
		} else {
			w("%s: MISMATCH: %s\n", label, mismatch)
		}
	}
	verdict("Optimizer (unopt-interp vs opt-interp)", r.OptimizerMatch, r.OptimizerMismatch)
	verdict("Backend   (opt-interp vs native)      ", r.BackendMatch, r.BackendMismatch)
	verdict("End-to-end (unopt-interp vs native)   ", r.Match, r.Mismatch)
	return sb.String()
}
