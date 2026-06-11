package q

import (
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

const assignPoolFillsScript = "x:100#0N 3 0N 7 5;y:0^fills x;+/8 msum y"

// TestAssignPoolWarmRepeatStable pins that pooled materialization recomputes
// (never memoizes) values: a warm-repeated fills script returns the identical
// scalar on every run, including the runs where the pool is active.
func TestAssignPoolWarmRepeatStable(t *testing.T) {
	s := NewEvalSession(nil)
	first, err := s.Eval(assignPoolFillsScript)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	for i := 0; i < 10; i++ {
		out, err := s.Eval(assignPoolFillsScript)
		if err != nil {
			t.Fatalf("eval %d: %v", i, err)
		}
		if out != first {
			t.Fatalf("eval %d: result %v != first %v", i, out, first)
		}
	}
	if s.state.assignPool.slots["y"] == nil {
		t.Fatalf("expected the warm fills script to pool the y assignment; pooling never activated")
	}
}

// TestAssignPoolCrossScriptAliasSafe pins the orphan-on-source-switch rule:
// a second script that aliases a pooled assignment must keep observing its
// original contents even after the producing script runs again (the producing
// script must not recycle buffers once another source ran in between).
func TestAssignPoolCrossScriptAliasSafe(t *testing.T) {
	s := NewEvalSession(nil)
	// Warm the producing script until pooling is active.
	for i := 0; i < 4; i++ {
		if _, err := s.Eval(assignPoolFillsScript); err != nil {
			t.Fatalf("warm eval %d: %v", i, err)
		}
	}
	// Alias the pooled column under another name (different source).
	if _, err := s.Eval("z:y"); err != nil {
		t.Fatalf("alias eval: %v", err)
	}
	sumBefore, err := s.Eval("+/z")
	if err != nil {
		t.Fatalf("sum before: %v", err)
	}
	// Re-run the producing script; z must keep its original contents.
	for i := 0; i < 6; i++ {
		if _, err := s.Eval(assignPoolFillsScript); err != nil {
			t.Fatalf("rerun eval %d: %v", i, err)
		}
	}
	sumAfter, err := s.Eval("+/z")
	if err != nil {
		t.Fatalf("sum after: %v", err)
	}
	if sumBefore != sumAfter {
		t.Fatalf("alias corrupted: +/z before=%v after=%v", sumBefore, sumAfter)
	}
}

// TestAssignPoolNonScalarResultSafe pins the non-scalar-result bar: a script
// whose result is the assigned vector never recycles, so arrays returned by
// earlier evals stay intact after later evals of the same source.
func TestAssignPoolNonScalarResultSafe(t *testing.T) {
	s := NewEvalSession(nil)
	src := "y:0^fills 100#0N 3 0N 7 5;y"
	out, err := s.Eval(src)
	if err != nil {
		t.Fatalf("eval: %v", err)
	}
	first, ok := out.(data.Array)
	if !ok {
		t.Fatalf("expected array result, got %T", out)
	}
	want := make([]any, first.Len())
	for i := 0; i < first.Len(); i++ {
		v, ok := first.At(i)
		if !ok {
			t.Fatalf("first.At(%d) not ok", i)
		}
		want[i] = v
	}
	for i := 0; i < 8; i++ {
		if _, err := s.Eval(src); err != nil {
			t.Fatalf("re-eval %d: %v", i, err)
		}
	}
	for i := 0; i < first.Len(); i++ {
		v, ok := first.At(i)
		if !ok || v != want[i] {
			t.Fatalf("returned array mutated at row %d: got %v want %v", i, v, want[i])
		}
	}
}
