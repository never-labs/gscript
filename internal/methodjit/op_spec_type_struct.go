package methodjit

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
	OracleSupport OpOracleSupport
	// OracleUnsupportedReason explains why the IR interpreter oracle cannot
	// execute this op. It is required when OracleSupport is OpOracleUnsupported.
	OracleUnsupportedReason string

	// Runtime replay, deopt, and side-effect contracts.
	NativeReplayMayExit              bool
	NativeReplayVisibleSideEffect    bool
	NativeReplayVisibleTableMutation bool
	NativeCalleeResumeUnsafe         bool
	RestartVisibleSideEffect         bool

	// Field, shape, and load-elimination contracts.
	FieldShapeSplitInlineSafe         bool
	FieldShapePreEffectInlineSafe     bool
	FieldShapeInlineSideEffect        bool
	FieldShapePostEffectInlineUnsafe  bool
	GlobalConstUnsafe                 bool
	NestedCallLike                    bool
	LoadElimConstCSE                  bool
	LiteralConst                      bool
	LoadElimPureCSE                   bool
	LoadElimShapeFactKiller           bool
	LoadElimDynamicTableCacheMutation bool
	LoadElimTypedArrayFactMutation    bool
	LoadElimTableCacheKeyArgIndex     int
	LoadElimTableCacheValueArgIndex   int
	LoadElimFactBarrier               bool

	// Result representation and value-lowering contracts.
	NoSSAResult                       bool
	RawIntResult                      bool
	RawTablePtrResult                 bool
	RawDataPtrResult                  bool
	RawFloatResult                    bool
	MatrixNative                      bool
	MatrixLoweredOp                   Op
	MatrixRowLoweredOp                Op
	MatrixRowConstLoweredOp           Op
	TableArrayGPRInvariant            bool
	TableArrayGPRInvariantRank        int
	TableArrayGPRInvariantUseMask     uint8
	TableArrayKeyArgIndex             int
	TableArrayTableArgIndex           int
	TableArrayDataArgIndex            int
	TableArrayLenArgIndex             int
	TableArrayLoweredOp               Op
	ClosureScalarLocalUseArgIndex     int
	ClosureScalarLoadClosureArgIndex  int
	ClosureScalarStoreClosureArgIndex int
	ClosureScalarStoreValueArgIndex   int
	TableArrayFactRole                OpTableArrayFactRole
	TableIntArraySwapPairsBodyBenign  bool
	TableIntArrayCopyPrefixBodyBenign bool
	TableIntArrayReverseBodyBenign    bool
	FixedShapeArrayElementWriteRole   OpFixedShapeArrayElementWriteRole
	FixedShapeArrayElementReadRole    OpFixedShapeArrayElementReadRole
	FixedShapeReturnArrayElementRole  OpFixedShapeReturnArrayElementRole
	LocalStringArrayTableUseRole      OpLocalStringArrayTableUseRole
	LocalStringArrayTableArgIndex     int
	ReadonlyTableParamUseRole         OpReadonlyTableParamUseRole
	InlineAllocationRole              OpInlineAllocationRole

	// Optimizer admission and numeric specialization contracts.
	LICMHoistable               bool
	LICMInterestingMiss         bool
	LICMIntArith                bool
	LICMLoopEffectRole          OpLICMLoopEffectRole
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
	FieldSvalsCrossBlockBarrier       bool
	FieldSvalsGlobalBarrier           bool
	FieldSvalsFirstArgMutationBarrier bool
	FieldSvalsLoweredOp               Op
	FieldLenFoldBarrier               bool
	FieldCallPolyLenFusionBarrier     bool
	FieldNumFusionLoweredOp           Op

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
	BoolTableFillStoreTableArg    int
	BoolTableFillStoreKeyArg      int
	BoolTableFillStoreValueArg    int
	BoolTableFillStoreKindSource  OpBoolTableFillKindSource
	BoolTableCountLoadBodyBenign  bool
	BoolTableCountLoad            bool
	BoolTableCountIncrementBenign bool
	BoolTableCountIncrement       bool
	CallResultRangeGuardCandidate bool
	ModuloReducibleCallFloor      bool
	CallFloorProjectionOp         Op
	FieldCallFloorProjectionOp    Op
	CallFloorSpecStableCallee     bool
	CallFloorSpecFieldShape       bool
	Tier2LoopCall                 bool
	Tier2LoopFeedbackVMProtoCall  bool
	Tier2ResidualCallBlocker      bool
	Tier2LoopNativeCandidate      bool
	TableArrayStoreLoopCandidate  bool
	TableArrayStoreLoopBlocker    bool
	TableArrayStoreLoopEscapeCall bool
	TableArrayStoreLoopUseOK      bool
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
	StaticTableLenBuilder      bool
	StaticTableLenInitializer  bool
	StaticTableLenInvalidator  bool
	ClosureScalarLocalUseAny   bool
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
