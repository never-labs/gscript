package methodjit

import "testing"

func TestTableCallContractsLiveInOpSpec(t *testing.T) {
	if lowered, ok := callFloorProjectionOp(OpCall); !ok || lowered != OpCallFloor {
		t.Fatalf("Call floor projection target = %s, %v; want CallFloor, true", lowered, ok)
	}
	if lowered, ok := fieldCallFloorProjectionOp(OpCall); !ok || lowered != OpFieldCallFloor {
		t.Fatalf("field call floor projection target = %s, %v; want FieldCallFloor, true", lowered, ok)
	}
	if lowered, ok := callFloorProjectionOp(OpCallFloor); ok || lowered != OpMax {
		t.Fatalf("CallFloor projection target = %s, %v; want OpMax, false", lowered, ok)
	}
	if lowered, ok := fieldCalleeGuardLoweredOp(OpGuardCalleeProto); !ok || lowered != OpGuardFieldCalleeProto {
		t.Fatalf("field callee guard target = %s, %v; want GuardFieldCalleeProto, true", lowered, ok)
	}
	if lowered, ok := fieldCalleeGuardLoweredOp(OpGuardFieldCalleeProto); ok || lowered != OpMax {
		t.Fatalf("GuardFieldCalleeProto lowering target = %s, %v; want OpMax, false", lowered, ok)
	}
	for _, op := range []Op{OpGetField, OpGetFieldNumToFloat, OpSetField} {
		spec, ok := op.Spec()
		if !ok || !spec.NeedsTier2FieldCache {
			t.Fatalf("%s field-cache requirement should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpGetField, OpGetFieldNumToFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldRead || !opIsFieldRead(op) {
			t.Fatalf("%s field-read contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpFieldLoad, OpFieldLoadNumToFloat} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldSlotLoad || !opIsFieldSlotLoad(op) {
			t.Fatalf("%s field-slot-load contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpSetField, OpFieldStore} {
		spec, ok := op.Spec()
		if !ok || !spec.FieldWrite || !opIsFieldWrite(op) {
			t.Fatalf("%s field-write contract should be driven by OpSpec", op)
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
		layout, layoutOK := boolTableFillStoreLayoutForOp(op)
		if !ok || !spec.BoolTableFillStore || !layoutOK {
			t.Fatalf("%s bool-fill store contract should be driven by OpSpec", op)
		}
		if layout.TableArg != 0 || layout.KindSource == OpBoolTableFillKindNone {
			t.Fatalf("%s bool-fill store layout should include table arg and kind source", op)
		}
	}
	for _, op := range []Op{OpNop, OpJump, OpBranch, OpGuardTruthy} {
		spec, ok := op.Spec()
		if !ok || !spec.BoolTableCountLoadBodyBenign {
			t.Fatalf("%s bool-count load-body benign contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpTableArrayLoad.Spec(); !ok || !spec.BoolTableCountLoad || spec.TableArrayKeyArgIndex != 2 {
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
	for _, op := range []Op{OpCall, OpCallFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.Tier2LoopFeedbackVMProtoCall {
			t.Fatalf("%s Tier2 loop-call feedback VM proto contract should be driven by OpSpec", op)
		}
	}
	for _, op := range []Op{OpCall, OpCallFloor} {
		spec, ok := op.Spec()
		if !ok || !spec.Tier2ResidualCallBlocker {
			t.Fatalf("%s Tier2 residual-call blocker contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpFieldCallFloor.Spec(); !ok || !spec.Tier2LoopNativeCandidate {
		t.Fatalf("FieldCallFloor Tier2 native-candidate contract should be driven by OpSpec")
	}
	for _, op := range []Op{
		OpSelf, OpConcat, OpAppend, OpSetList,
		OpGo, OpMakeChan, OpSend, OpRecv,
		OpClosure, OpClose, OpVararg, OpPow,
		OpTForCall, OpTForLoop,
	} {
		spec, ok := op.Spec()
		if !ok || !spec.Tier2CallBoundaryLoopBlocker || !opIsTier2CallBoundaryLoopBlocker(op) {
			t.Fatalf("%s Tier2 call-boundary loop blocker contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpSetList.Spec(); !ok || !spec.Tier2LoopAllocationBlocker || !opIsTier2LoopAllocationBlocker(OpSetList) {
		t.Fatalf("SetList Tier2 loop-allocation blocker contract should be driven by OpSpec")
	}
	for _, tc := range []struct {
		op    Op
		start int
	}{
		{op: OpCall, start: 1},
		{op: OpCallFloor, start: 1},
		{op: OpFieldCallFloor, start: 0},
	} {
		spec, ok := tc.op.Spec()
		if !ok || spec.CallUserArgStart != tc.start {
			t.Fatalf("%s call-user arg layout should be driven by OpSpec", tc.op)
		}
	}
}

func TestTableArrayStoreLoopContractsLiveInOpSpec(t *testing.T) {
	if spec, ok := OpSetTable.Spec(); !ok || !spec.TableArrayStoreLoopCandidate || !opIsTableArrayStoreLoopCandidate(OpSetTable) {
		t.Fatalf("SetTable typed-store-loop candidate contract should be driven by OpSpec")
	}
	for _, op := range []Op{OpTableArrayStore, OpResume, OpSelf, OpSetField, OpAppend, OpSetList, OpTableBoolArrayFill} {
		spec, ok := op.Spec()
		if !ok || !spec.TableArrayStoreLoopBlocker || !opIsTableArrayStoreLoopBlocker(op) {
			t.Fatalf("%s typed-store-loop blocker contract should be driven by OpSpec", op)
		}
	}
	if spec, ok := OpCall.Spec(); !ok || !spec.TableArrayStoreLoopEscapeCall || !opIsTableArrayStoreLoopEscapeCall(OpCall) {
		t.Fatalf("Call typed-store-loop escape-analysis contract should be driven by OpSpec")
	}
	for _, op := range []Op{OpGuardTableKind, OpGuardType, OpReturn} {
		spec, ok := op.Spec()
		if !ok || !spec.TableArrayStoreLoopUseOK {
			t.Fatalf("%s typed-store-loop use whitelist contract should be driven by OpSpec", op)
		}
	}
	if !tableArrayStoreLoopUseOK(&Instr{Op: OpGuardTableKind}, true) {
		t.Fatalf("GuardTableKind use whitelist should be driven by OpSpec")
	}
	if !tableArrayStoreLoopUseOK(&Instr{Op: OpGuardType, Type: TypeTable, Aux: int64(TypeTable)}, true) {
		t.Fatalf("GuardType(TypeTable) use whitelist should be driven by OpSpec")
	}
	if tableArrayStoreLoopUseOK(&Instr{Op: OpGuardType, Type: TypeInt, Aux: int64(TypeInt)}, true) {
		t.Fatalf("GuardType non-table uses must not be whitelisted")
	}
	if !tableArrayStoreLoopUseOK(&Instr{Op: OpReturn}, false) || tableArrayStoreLoopUseOK(&Instr{Op: OpReturn}, true) {
		t.Fatalf("Return use whitelist should only allow non-loop returns")
	}
}
