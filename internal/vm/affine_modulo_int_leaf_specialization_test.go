package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestAffineModuloIntLeafRuntimeSpecialization(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
MOD := 1000000007
func mix(sum, v) {
    return (sum * 131 + v) % MOD
}
result := 0
for i := 1; i <= 5; i++ {
    result = mix(result, i)
}
`)
	expectGlobalInt(t, globals, "result", 299048115)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "affine_modulo_int_leaf"); got != 5 {
		t.Fatalf("affine_modulo_int_leaf hit count = %d, want 5", got)
	}
}

func TestAffineModuloIntLeafRejectsDifferentShape(t *testing.T) {
	top := compileProto(t, `
MOD := 1000000007
func notMix(sum, v) {
    return (sum * 131 - v) % MOD
}
`)
	child := findTestProtoByName(top, "notMix")
	if child == nil {
		t.Fatal("missing notMix proto")
	}
	if isAffineModuloIntLeafProto(child) {
		t.Fatal("affine modulo leaf should reject different arithmetic shape")
	}
}
