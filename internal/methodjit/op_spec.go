package methodjit

import "fmt"

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
	OpEmitterSpecialization
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

const OpCountAny = -1

// OpCountPolicy describes an optional validator count contract. When Set is
// false, the validator does not enforce the count.
type OpCountPolicy struct {
	Min int
	Max int
	Set bool
}

func OpFixedCount(n int) OpCountPolicy {
	return OpCountPolicy{Min: n, Max: n, Set: true}
}

func OpRangedCount(min, max int) OpCountPolicy {
	return OpCountPolicy{Min: min, Max: max, Set: true}
}

func (p OpCountPolicy) accepts(got int) bool {
	if !p.Set {
		return true
	}
	if got < p.Min {
		return false
	}
	return p.Max == OpCountAny || got <= p.Max
}

func (p OpCountPolicy) describe() string {
	if p.Max == OpCountAny {
		return fmt.Sprintf("at least %d", p.Min)
	}
	return fmt.Sprintf("%d..%d", p.Min, p.Max)
}

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

type OpSourceFeedbackPolicy uint8

const (
	OpSourceFeedbackNone     OpSourceFeedbackPolicy = 0
	OpSourceFeedbackGetField OpSourceFeedbackPolicy = 1 << iota
	OpSourceFeedbackSetField
	OpSourceFeedbackGetTable
	OpSourceFeedbackSetTable
	OpSourceFeedbackResultType
)

// OpSpec is the lightweight metadata contract for an IR op.
type OpSpec struct {
	Name                             string
	Terminator                       bool
	SideEffect                       OpSideEffect
	ArgPolicy                        OpArgPolicy
	ArgCount                         OpCountPolicy
	SuccCount                        int // OpCountAny means successor count is not checked.
	KeepUnused                       bool
	EmitterFamily                    OpEmitterFamily
	MayDeopt                         bool
	NativeReplayMayExit              bool
	NativeReplayVisibleSideEffect    bool
	NativeReplayVisibleTableMutation bool
	NativeCalleeResumeUnsafe         bool
	RestartVisibleSideEffect         bool
	FieldShapeSplitInlineSafe        bool
	FieldShapePreEffectInlineSafe    bool
	FieldShapeInlineSideEffect       bool
	FieldShapePostEffectInlineUnsafe bool
	GlobalConstUnsafe                bool
	NestedCallLike                   bool
	LoadElimConstCSE                 bool
	LoadElimPureCSE                  bool
	LoadElimShapeFactKiller          bool
	NoSSAResult                      bool
	RawIntResult                     bool
	RawTablePtrResult                bool
	RawDataPtrResult                 bool
	RawFloatResult                   bool
	MatrixNative                     bool
	TableArrayGPRInvariant           bool
	LICMHoistable                    bool
	LICMInterestingMiss              bool
	LICMIntArith                     bool
	PureNumericInline                bool
	NativeEffectLoopInline           bool
	DirectDeoptWithoutFullFlush      bool
	GenericSpecializable             bool
	NumToFloatInsertCandidate        bool
	IntRecurrence                    bool
	NumericOperand                   bool
	FieldSvalsCrossBlockBarrier      bool
	FieldSvalsGlobalBarrier          bool
	FieldLenFoldBarrier              bool
	FieldCallPolyLenFusionBarrier    bool
	BoxableIntArithmetic             bool
	UnsafeIntArithmeticCandidate     bool
	ExactDivAllowedExternalUse       bool
	NonNegativeDerivationCandidate   bool
	Int48RuntimeValue                bool
	FusableComparison                bool
	ConstPoolUser                    bool
	UnrollCloneable                  bool
	CallResultRangeGuardCandidate    bool
	SpeculativeIntUseCandidate       bool
	FloatRegResult                   bool
	RawIntCarryValue                 bool
	TableArrayRegionGlobalBarrier    bool
	TableArrayRegionAliasingCall     bool
	TableArrayRegionAliasingAlways   bool
	TableArrayRegionTableMutation    bool
	RuntimeOverflowBoxable           bool
	RuntimeGuardRefreshable          bool
	NativeNumericValueProducer       bool
	PureNumericUnknownValue          bool
	TableArraySwapPureBetween        bool
	StaticTableLenBenignUse          bool
	FixedResultType                  Type
	ProvesNonNilResult               bool
	GuardProvenResultType            Type
	RawFloatValueProducer            bool
	FieldFactWideKiller              bool
	TableMutationFirstArg            bool
	CallLikeFactBarrier              bool
	ExactDivComponent                bool
	IntNarrowCandidate               bool
	IntNarrowAllArgsConstraint       bool
	FieldNumFusionGapSafe            bool
	RawIntSpecializationBlocker      bool
	RawIntSpecializedOp              Op
	BackendPolicy                    OpBackendPolicy
	SourceFeedbackPolicy             OpSourceFeedbackPolicy
}

func opSpec(name string, family OpEmitterFamily, args OpArgPolicy, effect OpSideEffect, mayDeopt bool) OpSpec {
	return OpSpec{
		Name:          name,
		SideEffect:    effect,
		ArgPolicy:     args,
		SuccCount:     OpCountAny,
		EmitterFamily: family,
		MayDeopt:      mayDeopt,
	}
}

func opTerminatorSpec(name string, args OpArgPolicy, argCount OpCountPolicy, succCount int) OpSpec {
	spec := opSpec(name, OpEmitterControl, args, OpSideEffectControl, false)
	spec.Terminator = true
	spec.ArgCount = argCount
	spec.SuccCount = succCount
	return spec
}

func opSpecArgCount(spec OpSpec, count OpCountPolicy) OpSpec {
	spec.ArgCount = count
	return spec
}

var opSpecs = [...]OpSpec{
	OpConstInt:                      opSpec("ConstInt", OpEmitterConst, OpArgAux, OpSideEffectNone, false),
	OpConstFloat:                    opSpec("ConstFloat", OpEmitterConst, OpArgAux, OpSideEffectNone, false),
	OpConstBool:                     opSpec("ConstBool", OpEmitterConst, OpArgAux, OpSideEffectNone, false),
	OpConstNil:                      opSpec("ConstNil", OpEmitterConst, OpArgNone, OpSideEffectNone, false),
	OpConstString:                   opSpec("ConstString", OpEmitterConst, OpArgAux, OpSideEffectRead, false),
	OpLoadSlot:                      opSpec("LoadSlot", OpEmitterSlot, OpArgAux, OpSideEffectRead, false),
	OpStoreSlot:                     opSpec("StoreSlot", OpEmitterSlot, OpArgFixedAux, OpSideEffectWrite, false),
	OpAdd:                           opSpec("Add", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpSub:                           opSpec("Sub", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpMul:                           opSpec("Mul", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpDiv:                           opSpec("Div", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpMod:                           opSpec("Mod", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpPow:                           opSpec("Pow", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpUnm:                           opSpec("Unm", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpNot:                           opSpec("Not", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpLen:                           opSpec("Len", OpEmitterArithmetic, OpArgFixed, OpSideEffectRead, true),
	OpAddInt:                        opSpec("AddInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpSubInt:                        opSpec("SubInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpMulInt:                        opSpec("MulInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpModInt:                        opSpec("ModInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpDivIntExact:                   opSpec("DivIntExact", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpNegInt:                        opSpec("NegInt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpAddFloat:                      opSpec("AddFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpSubFloat:                      opSpec("SubFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpMulFloat:                      opSpec("MulFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpDivFloat:                      opSpec("DivFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpNegFloat:                      opSpec("NegFloat", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpSqrt:                          opSpec("Sqrt", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpFloor:                         opSpec("Floor", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, true),
	OpMatrixDense:                   opSpec("MatrixDense", OpEmitterMatrix, OpArgFixed, OpSideEffectAllocate, true),
	OpMatrixGetF:                    opSpec("MatrixGetF", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, true),
	OpMatrixSetF:                    opSpec("MatrixSetF", OpEmitterMatrix, OpArgFixed, OpSideEffectWrite, true),
	OpMatrixFlat:                    opSpec("MatrixFlat", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, true),
	OpMatrixStride:                  opSpec("MatrixStride", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, true),
	OpMatrixLoadFAt:                 opSpec("MatrixLoadFAt", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, false),
	OpMatrixStoreFAt:                opSpec("MatrixStoreFAt", OpEmitterMatrix, OpArgFixed, OpSideEffectWrite, false),
	OpMatrixRowPtr:                  opSpec("MatrixRowPtr", OpEmitterMatrix, OpArgFixed, OpSideEffectNone, false),
	OpMatrixLoadFRow:                opSpec("MatrixLoadFRow", OpEmitterMatrix, OpArgFixed, OpSideEffectRead, false),
	OpMatrixStoreFRow:               opSpec("MatrixStoreFRow", OpEmitterMatrix, OpArgFixed, OpSideEffectWrite, false),
	OpMatrixLoadFRowConst:           opSpec("MatrixLoadFRowConst", OpEmitterMatrix, OpArgFixedAux, OpSideEffectRead, false),
	OpMatrixStoreFRowConst:          opSpec("MatrixStoreFRowConst", OpEmitterMatrix, OpArgFixedAux, OpSideEffectWrite, false),
	OpFMA:                           opSpec("FMA", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpFMSUB:                         opSpec("FMSUB", OpEmitterArithmetic, OpArgFixed, OpSideEffectNone, false),
	OpComplexEscapeInSet:            opSpec("ComplexEscapeInSet", OpEmitterSpecialization, OpArgFixedAux, OpSideEffectNone, false),
	OpComplexEscapeRowCount:         opSpec("ComplexEscapeRowCount", OpEmitterSpecialization, OpArgFixedAux, OpSideEffectNone, false),
	OpRecordArrayLoopSpecialization: opSpecArgCount(opSpec("RecordArrayLoopSpecialization", OpEmitterSpecialization, OpArgVariadicAux, OpSideEffectReadWrite, true), OpRangedCount(3, 16)),
	OpEq:                            opSpec("Eq", OpEmitterCompare, OpArgFixed, OpSideEffectNone, true),
	OpLt:                            opSpec("Lt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, true),
	OpLe:                            opSpec("Le", OpEmitterCompare, OpArgFixed, OpSideEffectNone, true),
	OpEqInt:                         opSpec("EqInt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpLtInt:                         opSpec("LtInt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpLeInt:                         opSpec("LeInt", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpModZeroInt:                    opSpec("ModZeroInt", OpEmitterCompare, OpArgFixedAux, OpSideEffectNone, false),
	OpLtFloat:                       opSpec("LtFloat", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpLeFloat:                       opSpec("LeFloat", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpEqString:                      opSpec("EqString", OpEmitterCompare, OpArgFixed, OpSideEffectNone, false),
	OpConcat:                        opSpec("Concat", OpEmitterString, OpArgVariadic, OpSideEffectRead, true),
	OpStringConstLookup:             opSpec("StringConstLookup", OpEmitterString, OpArgFixedAux, OpSideEffectRead, true),
	OpStringFormatInt:               opSpec("StringFormatInt", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringFormatConst:             opSpec("StringFormatConst", OpEmitterString, OpArgVariadicAux, OpSideEffectCall, true),
	OpStringFormatConstLen:          opSpec("StringFormatConstLen", OpEmitterString, OpArgVariadicAux, OpSideEffectCall, true),
	OpGetTableStringFormatInt:       opSpec("GetTableStringFormatInt", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringSplitPart:               opSpec("StringSplitPart", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringSplitSubstr:             opSpec("StringSplitSubstr", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpStringSplitSubstrNumber:       opSpec("StringSplitSubstrNumber", OpEmitterString, OpArgFixedAux, OpSideEffectCall, true),
	OpNewTable:                      opSpec("NewTable", OpEmitterTable, OpArgAux, OpSideEffectAllocate, true),
	OpNewFixedTable:                 opSpec("NewFixedTable", OpEmitterTable, OpArgVariadicAux, OpSideEffectAllocate, true),
	OpGetTable:                      opSpec("GetTable", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpSetTable:                      opSpecArgCount(opSpec("SetTable", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true), OpFixedCount(3)),
	OpTableArrayHeader:              opSpec("TableArrayHeader", OpEmitterTable, OpArgFixedAux, OpSideEffectRead, true),
	OpTableArrayLen:                 opSpec("TableArrayLen", OpEmitterTable, OpArgFixed, OpSideEffectRead, false),
	OpTableArrayData:                opSpec("TableArrayData", OpEmitterTable, OpArgFixed, OpSideEffectRead, false),
	OpTableArrayLoad:                opSpec("TableArrayLoad", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpTableShapeID:                  opSpecArgCount(opSpec("TableShapeID", OpEmitterTable, OpArgFixed, OpSideEffectRead, true), OpFixedCount(1)),
	OpTableArrayStore:               opSpecArgCount(opSpec("TableArrayStore", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true), OpRangedCount(5, 6)),
	OpTableArraySwap:                opSpec("TableArraySwap", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableArraySwapPairs:           opSpecArgCount(opSpec("TableArraySwapPairs", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true), OpFixedCount(3)),
	OpTableBoolArrayFill:            opSpec("TableBoolArrayFill", OpEmitterTable, OpArgVariadicAux, OpSideEffectWrite, true),
	OpTableBoolArrayCount:           opSpec("TableBoolArrayCount", OpEmitterTable, OpArgFixed, OpSideEffectRead, true),
	OpTableIntArrayReversePrefix:    opSpec("TableIntArrayReversePrefix", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableIntArrayCopyPrefix:       opSpec("TableIntArrayCopyPrefix", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpTableArrayNestedLoad:          opSpec("TableNestedLoad", OpEmitterTable, OpArgFixedAux, OpSideEffectRead, true),
	OpGetField:                      opSpec("GetField", OpEmitterField, OpArgFixedAux, OpSideEffectRead, true),
	OpGetFieldNumToFloat:            opSpec("GetFieldNumToFloat", OpEmitterField, OpArgFixedAux, OpSideEffectRead, true),
	OpFieldPolyLen:                  opSpecArgCount(opSpec("FieldPolyLen", OpEmitterField, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(1)),
	OpFieldSvals:                    opSpecArgCount(opSpec("FieldSvals", OpEmitterField, OpArgFixed, OpSideEffectRead, true), OpFixedCount(1)),
	OpFieldLoad:                     opSpecArgCount(opSpec("FieldLoad", OpEmitterField, OpArgFixedAux, OpSideEffectRead, false), OpFixedCount(1)),
	OpFieldLoadNumToFloat:           opSpecArgCount(opSpec("FieldLoadNumToFloat", OpEmitterField, OpArgFixedAux, OpSideEffectRead, false), OpFixedCount(1)),
	OpFieldStore:                    opSpecArgCount(opSpec("FieldStore", OpEmitterField, OpArgFixedAux, OpSideEffectWrite, false), OpFixedCount(2)),
	OpSetField:                      opSpec("SetField", OpEmitterField, OpArgFixedAux, OpSideEffectWrite, true),
	OpSetList:                       opSpec("SetList", OpEmitterTable, OpArgVariadic, OpSideEffectWrite, true),
	OpAppend:                        opSpec("Append", OpEmitterTable, OpArgFixed, OpSideEffectWrite, true),
	OpGetGlobal:                     opSpec("GetGlobal", OpEmitterGlobal, OpArgAux, OpSideEffectRead, true),
	OpSetGlobal:                     opSpec("SetGlobal", OpEmitterGlobal, OpArgFixedAux, OpSideEffectWrite, true),
	OpGetUpval:                      opSpec("GetUpval", OpEmitterUpvalue, OpArgAux, OpSideEffectRead, false),
	OpSetUpval:                      opSpec("SetUpval", OpEmitterUpvalue, OpArgFixedAux, OpSideEffectWrite, false),
	OpBoxInt:                        opSpec("BoxInt", OpEmitterConversion, OpArgFixed, OpSideEffectNone, false),
	OpBoxFloat:                      opSpec("BoxFloat", OpEmitterConversion, OpArgFixed, OpSideEffectNone, false),
	OpUnboxInt:                      opSpec("UnboxInt", OpEmitterConversion, OpArgFixed, OpSideEffectNone, true),
	OpUnboxFloat:                    opSpec("UnboxFloat", OpEmitterConversion, OpArgFixed, OpSideEffectNone, true),
	OpNumToFloat:                    opSpec("NumToFloat", OpEmitterConversion, OpArgFixed, OpSideEffectNone, true),
	OpGuardType:                     opSpecArgCount(opSpec("GuardType", OpEmitterGuard, OpArgFixedAux, OpSideEffectNone, true), OpFixedCount(1)),
	OpGuardIntRange:                 opSpec("GuardIntRange", OpEmitterGuard, OpArgFixedAux, OpSideEffectNone, true),
	OpGuardGlobalConst:              opSpecArgCount(opSpec("GuardGlobalConst", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(0)),
	OpGuardConstString:              opSpecArgCount(opSpec("GuardConstString", OpEmitterGuard, OpArgFixedAux, OpSideEffectNone, true), OpFixedCount(1)),
	OpGuardTableKind:                opSpecArgCount(opSpec("GuardTableKind", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(1)),
	OpGuardCalleeProto:              opSpecArgCount(opSpec("GuardCalleeProto", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(1)),
	OpGuardFieldCalleeProto:         opSpecArgCount(opSpec("GuardFieldCalleeProto", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(1)),
	OpGuardShapeFieldType:           opSpecArgCount(opSpec("GuardShapeFieldType", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(0)),
	OpGuardShapeFieldTypeMask:       opSpecArgCount(opSpec("GuardShapeFieldTypeMask", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(0)),
	OpGuardShapeFieldVMClosure:      opSpecArgCount(opSpec("GuardShapeFieldVMClosure", OpEmitterGuard, OpArgFixedAux, OpSideEffectRead, true), OpFixedCount(0)),
	OpGuardNonNil:                   opSpecArgCount(opSpec("GuardNonNil", OpEmitterGuard, OpArgFixed, OpSideEffectNone, true), OpFixedCount(1)),
	OpGuardTruthy:                   opSpecArgCount(opSpec("GuardTruthy", OpEmitterGuard, OpArgFixed, OpSideEffectNone, true), OpFixedCount(1)),
	OpJump:                          opTerminatorSpec("Jump", OpArgControl, OpFixedCount(0), 1),
	OpBranch:                        opTerminatorSpec("Branch", OpArgControl, OpFixedCount(1), 2),
	OpReturn:                        opTerminatorSpec("Return", OpArgVariadic, OpRangedCount(0, OpCountAny), 0),
	OpCall:                          opSpec("Call", OpEmitterCall, OpArgVariadic, OpSideEffectCall, true),
	OpCallFloor:                     opSpec("CallFloor", OpEmitterCall, OpArgVariadic, OpSideEffectCall, true),
	OpFieldCallFloor:                opSpec("FieldCallFloor", OpEmitterCall, OpArgVariadic, OpSideEffectCall, true),
	OpResume:                        opSpec("Resume", OpEmitterCall, OpArgVariadicAux, OpSideEffectCall, true),
	OpYield:                         opSpec("Yield", OpEmitterCall, OpArgVariadicAux, OpSideEffectCall, true),
	OpSelf:                          opSpec("Self", OpEmitterCall, OpArgFixed, OpSideEffectCall, true),
	OpForPrep:                       opSpec("ForPrep", OpEmitterLoop, OpArgFixedAux, OpSideEffectControl, true),
	OpForLoop:                       opSpec("ForLoop", OpEmitterLoop, OpArgFixedAux, OpSideEffectControl, true),
	OpTForCall:                      opSpec("TForCall", OpEmitterLoop, OpArgVariadicAux, OpSideEffectCall, true),
	OpTForLoop:                      opSpec("TForLoop", OpEmitterLoop, OpArgFixedAux, OpSideEffectControl, false),
	OpClosure:                       opSpec("Closure", OpEmitterClosure, OpArgAux, OpSideEffectAllocate, true),
	OpClose:                         opSpec("Close", OpEmitterClosure, OpArgAux, OpSideEffectWrite, true),
	OpVararg:                        opSpec("Vararg", OpEmitterVararg, OpArgAux, OpSideEffectRead, true),
	OpTestSet:                       opSpec("TestSet", OpEmitterControl, OpArgFixedAux, OpSideEffectControl, false),
	OpGo:                            opSpec("Go", OpEmitterConcurrency, OpArgVariadic, OpSideEffectConcurrency, true),
	OpMakeChan:                      opSpec("MakeChan", OpEmitterConcurrency, OpArgAux, OpSideEffectAllocate, true),
	OpSend:                          opSpec("Send", OpEmitterConcurrency, OpArgFixed, OpSideEffectConcurrency, true),
	OpRecv:                          opSpec("Recv", OpEmitterConcurrency, OpArgFixed, OpSideEffectConcurrency, true),
	OpPhi:                           opSpec("Phi", OpEmitterPhi, OpArgVariadic, OpSideEffectNone, false),
	OpNop:                           opSpec("Nop", OpEmitterSpecial, OpArgNone, OpSideEffectNone, false),
}

func buildExpandedOpSpecs() [OpMax]OpSpec {
	var out [OpMax]OpSpec
	for op := Op(0); op < OpMax; op++ {
		if spec, ok := buildOpSpec(op); ok {
			out[op] = spec
		}
	}
	return out
}

func (op Op) Spec() (OpSpec, bool) {
	if int(op) < len(expandedOpSpecs) && expandedOpSpecs[op].Name != "" {
		return expandedOpSpecs[op], true
	}
	return OpSpec{}, false
}

func buildOpSpec(op Op) (OpSpec, bool) {
	if int(op) < len(opSpecs) && opSpecs[op].Name != "" {
		spec := opSpecs[op]
		if int(op) < len(opBackendPolicies) {
			spec.BackendPolicy = opBackendPolicies[op]
		}
		if int(op) < len(opKeepUnusedPolicies) {
			spec.KeepUnused = opKeepUnusedPolicies[op]
		}
		if int(op) < len(opNativeReplayMayExitPolicies) {
			spec.NativeReplayMayExit = opNativeReplayMayExitPolicies[op]
		}
		if int(op) < len(opNativeReplayVisibleSideEffectPolicies) {
			spec.NativeReplayVisibleSideEffect = opNativeReplayVisibleSideEffectPolicies[op]
		}
		if int(op) < len(opNativeReplayVisibleTableMutationPolicies) {
			spec.NativeReplayVisibleTableMutation = opNativeReplayVisibleTableMutationPolicies[op]
		}
		if int(op) < len(opNativeCalleeResumeUnsafePolicies) {
			spec.NativeCalleeResumeUnsafe = opNativeCalleeResumeUnsafePolicies[op]
		}
		if int(op) < len(opRestartVisibleSideEffectPolicies) {
			spec.RestartVisibleSideEffect = opRestartVisibleSideEffectPolicies[op]
		}
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
		if int(op) < len(opLoadElimPureCSEPolicies) {
			spec.LoadElimPureCSE = opLoadElimPureCSEPolicies[op]
		}
		if int(op) < len(opLoadElimShapeFactKillerPolicies) {
			spec.LoadElimShapeFactKiller = opLoadElimShapeFactKillerPolicies[op]
		}
		if int(op) < len(opNoSSAResultPolicies) {
			spec.NoSSAResult = opNoSSAResultPolicies[op]
		}
		if int(op) < len(opRawIntResultPolicies) {
			spec.RawIntResult = opRawIntResultPolicies[op]
		}
		if int(op) < len(opRawTablePtrResultPolicies) {
			spec.RawTablePtrResult = opRawTablePtrResultPolicies[op]
		}
		if int(op) < len(opRawDataPtrResultPolicies) {
			spec.RawDataPtrResult = opRawDataPtrResultPolicies[op]
		}
		if int(op) < len(opRawFloatResultPolicies) {
			spec.RawFloatResult = opRawFloatResultPolicies[op]
		}
		if int(op) < len(opMatrixNativePolicies) {
			spec.MatrixNative = opMatrixNativePolicies[op]
		}
		if int(op) < len(opTableArrayGPRInvariantPolicies) {
			spec.TableArrayGPRInvariant = opTableArrayGPRInvariantPolicies[op]
		}
		if int(op) < len(opLICMHoistablePolicies) {
			spec.LICMHoistable = opLICMHoistablePolicies[op]
		}
		if int(op) < len(opLICMInterestingMissPolicies) {
			spec.LICMInterestingMiss = opLICMInterestingMissPolicies[op]
		}
		if int(op) < len(opLICMIntArithPolicies) {
			spec.LICMIntArith = opLICMIntArithPolicies[op]
		}
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
		if int(op) < len(opNumToFloatInsertCandidatePolicies) {
			spec.NumToFloatInsertCandidate = opNumToFloatInsertCandidatePolicies[op]
		}
		if int(op) < len(opIntRecurrencePolicies) {
			spec.IntRecurrence = opIntRecurrencePolicies[op]
		}
		if int(op) < len(opNumericOperandPolicies) {
			spec.NumericOperand = opNumericOperandPolicies[op]
		}
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
		if int(op) < len(opBoxableIntArithmeticPolicies) {
			spec.BoxableIntArithmetic = opBoxableIntArithmeticPolicies[op]
		}
		if int(op) < len(opUnsafeIntArithmeticCandidatePolicies) {
			spec.UnsafeIntArithmeticCandidate = opUnsafeIntArithmeticCandidatePolicies[op]
		}
		if int(op) < len(opExactDivAllowedExternalUsePolicies) {
			spec.ExactDivAllowedExternalUse = opExactDivAllowedExternalUsePolicies[op]
		}
		if int(op) < len(opNonNegativeDerivationCandidatePolicies) {
			spec.NonNegativeDerivationCandidate = opNonNegativeDerivationCandidatePolicies[op]
		}
		if int(op) < len(opInt48RuntimeValuePolicies) {
			spec.Int48RuntimeValue = opInt48RuntimeValuePolicies[op]
		}
		if int(op) < len(opFusableComparisonPolicies) {
			spec.FusableComparison = opFusableComparisonPolicies[op]
		}
		if int(op) < len(opConstPoolUserPolicies) {
			spec.ConstPoolUser = opConstPoolUserPolicies[op]
		}
		if int(op) < len(opUnrollCloneablePolicies) {
			spec.UnrollCloneable = opUnrollCloneablePolicies[op]
		}
		if int(op) < len(opCallResultRangeGuardCandidatePolicies) {
			spec.CallResultRangeGuardCandidate = opCallResultRangeGuardCandidatePolicies[op]
		}
		if int(op) < len(opSpeculativeIntUseCandidatePolicies) {
			spec.SpeculativeIntUseCandidate = opSpeculativeIntUseCandidatePolicies[op]
		}
		if int(op) < len(opFloatRegResultPolicies) {
			spec.FloatRegResult = opFloatRegResultPolicies[op]
		}
		if int(op) < len(opRawIntCarryValuePolicies) {
			spec.RawIntCarryValue = opRawIntCarryValuePolicies[op]
		}
		if int(op) < len(opTableArrayRegionGlobalBarrierPolicies) {
			spec.TableArrayRegionGlobalBarrier = opTableArrayRegionGlobalBarrierPolicies[op]
		}
		if int(op) < len(opTableArrayRegionAliasingCallPolicies) {
			spec.TableArrayRegionAliasingCall = opTableArrayRegionAliasingCallPolicies[op]
		}
		if int(op) < len(opTableArrayRegionAliasingAlwaysPolicies) {
			spec.TableArrayRegionAliasingAlways = opTableArrayRegionAliasingAlwaysPolicies[op]
		}
		if int(op) < len(opTableArrayRegionTableMutationPolicies) {
			spec.TableArrayRegionTableMutation = opTableArrayRegionTableMutationPolicies[op]
		}
		if int(op) < len(opRuntimeOverflowBoxablePolicies) {
			spec.RuntimeOverflowBoxable = opRuntimeOverflowBoxablePolicies[op]
		}
		if int(op) < len(opRuntimeGuardRefreshablePolicies) {
			spec.RuntimeGuardRefreshable = opRuntimeGuardRefreshablePolicies[op]
		}
		if int(op) < len(opNativeNumericValueProducerPolicies) {
			spec.NativeNumericValueProducer = opNativeNumericValueProducerPolicies[op]
		}
		if int(op) < len(opPureNumericUnknownValuePolicies) {
			spec.PureNumericUnknownValue = opPureNumericUnknownValuePolicies[op]
		}
		if int(op) < len(opTableArraySwapPureBetweenPolicies) {
			spec.TableArraySwapPureBetween = opTableArraySwapPureBetweenPolicies[op]
		}
		if int(op) < len(opStaticTableLenBenignUsePolicies) {
			spec.StaticTableLenBenignUse = opStaticTableLenBenignUsePolicies[op]
		}
		if int(op) < len(opFixedResultTypePolicies) {
			spec.FixedResultType = opFixedResultTypePolicies[op]
		}
		if int(op) < len(opProvesNonNilResultPolicies) {
			spec.ProvesNonNilResult = opProvesNonNilResultPolicies[op]
		}
		if int(op) < len(opGuardProvenResultTypePolicies) {
			spec.GuardProvenResultType = opGuardProvenResultTypePolicies[op]
		}
		if int(op) < len(opRawFloatValueProducerPolicies) {
			spec.RawFloatValueProducer = opRawFloatValueProducerPolicies[op]
		}
		if int(op) < len(opFieldFactWideKillerPolicies) {
			spec.FieldFactWideKiller = opFieldFactWideKillerPolicies[op]
		}
		if int(op) < len(opTableMutationFirstArgPolicies) {
			spec.TableMutationFirstArg = opTableMutationFirstArgPolicies[op]
		}
		if int(op) < len(opCallLikeFactBarrierPolicies) {
			spec.CallLikeFactBarrier = opCallLikeFactBarrierPolicies[op]
		}
		if int(op) < len(opExactDivComponentPolicies) {
			spec.ExactDivComponent = opExactDivComponentPolicies[op]
		}
		if int(op) < len(opIntNarrowCandidatePolicies) {
			spec.IntNarrowCandidate = opIntNarrowCandidatePolicies[op]
		}
		if int(op) < len(opIntNarrowAllArgsConstraintPolicies) {
			spec.IntNarrowAllArgsConstraint = opIntNarrowAllArgsConstraintPolicies[op]
		}
		if int(op) < len(opFieldNumFusionGapSafePolicies) {
			spec.FieldNumFusionGapSafe = opFieldNumFusionGapSafePolicies[op]
		}
		if int(op) < len(opRawIntSpecializationBlockerPolicies) {
			spec.RawIntSpecializationBlocker = opRawIntSpecializationBlockerPolicies[op]
		}
		if int(op) < len(opRawIntSpecializedOpPolicies) {
			spec.RawIntSpecializedOp = opRawIntSpecializedOpPolicies[op]
		}
		if int(op) < len(opSourceFeedbackPolicies) {
			spec.SourceFeedbackPolicy = opSourceFeedbackPolicies[op]
		}
		return spec, true
	}
	return OpSpec{}, false
}

func OpByName(name string) (Op, bool) {
	op, ok := opNameLookup[name]
	return op, ok
}

func (spec OpSpec) MayCallOrRunConcurrently() bool {
	return spec.SideEffect == OpSideEffectCall || spec.SideEffect == OpSideEffectConcurrency
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
	OpConstInt:                      OpBackendPreservesTableArrayBounds,
	OpConstFloat:                    OpBackendPreservesTableArrayBounds,
	OpConstBool:                     OpBackendPreservesTableArrayBounds,
	OpConstNil:                      OpBackendPreservesTableArrayBounds,
	OpConstString:                   OpBackendPreservesTableArrayBounds,
	OpLoadSlot:                      OpBackendPreservesTableArrayBounds,
	OpStoreSlot:                     OpBackendPreservesTableArrayBounds,
	OpAdd:                           OpBackendPreservesTableArrayBounds,
	OpSub:                           OpBackendPreservesTableArrayBounds,
	OpMul:                           OpBackendPreservesTableArrayBounds,
	OpDiv:                           OpBackendPreservesTableArrayBounds,
	OpMod:                           OpBackendPreservesTableArrayBounds,
	OpUnm:                           OpBackendPreservesTableArrayBounds,
	OpNot:                           OpBackendPreservesTableArrayBounds,
	OpLen:                           OpBackendClearsTableArrayBounds,
	OpAddInt:                        OpBackendPreservesTableArrayBounds,
	OpSubInt:                        OpBackendPreservesTableArrayBounds,
	OpMulInt:                        OpBackendPreservesTableArrayBounds,
	OpModInt:                        OpBackendPreservesTableArrayBounds,
	OpNegInt:                        OpBackendPreservesTableArrayBounds,
	OpAddFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpSubFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpMulFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpDivFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpNegFloat:                      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpSqrt:                          OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpFloor:                         OpBackendPreservesTableArrayBounds,
	OpFMA:                           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpFMSUB:                         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCacheForFloatResult | OpBackendPreservesScratchFPRCacheForFloatResult,
	OpComplexEscapeInSet:            OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpComplexEscapeRowCount:         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpRecordArrayLoopSpecialization: OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpEq:                            OpBackendPreservesTableArrayBounds,
	OpLt:                            OpBackendPreservesTableArrayBounds,
	OpLe:                            OpBackendPreservesTableArrayBounds,
	OpEqInt:                         OpBackendPreservesTableArrayBounds,
	OpLtInt:                         OpBackendPreservesTableArrayBounds,
	OpLeInt:                         OpBackendPreservesTableArrayBounds,
	OpModZeroInt:                    OpBackendPreservesTableArrayBounds,
	OpLtFloat:                       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache | OpBackendPreservesScratchFPRCache,
	OpLeFloat:                       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache | OpBackendPreservesScratchFPRCache,
	OpEqString:                      OpBackendPreservesTableArrayBounds,
	OpStringFormatConstLen:          OpBackendPreservesTableArrayBounds | OpBackendClearsTableArrayBounds,
	OpNewTable:                      0,
	OpSetTable:                      OpBackendClearsTableArrayBounds | OpBackendInvalidatesShape,
	OpTableArrayHeader:              OpBackendPreservesTableArrayBounds,
	OpTableArrayLen:                 OpBackendPreservesTableArrayBounds,
	OpTableArrayData:                OpBackendPreservesTableArrayBounds,
	OpTableArrayLoad:                OpBackendPreservesTableArrayBounds,
	OpTableShapeID:                  OpBackendPreservesTableArrayBounds,
	OpTableArrayStore:               OpBackendPreservesTableArrayBounds,
	OpTableArraySwap:                OpBackendPreservesTableArrayBounds,
	OpTableArraySwapPairs:           OpBackendPreservesTableArrayBounds | OpBackendClearsTableArrayBounds,
	OpTableBoolArrayFill:            OpBackendClearsTableArrayBounds,
	OpTableIntArrayReversePrefix:    OpBackendClearsTableArrayBounds,
	OpTableIntArrayCopyPrefix:       OpBackendClearsTableArrayBounds,
	OpGetField:                      OpBackendPreservesFieldSvalsCache,
	OpGetFieldNumToFloat:            OpBackendPreservesFieldSvalsCache,
	OpFieldPolyLen:                  OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpSetField:                      OpBackendPreservesFieldSvalsCache | OpBackendClearsTableArrayBounds,
	OpGetGlobal:                     OpBackendClearsTableArrayBounds,
	OpSetGlobal:                     OpBackendClearsTableArrayBounds,
	OpGetUpval:                      OpBackendClearsTableArrayBounds,
	OpSetUpval:                      OpBackendClearsTableArrayBounds,
	OpBoxInt:                        OpBackendPreservesTableArrayBounds,
	OpBoxFloat:                      OpBackendPreservesTableArrayBounds,
	OpUnboxInt:                      OpBackendPreservesTableArrayBounds,
	OpUnboxFloat:                    OpBackendPreservesTableArrayBounds,
	OpNumToFloat:                    OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardType:                     OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardIntRange:                 OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardGlobalConst:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardConstString:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardTableKind:                OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardCalleeProto:              OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardFieldCalleeProto:         OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldType:           OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldTypeMask:       OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardShapeFieldVMClosure:      OpBackendPreservesTableArrayBounds | OpBackendPreservesFieldSvalsCache,
	OpGuardTruthy:                   OpBackendPreservesTableArrayBounds,
	OpCall:                          OpBackendClearsTableArrayBounds,
	OpCallFloor:                     OpBackendClearsTableArrayBounds,
	OpFieldCallFloor:                OpBackendClearsTableArrayBounds,
	OpResume:                        OpBackendClearsTableArrayBounds,
	OpSelf:                          OpBackendClearsTableArrayBounds | OpBackendInvalidatesShape,
	OpForPrep:                       OpBackendClearsTableArrayBounds,
	OpForLoop:                       OpBackendClearsTableArrayBounds,
	OpTForCall:                      OpBackendClearsTableArrayBounds,
	OpTForLoop:                      OpBackendClearsTableArrayBounds,
	OpClosure:                       OpBackendClearsTableArrayBounds,
	OpClose:                         OpBackendClearsTableArrayBounds,
	OpVararg:                        OpBackendClearsTableArrayBounds,
	OpTestSet:                       OpBackendClearsTableArrayBounds,
	OpGo:                            OpBackendClearsTableArrayBounds,
	OpMakeChan:                      OpBackendClearsTableArrayBounds,
	OpSend:                          OpBackendClearsTableArrayBounds,
	OpRecv:                          OpBackendClearsTableArrayBounds,
	OpNop:                           OpBackendPreservesTableArrayBounds | OpBackendPreservesScratchFPRCache,
	OpConcat:                        OpBackendClearsTableArrayBounds,
	OpStringConstLookup:             OpBackendClearsTableArrayBounds,
	OpStringFormatInt:               OpBackendClearsTableArrayBounds,
	OpStringFormatConst:             OpBackendClearsTableArrayBounds,
	OpStringSplitPart:               OpBackendClearsTableArrayBounds,
	OpStringSplitSubstr:             OpBackendClearsTableArrayBounds,
	OpStringSplitSubstrNumber:       OpBackendClearsTableArrayBounds,
	OpAppend:                        OpBackendClearsTableArrayBounds,
	OpPow:                           OpBackendClearsTableArrayBounds,
	OpGuardNonNil:                   OpBackendClearsTableArrayBounds,
}

var opKeepUnusedPolicies = [...]bool{
	OpJump:                          true,
	OpBranch:                        true,
	OpReturn:                        true,
	OpStoreSlot:                     true,
	OpSetGlobal:                     true,
	OpSetUpval:                      true,
	OpSetTable:                      true,
	OpTableArrayStore:               true,
	OpTableArraySwap:                true,
	OpTableArraySwapPairs:           true,
	OpTableBoolArrayFill:            true,
	OpTableIntArrayReversePrefix:    true,
	OpTableIntArrayCopyPrefix:       true,
	OpRecordArrayLoopSpecialization: true,
	OpFieldStore:                    true,
	OpSetField:                      true,
	OpSetList:                       true,
	OpAppend:                        true,
	OpMatrixSetF:                    true,
	OpMatrixStoreFAt:                true,
	OpMatrixStoreFRow:               true,
	OpMatrixStoreFRowConst:          true,
	OpCall:                          true,
	OpCallFloor:                     true,
	OpFieldCallFloor:                true,
	OpResume:                        true,
	OpYield:                         true,
	OpSelf:                          true,
	OpGuardType:                     true,
	OpGuardIntRange:                 true,
	OpGuardGlobalConst:              true,
	OpGuardConstString:              true,
	OpGuardTableKind:                true,
	OpGuardCalleeProto:              true,
	OpGuardFieldCalleeProto:         true,
	OpGuardShapeFieldType:           true,
	OpGuardShapeFieldTypeMask:       true,
	OpGuardShapeFieldVMClosure:      true,
	OpGuardNonNil:                   true,
	OpGuardTruthy:                   true,
	OpForPrep:                       true,
	OpForLoop:                       true,
	OpTForCall:                      true,
	OpTForLoop:                      true,
	OpClosure:                       true,
	OpClose:                         true,
	OpGo:                            true,
	OpMakeChan:                      true,
	OpSend:                          true,
	OpRecv:                          true,
}

var opNativeReplayMayExitPolicies = [...]bool{
	OpCall:                       true,
	OpCallFloor:                  true,
	OpFieldCallFloor:             true,
	OpSelf:                       true,
	OpNewTable:                   true,
	OpNewFixedTable:              true,
	OpGetTable:                   true,
	OpSetTable:                   true,
	OpTableArrayHeader:           true,
	OpTableArrayLen:              true,
	OpTableArrayData:             true,
	OpTableArrayLoad:             true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableBoolArrayCount:        true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpTableArrayNestedLoad:       true,
	OpGetField:                   true,
	OpGetFieldNumToFloat:         true,
	OpFieldPolyLen:               true,
	OpSetField:                   true,
	OpFieldStore:                 true,
	OpSetList:                    true,
	OpAppend:                     true,
	OpGetGlobal:                  true,
	OpSetGlobal:                  true,
	OpGetUpval:                   true,
	OpSetUpval:                   true,
	OpConstString:                true,
	OpConcat:                     true,
	OpStringConstLookup:          true,
	OpStringFormatInt:            true,
	OpStringFormatConst:          true,
	OpStringFormatConstLen:       true,
	OpGetTableStringFormatInt:    true,
	OpStringSplitPart:            true,
	OpStringSplitSubstr:          true,
	OpStringSplitSubstrNumber:    true,
	OpLen:                        true,
	OpPow:                        true,
	OpFloor:                      true,
	OpClosure:                    true,
	OpClose:                      true,
	OpVararg:                     true,
	OpTForCall:                   true,
	OpTForLoop:                   true,
	OpGo:                         true,
	OpMakeChan:                   true,
	OpSend:                       true,
	OpRecv:                       true,
	OpGuardType:                  true,
	OpGuardIntRange:              true,
	OpGuardGlobalConst:           true,
	OpGuardConstString:           true,
	OpGuardTableKind:             true,
	OpGuardCalleeProto:           true,
	OpGuardFieldCalleeProto:      true,
	OpGuardShapeFieldType:        true,
	OpGuardShapeFieldTypeMask:    true,
	OpGuardShapeFieldVMClosure:   true,
	OpGuardNonNil:                true,
	OpGuardTruthy:                true,
	OpNumToFloat:                 true,
	OpDivIntExact:                true,
	OpMatrixDense:                true,
	OpMatrixGetF:                 true,
	OpMatrixSetF:                 true,
	OpMatrixFlat:                 true,
	OpMatrixStride:               true,
	OpAddInt:                     true,
	OpSubInt:                     true,
	OpMulInt:                     true,
	OpNegInt:                     true,
	OpModInt:                     true,
	OpModZeroInt:                 true,
}

var opNativeReplayVisibleSideEffectPolicies = [...]bool{
	OpSetGlobal:            true,
	OpSetUpval:             true,
	OpMatrixSetF:           true,
	OpMatrixStoreFAt:       true,
	OpMatrixStoreFRow:      true,
	OpMatrixStoreFRowConst: true,
	OpClose:                true,
	OpGo:                   true,
	OpSend:                 true,
	OpRecv:                 true,
}

var opNativeReplayVisibleTableMutationPolicies = [...]bool{
	OpSetTable:                   true,
	OpTableArrayStore:            true,
	OpTableArraySwap:             true,
	OpTableArraySwapPairs:        true,
	OpTableBoolArrayFill:         true,
	OpTableIntArrayReversePrefix: true,
	OpTableIntArrayCopyPrefix:    true,
	OpSetField:                   true,
	OpFieldStore:                 true,
	OpSetList:                    true,
	OpAppend:                     true,
}

var opNativeCalleeResumeUnsafePolicies = [...]bool{
	OpSetGlobal: true,
	OpSetUpval:  true,
	OpClose:     true,
	OpGo:        true,
	OpSend:      true,
	OpRecv:      true,
}

var opRestartVisibleSideEffectPolicies = [...]bool{
	OpCall:                true,
	OpSetGlobal:           true,
	OpSetTable:            true,
	OpTableArrayStore:     true,
	OpTableArraySwap:      true,
	OpTableArraySwapPairs: true,
	OpSetField:            true,
	OpNewTable:            true,
	OpNewFixedTable:       true,
	OpSetList:             true,
	OpAppend:              true,
	OpSelf:                true,
	OpSetUpval:            true,
	OpGo:                  true,
	OpMakeChan:            true,
	OpSend:                true,
	OpRecv:                true,
	OpClosure:             true,
	OpClose:               true,
	OpVararg:              true,
	OpConcat:              true,
	OpLen:                 true,
	OpPow:                 true,
	OpTForCall:            true,
	OpTForLoop:            true,
}

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

var opNoSSAResultPolicies = [...]bool{
	OpNop:                      true,
	OpStoreSlot:                true,
	OpSetTable:                 true,
	OpTableArrayStore:          true,
	OpTableArraySwap:           true,
	OpTableArraySwapPairs:      true,
	OpTableBoolArrayFill:       true,
	OpFieldStore:               true,
	OpSetField:                 true,
	OpSetList:                  true,
	OpAppend:                   true,
	OpGuardGlobalConst:         true,
	OpGuardTableKind:           true,
	OpGuardShapeFieldType:      true,
	OpGuardShapeFieldTypeMask:  true,
	OpGuardShapeFieldVMClosure: true,
	OpSetGlobal:                true,
	OpSetUpval:                 true,
	OpMatrixSetF:               true,
	OpMatrixStoreFAt:           true,
	OpMatrixStoreFRow:          true,
	OpMatrixStoreFRowConst:     true,
	OpClose:                    true,
	OpGo:                       true,
	OpSend:                     true,
}

var opRawIntResultPolicies = [...]bool{
	OpAddInt:        true,
	OpSubInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpDivIntExact:   true,
	OpNegInt:        true,
	OpTableArrayLen: true,
	OpTableShapeID:  true,
}

var opRawTablePtrResultPolicies = [...]bool{
	OpTableArrayHeader: true,
}

var opRawDataPtrResultPolicies = [...]bool{
	OpTableArrayData: true,
	OpFieldSvals:     true,
}

var opRawFloatResultPolicies = [...]bool{
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpNumToFloat:          true,
	OpGetFieldNumToFloat:  true,
	OpFieldLoadNumToFloat: true,
	OpSqrt:                true,
	OpFMA:                 true,
	OpFMSUB:               true,
}

var opMatrixNativePolicies = [...]bool{
	OpMatrixDense:          true,
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

var opTableArrayGPRInvariantPolicies = [...]bool{
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
	OpTableShapeID:     true,
	OpMatrixFlat:       true,
	OpMatrixStride:     true,
}

var opLICMHoistablePolicies = [...]bool{
	OpConstInt:            true,
	OpConstFloat:          true,
	OpConstBool:           true,
	OpConstNil:            true,
	OpLoadSlot:            true,
	OpGetField:            true,
	OpGetGlobal:           true,
	OpGuardGlobalConst:    true,
	OpGetUpval:            true,
	OpSqrt:                true,
	OpFloor:               true,
	OpLen:                 true,
	OpGetTable:            true,
	OpAddFloat:            true,
	OpSubFloat:            true,
	OpMulFloat:            true,
	OpDivFloat:            true,
	OpNegFloat:            true,
	OpFMA:                 true,
	OpFMSUB:               true,
	OpAddInt:              true,
	OpSubInt:              true,
	OpMulInt:              true,
	OpDivIntExact:         true,
	OpNegInt:              true,
	OpLtInt:               true,
	OpLeInt:               true,
	OpEqInt:               true,
	OpModZeroInt:          true,
	OpLtFloat:             true,
	OpLeFloat:             true,
	OpEqString:            true,
	OpNot:                 true,
	OpGuardType:           true,
	OpGuardIntRange:       true,
	OpGuardCalleeProto:    true,
	OpNumToFloat:          true,
	OpTableShapeID:        true,
	OpFieldSvals:          true,
	OpFieldLoad:           true,
	OpFieldLoadNumToFloat: true,
	OpMatrixFlat:          true,
	OpMatrixStride:        true,
	OpTableArrayHeader:    true,
	OpTableArrayLen:       true,
	OpTableArrayData:      true,
	OpMatrixRowPtr:        true,
}

var opLICMInterestingMissPolicies = [...]bool{
	OpGetField:         true,
	OpGetTable:         true,
	OpGetGlobal:        true,
	OpGuardGlobalConst: true,
	OpGuardCalleeProto: true,
	OpGetUpval:         true,
	OpLoadSlot:         true,
	OpAdd:              true,
	OpSub:              true,
	OpMul:              true,
	OpDiv:              true,
	OpMod:              true,
	OpUnm:              true,
	OpAddInt:           true,
	OpSubInt:           true,
	OpMulInt:           true,
	OpModInt:           true,
	OpDivIntExact:      true,
	OpNegInt:           true,
	OpAddFloat:         true,
	OpSubFloat:         true,
	OpMulFloat:         true,
	OpDivFloat:         true,
	OpNegFloat:         true,
	OpFMA:              true,
	OpFMSUB:            true,
	OpMatrixFlat:       true,
	OpMatrixStride:     true,
	OpMatrixRowPtr:     true,
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
	OpSqrt:             true,
	OpFloor:            true,
	OpLen:              true,
	OpNumToFloat:       true,
}

var opLICMIntArithPolicies = [...]bool{
	OpAddInt:      true,
	OpSubInt:      true,
	OpMulInt:      true,
	OpDivIntExact: true,
	OpNegInt:      true,
}

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

var opNonNegativeDerivationCandidatePolicies = [...]bool{
	OpConstInt:      true,
	OpLen:           true,
	OpTableArrayLen: true,
	OpGuardIntRange: true,
	OpAddInt:        true,
	OpMulInt:        true,
	OpModInt:        true,
	OpDivIntExact:   true,
	OpPhi:           true,
	OpBoxInt:        true,
	OpUnboxInt:      true,
}

var opInt48RuntimeValuePolicies = [...]bool{
	OpConstInt:      true,
	OpGuardType:     true,
	OpGuardIntRange: true,
	OpLoadSlot:      true,
	OpUnboxInt:      true,
}

var opFusableComparisonPolicies = [...]bool{
	OpEq:         true,
	OpLtInt:      true,
	OpLeInt:      true,
	OpEqInt:      true,
	OpModZeroInt: true,
	OpLtFloat:    true,
	OpLeFloat:    true,
}

var opConstPoolUserPolicies = [...]bool{
	OpConstString:             true,
	OpStringConstLookup:       true,
	OpStringFormatInt:         true,
	OpStringFormatConst:       true,
	OpStringFormatConstLen:    true,
	OpStringSplitPart:         true,
	OpStringSplitSubstr:       true,
	OpStringSplitSubstrNumber: true,
	OpGuardConstString:        true,
}

var opUnrollCloneablePolicies = [...]bool{
	OpConstInt:             true,
	OpConstFloat:           true,
	OpConstBool:            true,
	OpConstNil:             true,
	OpConstString:          true,
	OpAddInt:               true,
	OpSubInt:               true,
	OpMulInt:               true,
	OpModInt:               true,
	OpDivIntExact:          true,
	OpNegInt:               true,
	OpAddFloat:             true,
	OpSubFloat:             true,
	OpMulFloat:             true,
	OpDivFloat:             true,
	OpNegFloat:             true,
	OpSqrt:                 true,
	OpFloor:                true,
	OpFMA:                  true,
	OpFMSUB:                true,
	OpNumToFloat:           true,
	OpGuardType:            true,
	OpGuardIntRange:        true,
	OpGuardNonNil:          true,
	OpGuardTruthy:          true,
	OpMatrixLoadFAt:        true,
	OpMatrixLoadFRow:       true,
	OpMatrixLoadFRowConst:  true,
	OpTableArrayLoad:       true,
	OpTableArrayNestedLoad: true,
}

var opCallResultRangeGuardCandidatePolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opSpeculativeIntUseCandidatePolicies = [...]bool{
	OpAdd: true,
	OpSub: true,
	OpMul: true,
	OpMod: true,
	OpLt:  true,
	OpLe:  true,
}

var opFloatRegResultPolicies = [...]bool{
	OpConstFloat: true,
	OpAddFloat:   true,
	OpSubFloat:   true,
	OpMulFloat:   true,
	OpDivFloat:   true,
	OpNegFloat:   true,
	OpUnboxFloat: true,
	OpBoxFloat:   true,
}

var opRawIntCarryValuePolicies = [...]bool{
	OpConstInt:         true,
	OpLoadSlot:         true,
	OpGuardType:        true,
	OpGuardIntRange:    true,
	OpCall:             true,
	OpCallFloor:        true,
	OpFieldCallFloor:   true,
	OpPhi:              true,
	OpTableArrayHeader: true,
	OpTableArrayLen:    true,
	OpTableArrayData:   true,
}

var opTableArrayRegionGlobalBarrierPolicies = [...]bool{
	OpCall:               true,
	OpCallFloor:          true,
	OpFieldCallFloor:     true,
	OpResume:             true,
	OpSelf:               true,
	OpSetTable:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

var opTableArrayRegionAliasingCallPolicies = [...]bool{
	OpCall:           true,
	OpCallFloor:      true,
	OpFieldCallFloor: true,
}

var opTableArrayRegionAliasingAlwaysPolicies = [...]bool{
	OpResume: true,
	OpSelf:   true,
}

var opTableArrayRegionTableMutationPolicies = [...]bool{
	OpSetTable:           true,
	OpAppend:             true,
	OpSetList:            true,
	OpTableBoolArrayFill: true,
}

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

var expandedOpSpecs = buildExpandedOpSpecs()
var opNameLookup = buildOpNameLookup(expandedOpSpecs)

func buildOpNameLookup(specs [OpMax]OpSpec) map[string]Op {
	out := make(map[string]Op, int(OpMax))
	for op := Op(0); op < OpMax; op++ {
		if specs[op].Name != "" {
			out[specs[op].Name] = op
		}
	}
	return out
}
