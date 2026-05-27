package methodjit

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
