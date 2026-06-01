//go:build darwin && arm64

package tests_test

import (
	"testing"

	leia "github.com/never-labs/leia"
)

// TestJIT_SideExit_TableOps tests that table operations cause JIT side-exits
// and still produce correct results.
func TestJIT_SideExit_TableOps(t *testing.T) {
	src := `
t := {}
for i := 1; i <= 100; i++ {
    t[i] = i * i
}
sum := 0
for i := 1; i <= 100; i++ {
    sum = sum + t[i]
}
result := sum
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	// sum of squares 1..100 = 100*101*201/6 = 338350
	expected := int64(338350)
	if vmResult != expected {
		t.Errorf("VM table ops: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT table ops: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_SideExit_Closures tests that closures cause JIT side-exits
// and still produce correct results.
func TestJIT_SideExit_Closures(t *testing.T) {
	src := `
func makeAdder(x) {
    return func(y) { return x + y }
}

sum := 0
for i := 1; i <= 50; i++ {
    adder := makeAdder(i)
    sum = sum + adder(i)
}
result := sum
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	// sum of 2*i for i=1..50 = 2 * (50*51/2) = 2550
	expected := int64(2550)
	if vmResult != expected {
		t.Errorf("VM closures: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT closures: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_SideExit_StringOps tests string operations that cause JIT side-exits.
func TestJIT_SideExit_StringOps(t *testing.T) {
	src := `
s := ""
for i := 0; i < 50; i++ {
    s = s .. "x"
}
result := #s
`
	vmResult := runAndGet(t, src, "result", leia.WithVM())
	jitResult := runAndGet(t, src, "result", leia.WithJIT())

	expected := int64(50)
	if vmResult != expected {
		t.Errorf("VM string ops: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT string ops: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}
