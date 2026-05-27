package methodjit

import "testing"

func TestRangeAndBackendContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpConstInt, OpLen, OpTableArrayLen, OpGuardIntRange, OpAddInt, OpMulInt, OpModInt, OpDivIntExact, OpPhi, OpBoxInt, OpUnboxInt} {
		spec, ok := op.Spec()
		if !ok || !spec.NonNegativeDerivationCandidate || spec.NonNegativeDerivationKind == OpNonNegativeNone || !opCanDeriveNonNegative(&Instr{Op: op}) {
			t.Fatalf("%s non-negative derivation contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpGuardType, OpGuardIntRange, OpLoadSlot, OpUnboxInt} {
		spec, ok := op.Spec()
		if !ok || !spec.Int48RuntimeValue || !isInt48RuntimeValue(&Instr{Op: op, Type: TypeInt}) {
			t.Fatalf("%s int48 runtime-value contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpEq, OpLtInt, OpLeInt, OpEqInt, OpModZeroInt, OpLtFloat, OpLeFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.FusableComparison || !isFusableComparison(op) {
			t.Fatalf("%s fusable comparison contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpLtInt, OpLeInt, OpEqInt} {
		spec, ok := op.Spec()
		if !ok || !spec.LoopBoundComparison {
			t.Fatalf("%s loop-bound comparison contract should be driven by OpSpec", op)
		}
	}
}
