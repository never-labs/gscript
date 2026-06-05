//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

func TestMethodJITVarargsMultiReturnFeatureMatrixTier1(t *testing.T) {
	const src = `
func vararg_sum(prefix, ...) {
  return prefix + select("#", ...) + select(1, ...) * 10 + select(2, ...) * 100
}

func triple() {
  return 10, 20, 30
}

func none() {
}

func forward() {
  return triple()
}

func adjust_and_non_tail() {
  a, b, c, d := triple()
  e, f := none()
  first := triple()
  return a, b, c, d, e, f, first
}
`
	top := compileTop(t, src)
	varargProto := requireFeatureMatrixProto(t, top, "vararg_sum")
	forwardProto := requireFeatureMatrixProto(t, top, "forward")
	adjustProto := requireFeatureMatrixProto(t, top, "adjust_and_non_tail")

	tm := NewTieringManager()
	gotVararg := runVarargsFeatureCompiledTop(t, top, "vararg_sum", []runtime.Value{
		runtime.IntValue(5),
		runtime.IntValue(7),
		runtime.IntValue(11),
	}, tm, 3)
	wantVararg := runVarargsFeatureCompiledTop(t, compileTop(t, src), "vararg_sum", []runtime.Value{
		runtime.IntValue(5),
		runtime.IntValue(7),
		runtime.IntValue(11),
	}, nil, 1)
	assertVarargsFeatureResultsEqual(t, "Tier1 vararg callee", gotVararg, wantVararg)
	assertVarargsFeatureResultsEqual(t, "Tier1 vararg callee expected", gotVararg, []runtime.Value{runtime.IntValue(1177)})
	if tm.tier1.compiled[varargProto] == nil {
		t.Fatalf("vararg_sum was not compiled at Tier1; Tier2Entered=%d Tier2Failed=%v", varargProto.EnteredTier2, tm.Tier2Failed())
	}
	if varargProto.EnteredTier2 != 0 {
		t.Fatalf("vararg_sum EnteredTier2=%d, want 0 for OP_VARARG Tier1-only shape", varargProto.EnteredTier2)
	}

	gotForward := runVarargsFeatureCompiledTop(t, top, "forward", nil, tm, 3)
	wantForward := runVarargsFeatureCompiledTop(t, compileTop(t, src), "forward", nil, nil, 1)
	assertVarargsFeatureResultsEqual(t, "return g() open return fallback", gotForward, wantForward)
	assertVarargsFeatureResultsEqual(t, "return g() open return expected", gotForward, []runtime.Value{
		runtime.IntValue(10),
		runtime.IntValue(20),
		runtime.IntValue(30),
	})
	if tm.tier1.compiled[forwardProto] != nil || forwardProto.EnteredTier2 != 0 {
		t.Fatalf("forward should stay interpreted for return-all ABI; tier1=%v enteredTier2=%d",
			tm.tier1.compiled[forwardProto] != nil, forwardProto.EnteredTier2)
	}

	gotAdjust := runVarargsFeatureCompiledTop(t, top, "adjust_and_non_tail", nil, tm, 3)
	wantAdjust := runVarargsFeatureCompiledTop(t, compileTop(t, src), "adjust_and_non_tail", nil, nil, 1)
	assertVarargsFeatureResultsEqual(t, "multi-return assignment and non-tail call fallback", gotAdjust, wantAdjust)
	assertVarargsFeatureResultsEqual(t, "multi-return assignment and non-tail call expected", gotAdjust, []runtime.Value{
		runtime.IntValue(10),
		runtime.IntValue(20),
		runtime.IntValue(30),
		runtime.NilValue(),
		runtime.NilValue(),
		runtime.NilValue(),
		runtime.IntValue(10),
	})
	if tm.tier1.compiled[adjustProto] != nil || adjustProto.EnteredTier2 != 0 {
		t.Fatalf("adjust_and_non_tail should stay interpreted for multi-return ABI; tier1=%v enteredTier2=%d",
			tm.tier1.compiled[adjustProto] != nil, adjustProto.EnteredTier2)
	}
}

func TestMethodJITDeclaredVarargUnreadTier2Partial(t *testing.T) {
	const src = `
func ignore_extra(a, ...) {
  return a + 1
}
`
	top := compileTop(t, src)
	proto := requireFeatureMatrixProto(t, top, "ignore_extra")
	if !proto.IsVarArg || proto.UsesVarargBytecode {
		t.Fatalf("ignore_extra shape IsVarArg=%v UsesVarargBytecode=%v, want declared vararg without OP_VARARG",
			proto.IsVarArg, proto.UsesVarargBytecode)
	}

	tm := NewTieringManager()
	if err := tm.CompileTier2(proto); err != nil {
		t.Fatalf("CompileTier2(ignore_extra): %v", err)
	}
	got := runVarargsFeatureCompiledTop(t, top, "ignore_extra", []runtime.Value{
		runtime.IntValue(41),
		runtime.IntValue(100),
		runtime.IntValue(200),
	}, tm, 3)
	if len(got) != 1 {
		t.Fatalf("ignore_extra returned %d values: %v", len(got), got)
	}
	assertValuesEqual(t, "declared vararg unread Tier2", got[0], runtime.IntValue(42))
	if proto.EnteredTier2 == 0 {
		t.Fatal("ignore_extra did not enter Tier2 after explicit Tier2 compile")
	}
}

func TestMethodJITOPVarargTier2FallbackCorrectness(t *testing.T) {
	const src = `
func read_varargs(prefix, ...) {
  return prefix + select("#", ...) + select(1, ...) * 10 + select(2, ...) * 100
}
`
	top := compileTop(t, src)
	proto := requireFeatureMatrixProto(t, top, "read_varargs")
	if !proto.UsesVarargBytecode {
		t.Fatal("read_varargs should use OP_VARARG")
	}

	tm := NewTieringManager()
	err := tm.CompileTier2(proto)
	if err == nil {
		t.Fatal("CompileTier2(read_varargs) unexpectedly accepted OP_VARARG shape")
	}
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "unsupported") && !strings.Contains(errText, "vararg") {
		t.Fatalf("CompileTier2(read_varargs) error = %q, want explicit unsupported/vararg rejection", err)
	}

	args := []runtime.Value{runtime.IntValue(5), runtime.IntValue(7), runtime.IntValue(11)}
	got := runVarargsFeatureCompiledTop(t, top, "read_varargs", args, tm, 3)
	want := runVarargsFeatureCompiledTop(t, compileTop(t, src), "read_varargs", args, nil, 1)
	assertVarargsFeatureResultsEqual(t, "OP_VARARG Tier2 rejected fallback", got, want)
	assertVarargsFeatureResultsEqual(t, "OP_VARARG Tier2 rejected fallback expected", got, []runtime.Value{runtime.IntValue(1177)})
	if proto.EnteredTier2 != 0 {
		t.Fatalf("read_varargs EnteredTier2=%d after rejected Tier2 compile, want 0", proto.EnteredTier2)
	}
}

func requireFeatureMatrixProto(t *testing.T, top *vm.FuncProto, name string) *vm.FuncProto {
	t.Helper()
	proto := findProtoByName(top, name)
	if proto == nil {
		t.Fatalf("%s proto not found", name)
	}
	return proto
}

func runVarargsFeatureCompiledTop(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
	t.Helper()
	globals := vmtest.NewInterpreterGlobals()
	v := vm.New(globals)
	defer v.Close()
	if tm != nil {
		v.SetMethodJIT(tm)
	}
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute top: %v", err)
	}
	fn := v.GetGlobal(fnName)
	if fn.IsNil() {
		t.Fatalf("function %q not found in globals", fnName)
	}
	var got []runtime.Value
	var err error
	for i := 0; i < calls; i++ {
		got, err = v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("CallValue(%s) #%d: %v", fnName, i+1, err)
		}
	}
	return got
}

func assertVarargsFeatureResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, VM returned %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertValuesEqual(t, label, got[i], want[i])
	}
}
