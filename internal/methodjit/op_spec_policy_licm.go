package methodjit

var opLICMHoistablePolicies = [...]bool{
	OpConstInt:            true,
	OpConstFloat:          true,
	OpConstBool:           true,
	OpConstNil:            true,
	OpLoadSlot:            true,
	OpGetField:            true,
	OpGetGlobal:           true,
	OpGuardGlobalConst:    true,
	OpGetUpval:            true,
	OpSqrt:                true,
	OpFloor:               true,
	OpLen:                 true,
	OpGetTable:            true,
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpFMA:                 true,
	OpFMSUB:               true,
	OpAddInt:              true,
	OpSubInt:              true,
	OpMulInt:              true,
	OpDivIntExact:         true,
	OpNegInt:              true,
	OpLtInt:               true,
	OpLeInt:               true,
	OpEqInt:               true,
	OpModZeroInt:          true,
	OpLtFloat:             true,
	OpLeFloat:             true,
	OpEqString:            true,
	OpNot:                 true,
	OpGuardType:           true,
	OpGuardIntRange:       true,
	OpGuardCalleeProto:    true,
	OpNumToFloat:          true,
	OpTableShapeID:        true,
	OpFieldSvals:          true,
	OpFieldLoad:           true,
	OpFieldLoadNumToFloat: true,
	OpMatrixFlat:          true,
	OpMatrixStride:        true,
	OpTableArrayHeader:    true,
	OpTableArrayLen:       true,
	OpTableArrayData:      true,
	OpMatrixRowPtr:        true,
}

var opLICMInterestingMissPolicies = [...]bool{
	OpGetField:         true,
	OpGetTable:         true,
	OpGetGlobal:        true,
	OpGuardGlobalConst: true,
	OpGuardCalleeProto: true,
	OpGetUpval:         true,
	OpLoadSlot:         true,
	OpAdd:              true,
	OpSub:              true,
	OpMul:              true,
	OpDiv:              true,
	OpMod:              true,
	OpUnm:              true,
	OpAddInt:           true,
	OpSubInt:           true,
	OpMulInt:           true,
	OpModInt:           true,
	OpDivIntExact:      true,
	OpNegInt:           true,
	OpAddFloat:         true,
	OpSubFloat:         true,
	OpMulFloat:         true,
	OpDivFloat:         true,
	OpNegFloat:         true,
	OpFMA:              true,
	OpFMSUB:            true,
	OpMatrixFlat:       true,
	OpMatrixStride:     true,
	OpMatrixRowPtr:     true,
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
	OpSqrt:             true,
	OpFloor:            true,
	OpLen:              true,
	OpNumToFloat:       true,
}

var opLICMIntArithPolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
}

func applyOpSpecLICMPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opLICMHoistablePolicies) {
		spec.LICMHoistable = opLICMHoistablePolicies[op]
	}
	if int(op) < len(opLICMInterestingMissPolicies) {
		spec.LICMInterestingMiss = opLICMInterestingMissPolicies[op]
	}
	if int(op) < len(opLICMIntArithPolicies) {
		spec.LICMIntArith = opLICMIntArithPolicies[op]
	}
}
