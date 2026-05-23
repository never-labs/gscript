package vm

import "github.com/gscript/gscript/internal/runtime"

const (
	runtimeSpecializationRawIntNested = iota
	runtimeSpecializationLazyRecursiveTableBuilder
	runtimeSpecializationLazyRecursiveTableFold
	runtimeSpecializationPermutationFlipChecksum
	runtimeSpecializationIntGridAggregate
	runtimeSpecializationMatrixMultiply
	runtimeSpecializationRecordWalkFold
	runtimeSpecializationBoolTableStrikeCount
	runtimeSpecializationCount
)

const (
	wholeCallNoResultRuntimeSpecializationRecordPairwiseNumeric = iota
	wholeCallNoResultRuntimeSpecializationNumericArrayRegionSort
	wholeCallNoResultRuntimeSpecializationDenseMatrixMultiplyTransposed
	wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixVector
	wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixTransposeVector
	wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixAtAVector
	wholeCallNoResultRuntimeSpecializationSpectralDenseCoefficientMatrixAtAVector
	wholeCallNoResultRuntimeSpecializationCount
)

const (
	driverLoopRuntimeSpecializationGenericRecordArrayLoop = iota
	driverLoopRuntimeSpecializationRecordPairwiseNumericLoop
	driverLoopRuntimeSpecializationCount
)

type runtimeSpecializationProtoCache struct {
	fingerprint runtimeSpecializationFingerprint
	recognized  uint64
}

// RuntimeSpecialization is the shared recognizer metadata for guarded runtime
// specializations. A specialization may still perform runtime dependency
// checks before handling a call.
type RuntimeSpecialization struct {
	Info      RuntimeSpecializationInfo
	Recognize func(*FuncProto) bool
}

// WholeCallValueSpecialization handles OP_CALL sites that return values.
type WholeCallValueSpecialization struct {
	RuntimeSpecialization
	Run            wholeCallValueRuntimeSpecializationRunner
	RecursiveTable bool
}

// WholeCallNoResultSpecialization handles OP_CALL sites using the no-result
// convention for in-place kernels.
type WholeCallNoResultSpecialization struct {
	RuntimeSpecialization
	Run wholeCallNoResultRuntimeSpecializationRunner
}

type driverLoopRuntimeSpecializationRunner func(*VM, *CallFrame, int, []uint32, []runtime.Value, int, int) (bool, error)

// DriverLoopRuntimeSpecialization handles OP_FORPREP sites whose admission
// depends on runtime values in addition to structural bytecode shape.
type DriverLoopRuntimeSpecialization struct {
	Info      RuntimeSpecializationInfo
	Recognize func(*FuncProto, map[string]*FuncProto) bool
	Run       driverLoopRuntimeSpecializationRunner
}

var wholeCallValueRuntimeSpecializationRegistry = [runtimeSpecializationCount]WholeCallValueSpecialization{
	runtimeSpecializationRawIntNested: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "nested_int_recurrence",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         2,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: IsRawIntNestedKernelProto,
		},
		Run: (*VM).runRawIntNestedValueKernel,
	},
	runtimeSpecializationLazyRecursiveTableBuilder: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "lazy_recursive_table_builder",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         1,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: IsLazyRecursiveTableBuilderKernelProto,
		},
		Run:            (*VM).tryRunRecursiveTableValueKernel,
		RecursiveTable: true,
	},
	runtimeSpecializationLazyRecursiveTableFold: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "lazy_recursive_table_fold",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         1,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: IsLazyRecursiveTableFoldKernelProto,
		},
		Run:            (*VM).tryRunRecursiveTableValueKernel,
		RecursiveTable: true,
	},
	runtimeSpecializationPermutationFlipChecksum: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "permutation_flip_checksum",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         1,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isPermutationFlipChecksumKernelProto,
		},
		Run: (*VM).runPermutationFlipChecksumRuntimeSpecialization,
	},
	runtimeSpecializationIntGridAggregate: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "int_grid_aggregate",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         2,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isIntGridAggregateProto,
		},
		Run: (*VM).runIntGridAggregateRuntimeSpecialization,
	},
	runtimeSpecializationMatrixMultiply: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "matrix_multiply",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         3,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isMatrixMultiplyProto,
		},
		Run: (*VM).runMatrixMultiplyRuntimeSpecialization,
	},
	runtimeSpecializationRecordWalkFold: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "record_walk_fold",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         3,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isRecordWalkFoldProto,
		},
		Run: (*VM).runRecordWalkFoldRuntimeSpecialization,
	},
	runtimeSpecializationBoolTableStrikeCount: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "bool_table_strike_count",
				Route:         RuntimeSpecializationRouteWholeCallValue,
				Arity:         1,
				Results:       runtimeSpecializationWholeCallSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isBoolTableStrikeCountProto,
		},
		Run: (*VM).runBoolTableStrikeCountRuntimeSpecialization,
	},
}

var wholeCallNoResultRuntimeSpecializationRegistry = [wholeCallNoResultRuntimeSpecializationCount]WholeCallNoResultSpecialization{
	wholeCallNoResultRuntimeSpecializationRecordPairwiseNumeric: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "record_pairwise_numeric",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         1,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isRecordPairwiseNumericProto,
		},
		Run: (*VM).runRecordPairwiseNumericKernel,
	},
	wholeCallNoResultRuntimeSpecializationNumericArrayRegionSort: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "numeric_array_region_sort",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         3,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isNumericArrayRegionSortProto,
		},
		Run: (*VM).runNumericArrayRegionSortRuntimeSpecialization,
	},
	wholeCallNoResultRuntimeSpecializationDenseMatrixMultiplyTransposed: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "dense_matrix_multiply_transposed",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         4,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isDenseMatrixMultiplyTransposedProto,
		},
		Run: (*VM).runDenseMatrixMultiplyTransposedRuntimeSpecialization,
	},
	wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coefficient_matrix_vector",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         3,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSpectralAvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixTransposeVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coefficient_matrix_transpose_vector",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         3,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSpectralAtvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	wholeCallNoResultRuntimeSpecializationSpectralCoefficientMatrixAtAVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coefficient_matrix_ata_vector",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         3,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSpectralAtAvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	wholeCallNoResultRuntimeSpecializationSpectralDenseCoefficientMatrixAtAVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "dense_coefficient_matrix_ata_vector",
				Route:         RuntimeSpecializationRouteWholeCallNoResult,
				Arity:         4,
				Results:       runtimeSpecializationWholeCallInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isDenseSpectralAtAvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
}

var driverLoopRuntimeSpecializationRegistry = [driverLoopRuntimeSpecializationCount]DriverLoopRuntimeSpecialization{
	driverLoopRuntimeSpecializationGenericRecordArrayLoop: {
		Info: RuntimeSpecializationInfo{
			Name:          "generic_record_array_loop",
			Route:         RuntimeSpecializationRouteDriverLoop,
			Arity:         runtimeSpecializationUnknownDriverLoopArity,
			Results:       runtimeSpecializationUnknownDriverLoopResultCount,
			TieringPolicy: runtimeSpecializationTieringStructural,
		},
		Recognize: HasGenericRecordArrayDriverLoopKernel,
		Run:       (*VM).tryGenericRecordArrayForLoopKernel,
	},
	driverLoopRuntimeSpecializationRecordPairwiseNumericLoop: {
		Info: RuntimeSpecializationInfo{
			Name:          "record_pairwise_numeric_loop",
			Route:         RuntimeSpecializationRouteDriverLoop,
			Arity:         runtimeSpecializationUnknownDriverLoopArity,
			Results:       runtimeSpecializationUnknownDriverLoopResultCount,
			TieringPolicy: runtimeSpecializationTieringStructural,
		},
		Recognize: HasRecordPairwiseNumericDriverLoopKernel,
		Run:       (*VM).tryRecordPairwiseNumericForLoopKernel,
	},
}

func (vm *VM) tryRunWholeCallValueRuntimeSpecialization(cl *Closure, args []runtime.Value, includeRecursiveTable bool) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil {
		return false, nil, nil
	}
	if !mayHaveWholeCallValueRuntimeSpecializationCandidate(cl.Proto, len(args), includeRecursiveTable) {
		return false, nil, nil
	}
	recognized := cachedRuntimeSpecializationBits(cl.Proto)
	if recognized == 0 {
		return false, nil, nil
	}
	for i, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if recognized&(uint64(1)<<uint(i)) == 0 || entry.Run == nil {
			continue
		}
		if entry.RecursiveTable && !includeRecursiveTable {
			continue
		}
		handled, results, err := entry.Run(vm, cl, args)
		if handled || err != nil {
			if handled {
				runtime.RecordRuntimePathRuntimeSpecializationHit(string(entry.Info.Route), entry.Info.Name)
			}
			return handled, results, err
		}
	}
	return false, nil, nil
}

func (vm *VM) tryRunWholeCallNoResultRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, error) {
	if cl == nil || cl.Proto == nil {
		return false, nil
	}
	if !mayHaveWholeCallNoResultRuntimeSpecializationCandidate(cl.Proto, len(args)) {
		return false, nil
	}
	recognized := cachedWholeCallNoResultRuntimeSpecializationBits(cl.Proto)
	if recognized == 0 {
		return false, nil
	}
	for i, entry := range wholeCallNoResultRuntimeSpecializationRegistry {
		if recognized&(uint64(1)<<uint(i)) == 0 || entry.Run == nil {
			continue
		}
		handled, err := entry.Run(vm, cl, args)
		if handled || err != nil {
			if handled {
				runtime.RecordRuntimePathRuntimeSpecializationHit(string(entry.Info.Route), entry.Info.Name)
			}
			return handled, err
		}
	}
	return false, nil
}

func (vm *VM) tryRunDriverLoopRuntimeSpecialization(frame *CallFrame, base int, code []uint32, constants []runtime.Value, a int, sbx int) (bool, error) {
	if frame == nil || frame.closure == nil || frame.closure.Proto == nil {
		return false, nil
	}
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Run == nil {
			continue
		}
		handled, err := entry.Run(vm, frame, base, code, constants, a, sbx)
		if handled || err != nil {
			if handled {
				runtime.RecordRuntimePathRuntimeSpecializationHit(string(entry.Info.Route), entry.Info.Name)
			}
			return handled, err
		}
	}
	return false, nil
}

func mayHaveWholeCallValueRuntimeSpecializationCandidate(proto *FuncProto, argc int, includeRecursiveTable bool) bool {
	if proto == nil || proto.IsVarArg {
		return false
	}
	for _, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.RecursiveTable && !includeRecursiveTable {
			continue
		}
		if entry.Info.Arity == argc && proto.NumParams == argc {
			return true
		}
	}
	return false
}

func mayHaveWholeCallNoResultRuntimeSpecializationCandidate(proto *FuncProto, argc int) bool {
	if proto == nil || proto.IsVarArg {
		return false
	}
	for _, entry := range wholeCallNoResultRuntimeSpecializationRegistry {
		if entry.Info.Arity == argc && proto.NumParams == argc {
			return true
		}
	}
	return false
}

func recognizedRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	return runtimeSpecializationCacheForProto(proto).recognized
}

func recognizedWholeCallNoResultRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	return wholeCallNoResultRuntimeSpecializationCacheForProto(proto).recognized
}

func cachedRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	if cache := proto.RuntimeSpecialization; cache != nil {
		return cache.recognized
	}
	return runtimeSpecializationCacheForProto(proto).recognized
}

func cachedWholeCallNoResultRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	if cache := proto.WholeCallNoResultRuntime; cache != nil {
		return cache.recognized
	}
	return wholeCallNoResultRuntimeSpecializationCacheForProto(proto).recognized
}

func cachedRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(wholeCallValueRuntimeSpecializationRegistry) {
		return false
	}
	return cachedRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func cachedWholeCallNoResultRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(wholeCallNoResultRuntimeSpecializationRegistry) {
		return false
	}
	return cachedWholeCallNoResultRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func hotWholeCallNoResultRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(wholeCallNoResultRuntimeSpecializationRegistry) {
		return false
	}
	return cachedWholeCallNoResultRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func runtimeSpecializationCacheForProto(proto *FuncProto) *runtimeSpecializationProtoCache {
	fp := runtimeSpecializationFingerprintForProto(proto)
	cache := proto.RuntimeSpecialization
	if cache != nil && cache.fingerprint == fp {
		return cache
	}
	cache = &runtimeSpecializationProtoCache{fingerprint: fp}
	for i, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Recognize == nil {
			continue
		}
		if entry.Recognize(proto) {
			cache.recognized |= uint64(1) << uint(i)
		}
	}
	proto.RuntimeSpecialization = cache
	return cache
}

func wholeCallNoResultRuntimeSpecializationCacheForProto(proto *FuncProto) *runtimeSpecializationProtoCache {
	fp := runtimeSpecializationFingerprintForProto(proto)
	cache := proto.WholeCallNoResultRuntime
	if cache != nil && cache.fingerprint == fp {
		return cache
	}
	cache = &runtimeSpecializationProtoCache{fingerprint: fp}
	for i, entry := range wholeCallNoResultRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Recognize == nil {
			continue
		}
		if entry.Recognize(proto) {
			cache.recognized |= uint64(1) << uint(i)
		}
	}
	proto.WholeCallNoResultRuntime = cache
	return cache
}
