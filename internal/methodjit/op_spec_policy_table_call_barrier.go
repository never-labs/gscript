package methodjit

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

var opTableArrayFactRolePolicies = [...]OpTableArrayFactRole{
	OpTableArrayHeader: OpTableArrayFactHeader,
	OpTableArrayLen:    OpTableArrayFactLen,
	OpTableArrayData:   OpTableArrayFactData,
}

var opTableArrayStoreLoopCandidatePolicies = [...]bool{
	OpSetTable: true,
}

var opTableArrayStoreLoopBlockerPolicies = [...]bool{
	OpTableArrayStore:    true,
	OpResume:             true,
	OpSelf:               true,
	OpSetField:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

var opTableArrayStoreLoopEscapeCallPolicies = [...]bool{
	OpCall: true,
}

var opTableArrayStoreLoopUseOKPolicies = [...]bool{
	OpGuardTableKind: true,
	OpGuardType:      true,
	OpReturn:         true,
}
