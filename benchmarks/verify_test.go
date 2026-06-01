package benchmarks

import (
	"testing"

	leia "github.com/never-labs/leia"
	lua "github.com/yuin/gopher-lua"
	"go.starlark.net/starlark"
)

// TestVerifyResults ensures all runtimes produce correct results.
func TestVerifyResults(t *testing.T) {
	// Leia tree-walker: fib(20)
	vm := leia.New()
	if err := vm.Exec(`
func fib(n) {
    if n < 2 { return n }
    return fib(n-1) + fib(n-2)
}
result := fib(20)
`); err != nil {
		t.Fatalf("Leia tree-walker error: %v", err)
	}
	r, _ := vm.Get("result")
	if r != int64(6765) {
		t.Errorf("Leia tree-walker: got %v (%T), want 6765", r, r)
	} else {
		t.Logf("Leia tree-walker: fib(20) = %v  OK", r)
	}

	// Leia bytecode VM: fib(20)
	vm2 := leia.New(leia.WithVM())
	if err := vm2.Exec(`
func fib(n) {
    if n < 2 { return n }
    return fib(n-1) + fib(n-2)
}
result := fib(20)
`); err != nil {
		t.Fatalf("Leia bytecode VM error: %v", err)
	}
	r2, _ := vm2.Get("result")
	if r2 != int64(6765) {
		t.Errorf("Leia bytecode VM: got %v (%T), want 6765", r2, r2)
	} else {
		t.Logf("Leia bytecode VM: fib(20) = %v  OK", r2)
	}

	// gopher-lua: fib(20)
	L := lua.NewState()
	defer L.Close()
	L.DoString(`
function fib(n)
    if n < 2 then return n end
    return fib(n-1) + fib(n-2)
end
result = fib(20)
`)
	luaResult := L.GetGlobal("result")
	if ln, ok := luaResult.(lua.LNumber); !ok || int(ln) != 6765 {
		t.Errorf("gopher-lua: got %v, want 6765", luaResult)
	} else {
		t.Logf("gopher-lua: fib(20) = %v  OK", int(ln))
	}

	// Starlark: fib iterative (recursion not supported)
	thread := &starlark.Thread{Name: "verify"}
	globals, err := starlark.ExecFile(thread, "fib.star", `
def fib(n):
    a, b = 0, 1
    for i in range(n):
        a, b = b, a + b
    return a

result = fib(20)
`, nil)
	if err != nil {
		t.Fatalf("Starlark error: %v", err)
	}
	starResult := globals["result"]
	if starResult.String() != "6765" {
		t.Errorf("Starlark: got %v, want 6765", starResult)
	} else {
		t.Logf("Starlark: fib(20) = %v  OK (iterative)", starResult)
	}
}
