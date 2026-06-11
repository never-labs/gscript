//go:build darwin && arm64

package benchmarks

import "testing"

// TestQEvalSessionWarmRepeatConsistency evaluates every qEvalVectorCases
// expression five times on one warm session and requires identical results
// on every run. This pins the q.eval warm-execution contract that buffer
// reuse mechanisms (assignment statement-slot pooling, bulk pools) recompute
// values per call and never let a later eval corrupt or memoize an earlier
// result.
// qEvalSessionRepeatStatefulCases lists expressions whose results observe
// session state that legitimately changes between evals (never add cases to
// silence a buffer-reuse corruption; those are bugs).
var qEvalSessionRepeatStatefulCases = map[string]string{
	"SafeSystemAndLoopbackIPC": "counts \\v workspace variables, which grow once h and v are defined by the first eval",
}

func TestQEvalSessionWarmRepeatConsistency(t *testing.T) {
	for _, tc := range qEvalVectorCases {
		if _, stateful := qEvalSessionRepeatStatefulCases[tc.name]; stateful {
			continue
		}
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			eval := qSessionEvalVectorEval(t)
			src := tc.expr(qEvalVectorRows)
			first := qEvalVectorRun(t, eval, src)
			for i := 0; i < 4; i++ {
				if out := qEvalVectorRun(t, eval, src); out != first {
					t.Fatalf("warm repeat %d: result %d != first %d", i+1, out, first)
				}
			}
		})
	}
}
