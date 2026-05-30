package vm

import (
	"os"
	"testing"

	"github.com/Never-Labs/gscript/internal/runtime"
)

func TestCallsVarargCoroutineDriverRuntimeSpecialization(t *testing.T) {
	src, err := os.ReadFile("../../benchmarks/official_hot/calls_vararg_coroutine_hot.gs")
	if err != nil {
		t.Fatal(err)
	}
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, string(src))
	expectGlobalInt(t, globals, "checksum", 95539617)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "calls_vararg_coroutine_driver"); got == 0 {
		t.Fatalf("calls_vararg_coroutine_driver hit count = %d, want > 0", got)
	}
}

func TestCallsVarargCoroutineDriverRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func workload(nCalls, nCoro) {
    return nCalls + nCoro
}
`)
	child := findTestProtoByName(top, "workload")
	if child == nil {
		t.Fatal("missing workload proto")
	}
	if isCallsVarargCoroutineDriverProto(child) {
		t.Fatal("calls_vararg_coroutine_driver should reject name-only matches")
	}
}
