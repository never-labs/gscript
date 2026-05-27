//go:build darwin && arm64

package methodjit

import "testing"

func TestOpEmitterFamiliesMatchEmitDispatch(t *testing.T) {
	handled := emitInstrHandledOps(t)
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		dispatchFamily, isHandled := handled[op]
		if isHandled {
			if dispatchFamily != OpEmitterInvalid && spec.EmitterFamily != dispatchFamily {
				t.Fatalf("%s OpSpec family=%v, emitter helper family=%v", spec.Name, spec.EmitterFamily, dispatchFamily)
			}
			continue
		}
		if !opIntentionallyNotHandledByEmitInstr(op) {
			t.Fatalf("%s has OpSpec family %v but is not handled by emit_dispatch", spec.Name, spec.EmitterFamily)
		}
	}
}

func TestEmitInstrDelegatesRegisteredEmitterFamilies(t *testing.T) {
	calls := emitterMethodCalls(t, "emit_dispatch.go", "emitInstr")
	for _, delegate := range emitterFamilyDelegateRegistry() {
		if !calls[delegate.funcName] {
			t.Fatalf("emitInstr does not delegate to %s for family %v", delegate.funcName, delegate.family)
		}
	}
}

func TestOpBackendPolicyMatchesEmitDispatchExplicitCleanup(t *testing.T) {
	explicitClears := emitInstrOpsCalling(t, "clearTableArrayBoundedKeys")
	for _, delegate := range emitterFamilyDelegateRegistry() {
		mergeOps(explicitClears, emitterOpsCalling(t, delegate.filename, delegate.funcName, "clearTableArrayBoundedKeys"))
	}
	shapeInvalidations := emitInstrOpsInvalidatingShape(t)
	for _, delegate := range emitterFamilyDelegateRegistry() {
		mergeOps(shapeInvalidations, emitterOpsInvalidatingShape(t, delegate.filename, delegate.funcName))
	}
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if got, want := spec.BackendPolicy&OpBackendClearsTableArrayBounds != 0, explicitClears[op]; got != want {
			t.Fatalf("%s clears table array bounds policy=%v, emit_dispatch=%v", spec.Name, got, want)
		}
		if got, want := spec.BackendPolicy&OpBackendInvalidatesShape != 0, shapeInvalidations[op]; got != want {
			t.Fatalf("%s invalidates shape policy=%v, emit_dispatch=%v", spec.Name, got, want)
		}
	}
}

func TestOpBackendPolicyMatchesDispatchPreserveHelpers(t *testing.T) {
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		policy := spec.BackendPolicy
		if got, want := instrPreservesTableArrayBoundedKeys(&Instr{Op: op}), policy&OpBackendPreservesTableArrayBounds != 0; got != want {
			t.Fatalf("%s table array bounds preserve helper=%v, policy=%v", spec.Name, got, want)
		}

		unknownInstr := &Instr{Op: op, Type: TypeUnknown}
		floatInstr := &Instr{Op: op, Type: TypeFloat}
		wantUnknown := policy&OpBackendPreservesFieldSvalsCache != 0
		wantFloat := wantUnknown || policy&OpBackendPreservesFieldSvalsCacheForFloatResult != 0
		if got := instrPreservesFieldSvalsCache(unknownInstr); got != wantUnknown {
			t.Fatalf("%s field svals preserve helper with unknown type=%v, policy=%v", spec.Name, got, wantUnknown)
		}
		if got := instrPreservesFieldSvalsCache(floatInstr); got != wantFloat {
			t.Fatalf("%s field svals preserve helper with float type=%v, policy=%v", spec.Name, got, wantFloat)
		}

		wantUnknown = policy&OpBackendPreservesScratchFPRCache != 0
		wantFloat = wantUnknown || policy&OpBackendPreservesScratchFPRCacheForFloatResult != 0
		if got := instrPreservesScratchFPRCache(unknownInstr); got != wantUnknown {
			t.Fatalf("%s scratch FPR preserve helper with unknown type=%v, policy=%v", spec.Name, got, wantUnknown)
		}
		if got := instrPreservesScratchFPRCache(floatInstr); got != wantFloat {
			t.Fatalf("%s scratch FPR preserve helper with float type=%v, policy=%v", spec.Name, got, wantFloat)
		}
	}
}

func TestScratchFPRCachePreservePolicyBoundaries(t *testing.T) {
	floatResultOps := []Op{
		OpAddFloat,
		OpSubFloat,
		OpMulFloat,
		OpDivFloat,
		OpNegFloat,
		OpSqrt,
		OpFMA,
		OpFMSUB,
	}
	for _, op := range floatResultOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.BackendPolicy&OpBackendPreservesScratchFPRCacheForFloatResult == 0 {
			t.Fatalf("%s missing scratch FPR float-result preserve policy", spec.Name)
		}
		if instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeUnknown}) {
			t.Fatalf("%s preserves scratch FPR cache without float result type", spec.Name)
		}
		if !instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeFloat}) {
			t.Fatalf("%s does not preserve scratch FPR cache with float result type", spec.Name)
		}
	}

	generalOps := []Op{OpLtFloat, OpLeFloat, OpNop}
	for _, op := range generalOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.BackendPolicy&OpBackendPreservesScratchFPRCache == 0 {
			t.Fatalf("%s missing unconditional scratch FPR preserve policy", spec.Name)
		}
		if !instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeUnknown}) {
			t.Fatalf("%s should preserve scratch FPR cache independent of result type", spec.Name)
		}
	}

	clearingOps := []Op{OpAddInt, OpLtInt, OpNumToFloat, OpBoxFloat}
	for _, op := range clearingOps {
		spec, ok := op.Spec()
		if !ok {
			t.Fatalf("%d has no OpSpec", op)
		}
		if spec.BackendPolicy&(OpBackendPreservesScratchFPRCache|OpBackendPreservesScratchFPRCacheForFloatResult) != 0 {
			t.Fatalf("%s unexpectedly declares scratch FPR preserve policy", spec.Name)
		}
		if instrPreservesScratchFPRCache(&Instr{Op: op, Type: TypeFloat}) {
			t.Fatalf("%s unexpectedly preserves scratch FPR cache", spec.Name)
		}
	}
}
