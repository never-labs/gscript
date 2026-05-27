package methodjit

import "testing"

func TestFieldShapeInlineContractsLiveInOpSpec(t *testing.T) {
	splitSafe := []Op{OpConstInt, OpAddInt, OpFieldLoad, OpGuardType, OpBranch, OpPhi, OpTableArrayLoad}
	for _, op := range splitSafe {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.FieldShapeSplitInlineSafe {
			t.Fatalf("%s should be field-shape split-inline-safe by OpSpec", op)
		}
		if !fieldShapeSplitInlineInstrSafe(&Instr{Op: op}) {
			t.Fatalf("%s split-inline safety query should be driven by OpSpec", op)
		}
	}

	if fieldShapeSplitInlineInstrSafe(&Instr{Op: OpGetField}) {
		t.Fatalf("GetField without Aux2 should keep its instr-specific split-inline guard")
	}
	if !fieldShapeSplitInlineInstrSafe(&Instr{Op: OpGetField, Aux2: 1}) {
		t.Fatalf("GetField with Aux2 should keep its instr-specific split-inline allowance")
	}

	preSafe := []Op{OpGetField, OpSetTable, OpAdd, OpLen}
	for _, op := range preSafe {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.FieldShapePreEffectInlineSafe || !fieldShapePreEffectInlineInstrSafe(&Instr{Op: op}) {
			t.Fatalf("%s pre-effect inline safety should be driven by OpSpec", op)
		}
	}

	sideEffects := []Op{OpFieldStore, OpTableArrayStore, OpSetField, OpSetTable}
	for _, op := range sideEffects {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.FieldShapeInlineSideEffect || !fieldShapeInlineInstrHasSideEffect(&Instr{Op: op}) {
			t.Fatalf("%s inline side-effect contract should be driven by OpSpec", op)
		}
	}

	postUnsafe := []Op{OpSetField, OpGetField, OpSetTable, OpCall}
	for _, op := range postUnsafe {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.FieldShapePostEffectInlineUnsafe || fieldShapePostEffectInlineInstrSafe(&Instr{Op: op, Aux2: 1}) {
			t.Fatalf("%s post-effect unsafe contract should be driven by OpSpec", op)
		}
	}
}

func TestGlobalConstAndNestedCallContractsLiveInOpSpec(t *testing.T) {
	globalUnsafe := []Op{OpCall, OpResume, OpYield, OpSelf, OpSetGlobal, OpSetUpval, OpGo, OpSend, OpRecv}
	for _, op := range globalUnsafe {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.GlobalConstUnsafe {
			t.Fatalf("%s should be global-const-unsafe by OpSpec", op)
		}
		fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{Op: op}}}}}
		if globalConstFunctionSafe(fn) {
			t.Fatalf("%s global const safety should be driven by OpSpec", op)
		}
	}

	nestedCallLike := []Op{OpCall, OpCallFloor, OpFieldCallFloor, OpResume, OpYield, OpTForCall, OpGo}
	for _, op := range nestedCallLike {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.NestedCallLike {
			t.Fatalf("%s should be nested-call-like by OpSpec", op)
		}
		fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{Op: op}}}}}
		if !irHasNestedCallLike(fn) {
			t.Fatalf("%s nested call query should be driven by OpSpec", op)
		}
	}

	fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{Op: OpAddInt}}}}}
	if !globalConstFunctionSafe(fn) || irHasNestedCallLike(fn) {
		t.Fatalf("pure op should be safe for global const and not nested-call-like")
	}
}

func TestLoadElimContractsLiveInOpSpec(t *testing.T) {
	constOps := []Op{OpConstInt, OpConstFloat, OpConstBool, OpConstNil, OpConstString}
	for _, op := range constOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.LoadElimConstCSE || !loadElimConstCSE(&Instr{Op: op}) {
			t.Fatalf("%s const CSE contract should be driven by OpSpec", op)
		}
		if !spec.LiteralConst || !opIsLiteralConst(op) {
			t.Fatalf("%s literal-const contract should be driven by OpSpec", op)
		}
	}

	pureOps := []Op{OpAddInt, OpDivIntExact, OpNumToFloat, OpSqrt, OpModZeroInt, OpTableShapeID}
	for _, op := range pureOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.LoadElimPureCSE || !loadElimPureCSE(&Instr{Op: op}) {
			t.Fatalf("%s pure CSE contract should be driven by OpSpec", op)
		}
	}

	killers := []Op{OpSetField, OpSetTable, OpTableArrayStore, OpAppend, OpCall, OpResume, OpSelf}
	for _, op := range killers {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.LoadElimShapeFactKiller {
			t.Fatalf("%s should kill load-elim shape facts by OpSpec", op)
		}
		facts := map[int]int{1: 2}
		out := transferShapeFactInstr(facts, &Instr{Op: op, ID: 3})
		if len(out) != 0 {
			t.Fatalf("%s should clear shape facts through OpSpec, got %v", op, out)
		}
	}
}

func TestTypeAndBarrierContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpDiv, OpMod, OpUnm, OpEq, OpLt, OpLe} {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.GenericSpecializable || !isGenericSpecializableOp(op) {
			t.Fatalf("%s generic specialization contract should be driven by OpSpec", op)
		}
	}
	for _, tc := range []struct {
		op      Op
		intOp   Op
		floatOp Op
		strOp   Op
	}{
		{OpAdd, OpAddInt, OpAddFloat, OpMax},
		{OpSub, OpSubInt, OpSubFloat, OpMax},
		{OpMul, OpMulInt, OpMulFloat, OpMax},
		{OpMod, OpModInt, OpMax, OpMax},
		{OpDiv, OpDivFloat, OpDivFloat, OpMax},
		{OpUnm, OpNegInt, OpNegFloat, OpMax},
		{OpEq, OpEqInt, OpMax, OpEqString},
		{OpLt, OpLtInt, OpLtFloat, OpMax},
		{OpLe, OpLeInt, OpLeFloat, OpMax},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.TypeSpecializeIntOp != tc.intOp ||
			spec.TypeSpecializeFloatOp != tc.floatOp ||
			spec.TypeSpecializeStringOp != tc.strOp {
			t.Fatalf("%s type-specialization targets should be driven by OpSpec", tc.op)
		}
	}
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpDiv, OpLt, OpLe} {
		if !shouldInsertNumToFloat(op) {
			t.Fatalf("%s NumToFloat insertion contract should be driven by OpSpec", op)
		}
	}
	if !isIntRecurrenceOp(OpAddInt) || !isNumericOp(OpAddFloat) {
		t.Fatalf("type-specialization recurrence/numeric contracts should be driven by OpSpec")
	}

	for _, op := range []Op{OpCall, OpSetTable, OpSetList, OpAppend} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldSvalsCrossBlockBarrier || !crossBlockFieldSvalsGlobalBarrier(&Instr{Op: op}) {
			t.Fatalf("%s cross-block FieldSvals barrier should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpSetTable, OpCall, OpSetGlobal} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldSvalsGlobalBarrier || !fieldSvalsGlobalBarrier(&Instr{Op: op}) {
			t.Fatalf("%s FieldSvals global barrier should be driven by OpSpec", op)
		}
	}
	if !fieldLenFoldBarrier(&Instr{Op: OpSetTable}) || !fieldCallPolyLenFusionBarrier(&Instr{Op: OpGo}) {
		t.Fatalf("field length barrier contracts should be driven by OpSpec")
	}
}
