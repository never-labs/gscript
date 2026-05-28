package methodjit

import "testing"

func TestFieldSvalsFirstArgMutationContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpTableArrayStore, OpTableArraySwap, OpTableArraySwapPairs, OpTableBoolArrayFill, OpTableIntArrayReversePrefix, OpTableIntArrayCopyPrefix} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldSvalsFirstArgMutationBarrier {
			t.Fatalf("%s FieldSvals first-arg mutation barrier should be driven by OpSpec", op)
		}
		if fieldSvalsGlobalBarrier(&Instr{Op: op, Args: []*Value{{ID: 1}}}) {
			t.Fatalf("%s with table arg should be table-specific, not a global FieldSvals barrier", op)
		}
		if !fieldSvalsGlobalBarrier(&Instr{Op: op}) {
			t.Fatalf("%s without table arg should be a conservative global FieldSvals barrier", op)
		}
	}
}
