package methodjit

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
