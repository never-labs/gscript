package methodjit

var opExactDivComponentPolicies = [...]bool{
	OpPhi:      true,
	OpAdd:      true,
	OpSub:      true,
	OpMul:      true,
	OpMod:      true,
	OpAddInt:   true,
	OpSubInt:   true,
	OpMulInt:   true,
	OpModInt:   true,
	OpAddFloat: true,
	OpSubFloat: true,
	OpMulFloat: true,
}

var opIntNarrowCandidatePolicies = [...]bool{
	OpConstInt:      true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpUnboxInt:      true,
	OpPhi:           true,
	OpAdd:           true,
	OpSub:           true,
	OpMul:           true,
	OpMod:           true,
	OpAddInt:        true,
	OpSubInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpNegInt:        true,
	OpAddFloat:      true,
	OpSubFloat:      true,
	OpMulFloat:      true,
}

var opIntNarrowAllArgsConstraintPolicies = [...]bool{
	OpPhi:      true,
	OpAdd:      true,
	OpSub:      true,
	OpMul:      true,
	OpMod:      true,
	OpAddInt:   true,
	OpSubInt:   true,
	OpMulInt:   true,
	OpModInt:   true,
	OpAddFloat: true,
	OpSubFloat: true,
	OpMulFloat: true,
	OpNegInt:   true,
}

var opFieldNumFusionGapSafePolicies = [...]bool{
	OpNop:         true,
	OpConstInt:    true,
	OpConstFloat:  true,
	OpConstBool:   true,
	OpConstNil:    true,
	OpConstString: true,
	OpLoadSlot:    true,
	OpAddFloat:    true,
	OpSubFloat:    true,
	OpMulFloat:    true,
	OpDivFloat:    true,
	OpNegFloat:    true,
	OpSqrt:        true,
	OpFloor:       true,
	OpFMA:         true,
	OpFMSUB:       true,
	OpLtFloat:     true,
	OpLeFloat:     true,
}

var opRawIntSpecializationBlockerPolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpDiv: true,
	OpMod: true,
	OpUnm: true,
}

type opTargetPolicy struct {
	Op  Op
	Set bool
}

var opRawIntSpecializedOpPolicies = [...]opTargetPolicy{
	OpAdd: {Op: OpAddInt, Set: true},
	OpSub: {Op: OpSubInt, Set: true},
	OpMul: {Op: OpMulInt, Set: true},
	OpMod: {Op: OpModInt, Set: true},
	OpEq:  {Op: OpEqInt, Set: true},
	OpLt:  {Op: OpLtInt, Set: true},
	OpLe:  {Op: OpLeInt, Set: true},
}

var opExactIntNarrowOpPolicies = [...]opTargetPolicy{
	OpAdd:      {Op: OpAddInt, Set: true},
	OpAddFloat: {Op: OpAddInt, Set: true},
	OpSub:      {Op: OpSubInt, Set: true},
	OpSubFloat: {Op: OpSubInt, Set: true},
	OpMul:      {Op: OpMulInt, Set: true},
	OpMulFloat: {Op: OpMulInt, Set: true},
	OpMod:      {Op: OpModInt, Set: true},
	OpDiv:      {Op: OpDivIntExact, Set: true},
	OpDivFloat: {Op: OpDivIntExact, Set: true},
	OpEq:       {Op: OpEqInt, Set: true},
	OpLt:       {Op: OpLtInt, Set: true},
	OpLe:       {Op: OpLeInt, Set: true},
}

var opBoxedFallbackOpPolicies = [...]opTargetPolicy{
	OpAddInt:      {Op: OpAdd, Set: true},
	OpSubInt:      {Op: OpSub, Set: true},
	OpMulInt:      {Op: OpMul, Set: true},
	OpModInt:      {Op: OpMod, Set: true},
	OpDivIntExact: {Op: OpDiv, Set: true},
	OpNegInt:      {Op: OpUnm, Set: true},
	OpEqInt:       {Op: OpEq, Set: true},
	OpLtInt:       {Op: OpLt, Set: true},
	OpLeInt:       {Op: OpLe, Set: true},
}

var opBoxedFallbackResultUnknownPolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpModInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
}

var opSourceFeedbackPolicies = [...]OpSourceFeedbackPolicy{
	OpGetField:           OpSourceFeedbackGetField,
	OpGetFieldNumToFloat: OpSourceFeedbackGetField,
	OpSetField:           OpSourceFeedbackSetField,
	OpGetTable:           OpSourceFeedbackGetTable,
	OpSetTable:           OpSourceFeedbackSetTable,
	OpAdd:                OpSourceFeedbackResultType,
	OpSub:                OpSourceFeedbackResultType,
	OpMul:                OpSourceFeedbackResultType,
	OpDiv:                OpSourceFeedbackResultType,
	OpMod:                OpSourceFeedbackResultType,
	OpUnm:                OpSourceFeedbackResultType,
	OpEq:                 OpSourceFeedbackResultType,
	OpLt:                 OpSourceFeedbackResultType,
	OpLe:                 OpSourceFeedbackResultType,
}

var opRangeRefineKindPolicies = [...]OpRangeRefineKind{
	OpLt:    OpRangeRefineLessThan,
	OpLtInt: OpRangeRefineLessThan,
	OpLe:    OpRangeRefineLessEqual,
	OpLeInt: OpRangeRefineLessEqual,
	OpEqInt: OpRangeRefineEqualInt,
}
