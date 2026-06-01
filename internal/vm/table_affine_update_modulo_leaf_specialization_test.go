package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestTableAffineUpdateModuloLeafRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
MOD := 1000000007
func update(t, x, y) {
    t.count = t.count + 1
    return (x * 17 + y * 19 + t.bias + t.count * 23) % MOD
}
obj := {bias: 5, count: 0}
result := 0
for i := 1; i <= 5; i++ {
    result = result + update(obj, i, i % 3)
}
final := obj.count
`)
	expectGlobalInt(t, globals, "result", 739)
	expectGlobalInt(t, globals, "final", 5)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "table_affine_update_modulo_leaf"); got == 0 {
		t.Fatalf("table_affine_update_modulo_leaf hit count = %d, want > 0", got)
	}
}

func TestTableAffineUpdateModuloLeafRecognizesShapeIndependentOfNames(t *testing.T) {
	top := compileProto(t, `
M := 97
func step(state, left, right) {
    state.n = state.n + 1
    return (left * 3 + right * 5 + state.offset + state.n * 7) % M
}
`)
	child := findTestProtoByName(top, "step")
	if child == nil {
		t.Fatal("missing step proto")
	}
	if !isTableAffineUpdateModuloLeafProto(child) {
		t.Fatal("table affine update modulo leaf should recognize bytecode shape independent of names")
	}
}

func TestTableAffineUpdateModuloLeafRejectsDifferentShape(t *testing.T) {
	top := compileProto(t, `
MOD := 1000000007
func update(t, x, y) {
    t.count = t.count + 2
    return (x * 17 + y * 19 + t.bias + t.count * 23) % MOD
}
`)
	child := findTestProtoByName(top, "update")
	if child == nil {
		t.Fatal("missing update proto")
	}
	if isTableAffineUpdateModuloLeafProto(child) {
		t.Fatal("table affine update modulo leaf should reject different update step")
	}
}
