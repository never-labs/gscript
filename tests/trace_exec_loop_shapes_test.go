//go:build darwin && arm64

package tests_test

import (
	"testing"

	gs "github.com/never-labs/gscript"
)

func TestTraceExec_EmptyLoopBody(t *testing.T) {
	src := `
func emptyLoop(n) {
    s := 0
    for i := 1; i <= n; i++ {
        // intentionally empty body — just iterate
    }
    return i
}
result := emptyLoop(1000)
`
	// If the language leaks i from the for-loop, verify VM/JIT agree.
	// Otherwise both will return nil/0.
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())
	// nil and int64(0) are both acceptable "undefined variable" results.
	isFalsy := func(v interface{}) bool {
		switch v := v.(type) {
		case nil:
			return true
		case int64:
			return v == 0
		case float64:
			return v == 0
		}
		return false
	}
	if !(isFalsy(vmResult) && isFalsy(jitResult)) && vmResult != jitResult {
		t.Errorf("VM and JIT empty loop results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestTraceExec_LargeIterationCount tests correctness with a large number
// of iterations to stress-test the trace execution.
func TestTraceExec_LargeIterationCount(t *testing.T) {
	src := `
func bigSum(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + i
    }
    return s
}
result := bigSum(100000)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	expected := int64(5000050000)
	if vmResult != expected {
		t.Errorf("VM bigSum: got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT bigSum: got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_NegativeStep tests a for loop with a negative step value.
func TestTraceExec_NegativeStep(t *testing.T) {
	src := `
func countDown(n) {
    s := 0
    for i := n; i >= 1; i-- {
        s = s + i
    }
    return s
}
result := countDown(100)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	expected := int64(5050)
	if vmResult != expected {
		t.Errorf("VM countDown(100): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT countDown(100): got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_NestedLoopCorrectness tests that inner and outer loop
// accumulators are independently correct after trace compilation.
func TestTraceExec_NestedLoopCorrectness(t *testing.T) {
	src := `
func nested(m, n) {
    total := 0
    for i := 1; i <= m; i++ {
        inner := 0
        for j := 1; j <= n; j++ {
            inner = inner + j
        }
        total = total + inner
    }
    return total
}
result := nested(20, 50)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// inner = sum(1..50) = 1275, repeated 20 times = 25500
	expected := int64(25500)
	if vmResult != expected {
		t.Errorf("VM nested(20,50): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT nested(20,50): got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_LoopWithTableWrite tests that table writes inside a
// traced loop produce correct results (exercises call-exit for SETTABLE).
func TestTraceExec_LoopWithTableWrite(t *testing.T) {
	// Note: the existing TestJIT_SideExit_TableOps passes with a similar pattern.
	// The difference here is that both fill and sum loops are inside a function,
	// which may trigger different trace compilation behavior.
	src := `
func fillAndSum(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * i
    }
    s := 0
    for i := 1; i <= n; i++ {
        s = s + t[i]
    }
    return s
}
result := fillAndSum(100)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// sum of i^2 for i=1..100 = 100*101*201/6 = 338350
	expected := int64(338350)
	if vmResult != expected {
		t.Fatalf("VM fillAndSum(100): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT fillAndSum(100): got %v, want %d (VM=%v)", jitResult, expected, vmResult)
	}
}

// TestTraceExec_LoopWithStringConcat tests that string concatenation
// (a call-exit operation) inside a traced loop works correctly.
func TestTraceExec_LoopWithStringConcat(t *testing.T) {
	src := `
s := ""
for i := 1; i <= 50; i++ {
    s = s .. "a"
}
result := #s
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	expected := int64(50)
	if vmResult != expected {
		t.Errorf("VM string concat: got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT string concat: got %v, want %d", jitResult, expected)
	}
}

// TestTraceExec_ModuloInLoop tests the modulo operation in a traced loop
// (exercises SSA_MOD_INT).
func TestTraceExec_ModuloInLoop(t *testing.T) {
	// Known JIT issue: conditional branches with modulo guard produce
	// incorrect results (same class as TestJIT_ConditionalBranching).
	src := `
func countEven(n) {
    count := 0
    for i := 1; i <= n; i++ {
        if i % 2 == 0 {
            count = count + 1
        }
    }
    return count
}
result := countEven(1000)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(500)
	if vmResult != expected {
		t.Fatalf("VM countEven(1000): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT countEven(1000): got %v, want %d (VM=%v)", jitResult, expected, vmResult)
	}
}

// TestTraceExec_MultiplyInLoop tests multiplication in a traced loop
// (exercises SSA_MUL_INT).
func TestTraceExec_MultiplyInLoop(t *testing.T) {
	src := `
func factorial(n) {
    result := 1
    for i := 2; i <= n; i++ {
        result = result * i
    }
    return result
}
result := factorial(12)
`
	vmResult, jitResult := runAndCompare(t, src, "result")

	// 12! = 479001600
	expected := int64(479001600)
	if vmResult != expected {
		t.Errorf("VM factorial(12): got %v, want %d", vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT factorial(12): got %v, want %d", jitResult, expected)
	}
}
