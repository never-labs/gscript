package methodjit

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
