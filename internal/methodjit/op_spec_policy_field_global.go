package methodjit

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
