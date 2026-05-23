package methodjit

// OpSideEffect describes the broad runtime effects an IR op may have.
type OpSideEffect uint8

const (
	OpSideEffectInvalid OpSideEffect = iota
	OpSideEffectNone
	OpSideEffectRead
	OpSideEffectWrite
	OpSideEffectReadWrite
	OpSideEffectAllocate
	OpSideEffectCall
	OpSideEffectControl
	OpSideEffectConcurrency
)

// OpEmitterFamily groups ops by the emitter area that owns their lowering.
type OpEmitterFamily uint8

const (
	OpEmitterInvalid OpEmitterFamily = iota
	OpEmitterConst
	OpEmitterSlot
	OpEmitterArithmetic
	OpEmitterMatrix
	OpEmitterKernel
	OpEmitterCompare
	OpEmitterString
	OpEmitterTable
	OpEmitterField
	OpEmitterGlobal
	OpEmitterUpvalue
	OpEmitterConversion
	OpEmitterGuard
	OpEmitterControl
	OpEmitterCall
	OpEmitterLoop
	OpEmitterClosure
	OpEmitterVararg
	OpEmitterConcurrency
	OpEmitterPhi
	OpEmitterSpecial
)

// OpArgPolicy summarizes how an op consumes Args/Aux metadata.
type OpArgPolicy uint8

const (
	OpArgInvalid OpArgPolicy = iota
	OpArgNone
	OpArgFixed
	OpArgVariadic
	OpArgAux
	OpArgFixedAux
	OpArgVariadicAux
	OpArgControl
)

// OpBackendPolicy describes backend-local cache and verification effects that
// are easy to miss in emit_dispatch.go.
type OpBackendPolicy uint16

const (
	OpBackendPreservesFieldSvalsCache OpBackendPolicy = 1 << iota
	OpBackendPreservesFieldSvalsCacheForFloatResult
	OpBackendPreservesTableArrayBounds
	OpBackendClearsTableArrayBounds
	OpBackendInvalidatesShape
	OpBackendPreservesScratchFPRCache
	OpBackendPreservesScratchFPRCacheForFloatResult
)

// OpSpec is the lightweight metadata contract for an IR op.
type OpSpec struct {
	Name          string
	Terminator    bool
	SideEffect    OpSideEffect
	ArgPolicy     OpArgPolicy
	EmitterFamily OpEmitterFamily
	MayDeopt      bool
	BackendPolicy OpBackendPolicy
}

func opSpec(name string, family OpEmitterFamily, args OpArgPolicy, effect OpSideEffect, mayDeopt bool) OpSpec {
	return OpSpec{
		Name:          name,
		SideEffect:    effect,
		ArgPolicy:     args,
		EmitterFamily: family,
		MayDeopt:      mayDeopt,
	}
}

func opTerminatorSpec(name string, args OpArgPolicy) OpSpec {
	spec := opSpec(name, OpEmitterControl, args, OpSideEffectControl, false)
	spec.Terminator = true
	return spec
}

var opSpecs = [...]OpSpec{
	OpConstInt:                   opSpec("ConstInt", OpEmitterConst, OpArgAux, OpSideEffectNone, false),
	OpConstFloat:                 opSpec("ConstFloat", OpEmitterConst, OpArgAux, OpSideEffectNone, false),
	OpConstBool:                  opSpec("ConstBool", OpEmitterConst, OpArgAux, OpSideEffectNone, false),
	OpConstNil:                   opSpec("ConstNil", OpEmitterConst, OpArgNone, OpSideEffectNone, false),
	OpConstString:                opSpec("ConstString", OpEmitterConst, OpArgAux, OpSideEffectRead, false),
	OpLoadSlot:                   opSpec("LoadSlot", OpEmitterSlot, OpArgAux, OpSideEffectRead, false),
	OpStoreSlot:                  opSpec("StoreSlot", OpEmitterSlot, OpArgFixedAux, OpSideEffectWrite, false),
	OpAdd:                        opSpec("Add", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpSub:                        opSpec("Sub", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpMul:                        opSpec("Mul", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpDiv:                        opSpec("Div", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpMod:                        opSpec("Mod", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpPow:                        opSpec("Pow", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpUnm:                        opSpec("Unm", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpNot:                        opSpec("Not", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpLen:                        opSpec("Len", OpEmitterArithmetic, OpArgFixed, OpSideEffectRead, true),
	OpAddInt:                     opSpec("AddInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpSubInt:                     opSpec("SubInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpMulInt:                     opSpec("MulInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpModInt:                     opSpec("ModInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpDivIntExact:                opSpec("DivIntExact", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpNegInt:                     opSpec("NegInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpAddFloat:                   opSpec("AddFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpSubFloat:                   opSpec("SubFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpMulFloat:                   opSpec("MulFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpDivFloat:                   opSpec("DivFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpNegFloat:                   opSpec("NegFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpSqrt:                       opSpec("Sqrt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpFloor:                      opSpec("Floor", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpMatrixDense:                opSpec("MatrixDense", OpEmitterMatrix, OpArgFixed, OpSideEffectAllocate, true),
	OpMatrixGetF:                 opSpec("MatrixGetF", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, true),
	OpMatrixSetF:                 opSpec("MatrixSetF", OpEmitterMatrix, OpArgFixed, OpSideEffectWrite, true),
	OpMatrixFlat:                 opSpec("MatrixFlat", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, true),
	OpMatrixStride:               opSpec("MatrixStride", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, true),
	OpMatrixLoadFAt:              opSpec("MatrixLoadFAt", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, false),
	OpMatrixStoreFAt:             opSpec("MatrixStoreFAt", OpEmitterMatrix, OpArgFixed, OpSideEffectWrite, false),
	OpMatrixRowPtr:               opSpec("MatrixRowPtr", OpEmitterMatrix, OpArgFixed, OpSideEffectNone, false),
	OpMatrixLoadFRow:             opSpec("MatrixLoadFRow", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, false),
	OpMatrixStoreFRow:            opSpec("MatrixStoreFRow", OpEmitterMatrix, OpArgFixed, OpSideEffectWrite, false),
	OpMatrixLoadFRowConst:        opSpec("MatrixLoadFRowConst", OpEmitterMatrix, OpArgFixedAux, OpSideEffectRead, false),
	OpMatrixStoreFRowConst:       opSpec("MatrixStoreFRowConst", OpEmitterMatrix, OpArgFixedAux, OpSideEffectWrite, false),
	OpFMA:                        opSpec("FMA", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpFMSUB:                      opSpec("FMSUB", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpComplexEscapeInSet:         opSpec("ComplexEscapeInSet", OpEmitterKernel, OpArgFixedAux, OpSideEffectNone, false),
	OpComplexEscapeRowCount:      opSpec("ComplexEscapeRowCount", OpEmitterKernel, OpArgFixedAux, OpSideEffectNone, false),
	OpRecordArrayLoopKernel:      opSpec("RecordArrayLoopKernel", OpEmitterKernel, OpArgVariadicAux, OpSideEffectReadWrite, true),
	OpEq:                         opSpec("Eq", OpEmitterCompare, OpArgFixed, OpSideEffectNone, true),
	OpLt:                         opSpec("Lt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, true),
	OpLe:                         opSpec("Le", OpEmitterCompare, OpArgFixed, OpSideEffectNone, true),
	OpEqInt:                      opSpec("EqInt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpLtInt:                      opSpec("LtInt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpLeInt:                      opSpec("LeInt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpModZeroInt:                 opSpec("ModZeroInt", OpEmitterCompare, OpArgFixedAux, OpSideEffectNone, false),
	OpLtFloat:                    opSpec("LtFloat", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpLeFloat:                    opSpec("LeFloat", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpEqString:                   opSpec("EqString", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpConcat:                     opSpec("Concat", OpEmitterString, OpArgVariadic, OpSideEffectRead, true),
	OpStringConstLookup:          opSpec("StringConstLookup", OpEmitterString, OpArgFixedAux, OpSideEffectRead, true),
	OpStringFormatInt:            opSpec("StringFormatInt", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringFormatConst:          opSpec("StringFormatConst", OpEmitterString, OpArgVariadicAux, OpSideEffectCall, true),
	OpStringFormatConstLen:       opSpec("StringFormatConstLen", OpEmitterString, OpArgVariadicAux, OpSideEffectCall, true),
	OpGetTableStringFormatInt:    opSpec("GetTableStringFormatInt", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringSplitPart:            opSpec("StringSplitPart", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringSplitSubstr:          opSpec("StringSplitSubstr", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringSplitSubstrNumber:    opSpec("StringSplitSubstrNumber", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpNewTable:                   opSpec("NewTable", OpEmitterTable, OpArgAux, OpSideEffectAllocate, true),
	OpNewFixedTable:              opSpec("NewFixedTable", OpEmitterTable, OpArgVariadicAux, OpSideEffectAllocate, true),
	OpGetTable:                   opSpec("GetTable", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpSetTable:                   opSpec("SetTable", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableArrayHeader:           opSpec("TableArrayHeader", OpEmitterTable, OpArgFixedAux, OpSideEffectRead, true),
	OpTableArrayLen:              opSpec("TableArrayLen", OpEmitterTable, OpArgFixed, OpSideEffectRead, false),
	OpTableArrayData:             opSpec("TableArrayData", OpEmitterTable, OpArgFixed, OpSideEffectRead, false),
	OpTableArrayLoad:             opSpec("TableArrayLoad", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpTableShapeID:               opSpec("TableShapeID", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpTableArrayStore:            opSpec("TableArrayStore", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableArraySwap:             opSpec("TableArraySwap", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableArraySwapPairs:        opSpec("TableArraySwapPairs", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableBoolArrayFill:         opSpec("TableBoolArrayFill", OpEmitterTable, OpArgVariadicAux, OpSideEffectWrite, true),
	OpTableBoolArrayCount:        opSpec("TableBoolArrayCount", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpTableIntArrayReversePrefix: opSpec("TableIntArrayReversePrefix", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableIntArrayCopyPrefix:    opSpec("TableIntArrayCopyPrefix", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableArrayNestedLoad:       opSpec("TableNestedLoad", OpEmitterTable, OpArgFixedAux, OpSideEffectRead, true),
	OpGetField:                   opSpec("GetField", OpEmitterField, OpArgFixedAux, OpSideEffectRead, true),
	OpGetFieldNumToFloat:         opSpec("GetFieldNumToFloat", OpEmitterField, OpArgFixedAux, OpSideEffectRead, true),
	OpFieldPolyLen:               opSpec("FieldPolyLen", OpEmitterField, OpArgFixedAux, OpSideEffectRead, true),
	OpFieldSvals:                 opSpec("FieldSvals", OpEmitterField, OpArgFixed, OpSideEffectRead, true),
	OpFieldLoad:                  opSpec("FieldLoad", OpEmitterField, OpArgFixedAux, OpSideEffectRead, false),
	OpFieldLoadNumToFloat:        opSpec("FieldLoadNumToFloat", OpEmitterField, OpArgFixedAux, OpSideEffectRead, false),
	OpFieldStore:                 opSpec("FieldStore", OpEmitterField, OpArgFixedAux, OpSideEffectWrite, false),
	OpSetField:                   opSpec("SetField", OpEmitterField, OpArgFixedAux, OpSideEffectWrite, true),
	OpSetList:                    opSpec("SetList", OpEmitterTable, OpArgVariadic, OpSideEffectWrite, true),
	OpAppend:                     opSpec("Append", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpGetGlobal:                  opSpec("GetGlobal", OpEmitterGlobal, OpArgAux, OpSideEffectRead, true),
	OpSetGlobal:                  opSpec("SetGlobal", OpEmitterGlobal, OpArgFixedAux, OpSideEffectWrite, true),
	OpGetUpval:                   opSpec("GetUpval", OpEmitterUpvalue, OpArgAux, OpSideEffectRead, false),
	OpSetUpval:                   opSpec("SetUpval", OpEmitterUpvalue, OpArgFixedAux, OpSideEffectWrite, false),
	OpBoxInt:                     opSpec("BoxInt", OpEmitterConversion, OpArgFixed, OpSideEffectNone, false),
	OpBoxFloat:                   opSpec("BoxFloat", OpEmitterConversion, OpArgFixed, OpSideEffectNone, false),
	OpUnboxInt:                   opSpec("UnboxInt", OpEmitterConversion, OpArgFixed, OpSideEffectNone, true),
	OpUnboxFloat:                 opSpec("UnboxFloat", OpEmitterConversion, OpArgFixed, OpSideEffectNone, true),
	OpNumToFloat:                 opSpec("NumToFloat", OpEmitterConversion, OpArgFixed, OpSideEffectNone, true),
	OpGuardType:                  opSpec("GuardType", OpEmitterGuard, OpArgFixedAux, OpSideEffectNone, true),
	OpGuardIntRange:              opSpec("GuardIntRange", OpEmitterGuard, OpArgFixedAux, OpSideEffectNone, true),
	OpGuardGlobalConst:           opSpec("GuardGlobalConst", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true),
	OpGuardConstString:           opSpec("GuardConstString", OpEmitterGuard, OpArgFixedAux, OpSideEffectNone, true),
	OpGuardTableKind:             opSpec("GuardTableKind", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true),
	OpGuardCalleeProto:           opSpec("GuardCalleeProto", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true),
	OpGuardFieldCalleeProto:      opSpec("GuardFieldCalleeProto", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true),
	OpGuardShapeFieldType:        opSpec("GuardShapeFieldType", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true),
	OpGuardShapeFieldTypeMask:    opSpec("GuardShapeFieldTypeMask", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true),
	OpGuardNonNil:                opSpec("GuardNonNil", OpEmitterGuard, OpArgFixed, OpSideEffectNone, true),
	OpGuardTruthy:                opSpec("GuardTruthy", OpEmitterGuard, OpArgFixed, OpSideEffectNone, true),
	OpJump:                       opTerminatorSpec("Jump", OpArgControl),
	OpBranch:                     opTerminatorSpec("Branch", OpArgControl),
	OpReturn:                     opTerminatorSpec("Return", OpArgVariadic),
	OpCall:                       opSpec("Call", OpEmitterCall, OpArgVariadic, OpSideEffectCall, true),
	OpCallFloor:                  opSpec("CallFloor", OpEmitterCall, OpArgVariadic, OpSideEffectCall, true),
	OpFieldCallFloor:             opSpec("FieldCallFloor", OpEmitterCall, OpArgVariadic, OpSideEffectCall, true),
	OpResume:                     opSpec("Resume", OpEmitterCall, OpArgVariadicAux, OpSideEffectCall, true),
	OpYield:                      opSpec("Yield", OpEmitterCall, OpArgVariadicAux, OpSideEffectCall, true),
	OpSelf:                       opSpec("Self", OpEmitterCall, OpArgFixed, OpSideEffectCall, true),
	OpForPrep:                    opSpec("ForPrep", OpEmitterLoop, OpArgFixedAux, OpSideEffectControl, true),
	OpForLoop:                    opSpec("ForLoop", OpEmitterLoop, OpArgFixedAux, OpSideEffectControl, true),
	OpTForCall:                   opSpec("TForCall", OpEmitterLoop, OpArgVariadicAux, OpSideEffectCall, true),
	OpTForLoop:                   opSpec("TForLoop", OpEmitterLoop, OpArgFixedAux, OpSideEffectControl, false),
	OpClosure:                    opSpec("Closure", OpEmitterClosure, OpArgAux, OpSideEffectAllocate, true),
	OpClose:                      opSpec("Close", OpEmitterClosure, OpArgAux, OpSideEffectWrite, true),
	OpVararg:                     opSpec("Vararg", OpEmitterVararg, OpArgAux, OpSideEffectRead, true),
	OpTestSet:                    opSpec("TestSet", OpEmitterControl, OpArgFixedAux, OpSideEffectControl, false),
	OpGo:                         opSpec("Go", OpEmitterConcurrency, OpArgVariadic, OpSideEffectConcurrency, true),
	OpMakeChan:                   opSpec("MakeChan", OpEmitterConcurrency, OpArgAux, OpSideEffectAllocate, true),
	OpSend:                       opSpec("Send", OpEmitterConcurrency, OpArgFixed, OpSideEffectConcurrency, true),
	OpRecv:                       opSpec("Recv", OpEmitterConcurrency, OpArgFixed, OpSideEffectConcurrency, true),
	OpPhi:                        opSpec("Phi", OpEmitterPhi, OpArgVariadic, OpSideEffectNone, false),
	OpNop:                        opSpec("Nop", OpEmitterSpecial, OpArgNone, OpSideEffectNone, false),
}

func (op Op) Spec() (OpSpec, bool) {
	if int(op) < len(opSpecs) && opSpecs[op].Name != "" {
		spec := opSpecs[op]
		if int(op) < len(opBackendPolicies) {
			spec.BackendPolicy = opBackendPolicies[op]
		}
		return spec, true
	}
	return OpSpec{}, false
}

func OpsByEmitterFamily(family OpEmitterFamily) []Op {
	var ops []Op
	for op := Op(0); op < OpMax; op++ {
		spec, ok := op.Spec()
		if !ok || spec.EmitterFamily != family {
			continue
		}
		ops = append(ops, op)
	}
	return ops
}

var opBackendPolicies = [...]OpBackendPolicy{
	OpConstInt:                   OpBackendPreservesTableArrayBounds,
	OpConstFloat:                 OpBackendPreservesTableArrayBounds,
	OpConstBool:                  OpBackendPreservesTableArrayBounds,
	OpConstNil:                   OpBackendPreservesTableArrayBounds,
	OpConstString:                OpBackendPreservesTableArrayBounds,
	OpLoadSlot:                   OpBackendPreservesTableArrayBounds,
	OpStoreSlot:                  OpBackendPreservesTableArrayBounds,
	OpAdd:                        OpBackendPreservesTableArrayBounds,
	OpSub:                        OpBackendPreservesTableArrayBounds,
	OpMul:                        OpBackendPreservesTableArrayBounds,
	OpDiv:                        OpBackendPreservesTableArrayBounds,
	OpMod:                        OpBackendPreservesTableArrayBounds,
	OpUnm:                        OpBackendPreservesTableArrayBounds,
	OpNot:                        OpBackendPreservesTableArrayBounds,
	OpLen:                        OpBackendClearsTableArrayBounds,
	OpAddInt:                     OpBackendPreservesTableArrayBounds,
	OpSubInt:                     OpBackendPreservesTableArrayBounds,
	OpMulInt:                     OpBackendPreservesTableArrayBounds,
	OpModInt:                     OpBackendPreservesTableArrayBounds,
	OpNegInt:                     OpBackendPreservesTableArrayBounds,
	OpAddFloat:                   OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpSubFloat:                   OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpMulFloat:                   OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpDivFloat:                   OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpNegFloat:                   OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpSqrt:                       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpFloor:                      OpBackendPreservesTableArrayBounds,
	OpFMA:                        OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpFMSUB:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpComplexEscapeInSet:         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpComplexEscapeRowCount:      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpRecordArrayLoopKernel:      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpEq:                         OpBackendPreservesTableArrayBounds,
	OpLt:                         OpBackendPreservesTableArrayBounds,
	OpLe:                         OpBackendPreservesTableArrayBounds,
	OpEqInt:                      OpBackendPreservesTableArrayBounds,
	OpLtInt:                      OpBackendPreservesTableArrayBounds,
	OpLeInt:                      OpBackendPreservesTableArrayBounds,
	OpModZeroInt:                 OpBackendPreservesTableArrayBounds,
	OpLtFloat:                    OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache | OpBackendPreservesScratchFPRCache,
	OpLeFloat:                    OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache | OpBackendPreservesScratchFPRCache,
	OpEqString:                   OpBackendPreservesTableArrayBounds,
	OpStringFormatConstLen:       OpBackendPreservesTableArrayBounds | OpBackendClearsTableArrayBounds,
	OpNewTable:                   0,
	OpSetTable:                   OpBackendClearsTableArrayBounds | OpBackendInvalidatesShape,
	OpTableArrayHeader:           OpBackendPreservesTableArrayBounds,
	OpTableArrayLen:              OpBackendPreservesTableArrayBounds,
	OpTableArrayData:             OpBackendPreservesTableArrayBounds,
	OpTableArrayLoad:             OpBackendPreservesTableArrayBounds,
	OpTableShapeID:               OpBackendPreservesTableArrayBounds,
	OpTableArrayStore:            OpBackendPreservesTableArrayBounds,
	OpTableArraySwap:             OpBackendPreservesTableArrayBounds,
	OpTableArraySwapPairs:        OpBackendPreservesTableArrayBounds | OpBackendClearsTableArrayBounds,
	OpTableBoolArrayFill:         OpBackendClearsTableArrayBounds,
	OpTableIntArrayReversePrefix: OpBackendClearsTableArrayBounds,
	OpTableIntArrayCopyPrefix:    OpBackendClearsTableArrayBounds,
	OpGetField:                   OpBackendPreservesFieldSvalsCache,
	OpGetFieldNumToFloat:         OpBackendPreservesFieldSvalsCache,
	OpFieldPolyLen:               OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpSetField:                   OpBackendPreservesFieldSvalsCache | OpBackendClearsTableArrayBounds,
	OpGetGlobal:                  OpBackendClearsTableArrayBounds,
	OpSetGlobal:                  OpBackendClearsTableArrayBounds,
	OpGetUpval:                   OpBackendClearsTableArrayBounds,
	OpSetUpval:                   OpBackendClearsTableArrayBounds,
	OpBoxInt:                     OpBackendPreservesTableArrayBounds,
	OpBoxFloat:                   OpBackendPreservesTableArrayBounds,
	OpUnboxInt:                   OpBackendPreservesTableArrayBounds,
	OpUnboxFloat:                 OpBackendPreservesTableArrayBounds,
	OpNumToFloat:                 OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardType:                  OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardIntRange:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardGlobalConst:           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardConstString:           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardTableKind:             OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardCalleeProto:           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardFieldCalleeProto:      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldType:        OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldTypeMask:    OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardTruthy:                OpBackendPreservesTableArrayBounds,
	OpCall:                       OpBackendClearsTableArrayBounds,
	OpCallFloor:                  OpBackendClearsTableArrayBounds,
	OpFieldCallFloor:             OpBackendClearsTableArrayBounds,
	OpResume:                     OpBackendClearsTableArrayBounds,
	OpSelf:                       OpBackendClearsTableArrayBounds | OpBackendInvalidatesShape,
	OpForPrep:                    OpBackendClearsTableArrayBounds,
	OpForLoop:                    OpBackendClearsTableArrayBounds,
	OpTForCall:                   OpBackendClearsTableArrayBounds,
	OpTForLoop:                   OpBackendClearsTableArrayBounds,
	OpClosure:                    OpBackendClearsTableArrayBounds,
	OpClose:                      OpBackendClearsTableArrayBounds,
	OpVararg:                     OpBackendClearsTableArrayBounds,
	OpTestSet:                    OpBackendClearsTableArrayBounds,
	OpGo:                         OpBackendClearsTableArrayBounds,
	OpMakeChan:                   OpBackendClearsTableArrayBounds,
	OpSend:                       OpBackendClearsTableArrayBounds,
	OpRecv:                       OpBackendClearsTableArrayBounds,
	OpNop:                        OpBackendPreservesTableArrayBounds | OpBackendPreservesScratchFPRCache,
	OpConcat:                     OpBackendClearsTableArrayBounds,
	OpStringConstLookup:          OpBackendClearsTableArrayBounds,
	OpStringFormatInt:            OpBackendClearsTableArrayBounds,
	OpStringFormatConst:          OpBackendClearsTableArrayBounds,
	OpStringSplitPart:            OpBackendClearsTableArrayBounds,
	OpStringSplitSubstr:          OpBackendClearsTableArrayBounds,
	OpStringSplitSubstrNumber:    OpBackendClearsTableArrayBounds,
	OpAppend:                     OpBackendClearsTableArrayBounds,
	OpPow:                        OpBackendClearsTableArrayBounds,
	OpGuardNonNil:                OpBackendClearsTableArrayBounds,
}
