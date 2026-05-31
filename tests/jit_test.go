//go:build darwin && arm64

package tests_test

import (
	"fmt"
	"strings"
	"testing"

	gs "github.com/never-labs/gscript/gscript"
)

// captureOutput creates a VM with the given options and a print capture function.
// Returns the VM and a pointer to the captured output slice.
func captureOutput(opts ...gs.Option) (*gs.VM, *[]string) {
	var output []string
	allOpts := append([]gs.Option{gs.WithPrint(func(args ...interface{}) {
		parts := make([]string, len(args))
		for i, a := range args {
			parts[i] = fmt.Sprint(a)
		}
		output = append(output, strings.Join(parts, "\t"))
	})}, opts...)
	vm := gs.New(allOpts...)
	return vm, &output
}

// runAndGet executes source on a VM with the given options and returns a named global.
func runAndGet(t *testing.T, src, varName string, opts ...gs.Option) interface{} {
	t.Helper()
	vm := gs.New(opts...)
	if err := vm.Exec(src); err != nil {
		t.Fatalf("exec error: %v", err)
	}
	val, err := vm.Get(varName)
	if err != nil {
		t.Fatalf("get %q error: %v", varName, err)
	}
	return val
}

// TestJIT_FibRecursive verifies that JIT and VM produce the same correct result
// for recursive Fibonacci (n=15).
func TestJIT_FibRecursive(t *testing.T) {
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
		t.Errorf("VM fib(15): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT fib(15): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestJIT_HeavyLoop verifies sum 1..10000 produces the exact correct result.
func TestJIT_HeavyLoop(t *testing.T) {
	src := `
func sumN(n) {
    s := 0
    for i := 1; i <= n; i++ {
        s = s + i
    }
    return s
}
result := sumN(10000)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(50005000)
	if vmResult != expected {
		t.Errorf("VM sumN(10000): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT sumN(10000): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestJIT_FunctionCallsVaryingArgs tests functions called with different argument counts.
func TestJIT_FunctionCallsVaryingArgs(t *testing.T) {
	src := `
func add0() { return 0 }
func add1(a) { return a }
func add2(a, b) { return a + b }
func add3(a, b, c) { return a + b + c }
func add4(a, b, c, d) { return a + b + c + d }

r0 := add0()
r1 := add1(10)
r2 := add2(10, 20)
r3 := add3(10, 20, 30)
r4 := add4(10, 20, 30, 40)
`
	tests := []struct {
		name     string
		varName  string
		expected int64
	}{
		{"zero args", "r0", 0},
		{"one arg", "r1", 10},
		{"two args", "r2", 30},
		{"three args", "r3", 60},
		{"four args", "r4", 100},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vmResult := runAndGet(t, src, tc.varName, gs.WithVM())
			jitResult := runAndGet(t, src, tc.varName, gs.WithJIT())

			if vmResult != tc.expected {
				t.Errorf("VM %s: got %v (%T), want %d", tc.varName, vmResult, vmResult, tc.expected)
			}
			if jitResult != tc.expected {
				t.Errorf("JIT %s: got %v (%T), want %d", tc.varName, jitResult, jitResult, tc.expected)
			}
		})
	}
}

// TestJIT_NestedForLoops verifies nested loop computation correctness.
func TestJIT_NestedForLoops(t *testing.T) {
	src := `
func nestedSum(n) {
    total := 0
    for i := 1; i <= n; i++ {
        for j := 1; j <= n; j++ {
            total = total + i * j
        }
    }
    return total
}
result := nestedSum(50)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// sum(1..50) = 1275, nestedSum = 1275 * 1275 = 1625625
	expected := int64(1625625)
	if vmResult != expected {
		t.Errorf("VM nestedSum(50): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT nestedSum(50): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestJIT_MixedArithmetic tests int and float mixed arithmetic.
func TestJIT_MixedArithmetic(t *testing.T) {
	src := `
func compute() {
    a := 10
    b := 3.5
    c := a + b
    d := c * 2
    e := d - 1
    f := e / 3
    return f
}
result := compute()
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// a=10, b=3.5, c=13.5, d=27.0, e=26.0, f=26.0/3 = 8.666...
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v (%T), JIT=%v (%T)", vmResult, vmResult, jitResult, jitResult)
	}
	// Check approximate value
	var fResult float64
	switch v := jitResult.(type) {
	case float64:
		fResult = v
	case int64:
		fResult = float64(v)
	default:
		t.Fatalf("unexpected type %T for result", jitResult)
	}
	expected := 26.0 / 3.0
	if diff := fResult - expected; diff > 0.0001 || diff < -0.0001 {
		t.Errorf("JIT compute(): got %v, want ~%v", fResult, expected)
	}
}

// TestJIT_IntArithmeticOps tests various integer arithmetic operations.
func TestJIT_IntArithmeticOps(t *testing.T) {
	src := `
func intOps() {
    a := 100
    b := 7
    sum := a + b
    diff := a - b
    prod := a * b
    quot := a / b
    rem := a % b
    return sum + diff + prod + quot + rem
}
result := intOps()
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// sum=107, diff=93, prod=700, quot=14 (int div), rem=2
	// total = 107 + 93 + 700 + 14 + 2 = 916
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v (%T), JIT=%v (%T)", vmResult, vmResult, jitResult, jitResult)
	}
}

// TestJIT_HighlyRecursive tests deeper recursion to stress the JIT.
func TestJIT_HighlyRecursive(t *testing.T) {
	src := `
func ack(m, n) {
    if m == 0 { return n + 1 }
    if n == 0 { return ack(m - 1, 1) }
    return ack(m - 1, ack(m, n - 1))
}
result := ack(3, 4)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// ack(3,4) = 125
	expected := int64(125)
	if vmResult != expected {
		t.Errorf("VM ack(3,4): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT ack(3,4): got %v (%T), want %d", jitResult, jitResult, expected)
	}
}

// TestJIT_MixedTableAndArithmetic tests a program that mixes table ops with
// arithmetic in the same loop, causing JIT side-exits mid-trace.
func TestJIT_MixedTableAndArithmetic(t *testing.T) {
	src := `
func matmul() {
    a := {}
    b := {}
    n := 10
    for i := 1; i <= n; i++ {
        a[i] = {}
        b[i] = {}
        for j := 1; j <= n; j++ {
            a[i][j] = i + j
            b[i][j] = i * j
        }
    }
    // Compute c[1][1] = dot product of a's row 1 and b's col 1
    sum := 0
    for k := 1; k <= n; k++ {
        sum = sum + a[1][k] * b[k][1]
    }
    return sum
}
result := matmul()
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// a[1][k] = 1+k, b[k][1] = k*1 = k
	// sum = sum of (1+k)*k for k=1..10 = sum of k+k^2 = 55 + 385 = 440
	expected := int64(440)
	if vmResult != expected {
		t.Errorf("VM matmul: got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT matmul: got %v (%T), want %d", jitResult, jitResult, expected)
	}
}
