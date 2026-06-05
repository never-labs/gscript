//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

const methodJITMetamethodsFeatureMatrixSrc = `
func metamethod_matrix(n) {
  backing := {}
  obj := setmetatable({seed: 3}, {
    __index: func(_, k) {
      if k == "bonus" {
        return backing[k] + 1
      }
      return backing[k] + 2
    },
    __newindex: func(_, k, v) {
      backing[k] = v * 2
    },
    __call: func(t, a, b) {
      return t.seed + a + b
    },
  })

  cmpmt := {}
  cmpmt.__eq = func(a, b) { return a.v == b.v }
  cmpmt.__lt = func(a, b) { return a.v < b.v }
  cmpmt.__le = func(a, b) { return a.v <= b.v }

  obj.bonus = 10
  total := 0
  for i := 1; i <= n; i++ {
    left := setmetatable({v: i}, cmpmt)
    right := setmetatable({v: i + 1}, cmpmt)
    same := setmetatable({v: i}, cmpmt)

    if left < right {
      total = total + 1
    }
    if left <= right {
      total = total + 2
    }
    if left == same {
      total = total + 3
    }

    obj.extra = i
    total = total + obj.bonus + obj(i, 1) + obj.extra
  }
  return total
}
`

func TestMethodJITMetamethodsTier1MatchesVM(t *testing.T) {
	args := []runtime.Value{runtime.IntValue(8)}
	want := runMethodJITMetamethodsFunction(t, compileTop(t, methodJITMetamethodsFeatureMatrixSrc), "metamethod_matrix", args, nil, 1)

	tm := NewTieringManager()
	got := runMethodJITMetamethodsFunction(t, compileTop(t, methodJITMetamethodsFeatureMatrixSrc), "metamethod_matrix", args, tm, 20)

	assertMethodJITMetamethodsResultsEqual(t, "metamethods Tier1-vs-VM", got, want)
	assertMethodJITMetamethodsResultsEqual(t, "metamethods expected", got, []runtime.Value{runtime.IntValue(372)})
	if tm.Tier1Count() == 0 {
		t.Fatalf("metamethod_matrix did not compile any Tier1 code; Tier2Entered=%v Tier2Failed=%v", tm.Tier2Entered(), tm.Tier2Failed())
	}
}

func TestMethodJITMetamethodsTier2CompileOrFallbackCorrectness(t *testing.T) {
	const fnName = "metamethod_matrix"
	args := []runtime.Value{runtime.IntValue(10)}
	want := runMethodJITMetamethodsFunction(t, compileTop(t, methodJITMetamethodsFeatureMatrixSrc), fnName, args, nil, 1)

	top := compileTop(t, methodJITMetamethodsFeatureMatrixSrc)
	proto := findProtoByName(top, fnName)
	if proto == nil {
		t.Fatalf("%s proto not found", fnName)
	}

	globals := vmtest.NewInterpreterGlobals()
	v := vm.New(globals)
	defer v.Close()
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute top: %v", err)
	}
	fn := v.GetGlobal(fnName)
	if fn.IsNil() {
		t.Fatalf("function %q not found in globals", fnName)
	}
	warm, err := v.CallValue(fn, args)
	if err != nil {
		t.Fatalf("warm CallValue(%s): %v", fnName, err)
	}
	assertMethodJITMetamethodsResultsEqual(t, "metamethods warm VM", warm, want)

	tm := NewTieringManager()
	v.SetMethodJIT(tm)
	tier2Err := tm.CompileTier2(proto)
	if tier2Err != nil {
		errText := strings.ToLower(tier2Err.Error())
		if !strings.Contains(errText, "metamethod") &&
			!strings.Contains(errText, "call") &&
			!strings.Contains(errText, "unsupported") &&
			!strings.Contains(errText, "staying at tier 1") {
			t.Fatalf("CompileTier2(%s) error = %q, want explicit metamethod-heavy unsupported fallback", fnName, tier2Err)
		}

		got := callMethodJITMetamethodsFunction(t, v, fn, fnName, args, 3)
		assertMethodJITMetamethodsResultsEqual(t, "metamethods rejected Tier2 fallback", got, want)
		if proto.EnteredTier2 != 0 {
			t.Fatalf("%s EnteredTier2=%d after rejected Tier2 compile, want 0", fnName, proto.EnteredTier2)
		}
		if _, compiled := tm.tier2CompiledFor(proto); compiled {
			t.Fatalf("%s is present in tier2Compiled after rejected Tier2 compile", fnName)
		}
		return
	}

	proto.CallCount = tmDefaultTier2Threshold + 1
	got := callMethodJITMetamethodsFunction(t, v, fn, fnName, args, 3)
	assertMethodJITMetamethodsResultsEqual(t, "metamethods accepted Tier2 correctness", got, want)
	if proto.EnteredTier2 == 0 {
		t.Fatalf("%s compiled for Tier2 but never entered", fnName)
	}
}

func runMethodJITMetamethodsFunction(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
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
	return callMethodJITMetamethodsFunction(t, v, fn, fnName, args, calls)
}

func callMethodJITMetamethodsFunction(t *testing.T, v *vm.VM, fn runtime.Value, fnName string, args []runtime.Value, calls int) []runtime.Value {
	t.Helper()
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

func assertMethodJITMetamethodsResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, want %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertValuesEqual(t, label, got[i], want[i])
	}
}
