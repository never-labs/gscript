package runtime

import "testing"

func TestCollectGarbageStats(t *testing.T) {
	interp := New()

	execBinaryIOTest(t, interp, `
		before := collectgarbage("stats")
		collectgarbage("collect")
		after := collectgarbage("stats")
		allocKB := after.allocKB
		mode := after.mode
		running := after.running
		rootLog := after.rootLog
		hasNumGC := after.numGC >= before.numGC
	`)

	if got := interp.GetGlobal("allocKB"); !got.IsNumber() || got.Number() < 0 {
		t.Fatalf("allocKB = %v, want non-negative number", got)
	}
	if got := interp.GetGlobal("mode").Str(); got == "" {
		t.Fatalf("mode should be non-empty")
	}
	if got := interp.GetGlobal("running").Bool(); !got {
		t.Fatalf("running = false, want true")
	}
	if got := interp.GetGlobal("rootLog"); !got.IsInt() || got.Int() < 0 {
		t.Fatalf("rootLog = %v, want non-negative int", got)
	}
	if got := interp.GetGlobal("hasNumGC").Bool(); !got {
		t.Fatalf("numGC should not decrease")
	}
}
