package methodjit

import "testing"

func TestFixedShapeArrayElementContractsLiveInOpSpec(t *testing.T) {
	for _, tc := range []struct {
		op   Op
		role OpFixedShapeArrayElementWriteRole
	}{
		{OpSetTable, OpFixedShapeArrayElementWriteSingle},
		{OpSetList, OpFixedShapeArrayElementWriteVariadic},
		{OpAppend, OpFixedShapeArrayElementWriteConflict},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.FixedShapeArrayElementWriteRole != tc.role || fixedShapeArrayElementWriteRole(tc.op) != tc.role {
			t.Fatalf("%s fixed-shape array-element write role should be driven by OpSpec", tc.op)
		}
	}
	for _, tc := range []struct {
		op   Op
		role OpFixedShapeArrayElementReadRole
	}{
		{OpGetTable, OpFixedShapeArrayElementReadDirect},
		{OpTableArrayLoad, OpFixedShapeArrayElementReadLoweredArray},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.FixedShapeArrayElementReadRole != tc.role || fixedShapeArrayElementReadRole(tc.op) != tc.role {
			t.Fatalf("%s fixed-shape array-element read role should be driven by OpSpec", tc.op)
		}
	}
	if fixedShapeArrayElementWriteRole(OpSetField) != OpFixedShapeArrayElementWriteNone {
		t.Fatalf("%s should not report fixed-shape array-element write role", OpSetField)
	}
	if fixedShapeArrayElementReadRole(OpLen) != OpFixedShapeArrayElementReadNone {
		t.Fatalf("%s should not report fixed-shape array-element read role", OpLen)
	}
}
