//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

const methodJITCoroutinesFeatureMatrixSrc = `
func coroutine_matrix(n) {
  co := coroutine.create(func(seed) {
    total := seed
    for i := 1; i <= n; i++ {
      step := coroutine.yield("step", total + i)
      total = total + step
    }
    return "done", total
  })

  s0 := coroutine.status(co)
  ok, tag, value := coroutine.resume(co, 10)
  s1 := coroutine.status(co)
  if !ok || tag != "step" || value != 11 || s0 != "suspended" || s1 != "suspended" {
    return "bad:first:" .. tostring(ok) .. ":" .. tostring(tag) .. ":" .. tostring(value) .. ":" .. s0 .. ":" .. s1
  }

  checksum := value
  for i := 1; i <= n; i++ {
    ok, tag, value = coroutine.resume(co, i)
    if !ok {
      return "bad:resume:" .. tostring(i) .. ":" .. tostring(tag)
    }
    if i < n {
      if tag != "step" || coroutine.status(co) != "suspended" {
        return "bad:yield:" .. tostring(i) .. ":" .. tostring(tag) .. ":" .. coroutine.status(co)
      }
      checksum = checksum + value
    } else {
      if tag != "done" || coroutine.status(co) != "dead" {
        return "bad:done:" .. tostring(tag) .. ":" .. coroutine.status(co)
      }
      checksum = checksum + value
    }
  }

  return s0 .. "|" .. s1 .. "|" .. coroutine.status(co) .. "|" .. tostring(checksum)
}
`

func TestMethodJITCoroutinesTier1Parity(t *testing.T) {
	args := []runtime.Value{runtime.IntValue(3)}
	want := runMethodJITCoroutinesFunction(t, compileTop(t, methodJITCoroutinesFeatureMatrixSrc), "coroutine_matrix", args, nil, 1)

	tm := NewTieringManager()
	got := runMethodJITCoroutinesFunction(t, compileTop(t, methodJITCoroutinesFeatureMatrixSrc), "coroutine_matrix", args, tm, 20)

	assertMethodJITCoroutinesResultsEqual(t, "coroutines Tier1-vs-VM", got, want)
	assertMethodJITCoroutinesResultsEqual(t, "coroutines expected", got, []runtime.Value{
		runtime.StringValue("suspended|suspended|dead|56"),
	})
	if tm.Tier1Count() == 0 {
		t.Fatalf("coroutine matrix did not compile any Tier1 code; Tier2Entered=%v Tier2Failed=%v", tm.Tier2Entered(), tm.Tier2Failed())
	}
}

func TestMethodJITCoroutinesTier2CompileOrFallbackCorrectness(t *testing.T) {
	const fnName = "coroutine_matrix"
	args := []runtime.Value{runtime.IntValue(3)}
	want := runMethodJITCoroutinesFunction(t, compileTop(t, methodJITCoroutinesFeatureMatrixSrc), fnName, args, nil, 1)

	top := compileTop(t, methodJITCoroutinesFeatureMatrixSrc)
	proto := requireMethodJITCoroutinesProto(t, top, fnName)

	tm := NewTieringManager()
	tier2Err := tm.CompileTier2(proto)
	if tier2Err != nil {
		errText := strings.ToLower(tier2Err.Error())
		if !strings.Contains(errText, "unsupported") &&
			!strings.Contains(errText, "coroutine") &&
			!strings.Contains(errText, "yield") &&
			!strings.Contains(errText, "call") &&
			!strings.Contains(errText, "staying at tier 1") {
			t.Fatalf("CompileTier2(%s) error = %q, want explicit coroutine unsupported fallback", fnName, tier2Err)
		}

		got := runMethodJITCoroutinesFunction(t, top, fnName, args, tm, 8)
		assertMethodJITCoroutinesResultsEqual(t, "coroutines rejected Tier2 fallback", got, want)
		if proto.EnteredTier2 != 0 {
			t.Fatalf("%s EnteredTier2=%d after rejected Tier2 compile, want 0", fnName, proto.EnteredTier2)
		}
		if _, compiled := tm.tier2CompiledFor(proto); compiled {
			t.Fatalf("%s has Tier2 code after rejected compile", fnName)
		}
		return
	}

	proto.CallCount = tmDefaultTier2Threshold + 1
	got := runMethodJITCoroutinesFunction(t, top, fnName, args, tm, 8)
	assertMethodJITCoroutinesResultsEqual(t, "coroutines accepted Tier2 correctness", got, want)
	if proto.EnteredTier2 == 0 {
		t.Fatalf("%s compiled for Tier2 but never entered", fnName)
	}
}

func requireMethodJITCoroutinesProto(t *testing.T, top *vm.FuncProto, name string) *vm.FuncProto {
	t.Helper()
	proto := findProtoByName(top, name)
	if proto == nil {
		t.Fatalf("%s proto not found", name)
	}
	return proto
}

func runMethodJITCoroutinesFunction(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
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

func assertMethodJITCoroutinesResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, VM returned %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertMethodJITCoroutinesValueEqual(t, label, got[i], want[i])
	}
}

func assertMethodJITCoroutinesValueEqual(t *testing.T, label string, got, want runtime.Value) {
	t.Helper()
	if got.IsString() && want.IsString() {
		if got.Str() != want.Str() {
			t.Fatalf("%s: got %q, want %q", label, got.Str(), want.Str())
		}
		return
	}
	assertValuesEqual(t, label, got, want)
}
