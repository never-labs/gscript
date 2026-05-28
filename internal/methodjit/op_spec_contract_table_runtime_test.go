package methodjit

import "testing"

func TestTableRuntimeTypeContractsLiveInOpSpec(t *testing.T) {
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
	if spec, ok := OpNewTable.Spec(); !ok || !spec.StaticTableLenBuilder || !opIsStaticTableLenBuilder(OpNewTable) {
		t.Fatalf("%s static table len builder contract should be driven by OpSpec", OpNewTable)
	}
	if spec, ok := OpSetList.Spec(); !ok || !spec.StaticTableLenInitializer || !opIsStaticTableLenInitializer(OpSetList) {
		t.Fatalf("%s static table len initializer contract should be driven by OpSpec", OpSetList)
	}
	for _, op := range []Op{OpSetTable, OpAppend} {
		spec, ok := op.Spec()
		if !ok || !spec.StaticTableLenInvalidator || !opIsStaticTableLenInvalidator(op) {
			t.Fatalf("%s static table len invalidator contract should be driven by OpSpec", op)
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
}
