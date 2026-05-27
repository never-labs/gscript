package methodjit

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
