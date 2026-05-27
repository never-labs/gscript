package methodjit

var opNeedsTier2FieldCachePolicies = [...]bool{
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
	OpSetField:           true,
}

var opFieldReadPolicies = [...]bool{
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
}

var opFieldSlotLoadPolicies = [...]bool{
	OpFieldLoad:           true,
	OpFieldLoadNumToFloat: true,
}

var opFieldWritePolicies = [...]bool{
	OpSetField:   true,
	OpFieldStore: true,
}

var opBoolTableFillBodyBenignPolicies = [...]bool{
	OpNop:       true,
	OpJump:      true,
	OpConstInt:  true,
	OpConstBool: true,
	OpConstNil:  true,
	OpAddInt:    true,
}

var opBoolTableFillStorePolicies = [...]bool{
	OpSetTable:        true,
	OpTableArrayStore: true,
}

var opBoolTableCountLoadBodyBenignPolicies = [...]bool{
	OpNop:         true,
	OpJump:        true,
	OpBranch:      true,
	OpGuardTruthy: true,
}

var opBoolTableCountLoadPolicies = [...]bool{
	OpTableArrayLoad: true,
}

var opBoolTableCountIncrementBenignPolicies = [...]bool{
	OpNop:      true,
	OpJump:     true,
	OpConstInt: true,
}

var opBoolTableCountIncrementPolicies = [...]bool{
	OpAdd:    true,
	OpAddInt: true,
}

var opCallResultRangeGuardCandidatePolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opModuloReducibleCallFloorPolicies = [...]bool{
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opCallFloorSpecStableCalleePolicies = [...]bool{
	OpCallFloor: true,
}

var opCallFloorSpecFieldShapePolicies = [...]bool{
	OpFieldCallFloor: true,
}

var opTier2LoopCallPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opTier2LoopFeedbackVMProtoCallPolicies = [...]bool{
	OpCall:      true,
	OpCallFloor: true,
}

var opTier2ResidualCallBlockerPolicies = [...]bool{
	OpCall:      true,
	OpCallFloor: true,
}

var opTier2LoopNativeCandidatePolicies = [...]bool{
	OpFieldCallFloor: true,
}

type opCallUserArgStartPolicy struct {
	Start int
	Set   bool
}

var opCallUserArgStartPolicies = [...]opCallUserArgStartPolicy{
	OpCall:           {Start: 1, Set: true},
	OpCallFloor:      {Start: 1, Set: true},
	OpFieldCallFloor: {Start: 0, Set: true},
}

var opSpeculativeIntUseCandidatePolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpMod: true,
	OpLt:  true,
	OpLe:  true,
}

var opFloatRegResultPolicies = [...]bool{
	OpConstFloat: true,
	OpAddFloat:   true,
	OpSubFloat:   true,
	OpMulFloat:   true,
	OpDivFloat:   true,
	OpNegFloat:   true,
	OpUnboxFloat: true,
	OpBoxFloat:   true,
}

var opFloatRegResultBlockedPolicies = [...]bool{
	OpLtFloat:            true,
	OpLeFloat:            true,
	OpComplexEscapeInSet: true,
}

var opRawIntCarryValuePolicies = [...]bool{
	OpConstInt:         true,
	OpLoadSlot:         true,
	OpGuardType:        true,
	OpGuardIntRange:    true,
	OpCall:             true,
	OpCallFloor:        true,
	OpFieldCallFloor:   true,
	OpPhi:              true,
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
}

var opTableResultRawTablePtrPolicies = [...]bool{
	OpTableArrayLoad: true,
}

var opTableArrayRegionGlobalBarrierPolicies = [...]bool{
	OpCall:               true,
	OpCallFloor:          true,
	OpFieldCallFloor:     true,
	OpResume:             true,
	OpSelf:               true,
	OpSetTable:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

var opTableArrayRegionAliasingCallPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opTableArrayRegionAliasingAlwaysPolicies = [...]bool{
	OpResume: true,
	OpSelf:   true,
}

var opTableArrayRegionTableMutationPolicies = [...]bool{
	OpSetTable:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

var opTableMetatableMutationBarrierPolicies = [...]bool{
	OpCall:      true,
	OpSelf:      true,
	OpSetGlobal: true,
	OpSetUpval:  true,
	OpAppend:    true,
	OpSetList:   true,
	OpConcat:    true,
	OpPow:       true,
	OpClosure:   true,
	OpClose:     true,
	OpTForCall:  true,
	OpTForLoop:  true,
	OpVararg:    true,
	OpTestSet:   true,
	OpGo:        true,
	OpMakeChan:  true,
	OpSend:      true,
	OpRecv:      true,
}

func applyOpSpecTableCallPolicies(op Op, spec *OpSpec) {
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
}
