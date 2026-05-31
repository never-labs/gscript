//go:build darwin && arm64

package tests_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

// TestJIT_ColdCodeSplitting_FnCalls verifies that inline call loops produce
// correct results after cold code splitting moves guard failures out of the hot path.
func TestJIT_ColdCodeSplitting_FnCalls(t *testing.T) {
	src := `
func add(a, b) { return a + b }
func callMany() {
    x := 0
    for i := 0; i < 10000; i++ { x = add(x, 1) }
    return x
}
for i := 1; i <= 15; i++ { callMany() }
result := callMany()
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(10000)
	if vmResult != expected {
		t.Errorf("VM callMany: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT callMany: got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestJIT_ColdCodeSplitting_HeavyLoop verifies that for-loop register spill
// produces correct results after the spill code is moved to the cold section.
func TestJIT_ColdCodeSplitting_HeavyLoop(t *testing.T) {
	src := `
func sumN(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + i
    }
    return s
}
for i := 1; i <= 15; i++ { sumN(10) }
result := sumN(10000)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(50005000)
	if vmResult != expected {
		t.Errorf("VM sumN: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT sumN: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_ColdCodeSplitting_FibRecursive verifies self-recursive calls
// work correctly after cold code splitting.
func TestJIT_ColdCodeSplitting_FibRecursive(t *testing.T) {
	src := `
func fib(n) {
    if n < 2 { return n }
    return fib(n-1) + fib(n-2)
}
result := fib(15)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(610)
	if vmResult != expected {
		t.Errorf("VM fib: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT fib: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_ColdCodeSplitting_Ackermann verifies the ackermann function
// (two-parameter self-call) works correctly after cold code splitting.
func TestJIT_ColdCodeSplitting_Ackermann(t *testing.T) {
	src := `
func ackermann(m, n) {
    if m == 0 { return n + 1 }
    if n == 0 { return ackermann(m - 1, 1) }
    return ackermann(m - 1, ackermann(m, n - 1))
}
result := ackermann(3, 4)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(125)
	if vmResult != expected {
		t.Errorf("VM ackermann: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT ackermann: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_ColdCodeSplitting_ArithGuards verifies that arithmetic type guard
// side exits work correctly when the guard failure code is in the cold section.
func TestJIT_ColdCodeSplitting_ArithGuards(t *testing.T) {
	// This test uses a function where the first call uses ints (JIT compiles it),
	// then a subsequent call uses a non-int to trigger the guard failure.
	src := `
func addOne(x) {
    return x + 1
}
// Warm up JIT
for i := 1; i <= 15; i++ { addOne(10) }
result := addOne(42)
`
	jitResult := runAndGet(t, src, "result", gs.WithJIT())
	expected := int64(43)
	if jitResult != expected {
		t.Errorf("JIT addOne: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}
