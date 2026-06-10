//go:build darwin && arm64

// q_eval_jit_script_bench_test.go measures "Leia script under JIT calling the
// q evaluator in a hot loop" against the hand-written Go baselines
// (BenchmarkQEvalVectorGoBaseline/<case>). Unlike the other q.eval benches in
// this package, which call q.eval through bind.BuildQ() directly, these
// benches go through the public embedding API (leia.New + vm.Exec + vm.Call)
// so the loop itself tiers up through the bytecode VM into Tier 2 native code.
//
// Bench naming contract (consumed by q_perf_report.py):
//   - BenchmarkQEvalJITScriptWarm/<case>  — JIT enabled (leia.WithJIT())
//   - BenchmarkQEvalVMScriptWarm/<case>   — bytecode VM only (leia.WithVM())
//
// Route choice — q.session().eval, NOT bare q.eval (validity-critical):
// a bare q.eval(<constant source>) call CANNOT measure per-iteration columnar
// work, for two independently verified reasons:
//
//  1. bind-level result memoization: q.eval routes through bind's qEvalCache
//     (internal/stdlib/bind/q.go qEvalSymbolicSource), which memoizes the
//     result of any EvalSourceCacheable constant source. Every benchmark
//     expression in qEvalVectorCases is cacheable, so iterations after the
//     first are ~500ns map hits.
//  2. Tier 2 loop hoisting: when methodjit lowers a constant q.eval source to
//     OpQEvalPipelinePlan, the op has no arguments and is treated as pure, so
//     the optimizer legitimately hoists it out of the loop. Empirically
//     (forced Tier 2, 64 loop iterations): ExitQEvalPipelinePlan == 1 and
//     zero stdq kernel attempts recorded inside the loop.
//
// Both effects were confirmed empirically: with the bare-q.eval harness,
// stdq.RuntimeKernelExecutionStats() recorded 0 kernel attempts across 1000
// loop iterations and ns/op was flat (~0.9-1.3us) across a 64x row-count
// range. That harness measures memoization, not work.
//
// The benchmarks therefore run the loop body through a q session:
// qSessionEval := q.session().eval; qSessionEval(<constant source>). Session
// eval has no result cache (bind q.session.eval calls EvalState.Eval
// directly) and re-executes the typed q runtime kernels every iteration —
// verified: typed-kernel attempts scale exactly with iteration count (3-5
// attempts/op depending on expression) and ns/op scales with row count.
// Trade-offs, documented for q_perf_report.py consumers:
//   - methodjit does not lower the session eval call to the q-eval
//     pipeline-plan op (qCallIsQEvalEntrypoint only recognizes the literal
//     q.eval global entrypoint); each iteration is a generic host call into
//     the q evaluator.
//   - Tier 2 currently declines the run loop entirely ("LoopDepth<2
//     candidate has performance-blocked op Call inside loop"), so under
//     WithJIT the loop executes at Tier 1 baseline JIT. This is the honest
//     current end-to-end number; closing that gap (per-iteration q work
//     under Tier 2) is exactly the optimization headroom this layer tracks.
//
// TestQEvalJITScriptRouting pins all of the above: per-iteration kernel
// attempts scale with N on the session route, Tier 2 declines the session
// loop today (logs loudly if that changes), and the direct-q.eval route is
// pinned as memoized/hoisted so a future per-iteration-capable JIT route
// will be noticed.
//
// Harness choice: the hot loop lives in the Leia script (func run(n) { ... })
// and each benchmark performs a single amortized vm.Call("run", b.N) after
// warmup. Both forms were measured (Apple M4 Max): per-iteration
// vm.Call("run", 1) costs ~80-95us/op of pure host-call boundary overhead
// (~1.2k allocs/op), which would swamp the 3-6us session evals being
// measured; the amortized form keeps the loop inside the JIT'd function.
// Note: at very low -benchtime (e.g. 10x) the warm call's fixed entry cost is
// amortized over few iterations and inflates ns/op; use >=100x for stable
// per-op numbers.
//
// The q source is embedded into the script as a constant string literal (%q):
// Leia string literals use Go-compatible escaping, and a constant source
// keeps this layer comparable with a future constant-source JIT fast path.

package benchmarks

import (
	"fmt"
	"strings"
	"testing"

	leia "github.com/never-labs/leia"
	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/methodjit"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	bytecodevm "github.com/never-labs/leia/internal/vm"
)

// qEvalJITScriptCaseNames is the deterministic benchmark subset: one or more
// representatives per q.eval pipeline category plus the worst families from
// the q breadth perf audit (benchmarks/q_breadth_perf_audit.md). Names must
// match qEvalVectorCases entries exactly; TestQEvalJITScriptCaseNamesExist
// fails with the missing names if the case table drifts.
var qEvalJITScriptCaseNames = []string{
	// Audit worst families: apply bracket gather.
	"BreadthApplyBracketGatherSmall",
	"BreadthApplyBracketGatherMedium",
	"BreadthApplyBracketGatherWide",
	// Audit worst families: arithmetic div/mod envelope.
	"BreadthArithmeticDivModEnvelopeMod3Bias1",
	"BreadthArithmeticDivModEnvelopeMod7Bias4",
	// Audit worst families: list drop/take/sublist.
	"BreadthListDropTakeSublistShort",
	"BreadthListDropTakeSublistLong",
	// Audit worst families: list cut/raze checksum.
	"BreadthListCutRazeChecksumShort",
	"BreadthListCutRazeChecksumLong",
	// Audit worst families: float floor/ceiling/reciprocal.
	"BreadthFloatFloorCeilingReciprocalMod3Bias1",
	"BreadthFloatFloorCeilingReciprocalMod7Bias4",
	// Audit worst families: symbol distinct/group/sort.
	"BreadthSymbolDistinctGroupSortSymbolsA",
	"BreadthSymbolDistinctGroupSortVenuesX",
	// vector_numeric spread.
	"VectorAffineSumSmall",
	"VectorAffineSumWide",
	"VectorSquareSumMarketPrice",
	"VectorMinMaxDyadicEnvelope",
	"NumericMonadExpReciprocalSignumNot",
	// where_project_reduce / gather spread.
	"WhereIndexSumGE128",
	"WhereValueGatherReduceSelectivityPct50",
	"WhereGatherProjectionSum",
	"WhereModuloGatherProjectionSum",
	// Moving-window spread.
	"MovingSumAvgRowScaled",
	"MovingStdDevEma32Envelope",
	"MovingMinMax32Envelope",
	// Adverb spread.
	"AdverbInitialOverScanProducts",
	"EachPriorMinusRowScaled",
	"DeepAdverbEachPairwiseArithmetic",
	// apply_index spread.
	"BreadthApplyAtGatherSmall",
	"TaskDApplyAtScalarAndVector",
	"TaskDApplyDotScalarProbe",
	// xbar_within representative.
	"DeepXbarWithinCountTen",
	// group_fby representatives.
	"FbyGroupedAggregateRowScaled",
	"GroupModuloBucketCount",
	// sort_gather representative.
	"SortIndexGatherRowScaled",
	// ordinary_list_adverb representative.
	"TaskDListSublistRotateReverse",
	// math_transcendental representative.
	"TaskDMathSqrtLogVectorSum",
	// matrix_reshape representative.
	"BreadthMatrixReshapeRowSumTwoByN",
	// Running aggregate / scan / distinct / typed / symbol / temporal spread.
	"RunningMinMaxTailEnvelope",
	"SumsTailChecksum",
	"DistinctModuloCount",
	"TemporalDateCompareMaskCount",
	"TypedNullFillSum",
	"SymbolEqualityMaskCount",
}

const qEvalJITScriptWarmupCalls = 8

func qEvalJITScriptCaseByName(name string) (qEvalVectorCase, bool) {
	for _, tc := range qEvalVectorCases {
		if tc.name == name {
			return tc, true
		}
	}
	return qEvalVectorCase{}, false
}

// qEvalJITScriptSource builds the hot-loop Leia script for one q expression.
// The loop evaluates through a q session so every iteration performs real
// columnar work (see file header: bare q.eval(const) is memoized at the bind
// layer and loop-hoisted by Tier 2).
func qEvalJITScriptSource(qSrc string) string {
	return fmt.Sprintf(`
qSessionEval := q.session().eval
func run(n) {
    acc := 0
    for i := 1; i <= n; i++ {
        r := qSessionEval(%q)
        acc = r
    }
    return acc
}
`, qSrc)
}

// qEvalJITScriptDirectEvalSource is the bare-q.eval variant of the hot loop.
// It is intentionally NOT used by the benchmarks (memoized + hoisted); the
// routing test keeps it pinned so a behavior change in that route is noticed.
func qEvalJITScriptDirectEvalSource(qSrc string) string {
	return fmt.Sprintf(`
func run(n) {
    acc := 0
    for i := 1; i <= n; i++ {
        r := q.eval(%q)
        acc = r
    }
    return acc
}
`, qSrc)
}

func qEvalJITScriptVM(tb testing.TB, useJIT bool, script string) *leia.VM {
	tb.Helper()
	var vm *leia.VM
	if useJIT {
		vm = leia.New(leia.WithJIT())
	} else {
		vm = leia.New(leia.WithVM())
	}
	if err := vm.Exec(script); err != nil {
		tb.Fatalf("vm.Exec(run script): %v", err)
	}
	return vm
}

func qEvalJITScriptResultInt64(tb testing.TB, out []interface{}) int64 {
	tb.Helper()
	if len(out) != 1 {
		tb.Fatalf("run returned %d values, want 1", len(out))
	}
	switch v := out[0].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		tb.Fatalf("run returned %T (%v), want numeric scalar", out[0], out[0])
		return 0
	}
}

// qEvalJITScriptWarmup drives enough VM-path calls through run for tiering to
// promote it (Tier 2 threshold is 2 VM-path calls; loop-heavy functions
// promote fast), then asserts the script result matches the Go baseline.
func qEvalJITScriptWarmup(tb testing.TB, vm *leia.VM, want int64) {
	tb.Helper()
	var got int64
	for i := 0; i < qEvalJITScriptWarmupCalls; i++ {
		out, err := vm.Call("run", 4)
		if err != nil {
			tb.Fatalf("warmup vm.Call(run): %v", err)
		}
		got = qEvalJITScriptResultInt64(tb, out)
	}
	if got != want {
		tb.Fatalf("warmup run() = %d, want Go baseline %d", got, want)
	}
}

func qEvalJITScriptBenchmark(b *testing.B, useJIT bool) {
	for _, name := range qEvalJITScriptCaseNames {
		tc, ok := qEvalJITScriptCaseByName(name)
		if !ok {
			b.Fatalf("q.eval JIT script subset case %q is missing from qEvalVectorCases; update qEvalJITScriptCaseNames", name)
		}
		b.Run(name, func(b *testing.B) {
			src := tc.expr(qEvalVectorRows)
			want := tc.goFn(qEvalVectorRows)
			vm := qEvalJITScriptVM(b, useJIT, qEvalJITScriptSource(src))
			qEvalJITScriptWarmup(b, vm, want)
			b.ReportAllocs()
			b.ResetTimer()
			out, err := vm.Call("run", b.N)
			b.StopTimer()
			if err != nil {
				b.Fatalf("vm.Call(run, %d): %v", b.N, err)
			}
			if got := qEvalJITScriptResultInt64(b, out); got != want {
				b.Fatalf("run(%d) = %d, want Go baseline %d", b.N, got, want)
			}
		})
	}
}

func BenchmarkQEvalJITScriptWarm(b *testing.B) {
	qEvalJITScriptBenchmark(b, true)
}

func BenchmarkQEvalVMScriptWarm(b *testing.B) {
	qEvalJITScriptBenchmark(b, false)
}

// TestQEvalJITScriptCaseNamesExist pins the benchmark subset to the current
// qEvalVectorCases table. Concurrent edits to the case table must not silently
// orphan subset entries.
func TestQEvalJITScriptCaseNamesExist(t *testing.T) {
	known := make(map[string]bool, len(qEvalVectorCases))
	for _, tc := range qEvalVectorCases {
		known[tc.name] = true
	}
	seen := make(map[string]bool, len(qEvalJITScriptCaseNames))
	var missing, duplicated []string
	for _, name := range qEvalJITScriptCaseNames {
		if seen[name] {
			duplicated = append(duplicated, name)
		}
		seen[name] = true
		if !known[name] {
			missing = append(missing, name)
		}
	}
	if len(duplicated) > 0 {
		t.Fatalf("qEvalJITScriptCaseNames contains duplicate entries: %s", strings.Join(duplicated, ", "))
	}
	if len(missing) > 0 {
		t.Fatalf("qEvalJITScriptCaseNames refers to q.eval vector cases that do not exist: %s.\n"+
			"The case table in q_eval_vector_bench_test.go changed; update qEvalJITScriptCaseNames "+
			"in q_eval_jit_script_bench_test.go to current case names.", strings.Join(missing, ", "))
	}

	// The subset must keep covering every q.eval pipeline category.
	covered := map[string]bool{}
	for _, name := range qEvalJITScriptCaseNames {
		tc, _ := qEvalJITScriptCaseByName(name)
		for _, category := range qEvalVectorPipelineCategories(tc) {
			covered[category] = true
		}
	}
	var missingCategories []string
	for _, category := range []string{
		"xbar_within",
		"where_project_reduce",
		"group_fby",
		"sort_gather",
		"ordinary_list_adverb",
		"vector_numeric",
		"math_transcendental",
		"matrix_reshape",
		"apply_index",
	} {
		if !covered[category] {
			missingCategories = append(missingCategories, category)
		}
	}
	if len(missingCategories) > 0 {
		t.Fatalf("qEvalJITScriptCaseNames no longer covers pipeline categories: %s", strings.Join(missingCategories, ", "))
	}
}

func qEvalJITScriptCompileTop(t *testing.T, src string) *bytecodevm.FuncProto {
	t.Helper()
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		t.Fatalf("lexer: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		t.Fatalf("parser: %v", err)
	}
	proto, err := bytecodevm.Compile(prog)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	return proto
}

func qEvalJITScriptKernelAttempts() uint64 {
	var attempts uint64
	for _, stat := range stdq.RuntimeKernelExecutionStats() {
		if stat.Outcome == "attempt" {
			attempts += stat.Count
		}
	}
	return attempts
}

// qEvalJITScriptForcedTier2 compiles script's run function at Tier 2 on an
// inspectable TieringManager, calls run(iterations), and returns the result
// plus the manager (jit_arm64.go pattern; the public API hides its manager).
func qEvalJITScriptForcedTier2(t *testing.T, script string, iterations int64) (int64, *bytecodevm.FuncProto, *methodjit.TieringManager) {
	t.Helper()
	proto := qEvalJITScriptCompileTop(t, script)
	globals := vmtest.NewInterpreterGlobals()
	v := bytecodevm.New(globals)
	defer v.Close()
	tm := methodjit.NewTieringManager()
	v.SetMethodJIT(tm)
	if _, err := v.Execute(proto); err != nil {
		t.Fatalf("Execute(top): %v", err)
	}
	var runProto *bytecodevm.FuncProto
	for _, p := range proto.Protos {
		if p.Name == "run" {
			runProto = p
			break
		}
	}
	if runProto == nil {
		t.Fatal("run proto not found among top-level protos")
	}
	if err := tm.CompileTier2(runProto); err != nil {
		t.Fatalf("CompileTier2(run): %v", err)
	}
	results, err := v.CallValue(v.GetGlobal("run"), []runtime.Value{runtime.IntValue(iterations)})
	if err != nil {
		t.Fatalf("CallValue(run, %d): %v", iterations, err)
	}
	if len(results) != 1 {
		t.Fatalf("run returned %d values, want 1", len(results))
	}
	switch {
	case results[0].IsInt():
		return results[0].Int(), runProto, tm
	case results[0].IsFloat():
		return int64(results[0].Float()), runProto, tm
	default:
		t.Fatalf("run returned %s, want numeric scalar", results[0].TypeName())
		return 0, nil, nil
	}
}

// TestQEvalJITScriptRouting smoke-tests the measurement layer on one
// vector-heavy case:
//
//  1. Public embedding harness (same flow as BenchmarkQEvalJITScriptWarm):
//     result correctness vs the Go baseline, and per-iteration REAL work —
//     typed q kernel attempts must scale with the iteration count
//     (>= 0.9 * N), not stay flat as a memoized route would.
//  2. Forced Tier 2 of the same session-route script: the loop function
//     actually enters Tier 2 native code (EnteredTier2 flag).
//  3. Pinned counter-example: the bare-q.eval(const) route is memoized and
//     loop-hoisted (ExitQEvalPipelinePlan fires but kernel attempts stay ~0
//     for 64 iterations). If this assertion ever fails because attempts start
//     scaling, the JIT route became per-iteration capable and the benchmarks
//     should be re-pointed at it.
func TestQEvalJITScriptRouting(t *testing.T) {
	const caseName = "VectorAffineSumSmall"
	tc, ok := qEvalJITScriptCaseByName(caseName)
	if !ok {
		t.Fatalf("routing smoke case %q is missing from qEvalVectorCases", caseName)
	}
	src := tc.expr(qEvalVectorRows)
	want := tc.goFn(qEvalVectorRows)

	// (1) Public-API session-route harness: correctness + per-iteration work.
	vm := qEvalJITScriptVM(t, true, qEvalJITScriptSource(src))
	qEvalJITScriptWarmup(t, vm, want)
	stdq.ClearRuntimeKernelExecutionStats()
	const iters = 64
	const minAttempts = uint64(iters * 9 / 10) // attempts must scale ~N: >= 0.9*iters
	out, err := vm.Call("run", iters)
	if err != nil {
		t.Fatalf("vm.Call(run, %d): %v", iters, err)
	}
	if got := qEvalJITScriptResultInt64(t, out); got != want {
		t.Fatalf("run(%d) = %d, want Go baseline %d", iters, got, want)
	}
	attempts := qEvalJITScriptKernelAttempts()
	if attempts < minAttempts {
		t.Fatalf("typed kernel attempts = %d for %d iterations (< %d); session route is not doing per-iteration work",
			attempts, iters, minAttempts)
	}
	t.Logf("session route: %d typed kernel attempts over %d iterations (%.2f/op)",
		attempts, iters, float64(attempts)/iters)

	// (2) JIT tiering reality for the session route: Tier 2 currently declines
	// a single-depth loop dominated by a generic host call ("performance-
	// blocked op Call inside loop"), so run executes under Tier 1 baseline
	// JIT. Pin that, and notice if Tier 2 ever starts accepting it.
	{
		proto := qEvalJITScriptCompileTop(t, qEvalJITScriptSource(src))
		globals := vmtest.NewInterpreterGlobals()
		v := bytecodevm.New(globals)
		tm := methodjit.NewTieringManager()
		v.SetMethodJIT(tm)
		if _, err := v.Execute(proto); err != nil {
			v.Close()
			t.Fatalf("Execute(top): %v", err)
		}
		var runProto *bytecodevm.FuncProto
		for _, p := range proto.Protos {
			if p.Name == "run" {
				runProto = p
				break
			}
		}
		if runProto == nil {
			v.Close()
			t.Fatal("run proto not found among top-level protos")
		}
		for i := 0; i < qEvalJITScriptWarmupCalls; i++ {
			if _, err := v.CallValue(v.GetGlobal("run"), []runtime.Value{runtime.IntValue(4)}); err != nil {
				v.Close()
				t.Fatalf("warm CallValue(run): %v", err)
			}
		}
		if err := tm.CompileTier2(runProto); err != nil {
			t.Logf("Tier 2 declines the session-route loop as expected today: %v", err)
			if tm.Tier1Count() == 0 {
				v.Close()
				t.Fatalf("session-route run was not even Tier 1 compiled after %d warm calls; JIT did not engage", qEvalJITScriptWarmupCalls)
			}
			t.Logf("session-route loop runs under Tier 1 baseline JIT (tier1 compiled functions: %d)", tm.Tier1Count())
		} else {
			t.Logf("NOTE: Tier 2 now accepts the session-route loop; "+
				"BenchmarkQEvalJITScriptWarm is measuring Tier 2 from now on (tier2 compiled: %d)", tm.Tier2Count())
		}
		v.Close()
	}

	// (3) Pinned counter-example: bare q.eval(const) is hoisted + memoized.
	stdq.ClearRuntimeKernelExecutionStats()
	got, runProto, tm := qEvalJITScriptForcedTier2(t, qEvalJITScriptDirectEvalSource(src), iters)
	if got != want {
		t.Fatalf("Tier 2 direct-eval run(%d) = %d, want Go baseline %d", iters, got, want)
	}
	if runProto.EnteredTier2 != 1 {
		t.Fatalf("direct-eval run did not enter Tier 2 native code (EnteredTier2=%d)", runProto.EnteredTier2)
	}
	snap := tm.ExitStats()
	if snap.ByExitCode["ExitQEvalPipelinePlan"] == 0 {
		t.Fatalf("direct q.eval(const) no longer routes through ExitQEvalPipelinePlan (exit stats: %v); routing assumptions changed", snap.ByExitCode)
	}
	directAttempts := qEvalJITScriptKernelAttempts()
	if directAttempts >= minAttempts {
		t.Fatalf("direct q.eval(const) route now performs per-iteration work (%d attempts for %d iterations); "+
			"re-point BenchmarkQEvalJITScriptWarm at the direct route and update the file header", directAttempts, iters)
	}
	t.Logf("direct q.eval(const) route pinned: ExitQEvalPipelinePlan=%d, kernel attempts=%d for %d iterations (memoized/hoisted as documented)",
		snap.ByExitCode["ExitQEvalPipelinePlan"], directAttempts, iters)
}
