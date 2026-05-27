package methodjit

var opRuntimeOverflowBoxablePolicies = [...]bool{
	OpAddInt: true,
	OpSubInt: true,
	OpMulInt: true,
	OpNegInt: true,
}

var opRuntimeGuardRefreshablePolicies = [...]bool{
	OpGuardType:        true,
	OpGuardCalleeProto: true,
	OpGuardConstString: true,
	OpGuardTableKind:   true,
	OpGuardIntRange:    true,
}

var opNativeNumericValueProducerPolicies = [...]bool{
	OpConstInt:   true,
	OpConstFloat: true,
	OpUnboxInt:   true,
	OpUnboxFloat: true,
	OpAdd:        true,
	OpSub:        true,
	OpMul:        true,
	OpDiv:        true,
	OpMod:        true,
	OpUnm:        true,
	OpAddInt:     true,
	OpSubInt:     true,
	OpMulInt:     true,
	OpModInt:     true,
	OpNegInt:     true,
	OpAddFloat:   true,
	OpSubFloat:   true,
	OpMulFloat:   true,
	OpDivFloat:   true,
	OpNegFloat:   true,
	OpFloor:      true,
}

var opPureNumericUnknownValuePolicies = [...]bool{
	OpAdd:         true,
	OpSub:         true,
	OpMul:         true,
	OpDiv:         true,
	OpMod:         true,
	OpUnm:         true,
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpModInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
	OpAddFloat:    true,
	OpSubFloat:    true,
	OpMulFloat:    true,
	OpDivFloat:    true,
	OpNegFloat:    true,
	OpNumToFloat:  true,
	OpPhi:         true,
	OpLoadSlot:    true,
}

var opTableArraySwapPureBetweenPolicies = [...]bool{
	OpConstInt:         true,
	OpConstFloat:       true,
	OpConstBool:        true,
	OpConstNil:         true,
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
	OpAddInt:           true,
	OpSubInt:           true,
	OpMulInt:           true,
	OpNegInt:           true,
	OpBoxInt:           true,
	OpUnboxInt:         true,
	OpGuardTableKind:   true,
	OpNop:              true,
}

var opStaticTableLenBenignUsePolicies = [...]bool{
	OpLen:              true,
	OpGetTable:         true,
	OpTableArrayHeader: true,
}

var opFixedResultTypePolicies = [...]Type{
	OpConstInt:              TypeInt,
	OpConstFloat:            TypeFloat,
	OpConstBool:             TypeBool,
	OpConstNil:              TypeNil,
	OpConstString:           TypeString,
	OpAddInt:                TypeInt,
	OpSubInt:                TypeInt,
	OpMulInt:                TypeInt,
	OpModInt:                TypeInt,
	OpNegInt:                TypeInt,
	OpAddFloat:              TypeFloat,
	OpSubFloat:              TypeFloat,
	OpMulFloat:              TypeFloat,
	OpDivFloat:              TypeFloat,
	OpNegFloat:              TypeFloat,
	OpNumToFloat:            TypeFloat,
	OpSqrt:                  TypeFloat,
	OpFloor:                 TypeInt,
	OpEqInt:                 TypeBool,
	OpLtInt:                 TypeBool,
	OpLeInt:                 TypeBool,
	OpModZeroInt:            TypeBool,
	OpLtFloat:               TypeBool,
	OpLeFloat:               TypeBool,
	OpEqString:              TypeBool,
	OpComplexEscapeInSet:    TypeBool,
	OpComplexEscapeRowCount: TypeInt,
	OpEq:                    TypeBool,
	OpLt:                    TypeBool,
	OpLe:                    TypeBool,
	OpNot:                   TypeBool,
	OpLen:                   TypeInt,
	OpDiv:                   TypeFloat,
	OpGuardIntRange:         TypeInt,
	OpGetFieldNumToFloat:    TypeFloat,
	OpFieldLoadNumToFloat:   TypeFloat,
	OpNewTable:              TypeTable,
	OpNewFixedTable:         TypeTable,
	OpClosure:               TypeFunction,
}

var opProvesNonNilResultPolicies = [...]bool{
	OpConstInt:            true,
	OpConstFloat:          true,
	OpConstBool:           true,
	OpConstString:         true,
	OpAdd:                 true,
	OpSub:                 true,
	OpMul:                 true,
	OpDiv:                 true,
	OpMod:                 true,
	OpPow:                 true,
	OpAddInt:              true,
	OpSubInt:              true,
	OpMulInt:              true,
	OpModInt:              true,
	OpDivIntExact:         true,
	OpNegInt:              true,
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpSqrt:                true,
	OpFloor:               true,
	OpFMA:                 true,
	OpFMSUB:               true,
	OpNumToFloat:          true,
	OpGetFieldNumToFloat:  true,
	OpFieldLoadNumToFloat: true,
	OpLen:                 true,
	OpLtInt:               true,
	OpLeInt:               true,
	OpEqInt:               true,
	OpLtFloat:             true,
	OpLeFloat:             true,
	OpEqString:            true,
}

var opGuardProvenResultTypePolicies = [...]Type{
	OpConstInt:            TypeInt,
	OpAddInt:              TypeInt,
	OpSubInt:              TypeInt,
	OpMulInt:              TypeInt,
	OpModInt:              TypeInt,
	OpDivIntExact:         TypeInt,
	OpNegInt:              TypeInt,
	OpFloor:               TypeInt,
	OpConstFloat:          TypeFloat,
	OpAddFloat:            TypeFloat,
	OpSubFloat:            TypeFloat,
	OpMulFloat:            TypeFloat,
	OpDivFloat:            TypeFloat,
	OpNegFloat:            TypeFloat,
	OpNumToFloat:          TypeFloat,
	OpGetFieldNumToFloat:  TypeFloat,
	OpFieldLoadNumToFloat: TypeFloat,
	OpSqrt:                TypeFloat,
	OpFMA:                 TypeFloat,
	OpFMSUB:               TypeFloat,
	OpConstBool:           TypeBool,
	OpEqInt:               TypeBool,
	OpLtInt:               TypeBool,
	OpLeInt:               TypeBool,
	OpModZeroInt:          TypeBool,
	OpLtFloat:             TypeBool,
	OpLeFloat:             TypeBool,
	OpEqString:            TypeBool,
	OpEq:                  TypeBool,
	OpLt:                  TypeBool,
	OpLe:                  TypeBool,
	OpNot:                 TypeBool,
	OpConstNil:            TypeNil,
	OpConstString:         TypeString,
	OpNewTable:            TypeTable,
	OpNewFixedTable:       TypeTable,
	OpClosure:             TypeFunction,
}

var opRawFloatValueProducerPolicies = [...]bool{
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpSqrt:                true,
	OpFMA:                 true,
	OpFMSUB:               true,
	OpGetFieldNumToFloat:  true,
	OpFieldLoadNumToFloat: true,
	OpNumToFloat:          true,
}

var opFieldFactWideKillerPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpAppend:                     true,
	OpSetList:                    true,
}

var opTableMutationFirstArgPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpSetList:                    true,
	OpAppend:                     true,
}

var opCallLikeFactBarrierPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpSelf:           true,
	OpGo:             true,
	OpSend:           true,
	OpRecv:           true,
}

var opRawCarryClobberPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

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

var opRawIntSpecializedOpPolicies = [...]Op{
	OpAdd: OpAddInt,
	OpSub: OpSubInt,
	OpMul: OpMulInt,
	OpMod: OpModInt,
	OpEq:  OpEqInt,
	OpLt:  OpLtInt,
	OpLe:  OpLeInt,
}

type opExactIntNarrowOpPolicy struct {
	Op  Op
	Set bool
}

var opExactIntNarrowOpPolicies = [...]opExactIntNarrowOpPolicy{
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

type opBoxedFallbackOpPolicy struct {
	Op  Op
	Set bool
}

var opBoxedFallbackOpPolicies = [...]opBoxedFallbackOpPolicy{
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
