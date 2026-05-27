package methodjit

var opPureNumericInlinePolicies = [...]bool{
	OpConstInt:                 true,
	OpConstFloat:               true,
	OpLoadSlot:                 true,
	OpAdd:                      true,
	OpSub:                      true,
	OpMul:                      true,
	OpDiv:                      true,
	OpMod:                      true,
	OpUnm:                      true,
	OpAddInt:                   true,
	OpSubInt:                   true,
	OpMulInt:                   true,
	OpModInt:                   true,
	OpDivIntExact:              true,
	OpNegInt:                   true,
	OpAddFloat:                 true,
	OpSubFloat:                 true,
	OpMulFloat:                 true,
	OpDivFloat:                 true,
	OpNegFloat:                 true,
	OpNumToFloat:               true,
	OpSqrt:                     true,
	OpFloor:                    true,
	OpFMA:                      true,
	OpFMSUB:                    true,
	OpEq:                       true,
	OpLt:                       true,
	OpLe:                       true,
	OpEqInt:                    true,
	OpLtInt:                    true,
	OpLeInt:                    true,
	OpLtFloat:                  true,
	OpLeFloat:                  true,
	OpEqString:                 true,
	OpModZeroInt:               true,
	OpTableShapeID:             true,
	OpGuardType:                true,
	OpGuardIntRange:            true,
	OpGuardConstString:         true,
	OpGuardTableKind:           true,
	OpGuardCalleeProto:         true,
	OpGuardFieldCalleeProto:    true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpJump:                     true,
	OpBranch:                   true,
	OpPhi:                      true,
}

var opNativeEffectLoopInlinePolicies = [...]bool{
	OpGetGlobal:            true,
	OpGuardGlobalConst:     true,
	OpTableArrayHeader:     true,
	OpTableArrayLen:        true,
	OpTableArrayData:       true,
	OpTableArrayLoad:       true,
	OpTableArrayNestedLoad: true,
	OpTableArrayStore:      true,
	OpFieldSvals:           true,
	OpFieldLoad:            true,
	OpFieldStore:           true,
	OpMatrixGetF:           true,
	OpMatrixSetF:           true,
	OpMatrixFlat:           true,
	OpMatrixStride:         true,
	OpMatrixLoadFAt:        true,
	OpMatrixStoreFAt:       true,
	OpMatrixRowPtr:         true,
	OpMatrixLoadFRow:       true,
	OpMatrixStoreFRow:      true,
	OpMatrixLoadFRowConst:  true,
	OpMatrixStoreFRowConst: true,
}

var opDirectDeoptWithoutFullFlushPolicies = [...]bool{
	OpGuardType:                true,
	OpGuardIntRange:            true,
	OpGuardGlobalConst:         true,
	OpGuardConstString:         true,
	OpGuardTableKind:           true,
	OpGuardCalleeProto:         true,
	OpGuardFieldCalleeProto:    true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpNumToFloat:               true,
	OpDivIntExact:              true,
	OpGetFieldNumToFloat:       true,
	OpFieldPolyLen:             true,
	OpFieldSvals:               true,
	OpFieldLoad:                true,
	OpFieldLoadNumToFloat:      true,
	OpMatrixGetF:               true,
	OpMatrixSetF:               true,
	OpMatrixFlat:               true,
	OpMatrixStride:             true,
	OpTableArrayHeader:         true,
	OpTableArrayLoad:           true,
	OpTableShapeID:             true,
	OpTableArrayStore:          true,
	OpTableArraySwap:           true,
	OpTableArraySwapPairs:      true,
	OpTableArrayNestedLoad:     true,
}

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

func applyOpSpecNumericPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opPureNumericInlinePolicies) {
		spec.PureNumericInline = opPureNumericInlinePolicies[op]
	}
	if int(op) < len(opNativeEffectLoopInlinePolicies) {
		spec.NativeEffectLoopInline = opNativeEffectLoopInlinePolicies[op]
	}
	if int(op) < len(opDirectDeoptWithoutFullFlushPolicies) {
		spec.DirectDeoptWithoutFullFlush = opDirectDeoptWithoutFullFlushPolicies[op]
	}
	if int(op) < len(opGenericSpecializablePolicies) {
		spec.GenericSpecializable = opGenericSpecializablePolicies[op]
	}
	if int(op) < len(opTypeSpecializationPolicies) && opTypeSpecializationPolicies[op].Set {
		policy := opTypeSpecializationPolicies[op]
		spec.TypeSpecializeIntOp = policy.IntOp
		spec.TypeSpecializeFloatOp = policy.FloatOp
		spec.TypeSpecializeStringOp = policy.StringOp
	}
	if int(op) < len(opNumToFloatInsertCandidatePolicies) {
		spec.NumToFloatInsertCandidate = opNumToFloatInsertCandidatePolicies[op]
	}
	if int(op) < len(opIntRecurrencePolicies) {
		spec.IntRecurrence = opIntRecurrencePolicies[op]
	}
	if int(op) < len(opNumericOperandPolicies) {
		spec.NumericOperand = opNumericOperandPolicies[op]
	}
}
