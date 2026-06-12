package q

import "testing"

// TestSumRazeNestedBroadcastGuard pins the R15-R sum-raze nested-broadcast
// guard semantics: when one raze level still leaves nested arrays behind,
// sum broadcasts on the generic route instead of reducing all levels.
func TestSumRazeNestedBroadcastGuard(t *testing.T) {
	broadcast := []struct {
		expr string
		want []int64
	}{
		{expr: "sum raze ((0;0 0);0)", want: []int64{0, 0}},
		{expr: "sum raze (0;0 0)0", want: []int64{0, 0}},
		{expr: "sum raze ((1;2 3);4)", want: []int64{7, 8}},
	}
	for _, tc := range broadcast {
		assertEvalI64Vector(t, tc.expr, tc.want)
	}
}

// TestSumRazeMatrixViewFusedScalar pins the fused scalar reduction for
// rectangular matrix views (flip/reshape results): one raze level fully
// flattens them, so the typed kernel keeps the claim and the eligibility
// guard must answer in O(1) without walking the rows.
func TestSumRazeMatrixViewFusedScalar(t *testing.T) {
	assertEvalValue(t, "+/raze 2 3#til 6", int64(15))
	assertEvalValue(t, "m:2 3#til 6;t:flip m;(+/raze t)+count t", int64(18))
	assertEvalValue(t, "m:2 3#1 2 3 4;s:+/raze flip m;s+m . 1 1", int64(14))
	assertEvalValue(t, "+/raze flip flip 2 3#til 6", int64(15))
	assertEvalValue(t, "+/raze ((1 2;(3;4 5));6)", int64(21))
}
