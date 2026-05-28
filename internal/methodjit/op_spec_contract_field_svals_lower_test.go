package methodjit

import "testing"

func TestFieldSvalsLoweringTargetsLiveInOpSpec(t *testing.T) {
	cases := []struct {
		op      Op
		lowered Op
	}{
		{OpGetField, OpFieldLoad},
		{OpGetFieldNumToFloat, OpFieldLoadNumToFloat},
		{OpSetField, OpFieldStore},
	}
	for _, tc := range cases {
		got, ok := fieldSvalsLoweredOp(tc.op)
		if !ok || got != tc.lowered {
			t.Fatalf("%s FieldSvals lowered op = %s, %v; want %s, true", tc.op, got, ok, tc.lowered)
		}
	}
	if lowered, ok := fieldSvalsLoweredOp(OpAddInt); ok || lowered != OpMax {
		t.Fatalf("AddInt FieldSvals lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}
