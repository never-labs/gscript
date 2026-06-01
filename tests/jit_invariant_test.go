//go:build darwin && arm64

package tests_test

import (
	"fmt"
	"math"
	"testing"
	"time"

	leia "github.com/never-labs/leia"
)

// assertVMEqualsJIT runs the same source in bytecode-VM mode (no JIT) and in
// JIT mode, then compares the named global variables. The JIT execution is
// wrapped in a goroutine with a timeout to catch hangs.
func assertVMEqualsJIT(t *testing.T, src string, vars ...string) {
	t.Helper()

	// --- VM (bytecode interpreter, no JIT) ---
	vmInstance := leia.New(leia.WithVM())
	if err := vmInstance.Exec(src); err != nil {
		t.Fatalf("VM exec error: %v", err)
	}

	// --- JIT (with trace compilation) ---
	type jitResult struct {
		vm       *leia.VM
		err      error
		panicVal interface{}
	}
	done := make(chan jitResult, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- jitResult{panicVal: r}
			}
		}()
		jitVM := leia.New(leia.WithJIT())
		err := jitVM.Exec(src)
		done <- jitResult{vm: jitVM, err: err}
	}()

	var jitInstance *leia.VM
	select {
	case res := <-done:
		if res.panicVal != nil {
			t.Fatalf("JIT panic: %v", res.panicVal)
		}
		if res.err != nil {
			t.Fatalf("JIT exec error: %v", res.err)
		}
		jitInstance = res.vm
	case <-time.After(5 * time.Second):
		t.Fatal("JIT execution hung (timeout)")
		return
	}

	// --- Compare specified globals ---
	for _, varName := range vars {
		vmVal, vmErr := vmInstance.Get(varName)
		jitVal, jitErr := jitInstance.Get(varName)
		if vmErr != nil {
			t.Fatalf("VM Get(%q) error: %v", varName, vmErr)
		}
		if jitErr != nil {
			t.Fatalf("JIT Get(%q) error: %v", varName, jitErr)
		}

		if !valuesEqual(vmVal, jitVal) {
			t.Errorf("var %q: VM=%v (%T), JIT=%v (%T)", varName, vmVal, vmVal, jitVal, jitVal)
		}
	}
}

// valuesEqual compares two values with tolerance for floats.
func valuesEqual(a, b interface{}) bool {
	fa, aIsFloat := toFloat(a)
	fb, bIsFloat := toFloat(b)
	if aIsFloat && bIsFloat {
		if fa == fb {
			return true
		}
		// Relative tolerance for floating point
		diff := math.Abs(fa - fb)
		mag := math.Max(math.Abs(fa), math.Abs(fb))
		if mag == 0 {
			return diff < 1e-12
		}
		return diff/mag < 1e-9
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int64:
		return float64(x), true
	case int:
		return float64(x), true
	}
	return 0, false
}

// ============================================================================
// Invariant 4: Accumulated correctness (enter-exit-reenter cycles)
// ============================================================================

func TestInv4_Reenter_CounterIncrement(t *testing.T) {
	// trace records "i%10!=0" path. Each time i%10==0, side-exit -> interpreter does c++ -> re-enter
	assertVMEqualsJIT(t, `c:=0; for i:=0;i<100;i++ { if i%10==0 { c=c+1 } }; result:=c`, "result")
}

func TestInv4_Reenter_AccumPlusCondition(t *testing.T) {
	// mix: trace does accumulation, side-exits on condition, interpreter does different accumulation
	assertVMEqualsJIT(t, `a:=0; b:=0; for i:=1;i<=100;i++ { a=a+i; if i%20==0 { b=b+a } }; ra:=a; rb:=b`, "ra", "rb")
}

func TestInv4_Reenter_FibonacciLike(t *testing.T) {
	// loop-carried values that change every iteration
	assertVMEqualsJIT(t, `a:=0; b:=1; for i:=1;i<=30;i++ { c:=a+b; a=b; b=c }; result:=b`, "result")
}

func TestInv4_Mandelbrot_Mini(t *testing.T) {
	// The hardest pattern: nested loop + float + break + accumulator across outer iterations
	src := `
count := 0
for y := 0; y < 5; y++ {
    for x := 0; x < 5; x++ {
        cr := (x * 2.0 / 5) - 1.5
        ci := (y * 2.0 / 5) - 1.0
        zr := 0.0
        zi := 0.0
        escaped := false
        for iter := 0; iter < 20; iter++ {
            zr2 := zr * zr
            zi2 := zi * zi
            if zr2 + zi2 > 4.0 {
                escaped = true
                break
            }
            zi = 2.0 * zr * zi + ci
            zr = zr2 - zi2 + cr
        }
        if !escaped {
            count = count + 1
        }
    }
}
result := count`
	assertVMEqualsJIT(t, src, "result")
}

func TestInv4_Sieve_Mini(t *testing.T) {
	// Array read + conditional + accumulator
	src := `
n := 100
is_prime := {}
for i := 2; i <= n; i++ { is_prime[i] = true }
for i := 2; i * i <= n; i++ {
    if is_prime[i] {
        j := i * i
        for j <= n {
            is_prime[j] = false
            j = j + i
        }
    }
}
count := 0
for i := 2; i <= n; i++ {
    if is_prime[i] { count = count + 1 }
}
result := count`
	assertVMEqualsJIT(t, src, "result")
}

func TestInv4_NBody_Mini(t *testing.T) {
	// Field access + float arithmetic + nested loop + accumulation
	src := `
bodies := {
    {x: 0.0, y: 0.0, vx: 0.0, vy: 0.0, mass: 10.0},
    {x: 1.0, y: 0.0, vx: 0.0, vy: 0.5, mass: 1.0},
}
for step := 1; step <= 50; step++ {
    n := #bodies
    for i := 1; i <= n; i++ {
        bi := bodies[i]
        for j := i + 1; j <= n; j++ {
            bj := bodies[j]
            dx := bi.x - bj.x
            dy := bi.y - bj.y
            dsq := dx*dx + dy*dy + 0.001
            dist := math.sqrt(dsq)
            mag := 0.01 / (dsq * dist)
            bi.vx = bi.vx - dx * bj.mass * mag
            bi.vy = bi.vy - dy * bj.mass * mag
            bj.vx = bj.vx + dx * bi.mass * mag
            bj.vy = bj.vy + dy * bi.mass * mag
        }
    }
    for i := 1; i <= n; i++ {
        b := bodies[i]
        b.x = b.x + 0.01 * b.vx
        b.y = b.y + 0.01 * b.vy
    }
}
result := bodies[2].x`
	assertVMEqualsJIT(t, src, "result")
}

// ============================================================================
// Invariant 5: Type safety
// ============================================================================

func TestInv5_IntGuard(t *testing.T) {
	// Loop computes with ints, trace should work
	assertVMEqualsJIT(t, `s:=0; for i:=1;i<=50;i++ { s=s+i }; result:=s`, "result")
}

func TestInv5_FloatGuard(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0.0; for i:=1;i<=50;i++ { s=s+1.5 }; result:=s`, "result")
}

func TestInv5_TableGuard(t *testing.T) {
	assertVMEqualsJIT(t, `t:={x:1}; s:=0; for i:=1;i<=50;i++ { s=s+t.x; t.x=t.x+1 }; result:=s`, "result")
}

func TestInv5_BoolGuard(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; flag:=true; for i:=1;i<=50;i++ { if flag { c=c+1 } }; result:=c`, "result")
}

// ============================================================================
// CSE correctness — repeated subexpressions
// ============================================================================

func TestInv_CSE_RepeatedSubexpr(t *testing.T) {
	// x*x appears twice per iteration — CSE should deduplicate but result stays correct
	assertVMEqualsJIT(t, `s := 0.0; for i := 1; i <= 100; i++ { x := i * 0.1; s = s + x*x + x*x }`, "s")
}

func TestInv_CSE_IntRepeatedSubexpr(t *testing.T) {
	// Integer repeated subexpression
	assertVMEqualsJIT(t, `s := 0; for i := 1; i <= 100; i++ { s = s + i*i + i*i }`, "s")
}
