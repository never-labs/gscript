package methodjit

import "testing"

func TestReadonlyTableParamUseContractsLiveInOpSpec(t *testing.T) {
	for _, tc := range []struct {
		op   Op
		role OpReadonlyTableParamUseRole
	}{
		{OpGetTable, OpReadonlyTableParamUseBenign},
		{OpLen, OpReadonlyTableParamUseBenign},
		{OpReturn, OpReadonlyTableParamUseBenign},
		{OpSetTable, OpReadonlyTableParamUseFirstArgMutation},
		{OpSetField, OpReadonlyTableParamUseFirstArgMutation},
		{OpFieldStore, OpReadonlyTableParamUseFirstArgMutation},
		{OpSetList, OpReadonlyTableParamUseFirstArgMutation},
		{OpAppend, OpReadonlyTableParamUseFirstArgMutation},
		{OpTableArrayStore, OpReadonlyTableParamUseFirstArgMutation},
		{OpTableArraySwap, OpReadonlyTableParamUseFirstArgMutation},
		{OpTableArraySwapPairs, OpReadonlyTableParamUseFirstArgMutation},
		{OpTableBoolArrayFill, OpReadonlyTableParamUseFirstArgMutation},
		{OpTableIntArrayReversePrefix, OpReadonlyTableParamUseFirstArgMutation},
		{OpTableIntArrayCopyPrefix, OpReadonlyTableParamUseFirstArgMutation},
		{OpCall, OpReadonlyTableParamUseCallEscape},
		{OpCallFloor, OpReadonlyTableParamUseCallEscape},
		{OpFieldCallFloor, OpReadonlyTableParamUseCallEscape},
		{OpSelf, OpReadonlyTableParamUseCallEscape},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.ReadonlyTableParamUseRole != tc.role || readonlyTableParamUseRole(tc.op) != tc.role {
			t.Fatalf("%s readonly table-param role should be driven by OpSpec", tc.op)
		}
	}
	if readonlyTableParamUseRole(OpAddInt) != OpReadonlyTableParamUseNone {
		t.Fatalf("%s should not report readonly table-param role", OpAddInt)
	}
}
