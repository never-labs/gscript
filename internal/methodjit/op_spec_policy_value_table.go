package methodjit

var opTableArrayGPRInvariantPolicies = [...]bool{
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
	OpTableShapeID:     true,
	OpMatrixFlat:       true,
	OpMatrixStride:     true,
}

var opTableArrayGPRInvariantRankPolicies = [...]uint8{
	OpTableArrayData:   1,
	OpMatrixFlat:       1,
	OpTableArrayHeader: 3,
}

var opTableArrayGPRInvariantUseMaskPolicies = [...]uint8{
	OpTableArrayLoad:       1<<0 | 1<<1,
	OpTableArrayNestedLoad: 1<<0 | 1<<1 | 1<<2,
	OpTableArrayStore:      1<<1 | 1<<2 | 1<<5,
	OpTableArraySwap:       1<<1 | 1<<2,
	OpMatrixRowPtr:         1<<0 | 1<<1,
	OpMatrixLoadFAt:        1<<0 | 1<<1,
	OpMatrixStoreFAt:       1<<0 | 1<<1,
}

var opTableArrayKeyArgIndexPolicies = [...]uint8{
	OpTableArrayLoad:       3,
	OpTableArrayStore:      4,
	OpTableArraySwap:       2,
	OpTableArraySwapPairs:  2,
	OpTableArrayNestedLoad: 4,
}

var opTableArrayTableArgIndexPolicies = [...]uint8{
	OpTableArrayStore: 1,
}

var opTableArrayDataArgIndexPolicies = [...]uint8{
	OpTableArrayLoad:  1,
	OpTableArrayStore: 2,
}

var opTableArrayLenArgIndexPolicies = [...]uint8{
	OpTableArrayLoad:  2,
	OpTableArrayStore: 3,
}

var opTableIntArraySwapPairsBodyBenignPolicies = [...]bool{
	OpAddInt:         true,
	OpGuardTableKind: true,
	OpJump:           true,
	OpNop:            true,
}

var opTableIntArrayCopyPrefixBodyBenignPolicies = [...]bool{
	OpAddInt:         true,
	OpGuardTableKind: true,
	OpJump:           true,
}

var opTableIntArrayReverseBodyBenignPolicies = [...]bool{
	OpAddInt:         true,
	OpSubInt:         true,
	OpGuardTableKind: true,
	OpJump:           true,
	OpNop:            true,
}

var opFixedShapeArrayElementWriteRolePolicies = [...]OpFixedShapeArrayElementWriteRole{
	OpSetTable: OpFixedShapeArrayElementWriteSingle,
	OpSetList:  OpFixedShapeArrayElementWriteVariadic,
	OpAppend:   OpFixedShapeArrayElementWriteConflict,
}

var opFixedShapeArrayElementReadRolePolicies = [...]OpFixedShapeArrayElementReadRole{
	OpGetTable:       OpFixedShapeArrayElementReadDirect,
	OpTableArrayLoad: OpFixedShapeArrayElementReadLoweredArray,
}

var opFixedShapeReturnArrayElementRolePolicies = [...]OpFixedShapeReturnArrayElementRole{
	OpSetTable: OpFixedShapeReturnArrayElementStore,
	OpAppend:   OpFixedShapeReturnArrayElementInvalidator,
	OpSetList:  OpFixedShapeReturnArrayElementInvalidator,
	OpSetField: OpFixedShapeReturnArrayElementInvalidator,
}

var opLocalStringArrayTableUseRolePolicies = [...]OpLocalStringArrayTableUseRole{
	OpSetTable:         OpLocalStringArrayTableUseStore,
	OpLen:              OpLocalStringArrayTableUseRead,
	OpTableArrayHeader: OpLocalStringArrayTableUseRead,
}

var opLocalStringArrayTableArgIndexPolicies = [...]uint8{
	OpSetTable:         1,
	OpLen:              1,
	OpTableArrayHeader: 1,
}

var opReadonlyTableParamUseRolePolicies = [...]OpReadonlyTableParamUseRole{
	OpGetTable:                   OpReadonlyTableParamUseBenign,
	OpLen:                        OpReadonlyTableParamUseBenign,
	OpReturn:                     OpReadonlyTableParamUseBenign,
	OpSetTable:                   OpReadonlyTableParamUseFirstArgMutation,
	OpSetField:                   OpReadonlyTableParamUseFirstArgMutation,
	OpFieldStore:                 OpReadonlyTableParamUseFirstArgMutation,
	OpSetList:                    OpReadonlyTableParamUseFirstArgMutation,
	OpAppend:                     OpReadonlyTableParamUseFirstArgMutation,
	OpTableArrayStore:            OpReadonlyTableParamUseFirstArgMutation,
	OpTableArraySwap:             OpReadonlyTableParamUseFirstArgMutation,
	OpTableArraySwapPairs:        OpReadonlyTableParamUseFirstArgMutation,
	OpTableBoolArrayFill:         OpReadonlyTableParamUseFirstArgMutation,
	OpTableIntArrayReversePrefix: OpReadonlyTableParamUseFirstArgMutation,
	OpTableIntArrayCopyPrefix:    OpReadonlyTableParamUseFirstArgMutation,
	OpCall:                       OpReadonlyTableParamUseCallEscape,
	OpCallFloor:                  OpReadonlyTableParamUseCallEscape,
	OpFieldCallFloor:             OpReadonlyTableParamUseCallEscape,
	OpSelf:                       OpReadonlyTableParamUseCallEscape,
}

var opInlineAllocationRolePolicies = [...]OpInlineAllocationRole{
	OpNewTable:      OpInlineAllocationDynamic,
	OpNewFixedTable: OpInlineAllocationFixed,
	OpSetField:      OpInlineAllocationFieldInit,
	OpSetList:       OpInlineAllocationArrayInit,
}
