package vm

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestUnaryIntArrayMapRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
func map_array(a, f) {
    result := {}
    n := #a
    for i := 1; i <= n; i++ {
        result[i] = f(a[i])
    }
    return result
}
arr := {}
for i := 1; i <= 4; i++ {
    arr[i] = i
}
mapped := map_array(arr, func(x) { return x * 2 + 1 })
result := mapped[3]
`)
	expectGlobalInt(t, globals, "result", 7)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "unary_int_array_map"); got == 0 {
		t.Fatalf("unary_int_array_map hit count = %d, want > 0", got)
	}
}

func TestUnaryIntArrayMapRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func map_array(a, f) {
    return a
}
`)
	child := findTestProtoByName(top, "map_array")
	if child == nil {
		t.Fatal("missing map_array proto")
	}
	if isUnaryIntArrayMapProto(child) {
		t.Fatal("unary_int_array_map should reject name-only matches")
	}
}
