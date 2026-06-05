//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

func TestMethodJITBitwiseDirectOpsTier1FallbackCorrectness(t *testing.T) {
	const src = `
func direct_bitwise(n) {
  total := 0
  for i := 0; i < n; i++ {
    a := (i << 5) | 0xF0
    b := (i * 17) ^ 0xCC
    c := (a & b) + (a | b) + (a ^ b)
    d := (a &^ b) + (^b & 255)
    e := (0x12345678 << (i % 5)) >> (i % 7)
    total = total + c + d + e
  }
  return total
}
`
	want := runBitwiseFeatureFunction(t, src, "direct_bitwise", []runtime.Value{runtime.IntValue(32)}, false, nil)
	tm := NewTieringManager()
	got := runBitwiseFeatureFunction(t, src, "direct_bitwise", []runtime.Value{runtime.IntValue(32)}, true, tm)
	assertBitwiseFeatureResultsEqual(t, "direct bitwise MethodJIT", got, want)
	if tm.Tier1Count() == 0 {
		t.Fatalf("direct_bitwise did not compile at Tier1; Tier2Entered=%v Tier2Failed=%v", tm.Tier2Entered(), tm.Tier2Failed())
	}

	const tier2Src = `
func direct_bitwise_once(a, b, s) {
  c := (a & b) + (a | b) + (a ^ b)
  d := (a &^ b) + (^b & 255)
  e := (0x12345678 << s) >> (s + 1)
  return c + d + e
}
`
	tier2Want := runBitwiseFeatureFunction(t, tier2Src, "direct_bitwise_once", []runtime.Value{
		runtime.IntValue(0x1F2F),
		runtime.IntValue(0x0CC3),
		runtime.IntValue(3),
	}, false, nil)
	top := compileTop(t, tier2Src)
	proto := findProtoByName(top, "direct_bitwise_once")
	if proto == nil {
		t.Fatal("direct_bitwise_once proto not found")
	}
	tm2 := NewTieringManager()
	tier2Err := tm2.CompileTier2(proto)
	if tier2Err != nil {
		errText := strings.ToLower(tier2Err.Error())
		if !strings.Contains(errText, "bit") && !strings.Contains(errText, "unsupported") && !strings.Contains(errText, "staying at tier 1") {
			t.Fatalf("CompileTier2(direct_bitwise_once) error = %q, want explicit bitwise/unsupported/Tier1 fallback rejection", tier2Err)
		}
	} else {
		proto.CallCount = tmDefaultTier2Threshold + 1
	}
	got = runBitwiseFeatureCompiledTop(t, top, "direct_bitwise_once", []runtime.Value{
		runtime.IntValue(0x1F2F),
		runtime.IntValue(0x0CC3),
		runtime.IntValue(3),
	}, tm2)
	assertBitwiseFeatureResultsEqual(t, "direct bitwise Tier2/fallback", got, tier2Want)
	if tier2Err == nil && proto.EnteredTier2 == 0 {
		t.Fatal("direct_bitwise_once compiled for Tier2 but never entered")
	}
	if tier2Err != nil && proto.EnteredTier2 != 0 {
		t.Fatalf("direct_bitwise_once entered Tier2 after rejected compile; entered=%d failed=%v", proto.EnteredTier2, tm2.Tier2Failed())
	}
}

func TestMethodJITBit32FeatureMatrixFallbackCorrectness(t *testing.T) {
	const src = `
func bit32_mix(n) {
  total := 0
  for i := 1; i <= n; i++ {
    x := i * 2654435761
    y := i * 97 + 0x12345678
    folded := bit32.bxor(bit32.band(x, 0xFFFF, 0xFF00FF), bit32.bor(y, 0x1000), bit32.bnot(i))
    left := bit32.lshift(folded, i % 9)
    right := bit32.rshift(folded, i % 11)
    arith := bit32.arshift(0x80000000 + i, i % 5)
    rot := bit32.bxor(bit32.lrotate(left, i % 32), bit32.rrotate(right, i % 32))
    field := i % 16
    extracted := bit32.extract(rot, field, 4)
    replaced := bit32.replace(rot, extracted + i, field, 4)
    total = total + bit32.band(replaced, 0xFFFF) + bit32.band(arith, 0xFF)
  }
  return total
}
`
	want := runBitwiseFeatureFunction(t, src, "bit32_mix", []runtime.Value{runtime.IntValue(24)}, false, nil)

	top := compileTop(t, src)
	proto := findProtoByName(top, "bit32_mix")
	if proto == nil {
		t.Fatal("bit32_mix proto not found")
	}
	tm := NewTieringManager()
	tier2Err := tm.CompileTier2(proto)
	if tier2Err != nil {
		errText := strings.ToLower(tier2Err.Error())
		if !strings.Contains(errText, "call") && !strings.Contains(errText, "unsupported") && !strings.Contains(errText, "bit") {
			t.Fatalf("CompileTier2(bit32_mix) error = %q, want an explicit unsupported/call rejection", tier2Err)
		}
	}

	got := runBitwiseFeatureCompiledTop(t, top, "bit32_mix", []runtime.Value{runtime.IntValue(24)}, tm)
	assertBitwiseFeatureResultsEqual(t, "bit32 MethodJIT", got, want)
	if tier2Err != nil && proto.EnteredTier2 != 0 {
		t.Fatalf("bit32_mix entered Tier2 after rejected compile; entered=%d failed=%v", proto.EnteredTier2, tm.Tier2Failed())
	}
}

func runBitwiseFeatureFunction(t *testing.T, src, fnName string, args []runtime.Value, jit bool, tm *TieringManager) []runtime.Value {
	t.Helper()
	top := compileTop(t, src)
	if jit && tm == nil {
		tm = NewTieringManager()
	}
	return runBitwiseFeatureCompiledTop(t, top, fnName, args, tm)
}

func runBitwiseFeatureCompiledTop(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager) []runtime.Value {
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
	for i := 0; i < 20; i++ {
		got, err = v.CallValue(fn, args)
		if err != nil {
			t.Fatalf("CallValue(%s) #%d: %v", fnName, i+1, err)
		}
	}
	return got
}

func assertBitwiseFeatureResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, VM returned %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertValuesEqual(t, label, got[i], want[i])
	}
}
