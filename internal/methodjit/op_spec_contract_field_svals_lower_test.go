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

func TestFieldNumFusionLoweringTargetsLiveInOpSpec(t *testing.T) {
	if lowered, ok := fieldNumFusionLoweredOp(OpGetField); !ok || lowered != OpGetFieldNumToFloat {
		t.Fatalf("GetField numeric-fusion lowered op = %s, %v; want GetFieldNumToFloat, true", lowered, ok)
	}
	if lowered, ok := fieldNumFusionLoweredOp(OpNumToFloat); ok || lowered != OpMax {
		t.Fatalf("NumToFloat numeric-fusion lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}

func TestFieldLenLoweringTargetsLiveInOpSpec(t *testing.T) {
	if lowered, ok := fieldLenLoweredOp(OpLen); !ok || lowered != OpFieldPolyLen {
		t.Fatalf("Len field-len lowered op = %s, %v; want FieldPolyLen, true", lowered, ok)
	}
	if lowered, ok := fieldLenLoweredOp(OpFieldPolyLen); ok || lowered != OpMax {
		t.Fatalf("FieldPolyLen field-len lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}
