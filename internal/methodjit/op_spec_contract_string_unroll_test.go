package methodjit

import "testing"

func TestStringUnrollContractsLiveInOpSpec(t *testing.T) {
	if lowered, ok := stringEnumCompareLoweredOp(OpEqString); !ok || lowered != OpEqInt {
		t.Fatalf("EqString enum lowered op = %s, %v; want EqInt, true", lowered, ok)
	}
	if lowered, ok := stringEnumCompareLoweredOp(OpEqInt); ok || lowered != OpMax {
		t.Fatalf("EqInt enum lowered op = %s, %v; want OpMax, false", lowered, ok)
	}
	for _, op := range []Op{OpConstString, OpStringConstLookup, OpStringFormatInt, OpStringFormatConst, OpStringFormatConstLen, OpStringSplitPart, OpStringSplitSubstr, OpStringSplitSubstrNumber, OpGuardConstString} {
		spec, ok := op.Spec()
		if !ok || !spec.ConstPoolUser || !instrUsesConstPool(&Instr{Op: op}) {
			t.Fatalf("%s const-pool contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstString, OpStringConstLookup, OpStringFormatInt, OpStringFormatConst, OpStringFormatConstLen, OpStringSplitPart, OpStringSplitSubstr, OpGuardConstString, OpGuardCalleeProto} {
		spec, ok := op.Spec()
		if !ok || !spec.RawStringResult {
			t.Fatalf("%s raw-string result contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstString, OpStringConstLookup, OpStringFormatInt} {
		spec, ok := op.Spec()
		if !ok || !spec.DynamicStringQueryCacheKey {
			t.Fatalf("%s dynamic string-query cache key contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpAddInt, OpDivFloat, OpSqrt, OpNumToFloat, OpGuardType, OpMatrixLoadFAt, OpTableArrayLoad} {
		spec, ok := op.Spec()
		if !ok || !spec.UnrollCloneable || !isUnrollCloneableOp(op) {
			t.Fatalf("%s unroll cloneability contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpLoadSlot, OpPhi, OpAddFloat, OpSqrt, OpGuardType, OpBranch, OpReturn} {
		spec, ok := op.Spec()
		if !ok || !spec.NestedFloatPhiOverrideSafe {
			t.Fatalf("%s nested-float-phi override safety should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpDivFloat, OpFMA, OpFMSUB, OpSqrt, OpFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.FloatReductionWideUnrollBarrier {
			t.Fatalf("%s float-reduction wide-unroll barrier should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpSqrt.Spec(); !ok || !spec.FloatReductionLatencyUnrollSeed {
		t.Fatalf("Sqrt latency-unroll seed should be driven by OpSpec")
	}
	for _, op := range []Op{OpDivFloat, OpFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.FloatReductionLatencyUnrollBlock {
			t.Fatalf("%s latency-unroll block should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpDivFloat.Spec(); !ok || !spec.FloatReductionDivOp {
		t.Fatalf("DivFloat float-reduction div-op contract should be driven by OpSpec")
	}
	for _, op := range []Op{OpPhi, OpConstInt, OpConstBool, OpEqInt, OpNot} {
		spec, ok := op.Spec()
		if !ok || !spec.ConstantPhiBranchThreadPure {
			t.Fatalf("%s constant-phi branch threading purity should be driven by OpSpec", op)
		}
	}
}
