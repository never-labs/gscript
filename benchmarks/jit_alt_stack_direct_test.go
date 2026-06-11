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

	"github.com/never-labs/leia/internal/methodjit"
)

func TestJITAltStackDirectHelperEngagement(t *testing.T) {
	altStack := os.Getenv("LEIA_JIT_ALT_STACK") == "1"

	const caseName = "VectorAffineSumSmall"
	tc, ok := qEvalJITScriptCaseByName(caseName)
	if !ok {
		t.Fatalf("case %q missing from qEvalVectorCases", caseName)
	}
	src := tc.expr(qEvalVectorRows)
	want := tc.goFn(qEvalVectorRows)

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
	tc, ok := qEvalJITScriptCaseByName("VectorAffineSumSmall")
	if !ok {
		b.Fatal("case VectorAffineSumSmall missing")
	}
	src := tc.expr(qEvalVectorRows)
	want := tc.goFn(qEvalVectorRows)
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
