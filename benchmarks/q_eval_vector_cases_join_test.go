//go:build leia_q

package benchmarks

// qEvalVectorJoinCases covers the canonical `,` join verb family: dense
// typed vector,vector concatenation (the no-box kernel), atom append/prepend
// chains, the `,/` raze-equivalent fold, and `,'` each-both pairwise joins.
// Required by the verb coverage gate once `,` joined the lookupDyadicVerb
// dispatch table.

import (
	"fmt"
	"testing"
)

func qEvalVectorJoinCases() []qEvalVectorCase {
	return []qEvalVectorCase{
		{
			name:   "JoinTypedVectorConcatSum",
			tags:   []string{"numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"join:typed-concat:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:x,x+1;+/y", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorJoinGoConcatSum(rows)
			},
		},
		{
			name:   "JoinAtomAppendPrependCountSum",
			tags:   []string{"numeric-vector", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"join:atom-append-prepend:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;y:0,x,100;(+/y)+count y", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorJoinGoAppendPrependCountSum(rows)
			},
		},
		{
			// One adverb-form case: `,/` (raze-equivalent fold) stays on the
			// string evaluator like every adverb, so this family keeps a
			// single warm string-path statement (`,'`/`,\:` coverage lives in
			// the canonical harness and the dual-route fuzzer).
			name:   "JoinRazeOverNestedSum",
			tags:   []string{"numeric-vector", "raze", "adverb-over-scan", "sum"},
			matrix: []string{"numeric-arithmetic:int-vector:hot"},
			shapes: []string{"join:over-raze:row-scaled"},
			expr: func(rows int) string {
				return fmt.Sprintf("x:til %d;m:(x;x*2;x*3);+/,/m", rows)
			},
			goFn: func(rows int) int64 {
				return qEvalVectorJoinGoRazeOverSum(rows)
			},
		},
	}
}

func qEvalVectorJoinGoConcatSum(rows int) int64 {
	var sum int64
	for i := int64(0); i < int64(rows); i++ {
		sum += i + (i + 1)
	}
	return sum
}

func qEvalVectorJoinGoAppendPrependCountSum(rows int) int64 {
	sum := int64(0) + int64(100)
	for i := int64(0); i < int64(rows); i++ {
		sum += i
	}
	return sum + int64(rows) + 2
}

func qEvalVectorJoinGoRazeOverSum(rows int) int64 {
	var sum int64
	for i := int64(0); i < int64(rows); i++ {
		sum += i + i*2 + i*3
	}
	return sum
}

// TestQEvalVectorJoinCaseValues is the oracle for this case family: every
// case's q.eval result must exactly equal its Go baseline at full and half
// row counts, and the Go baseline must be row-scaled.
func TestQEvalVectorJoinCaseValues(t *testing.T) {
	eval := qEvalVectorEval(t)
	seen := map[string]struct{}{}
	for _, tc := range qEvalVectorJoinCases() {
		tc := tc
		if _, dup := seen[tc.name]; dup {
			t.Fatalf("duplicate join case name %q", tc.name)
		}
		seen[tc.name] = struct{}{}
		t.Run(tc.name, func(t *testing.T) {
			for _, rows := range []int{qEvalVectorRows, qEvalVectorRows / 2} {
				src := tc.expr(rows)
				got := qEvalVectorRun(t, eval, src)
				want := tc.goFn(rows)
				if got != want {
					t.Fatalf("rows=%d q.eval(%q) = %d, want Go baseline %d", rows, src, got, want)
				}
			}
			if tc.goFn(qEvalVectorRows) == tc.goFn(qEvalVectorRows/2) {
				t.Fatalf("case %q is row-invariant: goFn(%d) == goFn(%d)", tc.name, qEvalVectorRows, qEvalVectorRows/2)
			}
		})
	}
}
