package methodjit

func applyOpSpecTableCallPolicies(op Op, spec *OpSpec) {
	applyOpSpecTableCallFieldPolicies(op, spec)
	applyOpSpecTableCallBoolPolicies(op, spec)
	applyOpSpecTableCallCallPolicies(op, spec)
	applyOpSpecTableCallNumericPolicies(op, spec)
	applyOpSpecTableCallBarrierPolicies(op, spec)
}

func applyOpSpecTableCallFieldPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opNeedsTier2FieldCachePolicies) {
		spec.NeedsTier2FieldCache = opNeedsTier2FieldCachePolicies[op]
	}
	if int(op) < len(opFieldReadPolicies) {
		spec.FieldRead = opFieldReadPolicies[op]
	}
	if int(op) < len(opFieldSlotLoadPolicies) {
		spec.FieldSlotLoad = opFieldSlotLoadPolicies[op]
	}
	if int(op) < len(opFieldWritePolicies) {
		spec.FieldWrite = opFieldWritePolicies[op]
	}
}

func applyOpSpecTableCallBoolPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opBoolTableFillBodyBenignPolicies) {
		spec.BoolTableFillBodyBenign = opBoolTableFillBodyBenignPolicies[op]
	}
	if int(op) < len(opBoolTableFillStorePolicies) {
		spec.BoolTableFillStore = opBoolTableFillStorePolicies[op]
	}
	if int(op) < len(opBoolTableCountLoadBodyBenignPolicies) {
		spec.BoolTableCountLoadBodyBenign = opBoolTableCountLoadBodyBenignPolicies[op]
	}
	if int(op) < len(opBoolTableCountLoadPolicies) {
		spec.BoolTableCountLoad = opBoolTableCountLoadPolicies[op]
	}
	if int(op) < len(opBoolTableCountIncrementBenignPolicies) {
		spec.BoolTableCountIncrementBenign = opBoolTableCountIncrementBenignPolicies[op]
	}
	if int(op) < len(opBoolTableCountIncrementPolicies) {
		spec.BoolTableCountIncrement = opBoolTableCountIncrementPolicies[op]
	}
}

func applyOpSpecTableCallCallPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opCallResultRangeGuardCandidatePolicies) {
		spec.CallResultRangeGuardCandidate = opCallResultRangeGuardCandidatePolicies[op]
	}
	if int(op) < len(opModuloReducibleCallFloorPolicies) {
		spec.ModuloReducibleCallFloor = opModuloReducibleCallFloorPolicies[op]
	}
	if int(op) < len(opCallFloorSpecStableCalleePolicies) {
		spec.CallFloorSpecStableCallee = opCallFloorSpecStableCalleePolicies[op]
	}
	if int(op) < len(opCallFloorSpecFieldShapePolicies) {
		spec.CallFloorSpecFieldShape = opCallFloorSpecFieldShapePolicies[op]
	}
	if int(op) < len(opTier2LoopCallPolicies) {
		spec.Tier2LoopCall = opTier2LoopCallPolicies[op]
	}
	if int(op) < len(opTier2LoopFeedbackVMProtoCallPolicies) {
		spec.Tier2LoopFeedbackVMProtoCall = opTier2LoopFeedbackVMProtoCallPolicies[op]
	}
	if int(op) < len(opTier2ResidualCallBlockerPolicies) {
		spec.Tier2ResidualCallBlocker = opTier2ResidualCallBlockerPolicies[op]
	}
	if int(op) < len(opTier2LoopNativeCandidatePolicies) {
		spec.Tier2LoopNativeCandidate = opTier2LoopNativeCandidatePolicies[op]
	}
	if int(op) < len(opCallUserArgStartPolicies) && opCallUserArgStartPolicies[op].Set {
		spec.CallUserArgStart = opCallUserArgStartPolicies[op].Start
	}
}

func applyOpSpecTableCallNumericPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opSpeculativeIntUseCandidatePolicies) {
		spec.SpeculativeIntUseCandidate = opSpeculativeIntUseCandidatePolicies[op]
	}
	if int(op) < len(opFloatRegResultPolicies) {
		spec.FloatRegResult = opFloatRegResultPolicies[op]
	}
	if int(op) < len(opFloatRegResultBlockedPolicies) {
		spec.FloatRegResultBlocked = opFloatRegResultBlockedPolicies[op]
	}
	if int(op) < len(opRawIntCarryValuePolicies) {
		spec.RawIntCarryValue = opRawIntCarryValuePolicies[op]
	}
}

func applyOpSpecTableCallBarrierPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opTableResultRawTablePtrPolicies) {
		spec.TableResultRawTablePtr = opTableResultRawTablePtrPolicies[op]
	}
	if int(op) < len(opTableArrayRegionGlobalBarrierPolicies) {
		spec.TableArrayRegionGlobalBarrier = opTableArrayRegionGlobalBarrierPolicies[op]
	}
	if int(op) < len(opTableArrayRegionAliasingCallPolicies) {
		spec.TableArrayRegionAliasingCall = opTableArrayRegionAliasingCallPolicies[op]
	}
	if int(op) < len(opTableArrayRegionAliasingAlwaysPolicies) {
		spec.TableArrayRegionAliasingAlways = opTableArrayRegionAliasingAlwaysPolicies[op]
	}
	if int(op) < len(opTableArrayRegionTableMutationPolicies) {
		spec.TableArrayRegionTableMutation = opTableArrayRegionTableMutationPolicies[op]
	}
	if int(op) < len(opTableMetatableMutationBarrierPolicies) {
		spec.TableMetatableMutationBarrier = opTableMetatableMutationBarrierPolicies[op]
	}
	if int(op) < len(opTableArrayFactRolePolicies) {
		spec.TableArrayFactRole = opTableArrayFactRolePolicies[op]
	}
	if int(op) < len(opTableArrayStoreLoopCandidatePolicies) {
		spec.TableArrayStoreLoopCandidate = opTableArrayStoreLoopCandidatePolicies[op]
	}
	if int(op) < len(opTableArrayStoreLoopBlockerPolicies) {
		spec.TableArrayStoreLoopBlocker = opTableArrayStoreLoopBlockerPolicies[op]
	}
	if int(op) < len(opTableArrayStoreLoopEscapeCallPolicies) {
		spec.TableArrayStoreLoopEscapeCall = opTableArrayStoreLoopEscapeCallPolicies[op]
	}
	if int(op) < len(opTableArrayStoreLoopUseOKPolicies) {
		spec.TableArrayStoreLoopUseOK = opTableArrayStoreLoopUseOKPolicies[op]
	}
}
