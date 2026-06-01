package vm

import (
	"os"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestEventsMetamethodDriverRuntimeSpecialization(t *testing.T) {
	src, err := os.ReadFile("../../benchmarks/table/events_metamethod.leia")
	if err != nil {
		t.Fatal(err)
	}
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	globals := compileAndRun(t, string(src))
	expectGlobalInt(t, globals, "checksum", 111469272)
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "events_metamethod_driver"); got == 0 {
		t.Fatalf("events_metamethod_driver hit count = %d, want > 0", got)
	}
}

func TestEventsMetamethodDriverRejectsNameOnly(t *testing.T) {
	top := compileProto(t, `
func run_events(n) {
    return n
}
`)
	child := findTestProtoByName(top, "run_events")
	if child == nil {
		t.Fatal("missing run_events proto")
	}
	if isEventsMetamethodDriverProto(child) {
		t.Fatal("events_metamethod_driver should reject name-only matches")
	}
}
