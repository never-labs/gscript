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

var opBoolTableFillStoreTableArgPolicies = [...]uint8{
	OpSetTable:        1,
	OpTableArrayStore: 1,
}

var opBoolTableFillStoreKeyArgPolicies = [...]uint8{
	OpSetTable:        2,
	OpTableArrayStore: 4,
}

var opBoolTableFillStoreValueArgPolicies = [...]uint8{
	OpSetTable:        3,
	OpTableArrayStore: 5,
}

var opBoolTableFillStoreKindSourcePolicies = [...]OpBoolTableFillKindSource{
	OpSetTable:        OpBoolTableFillKindAux2,
	OpTableArrayStore: OpBoolTableFillKindAux,
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
