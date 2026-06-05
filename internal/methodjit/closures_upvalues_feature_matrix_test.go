//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

const closuresUpvaluesFeatureMatrixSrc = `
func make_counter(start) {
  value := start
  return func(delta) {
    value = value + delta
    return value
  }
}

func make_pair(start) {
  shared := start
  return func(delta) {
    shared = shared + delta
    return shared
  }, func(delta) {
    shared = shared - delta
    return shared
  }
}

func make_vararg_counter(start) {
  value := start
  func vararg_counter(prefix, ...) {
    value = value + prefix + select("#", ...) + select(1, ...)
    return value
  }
  return vararg_counter
}

func call_counter(counter, n) {
  total := 0
  for i := 1; i <= n; i++ {
    total = total + counter(i)
  }
  return total
}

func shared_pair_walk(inc, dec, n) {
  total := 0
  for i := 1; i <= n; i++ {
    total = total + inc(i)
    total = total + dec(1)
  }
  return total
}

func independent_counter_walk(left, right, n) {
  total := 0
  for i := 1; i <= n; i++ {
    total = total + left(i)
    total = total + right(i * 2)
  }
  return total
}

func closures_upvalues_matrix(n) {
  c1 := make_counter(10)
  c2 := make_counter(100)
  shared_inc, shared_dec := make_pair(50)
  return call_counter(c1, n) +
    call_counter(c2, n) +
    shared_pair_walk(shared_inc, shared_dec, n) +
    independent_counter_walk(c1, c2, n)
}
`

func TestMethodJITClosuresUpvaluesTier1Parity(t *testing.T) {
	args := []runtime.Value{runtime.IntValue(6)}
	want := runClosuresUpvaluesFunction(t, compileTop(t, closuresUpvaluesFeatureMatrixSrc), "closures_upvalues_matrix", args, nil, 1)

	tm := NewTieringManager()
	got := runClosuresUpvaluesFunction(t, compileTop(t, closuresUpvaluesFeatureMatrixSrc), "closures_upvalues_matrix", args, tm, 20)

	assertClosuresUpvaluesResultsEqual(t, "closures/upvalues Tier1-vs-VM", got, want)
	assertClosuresUpvaluesResultsEqual(t, "closures/upvalues expected", got, []runtime.Value{runtime.IntValue(2528)})
	if tm.Tier1Count() == 0 {
		t.Fatalf("closures/upvalues matrix did not compile any Tier1 code; Tier2Entered=%v Tier2Failed=%v", tm.Tier2Entered(), tm.Tier2Failed())
	}
}

func TestMethodJITClosuresUpvaluesTier2CompileOrFallbackCorrectness(t *testing.T) {
	t.Run("prebuilt closure call shape", func(t *testing.T) {
		top := compileTop(t, closuresUpvaluesFeatureMatrixSrc)
		wantRejected := runClosuresUpvaluesCallCounterWithFreshCounter(t, compileTop(t, closuresUpvaluesFeatureMatrixSrc), 10, 6, nil, 3)

		proto := requireFeatureMatrixProto(t, top, "call_counter")
		tm := NewTieringManager()
		tier2Err := tm.CompileTier2(proto)
		if tier2Err != nil {
			assertClosuresUpvaluesTier2RejectReason(t, "call_counter", tier2Err)
			got := runClosuresUpvaluesCallCounterWithFreshCounter(t, top, 10, 6, tm, 3)
			assertClosuresUpvaluesResultsEqual(t, "closure call rejected Tier2 fallback", got, wantRejected)
			if proto.EnteredTier2 != 0 {
				t.Fatalf("call_counter EnteredTier2=%d after rejected Tier2 compile, want 0", proto.EnteredTier2)
			}
			return
		}

		proto.CallCount = tmDefaultTier2Threshold + 1
		wantAccepted := runClosuresUpvaluesCallCounterWithFreshCounter(t, compileTop(t, closuresUpvaluesFeatureMatrixSrc), 10, 6, nil, 3)
		got := runClosuresUpvaluesCallCounterWithFreshCounter(t, top, 10, 6, tm, 3)
		assertClosuresUpvaluesResultsEqual(t, "closure call accepted Tier2 correctness", got, wantAccepted)
		if proto.EnteredTier2 == 0 {
			t.Fatalf("call_counter compiled for Tier2 but never entered")
		}
	})

	t.Run("upvalue-heavy vararg closure shape", func(t *testing.T) {
		top := compileTop(t, closuresUpvaluesFeatureMatrixSrc)
		want := runClosuresUpvaluesVarargCounterWithFreshCounter(t, compileTop(t, closuresUpvaluesFeatureMatrixSrc), 20, nil, 3)

		proto := requireFeatureMatrixProto(t, top, "vararg_counter")
		if len(proto.Upvalues) == 0 || !proto.UsesVarargBytecode {
			t.Fatalf("vararg_counter shape Upvalues=%d UsesVarargBytecode=%v, want captured upvalue plus OP_VARARG", len(proto.Upvalues), proto.UsesVarargBytecode)
		}
		tm := NewTieringManager()
		tier2Err := tm.CompileTier2(proto)
		if tier2Err == nil {
			t.Fatal("CompileTier2(vararg_counter) unexpectedly accepted upvalue-heavy OP_VARARG closure shape")
		}
		assertClosuresUpvaluesTier2RejectReason(t, "vararg_counter", tier2Err)
		got := runClosuresUpvaluesVarargCounterWithFreshCounter(t, top, 20, tm, 3)
		assertClosuresUpvaluesResultsEqual(t, "upvalue-heavy vararg closure rejected Tier2 fallback", got, want)
		if proto.EnteredTier2 != 0 {
			t.Fatalf("vararg_counter EnteredTier2=%d after rejected Tier2 compile, want 0", proto.EnteredTier2)
		}
	})
}

func runClosuresUpvaluesFunction(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
	t.Helper()
	return runClosuresUpvaluesFunctionValue(t, top, fnName, args, tm, calls)
}

func runClosuresUpvaluesFunctionValue(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
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

func runClosuresUpvaluesCallCounterWithFreshCounter(t *testing.T, top *vm.FuncProto, start, n int64, tm *TieringManager, calls int) []runtime.Value {
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
	makeCounter := v.GetGlobal("make_counter")
	if makeCounter.IsNil() {
		t.Fatal("function \"make_counter\" not found in globals")
	}
	counter, err := v.CallValue(makeCounter, []runtime.Value{runtime.IntValue(start)})
	if err != nil {
		t.Fatalf("CallValue(make_counter): %v", err)
	}
	if len(counter) != 1 {
		t.Fatalf("make_counter returned %d values, want 1: %v", len(counter), counter)
	}
	callCounter := v.GetGlobal("call_counter")
	if callCounter.IsNil() {
		t.Fatal("function \"call_counter\" not found in globals")
	}
	args := []runtime.Value{counter[0], runtime.IntValue(n)}
	var got []runtime.Value
	for i := 0; i < calls; i++ {
		got, err = v.CallValue(callCounter, args)
		if err != nil {
			t.Fatalf("CallValue(call_counter) #%d: %v", i+1, err)
		}
	}
	return got
}

func runClosuresUpvaluesVarargCounterWithFreshCounter(t *testing.T, top *vm.FuncProto, start int64, tm *TieringManager, calls int) []runtime.Value {
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
	makeCounter := v.GetGlobal("make_vararg_counter")
	if makeCounter.IsNil() {
		t.Fatal("function \"make_vararg_counter\" not found in globals")
	}
	counter, err := v.CallValue(makeCounter, []runtime.Value{runtime.IntValue(start)})
	if err != nil {
		t.Fatalf("CallValue(make_vararg_counter): %v", err)
	}
	if len(counter) != 1 {
		t.Fatalf("make_vararg_counter returned %d values, want 1: %v", len(counter), counter)
	}
	counterFn := counter[0]
	args := []runtime.Value{runtime.IntValue(3), runtime.IntValue(5), runtime.IntValue(7)}
	var got []runtime.Value
	for i := 0; i < calls; i++ {
		got, err = v.CallValue(counterFn, args)
		if err != nil {
			t.Fatalf("CallValue(vararg_counter) #%d: %v", i+1, err)
		}
	}
	return got
}

func assertClosuresUpvaluesTier2RejectReason(t *testing.T, fnName string, err error) {
	t.Helper()
	errText := strings.ToLower(err.Error())
	if strings.Contains(errText, "unsupported") ||
		strings.Contains(errText, "closure") ||
		strings.Contains(errText, "upval") ||
		strings.Contains(errText, "call") ||
		strings.Contains(errText, "staying at tier 1") {
		return
	}
	t.Fatalf("CompileTier2(%s) error = %q, want explicit closure/upvalue unsupported fallback", fnName, err)
}

func assertClosuresUpvaluesResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, VM returned %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertValuesEqual(t, label, got[i], want[i])
	}
}
