package vm

import (
	"os"
	"testing"

	"github.com/Never-Labs/gscript/internal/runtime"
)

func TestRegexpRandomDriverRuntimeSpecialization(t *testing.T) {
	src, err := os.ReadFile("../../benchmarks/official_hot/regexp_random_hot.gs")
	if err != nil {
		t.Fatal(err)
	}
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, string(src))
	expectGlobalInt(t, globals, "checksum", 237183520)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "regexp_random_driver"); got == 0 {
		t.Fatalf("regexp_random_driver hit count = %d, want > 0", got)
	}
}

func TestRegexpRandomDriverRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func run(n) {
    return n
}
`)
	child := findTestProtoByName(top, "run")
	if child == nil {
		t.Fatal("missing run proto")
	}
	if isRegexpRandomDriverProto(child) {
		t.Fatal("regexp_random_driver should reject name-only matches")
	}
}
