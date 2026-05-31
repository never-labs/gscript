//go:build darwin && arm64

package tests_test

import (
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestTraceExec_LoopDone_StoreBack(t *testing.T) {
	src := `
func sumTo(n) {
    sum := 0
    for i := 1; i <= n; i++ {
        sum = sum + i
    }
    return sum
}
result := sumTo(10)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	expected := int64(55)
	if vmResult != expected {
		t.Errorf("VM sumTo(10): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT sumTo(10): got %v, want %d", jitResult, jitResult)
	}
}

// TestTraceExec_LoopDone_StoreBack_MultiVar tests store-back with multiple
// loop-carried variables to verify all modified slots are written back.
func TestTraceExec_LoopDone_StoreBack_MultiVar(t *testing.T) {
	// Two accumulators updated each iteration. Both must be correctly
	// stored back when the loop exits.
	src := `
func twoSums(n) {
    even := 0
    odd := 0
    for i := 1; i <= n; i++ {
        if i % 2 == 0 {
            even = even + i
        } else {
            odd = odd + i
        }
    }
    return even + odd
}
result := twoSums(100)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// even + odd = sum(1..100) = 5050
	expected := int64(5050)
	if vmResult != expected {
		t.Errorf("VM twoSums(100): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT twoSums(100): got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_LoopDone_FibStoreBack tests store-back for iterative fibonacci,
// which has three loop-carried values (a, b, temp).
func TestTraceExec_LoopDone_FibStoreBack(t *testing.T) {
	src := `
func fib(n) {
    a := 0
    b := 1
    for i := 0; i < n; i++ {
        t := a + b
        a = b
        b = t
    }
    return a
}
result := fib(20)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	expected := int64(6765)
	if vmResult != expected {
		t.Errorf("VM fib(20): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT fib(20): got %v, want %d", jitResult, expected)
	}
}

// ============================================================================
// 4. Store-back doesn't corrupt multi-type slots
// ============================================================================

// TestTraceExec_StoreBack_MultiTypeSlot tests that when the same slot holds
// different types across iterations (due to side-exits), the final value
// is correct and not type-confused.
func TestTraceExec_StoreBack_MultiTypeSlot(t *testing.T) {
	// Known JIT issue: conditional branches with modulo inside traced loops
	// produce incorrect results (same as TestJIT_ConditionalBranching).
	// The trace records one branch path and the side-exit for the other
	// branch doesn't correctly restore/resume all state.
	// Verify VM correctness, then compare VM vs JIT.
	src := `
func mixedOps(n) {
    val := 0
    for i := 1; i <= n; i++ {
        if i % 10 == 0 {
            val = val + 100
        } else {
            val = val + 1
        }
    }
    return val
}
result := mixedOps(100)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// 90 non-multiples of 10 contribute 90
	// 10 multiples of 10 contribute 1000
	// total = 1090
	expected := int64(1090)
	if vmResult != expected {
		t.Fatalf("VM mixedOps(100): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT mixedOps(100): got %v, want %d (VM=%v)", jitResult, expected, vmResult)
	}
}

// TestTraceExec_StoreBack_SlotReuse tests that register/slot reuse across
// iterations doesn't cause value corruption.
func TestTraceExec_StoreBack_SlotReuse(t *testing.T) {
	// tmp is reused each iteration for a different computation.
	// The final values of a and b must be correct.
	src := `
func slotReuse(n) {
    a := 0
    b := 0
    for i := 1; i <= n; i++ {
        tmp := i * 2
        a = a + tmp
        tmp = i * 3
        b = b + tmp
    }
    return a + b
}
result := slotReuse(50)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// a = 2*sum(1..50) = 2*1275 = 2550
	// b = 3*sum(1..50) = 3*1275 = 3825
	// total = 6375
	expected := int64(6375)
	if vmResult != expected {
		t.Errorf("VM slotReuse(50): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT slotReuse(50): got %v, want %d", jitResult, expected)
	}
}

// ============================================================================
// 5. Call-exit as side-exit
// ============================================================================

// TestTraceExec_CallExit_AsSideExit tests that a trace containing a function
// call (SSA_CALL) correctly exits to the interpreter, executes the call,
// and either resumes the trace or falls back to the interpreter.
