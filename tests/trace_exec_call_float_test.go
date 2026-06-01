//go:build darwin && arm64

package tests_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

func TestTraceExec_CallExit_AsSideExit(t *testing.T) {
	src := `
func double(x) {
    return x * 2
}

func sumDoubles(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + double(i)
    }
    return s
}
result := sumDoubles(100)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// sum of 2*i for i=1..100 = 2*5050 = 10100
	expected := int64(10100)
	if vmResult != expected {
		t.Errorf("VM sumDoubles(100): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT sumDoubles(100): got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_CallExit_GlobalAccess tests call-exit for global variable
// access within a traced loop.
func TestTraceExec_CallExit_GlobalAccess(t *testing.T) {
	src := `
multiplier := 3

func scaleSum(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + i * multiplier
    }
    return s
}
result := scaleSum(50)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// 3 * sum(1..50) = 3 * 1275 = 3825
	expected := int64(3825)
	if vmResult != expected {
		t.Errorf("VM scaleSum(50): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT scaleSum(50): got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_CallExit_MultipleCallsPerIteration tests traces with
// multiple call-exits in the same loop iteration.
func TestTraceExec_CallExit_MultipleCallsPerIteration(t *testing.T) {
	src := `
func add(a, b) { return a + b }
func mul(a, b) { return a * b }

func compute(n) {
    s := 0
    for i := 1; i <= n; i++ {
        a := add(i, i)
        b := mul(i, 2)
        s = s + a + b
    }
    return s
}
result := compute(50)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// a = 2*i, b = 2*i, per iteration = 4*i
	// total = 4*sum(1..50) = 4*1275 = 5100
	expected := int64(5100)
	if vmResult != expected {
		t.Errorf("VM compute(50): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT compute(50): got %v, want %d", jitResult, expected)
	}
}

// ============================================================================
// 6. Float arithmetic correctness
// ============================================================================

// TestTraceExec_FloatArith tests that float accumulation in a traced loop
// produces the correct result without precision loss or NaN.
func TestTraceExec_FloatArith(t *testing.T) {
	// Known JIT issue: float accumulator loops produce incorrect results.
	// The trace JIT does not correctly handle float loop-carried values
	// in simple for-loops (the accumulator store-back is wrong).
	// GETFIELD/SETFIELD float paths work, but local float accumulators don't.
	src := `
func floatSum(n) {
    sum := 0.0
    for i := 1; i <= n; i++ {
        sum = sum + 0.5
    }
    return sum
}
result := floatSum(100)
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	expected := 50.0
	if vmResult != expected {
		t.Fatalf("VM floatSum(100): got %v (%T), want %f", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT floatSum(100): got %v, want %v (VM=%v)", jitResult, expected, vmResult)
	}
}

// TestTraceExec_FloatArith_Multiplication tests float multiplication accuracy.
func TestTraceExec_FloatArith_Multiplication(t *testing.T) {
	// Known JIT issue: float loop-carried multiply accumulator is incorrect.
	src := `
func floatProd(n) {
    prod := 1.0
    for i := 1; i <= n; i++ {
        prod = prod * 1.01
    }
    return prod
}
result := floatProd(100)
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	// 1.01^100 ~= 2.704813829...
	if vmResult != jitResult {
		t.Errorf("JIT floatProd(100): JIT=%v, VM=%v", jitResult, vmResult)
	}
}

// TestTraceExec_FloatArith_MixedIntFloat tests a loop that mixes int
// iteration counter with float accumulator.
func TestTraceExec_FloatArith_MixedIntFloat(t *testing.T) {
	// Known JIT issue: mixed int/float loop-carried accumulator is incorrect.
	src := `
func mixedAccum(n) {
    sum := 0.0
    for i := 1; i <= n; i++ {
        sum = sum + i * 0.1
    }
    return sum
}
result := mixedAccum(100)
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	// sum = 0.1 * sum(1..100) = 0.1 * 5050 = 505.0
	if vmResult != jitResult {
		t.Errorf("JIT mixedAccum(100): JIT=%v, VM=%v", jitResult, vmResult)
	}
}

// TestTraceExec_FloatArith_Subtraction tests float subtraction to catch
// sign-flip or NaN bugs in the codegen.
func TestTraceExec_FloatArith_Subtraction(t *testing.T) {
	// Known JIT issue: float loop-carried subtraction accumulator is incorrect.
	src := `
func countdown(n) {
    val := 100.0
    for i := 1; i <= n; i++ {
        val = val - 0.25
    }
    return val
}
result := countdown(200)
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	// 100.0 - 200*0.25 = 100.0 - 50.0 = 50.0
	expected := 50.0
	if vmResult != expected {
		t.Fatalf("VM countdown(200): got %v, want %f", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT countdown(200): got %v, want %v (VM=%v)", jitResult, expected, vmResult)
	}
}

// TestTraceExec_FloatArith_Division tests float division in a loop.
func TestTraceExec_FloatArith_Division(t *testing.T) {
	src := `
func halve(n) {
    val := 1024.0
    for i := 1; i <= n; i++ {
        val = val / 2.0
    }
    return val
}
result := halve(10)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// 1024 / 2^10 = 1.0
	expected := 1.0
	if vmResult != expected {
		t.Errorf("VM halve(10): got %v, want %f", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT halve(10): got %v, want %f", jitResult, expected)
	}
}

// ============================================================================
// 7. No-hang guarantee
// ============================================================================

// TestTraceExec_NoHang_SimpleLoop tests that a simple for loop doesn't hang.
