package methodjit

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
