package methodjit

var opRuntimeOverflowBoxablePolicies = [...]bool{
	OpAddInt: true,
	OpSubInt: true,
	OpMulInt: true,
	OpNegInt: true,
}

var opRuntimeGuardRefreshablePolicies = [...]bool{
	OpGuardType:        true,
	OpGuardCalleeProto: true,
	OpGuardConstString: true,
	OpGuardTableKind:   true,
	OpGuardIntRange:    true,
}

var opNativeNumericValueProducerPolicies = [...]bool{
	OpConstInt:   true,
	OpConstFloat: true,
	OpUnboxInt:   true,
	OpUnboxFloat: true,
	OpAdd:        true,
	OpSub:        true,
	OpMul:        true,
	OpDiv:        true,
	OpMod:        true,
	OpUnm:        true,
	OpAddInt:     true,
	OpSubInt:     true,
	OpMulInt:     true,
	OpModInt:     true,
	OpNegInt:     true,
	OpAddFloat:   true,
	OpSubFloat:   true,
	OpMulFloat:   true,
	OpDivFloat:   true,
	OpNegFloat:   true,
	OpFloor:      true,
}

var opPureNumericUnknownValuePolicies = [...]bool{
	OpAdd:         true,
	OpSub:         true,
	OpMul:         true,
	OpDiv:         true,
	OpMod:         true,
	OpUnm:         true,
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpModInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
	OpAddFloat:    true,
	OpSubFloat:    true,
	OpMulFloat:    true,
	OpDivFloat:    true,
	OpNegFloat:    true,
	OpNumToFloat:  true,
	OpPhi:         true,
	OpLoadSlot:    true,
}

var opTableArraySwapPureBetweenPolicies = [...]bool{
	OpConstInt:         true,
	OpConstFloat:       true,
	OpConstBool:        true,
	OpConstNil:         true,
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
	OpAddInt:           true,
	OpSubInt:           true,
	OpMulInt:           true,
	OpNegInt:           true,
	OpBoxInt:           true,
	OpUnboxInt:         true,
	OpGuardTableKind:   true,
	OpNop:              true,
}

var opStaticTableLenBenignUsePolicies = [...]bool{
	OpLen:              true,
	OpGetTable:         true,
	OpTableArrayHeader: true,
}

var opStaticTableLenBuilderPolicies = [...]bool{
	OpNewTable: true,
}

var opStaticTableLenInitializerPolicies = [...]bool{
	OpSetList: true,
}

var opStaticTableLenInvalidatorPolicies = [...]bool{
	OpSetTable: true,
	OpAppend:   true,
}
