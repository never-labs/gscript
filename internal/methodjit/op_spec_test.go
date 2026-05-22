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
