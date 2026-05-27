package methodjit

var opBackendPolicies = [...]OpBackendPolicy{
	OpConstInt:                      OpBackendPreservesTableArrayBounds,
	OpConstFloat:                    OpBackendPreservesTableArrayBounds,
	OpConstBool:                     OpBackendPreservesTableArrayBounds,
	OpConstNil:                      OpBackendPreservesTableArrayBounds,
	OpConstString:                   OpBackendPreservesTableArrayBounds,
	OpLoadSlot:                      OpBackendPreservesTableArrayBounds,
	OpStoreSlot:                     OpBackendPreservesTableArrayBounds,
	OpAdd:                           OpBackendPreservesTableArrayBounds,
	OpSub:                           OpBackendPreservesTableArrayBounds,
	OpMul:                           OpBackendPreservesTableArrayBounds,
	OpDiv:                           OpBackendPreservesTableArrayBounds,
	OpMod:                           OpBackendPreservesTableArrayBounds,
	OpUnm:                           OpBackendPreservesTableArrayBounds,
	OpNot:                           OpBackendPreservesTableArrayBounds,
	OpLen:                           OpBackendClearsTableArrayBounds,
	OpAddInt:                        OpBackendPreservesTableArrayBounds,
	OpSubInt:                        OpBackendPreservesTableArrayBounds,
	OpMulInt:                        OpBackendPreservesTableArrayBounds,
	OpModInt:                        OpBackendPreservesTableArrayBounds,
	OpNegInt:                        OpBackendPreservesTableArrayBounds,
	OpAddFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpSubFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpMulFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpDivFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpNegFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpSqrt:                          OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpFloor:                         OpBackendPreservesTableArrayBounds,
	OpFMA:                           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpFMSUB:                         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpComplexEscapeInSet:            OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpComplexEscapeRowCount:         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpRecordArrayLoopSpecialization: OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpEq:                            OpBackendPreservesTableArrayBounds,
	OpLt:                            OpBackendPreservesTableArrayBounds,
	OpLe:                            OpBackendPreservesTableArrayBounds,
	OpEqInt:                         OpBackendPreservesTableArrayBounds,
	OpLtInt:                         OpBackendPreservesTableArrayBounds,
	OpLeInt:                         OpBackendPreservesTableArrayBounds,
	OpModZeroInt:                    OpBackendPreservesTableArrayBounds,
	OpLtFloat:                       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache | OpBackendPreservesScratchFPRCache,
	OpLeFloat:                       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache | OpBackendPreservesScratchFPRCache,
	OpEqString:                      OpBackendPreservesTableArrayBounds,
	OpStringFormatConstLen:          OpBackendPreservesTableArrayBounds | OpBackendClearsTableArrayBounds,
	OpNewTable:                      0,
	OpSetTable:                      OpBackendClearsTableArrayBounds | OpBackendInvalidatesShape,
	OpTableArrayHeader:              OpBackendPreservesTableArrayBounds,
	OpTableArrayLen:                 OpBackendPreservesTableArrayBounds,
	OpTableArrayData:                OpBackendPreservesTableArrayBounds,
	OpTableArrayLoad:                OpBackendPreservesTableArrayBounds,
	OpTableShapeID:                  OpBackendPreservesTableArrayBounds,
	OpTableArrayStore:               OpBackendPreservesTableArrayBounds,
	OpTableArraySwap:                OpBackendPreservesTableArrayBounds,
	OpTableArraySwapPairs:           OpBackendPreservesTableArrayBounds | OpBackendClearsTableArrayBounds,
	OpTableBoolArrayFill:            OpBackendClearsTableArrayBounds,
	OpTableIntArrayReversePrefix:    OpBackendClearsTableArrayBounds,
	OpTableIntArrayCopyPrefix:       OpBackendClearsTableArrayBounds,
	OpGetField:                      OpBackendPreservesFieldSvalsCache,
	OpGetFieldNumToFloat:            OpBackendPreservesFieldSvalsCache,
	OpFieldPolyLen:                  OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpSetField:                      OpBackendPreservesFieldSvalsCache | OpBackendClearsTableArrayBounds,
	OpGetGlobal:                     OpBackendClearsTableArrayBounds,
	OpSetGlobal:                     OpBackendClearsTableArrayBounds,
	OpGetUpval:                      OpBackendClearsTableArrayBounds,
	OpSetUpval:                      OpBackendClearsTableArrayBounds,
	OpBoxInt:                        OpBackendPreservesTableArrayBounds,
	OpBoxFloat:                      OpBackendPreservesTableArrayBounds,
	OpUnboxInt:                      OpBackendPreservesTableArrayBounds,
	OpUnboxFloat:                    OpBackendPreservesTableArrayBounds,
	OpNumToFloat:                    OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardType:                     OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardIntRange:                 OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardGlobalConst:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardConstString:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardTableKind:                OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardCalleeProto:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardFieldCalleeProto:         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldType:           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldTypeMask:       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldVMClosure:      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardTruthy:                   OpBackendPreservesTableArrayBounds,
	OpCall:                          OpBackendClearsTableArrayBounds,
	OpCallFloor:                     OpBackendClearsTableArrayBounds,
	OpFieldCallFloor:                OpBackendClearsTableArrayBounds,
	OpResume:                        OpBackendClearsTableArrayBounds,
	OpSelf:                          OpBackendClearsTableArrayBounds | OpBackendInvalidatesShape,
	OpForPrep:                       OpBackendClearsTableArrayBounds,
	OpForLoop:                       OpBackendClearsTableArrayBounds,
	OpTForCall:                      OpBackendClearsTableArrayBounds,
	OpTForLoop:                      OpBackendClearsTableArrayBounds,
	OpClosure:                       OpBackendClearsTableArrayBounds,
	OpClose:                         OpBackendClearsTableArrayBounds,
	OpVararg:                        OpBackendClearsTableArrayBounds,
	OpTestSet:                       OpBackendClearsTableArrayBounds,
	OpGo:                            OpBackendClearsTableArrayBounds,
	OpMakeChan:                      OpBackendClearsTableArrayBounds,
	OpSend:                          OpBackendClearsTableArrayBounds,
	OpRecv:                          OpBackendClearsTableArrayBounds,
	OpNop:                           OpBackendPreservesTableArrayBounds | OpBackendPreservesScratchFPRCache,
	OpConcat:                        OpBackendClearsTableArrayBounds,
	OpStringConstLookup:             OpBackendClearsTableArrayBounds,
	OpStringFormatInt:               OpBackendClearsTableArrayBounds,
	OpStringFormatConst:             OpBackendClearsTableArrayBounds,
	OpStringSplitPart:               OpBackendClearsTableArrayBounds,
	OpStringSplitSubstr:             OpBackendClearsTableArrayBounds,
	OpStringSplitSubstrNumber:       OpBackendClearsTableArrayBounds,
	OpAppend:                        OpBackendClearsTableArrayBounds,
	OpPow:                           OpBackendClearsTableArrayBounds,
	OpGuardNonNil:                   OpBackendClearsTableArrayBounds,
}

var opKeepUnusedPolicies = [...]bool{
	OpJump:                          true,
	OpBranch:                        true,
	OpReturn:                        true,
	OpStoreSlot:                     true,
	OpSetGlobal:                     true,
	OpSetUpval:                      true,
	OpSetTable:                      true,
	OpTableArrayStore:               true,
	OpTableArraySwap:                true,
	OpTableArraySwapPairs:           true,
	OpTableBoolArrayFill:            true,
	OpTableIntArrayReversePrefix:    true,
	OpTableIntArrayCopyPrefix:       true,
	OpRecordArrayLoopSpecialization: true,
	OpFieldStore:                    true,
	OpSetField:                      true,
	OpSetList:                       true,
	OpAppend:                        true,
	OpMatrixSetF:                    true,
	OpMatrixStoreFAt:                true,
	OpMatrixStoreFRow:               true,
	OpMatrixStoreFRowConst:          true,
	OpCall:                          true,
	OpCallFloor:                     true,
	OpFieldCallFloor:                true,
	OpResume:                        true,
	OpYield:                         true,
	OpSelf:                          true,
	OpGuardType:                     true,
	OpGuardIntRange:                 true,
	OpGuardGlobalConst:              true,
	OpGuardConstString:              true,
	OpGuardTableKind:                true,
	OpGuardCalleeProto:              true,
	OpGuardFieldCalleeProto:         true,
	OpGuardShapeFieldType:           true,
	OpGuardShapeFieldTypeMask:       true,
	OpGuardShapeFieldVMClosure:      true,
	OpGuardNonNil:                   true,
	OpGuardTruthy:                   true,
	OpForPrep:                       true,
	OpForLoop:                       true,
	OpTForCall:                      true,
	OpTForLoop:                      true,
	OpClosure:                       true,
	OpClose:                         true,
	OpGo:                            true,
	OpMakeChan:                      true,
	OpSend:                          true,
	OpRecv:                          true,
}

var opNativeReplayMayExitPolicies = [...]bool{
	OpCall:                       true,
	OpCallFloor:                  true,
	OpFieldCallFloor:             true,
	OpSelf:                       true,
	OpNewTable:                   true,
	OpNewFixedTable:              true,
	OpGetTable:                   true,
	OpSetTable:                   true,
	OpTableArrayHeader:           true,
	OpTableArrayLen:              true,
	OpTableArrayData:             true,
	OpTableArrayLoad:             true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableBoolArrayCount:        true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpTableArrayNestedLoad:       true,
	OpGetField:                   true,
	OpGetFieldNumToFloat:         true,
	OpFieldPolyLen:               true,
	OpSetField:                   true,
	OpFieldStore:                 true,
	OpSetList:                    true,
	OpAppend:                     true,
	OpGetGlobal:                  true,
	OpSetGlobal:                  true,
	OpGetUpval:                   true,
	OpSetUpval:                   true,
	OpConstString:                true,
	OpConcat:                     true,
	OpStringConstLookup:          true,
	OpStringFormatInt:            true,
	OpStringFormatConst:          true,
	OpStringFormatConstLen:       true,
	OpGetTableStringFormatInt:    true,
	OpStringSplitPart:            true,
	OpStringSplitSubstr:          true,
	OpStringSplitSubstrNumber:    true,
	OpLen:                        true,
	OpPow:                        true,
	OpFloor:                      true,
	OpClosure:                    true,
	OpClose:                      true,
	OpVararg:                     true,
	OpTForCall:                   true,
	OpTForLoop:                   true,
	OpGo:                         true,
	OpMakeChan:                   true,
	OpSend:                       true,
	OpRecv:                       true,
	OpGuardType:                  true,
	OpGuardIntRange:              true,
	OpGuardGlobalConst:           true,
	OpGuardConstString:           true,
	OpGuardTableKind:             true,
	OpGuardCalleeProto:           true,
	OpGuardFieldCalleeProto:      true,
	OpGuardShapeFieldType:        true,
	OpGuardShapeFieldTypeMask:    true,
	OpGuardShapeFieldVMClosure:   true,
	OpGuardNonNil:                true,
	OpGuardTruthy:                true,
	OpNumToFloat:                 true,
	OpDivIntExact:                true,
	OpMatrixDense:                true,
	OpMatrixGetF:                 true,
	OpMatrixSetF:                 true,
	OpMatrixFlat:                 true,
	OpMatrixStride:               true,
	OpAddInt:                     true,
	OpSubInt:                     true,
	OpMulInt:                     true,
	OpNegInt:                     true,
	OpModInt:                     true,
	OpModZeroInt:                 true,
}

var opNativeReplayVisibleSideEffectPolicies = [...]bool{
	OpSetGlobal:            true,
	OpSetUpval:             true,
	OpMatrixSetF:           true,
	OpMatrixStoreFAt:       true,
	OpMatrixStoreFRow:      true,
	OpMatrixStoreFRowConst: true,
	OpClose:                true,
	OpGo:                   true,
	OpSend:                 true,
	OpRecv:                 true,
}

var opNativeReplayVisibleTableMutationPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpSetField:                   true,
	OpFieldStore:                 true,
	OpSetList:                    true,
	OpAppend:                     true,
}

var opNativeCalleeResumeUnsafePolicies = [...]bool{
	OpSetGlobal: true,
	OpSetUpval:  true,
	OpClose:     true,
	OpGo:        true,
	OpSend:      true,
	OpRecv:      true,
}

var opRestartVisibleSideEffectPolicies = [...]bool{
	OpCall:                true,
	OpSetGlobal:           true,
	OpSetTable:            true,
	OpTableArrayStore:     true,
	OpTableArraySwap:      true,
	OpTableArraySwapPairs: true,
	OpSetField:            true,
	OpNewTable:            true,
	OpNewFixedTable:       true,
	OpSetList:             true,
	OpAppend:              true,
	OpSelf:                true,
	OpSetUpval:            true,
	OpGo:                  true,
	OpMakeChan:            true,
	OpSend:                true,
	OpRecv:                true,
	OpClosure:             true,
	OpClose:               true,
	OpVararg:              true,
	OpConcat:              true,
	OpLen:                 true,
	OpPow:                 true,
	OpTForCall:            true,
	OpTForLoop:            true,
}

var opFieldShapeSplitInlineSafePolicies = [...]bool{
	OpConstInt:              true,
	OpConstFloat:            true,
	OpConstBool:             true,
	OpConstNil:              true,
	OpConstString:           true,
	OpAddInt:                true,
	OpSubInt:                true,
	OpMulInt:                true,
	OpModInt:                true,
	OpNegInt:                true,
	OpAddFloat:              true,
	OpSubFloat:              true,
	OpMulFloat:              true,
	OpDivFloat:              true,
	OpNegFloat:              true,
	OpEqInt:                 true,
	OpLtInt:                 true,
	OpLeInt:                 true,
	OpEqString:              true,
	OpLtFloat:               true,
	OpLeFloat:               true,
	OpFloor:                 true,
	OpNumToFloat:            true,
	OpFieldSvals:            true,
	OpFieldLoad:             true,
	OpFieldLoadNumToFloat:   true,
	OpFieldPolyLen:          true,
	OpGuardType:             true,
	OpGuardIntRange:         true,
	OpGuardCalleeProto:      true,
	OpGuardFieldCalleeProto: true,
	OpBranch:                true,
	OpJump:                  true,
	OpPhi:                   true,
	OpFieldStore:            true,
	OpTableArrayHeader:      true,
	OpTableArrayLen:         true,
	OpTableArrayData:        true,
	OpTableArrayLoad:        true,
	OpTableArrayStore:       true,
}

var opFieldShapePreEffectInlineSafePolicies = [...]bool{
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
	OpGetTable:           true,
	OpSetTable:           true,
	OpAdd:                true,
	OpSub:                true,
	OpMul:                true,
	OpDiv:                true,
	OpMod:                true,
	OpUnm:                true,
	OpLen:                true,
	OpFloor:              true,
	OpNumToFloat:         true,
}

var opFieldShapeInlineSideEffectPolicies = [...]bool{
	OpFieldStore:      true,
	OpTableArrayStore: true,
	OpSetField:        true,
	OpSetTable:        true,
}

var opFieldShapePostEffectInlineUnsafePolicies = [...]bool{
	OpSetField:           true,
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
	OpSetTable:           true,
	OpGetTable:           true,
	OpCall:               true,
	OpCallFloor:          true,
	OpFieldCallFloor:     true,
	OpResume:             true,
	OpYield:              true,
	OpSelf:               true,
}

var opGlobalConstUnsafePolicies = [...]bool{
	OpCall:      true,
	OpResume:    true,
	OpYield:     true,
	OpSelf:      true,
	OpSetGlobal: true,
	OpSetUpval:  true,
	OpGo:        true,
	OpSend:      true,
	OpRecv:      true,
}

var opNestedCallLikePolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpTForCall:       true,
	OpGo:             true,
}

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

var opPureNumericInlinePolicies = [...]bool{
	OpConstInt:                 true,
	OpConstFloat:               true,
	OpLoadSlot:                 true,
	OpAdd:                      true,
	OpSub:                      true,
	OpMul:                      true,
	OpDiv:                      true,
	OpMod:                      true,
	OpUnm:                      true,
	OpAddInt:                   true,
	OpSubInt:                   true,
	OpMulInt:                   true,
	OpModInt:                   true,
	OpDivIntExact:              true,
	OpNegInt:                   true,
	OpAddFloat:                 true,
	OpSubFloat:                 true,
	OpMulFloat:                 true,
	OpDivFloat:                 true,
	OpNegFloat:                 true,
	OpNumToFloat:               true,
	OpSqrt:                     true,
	OpFloor:                    true,
	OpFMA:                      true,
	OpFMSUB:                    true,
	OpEq:                       true,
	OpLt:                       true,
	OpLe:                       true,
	OpEqInt:                    true,
	OpLtInt:                    true,
	OpLeInt:                    true,
	OpLtFloat:                  true,
	OpLeFloat:                  true,
	OpEqString:                 true,
	OpModZeroInt:               true,
	OpTableShapeID:             true,
	OpGuardType:                true,
	OpGuardIntRange:            true,
	OpGuardConstString:         true,
	OpGuardTableKind:           true,
	OpGuardCalleeProto:         true,
	OpGuardFieldCalleeProto:    true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpJump:                     true,
	OpBranch:                   true,
	OpPhi:                      true,
}

var opNativeEffectLoopInlinePolicies = [...]bool{
	OpGetGlobal:            true,
	OpGuardGlobalConst:     true,
	OpTableArrayHeader:     true,
	OpTableArrayLen:        true,
	OpTableArrayData:       true,
	OpTableArrayLoad:       true,
	OpTableArrayNestedLoad: true,
	OpTableArrayStore:      true,
	OpFieldSvals:           true,
	OpFieldLoad:            true,
	OpFieldStore:           true,
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

var opDirectDeoptWithoutFullFlushPolicies = [...]bool{
	OpGuardType:                true,
	OpGuardIntRange:            true,
	OpGuardGlobalConst:         true,
	OpGuardConstString:         true,
	OpGuardTableKind:           true,
	OpGuardCalleeProto:         true,
	OpGuardFieldCalleeProto:    true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpNumToFloat:               true,
	OpDivIntExact:              true,
	OpGetFieldNumToFloat:       true,
	OpFieldPolyLen:             true,
	OpFieldSvals:               true,
	OpFieldLoad:                true,
	OpFieldLoadNumToFloat:      true,
	OpMatrixGetF:               true,
	OpMatrixSetF:               true,
	OpMatrixFlat:               true,
	OpMatrixStride:             true,
	OpTableArrayHeader:         true,
	OpTableArrayLoad:           true,
	OpTableShapeID:             true,
	OpTableArrayStore:          true,
	OpTableArraySwap:           true,
	OpTableArraySwapPairs:      true,
	OpTableArrayNestedLoad:     true,
}

var opGenericSpecializablePolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpDiv: true,
	OpMod: true,
	OpUnm: true,
	OpEq:  true,
	OpLt:  true,
	OpLe:  true,
}

type opTypeSpecializationPolicy struct {
	IntOp    Op
	FloatOp  Op
	StringOp Op
	Set      bool
}

var opTypeSpecializationPolicies = [...]opTypeSpecializationPolicy{
	OpAdd: {IntOp: OpAddInt, FloatOp: OpAddFloat, StringOp: OpMax, Set: true},
	OpSub: {IntOp: OpSubInt, FloatOp: OpSubFloat, StringOp: OpMax, Set: true},
	OpMul: {IntOp: OpMulInt, FloatOp: OpMulFloat, StringOp: OpMax, Set: true},
	OpMod: {IntOp: OpModInt, FloatOp: OpMax, StringOp: OpMax, Set: true},
	OpDiv: {IntOp: OpDivFloat, FloatOp: OpDivFloat, StringOp: OpMax, Set: true},
	OpUnm: {IntOp: OpNegInt, FloatOp: OpNegFloat, StringOp: OpMax, Set: true},
	OpEq:  {IntOp: OpEqInt, FloatOp: OpMax, StringOp: OpEqString, Set: true},
	OpLt:  {IntOp: OpLtInt, FloatOp: OpLtFloat, StringOp: OpMax, Set: true},
	OpLe:  {IntOp: OpLeInt, FloatOp: OpLeFloat, StringOp: OpMax, Set: true},
}

var opNumToFloatInsertCandidatePolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpDiv: true,
	OpLt:  true,
	OpLe:  true,
}

var opIntRecurrencePolicies = [...]bool{
	OpAdd:    true,
	OpSub:    true,
	OpMul:    true,
	OpMod:    true,
	OpAddInt: true,
	OpSubInt: true,
	OpMulInt: true,
	OpModInt: true,
}

var opNumericOperandPolicies = [...]bool{
	OpAdd:      true,
	OpSub:      true,
	OpMul:      true,
	OpDiv:      true,
	OpMod:      true,
	OpUnm:      true,
	OpAddInt:   true,
	OpSubInt:   true,
	OpMulInt:   true,
	OpModInt:   true,
	OpNegInt:   true,
	OpAddFloat: true,
	OpSubFloat: true,
	OpMulFloat: true,
	OpDivFloat: true,
	OpNegFloat: true,
	OpLt:       true,
	OpLe:       true,
	OpLtInt:    true,
	OpLeInt:    true,
	OpLtFloat:  true,
	OpLeFloat:  true,
}

var opFieldSvalsCrossBlockBarrierPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpSelf:           true,
	OpSetGlobal:      true,
	OpSetUpval:       true,
	OpSetTable:       true,
	OpSetList:        true,
	OpAppend:         true,
}

var opFieldSvalsGlobalBarrierPolicies = [...]bool{
	OpSetTable:       true,
	OpSetList:        true,
	OpAppend:         true,
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpSelf:           true,
	OpSetGlobal:      true,
	OpSetUpval:       true,
}

var opFieldLenFoldBarrierPolicies = [...]bool{
	OpCall:                true,
	OpSetField:            true,
	OpFieldStore:          true,
	OpSetTable:            true,
	OpTableArrayStore:     true,
	OpTableArraySwap:      true,
	OpTableArraySwapPairs: true,
	OpSetGlobal:           true,
	OpSetUpval:            true,
	OpAppend:              true,
	OpSetList:             true,
}

var opFieldCallPolyLenFusionBarrierPolicies = [...]bool{
	OpSetField:                   true,
	OpSetTable:                   true,
	OpSetList:                    true,
	OpAppend:                     true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpCall:                       true,
	OpCallFloor:                  true,
	OpFieldCallFloor:             true,
	OpResume:                     true,
	OpYield:                      true,
	OpSelf:                       true,
	OpGo:                         true,
	OpSend:                       true,
	OpRecv:                       true,
	OpReturn:                     true,
	OpJump:                       true,
	OpBranch:                     true,
}

var opBoxableIntArithmeticPolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpModInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
}

var opUnsafeIntArithmeticCandidatePolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpNegInt:      true,
	OpDivIntExact: true,
}

var opInt48SafeRangeCandidatePolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpNegInt:      true,
	OpDivIntExact: true,
}

var opExactDivAllowedExternalUsePolicies = [...]bool{
	OpEq:            true,
	OpLt:            true,
	OpLe:            true,
	OpEqInt:         true,
	OpLtInt:         true,
	OpLeInt:         true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpBranch:        true,
}

var opNonNegativeDerivationCandidatePolicies = [...]bool{
	OpConstInt:      true,
	OpLen:           true,
	OpTableArrayLen: true,
	OpGuardIntRange: true,
	OpAddInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpDivIntExact:   true,
	OpPhi:           true,
	OpBoxInt:        true,
	OpUnboxInt:      true,
}

var opNonNegativeDerivationKindPolicies = [...]OpNonNegativeDerivationKind{
	OpConstInt:      OpNonNegativeConstIntAux,
	OpLen:           OpNonNegativeAlways,
	OpTableArrayLen: OpNonNegativeAlways,
	OpGuardIntRange: OpNonNegativeGuardRangeMin,
	OpAddInt:        OpNonNegativeBinaryAllArgs,
	OpMulInt:        OpNonNegativeBinaryAllArgs,
	OpModInt:        OpNonNegativeModuloDivisor,
	OpDivIntExact:   OpNonNegativeExactDivPositiveDivisor,
	OpPhi:           OpNonNegativeAllArgs,
	OpBoxInt:        OpNonNegativeForwardArg,
	OpUnboxInt:      OpNonNegativeForwardArg,
}

var opInt48RuntimeValuePolicies = [...]bool{
	OpConstInt:      true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpLoadSlot:      true,
	OpUnboxInt:      true,
}

var opFusableComparisonPolicies = [...]bool{
	OpEq:         true,
	OpLtInt:      true,
	OpLeInt:      true,
	OpEqInt:      true,
	OpModZeroInt: true,
	OpLtFloat:    true,
	OpLeFloat:    true,
}

var opLoopBoundComparisonPolicies = [...]bool{
	OpLtInt: true,
	OpLeInt: true,
	OpEqInt: true,
}

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

var opNeedsTier2FieldCachePolicies = [...]bool{
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
	OpSetField:           true,
}

var opFieldReadPolicies = [...]bool{
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
}

var opFieldSlotLoadPolicies = [...]bool{
	OpFieldLoad:           true,
	OpFieldLoadNumToFloat: true,
}

var opFieldWritePolicies = [...]bool{
	OpSetField:   true,
	OpFieldStore: true,
}

var opBoolTableFillBodyBenignPolicies = [...]bool{
	OpNop:       true,
	OpJump:      true,
	OpConstInt:  true,
	OpConstBool: true,
	OpConstNil:  true,
	OpAddInt:    true,
}

var opBoolTableFillStorePolicies = [...]bool{
	OpSetTable:        true,
	OpTableArrayStore: true,
}

var opBoolTableCountLoadBodyBenignPolicies = [...]bool{
	OpNop:         true,
	OpJump:        true,
	OpBranch:      true,
	OpGuardTruthy: true,
}

var opBoolTableCountLoadPolicies = [...]bool{
	OpTableArrayLoad: true,
}

var opBoolTableCountIncrementBenignPolicies = [...]bool{
	OpNop:      true,
	OpJump:     true,
	OpConstInt: true,
}

var opBoolTableCountIncrementPolicies = [...]bool{
	OpAdd:    true,
	OpAddInt: true,
}

var opCallResultRangeGuardCandidatePolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opModuloReducibleCallFloorPolicies = [...]bool{
	OpCallFloor:      true,
	OpFieldCallFloor: true,
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

type opCallUserArgStartPolicy struct {
	Start int
	Set   bool
}

var opCallUserArgStartPolicies = [...]opCallUserArgStartPolicy{
	OpCall:           {Start: 1, Set: true},
	OpCallFloor:      {Start: 1, Set: true},
	OpFieldCallFloor: {Start: 0, Set: true},
}

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

var opTableResultRawTablePtrPolicies = [...]bool{
	OpTableArrayLoad: true,
}

var opTableArrayRegionGlobalBarrierPolicies = [...]bool{
	OpCall:               true,
	OpCallFloor:          true,
	OpFieldCallFloor:     true,
	OpResume:             true,
	OpSelf:               true,
	OpSetTable:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

var opTableArrayRegionAliasingCallPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opTableArrayRegionAliasingAlwaysPolicies = [...]bool{
	OpResume: true,
	OpSelf:   true,
}

var opTableArrayRegionTableMutationPolicies = [...]bool{
	OpSetTable:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

var opTableMetatableMutationBarrierPolicies = [...]bool{
	OpCall:      true,
	OpSelf:      true,
	OpSetGlobal: true,
	OpSetUpval:  true,
	OpAppend:    true,
	OpSetList:   true,
	OpConcat:    true,
	OpPow:       true,
	OpClosure:   true,
	OpClose:     true,
	OpTForCall:  true,
	OpTForLoop:  true,
	OpVararg:    true,
	OpTestSet:   true,
	OpGo:        true,
	OpMakeChan:  true,
	OpSend:      true,
	OpRecv:      true,
}

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

var opFixedResultTypePolicies = [...]Type{
	OpConstInt:              TypeInt,
	OpConstFloat:            TypeFloat,
	OpConstBool:             TypeBool,
	OpConstNil:              TypeNil,
	OpConstString:           TypeString,
	OpAddInt:                TypeInt,
	OpSubInt:                TypeInt,
	OpMulInt:                TypeInt,
	OpModInt:                TypeInt,
	OpNegInt:                TypeInt,
	OpAddFloat:              TypeFloat,
	OpSubFloat:              TypeFloat,
	OpMulFloat:              TypeFloat,
	OpDivFloat:              TypeFloat,
	OpNegFloat:              TypeFloat,
	OpNumToFloat:            TypeFloat,
	OpSqrt:                  TypeFloat,
	OpFloor:                 TypeInt,
	OpEqInt:                 TypeBool,
	OpLtInt:                 TypeBool,
	OpLeInt:                 TypeBool,
	OpModZeroInt:            TypeBool,
	OpLtFloat:               TypeBool,
	OpLeFloat:               TypeBool,
	OpEqString:              TypeBool,
	OpComplexEscapeInSet:    TypeBool,
	OpComplexEscapeRowCount: TypeInt,
	OpEq:                    TypeBool,
	OpLt:                    TypeBool,
	OpLe:                    TypeBool,
	OpNot:                   TypeBool,
	OpLen:                   TypeInt,
	OpDiv:                   TypeFloat,
	OpGuardIntRange:         TypeInt,
	OpGetFieldNumToFloat:    TypeFloat,
	OpFieldLoadNumToFloat:   TypeFloat,
	OpNewTable:              TypeTable,
	OpNewFixedTable:         TypeTable,
	OpClosure:               TypeFunction,
}

var opProvesNonNilResultPolicies = [...]bool{
	OpConstInt:            true,
	OpConstFloat:          true,
	OpConstBool:           true,
	OpConstString:         true,
	OpAdd:                 true,
	OpSub:                 true,
	OpMul:                 true,
	OpDiv:                 true,
	OpMod:                 true,
	OpPow:                 true,
	OpAddInt:              true,
	OpSubInt:              true,
	OpMulInt:              true,
	OpModInt:              true,
	OpDivIntExact:         true,
	OpNegInt:              true,
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpSqrt:                true,
	OpFloor:               true,
	OpFMA:                 true,
	OpFMSUB:               true,
	OpNumToFloat:          true,
	OpGetFieldNumToFloat:  true,
	OpFieldLoadNumToFloat: true,
	OpLen:                 true,
	OpLtInt:               true,
	OpLeInt:               true,
	OpEqInt:               true,
	OpLtFloat:             true,
	OpLeFloat:             true,
	OpEqString:            true,
}

var opGuardProvenResultTypePolicies = [...]Type{
	OpConstInt:            TypeInt,
	OpAddInt:              TypeInt,
	OpSubInt:              TypeInt,
	OpMulInt:              TypeInt,
	OpModInt:              TypeInt,
	OpDivIntExact:         TypeInt,
	OpNegInt:              TypeInt,
	OpFloor:               TypeInt,
	OpConstFloat:          TypeFloat,
	OpAddFloat:            TypeFloat,
	OpSubFloat:            TypeFloat,
	OpMulFloat:            TypeFloat,
	OpDivFloat:            TypeFloat,
	OpNegFloat:            TypeFloat,
	OpNumToFloat:          TypeFloat,
	OpGetFieldNumToFloat:  TypeFloat,
	OpFieldLoadNumToFloat: TypeFloat,
	OpSqrt:                TypeFloat,
	OpFMA:                 TypeFloat,
	OpFMSUB:               TypeFloat,
	OpConstBool:           TypeBool,
	OpEqInt:               TypeBool,
	OpLtInt:               TypeBool,
	OpLeInt:               TypeBool,
	OpModZeroInt:          TypeBool,
	OpLtFloat:             TypeBool,
	OpLeFloat:             TypeBool,
	OpEqString:            TypeBool,
	OpEq:                  TypeBool,
	OpLt:                  TypeBool,
	OpLe:                  TypeBool,
	OpNot:                 TypeBool,
	OpConstNil:            TypeNil,
	OpConstString:         TypeString,
	OpNewTable:            TypeTable,
	OpNewFixedTable:       TypeTable,
	OpClosure:             TypeFunction,
}

var opRawFloatValueProducerPolicies = [...]bool{
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpSqrt:                true,
	OpFMA:                 true,
	OpFMSUB:               true,
	OpGetFieldNumToFloat:  true,
	OpFieldLoadNumToFloat: true,
	OpNumToFloat:          true,
}

var opFieldFactWideKillerPolicies = [...]bool{
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

var opTableMutationFirstArgPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpSetList:                    true,
	OpAppend:                     true,
}

var opCallLikeFactBarrierPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpSelf:           true,
	OpGo:             true,
	OpSend:           true,
	OpRecv:           true,
}

var opRawCarryClobberPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opExactDivComponentPolicies = [...]bool{
	OpPhi:      true,
	OpAdd:      true,
	OpSub:      true,
	OpMul:      true,
	OpMod:      true,
	OpAddInt:   true,
	OpSubInt:   true,
	OpMulInt:   true,
	OpModInt:   true,
	OpAddFloat: true,
	OpSubFloat: true,
	OpMulFloat: true,
}

var opIntNarrowCandidatePolicies = [...]bool{
	OpConstInt:      true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpUnboxInt:      true,
	OpPhi:           true,
	OpAdd:           true,
	OpSub:           true,
	OpMul:           true,
	OpMod:           true,
	OpAddInt:        true,
	OpSubInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpNegInt:        true,
	OpAddFloat:      true,
	OpSubFloat:      true,
	OpMulFloat:      true,
}

var opIntNarrowAllArgsConstraintPolicies = [...]bool{
	OpPhi:      true,
	OpAdd:      true,
	OpSub:      true,
	OpMul:      true,
	OpMod:      true,
	OpAddInt:   true,
	OpSubInt:   true,
	OpMulInt:   true,
	OpModInt:   true,
	OpAddFloat: true,
	OpSubFloat: true,
	OpMulFloat: true,
	OpNegInt:   true,
}

var opFieldNumFusionGapSafePolicies = [...]bool{
	OpNop:         true,
	OpConstInt:    true,
	OpConstFloat:  true,
	OpConstBool:   true,
	OpConstNil:    true,
	OpConstString: true,
	OpLoadSlot:    true,
	OpAddFloat:    true,
	OpSubFloat:    true,
	OpMulFloat:    true,
	OpDivFloat:    true,
	OpNegFloat:    true,
	OpSqrt:        true,
	OpFloor:       true,
	OpFMA:         true,
	OpFMSUB:       true,
	OpLtFloat:     true,
	OpLeFloat:     true,
}

var opRawIntSpecializationBlockerPolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpDiv: true,
	OpMod: true,
	OpUnm: true,
}

var opRawIntSpecializedOpPolicies = [...]Op{
	OpAdd: OpAddInt,
	OpSub: OpSubInt,
	OpMul: OpMulInt,
	OpMod: OpModInt,
	OpEq:  OpEqInt,
	OpLt:  OpLtInt,
	OpLe:  OpLeInt,
}

type opExactIntNarrowOpPolicy struct {
	Op  Op
	Set bool
}

var opExactIntNarrowOpPolicies = [...]opExactIntNarrowOpPolicy{
	OpAdd:      {Op: OpAddInt, Set: true},
	OpAddFloat: {Op: OpAddInt, Set: true},
	OpSub:      {Op: OpSubInt, Set: true},
	OpSubFloat: {Op: OpSubInt, Set: true},
	OpMul:      {Op: OpMulInt, Set: true},
	OpMulFloat: {Op: OpMulInt, Set: true},
	OpMod:      {Op: OpModInt, Set: true},
	OpDiv:      {Op: OpDivIntExact, Set: true},
	OpDivFloat: {Op: OpDivIntExact, Set: true},
	OpEq:       {Op: OpEqInt, Set: true},
	OpLt:       {Op: OpLtInt, Set: true},
	OpLe:       {Op: OpLeInt, Set: true},
}

type opBoxedFallbackOpPolicy struct {
	Op  Op
	Set bool
}

var opBoxedFallbackOpPolicies = [...]opBoxedFallbackOpPolicy{
	OpAddInt:      {Op: OpAdd, Set: true},
	OpSubInt:      {Op: OpSub, Set: true},
	OpMulInt:      {Op: OpMul, Set: true},
	OpModInt:      {Op: OpMod, Set: true},
	OpDivIntExact: {Op: OpDiv, Set: true},
	OpNegInt:      {Op: OpUnm, Set: true},
	OpEqInt:       {Op: OpEq, Set: true},
	OpLtInt:       {Op: OpLt, Set: true},
	OpLeInt:       {Op: OpLe, Set: true},
}

var opBoxedFallbackResultUnknownPolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpModInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
}

var opSourceFeedbackPolicies = [...]OpSourceFeedbackPolicy{
	OpGetField:           OpSourceFeedbackGetField,
	OpGetFieldNumToFloat: OpSourceFeedbackGetField,
	OpSetField:           OpSourceFeedbackSetField,
	OpGetTable:           OpSourceFeedbackGetTable,
	OpSetTable:           OpSourceFeedbackSetTable,
	OpAdd:                OpSourceFeedbackResultType,
	OpSub:                OpSourceFeedbackResultType,
	OpMul:                OpSourceFeedbackResultType,
	OpDiv:                OpSourceFeedbackResultType,
	OpMod:                OpSourceFeedbackResultType,
	OpUnm:                OpSourceFeedbackResultType,
	OpEq:                 OpSourceFeedbackResultType,
	OpLt:                 OpSourceFeedbackResultType,
	OpLe:                 OpSourceFeedbackResultType,
}

var opRangeRefineKindPolicies = [...]OpRangeRefineKind{
	OpLt:    OpRangeRefineLessThan,
	OpLtInt: OpRangeRefineLessThan,
	OpLe:    OpRangeRefineLessEqual,
	OpLeInt: OpRangeRefineLessEqual,
	OpEqInt: OpRangeRefineEqualInt,
}
