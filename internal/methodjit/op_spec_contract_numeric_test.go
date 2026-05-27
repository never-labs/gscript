package methodjit

import "testing"

func TestIntArithmeticContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpAddInt, OpSubInt, OpMulInt, OpModInt, OpDivIntExact, OpNegInt} {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.BoxableIntArithmetic || !isBoxableIntArithmetic(&Instr{Op: op}) {
			t.Fatalf("%s boxable int arithmetic contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAddInt, OpSubInt, OpMulInt, OpNegInt, OpDivIntExact} {
		spec, ok := op.Spec()
		if !ok || !spec.UnsafeIntArithmeticCandidate {
			t.Fatalf("%s unsafe int arithmetic candidate should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpEq, OpLt, OpLe, OpEqInt, OpLtInt, OpLeInt, OpGuardType, OpGuardIntRange, OpBranch} {
		spec, ok := op.Spec()
		if !ok || !spec.ExactDivAllowedExternalUse || !isExactDivAllowedExternalUse(op) {
			t.Fatalf("%s exact-div external-use contract should be driven by OpSpec", op)
		}
	}
}
