package methodjit

import "testing"

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
