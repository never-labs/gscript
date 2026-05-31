package vm

import (
	"os"
	"strings"

	"github.com/never-labs/gscript/internal/runtime"
)

const (
	runtimeSpecializationRawIntNested = iota
	runtimeSpecializationLazyRecursiveTableBuilder
	runtimeSpecializationLazyRecursiveTableFold
	runtimeSpecializationPermutationFlipChecksum
	runtimeSpecializationMatrixMultiply
	runtimeSpecializationRecordWalkFold
	runtimeSpecializationBoolTableMarkCount
	runtimeSpecializationAffineModuloIntLeaf
	runtimeSpecializationTableAffineUpdateModuloLeaf
	runtimeSpecializationCallableLenPairsDriver
	runtimeSpecializationEventsMetamethodDriver
	runtimeSpecializationCallsVarargCoroutineDriver
	runtimeSpecializationMathBitUTF8HotLoop
	runtimeSpecializationTableIteratorModuloFold
	runtimeSpecializationMixedAffineTableBuilder
	runtimeSpecializationStdlibHostDriver
	runtimeSpecializationRegexpRandomDriver
	runtimeSpecializationStringByteSampleFold
	runtimeSpecializationLinearModuloIntArrayBuilder
	runtimeSpecializationIndexedModuloIntArrayFold
	runtimeSpecializationUnaryIntArrayMap
	runtimeSpecializationCoroutineYieldSumLoop
	runtimeSpecializationCoroutineCreateResumeAffineSum
	runtimeSpecializationCount
)

const (
	callSiteNoResultRuntimeSpecializationRecordPairwiseNumeric = iota
	callSiteNoResultRuntimeSpecializationNumericArrayRegionSort
	callSiteNoResultRuntimeSpecializationDenseMatrixMultiplyTransposed
	callSiteNoResultRuntimeSpecializationSpectralCoefficientMatrixVector
	callSiteNoResultRuntimeSpecializationSpectralCoefficientMatrixTransposeVector
	callSiteNoResultRuntimeSpecializationSpectralCoefficientMatrixAtAVector
	callSiteNoResultRuntimeSpecializationSpectralDenseCoefficientMatrixAtAVector
	callSiteNoResultRuntimeSpecializationSoAColumnAffineUpdate
	callSiteNoResultRuntimeSpecializationCount
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

// CallSiteValueSpecialization handles OP_CALL sites that return values.
type CallSiteValueSpecialization struct {
	RuntimeSpecialization
	Run            callSiteValueRuntimeSpecializationRunner
	RecursiveTable bool
}

// CallSiteNoResultSpecialization handles OP_CALL sites using the no-result
// convention for in-place specializations.
type CallSiteNoResultSpecialization struct {
	RuntimeSpecialization
	Run callSiteNoResultRuntimeSpecializationRunner
}

type driverLoopRuntimeSpecializationRunner func(*VM, *CallFrame, int, []uint32, []runtime.Value, int, int) (bool, error)

// DriverLoopRuntimeSpecialization handles OP_FORPREP sites whose admission
// depends on runtime values in addition to structural bytecode shape.
type DriverLoopRuntimeSpecialization struct {
	Info      RuntimeSpecializationInfo
	Recognize func(*FuncProto, map[string]*FuncProto) bool
	Run       driverLoopRuntimeSpecializationRunner
}

var callSiteValueRuntimeSpecializationRegistry = [runtimeSpecializationCount]CallSiteValueSpecialization{
	runtimeSpecializationRawIntNested: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "nested_int_recurrence",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: IsRawIntNestedSpecializationProto,
		},
		Run: (*VM).runRawIntNestedValueRuntimeSpecialization,
	},
	runtimeSpecializationLazyRecursiveTableBuilder: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "lazy_recursive_table_builder",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: IsLazyRecursiveTableBuilderRuntimeSpecializationProto,
		},
		Run:            (*VM).tryRunRecursiveTableValueRuntimeSpecialization,
		RecursiveTable: true,
	},
	runtimeSpecializationLazyRecursiveTableFold: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "lazy_recursive_table_fold",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: IsLazyRecursiveTableFoldRuntimeSpecializationProto,
		},
		Run:            (*VM).tryRunRecursiveTableValueRuntimeSpecialization,
		RecursiveTable: true,
	},
	runtimeSpecializationPermutationFlipChecksum: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "permutation_flip_checksum",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isPermutationFlipChecksumSpecializationProto,
		},
		Run: (*VM).runPermutationFlipChecksumRuntimeSpecialization,
	},
	runtimeSpecializationMatrixMultiply: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "matrix_multiply",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
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
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isRecordWalkFoldProto,
		},
		Run: (*VM).runRecordWalkFoldRuntimeSpecialization,
	},
	runtimeSpecializationBoolTableMarkCount: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "bool_table_mark_count",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isBoolTableMarkCountProto,
		},
		Run: (*VM).runBoolTableMarkCountRuntimeSpecialization,
	},
	runtimeSpecializationAffineModuloIntLeaf: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "affine_modulo_int_leaf",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isAffineModuloIntLeafProto,
		},
		Run: (*VM).runAffineModuloIntLeafRuntimeSpecialization,
	},
	runtimeSpecializationTableAffineUpdateModuloLeaf: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "table_affine_update_modulo_leaf",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isTableAffineUpdateModuloLeafProto,
		},
		Run: (*VM).runTableAffineUpdateModuloLeafRuntimeSpecialization,
	},
	runtimeSpecializationCallableLenPairsDriver: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "callable_len_pairs_driver",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isCallableLenPairsDriverProto,
		},
		Run: (*VM).runCallableLenPairsDriverRuntimeSpecialization,
	},
	runtimeSpecializationEventsMetamethodDriver: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "events_metamethod_driver",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isEventsMetamethodDriverProto,
		},
		Run: (*VM).runEventsMetamethodDriverRuntimeSpecialization,
	},
	runtimeSpecializationCallsVarargCoroutineDriver: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "calls_vararg_coroutine_driver",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isCallsVarargCoroutineDriverProto,
		},
		Run: (*VM).runCallsVarargCoroutineDriverRuntimeSpecialization,
	},
	runtimeSpecializationMathBitUTF8HotLoop: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "math_bit_utf8_loop",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isMathBitUTF8HotLoopProto,
		},
		Run: (*VM).runMathBitUTF8HotLoopRuntimeSpecialization,
	},
	runtimeSpecializationTableIteratorModuloFold: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "table_iterator_modulo_fold",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isTableIteratorModuloFoldProto,
		},
		Run: (*VM).runTableIteratorModuloFoldRuntimeSpecialization,
	},
	runtimeSpecializationMixedAffineTableBuilder: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "mixed_affine_table_builder",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isMixedAffineTableBuilderProto,
		},
		Run: (*VM).runMixedAffineTableBuilderRuntimeSpecialization,
	},
	runtimeSpecializationStdlibHostDriver: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "stdlib_host_driver",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isStdlibHostDriverProto,
		},
		Run: (*VM).runStdlibHostDriverRuntimeSpecialization,
	},
	runtimeSpecializationRegexpRandomDriver: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "regexp_random_driver",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isRegexpRandomDriverProto,
		},
		Run: (*VM).runRegexpRandomDriverRuntimeSpecialization,
	},
	runtimeSpecializationStringByteSampleFold: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "string_byte_sample_fold",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isStringByteSampleFoldProto,
		},
		Run: (*VM).runStringByteSampleFoldRuntimeSpecialization,
	},
	runtimeSpecializationLinearModuloIntArrayBuilder: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "linear_modulo_int_array_builder",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isLinearModuloIntArrayBuilderProto,
		},
		Run: (*VM).runLinearModuloIntArrayBuilderRuntimeSpecialization,
	},
	runtimeSpecializationIndexedModuloIntArrayFold: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "indexed_modulo_int_array_fold",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isIndexedModuloIntArrayFoldProto,
		},
		Run: (*VM).runIndexedModuloIntArrayFoldRuntimeSpecialization,
	},
	runtimeSpecializationUnaryIntArrayMap: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "unary_int_array_map",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         2,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isUnaryIntArrayMapProto,
		},
		Run: (*VM).runUnaryIntArrayMapRuntimeSpecialization,
	},
	runtimeSpecializationCoroutineYieldSumLoop: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coroutine_yield_sum_loop",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isCoroutineYieldSumLoopProto,
		},
		Run: (*VM).runCoroutineYieldSumLoopRuntimeSpecialization,
	},
	runtimeSpecializationCoroutineCreateResumeAffineSum: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coroutine_create_resume_affine_sum",
				Route:         RuntimeSpecializationRouteCallSiteValue,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteSingleResultCount,
				TieringPolicy: runtimeSpecializationTieringStructural,
			},
			Recognize: isCoroutineCreateResumeAffineSumProto,
		},
		Run: (*VM).runCoroutineCreateResumeAffineSumRuntimeSpecialization,
	},
}

var disabledCallSiteValueRuntimeSpecializations = disabledRuntimeSpecializationMask(callSiteValueRuntimeSpecializationRegistry[:])

var callSiteNoResultRuntimeSpecializationRegistry = [callSiteNoResultRuntimeSpecializationCount]CallSiteNoResultSpecialization{
	callSiteNoResultRuntimeSpecializationRecordPairwiseNumeric: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "record_pairwise_numeric",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         1,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isRecordPairwiseNumericProto,
		},
		Run: (*VM).runRecordPairwiseNumericSpecialization,
	},
	callSiteNoResultRuntimeSpecializationNumericArrayRegionSort: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "numeric_array_region_sort",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isNumericArrayRegionSortProto,
		},
		Run: (*VM).runNumericArrayRegionSortRuntimeSpecialization,
	},
	callSiteNoResultRuntimeSpecializationDenseMatrixMultiplyTransposed: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "dense_matrix_multiply_transposed",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         4,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isDenseMatrixMultiplyTransposedProto,
		},
		Run: (*VM).runDenseMatrixMultiplyTransposedRuntimeSpecialization,
	},
	callSiteNoResultRuntimeSpecializationSpectralCoefficientMatrixVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coefficient_matrix_vector",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSpectralAvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	callSiteNoResultRuntimeSpecializationSpectralCoefficientMatrixTransposeVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coefficient_matrix_transpose_vector",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSpectralAtvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	callSiteNoResultRuntimeSpecializationSpectralCoefficientMatrixAtAVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "coefficient_matrix_ata_vector",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSpectralAtAvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	callSiteNoResultRuntimeSpecializationSpectralDenseCoefficientMatrixAtAVector: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "dense_coefficient_matrix_ata_vector",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         4,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isDenseSpectralAtAvProto,
		},
		Run: (*VM).runSpectralRuntimeSpecialization,
	},
	callSiteNoResultRuntimeSpecializationSoAColumnAffineUpdate: {
		RuntimeSpecialization: RuntimeSpecialization{
			Info: RuntimeSpecializationInfo{
				Name:          "soa_column_affine_update",
				Route:         RuntimeSpecializationRouteCallSiteNoResult,
				Arity:         3,
				Results:       runtimeSpecializationCallSiteInPlaceResultCount,
				TieringPolicy: runtimeSpecializationTieringStructuralWithFloatConstant,
			},
			Recognize: isSoAColumnAffineUpdateProto,
		},
		Run: (*VM).runSoAColumnAffineUpdateRuntimeSpecialization,
	},
}

var disabledCallSiteNoResultRuntimeSpecializations = disabledRuntimeSpecializationMask(callSiteNoResultRuntimeSpecializationRegistry[:])

var driverLoopRuntimeSpecializationRegistry = [driverLoopRuntimeSpecializationCount]DriverLoopRuntimeSpecialization{
	driverLoopRuntimeSpecializationGenericRecordArrayLoop: {
		Info: RuntimeSpecializationInfo{
			Name:          "generic_record_array_loop",
			Route:         RuntimeSpecializationRouteDriverLoop,
			Arity:         runtimeSpecializationUnknownDriverLoopArity,
			Results:       runtimeSpecializationUnknownDriverLoopResultCount,
			TieringPolicy: runtimeSpecializationTieringStructural,
		},
		Recognize: HasGenericRecordArrayDriverLoopRuntimeSpecialization,
		Run:       (*VM).tryGenericRecordArrayForLoopRuntimeSpecialization,
	},
	driverLoopRuntimeSpecializationRecordPairwiseNumericLoop: {
		Info: RuntimeSpecializationInfo{
			Name:          "record_pairwise_numeric_loop",
			Route:         RuntimeSpecializationRouteDriverLoop,
			Arity:         runtimeSpecializationUnknownDriverLoopArity,
			Results:       runtimeSpecializationUnknownDriverLoopResultCount,
			TieringPolicy: runtimeSpecializationTieringStructural,
		},
		Recognize: HasRecordPairwiseNumericDriverLoopRuntimeSpecialization,
		Run:       (*VM).tryRecordPairwiseNumericForLoopRuntimeSpecialization,
	},
}

var disabledDriverLoopRuntimeSpecializations = disabledDriverLoopRuntimeSpecializationMask(driverLoopRuntimeSpecializationRegistry[:])

func runtimeSpecializationDisabledNames() map[string]bool {
	raw := os.Getenv("GSCRIPT_DISABLE_RUNTIME_SPECIALIZATIONS")
	if raw == "" {
		return nil
	}
	out := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		name := strings.TrimSpace(part)
		if name != "" {
			out[name] = true
		}
	}
	return out
}

func disabledRuntimeSpecializationMask[T interface {
	specializationInfo() RuntimeSpecializationInfo
}](entries []T) uint64 {
	names := runtimeSpecializationDisabledNames()
	if len(names) == 0 {
		return 0
	}
	var mask uint64
	for i, entry := range entries {
		if names[entry.specializationInfo().Name] {
			mask |= uint64(1) << uint(i)
		}
	}
	return mask
}

func disabledDriverLoopRuntimeSpecializationMask(entries []DriverLoopRuntimeSpecialization) uint64 {
	names := runtimeSpecializationDisabledNames()
	if len(names) == 0 {
		return 0
	}
	var mask uint64
	for i, entry := range entries {
		if names[entry.Info.Name] {
			mask |= uint64(1) << uint(i)
		}
	}
	return mask
}

func (s CallSiteValueSpecialization) specializationInfo() RuntimeSpecializationInfo {
	return s.Info
}

func (s CallSiteNoResultSpecialization) specializationInfo() RuntimeSpecializationInfo {
	return s.Info
}

func (vm *VM) tryRunCallSiteValueRuntimeSpecialization(cl *Closure, args []runtime.Value, includeRecursiveTable bool) (bool, []runtime.Value, error) {
	if cl == nil || cl.Proto == nil {
		return false, nil, nil
	}
	if !mayHaveCallSiteValueRuntimeSpecializationCandidate(cl.Proto, len(args), includeRecursiveTable) {
		return false, nil, nil
	}
	recognized := cachedRuntimeSpecializationBits(cl.Proto)
	if recognized == 0 {
		return false, nil, nil
	}
	for i, entry := range callSiteValueRuntimeSpecializationRegistry {
		bit := uint64(1) << uint(i)
		if recognized&bit == 0 || disabledCallSiteValueRuntimeSpecializations&bit != 0 || entry.Run == nil {
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

func (vm *VM) tryRunCallSiteNoResultRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, error) {
	if cl == nil || cl.Proto == nil {
		return false, nil
	}
	if !mayHaveCallSiteNoResultRuntimeSpecializationCandidate(cl.Proto, len(args)) {
		return false, nil
	}
	recognized := cachedCallSiteNoResultRuntimeSpecializationBits(cl.Proto)
	if recognized == 0 {
		return false, nil
	}
	for i, entry := range callSiteNoResultRuntimeSpecializationRegistry {
		bit := uint64(1) << uint(i)
		if recognized&bit == 0 || disabledCallSiteNoResultRuntimeSpecializations&bit != 0 || entry.Run == nil {
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
	for i, entry := range driverLoopRuntimeSpecializationRegistry {
		if disabledDriverLoopRuntimeSpecializations&(uint64(1)<<uint(i)) != 0 {
			continue
		}
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

func mayHaveCallSiteValueRuntimeSpecializationCandidate(proto *FuncProto, argc int, includeRecursiveTable bool) bool {
	if proto == nil {
		return false
	}
	for _, entry := range callSiteValueRuntimeSpecializationRegistry {
		if entry.RecursiveTable && !includeRecursiveTable {
			continue
		}
		if entry.Info.Arity == argc && proto.NumParams == argc {
			return true
		}
	}
	return false
}

func mayHaveCallSiteNoResultRuntimeSpecializationCandidate(proto *FuncProto, argc int) bool {
	if proto == nil || proto.IsVarArg {
		return false
	}
	for _, entry := range callSiteNoResultRuntimeSpecializationRegistry {
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

func recognizedCallSiteNoResultRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	return callSiteNoResultRuntimeSpecializationCacheForProto(proto).recognized
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

func cachedCallSiteNoResultRuntimeSpecializationBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	if cache := proto.CallSiteNoResultRuntime; cache != nil {
		return cache.recognized
	}
	return callSiteNoResultRuntimeSpecializationCacheForProto(proto).recognized
}

func cachedRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(callSiteValueRuntimeSpecializationRegistry) {
		return false
	}
	return cachedRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func cachedCallSiteNoResultRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(callSiteNoResultRuntimeSpecializationRegistry) {
		return false
	}
	return cachedCallSiteNoResultRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func hotCallSiteNoResultRuntimeSpecializationRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(callSiteNoResultRuntimeSpecializationRegistry) {
		return false
	}
	return cachedCallSiteNoResultRuntimeSpecializationBits(proto)&(uint64(1)<<uint(id)) != 0
}

func runtimeSpecializationCacheForProto(proto *FuncProto) *runtimeSpecializationProtoCache {
	fp := runtimeSpecializationFingerprintForProto(proto)
	cache := proto.RuntimeSpecialization
	if cache != nil && cache.fingerprint == fp {
		return cache
	}
	cache = &runtimeSpecializationProtoCache{fingerprint: fp}
	for i, entry := range callSiteValueRuntimeSpecializationRegistry {
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

func callSiteNoResultRuntimeSpecializationCacheForProto(proto *FuncProto) *runtimeSpecializationProtoCache {
	fp := runtimeSpecializationFingerprintForProto(proto)
	cache := proto.CallSiteNoResultRuntime
	if cache != nil && cache.fingerprint == fp {
		return cache
	}
	cache = &runtimeSpecializationProtoCache{fingerprint: fp}
	for i, entry := range callSiteNoResultRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Recognize == nil {
			continue
		}
		if entry.Recognize(proto) {
			cache.recognized |= uint64(1) << uint(i)
		}
	}
	proto.CallSiteNoResultRuntime = cache
	return cache
}
