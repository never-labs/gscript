package methodjit

func opSpec(name string, family OpEmitterFamily, args OpArgPolicy, effect OpSideEffect, mayDeopt bool) OpSpec {
	return OpSpec{
		Name:                            name,
		SideEffect:                      effect,
		ArgPolicy:                       args,
		SuccCount:                       OpCountAny,
		EmitterFamily:                   family,
		MayDeopt:                        mayDeopt,
		OracleSupport:                   OpOracleExecutable,
		TableArrayGPRInvariantRank:      1,
		TableArrayKeyArgIndex:           -1,
		TableArrayTableArgIndex:         -1,
		TableArrayDataArgIndex:          -1,
		TableArrayLenArgIndex:           -1,
		LoadElimTableCacheKeyArgIndex:   -1,
		LoadElimTableCacheValueArgIndex: -1,
		TypeSpecializeIntOp:             OpMax,
		TypeSpecializeFloatOp:           OpMax,
		TypeSpecializeStringOp:          OpMax,
		CallUserArgStart:                -1,
		ExactIntNarrowOp:                OpMax,
		BoxedFallbackOp:                 OpMax,
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
