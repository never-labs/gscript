package methodjit

import "testing"

func TestMatrixLoweringTargetsLiveInOpSpec(t *testing.T) {
	cases := []struct {
		op      Op
		lowered Op
	}{
		{OpMatrixGetF, OpMatrixLoadFAt},
		{OpMatrixSetF, OpMatrixStoreFAt},
	}
	for _, tc := range cases {
		got, ok := matrixLoweredOp(tc.op)
		if !ok || got != tc.lowered {
			t.Fatalf("%s matrix lowered op = %s, %v; want %s, true", tc.op, got, ok, tc.lowered)
		}
	}
	if lowered, ok := matrixLoweredOp(OpMatrixLoadFAt); ok || lowered != OpMax {
		t.Fatalf("MatrixLoadFAt matrix lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}
