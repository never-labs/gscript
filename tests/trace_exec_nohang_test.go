//go:build darwin && arm64

package tests_test

import "testing"

func TestTraceExec_NoHang_SimpleLoop(t *testing.T) {
	src := `
s := 0
for i := 1; i <= 10000; i++ {
    s = s + i
}
result := s
`
	err := runWithTimeout(t, src, 5)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
}

// TestTraceExec_NoHang_NestedLoops tests that nested loops don't hang.
func TestTraceExec_NoHang_NestedLoops(t *testing.T) {
	src := `
s := 0
for i := 1; i <= 100; i++ {
    for j := 1; j <= 100; j++ {
        s = s + 1
    }
}
result := s
`
	err := runWithTimeout(t, src, 5)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
}

// TestTraceExec_NoHang_CallInLoop tests that a loop with function calls doesn't hang.
func TestTraceExec_NoHang_CallInLoop(t *testing.T) {
	src := `
func inc(x) { return x + 1 }
s := 0
for i := 1; i <= 10000; i++ {
    s = inc(s)
}
result := s
`
	err := runWithTimeout(t, src, 5)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
}

// TestTraceExec_NoHang_SideExitHeavy tests a loop that triggers many side-exits
// per iteration (every iteration hits the conditional).
func TestTraceExec_NoHang_SideExitHeavy(t *testing.T) {
	src := `
count := 0
for i := 1; i <= 1000; i++ {
    if i % 2 == 0 {
        count = count + 1
    } else {
        count = count + 2
    }
}
result := count
`
	err := runWithTimeout(t, src, 5)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
}

// TestTraceExec_NoHang_RecursiveInLoop tests a loop that calls a recursive
// function (the most complex call-exit scenario).
func TestTraceExec_NoHang_RecursiveInLoop(t *testing.T) {
	src := `
func fib(n) {
    if n < 2 { return n }
    return fib(n-1) + fib(n-2)
}
s := 0
for i := 1; i <= 20; i++ {
    s = s + fib(10)
}
result := s
`
	err := runWithTimeout(t, src, 10)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
}

// TestTraceExec_NoHang_FloatLoop tests that a float-only loop doesn't hang.
func TestTraceExec_NoHang_FloatLoop(t *testing.T) {
	src := `
s := 0.0
for i := 1; i <= 10000; i++ {
    s = s + 0.001
}
result := s
`
	err := runWithTimeout(t, src, 5)
	if err != nil {
		t.Fatalf("execution error: %v", err)
	}
}

// ============================================================================
// Additional edge case tests
// ============================================================================

// TestTraceExec_EmptyLoopBody tests that an empty loop body still produces
// the correct loop counter result.
