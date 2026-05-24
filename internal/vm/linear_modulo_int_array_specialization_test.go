package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func TestLinearModuloIntArrayBuilderAndChecksumSpecializations(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, `
MOD := 1000000007
func make_array(n, salt) {
    a := {}
    for i := 1; i <= n; i++ {
        a[i] = (i * 97 + salt * 53 + n * 17) % 100000
    }
    return a
}
func checksum_array(a, n) {
    h := 17
    for i := 1; i <= n; i++ {
        h = (h * 131 + a[i] * (i % 97 + 1)) % MOD
    }
    return h
}
a := make_array(8, 3)
result := checksum_array(a, 8)
`)
	expectGlobalInt(t, globals, "result", 787589418)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "linear_modulo_int_array_builder"); got == 0 {
		t.Fatalf("linear_modulo_int_array_builder hit count = %d, want > 0", got)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "indexed_modulo_int_array_checksum"); got == 0 {
		t.Fatalf("indexed_modulo_int_array_checksum hit count = %d, want > 0", got)
	}
}

func TestLinearModuloIntArrayBuilderRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func make_array(n, salt) {
    return {n, salt}
}
`)
	child := findTestProtoByName(top, "make_array")
	if child == nil {
		t.Fatal("missing make_array proto")
	}
	if isLinearModuloIntArrayBuilderProto(child) {
		t.Fatal("linear_modulo_int_array_builder should reject name-only matches")
	}
}
