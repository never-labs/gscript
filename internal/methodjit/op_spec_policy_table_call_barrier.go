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
