package methodjit

import (
	"testing"

	"github.com/never-labs/gscript/internal/vm"
)

func buildForParamTypeGuardTest(t *testing.T, src string) *Function {
	t.Helper()
	proto := compile(t, src)
	proto.ParamTypeFeedback = []vm.ParamTypeFeedbackEntry{
		{Type: vm.FBFloat, Count: 10},
	}
	fn := BuildGraph(proto)
	if errs := Validate(fn); len(errs) > 0 {
		t.Fatalf("validate: %v", errs[0])
	}
	return fn
}

func TestObservedParamTypeGuard_InsertsGuardForFloatParam(t *testing.T) {
	fn := buildForParamTypeGuardTest(t, `
func f(x) {
    return x * 2.0
}
`)

	out, err := ObservedParamTypeGuardPass(fn)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, instr := range out.Entry.Instrs {
		if instr.Op == OpGuardType && instr.Type == TypeFloat {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected GuardType float, IR:\n%s", Print(out))
	}
}

func TestObservedParamTypeGuard_InsertsGuardForIntParam(t *testing.T) {
	proto := compile(t, `
func f(n) {
    total := 0
    for i := 0; i < n; i++ {
        total = total + i
    }
    return total
}
`)
	proto.ParamTypeFeedback = []vm.ParamTypeFeedbackEntry{
		{Type: vm.FBInt, Count: 10},
	}
	fn := BuildGraph(proto)

	out, err := ObservedParamTypeGuardPass(fn)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, instr := range out.Entry.Instrs {
		if instr.Op == OpGuardType && instr.Type == TypeInt {
			found = true
		}
	}
	if !found {
		t.Errorf("expected GuardType int, IR:\n%s", Print(out))
	}
}

func TestObservedParamTypeGuard_SkipsBelowMinCount(t *testing.T) {
	proto := compile(t, `
func f(x) {
    return x
}
`)
	proto.ParamTypeFeedback = []vm.ParamTypeFeedbackEntry{
		{Type: vm.FBFloat, Count: 1}, // below min count of 2
	}
	fn := BuildGraph(proto)

	out, err := ObservedParamTypeGuardPass(fn)
	if err != nil {
		t.Fatal(err)
	}

	for _, instr := range out.Entry.Instrs {
		if instr.Op == OpGuardType && instr.Type == TypeFloat {
			t.Error("should not insert guard below min count")
		}
	}
}

func TestObservedParamTypeGuard_SkipsNonNumeric(t *testing.T) {
	proto := compile(t, `
func f(x) {
    return x
}
`)
	proto.ParamTypeFeedback = []vm.ParamTypeFeedbackEntry{
		{Type: vm.FBTable, Count: 10},
	}
	fn := BuildGraph(proto)

	out, err := ObservedParamTypeGuardPass(fn)
	if err != nil {
		t.Fatal(err)
	}

	for _, instr := range out.Entry.Instrs {
		if instr.Op == OpGuardType {
			t.Error("should not insert guard for table type")
		}
	}
}
