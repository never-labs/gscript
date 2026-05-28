package methodjit

import "testing"

func TestLocalStringArrayTableContractsLiveInOpSpec(t *testing.T) {
	for _, tc := range []struct {
		op   Op
		role OpLocalStringArrayTableUseRole
	}{
		{OpSetTable, OpLocalStringArrayTableUseStore},
		{OpLen, OpLocalStringArrayTableUseRead},
		{OpTableArrayHeader, OpLocalStringArrayTableUseRead},
	} {
		spec, ok := tc.op.Spec()
		arg, argOK := localStringArrayTableArgIndex(tc.op)
		if !ok || spec.LocalStringArrayTableUseRole != tc.role || localStringArrayTableUseRole(tc.op) != tc.role ||
			!argOK || arg != 0 {
			t.Fatalf("%s local string-array table contract should be driven by OpSpec", tc.op)
		}
		instr := &Instr{Op: tc.op, Args: []*Value{{ID: 42}}}
		if !localStringArrayTableUseOK(instr, 0, 42) {
			t.Fatalf("%s local string-array table use should be accepted through OpSpec", tc.op)
		}
	}
	if localStringArrayTableUseRole(OpGetTable) != OpLocalStringArrayTableUseNone {
		t.Fatalf("%s should not report local string-array table role", OpGetTable)
	}
}
