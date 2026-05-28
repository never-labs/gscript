package methodjit

func opSpec(name string, family OpEmitterFamily, args OpArgPolicy, effect OpSideEffect, mayDeopt bool) OpSpec {
	return OpSpec{
		Name:                              name,
		SideEffect:                        effect,
		ArgPolicy:                         args,
		SuccCount:                         OpCountAny,
		EmitterFamily:                     family,
		MayDeopt:                          mayDeopt,
		OracleSupport:                     OpOracleExecutable,
		TableArrayGPRInvariantRank:        1,
		TableArrayKeyArgIndex:             -1,
		TableArrayTableArgIndex:           -1,
		TableArrayDataArgIndex:            -1,
		TableArrayLenArgIndex:             -1,
		TableArrayLoweredOp:               OpMax,
		ClosureScalarLocalUseArgIndex:     -1,
		ClosureScalarLoadClosureArgIndex:  -1,
		ClosureScalarStoreClosureArgIndex: -1,
		ClosureScalarStoreValueArgIndex:   -1,
		LocalStringArrayTableArgIndex:     -1,
		BoolTableFillStoreTableArg:        -1,
		BoolTableFillStoreKeyArg:          -1,
		BoolTableFillStoreValueArg:        -1,
		LoadElimTableCacheKeyArgIndex:     -1,
		LoadElimTableCacheValueArgIndex:   -1,
		TypeSpecializeIntOp:               OpMax,
		TypeSpecializeFloatOp:             OpMax,
		TypeSpecializeStringOp:            OpMax,
		RawIntSpecializedOp:               OpMax,
		FieldSvalsLoweredOp:               OpMax,
		FieldNumFusionLoweredOp:           OpMax,
		CallUserArgStart:                  -1,
		ExactIntNarrowOp:                  OpMax,
		MatrixLoweredOp:                   OpMax,
		MatrixRowLoweredOp:                OpMax,
		MatrixRowConstLoweredOp:           OpMax,
		BoxedFallbackOp:                   OpMax,
		CallFloorProjectionOp:             OpMax,
		FieldCallFloorProjectionOp:        OpMax,
	}
}

func opTerminatorSpec(name string, args OpArgPolicy, argCount OpCountPolicy, succCount int) OpSpec {
	spec := opSpec(name, OpEmitterControl, args, OpSideEffectControl, false)
	spec.Terminator = true
	spec.OracleSupport = OpOracleTerminator
	spec.ArgCount = argCount
	spec.SuccCount = succCount
	return spec
}

func opSpecArgCount(spec OpSpec, count OpCountPolicy) OpSpec {
	spec.ArgCount = count
	return spec
}
