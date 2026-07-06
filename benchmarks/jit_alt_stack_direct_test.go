//go:build darwin && arm64

// jit_alt_stack_direct_test.go pins the R5-K JIT alternate-stack direct
// helper-call lane (LEIA_JIT_ALT_STACK=1): when enabled, the per-iteration
// OpQEvalSessionEval helper must be dispatched via direct BLR calls from
// Tier 2 native code (methodjit.Tier2DirectHelperCallCount) instead of
// op-exit round trips, with results identical to the generic route.
//
// When the flag is off this test verifies the inverse: the direct lane stays
// silent and the legacy goroutine-stack trampoline handles everything.

package benchmarks

import (
	"os"
	"testing"

	"github.com/never-labs/leia/internal/lexer"
	"github.com/never-labs/leia/internal/methodjit"
	"github.com/never-labs/leia/internal/parser"
	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	bytecodevm "github.com/never-labs/leia/internal/vm"
)

func TestJITAltStackDirectHelperEngagement(t *testing.T) {
	t.Skip("optional q extension coverage is not part of the core default test suite")
	altStack := os.Getenv("LEIA_JIT_ALT_STACK") == "1"

	const src = qEvalJITTypedRuntimeSmokeSource
	const want = qEvalJITTypedRuntimeSmokeWant

	vm := qEvalJITScriptVM(t, true, qEvalJITScriptSource(src))
	qEvalJITScriptWarmup(t, vm, want)

	// Keep the same argument as warmup (run(4)): a novel argument value
	// (e.g. run(256)) trips an upstream argument-shape deopt on the first
	// post-warmup call and the remainder of that call runs outside Tier 2,
	// which would measure tiering dynamics instead of the direct lane.
	before := methodjit.Tier2DirectHelperCallCount()
	const calls = 64
	const itersPerCall = 4
	const iters = calls * itersPerCall
	for i := 0; i < calls; i++ {
		out, err := vm.Call("run", itersPerCall)
		if err != nil {
			t.Fatalf("vm.Call(run, %d): %v", itersPerCall, err)
		}
		if got := qEvalJITScriptResultInt64(t, out); got != want {
			t.Fatalf("run(%d) = %d, want %d", itersPerCall, got, want)
		}
	}
	direct := methodjit.Tier2DirectHelperCallCount() - before

	if altStack {
		// Per-iteration scaling: nearly every session eval should go
		// through the direct lane once the loop runs as Tier 2 native code.
		if direct < iters*9/10 {
			t.Fatalf("direct helper calls = %d over %d iterations; direct lane not engaging under LEIA_JIT_ALT_STACK=1", direct, iters)
		}
		t.Logf("direct lane engaged: %d direct helper calls over %d iterations", direct, iters)
	} else {
		if direct != 0 {
			t.Fatalf("direct helper calls = %d with LEIA_JIT_ALT_STACK off; legacy mode must not use the direct lane", direct)
		}
	}
}

// BenchmarkQEvalJITSessionEvalLane measures the per-iteration q session-eval
// round trip in isolation: run(K) is warmed and measured with the SAME
// argument so the loop stays in Tier 2 for the whole window (a novel argument
// triggers an upstream argument-shape deopt that would drop the rest of the
// call out of Tier 2 and measure the interpreter instead — this also affects
// BenchmarkQEvalJITScriptWarm, which warms with run(4) and then measures
// run(b.N)). Compare default (slim exit lane) vs LEIA_JIT_ALT_STACK=1
// (direct BLR helper lane); ns/eval is the per-iteration cost.
func BenchmarkQEvalJITSessionEvalLane(b *testing.B) {
	const src = qEvalJITTypedRuntimeSmokeSource
	const want = qEvalJITTypedRuntimeSmokeWant
	vm := qEvalJITScriptVM(b, true, qEvalJITScriptSource(src))
	const evalsPerCall = 512
	for i := 0; i < 8; i++ {
		out, err := vm.Call("run", evalsPerCall)
		if err != nil {
			b.Fatalf("warm vm.Call(run): %v", err)
		}
		if got := qEvalJITScriptResultInt64(b, out); got != want {
			b.Fatalf("warm run() = %d, want %d", got, want)
		}
	}
	before := methodjit.Tier2DirectHelperCallCount()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := vm.Call("run", evalsPerCall); err != nil {
			b.Fatalf("vm.Call(run): %v", err)
		}
	}
	b.StopTimer()
	direct := methodjit.Tier2DirectHelperCallCount() - before
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(int64(b.N)*evalsPerCall), "ns/eval")
	b.ReportMetric(float64(direct)/float64(int64(b.N)*evalsPerCall), "direct/eval")
}

// jitAltStackForcedTier2Harness compiles script on an inspectable
// TieringManager and force-compiles every named function at Tier 2
// (TB-generic variant of the qEvalJITScriptForcedTier2 pattern, shared by the
// pipeline-plan engagement test and lane benchmark). Callers own v.Close().
func jitAltStackForcedTier2Harness(tb testing.TB, script string, tier2Funcs ...string) (*bytecodevm.VM, *methodjit.TieringManager) {
	tb.Helper()
	tokens, err := lexer.New(script).Tokenize()
	if err != nil {
		tb.Fatalf("lexer: %v", err)
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		tb.Fatalf("parser: %v", err)
	}
	proto, err := bytecodevm.Compile(prog)
	if err != nil {
		tb.Fatalf("compile: %v", err)
	}
	v := bytecodevm.New(vmtest.NewInterpreterGlobals())
	tm := methodjit.NewTieringManager()
	v.SetMethodJIT(tm)
	if _, err := v.Execute(proto); err != nil {
		v.Close()
		tb.Fatalf("Execute(top): %v", err)
	}
	protosByName := map[string]*bytecodevm.FuncProto{}
	for _, p := range proto.Protos {
		protosByName[p.Name] = p
	}
	// Warm so arg-shape profiles exist before forcing Tier 2.
	for i := 0; i < 4; i++ {
		if _, err := v.CallValue(v.GetGlobal("run"), []runtime.Value{runtime.IntValue(4)}); err != nil {
			v.Close()
			tb.Fatalf("warm CallValue(run, 4): %v", err)
		}
	}
	for _, name := range tier2Funcs {
		p := protosByName[name]
		if p == nil {
			v.Close()
			tb.Fatalf("proto %q not found among top-level protos", name)
		}
		if err := tm.CompileTier2(p); err != nil {
			v.Close()
			tb.Fatalf("CompileTier2(%s): %v", name, err)
		}
	}
	return v, tm
}

func jitAltStackCallRunInt(tb testing.TB, v *bytecodevm.VM, n int64) int64 {
	tb.Helper()
	results, err := v.CallValue(v.GetGlobal("run"), []runtime.Value{runtime.IntValue(n)})
	if err != nil {
		tb.Fatalf("CallValue(run, %d): %v", n, err)
	}
	if len(results) != 1 || !results[0].IsInt() {
		tb.Fatalf("run(%d) = %v, want one int", n, results)
	}
	return results[0].Int()
}

// TestJITAltStackDirectPipelinePlanEngagement pins the R6-S conversion of the
// ExitQEvalPipelinePlan protocol to the direct BLR helper lane: under
// LEIA_JIT_ALT_STACK=1 the q.eval(const) pipeline-plan helper must dispatch
// through the bridge (Tier2DirectHelperCallCount moves) with results AND
// ExitStats diagnostics identical to the generic exit path; with the flag off
// the direct lane must stay silent.
func TestJITAltStackDirectPipelinePlanEngagement(t *testing.T) {
	t.Skip("optional q extension coverage is not part of the core default test suite")
	altStack := os.Getenv("LEIA_JIT_ALT_STACK") == "1"
	const qSrc = "+/til 64"
	const want = int64(2016)

	v, tm := jitAltStackForcedTier2Harness(t, qEvalJITScriptDirectEvalSource(qSrc), "run")
	defer v.Close()
	// Settle: the first full-size call can entry-deopt on the warmup-observed
	// argument range and recompile relaxed.
	for i := 0; i < 2; i++ {
		jitAltStackCallRunInt(t, v, 64)
	}

	beforeDirect := methodjit.Tier2DirectHelperCallCount()
	beforeExits := tm.ExitStats().ByExitCode["ExitQEvalPipelinePlan"]
	const calls = 16
	for i := 0; i < calls; i++ {
		if got := jitAltStackCallRunInt(t, v, 64); got != want {
			t.Fatalf("run(64) = %d, want %d", got, want)
		}
	}
	direct := methodjit.Tier2DirectHelperCallCount() - beforeDirect
	exits := tm.ExitStats().ByExitCode["ExitQEvalPipelinePlan"] - beforeExits

	// Diagnostic parity in BOTH modes: the direct lane records the same
	// ExitQEvalPipelinePlan exit-stat rows the generic protocol records.
	if exits == 0 {
		t.Fatalf("ExitQEvalPipelinePlan exit stats did not move over %d calls (direct=%d); diagnostic parity broken", calls, direct)
	}
	if altStack {
		if direct == 0 {
			t.Fatalf("direct helper calls = 0 over %d calls; pipeline-plan direct lane not engaging under LEIA_JIT_ALT_STACK=1", calls)
		}
		t.Logf("pipeline-plan direct lane engaged: %d direct calls, %d exit stats over %d calls", direct, exits, calls)
	} else if direct != 0 {
		t.Fatalf("direct helper calls = %d with LEIA_JIT_ALT_STACK off; legacy mode must not use the direct lane", direct)
	}
}

// TestJITAltStackDirectHelperCalleeCF pins the self-cf discipline: a direct
// helper site executing inside a natively-BLR'd CALLEE must dispatch with the
// callee's own CompiledFunction (the ExecContext is shared across the whole
// native execution, and ctx.HelperCF is seeded with the ENTRY function's cf;
// each site therefore re-publishes its own cf through the embedded self-cf
// cell). Before that fix, work()'s pipeline plan would be resolved against
// run's plan table — wrong constants/plans — whenever run called work
// natively under LEIA_JIT_ALT_STACK=1.
func TestJITAltStackDirectHelperCalleeCF(t *testing.T) {
	t.Skip("optional q extension coverage is not part of the core default test suite")
	const script = `
func work() {
    x := q.eval("+/til 64")
    return x + 1
}
func run(n) {
    acc := 0
    for i := 1; i <= n; i++ {
        acc = acc + work()
    }
    return acc
}
`
	v, _ := jitAltStackForcedTier2Harness(t, script, "work", "run")
	defer v.Close()
	for i := 0; i < 2; i++ {
		jitAltStackCallRunInt(t, v, 64)
	}
	before := methodjit.Tier2DirectHelperCallCount()
	const n = 64
	want := int64(n) * 2017
	if got := jitAltStackCallRunInt(t, v, n); got != want {
		t.Fatalf("run(%d) = %d, want %d (callee pipeline plan must execute against work's cf)", n, got, want)
	}
	t.Logf("callee-cf run ok (direct helper calls during measured run: %d)",
		methodjit.Tier2DirectHelperCallCount()-before)
}

// BenchmarkQEvalJITPipelinePlanLane measures the per-iteration
// ExitQEvalPipelinePlan transition: with an EXECUTABLE plan ("+/til 64"
// lowers to a typed q runtime kernel) the q.eval(const) site executes once
// per loop iteration in Tier 2 native code, so run(K) performs K plan
// transitions. Compare default (exit protocol: epilogue + Go dispatch +
// exit-stat record + resume re-entry) vs LEIA_JIT_ALT_STACK=1 (direct BLR
// helper lane); ns/eval is the per-iteration cost.
func BenchmarkQEvalJITPipelinePlanLane(b *testing.B) {
	const qSrc = "+/til 64"
	const want = int64(2016)
	v, _ := jitAltStackForcedTier2Harness(b, qEvalJITScriptDirectEvalSource(qSrc), "run")
	defer v.Close()
	const evalsPerCall = 512
	for i := 0; i < 8; i++ {
		if got := jitAltStackCallRunInt(b, v, evalsPerCall); got != want {
			b.Fatalf("warm run(%d) = %d, want %d", evalsPerCall, got, want)
		}
	}
	before := methodjit.Tier2DirectHelperCallCount()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := v.CallValue(v.GetGlobal("run"), []runtime.Value{runtime.IntValue(evalsPerCall)}); err != nil {
			b.Fatalf("CallValue(run, %d): %v", evalsPerCall, err)
		}
	}
	b.StopTimer()
	direct := methodjit.Tier2DirectHelperCallCount() - before
	b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(int64(b.N)*evalsPerCall), "ns/eval")
	b.ReportMetric(float64(direct)/float64(int64(b.N)*evalsPerCall), "direct/eval")
}
