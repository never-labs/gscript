package vm

import (
	"os"
	"testing"

	"github.com/Never-Labs/gscript/internal/runtime"
)

func TestStdlibHostDriverRuntimeSpecialization(t *testing.T) {
	src, err := os.ReadFile("../../benchmarks/official_hot/stdlib_host_hot.gs")
	if err != nil {
		t.Fatal(err)
	}
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	top := compileProto(t, string(src))
	runHot := findTestProtoByName(top, "run_hot")
	if runHot == nil {
		t.Fatal("missing run_hot proto")
	}
	if !isStdlibHostDriverProto(runHot) {
		t.Fatalf("stdlib_host_driver did not recognize run_hot: code=%d const=%d diagnostics=%+v", len(runHot.Code), len(runHot.Constants), DiagnoseCallSiteRuntimeSpecializationProto(runHot))
	}

	globals := compileAndRun(t, string(src))
	expectGlobalInt(t, globals, "checksum", 913531730)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "stdlib_host_driver"); got == 0 {
		t.Fatalf("stdlib_host_driver hit count = %d, want > 0", got)
	}
}

func TestStdlibHostDriverRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func run_hot(n) {
    return n
}
`)
	child := findTestProtoByName(top, "run_hot")
	if child == nil {
		t.Fatal("missing run_hot proto")
	}
	if isStdlibHostDriverProto(child) {
		t.Fatal("stdlib_host_driver should reject name-only matches")
	}
}
