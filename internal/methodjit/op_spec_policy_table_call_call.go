package methodjit

var opCallResultRangeGuardCandidatePolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opModuloReducibleCallFloorPolicies = [...]bool{
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opCallFloorProjectionOpPolicies = [...]opTargetPolicy{
	OpCall: {Op: OpCallFloor, Set: true},
}

var opFieldCallFloorProjectionOpPolicies = [...]opTargetPolicy{
	OpCall: {Op: OpFieldCallFloor, Set: true},
}

var opFieldCalleeGuardLoweredOpPolicies = [...]opTargetPolicy{
	OpGuardCalleeProto: {Op: OpGuardFieldCalleeProto, Set: true},
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

var opTier2CallBoundaryLoopBlockerPolicies = [...]bool{
	OpSelf:     true,
	OpConcat:   true,
	OpAppend:   true,
	OpSetList:  true,
	OpGo:       true,
	OpMakeChan: true,
	OpSend:     true,
	OpRecv:     true,
	OpClosure:  true,
	OpClose:    true,
	OpVararg:   true,
	OpPow:      true,
	OpTForCall: true,
	OpTForLoop: true,
}

var opTier2LoopAllocationBlockerPolicies = [...]bool{
	OpSetList: true,
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
