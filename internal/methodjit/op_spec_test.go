package methodjit

import "testing"

func TestEveryOpHasSpecAndName(t *testing.T) {
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.Name == "" {
			t.Fatalf("%d has empty OpSpec name", op)
		}
		if got := op.String(); got != spec.Name {
			t.Fatalf("%s String()=%q, want OpSpec name %q", spec.Name, got, spec.Name)
		}
		if spec.SideEffect == OpSideEffectInvalid {
			t.Fatalf("%s has invalid side effect", spec.Name)
		}
		if spec.ArgPolicy == OpArgInvalid {
			t.Fatalf("%s has invalid arg policy", spec.Name)
		}
		if spec.EmitterFamily == OpEmitterInvalid {
			t.Fatalf("%s has invalid emitter family", spec.Name)
		}
	}
}

func TestOpIsTerminatorUsesSpec(t *testing.T) {
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if got := op.IsTerminator(); got != spec.Terminator {
			t.Fatalf("%s IsTerminator()=%v, want %v", spec.Name, got, spec.Terminator)
		}
	}
}

func TestOnlyControlTransferOpsAreTerminators(t *testing.T) {
	allowed := map[Op]bool{
		OpJump:   true,
		OpBranch: true,
		OpReturn: true,
	}
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.Terminator != allowed[op] {
			t.Fatalf("%s terminator=%v, want %v", spec.Name, spec.Terminator, allowed[op])
		}
	}
}

func TestValidatorCountContractsLiveInOpSpec(t *testing.T) {
	cases := []struct {
		op       Op
		minArgs  int
		maxArgs  int
		succs    int
		hasSuccs bool
	}{
		{op: OpSetTable, minArgs: 3, maxArgs: 3},
		{op: OpGuardGlobalConst, minArgs: 0, maxArgs: 0},
		{op: OpGuardType, minArgs: 1, maxArgs: 1},
		{op: OpFieldStore, minArgs: 2, maxArgs: 2},
		{op: OpRecordArrayLoopSpecialization, minArgs: 3, maxArgs: 16},
		{op: OpJump, minArgs: 0, maxArgs: 0, succs: 1, hasSuccs: true},
		{op: OpBranch, minArgs: 1, maxArgs: 1, succs: 2, hasSuccs: true},
		{op: OpReturn, minArgs: 0, maxArgs: OpCountAny, succs: 0, hasSuccs: true},
	}
	for _, tc := range cases {
		spec, ok := tc.op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", tc.op)
		}
		if !spec.ArgCount.Set || spec.ArgCount.Min != tc.minArgs || spec.ArgCount.Max != tc.maxArgs {
			t.Fatalf("%s ArgCount=%+v, want %d..%d", tc.op, spec.ArgCount, tc.minArgs, tc.maxArgs)
		}
		if tc.hasSuccs && spec.SuccCount != tc.succs {
			t.Fatalf("%s SuccCount=%d, want %d", tc.op, spec.SuccCount, tc.succs)
		}
	}
}

func TestDCEKeepUnusedContractLivesInOpSpec(t *testing.T) {
	keep := []Op{
		OpReturn,
		OpSetTable,
		OpMatrixSetF,
		OpCall,
		OpGuardType,
		OpForLoop,
		OpClosure,
		OpGo,
	}
	for _, op := range keep {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.KeepUnused {
			t.Fatalf("%s should be kept when unused by OpSpec", op)
		}
		if !hasSideEffect(&Instr{Op: op}) {
			t.Fatalf("%s hasSideEffect should be driven by OpSpec KeepUnused", op)
		}
	}

	droppable := []Op{OpAddInt, OpConstInt, OpNewTable, OpTableArrayLoad}
	for _, op := range droppable {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if spec.KeepUnused {
			t.Fatalf("%s should be droppable when unused by OpSpec", op)
		}
		if hasSideEffect(&Instr{Op: op}) {
			t.Fatalf("%s hasSideEffect should be false through OpSpec KeepUnused", op)
		}
	}
}

func TestNativeReplayContractsLiveInOpSpec(t *testing.T) {
	mayExit := []Op{OpCall, OpSetTable, OpGetField, OpGuardType, OpAddInt, OpMatrixDense}
	for _, op := range mayExit {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.NativeReplayMayExit {
			t.Fatalf("%s should be marked as native-replay may-exit by OpSpec", op)
		}
		if !tier2OpMayExitForNativeReplay(&Instr{Op: op}) {
			t.Fatalf("%s native replay may-exit query should be driven by OpSpec", op)
		}
	}

	if spec, _ := OpSetGlobal.Spec(); !spec.NativeReplayVisibleSideEffect || !spec.NativeCalleeResumeUnsafe {
		t.Fatalf("SetGlobal should carry native visible and callee-resume-unsafe contracts in OpSpec")
	}
	if !tier2InstrHasNativeVisibleSideEffect(&Instr{Op: OpSetGlobal}) {
		t.Fatalf("SetGlobal native visible side-effect query should be driven by OpSpec")
	}

	if spec, _ := OpSetTable.Spec(); !spec.NativeReplayVisibleTableMutation {
		t.Fatalf("SetTable should carry native visible table-mutation contract in OpSpec")
	}
	if !tier2InstrHasNativeVisibleSideEffect(&Instr{Op: OpSetTable}) {
		t.Fatalf("SetTable without local allocation should be native visible")
	}

	localTable := &Instr{ID: 1, Op: OpNewTable}
	store := &Instr{ID: 2, Op: OpSetTable, Args: []*Value{localTable.Value()}}
	if tier2InstrHasNativeVisibleSideEffect(store) {
		t.Fatalf("SetTable to local allocation should keep its native replay exception")
	}
}

func TestRestartVisibleSideEffectContractLivesInOpSpec(t *testing.T) {
	visible := []Op{OpCall, OpSetTable, OpSetGlobal, OpNewTable, OpSelf, OpVararg, OpLen, OpTForLoop}
	for _, op := range visible {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.RestartVisibleSideEffect {
			t.Fatalf("%s should be marked restart-visible by OpSpec", op)
		}
		fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{Op: op}}}}}
		if !hasRestartVisibleSideEffect(fn) {
			t.Fatalf("%s restart-visible query should be driven by OpSpec", op)
		}
	}

	pure := []Op{OpAddInt, OpConstInt, OpTableArrayLoad, OpGetField}
	for _, op := range pure {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if spec.RestartVisibleSideEffect {
			t.Fatalf("%s should not be restart-visible by OpSpec", op)
		}
		fn := &Function{Blocks: []*Block{{Instrs: []*Instr{{Op: op}}}}}
		if hasRestartVisibleSideEffect(fn) {
			t.Fatalf("%s restart-visible query should be false through OpSpec", op)
		}
	}
}

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

func TestBackendAndLoopContractsLiveInOpSpec(t *testing.T) {
	noResult := []Op{OpSetTable, OpFieldStore, OpGuardGlobalConst, OpMatrixSetF, OpClose}
	for _, op := range noResult {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.NoSSAResult || !instructionHasNoSSAResult(&Instr{Op: op}) {
			t.Fatalf("%s no-SSA-result contract should be driven by OpSpec", op)
		}
	}

	raws := []struct {
		op   Op
		want string
	}{
		{OpAddInt, "int"},
		{OpTableArrayHeader, "tableptr"},
		{OpTableArrayData, "dataptr"},
		{OpAddFloat, "float"},
		{OpMatrixDense, "matrix"},
	}
	for _, tc := range raws {
		spec, ok := tc.op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", tc.op)
		}
		switch tc.want {
		case "int":
			if !spec.RawIntResult || !isRawIntOp(tc.op) {
				t.Fatalf("%s raw-int contract should be driven by OpSpec", tc.op)
			}
		case "tableptr":
			if !spec.RawTablePtrResult || !isRawTablePtrOp(tc.op) {
				t.Fatalf("%s raw-table-ptr contract should be driven by OpSpec", tc.op)
			}
		case "dataptr":
			if !spec.RawDataPtrResult || !isRawDataPtrOp(tc.op) {
				t.Fatalf("%s raw-data-ptr contract should be driven by OpSpec", tc.op)
			}
		case "float":
			if !spec.RawFloatResult || !isRawFloatOp(tc.op) {
				t.Fatalf("%s raw-float contract should be driven by OpSpec", tc.op)
			}
		case "matrix":
			if !spec.MatrixNative || !isMatrixNativeOp(tc.op) {
				t.Fatalf("%s matrix-native contract should be driven by OpSpec", tc.op)
			}
		}
	}

	licm := []Op{OpConstInt, OpGetField, OpAddFloat, OpGuardType, OpTableArrayHeader, OpMatrixRowPtr}
	for _, op := range licm {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", op)
		}
		if !spec.LICMHoistable || !canHoistOp(op) {
			t.Fatalf("%s LICM hoistability should be driven by OpSpec", op)
		}
	}
	if !isInterestingLICMMiss(OpGetField) || !isIntArithOp(OpAddInt) {
		t.Fatalf("LICM diagnostic/int-arith contracts should be driven by OpSpec")
	}
	if !pureNumericInlineOp(OpAddInt) || !nativeEffectLoopInlineOp(OpTableArrayLoad) {
		t.Fatalf("inline loop contracts should be driven by OpSpec")
	}
	if !instrMayDirectDeoptWithoutFullFlush(&Instr{Op: OpGuardType}) ||
		!instrMayDirectDeoptWithoutFullFlush(&Instr{Op: OpGetField, Type: TypeFloat}) {
		t.Fatalf("direct-deopt contracts should be driven by OpSpec plus instr-specific GetField float rule")
	}

	for _, tc := range []struct {
		op   Op
		rank int
		mask uint8
	}{
		{OpTableArrayData, 0, 0},
		{OpMatrixFlat, 0, 0},
		{OpTableArrayLen, 1, 0},
		{OpTableArrayHeader, 2, 0},
		{OpTableArrayLoad, 1, 1<<0 | 1<<1},
		{OpTableArrayNestedLoad, 1, 1<<0 | 1<<1 | 1<<2},
		{OpTableArrayStore, 1, 1<<1 | 1<<2 | 1<<5},
		{OpMatrixLoadFAt, 1, 1<<0 | 1<<1},
		{OpMatrixStoreFAt, 1, 1<<0 | 1<<1},
	} {
		spec, ok := tc.op.Spec()
		if !ok {
			t.Fatalf("%s has no OpSpec", tc.op)
		}
		if spec.TableArrayGPRInvariantRank != tc.rank || spec.TableArrayGPRInvariantUseMask != tc.mask {
			t.Fatalf("%s table-array invariant policy = rank %d mask %#x, want rank %d mask %#x",
				tc.op, spec.TableArrayGPRInvariantRank, spec.TableArrayGPRInvariantUseMask, tc.rank, tc.mask)
		}
	}
	for _, tc := range []struct {
		op  Op
		arg int
	}{
		{OpTableArrayLoad, 2},
		{OpTableArrayStore, 3},
		{OpTableArraySwap, 1},
		{OpTableArraySwapPairs, 1},
		{OpTableArrayNestedLoad, 3},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.TableArrayKeyArgIndex != tc.arg {
			t.Fatalf("%s table-array key arg = %d, want %d", tc.op, spec.TableArrayKeyArgIndex, tc.arg)
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

func TestRangeAndBackendContractsLiveInOpSpec(t *testing.T) {
	for _, op := range []Op{OpConstInt, OpLen, OpTableArrayLen, OpGuardIntRange, OpAddInt, OpMulInt, OpModInt, OpDivIntExact, OpPhi, OpBoxInt, OpUnboxInt} {
		spec, ok := op.Spec()
		if !ok || !spec.NonNegativeDerivationCandidate || !opCanDeriveNonNegative(&Instr{Op: op}) {
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
	for _, op := range []Op{OpGetField, OpGetFieldNumToFloat, OpSetField} {
		spec, ok := op.Spec()
		if !ok || !spec.NeedsTier2FieldCache {
			t.Fatalf("%s field-cache requirement should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpNop, OpJump, OpConstInt, OpConstBool, OpConstNil, OpAddInt} {
		spec, ok := op.Spec()
		if !ok || !spec.BoolTableFillBodyBenign {
			t.Fatalf("%s bool-fill body benign contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpSetTable, OpTableArrayStore} {
		spec, ok := op.Spec()
		if !ok || !spec.BoolTableFillStore {
			t.Fatalf("%s bool-fill store contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpNop, OpJump, OpBranch, OpGuardTruthy} {
		spec, ok := op.Spec()
		if !ok || !spec.BoolTableCountLoadBodyBenign {
			t.Fatalf("%s bool-count load-body benign contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpTableArrayLoad.Spec(); !ok || !spec.BoolTableCountLoad {
		t.Fatalf("TableArrayLoad bool-count load contract should be driven by OpSpec")
	}
	for _, op := range []Op{OpNop, OpJump, OpConstInt} {
		spec, ok := op.Spec()
		if !ok || !spec.BoolTableCountIncrementBenign {
			t.Fatalf("%s bool-count increment benign contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAdd, OpAddInt} {
		spec, ok := op.Spec()
		if !ok || !spec.BoolTableCountIncrement {
			t.Fatalf("%s bool-count increment contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCall, OpCallFloor, OpFieldCallFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.CallResultRangeGuardCandidate || !callResultRangeGuardCandidate(&Instr{Op: op}) {
			t.Fatalf("%s call-result range guard contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCallFloor, OpFieldCallFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.ModuloReducibleCallFloor || !isModuloReducibleCallFloor(&Instr{Op: op, Type: TypeInt}) {
			t.Fatalf("%s modulo-reducible floor-call contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpCallFloor.Spec(); !ok || !spec.CallFloorSpecStableCallee {
		t.Fatalf("CallFloor stable-callee range speculation contract should be driven by OpSpec")
	}
	if spec, ok := OpFieldCallFloor.Spec(); !ok || !spec.CallFloorSpecFieldShape {
		t.Fatalf("FieldCallFloor field-shape range speculation contract should be driven by OpSpec")
	}
	for _, op := range []Op{OpCall, OpCallFloor, OpFieldCallFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.Tier2LoopCall || !tier2LoopCallOp(op) {
			t.Fatalf("%s Tier2 loop-call contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpMod, OpLt, OpLe} {
		spec, ok := op.Spec()
		if !ok || !spec.SpeculativeIntUseCandidate {
			t.Fatalf("%s speculative int-use contract should be driven by OpSpec", op)
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
	if spec, ok := OpTableArrayLoad.Spec(); !ok || !spec.TableResultRawTablePtr || !isRawTablePtrValue(&Instr{Op: OpTableArrayLoad, Type: TypeTable}) {
		t.Fatalf("TableArrayLoad table-result raw-table-ptr contract should be driven by OpSpec")
	}
	for _, op := range []Op{OpCall, OpCallFloor, OpFieldCallFloor, OpResume, OpSelf, OpSetTable, OpAppend, OpSetList, OpTableBoolArrayFill} {
		spec, ok := op.Spec()
		if !ok || !spec.TableArrayRegionGlobalBarrier {
			t.Fatalf("%s table-array region global barrier should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCall, OpSelf, OpSetGlobal, OpSetUpval, OpAppend, OpSetList, OpConcat, OpPow, OpClosure, OpClose, OpTForCall, OpTForLoop, OpVararg, OpTestSet, OpGo, OpMakeChan, OpSend, OpRecv} {
		spec, ok := op.Spec()
		if !ok || !spec.TableMetatableMutationBarrier {
			t.Fatalf("%s table-metatable mutation barrier should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAddInt, OpSubInt, OpMulInt, OpNegInt} {
		spec, ok := op.Spec()
		if !ok || !spec.RuntimeOverflowBoxable || !tier2IntOverflowOpCanBox(op.String()) {
			t.Fatalf("%s runtime overflow boxing contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpGuardType, OpGuardCalleeProto, OpGuardConstString, OpGuardTableKind, OpGuardIntRange} {
		spec, ok := op.Spec()
		if !ok || !spec.RuntimeGuardRefreshable || !tier2GuardOpCanRefresh(op.String()) {
			t.Fatalf("%s runtime guard refresh contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCall, OpResume, OpGo, OpSend, OpRecv} {
		spec, ok := op.Spec()
		if !ok || !spec.MayCallOrRunConcurrently() {
			t.Fatalf("%s call/concurrency side-effect contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpConstFloat, OpUnboxInt, OpUnboxFloat, OpAdd, OpMod, OpAddInt, OpAddFloat, OpFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.NativeNumericValueProducer {
			t.Fatalf("%s native numeric value contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpDiv, OpMod, OpUnm, OpAddInt, OpDivIntExact, OpAddFloat, OpNumToFloat, OpPhi, OpLoadSlot} {
		spec, ok := op.Spec()
		if !ok || !spec.PureNumericUnknownValue {
			t.Fatalf("%s pure numeric unknown-value contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpConstInt, OpTableArrayHeader, OpAddInt, OpBoxInt, OpGuardTableKind, OpNop} {
		spec, ok := op.Spec()
		if !ok || !spec.TableArraySwapPureBetween || !tableArraySwapPureBetween(&Instr{Op: op}) {
			t.Fatalf("%s table-array swap pure-between contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpLen, OpGetTable, OpTableArrayHeader} {
		spec, ok := op.Spec()
		instr := &Instr{Op: op, Args: []*Value{{ID: 7}}}
		if !ok || !spec.StaticTableLenBenignUse || !staticTableLenBenignUse(instr, 7) {
			t.Fatalf("%s static table len benign-use contract should be driven by OpSpec", op)
		}
	}
	for _, tc := range []struct {
		op  Op
		typ Type
	}{
		{OpConstInt, TypeInt},
		{OpAddFloat, TypeFloat},
		{OpEqInt, TypeBool},
		{OpConstString, TypeString},
		{OpNewTable, TypeTable},
		{OpClosure, TypeFunction},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.FixedResultType != tc.typ || spec.GuardProvenResultType != tc.typ {
			t.Fatalf("%s fixed/proven result type contracts should be driven by OpSpec", tc.op)
		}
	}
	for _, op := range []Op{OpConstInt, OpAdd, OpDivIntExact, OpGetFieldNumToFloat, OpFieldLoadNumToFloat, OpLen, OpEqInt} {
		spec, ok := op.Spec()
		if !ok || !spec.ProvesNonNilResult || !valueProvenNonNil((&Instr{Op: op}).Value()) {
			t.Fatalf("%s non-nil producer contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpAddFloat, OpSqrt, OpFMA, OpGetFieldNumToFloat, OpFieldLoadNumToFloat, OpNumToFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.RawFloatValueProducer {
			t.Fatalf("%s raw-float value producer contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpSetTable, OpTableArrayStore, OpTableArraySwap, OpAppend, OpSetList} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldFactWideKiller || !spec.TableMutationFirstArg {
			t.Fatalf("%s table/field mutation contracts should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCall, OpCallFloor, OpFieldCallFloor, OpResume, OpYield, OpSelf, OpGo, OpSend, OpRecv} {
		spec, ok := op.Spec()
		if !ok || !spec.CallLikeFactBarrier {
			t.Fatalf("%s call-like fact barrier contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCall, OpCallFloor, OpFieldCallFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.RawCarryClobber {
			t.Fatalf("%s raw-carry clobber contract should be driven by OpSpec", op)
		}
	}
	for _, tc := range []struct {
		op     Op
		policy OpSourceFeedbackPolicy
	}{
		{OpGetField, OpSourceFeedbackGetField},
		{OpGetFieldNumToFloat, OpSourceFeedbackGetField},
		{OpSetField, OpSourceFeedbackSetField},
		{OpGetTable, OpSourceFeedbackGetTable},
		{OpSetTable, OpSourceFeedbackSetTable},
		{OpAdd, OpSourceFeedbackResultType},
		{OpLe, OpSourceFeedbackResultType},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.SourceFeedbackPolicy != tc.policy {
			t.Fatalf("%s source-feedback policy should be driven by OpSpec", tc.op)
		}
	}
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
	for _, op := range []Op{OpAdd, OpSub, OpMul, OpDiv, OpMod, OpUnm} {
		spec, ok := op.Spec()
		if !ok || !spec.RawIntSpecializationBlocker {
			t.Fatalf("%s raw-int residual blocker should be driven by OpSpec", op)
		}
	}
}

func TestOpsByEmitterFamily(t *testing.T) {
	got := OpsByEmitterFamily(OpEmitterControl)
	want := []Op{OpJump, OpBranch, OpReturn, OpTestSet}
	if !sameOps(got, want) {
		t.Fatalf("control family ops = %v, want %v", got, want)
	}

	got = OpsByEmitterFamily(OpEmitterMatrix)
	want = []Op{
		OpMatrixDense,
		OpMatrixGetF,
		OpMatrixSetF,
		OpMatrixFlat,
		OpMatrixStride,
		OpMatrixLoadFAt,
		OpMatrixStoreFAt,
		OpMatrixRowPtr,
		OpMatrixLoadFRow,
		OpMatrixStoreFRow,
		OpMatrixLoadFRowConst,
		OpMatrixStoreFRowConst,
	}
	if !sameOps(got, want) {
		t.Fatalf("matrix family ops = %v, want %v", got, want)
	}
}

func sameOps(a, b []Op) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
