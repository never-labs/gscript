package methodjit

var opFieldShapeSplitInlineSafePolicies = [...]bool{
	OpConstInt:              true,
	OpConstFloat:            true,
	OpConstBool:             true,
	OpConstNil:              true,
	OpConstString:           true,
	OpAddInt:                true,
	OpSubInt:                true,
	OpMulInt:                true,
	OpModInt:                true,
	OpNegInt:                true,
	OpAddFloat:              true,
	OpSubFloat:              true,
	OpMulFloat:              true,
	OpDivFloat:              true,
	OpNegFloat:              true,
	OpEqInt:                 true,
	OpLtInt:                 true,
	OpLeInt:                 true,
	OpEqString:              true,
	OpLtFloat:               true,
	OpLeFloat:               true,
	OpFloor:                 true,
	OpNumToFloat:            true,
	OpFieldSvals:            true,
	OpFieldLoad:             true,
	OpFieldLoadNumToFloat:   true,
	OpFieldPolyLen:          true,
	OpGuardType:             true,
	OpGuardIntRange:         true,
	OpGuardCalleeProto:      true,
	OpGuardFieldCalleeProto: true,
	OpBranch:                true,
	OpJump:                  true,
	OpPhi:                   true,
	OpFieldStore:            true,
	OpTableArrayHeader:      true,
	OpTableArrayLen:         true,
	OpTableArrayData:        true,
	OpTableArrayLoad:        true,
	OpTableArrayStore:       true,
}

var opFieldShapePreEffectInlineSafePolicies = [...]bool{
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
	OpGetTable:           true,
	OpSetTable:           true,
	OpAdd:                true,
	OpSub:                true,
	OpMul:                true,
	OpDiv:                true,
	OpMod:                true,
	OpUnm:                true,
	OpLen:                true,
	OpFloor:              true,
	OpNumToFloat:         true,
}

var opFieldShapeInlineSideEffectPolicies = [...]bool{
	OpFieldStore:      true,
	OpTableArrayStore: true,
	OpSetField:        true,
	OpSetTable:        true,
}

var opFieldShapePostEffectInlineUnsafePolicies = [...]bool{
	OpSetField:           true,
	OpGetField:           true,
	OpGetFieldNumToFloat: true,
	OpSetTable:           true,
	OpGetTable:           true,
	OpCall:               true,
	OpCallFloor:          true,
	OpFieldCallFloor:     true,
	OpResume:             true,
	OpYield:              true,
	OpSelf:               true,
}

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

var opLoadElimConstCSEPolicies = [...]bool{
	OpConstInt:    true,
	OpConstFloat:  true,
	OpConstBool:   true,
	OpConstNil:    true,
	OpConstString: true,
}

var opLiteralConstPolicies = [...]bool{
	OpConstInt:    true,
	OpConstFloat:  true,
	OpConstBool:   true,
	OpConstNil:    true,
	OpConstString: true,
}

var opLoadElimPureCSEPolicies = [...]bool{
	OpAddInt:       true,
	OpSubInt:       true,
	OpMulInt:       true,
	OpModInt:       true,
	OpDivIntExact:  true,
	OpNegInt:       true,
	OpAddFloat:     true,
	OpSubFloat:     true,
	OpMulFloat:     true,
	OpDivFloat:     true,
	OpNegFloat:     true,
	OpNumToFloat:   true,
	OpSqrt:         true,
	OpFloor:        true,
	OpFMA:          true,
	OpFMSUB:        true,
	OpEqInt:        true,
	OpLtInt:        true,
	OpLeInt:        true,
	OpModZeroInt:   true,
	OpLtFloat:      true,
	OpLeFloat:      true,
	OpEqString:     true,
	OpTableShapeID: true,
}

var opLoadElimShapeFactKillerPolicies = [...]bool{
	OpSetField:                   true,
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpAppend:                     true,
	OpSetList:                    true,
	OpCall:                       true,
	OpResume:                     true,
	OpSelf:                       true,
}

var opFieldSvalsCrossBlockBarrierPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpSelf:           true,
	OpSetGlobal:      true,
	OpSetUpval:       true,
	OpSetTable:       true,
	OpSetList:        true,
	OpAppend:         true,
}

var opFieldSvalsGlobalBarrierPolicies = [...]bool{
	OpSetTable:       true,
	OpSetList:        true,
	OpAppend:         true,
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
	OpResume:         true,
	OpYield:          true,
	OpSelf:           true,
	OpSetGlobal:      true,
	OpSetUpval:       true,
}

var opFieldLenFoldBarrierPolicies = [...]bool{
	OpCall:                true,
	OpSetField:            true,
	OpFieldStore:          true,
	OpSetTable:            true,
	OpTableArrayStore:     true,
	OpTableArraySwap:      true,
	OpTableArraySwapPairs: true,
	OpSetGlobal:           true,
	OpSetUpval:            true,
	OpAppend:              true,
	OpSetList:             true,
}

var opFieldCallPolyLenFusionBarrierPolicies = [...]bool{
	OpSetField:                   true,
	OpSetTable:                   true,
	OpSetList:                    true,
	OpAppend:                     true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpCall:                       true,
	OpCallFloor:                  true,
	OpFieldCallFloor:             true,
	OpResume:                     true,
	OpYield:                      true,
	OpSelf:                       true,
	OpGo:                         true,
	OpSend:                       true,
	OpRecv:                       true,
	OpReturn:                     true,
	OpJump:                       true,
	OpBranch:                     true,
}

func applyOpSpecFieldPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFieldShapeSplitInlineSafePolicies) {
		spec.FieldShapeSplitInlineSafe = opFieldShapeSplitInlineSafePolicies[op]
	}
	if int(op) < len(opFieldShapePreEffectInlineSafePolicies) {
		spec.FieldShapePreEffectInlineSafe = opFieldShapePreEffectInlineSafePolicies[op]
	}
	if int(op) < len(opFieldShapeInlineSideEffectPolicies) {
		spec.FieldShapeInlineSideEffect = opFieldShapeInlineSideEffectPolicies[op]
	}
	if int(op) < len(opFieldShapePostEffectInlineUnsafePolicies) {
		spec.FieldShapePostEffectInlineUnsafe = opFieldShapePostEffectInlineUnsafePolicies[op]
	}
	if int(op) < len(opGlobalConstUnsafePolicies) {
		spec.GlobalConstUnsafe = opGlobalConstUnsafePolicies[op]
	}
	if int(op) < len(opNestedCallLikePolicies) {
		spec.NestedCallLike = opNestedCallLikePolicies[op]
	}
	if int(op) < len(opLoadElimConstCSEPolicies) {
		spec.LoadElimConstCSE = opLoadElimConstCSEPolicies[op]
	}
	if int(op) < len(opLiteralConstPolicies) {
		spec.LiteralConst = opLiteralConstPolicies[op]
	}
	if int(op) < len(opLoadElimPureCSEPolicies) {
		spec.LoadElimPureCSE = opLoadElimPureCSEPolicies[op]
	}
	if int(op) < len(opLoadElimShapeFactKillerPolicies) {
		spec.LoadElimShapeFactKiller = opLoadElimShapeFactKillerPolicies[op]
	}
}

func applyOpSpecFieldBarrierPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opFieldSvalsCrossBlockBarrierPolicies) {
		spec.FieldSvalsCrossBlockBarrier = opFieldSvalsCrossBlockBarrierPolicies[op]
	}
	if int(op) < len(opFieldSvalsGlobalBarrierPolicies) {
		spec.FieldSvalsGlobalBarrier = opFieldSvalsGlobalBarrierPolicies[op]
	}
	if int(op) < len(opFieldLenFoldBarrierPolicies) {
		spec.FieldLenFoldBarrier = opFieldLenFoldBarrierPolicies[op]
	}
	if int(op) < len(opFieldCallPolyLenFusionBarrierPolicies) {
		spec.FieldCallPolyLenFusionBarrier = opFieldCallPolyLenFusionBarrierPolicies[op]
	}
}
