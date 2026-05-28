package methodjit

import "testing"

func TestInlineAllocationContractsLiveInOpSpec(t *testing.T) {
	for _, tc := range []struct {
		op   Op
		role OpInlineAllocationRole
	}{
		{OpNewTable, OpInlineAllocationDynamic},
		{OpNewFixedTable, OpInlineAllocationFixed},
		{OpSetField, OpInlineAllocationFieldInit},
		{OpSetList, OpInlineAllocationArrayInit},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.InlineAllocationRole != tc.role || inlineAllocationRole(tc.op) != tc.role {
			t.Fatalf("%s inline allocation role should be driven by OpSpec", tc.op)
		}
	}
	if inlineAllocationRole(OpSetTable) != OpInlineAllocationNone {
		t.Fatalf("%s should not report inline allocation role", OpSetTable)
	}
}
