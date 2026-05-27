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
