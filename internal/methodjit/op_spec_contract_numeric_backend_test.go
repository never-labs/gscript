package methodjit

import "testing"

func TestRangeRefineAndNumericBackendContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpMod, OpLt, OpLe} {
		spec, ok := op.Spec()
		if !ok || !spec.SpeculativeIntUseCandidate {
			t.Fatalf("%s speculative int-use contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpLt, OpLtInt} {
		spec, ok := op.Spec()
		if !ok || spec.RangeRefineKind != OpRangeRefineLessThan {
			t.Fatalf("%s range-refine less-than contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpLe, OpLeInt} {
		spec, ok := op.Spec()
		if !ok || spec.RangeRefineKind != OpRangeRefineLessEqual {
			t.Fatalf("%s range-refine less-equal contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpEqInt.Spec(); !ok || spec.RangeRefineKind != OpRangeRefineEqualInt {
		t.Fatalf("EqInt range-refine equality contract should be driven by OpSpec")
	}
	for _, tc := range []struct {
		raw      Op
		fallback Op
		unknown  bool
	}{
		{raw: OpAddInt, fallback: OpAdd, unknown: true},
		{raw: OpSubInt, fallback: OpSub, unknown: true},
		{raw: OpMulInt, fallback: OpMul, unknown: true},
		{raw: OpModInt, fallback: OpMod, unknown: true},
		{raw: OpDivIntExact, fallback: OpDiv, unknown: true},
		{raw: OpNegInt, fallback: OpUnm, unknown: true},
		{raw: OpEqInt, fallback: OpEq},
		{raw: OpLtInt, fallback: OpLt},
		{raw: OpLeInt, fallback: OpLe},
	} {
		spec, ok := tc.raw.Spec()
		if !ok || spec.BoxedFallbackOp != tc.fallback || spec.BoxedFallbackResultUnknown != tc.unknown {
			t.Fatalf("%s boxed fallback contract should be driven by OpSpec", tc.raw)
		}
	}
	for _, op := range []Op{OpAddInt, OpSubInt, OpMulInt, OpNegInt, OpDivIntExact} {
		spec, ok := op.Spec()
		if !ok || !spec.Int48SafeRangeCandidate {
			t.Fatalf("%s int48-safe range candidate contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstFloat, OpAddFloat, OpSubFloat, OpMulFloat, OpDivFloat, OpNegFloat, OpUnboxFloat, OpBoxFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.FloatRegResult || !needsFloatReg(&Instr{Op: op}) {
			t.Fatalf("%s float-register result contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpLtFloat, OpLeFloat, OpComplexEscapeInSet} {
		spec, ok := op.Spec()
		if !ok || !spec.FloatRegResultBlocked || needsFloatReg(&Instr{Op: op, Type: TypeFloat}) {
			t.Fatalf("%s float-register block contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpLoadSlot, OpGuardType, OpGuardIntRange, OpCall, OpCallFloor, OpFieldCallFloor, OpPhi, OpTableArrayHeader, OpTableArrayLen, OpTableArrayData} {
		spec, ok := op.Spec()
		if !ok || !spec.RawIntCarryValue || !isRawIntCarryValue(&Instr{Op: op, Type: TypeInt}) {
			t.Fatalf("%s raw-int carry contract should be driven by OpSpec", op)
		}
	}
}
