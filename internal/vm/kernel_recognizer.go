package vm

import (
	"math"

	"github.com/gscript/gscript/internal/runtime"
)

// KernelRoute identifies how a structural kernel is reached. Whole-call
// routes are probed at OP_CALL; driver-loop routes are probed at OP_FORPREP.
type KernelRoute string

const (
	KernelRouteWholeCallValue    KernelRoute = "whole_call_value"
	KernelRouteWholeCallNoResult KernelRoute = "whole_call_no_result"
	KernelRouteDriverLoop        KernelRoute = "driver_loop"
)

// KernelCapability identifies cross-package behavior exposed by structural
// kernel metadata.
type KernelCapability uint64

const (
	KernelCapabilityStructuralTiering KernelCapability = 1 << iota
)

// Has reports whether all bits in want are present.
func (c KernelCapability) Has(want KernelCapability) bool {
	return c&want == want
}

// KernelTieringPolicy describes whether a recognized kernel should keep its
// bytecode on the VM structural-kernel route instead of entering methodjit.
type KernelTieringPolicy struct {
	Capabilities         KernelCapability
	RequireFloatConstant bool
}

// AllowsStructuralTiering reports whether this policy permits structural
// kernel tiering for proto.
func (p KernelTieringPolicy) AllowsStructuralTiering(proto *FuncProto) bool {
	if !p.Capabilities.Has(KernelCapabilityStructuralTiering) {
		return false
	}
	if p.RequireFloatConstant && !kernelProtoHasFloatConstant(proto) {
		return false
	}
	return true
}

var (
	kernelTieringStructural = KernelTieringPolicy{
		Capabilities: KernelCapabilityStructuralTiering,
	}
	kernelTieringStructuralWithFloatConstant = KernelTieringPolicy{
		Capabilities:         KernelCapabilityStructuralTiering,
		RequireFloatConstant: true,
	}
)

// KernelInfo is stable diagnostic metadata for structural VM kernels.
//
// Recognizers intentionally inspect only bytecode, constants, arity, and
// guarded callee shapes. FuncProto.Name and FuncProto.Source are debugging
// metadata and are not part of kernel admission.
type KernelInfo struct {
	Name          string
	Route         KernelRoute
	Arity         int
	Results       int
	TieringPolicy KernelTieringPolicy
}

// HasCapability reports whether this kernel advertises cap.
func (info KernelInfo) HasCapability(cap KernelCapability) bool {
	return info.TieringPolicy.Capabilities.Has(cap)
}

// AllowsStructuralTiering reports whether this kernel can keep a recognized
// proto on the VM structural-kernel route.
func (info KernelInfo) AllowsStructuralTiering(proto *FuncProto) bool {
	return info.TieringPolicy.AllowsStructuralTiering(proto)
}

// KernelDiagnostic reports whether one registered structural kernel recognizes
// a prototype and, if not, the broad fallback reason.
type KernelDiagnostic struct {
	Kernel     KernelInfo
	Recognized bool
	Reason     string
}

const (
	kernelReasonRecognized             = "recognized_structural_bytecode"
	kernelReasonNilProto               = "nil_proto"
	kernelReasonShapeMismatch          = "bytecode_or_constant_shape_mismatch"
	kernelReasonDriverRecognized       = "recognized_structural_driver_loop"
	kernelReasonDriverMismatch         = "bytecode_or_callee_shape_mismatch"
	kernelReasonMissingGlobalProtoMap  = "missing_global_proto_map"
	kernelUnknownDriverLoopArity       = -1
	kernelUnknownDriverLoopResultCount = -1
	kernelWholeCallInPlaceResultCount  = 0
	kernelWholeCallSingleResultCount   = 1
)

const (
	wholeCallKernelRecordWalkFold = iota
	wholeCallKernelIntGridAggregate
	wholeCallKernelPermutationFlipChecksum
	wholeCallKernelBoolTableStrikeCount
	wholeCallKernelMatrixMultiply
	wholeCallKernelRecordPairwiseNumeric
	wholeCallKernelNumericArrayRegionSort
	wholeCallKernelCoefficientMatrixVector
	wholeCallKernelCoefficientMatrixTransposeVector
	wholeCallKernelCoefficientMatrixAtAVector
	wholeCallKernelDenseCoefficientMatrixAtAVector
	wholeCallKernelDenseMatrixMultiplyTransposed
	wholeCallKernelCount
)

type wholeCallKernelFingerprint struct {
	numParams    int
	isVarArg     bool
	maxStack     int
	codeLen      int
	constLen     int
	protoLen     int
	upvalueLen   int
	tableCtorLen int
	hash         uint64
}

type wholeCallKernelProtoCache struct {
	fingerprint wholeCallKernelFingerprint
	recognized  uint64
}

type wholeCallValueKernelRunner func(*VM, *Closure, []runtime.Value) (bool, []runtime.Value, error)
type wholeCallNoResultKernelRunner func(*VM, *Closure, []runtime.Value) (bool, error)

type wholeCallKernelRecognizer struct {
	info        KernelInfo
	recognize   func(*FuncProto) bool
	runValue    wholeCallValueKernelRunner
	runNoResult wholeCallNoResultKernelRunner
}

var wholeCallKernelRegistry = [wholeCallKernelCount]wholeCallKernelRecognizer{
	{
		info: KernelInfo{
			Name:          "record_walk_fold",
			Route:         KernelRouteWholeCallValue,
			Arity:         3,
			Results:       kernelWholeCallSingleResultCount,
			TieringPolicy: kernelTieringStructural,
		},
		recognize: isRecordWalkFoldProto,
		runValue:  (*VM).runRecordWalkFoldWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "int_grid_aggregate",
			Route:         KernelRouteWholeCallValue,
			Arity:         2,
			Results:       kernelWholeCallSingleResultCount,
			TieringPolicy: kernelTieringStructural,
		},
		recognize: isIntGridAggregateProto,
		runValue:  (*VM).runIntGridAggregateWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "permutation_flip_checksum",
			Route:         KernelRouteWholeCallValue,
			Arity:         1,
			Results:       kernelWholeCallSingleResultCount,
			TieringPolicy: kernelTieringStructural,
		},
		recognize: isPermutationFlipChecksumKernelProto,
		runValue:  (*VM).runPermutationFlipChecksumWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "bool_table_strike_count",
			Route:         KernelRouteWholeCallValue,
			Arity:         1,
			Results:       kernelWholeCallSingleResultCount,
			TieringPolicy: kernelTieringStructural,
		},
		recognize: isBoolTableStrikeCountProto,
		runValue:  (*VM).runBoolTableStrikeCountWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "matrix_multiply",
			Route:         KernelRouteWholeCallValue,
			Arity:         3,
			Results:       kernelWholeCallSingleResultCount,
			TieringPolicy: kernelTieringStructural,
		},
		recognize: isMatrixMultiplyProto,
		runValue:  (*VM).runMatrixMultiplyWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "record_pairwise_numeric",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         1,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   isRecordPairwiseNumericProto,
		runNoResult: (*VM).runRecordPairwiseNumericKernel,
	},
	{
		info: KernelInfo{
			Name:          "numeric_array_region_sort",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         3,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   isNumericArrayRegionSortProto,
		runNoResult: (*VM).runNumericArrayRegionSortWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "coefficient_matrix_vector",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         3,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   func(p *FuncProto) bool { return classifySpectralMultiplyProto(p) == spectralAv },
		runNoResult: (*VM).runSpectralWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "coefficient_matrix_transpose_vector",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         3,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   func(p *FuncProto) bool { return classifySpectralMultiplyProto(p) == spectralAtv },
		runNoResult: (*VM).runSpectralWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "coefficient_matrix_ata_vector",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         3,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   isSpectralAtAvProto,
		runNoResult: (*VM).runSpectralWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "dense_coefficient_matrix_ata_vector",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         4,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   isDenseSpectralAtAvProto,
		runNoResult: (*VM).runSpectralWholeCallKernel,
	},
	{
		info: KernelInfo{
			Name:          "dense_matrix_multiply_transposed",
			Route:         KernelRouteWholeCallNoResult,
			Arity:         4,
			Results:       kernelWholeCallInPlaceResultCount,
			TieringPolicy: kernelTieringStructuralWithFloatConstant,
		},
		recognize:   isDenseMatrixMultiplyTransposedProto,
		runNoResult: (*VM).runDenseMatrixMultiplyTransposedWholeCallKernel,
	},
}

// WholeCallKernelCatalog returns diagnostic metadata for OP_CALL structural
// kernels without probing any particular prototype.
func WholeCallKernelCatalog() []KernelInfo {
	out := make([]KernelInfo, 0, len(wholeCallValueRuntimeSpecializationRegistry)+len(wholeCallKernelRegistry))
	for _, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		out = append(out, entry.Info)
	}
	for _, entry := range wholeCallKernelRegistry {
		if entry.info.Name == "" {
			continue
		}
		out = append(out, entry.info)
	}
	return out
}

// DriverLoopKernelCatalog returns diagnostic metadata for OP_FORPREP driver
// loop kernels without probing any particular prototype.
func DriverLoopKernelCatalog() []KernelInfo {
	out := make([]KernelInfo, 0, len(driverLoopRuntimeSpecializationRegistry))
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		out = append(out, entry.Info)
	}
	return out
}

// RecognizedWholeCallKernels returns every registered whole-call kernel whose
// structural recognizer accepts p. It does not inspect FuncProto.Name or Source.
func RecognizedWholeCallKernels(p *FuncProto) []KernelInfo {
	out := make([]KernelInfo, 0, 1)
	runtimeRecognized := recognizedRuntimeSpecializationBits(p)
	for i, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		if runtimeRecognized&(uint64(1)<<uint(i)) != 0 {
			out = append(out, entry.Info)
		}
	}
	recognized := recognizedWholeCallKernelBits(p)
	for i, entry := range wholeCallKernelRegistry {
		if entry.info.Name == "" {
			continue
		}
		if recognized&(uint64(1)<<uint(i)) != 0 {
			out = append(out, entry.info)
		}
	}
	return out
}

// DiagnoseWholeCallKernelProto reports structural recognizer results for every
// registered whole-call kernel. It is intended for tests and diagnostics, not
// hot dispatch.
func DiagnoseWholeCallKernelProto(p *FuncProto) []KernelDiagnostic {
	out := make([]KernelDiagnostic, 0, len(wholeCallValueRuntimeSpecializationRegistry)+len(wholeCallKernelRegistry))
	runtimeRecognizedBits := recognizedRuntimeSpecializationBits(p)
	for i, entry := range wholeCallValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		recognized := runtimeRecognizedBits&(uint64(1)<<uint(i)) != 0
		out = append(out, KernelDiagnostic{
			Kernel:     entry.Info,
			Recognized: recognized,
			Reason:     wholeCallKernelReason(p, recognized),
		})
	}
	recognizedBits := recognizedWholeCallKernelBits(p)
	for i, entry := range wholeCallKernelRegistry {
		if entry.info.Name == "" {
			continue
		}
		recognized := recognizedBits&(uint64(1)<<uint(i)) != 0
		out = append(out, KernelDiagnostic{
			Kernel:     entry.info,
			Recognized: recognized,
			Reason:     wholeCallKernelReason(p, recognized),
		})
	}
	return out
}

func recognizedWholeCallKernelBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	return wholeCallKernelCacheForProto(proto).recognized
}

// cachedWholeCallKernelBits is for OP_CALL hot dispatch. FuncProto bytecode is
// immutable after compilation, while diagnostics use recognizedWholeCallKernelBits
// to keep mutation-oriented tests exact.
func cachedWholeCallKernelBits(proto *FuncProto) uint64 {
	if proto == nil {
		return 0
	}
	if cache := proto.WholeCallKernel; cache != nil {
		return cache.recognized
	}
	return wholeCallKernelCacheForProto(proto).recognized
}

func cachedWholeCallKernelRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(wholeCallKernelRegistry) {
		return false
	}
	return cachedWholeCallKernelBits(proto)&(uint64(1)<<uint(id)) != 0
}

func hotWholeCallKernelRecognized(proto *FuncProto, id int) bool {
	if id < 0 || id >= len(wholeCallKernelRegistry) {
		return false
	}
	return cachedWholeCallKernelBits(proto)&(uint64(1)<<uint(id)) != 0
}

func wholeCallKernelCacheForProto(proto *FuncProto) *wholeCallKernelProtoCache {
	fp := wholeCallKernelFingerprintForProto(proto)
	cache := proto.WholeCallKernel
	if cache != nil && cache.fingerprint == fp {
		return cache
	}
	cache = &wholeCallKernelProtoCache{fingerprint: fp}
	for i, entry := range wholeCallKernelRegistry {
		if entry.info.Name == "" || entry.recognize == nil {
			continue
		}
		if entry.recognize(proto) {
			cache.recognized |= uint64(1) << uint(i)
		}
	}
	proto.WholeCallKernel = cache
	return cache
}

func wholeCallKernelFingerprintForProto(proto *FuncProto) wholeCallKernelFingerprint {
	var fp wholeCallKernelFingerprint
	if proto == nil {
		return fp
	}
	fp.numParams = proto.NumParams
	fp.isVarArg = proto.IsVarArg
	fp.maxStack = proto.MaxStack
	fp.codeLen = len(proto.Code)
	fp.constLen = len(proto.Constants)
	fp.protoLen = len(proto.Protos)
	fp.upvalueLen = len(proto.Upvalues)
	fp.tableCtorLen = len(proto.TableCtors2)

	h := uint64(1469598103934665603)
	h = fnvMixUint64(h, uint64(fp.numParams))
	if fp.isVarArg {
		h = fnvMixUint64(h, 1)
	}
	h = fnvMixUint64(h, uint64(fp.maxStack))
	h = fnvMixUint64(h, uint64(fp.upvalueLen))
	for _, inst := range proto.Code {
		h = fnvMixUint64(h, uint64(inst))
	}
	for _, c := range proto.Constants {
		h = fnvMixRuntimeValue(h, c)
	}
	for _, ctor := range proto.TableCtors2 {
		h = fnvMixInt(h, ctor.Key1Const)
		h = fnvMixInt(h, ctor.Key2Const)
		h = fnvMixString(h, ctor.Runtime.Key1)
		h = fnvMixString(h, ctor.Runtime.Key2)
	}
	fp.hash = h
	return fp
}

func fnvMixRuntimeValue(h uint64, v runtime.Value) uint64 {
	h = fnvMixUint64(h, uint64(v.Type()))
	switch {
	case v.IsString():
		return fnvMixString(h, v.Str())
	case v.IsInt():
		return fnvMixUint64(h, uint64(v.Int()))
	case v.IsFloat():
		return fnvMixUint64(h, math.Float64bits(v.Float()))
	case v.IsBool():
		if v.Bool() {
			return fnvMixUint64(h, 1)
		}
		return fnvMixUint64(h, 0)
	default:
		return h
	}
}

func kernelProtoHasFloatConstant(proto *FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, c := range proto.Constants {
		if c.IsFloat() {
			return true
		}
	}
	return false
}

func fnvMixInt(h uint64, v int) uint64 {
	return fnvMixUint64(h, uint64(int64(v)))
}

func fnvMixString(h uint64, s string) uint64 {
	h = fnvMixUint64(h, uint64(len(s)))
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

func fnvMixUint64(h uint64, v uint64) uint64 {
	for i := 0; i < 8; i++ {
		h ^= uint64(byte(v))
		h *= 1099511628211
		v >>= 8
	}
	return h
}

// RecognizedDriverLoopKernels returns every registered driver-loop kernel whose
// structural recognizer accepts proto with the supplied global callee map.
func RecognizedDriverLoopKernels(proto *FuncProto, globals map[string]*FuncProto) []KernelInfo {
	out := make([]KernelInfo, 0, len(driverLoopRuntimeSpecializationRegistry))
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" || entry.Recognize == nil {
			continue
		}
		if entry.Recognize(proto, globals) {
			out = append(out, entry.Info)
		}
	}
	return out
}

// DiagnoseDriverLoopKernels reports structural driver-loop recognizer results.
// The globals map should contain compile-time global function protos by name.
func DiagnoseDriverLoopKernels(proto *FuncProto, globals map[string]*FuncProto) []KernelDiagnostic {
	out := make([]KernelDiagnostic, 0, len(driverLoopRuntimeSpecializationRegistry))
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		recognized := proto != nil && entry.Recognize != nil && entry.Recognize(proto, globals)
		out = append(out, KernelDiagnostic{
			Kernel:     entry.Info,
			Recognized: recognized,
			Reason:     driverLoopKernelReason(proto, globals, recognized),
		})
	}
	return out
}

func wholeCallKernelReason(proto *FuncProto, recognized bool) string {
	if proto == nil {
		return kernelReasonNilProto
	}
	if recognized {
		return kernelReasonRecognized
	}
	return kernelReasonShapeMismatch
}

func driverLoopKernelReason(proto *FuncProto, globals map[string]*FuncProto, recognized bool) string {
	if proto == nil {
		return kernelReasonNilProto
	}
	if recognized {
		return kernelReasonDriverRecognized
	}
	if len(globals) == 0 {
		return kernelReasonMissingGlobalProtoMap
	}
	return kernelReasonDriverMismatch
}

func codeEquals(got, want []uint32) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func numberConst(v runtime.Value, want float64) bool {
	return v.IsNumber() && v.Number() == want
}

func stringConst(constants []runtime.Value, idx int) bool {
	return idx >= 0 && idx < len(constants) && constants[idx].IsString()
}

func addOperandsMatch(inst uint32, left int, right int) bool {
	b := DecodeB(inst)
	c := DecodeC(inst)
	return (b == left && c == right) || (b == right && c == left)
}
