//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

const errorsDeferFeatureMatrixLibSrc = `
events := ""

func mark(s) {
  events = events .. s .. "|"
}

func pcall_success(n) {
  defer mark("ps:defer")
  ok, a, b := pcall(func(x) {
    mark("ps:body")
    return x + 1, x + 2
  }, n)
  if !ok {
    return "pcall-success-failed"
  }
  return tostring(a) .. "," .. tostring(b)
}

func pcall_failure(n) {
  ok, err := pcall(func(x) {
    defer mark("pf:defer")
    mark("pf:body")
    error("pf:" .. tostring(x))
  }, n)
  if ok {
    return "pcall-failure-succeeded"
  }
  return err
}

func xpcall_handler(n) {
  ok, msg := xpcall(func(x) {
    defer mark("xp:defer")
    mark("xp:body")
    error("xp:" .. tostring(x))
  }, func(err) {
    mark("xp:handler")
    return "handled:" .. err
  }, n)
  return tostring(ok) .. ":" .. msg
}

func defer_normal(n) {
  defer mark("dn:first")
  defer mark("dn:second")
  mark("dn:body")
  return n * 2
}

func defer_error_unwind(n) {
  defer mark("du:first")
  defer mark("du:second")
  mark("du:body")
  error("du:" .. tostring(n))
}

func catch_defer_error_unwind(n) {
  events = ""
  ok, err := pcall(defer_error_unwind, n)
  return tostring(ok) .. ":" .. err .. "|" .. events
}

func protected_defer_matrix(n) {
  a := pcall_success(n)
  b := pcall_failure(n + 1)
  c := xpcall_handler(n + 2)
  d := defer_normal(n + 3)
  ok, err := pcall(defer_error_unwind, n + 4)
  return a .. "|" .. b .. "|" .. c .. "|" .. tostring(d) .. "|" .. tostring(ok) .. ":" .. err .. "|" .. events
}
`

const errorsDeferFeatureMatrixSrc = errorsDeferFeatureMatrixLibSrc + `
result := protected_defer_matrix(10)
`

func TestMethodJITErrorsPcallXpcallDeferTier1Parity(t *testing.T) {
	want := runErrorsDeferFeatureTopResult(t, compileTop(t, errorsDeferFeatureMatrixSrc), nil)
	tm := NewTieringManager()
	got := runErrorsDeferFeatureTopResult(t, compileTop(t, errorsDeferFeatureMatrixSrc), tm)

	assertErrorsDeferFeatureValueEqual(t, "errors/pcall/xpcall/defer Tier1-vs-VM result", got, want)
	assertErrorsDeferFeatureValueEqual(t, "errors/pcall/xpcall/defer expected result", got, runtime.StringValue(
		"11,12|pf:11|false:handled:xp:12|26|false:du:14|"+
			"ps:body|ps:defer|pf:body|pf:defer|xp:body|xp:defer|xp:handler|"+
			"dn:body|dn:second|dn:first|du:body|du:second|du:first|",
	))
	if tm.Tier1Count() == 0 {
		t.Fatalf("errors/defer matrix did not compile any Tier1 code; Tier2Entered=%v Tier2Failed=%v", tm.Tier2Entered(), tm.Tier2Failed())
	}
}

func TestMethodJITErrorsPcallXpcallDeferTier2FallbackCorrectness(t *testing.T) {
	const fallbackFn = "protected_defer_matrix"
	want := runErrorsDeferFeatureFunction(t, compileTop(t, errorsDeferFeatureMatrixSrc), fallbackFn, []runtime.Value{
		runtime.IntValue(20),
	}, nil, 1)

	top := compileTop(t, errorsDeferFeatureMatrixLibSrc)
	tm := NewTieringManager()
	type tier2Case struct {
		compileName string
		callName    string
		args        []runtime.Value
	}
	tier2Cases := []tier2Case{
		{"pcall_success", "pcall_success", []runtime.Value{runtime.IntValue(20)}},
		{"pcall_failure", "pcall_failure", []runtime.Value{runtime.IntValue(21)}},
		{"xpcall_handler", "xpcall_handler", []runtime.Value{runtime.IntValue(22)}},
		{"defer_normal", "defer_normal", []runtime.Value{runtime.IntValue(23)}},
		{"defer_error_unwind", "catch_defer_error_unwind", []runtime.Value{runtime.IntValue(24)}},
	}
	for _, tc := range tier2Cases {
		proto := requireFeatureMatrixProto(t, top, tc.compileName)
		tier2Err := tm.CompileTier2(proto)
		if tier2Err != nil {
			errText := strings.ToLower(tier2Err.Error())
			if !strings.Contains(errText, "unsupported") &&
				!strings.Contains(errText, "call") &&
				!strings.Contains(errText, "defer") &&
				!strings.Contains(errText, "staying at tier 1") {
				t.Fatalf("CompileTier2(%s) error = %q, want explicit protected/defer unsupported fallback", tc.compileName, tier2Err)
			}
			continue
		}
		proto.CallCount = tmDefaultTier2Threshold + 1
		wantDirect := runErrorsDeferFeatureFunction(t, compileTop(t, errorsDeferFeatureMatrixLibSrc), tc.callName, tc.args, nil, 1)
		gotDirect := runErrorsDeferFeatureFunction(t, top, tc.callName, tc.args, tm, 3)
		assertErrorsDeferFeatureResultsEqual(t, tc.compileName+" Tier2 direct correctness", gotDirect, wantDirect)
		if proto.EnteredTier2 == 0 {
			t.Fatalf("%s compiled for Tier2 but never entered", tc.compileName)
		}
	}

	fallbackTop := compileTop(t, errorsDeferFeatureMatrixLibSrc)
	fallbackTM := NewTieringManager()
	got := runErrorsDeferFeatureFunction(t, fallbackTop, fallbackFn, []runtime.Value{
		runtime.IntValue(20),
	}, fallbackTM, 3)
	assertErrorsDeferFeatureResultsEqual(t, "protected/defer Tier2 or fallback", got, want)
	assertErrorsDeferFeatureResultsEqual(t, "protected/defer Tier2 or fallback expected", got, []runtime.Value{runtime.StringValue(
		"21,22|pf:21|false:handled:xp:22|46|false:du:24|" +
			"ps:body|ps:defer|pf:body|pf:defer|xp:body|xp:defer|xp:handler|" +
			"dn:body|dn:second|dn:first|du:body|du:second|du:first|",
	)})
	for _, tc := range tier2Cases {
		proto := requireFeatureMatrixProto(t, top, tc.compileName)
		if _, compiled := tm.tier2CompiledFor(proto); !compiled && proto.EnteredTier2 != 0 {
			t.Fatalf("%s entered Tier2 after rejected compile; entered=%d failed=%v", tc.compileName, proto.EnteredTier2, tm.Tier2Failed())
		}
	}
}

func runErrorsDeferFeatureTopResult(t *testing.T, top *vm.FuncProto, tm *TieringManager) runtime.Value {
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
	got := v.GetGlobal("result")
	if got.IsNil() {
		t.Fatal("global result not found")
	}
	return got
}

func runErrorsDeferFeatureFunction(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
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
	v.SetGlobal("events", runtime.StringValue(""))
	fn := v.GetGlobal(fnName)
	if fn.IsNil() {
		t.Fatalf("function %q not found in globals", fnName)
	}
	var got []runtime.Value
	var err error
	for i := 0; i < calls; i++ {
		v.SetGlobal("events", runtime.StringValue(""))
		got, err = v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("CallValue(%s) #%d: %v", fnName, i+1, err)
		}
	}
	return got
}

func assertErrorsDeferFeatureResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, VM returned %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertErrorsDeferFeatureValueEqual(t, label, got[i], want[i])
	}
}

func assertErrorsDeferFeatureValueEqual(t *testing.T, label string, got, want runtime.Value) {
	t.Helper()
	if got.IsString() && want.IsString() {
		if got.Str() != want.Str() {
			t.Fatalf("%s: got %q, want %q", label, got.Str(), want.Str())
		}
		return
	}
	assertValuesEqual(t, label, got, want)
}
