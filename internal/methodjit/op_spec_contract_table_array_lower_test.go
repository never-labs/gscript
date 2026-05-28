package methodjit

import "testing"

func TestTableArrayLoweringTargetsLiveInOpSpec(t *testing.T) {
	for _, tc := range []struct {
		op      Op
		lowered Op
	}{
		{OpGetTable, OpTableArrayLoad},
		{OpSetTable, OpTableArrayStore},
	} {
		if got, ok := tableArrayLoweredOp(tc.op); !ok || got != tc.lowered {
			t.Fatalf("%s table-array lowered op = %s, %v; want %s, true", tc.op, got, ok, tc.lowered)
		}
	}
	if lowered, ok := tableArrayLoweredOp(OpTableArrayLoad); ok || lowered != OpMax {
		t.Fatalf("TableArrayLoad table-array lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}

func TestTableArrayNestedLoweringTargetsLiveInOpSpec(t *testing.T) {
	if lowered, ok := tableArrayNestedLoweredOp(OpTableArrayLoad); !ok || lowered != OpTableArrayNestedLoad {
		t.Fatalf("TableArrayLoad nested lowered op = %s, %v; want TableArrayNestedLoad, true", lowered, ok)
	}
	if lowered, ok := tableArrayNestedLoweredOp(OpGetTable); ok || lowered != OpMax {
		t.Fatalf("GetTable nested lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
}
