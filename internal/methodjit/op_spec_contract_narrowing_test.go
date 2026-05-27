package methodjit

import "testing"

func TestNarrowingContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpPhi, OpAdd, OpMod, OpAddInt, OpAddFloat, OpMulFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.ExactDivComponent || !isExactDivComponentOp(&Instr{Op: op}, nil) {
			t.Fatalf("%s exact-div component contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpPhi, OpAdd, OpNegInt, OpAddFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.IntNarrowCandidate {
			t.Fatalf("%s int narrowing candidate contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpPhi, OpAdd, OpAddInt, OpAddFloat, OpNegInt} {
		spec, ok := op.Spec()
		if !ok || !spec.IntNarrowAllArgsConstraint {
			t.Fatalf("%s int narrowing constraint contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpNop, OpConstInt, OpLoadSlot, OpAddFloat, OpSqrt, OpLtFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldNumFusionGapSafe || !fieldNumFusionGapIsSafe([]*Instr{{Op: op}}) {
			t.Fatalf("%s field-num fusion gap contract should be driven by OpSpec", op)
		}
	}
	for _, tc := range []struct {
		generic Op
		raw     Op
	}{
		{OpAdd, OpAddInt},
		{OpSub, OpSubInt},
		{OpMul, OpMulInt},
		{OpMod, OpModInt},
		{OpEq, OpEqInt},
		{OpLt, OpLtInt},
		{OpLe, OpLeInt},
	} {
		spec, ok := tc.generic.Spec()
		if !ok || spec.RawIntSpecializedOp != tc.raw {
			t.Fatalf("%s raw-int specialization target should be %s through OpSpec", tc.generic, tc.raw)
		}
	}
	for _, tc := range []struct {
		op       Op
		narrowed Op
	}{
		{OpAdd, OpAddInt},
		{OpAddFloat, OpAddInt},
		{OpSub, OpSubInt},
		{OpSubFloat, OpSubInt},
		{OpMul, OpMulInt},
		{OpMulFloat, OpMulInt},
		{OpMod, OpModInt},
		{OpDiv, OpDivIntExact},
		{OpDivFloat, OpDivIntExact},
		{OpEq, OpEqInt},
		{OpLt, OpLtInt},
		{OpLe, OpLeInt},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.ExactIntNarrowOp != tc.narrowed {
			t.Fatalf("%s exact-int narrowing target should be %s through OpSpec", tc.op, tc.narrowed)
		}
	}
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpDiv, OpMod, OpUnm} {
		spec, ok := op.Spec()
		if !ok || !spec.RawIntSpecializationBlocker {
			t.Fatalf("%s raw-int residual blocker should be driven by OpSpec", op)
		}
	}
}
