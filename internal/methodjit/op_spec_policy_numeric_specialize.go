package methodjit

var opGenericSpecializablePolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpDiv: true,
	OpMod: true,
	OpUnm: true,
	OpEq:  true,
	OpLt:  true,
	OpLe:  true,
}

type opTypeSpecializationPolicy struct {
	IntOp    Op
	FloatOp  Op
	StringOp Op
	Set      bool
}

var opTypeSpecializationPolicies = [...]opTypeSpecializationPolicy{
	OpAdd: {IntOp: OpAddInt, FloatOp: OpAddFloat, StringOp: OpMax, Set: true},
	OpSub: {IntOp: OpSubInt, FloatOp: OpSubFloat, StringOp: OpMax, Set: true},
	OpMul: {IntOp: OpMulInt, FloatOp: OpMulFloat, StringOp: OpMax, Set: true},
	OpMod: {IntOp: OpModInt, FloatOp: OpMax, StringOp: OpMax, Set: true},
	OpDiv: {IntOp: OpDivFloat, FloatOp: OpDivFloat, StringOp: OpMax, Set: true},
	OpUnm: {IntOp: OpNegInt, FloatOp: OpNegFloat, StringOp: OpMax, Set: true},
	OpEq:  {IntOp: OpEqInt, FloatOp: OpMax, StringOp: OpEqString, Set: true},
	OpLt:  {IntOp: OpLtInt, FloatOp: OpLtFloat, StringOp: OpMax, Set: true},
	OpLe:  {IntOp: OpLeInt, FloatOp: OpLeFloat, StringOp: OpMax, Set: true},
}

var opNumToFloatInsertCandidatePolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpDiv: true,
	OpLt:  true,
	OpLe:  true,
}
