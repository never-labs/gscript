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
