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
