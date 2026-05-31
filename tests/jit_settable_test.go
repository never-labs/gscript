//go:build darwin && arm64

package tests_test

import (
	"testing"

	gs "github.com/never-labs/gscript"
)

// TestMethodJIT_SettableNative tests SETTABLE native fast path in method JIT.
// The JIT should compile table[int_key] = value natively without call-exit
// for integer keys within the array part.
func TestMethodJIT_SettableNative(t *testing.T) {
	src := `
func fillTable(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i * 2
    }
    return t[100]
}
result := fillTable(1000)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(200)
	if vmResult != expected {
		t.Errorf("VM fillTable(1000): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT fillTable(1000): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestMethodJIT_SettableNativeReadBack tests SETTABLE followed by GETTABLE
// to verify the native fast path writes correct values that can be read back.
func TestMethodJIT_SettableNativeReadBack(t *testing.T) {
	src := `
func buildAndSum(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i
    }
    s := 0
    for i := 1; i <= n; i++ {
        s = s + t[i]
    }
    return s
}
result := buildAndSum(100)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	// sum 1..100 = 5050
	expected := int64(5050)
	if vmResult != expected {
		t.Errorf("VM buildAndSum(100): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT buildAndSum(100): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestMethodJIT_SettableNativeOverwrite tests that SETTABLE correctly overwrites
// existing array elements.
func TestMethodJIT_SettableNativeOverwrite(t *testing.T) {
	src := `
func overwrite(n) {
    t := {}
    for i := 1; i <= n; i++ {
        t[i] = i
    }
    // Overwrite all values
    for i := 1; i <= n; i++ {
        t[i] = t[i] * 3
    }
    return t[50]
}
result := overwrite(100)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(150) // 50 * 3
	if vmResult != expected {
		t.Errorf("VM overwrite(100): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT overwrite(100): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestMethodJIT_SettableConstKey tests SETTABLE with a constant integer key (RK(B) is constant).
func TestMethodJIT_SettableConstKey(t *testing.T) {
	src := `
func setConst(n) {
    t := {}
    // t[1] uses a constant key
    for i := 1; i <= n; i++ {
        t[1] = i
    }
    return t[1]
}
result := setConst(100)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(100) // last write wins
	if vmResult != expected {
		t.Errorf("VM setConst(100): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT setConst(100): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestMethodJIT_SettableAppend tests the SETTABLE append fast path.
// When a table is filled sequentially from an empty state, the append
// fast path should handle most writes natively (when cap > len).
func TestMethodJIT_SettableAppend(t *testing.T) {
	src := `
func appendTest(n) {
    t := {}
    // Fill table sequentially with bool values (stays ArrayMixed).
    for i := 1; i <= n; i++ {
        t[i] = true
    }
    // Read back and count
    count := 0
    for i := 1; i <= n; i++ {
        if t[i] { count = count + 1 }
    }
    return count
}
result := appendTest(1000)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(1000)
	if vmResult != expected {
		t.Errorf("VM appendTest(1000): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT appendTest(1000): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}

// TestMethodJIT_SettableAppendSieve tests the append fast path in a
// sieve-like pattern (the main motivation for this optimization).
func TestMethodJIT_SettableAppendSieve(t *testing.T) {
	src := `
func sieve(n) {
    is_prime := {}
    for i := 2; i <= n; i++ {
        is_prime[i] = true
    }
    i := 2
    for i * i <= n {
        if is_prime[i] {
            j := i * i
            for j <= n {
                is_prime[j] = false
                j = j + i
            }
        }
        i = i + 1
    }
    count := 0
    for i := 2; i <= n; i++ {
        if is_prime[i] { count = count + 1 }
    }
    return count
}
result := sieve(10000)
`
	vmResult := runAndGet(t, src, "result", gs.WithVM())
	jitResult := runAndGet(t, src, "result", gs.WithJIT())

	expected := int64(1229) // number of primes up to 10000
	if vmResult != expected {
		t.Errorf("VM sieve(10000): got %v (%T), want %d", vmResult, vmResult, expected)
	}
	if jitResult != expected {
		t.Errorf("JIT sieve(10000): got %v (%T), want %d", jitResult, jitResult, expected)
	}
	if vmResult != jitResult {
		t.Errorf("VM and JIT results differ: VM=%v, JIT=%v", vmResult, jitResult)
	}
}
