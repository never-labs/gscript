package methodjit

var opLoadElimConstCSEPolicies = [...]bool{
	OpConstInt:    true,
	OpConstFloat:  true,
	OpConstBool:   true,
	OpConstNil:    true,
	OpConstString: true,
}

var opLiteralConstPolicies = [...]bool{
	OpConstInt:    true,
	OpConstFloat:  true,
	OpConstBool:   true,
	OpConstNil:    true,
	OpConstString: true,
}

var opLoadElimPureCSEPolicies = [...]bool{
	OpAddInt:       true,
	OpSubInt:       true,
	OpMulInt:       true,
	OpModInt:       true,
	OpDivIntExact:  true,
	OpNegInt:       true,
	OpAddFloat:     true,
	OpSubFloat:     true,
	OpMulFloat:     true,
	OpDivFloat:     true,
	OpNegFloat:     true,
	OpNumToFloat:   true,
	OpSqrt:         true,
	OpFloor:        true,
	OpFMA:          true,
	OpFMSUB:        true,
	OpEqInt:        true,
	OpLtInt:        true,
	OpLeInt:        true,
	OpModZeroInt:   true,
	OpLtFloat:      true,
	OpLeFloat:      true,
	OpEqString:     true,
	OpTableShapeID: true,
}

var opLoadElimShapeFactKillerPolicies = [...]bool{
	OpSetField:                   true,
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpAppend:                     true,
	OpSetList:                    true,
	OpCall:                       true,
	OpResume:                     true,
	OpSelf:                       true,
}

var opLoadElimDynamicTableCacheMutationPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpAppend:                     true,
	OpSetList:                    true,
}

var opLoadElimTypedArrayFactMutationPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpAppend:                     true,
	OpSetList:                    true,
}

var opLoadElimTableCacheKeyArgIndexPolicies = [...]uint8{
	OpSetTable:        2,
	OpTableArrayStore: 4,
}

var opLoadElimTableCacheValueArgIndexPolicies = [...]uint8{
	OpSetTable:        3,
	OpTableArrayStore: 5,
}

var opLoadElimFactBarrierPolicies = [...]bool{
	OpCall:   true,
	OpResume: true,
	OpSelf:   true,
}
