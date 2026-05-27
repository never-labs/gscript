package methodjit

var opConstPoolUserPolicies = [...]bool{
	OpConstString:             true,
	OpStringConstLookup:       true,
	OpStringFormatInt:         true,
	OpStringFormatConst:       true,
	OpStringFormatConstLen:    true,
	OpStringSplitPart:         true,
	OpStringSplitSubstr:       true,
	OpStringSplitSubstrNumber: true,
	OpGuardConstString:        true,
}

var opRawStringResultPolicies = [...]bool{
	OpConstString:          true,
	OpStringConstLookup:    true,
	OpStringFormatInt:      true,
	OpStringFormatConst:    true,
	OpStringFormatConstLen: true,
	OpStringSplitPart:      true,
	OpStringSplitSubstr:    true,
	OpGuardConstString:     true,
	OpGuardCalleeProto:     true,
}

var opDynamicStringQueryCacheKeyPolicies = [...]bool{
	OpConstString:       true,
	OpStringConstLookup: true,
	OpStringFormatInt:   true,
}

var opUnrollCloneablePolicies = [...]bool{
	OpConstInt:             true,
	OpConstFloat:           true,
	OpConstBool:            true,
	OpConstNil:             true,
	OpConstString:          true,
	OpAddInt:               true,
	OpSubInt:               true,
	OpMulInt:               true,
	OpModInt:               true,
	OpDivIntExact:          true,
	OpNegInt:               true,
	OpAddFloat:             true,
	OpSubFloat:             true,
	OpMulFloat:             true,
	OpDivFloat:             true,
	OpNegFloat:             true,
	OpSqrt:                 true,
	OpFloor:                true,
	OpFMA:                  true,
	OpFMSUB:                true,
	OpNumToFloat:           true,
	OpGuardType:            true,
	OpGuardIntRange:        true,
	OpGuardNonNil:          true,
	OpGuardTruthy:          true,
	OpMatrixLoadFAt:        true,
	OpMatrixLoadFRow:       true,
	OpMatrixLoadFRowConst:  true,
	OpTableArrayLoad:       true,
	OpTableArrayNestedLoad: true,
}

var opNestedFloatPhiOverrideSafePolicies = [...]bool{
	OpConstInt:                 true,
	OpConstFloat:               true,
	OpConstBool:                true,
	OpLoadSlot:                 true,
	OpPhi:                      true,
	OpAdd:                      true,
	OpSub:                      true,
	OpAddInt:                   true,
	OpSubInt:                   true,
	OpMulInt:                   true,
	OpNegInt:                   true,
	OpAddFloat:                 true,
	OpSubFloat:                 true,
	OpMulFloat:                 true,
	OpDivFloat:                 true,
	OpNegFloat:                 true,
	OpNumToFloat:               true,
	OpSqrt:                     true,
	OpFMA:                      true,
	OpFMSUB:                    true,
	OpLtInt:                    true,
	OpLeInt:                    true,
	OpEqInt:                    true,
	OpLtFloat:                  true,
	OpLeFloat:                  true,
	OpGuardType:                true,
	OpGuardIntRange:            true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpGuardTruthy:              true,
	OpJump:                     true,
	OpBranch:                   true,
	OpReturn:                   true,
}

var opFloatReductionWideUnrollBarrierPolicies = [...]bool{
	OpDivFloat: true,
	OpFMA:      true,
	OpFMSUB:    true,
	OpSqrt:     true,
	OpFloor:    true,
}

var opFloatReductionLatencyUnrollSeedPolicies = [...]bool{
	OpSqrt: true,
}

var opFloatReductionLatencyUnrollBlockPolicies = [...]bool{
	OpDivFloat: true,
	OpFloor:    true,
}

var opFloatReductionDivOpPolicies = [...]bool{
	OpDivFloat: true,
}

var opConstantPhiBranchThreadPurePolicies = [...]bool{
	OpPhi:       true,
	OpConstInt:  true,
	OpConstBool: true,
	OpEqInt:     true,
	OpNot:       true,
}

func applyOpSpecStringUnrollPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opConstPoolUserPolicies) {
		spec.ConstPoolUser = opConstPoolUserPolicies[op]
	}
	if int(op) < len(opRawStringResultPolicies) {
		spec.RawStringResult = opRawStringResultPolicies[op]
	}
	if int(op) < len(opDynamicStringQueryCacheKeyPolicies) {
		spec.DynamicStringQueryCacheKey = opDynamicStringQueryCacheKeyPolicies[op]
	}
	if int(op) < len(opUnrollCloneablePolicies) {
		spec.UnrollCloneable = opUnrollCloneablePolicies[op]
	}
	if int(op) < len(opNestedFloatPhiOverrideSafePolicies) {
		spec.NestedFloatPhiOverrideSafe = opNestedFloatPhiOverrideSafePolicies[op]
	}
	if int(op) < len(opFloatReductionWideUnrollBarrierPolicies) {
		spec.FloatReductionWideUnrollBarrier = opFloatReductionWideUnrollBarrierPolicies[op]
	}
	if int(op) < len(opFloatReductionLatencyUnrollSeedPolicies) {
		spec.FloatReductionLatencyUnrollSeed = opFloatReductionLatencyUnrollSeedPolicies[op]
	}
	if int(op) < len(opFloatReductionLatencyUnrollBlockPolicies) {
		spec.FloatReductionLatencyUnrollBlock = opFloatReductionLatencyUnrollBlockPolicies[op]
	}
	if int(op) < len(opFloatReductionDivOpPolicies) {
		spec.FloatReductionDivOp = opFloatReductionDivOpPolicies[op]
	}
	if int(op) < len(opConstantPhiBranchThreadPurePolicies) {
		spec.ConstantPhiBranchThreadPure = opConstantPhiBranchThreadPurePolicies[op]
	}
}
