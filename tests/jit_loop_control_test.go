//go:build darwin && arm64

package tests_test

import (
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

// TestJIT_ConditionalBranching tests various conditional patterns.
func TestJIT_ConditionalBranching(t *testing.T) {
	src := `
func classify(n) {
    count := 0
    for i := 1; i <= n; i++ {
        if i % 15 == 0 {
            count = count + 3
        } elseif i % 5 == 0 {
            count = count + 2
        } elseif i % 3 == 0 {
            count = count + 1
        }
    }
    return count
}
result := classify(100)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
	// Verify: multiples of 15 in 1..100: 6 (add 3 each = 18)
	// multiples of 5 but not 15: 14 (add 2 each = 28)
	// multiples of 3 but not 15: 27 (add 1 each = 27)
	// total = 18 + 28 + 27 = 73
	expected := int64(73)
	if jitResult != expected {
		t.Errorf("JIT classify(100): got %v, want %d", jitResult, expected)
	}
}

// TestJIT_WhileLoop tests while-style loops (for without init/post).
func TestJIT_WhileLoop(t *testing.T) {
	src := `
func collatzSteps(n) {
    steps := 0
    for n != 1 {
        if n % 2 == 0 {
            n = n / 2
        } else {
            n = n * 3 + 1
        }
        steps = steps + 1
    }
    return steps
}
result := collatzSteps(27)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// Collatz sequence for 27 takes 111 steps
	expected := int64(111)
	if vmResult != expected {
		t.Errorf("VM collatzSteps(27): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT collatzSteps(27): got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_FibIterative tests iterative fibonacci to verify loop-heavy JIT paths.
func TestJIT_FibIterative(t *testing.T) {
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
result := fib(30)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(832040)
	if vmResult != expected {
		t.Errorf("VM fib(30): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT fib(30): got %v (%T), want %d", jitResult, jitResult, expected)
	}
}
