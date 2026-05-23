package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func TestTostringNumericToIntegerWrapperRuntimeSpecializationRecordsHits(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
func toint(x) {
  n := tonumber(x)
  if n == nil || n == false {
    return false
  }
  i := math.tointeger(n)
  if i == nil {
    return false
  }
  return i
}

sum := 0
for i := 1; i <= 10; i = i + 1 {
  sum = sum + toint(tostring(i % 4))
}
`)
	if got := globals["sum"]; !got.IsInt() || got.Int() != 15 {
		t.Fatalf("sum = %s, want 15", got.String())
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "tostring_numeric_to_integer_wrapper"); got != 10 {
		t.Fatalf("tostring wrapper specialization hits = %d, want 10", got)
	}
}

func TestUTF8CodepointSumLoopRuntimeSpecializationRecordsHits(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
text := "Az" .. utf8.char(0x4e2d) .. utf8.char(0x03bb)

func bench(n) {
  total := 0
  for i := 1; i <= n; i = i + 1 {
    cpSum := 0
    for pos, cp := range utf8.codes(text) {
      cpSum = cpSum + cp + pos
    }
    total = total + cpSum
  }
  return total
}

result := bench(7)
`)
	if got := globals["result"]; !got.IsInt() || got.Int() != 148169 {
		t.Fatalf("result = %s, want 148169", got.String())
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteDriverLoop, "utf8_codepoint_sum_loop"); got != 7 {
		t.Fatalf("utf8 codepoint sum loop specialization hits = %d, want 7", got)
	}
}
