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

func TestMatrixRowLoweringTargetsLiveInOpSpec(t *testing.T) {
	cases := []struct {
		op       Op
		row      Op
		rowConst Op
	}{
		{OpMatrixLoadFAt, OpMatrixLoadFRow, OpMatrixLoadFRowConst},
		{OpMatrixStoreFAt, OpMatrixStoreFRow, OpMatrixStoreFRowConst},
	}
	for _, tc := range cases {
		got, ok := matrixRowLoweredOp(tc.op)
		if !ok || got != tc.row {
			t.Fatalf("%s matrix row lowered op = %s, %v; want %s, true", tc.op, got, ok, tc.row)
		}
		gotConst, ok := matrixRowConstLoweredOp(tc.op)
		if !ok || gotConst != tc.rowConst {
			t.Fatalf("%s matrix row const lowered op = %s, %v; want %s, true", tc.op, gotConst, ok, tc.rowConst)
		}
	}
	if lowered, ok := matrixRowLoweredOp(OpMatrixGetF); ok || lowered != OpMax {
		t.Fatalf("MatrixGetF matrix row lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
	if lowered, ok := matrixRowConstLoweredOp(OpMatrixGetF); ok || lowered != OpMax {
		t.Fatalf("MatrixGetF matrix row const lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}

func TestMatrixNestedLoweringTargetsLiveInOpSpec(t *testing.T) {
	if lowered, ok := matrixNestedLoweredOp(OpTableArrayNestedLoad); !ok || lowered != OpMatrixLoadFAt {
		t.Fatalf("TableArrayNestedLoad matrix nested lowered op = %s, %v; want MatrixLoadFAt, true", lowered, ok)
	}
	if lowered, ok := matrixNestedLoweredOp(OpMatrixLoadFAt); ok || lowered != OpMax {
		t.Fatalf("MatrixLoadFAt matrix nested lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}
