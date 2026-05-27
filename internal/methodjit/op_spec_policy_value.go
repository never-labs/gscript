package methodjit

var opNoSSAResultPolicies = [...]bool{
	OpNop:                      true,
	OpStoreSlot:                true,
	OpSetTable:                 true,
	OpTableArrayStore:          true,
	OpTableArraySwap:           true,
	OpTableArraySwapPairs:      true,
	OpTableBoolArrayFill:       true,
	OpFieldStore:               true,
	OpSetField:                 true,
	OpSetList:                  true,
	OpAppend:                   true,
	OpGuardGlobalConst:         true,
	OpGuardTableKind:           true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpSetGlobal:                true,
	OpSetUpval:                 true,
	OpMatrixSetF:               true,
	OpMatrixStoreFAt:           true,
	OpMatrixStoreFRow:          true,
	OpMatrixStoreFRowConst:     true,
	OpClose:                    true,
	OpGo:                       true,
	OpSend:                     true,
}

var opRawIntResultPolicies = [...]bool{
	OpAddInt:        true,
	OpSubInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpDivIntExact:   true,
	OpNegInt:        true,
	OpTableArrayLen: true,
	OpTableShapeID:  true,
}

var opRawTablePtrResultPolicies = [...]bool{
	OpTableArrayHeader: true,
}

var opRawDataPtrResultPolicies = [...]bool{
	OpTableArrayData: true,
	OpFieldSvals:     true,
}

var opRawFloatResultPolicies = [...]bool{
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpNumToFloat:          true,
	OpGetFieldNumToFloat:  true,
	OpFieldLoadNumToFloat: true,
	OpSqrt:                true,
	OpFMA:                 true,
	OpFMSUB:               true,
}

var opMatrixNativePolicies = [...]bool{
	OpMatrixDense:          true,
	OpMatrixGetF:           true,
	OpMatrixSetF:           true,
	OpMatrixFlat:           true,
	OpMatrixStride:         true,
	OpMatrixLoadFAt:        true,
	OpMatrixStoreFAt:       true,
	OpMatrixRowPtr:         true,
	OpMatrixLoadFRow:       true,
	OpMatrixStoreFRow:      true,
	OpMatrixLoadFRowConst:  true,
	OpMatrixStoreFRowConst: true,
}

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
