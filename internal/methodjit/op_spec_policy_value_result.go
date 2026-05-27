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
