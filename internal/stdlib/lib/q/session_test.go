package q

import "testing"

// TestEvalSessionPlannedMatchesEval pins the planned-handle contract: a
// pinned handle executes the same cached plan chain as EvalSession.Eval for
// the pinned source, binds session state at execution time (not at plan
// time), and pins the same cache entry Eval resolves per call.
func TestEvalSessionPlannedMatchesEval(t *testing.T) {
	session := NewEvalSession(nil)
	if _, err := session.Eval("x:41"); err != nil {
		t.Fatalf("EvalSession.Eval(x:41): %v", err)
	}

	planned := session.Planned("x+1")
	if planned == nil {
		t.Fatal("EvalSession.Planned returned nil handle")
	}
	got, err := planned.Eval()
	if err != nil || got != int64(42) {
		t.Fatalf("planned.Eval = %#v,%v; want 42,nil", got, err)
	}
	want, err := session.Eval("x+1")
	if err != nil || want != got {
		t.Fatalf("EvalSession.Eval(x+1) = %#v,%v; want planned result %#v", want, err, got)
	}

	// State binds at execution time: rebinding x must be visible through the
	// already-pinned handle, exactly as a fresh Eval would see it.
	if _, err := session.Eval("x:100"); err != nil {
		t.Fatalf("EvalSession.Eval(x:100): %v", err)
	}
	got, err = planned.Eval()
	if err != nil || got != int64(101) {
		t.Fatalf("planned.Eval after rebind = %#v,%v; want 101,nil", got, err)
	}

	// The handle pins the same cache entry Eval resolves per call.
	if entry := session.cache["x+1"]; entry == nil || entry != planned.entry {
		t.Fatalf("planned handle entry %p != session cache entry %p", planned.entry, session.cache["x+1"])
	}

	// Executable-pipeline sources go through the same chain.
	plannedPipeline := session.Planned("x:til 8192;idx:where x>=0;+/idx")
	if plannedPipeline == nil {
		t.Fatal("EvalSession.Planned(script pipeline) returned nil handle")
	}
	for i := 0; i < 2; i++ {
		got, err = plannedPipeline.Eval()
		if err != nil || got != int64(33550336) {
			t.Fatalf("planned pipeline Eval #%d = %#v,%v; want 33550336,nil", i, got, err)
		}
	}

	// Nil-safety mirrors the rest of the session API.
	if (*EvalSession)(nil).Planned("1+2") != nil {
		t.Fatal("nil session Planned must return nil")
	}
	if _, err := (*EvalSessionPlanned)(nil).Eval(); err == nil {
		t.Fatal("nil planned handle Eval must error")
	}
}
