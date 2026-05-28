package methodjit

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

var opFieldSvalsFirstArgMutationBarrierPolicies = [...]bool{
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
}

var opFieldSvalsLoweredOpPolicies = [...]Op{
	OpGetField:           OpFieldLoad,
	OpGetFieldNumToFloat: OpFieldLoadNumToFloat,
	OpSetField:           OpFieldStore,
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
