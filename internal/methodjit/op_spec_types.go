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

type OpRangeRefineKind uint8

const (
	OpRangeRefineNone OpRangeRefineKind = iota
	OpRangeRefineLessThan
	OpRangeRefineLessEqual
	OpRangeRefineEqualInt
)

type OpNonNegativeDerivationKind uint8

const (
	OpNonNegativeNone OpNonNegativeDerivationKind = iota
	OpNonNegativeConstIntAux
	OpNonNegativeAlways
	OpNonNegativeGuardRangeMin
	OpNonNegativeAllArgs
	OpNonNegativeBinaryAllArgs
	OpNonNegativeModuloDivisor
	OpNonNegativeExactDivPositiveDivisor
	OpNonNegativeForwardArg
)

// OpSpec is the lightweight metadata contract for an IR op.
type OpSpec struct {
	// Core IR shape and backend ownership.
	Name          string
	Terminator    bool
	SideEffect    OpSideEffect
	ArgPolicy     OpArgPolicy
	ArgCount      OpCountPolicy
	SuccCount     int // OpCountAny means successor count is not checked.
	KeepUnused    bool
	EmitterFamily OpEmitterFamily
	MayDeopt      bool

	// Runtime replay, deopt, and side-effect contracts.
	NativeReplayMayExit              bool
	NativeReplayVisibleSideEffect    bool
	NativeReplayVisibleTableMutation bool
	NativeCalleeResumeUnsafe         bool
	RestartVisibleSideEffect         bool

	// Field, shape, and load-elimination contracts.
	FieldShapeSplitInlineSafe        bool
	FieldShapePreEffectInlineSafe    bool
	FieldShapeInlineSideEffect       bool
	FieldShapePostEffectInlineUnsafe bool
	GlobalConstUnsafe                bool
	NestedCallLike                   bool
	LoadElimConstCSE                 bool
	LiteralConst                     bool
	LoadElimPureCSE                  bool
	LoadElimShapeFactKiller          bool

	// Result representation and value-lowering contracts.
	NoSSAResult                   bool
	RawIntResult                  bool
	RawTablePtrResult             bool
	RawDataPtrResult              bool
	RawFloatResult                bool
	MatrixNative                  bool
	TableArrayGPRInvariant        bool
	TableArrayGPRInvariantRank    int
	TableArrayGPRInvariantUseMask uint8
	TableArrayKeyArgIndex         int

	// Optimizer admission and numeric specialization contracts.
	LICMHoistable               bool
	LICMInterestingMiss         bool
	LICMIntArith                bool
	PureNumericInline           bool
	NativeEffectLoopInline      bool
	DirectDeoptWithoutFullFlush bool
	GenericSpecializable        bool
	TypeSpecializeIntOp         Op
	TypeSpecializeFloatOp       Op
	TypeSpecializeStringOp      Op
	NumToFloatInsertCandidate   bool
	IntRecurrence               bool
	NumericOperand              bool

	// Field/cache barrier contracts.
	FieldSvalsCrossBlockBarrier   bool
	FieldSvalsGlobalBarrier       bool
	FieldLenFoldBarrier           bool
	FieldCallPolyLenFusionBarrier bool

	// Integer range, boxing, narrowing, and recurrence contracts.
	BoxableIntArithmetic           bool
	UnsafeIntArithmeticCandidate   bool
	Int48SafeRangeCandidate        bool
	ExactDivAllowedExternalUse     bool
	NonNegativeDerivationCandidate bool
	NonNegativeDerivationKind      OpNonNegativeDerivationKind
	Int48RuntimeValue              bool
	FusableComparison              bool
	LoopBoundComparison            bool

	// String, unroll, and reduction contracts.
	ConstPoolUser                    bool
	RawStringResult                  bool
	DynamicStringQueryCacheKey       bool
	UnrollCloneable                  bool
	NestedFloatPhiOverrideSafe       bool
	FloatReductionWideUnrollBarrier  bool
	FloatReductionLatencyUnrollSeed  bool
	FloatReductionLatencyUnrollBlock bool
	FloatReductionDivOp              bool
	ConstantPhiBranchThreadPure      bool

	// Table, field, and call-shape specialization contracts.
	NeedsTier2FieldCache          bool
	FieldRead                     bool
	FieldSlotLoad                 bool
	FieldWrite                    bool
	BoolTableFillBodyBenign       bool
	BoolTableFillStore            bool
	BoolTableCountLoadBodyBenign  bool
	BoolTableCountLoad            bool
	BoolTableCountIncrementBenign bool
	BoolTableCountIncrement       bool
	CallResultRangeGuardCandidate bool
	ModuloReducibleCallFloor      bool
	CallFloorSpecStableCallee     bool
	CallFloorSpecFieldShape       bool
	Tier2LoopCall                 bool
	Tier2LoopFeedbackVMProtoCall  bool
	Tier2ResidualCallBlocker      bool
	Tier2LoopNativeCandidate      bool
	CallUserArgStart              int
	SpeculativeIntUseCandidate    bool

	// Register allocation and raw runtime value contracts.
	FloatRegResult         bool
	FloatRegResultBlocked  bool
	RawIntCarryValue       bool
	TableResultRawTablePtr bool

	// Region, mutation, and metatable invalidation contracts.
	TableArrayRegionGlobalBarrier  bool
	TableArrayRegionAliasingCall   bool
	TableArrayRegionAliasingAlways bool
	TableArrayRegionTableMutation  bool
	TableMetatableMutationBarrier  bool

	// Runtime-specialization and inferred result contracts.
	RuntimeOverflowBoxable     bool
	RuntimeGuardRefreshable    bool
	NativeNumericValueProducer bool
	PureNumericUnknownValue    bool
	TableArraySwapPureBetween  bool
	StaticTableLenBenignUse    bool
	FixedResultType            Type
	ProvesNonNilResult         bool
	GuardProvenResultType      Type
	RawFloatValueProducer      bool
	FieldFactWideKiller        bool

	// Fact invalidation and fallback target contracts.
	TableMutationFirstArg       bool
	CallLikeFactBarrier         bool
	RawCarryClobber             bool
	ExactDivComponent           bool
	IntNarrowCandidate          bool
	IntNarrowAllArgsConstraint  bool
	FieldNumFusionGapSafe       bool
	RawIntSpecializationBlocker bool
	RawIntSpecializedOp         Op
	ExactIntNarrowOp            Op
	BoxedFallbackOp             Op
	BoxedFallbackResultUnknown  bool

	// Compact enum policies.
	BackendPolicy        OpBackendPolicy
	SourceFeedbackPolicy OpSourceFeedbackPolicy
	RangeRefineKind      OpRangeRefineKind
}

func opSpec(name string, family OpEmitterFamily, args OpArgPolicy, effect OpSideEffect, mayDeopt bool) OpSpec {
	return OpSpec{
		Name:                       name,
		SideEffect:                 effect,
		ArgPolicy:                  args,
		SuccCount:                  OpCountAny,
		EmitterFamily:              family,
		MayDeopt:                   mayDeopt,
		TableArrayGPRInvariantRank: 1,
		TableArrayKeyArgIndex:      -1,
		TypeSpecializeIntOp:        OpMax,
		TypeSpecializeFloatOp:      OpMax,
		TypeSpecializeStringOp:     OpMax,
		CallUserArgStart:           -1,
		ExactIntNarrowOp:           OpMax,
		BoxedFallbackOp:            OpMax,
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
