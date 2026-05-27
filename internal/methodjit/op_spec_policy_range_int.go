package methodjit

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

var opInt48RuntimeValuePolicies = [...]bool{
	OpConstInt:      true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpLoadSlot:      true,
	OpUnboxInt:      true,
}
