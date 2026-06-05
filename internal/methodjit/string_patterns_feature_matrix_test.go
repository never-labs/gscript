//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

const stringPatternFeatureMatrixSrc = `
func pattern_summary(blob) {
  plain_a, plain_b := string.find(blob, "value=", 1, true)
  pat_a, pat_b := string.find(blob, "tag%d%d")
  item, value := string.match(blob, "(item_%d+);tag%d%d;value=(%d+)")
  rewritten, rewrites := string.gsub(blob, "item_(%d+);(tag%d%d)", func(n, tag) {
    return tag .. ":" .. n
  })
  total := 0
  count := 0
  for n, v := range string.gmatch(blob, "item_(%d+);tag%d%d;value=(%d+)") {
    total = total + tonumber(n) * 10 + tonumber(v)
    count = count + 1
  }
  return plain_a, plain_b, pat_a, pat_b, item, value, rewritten, rewrites, total, count
}

func concat_pattern_driver(n) {
  blob := ""
  for i := 1; i <= n; i++ {
    blob = blob .. "item_" .. tostring(i) .. ";tag" .. string.format("%02d", i) .. ";value=" .. tostring(i * 3) .. "|"
  }
  return pattern_summary(blob)
}
`

func TestMethodJITStringPatternsFeatureMatrixTier1Correctness(t *testing.T) {
	args := []runtime.Value{runtime.IntValue(4)}
	want := runStringPatternFeatureCase(t, compileTop(t, stringPatternFeatureMatrixSrc), "concat_pattern_driver", args, nil, 1)
	got := runStringPatternFeatureCase(t, compileTop(t, stringPatternFeatureMatrixSrc), "concat_pattern_driver", args, NewTieringManager(), 30)

	assertStringPatternFeatureResultsEqual(t, "Tier1 string pattern calls", got, want)
	assertStringPatternFeatureResultsEqual(t, "Tier1 string pattern calls expected", got, []runtime.Value{
		runtime.IntValue(14),
		runtime.IntValue(19),
		runtime.IntValue(8),
		runtime.IntValue(12),
		runtime.StringValue("item_1"),
		runtime.StringValue("3"),
		runtime.StringValue("tag01:1;value=3|tag02:2;value=6|tag03:3;value=9|tag04:4;value=12|"),
		runtime.IntValue(4),
		runtime.IntValue(130),
		runtime.IntValue(4),
	})
}

func TestMethodJITStringPatternsTier2PatternCallsFallbackSafely(t *testing.T) {
	top := compileTop(t, stringPatternFeatureMatrixSrc)
	proto := requireStringPatternFeatureProto(t, top, "pattern_summary")

	tm := NewTieringManager()
	err := tm.CompileTier2(proto)
	if err == nil {
		t.Fatal("CompileTier2(pattern_summary) unexpectedly accepted residual string pattern calls")
	}
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "call") && !strings.Contains(errText, "tier 1") && !strings.Contains(errText, "unsupported") {
		t.Fatalf("CompileTier2(pattern_summary) error = %q, want explicit residual-call/Tier1 fallback rejection", err)
	}
	if proto.EnteredTier2 != 0 {
		t.Fatalf("pattern_summary EnteredTier2=%d after rejected Tier2 compile, want 0", proto.EnteredTier2)
	}

	blob := runtime.StringValue("item_1;tag01;value=3|item_2;tag02;value=6|")
	got := runStringPatternFeatureCase(t, top, "pattern_summary", []runtime.Value{blob}, tm, 8)
	want := runStringPatternFeatureCase(t, compileTop(t, stringPatternFeatureMatrixSrc), "pattern_summary", []runtime.Value{blob}, nil, 1)
	assertStringPatternFeatureResultsEqual(t, "Tier2 rejected pattern-call fallback", got, want)
	assertStringPatternFeatureResultsEqual(t, "Tier2 rejected pattern-call fallback expected", got, []runtime.Value{
		runtime.IntValue(14),
		runtime.IntValue(19),
		runtime.IntValue(8),
		runtime.IntValue(12),
		runtime.StringValue("item_1"),
		runtime.StringValue("3"),
		runtime.StringValue("tag01:1;value=3|tag02:2;value=6|"),
		runtime.IntValue(2),
		runtime.IntValue(39),
		runtime.IntValue(2),
	})
	if proto.EnteredTier2 != 0 {
		t.Fatalf("pattern_summary EnteredTier2=%d after fallback execution, want 0", proto.EnteredTier2)
	}
}

func requireStringPatternFeatureProto(t *testing.T, top *vm.FuncProto, name string) *vm.FuncProto {
	t.Helper()
	proto := findProtoByName(top, name)
	if proto == nil {
		t.Fatalf("%s proto not found", name)
	}
	return proto
}

func runStringPatternFeatureCase(t *testing.T, top *vm.FuncProto, fnName string, args []runtime.Value, tm *TieringManager, calls int) []runtime.Value {
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

func assertStringPatternFeatureResultsEqual(t *testing.T, label string, got, want []runtime.Value) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s returned %d values, want %d: got=%v want=%v", label, len(got), len(want), got, want)
	}
	for i := range got {
		assertStringPatternFeatureValueEqual(t, label, got[i], want[i])
	}
}

func assertStringPatternFeatureValueEqual(t *testing.T, label string, got, want runtime.Value) {
	t.Helper()
	if got == want {
		return
	}
	if got.IsString() && want.IsString() && got.Str() == want.Str() {
		return
	}
	if got.IsNumber() && want.IsNumber() && got.Number() == want.Number() {
		return
	}
	t.Fatalf("%s: got=%v (type=%s), want=%v (type=%s)", label, got, got.TypeName(), want, want.TypeName())
}
