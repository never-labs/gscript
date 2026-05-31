package runtime

import "testing"

func TestConstBindingRejectsAssignment(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		const x := 10
		ok, err := pcall(func() {
			x = 11
		})
	`)

	if interp.GetGlobal("ok").Truthy() {
		t.Fatalf("assigning a const binding should fail")
	}
	if got := interp.GetGlobal("err").Str(); got == "" {
		t.Fatalf("expected const assignment error")
	}
}

func TestConstBindingAllowsTableMutationButNotRebind(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		const cfg = {count: 1}
		cfg.count = 2
		ok, err := pcall(func() {
			cfg = {}
		})
		count := cfg.count
	`)

	if got := interp.GetGlobal("count").Int(); got != 2 {
		t.Fatalf("count = %d, want 2", got)
	}
	if interp.GetGlobal("ok").Truthy() {
		t.Fatalf("rebinding a const table should fail")
	}
}

func TestConstBindingCapturedByClosure(t *testing.T) {
	interp := NewCore()

	execBinaryIOTest(t, interp, `
		func outer() {
			const x := 3
			return func() {
				return x
			}
		}
		f := outer()
		value := f()
	`)

	if got := interp.GetGlobal("value").Int(); got != 3 {
		t.Fatalf("value = %d, want 3", got)
	}
}
