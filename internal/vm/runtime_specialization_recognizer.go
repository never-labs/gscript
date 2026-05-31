package vm

import (
	"math"

	"github.com/never-labs/gscript/internal/runtime"
)

// RuntimeSpecializationRoute identifies how a structural runtime specialization
// is reached. Call-site routes are probed at OP_CALL; driver-loop routes are
// probed at OP_FORPREP.
type RuntimeSpecializationRoute string

const (
	RuntimeSpecializationRouteCallSiteValue    RuntimeSpecializationRoute = "call_site_value"
	RuntimeSpecializationRouteCallSiteNoResult RuntimeSpecializationRoute = "call_site_no_result"
	RuntimeSpecializationRouteDriverLoop       RuntimeSpecializationRoute = "driver_loop"
)

// RuntimeSpecializationCapability identifies cross-package behavior exposed by
// structural runtime specialization metadata.
type RuntimeSpecializationCapability uint64

const (
	RuntimeSpecializationCapabilityStructuralTiering RuntimeSpecializationCapability = 1 << iota
)

// Has reports whether all bits in want are present.
func (c RuntimeSpecializationCapability) Has(want RuntimeSpecializationCapability) bool {
	return c&want == want
}

// RuntimeSpecializationTieringPolicy describes whether a recognized runtime
// specialization should keep its bytecode on the VM structural-specialization
// route instead of entering methodjit.
type RuntimeSpecializationTieringPolicy struct {
	Capabilities         RuntimeSpecializationCapability
	RequireFloatConstant bool
}

// AllowsStructuralTiering reports whether this policy permits structural
// runtime-specialization tiering for proto.
func (p RuntimeSpecializationTieringPolicy) AllowsStructuralTiering(proto *FuncProto) bool {
	if !p.Capabilities.Has(RuntimeSpecializationCapabilityStructuralTiering) {
		return false
	}
	if p.RequireFloatConstant && !runtimeSpecializationProtoHasFloatConstant(proto) {
		return false
	}
	return true
}

var (
	runtimeSpecializationTieringStructural = RuntimeSpecializationTieringPolicy{
		Capabilities: RuntimeSpecializationCapabilityStructuralTiering,
	}
	runtimeSpecializationTieringStructuralWithFloatConstant = RuntimeSpecializationTieringPolicy{
		Capabilities:         RuntimeSpecializationCapabilityStructuralTiering,
		RequireFloatConstant: true,
	}
)

// RuntimeSpecializationInfo is stable diagnostic metadata for structural VM runtime specializations.
//
// Recognizers intentionally inspect only bytecode, constants, arity, and
// guarded callee shapes. FuncProto.Name and FuncProto.Source are debugging
// metadata and are not part of runtime-specialization admission.
type RuntimeSpecializationInfo struct {
	Name          string
	Route         RuntimeSpecializationRoute
	Arity         int
	Results       int
	TieringPolicy RuntimeSpecializationTieringPolicy
}

// HasCapability reports whether this runtime specialization advertises cap.
func (info RuntimeSpecializationInfo) HasCapability(cap RuntimeSpecializationCapability) bool {
	return info.TieringPolicy.Capabilities.Has(cap)
}

// AllowsStructuralTiering reports whether this runtime specialization can keep
// a recognized proto on the VM structural-specialization route.
func (info RuntimeSpecializationInfo) AllowsStructuralTiering(proto *FuncProto) bool {
	return info.TieringPolicy.AllowsStructuralTiering(proto)
}

// RuntimeSpecializationDiagnostic reports whether one registered structural
// runtime specialization recognizes a prototype and, if not, the broad fallback
// reason.
type RuntimeSpecializationDiagnostic struct {
	Specialization RuntimeSpecializationInfo
	Recognized     bool
	Reason         string
}

const (
	runtimeSpecializationReasonRecognized             = "recognized_structural_bytecode"
	runtimeSpecializationReasonNilProto               = "nil_proto"
	runtimeSpecializationReasonShapeMismatch          = "bytecode_or_constant_shape_mismatch"
	runtimeSpecializationReasonDriverRecognized       = "recognized_structural_driver_loop"
	runtimeSpecializationReasonDriverMismatch         = "bytecode_or_callee_shape_mismatch"
	runtimeSpecializationReasonMissingGlobalProtoMap  = "missing_global_proto_map"
	runtimeSpecializationUnknownDriverLoopArity       = -1
	runtimeSpecializationUnknownDriverLoopResultCount = -1
	runtimeSpecializationCallSiteInPlaceResultCount   = 0
	runtimeSpecializationCallSiteSingleResultCount    = 1
)

type runtimeSpecializationFingerprint struct {
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

type callSiteValueRuntimeSpecializationRunner func(*VM, *Closure, []runtime.Value) (bool, []runtime.Value, error)
type callSiteNoResultRuntimeSpecializationRunner func(*VM, *Closure, []runtime.Value) (bool, error)

// CallSiteRuntimeSpecializationCatalog returns diagnostic metadata for OP_CALL
// structural runtime specializations without probing any particular prototype.
func CallSiteRuntimeSpecializationCatalog() []RuntimeSpecializationInfo {
	out := make([]RuntimeSpecializationInfo, 0, len(callSiteValueRuntimeSpecializationRegistry)+len(callSiteNoResultRuntimeSpecializationRegistry))
	for _, entry := range callSiteValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		out = append(out, entry.Info)
	}
	for _, entry := range callSiteNoResultRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		out = append(out, entry.Info)
	}
	return out
}

// DriverLoopRuntimeSpecializationCatalog returns diagnostic metadata for
// OP_FORPREP driver-loop runtime specializations without probing any particular
// prototype.
func DriverLoopRuntimeSpecializationCatalog() []RuntimeSpecializationInfo {
	out := make([]RuntimeSpecializationInfo, 0, len(driverLoopRuntimeSpecializationRegistry))
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		out = append(out, entry.Info)
	}
	return out
}

// RecognizedCallSiteRuntimeSpecializations returns every registered call-site runtime specialization whose
// structural recognizer accepts p. It does not inspect FuncProto.Name or Source.
func RecognizedCallSiteRuntimeSpecializations(p *FuncProto) []RuntimeSpecializationInfo {
	out := make([]RuntimeSpecializationInfo, 0, 1)
	runtimeRecognized := recognizedRuntimeSpecializationBits(p)
	for i, entry := range callSiteValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		if runtimeRecognized&(uint64(1)<<uint(i)) != 0 {
			out = append(out, entry.Info)
		}
	}
	noResultRuntimeRecognized := recognizedCallSiteNoResultRuntimeSpecializationBits(p)
	for i, entry := range callSiteNoResultRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		if noResultRuntimeRecognized&(uint64(1)<<uint(i)) != 0 {
			out = append(out, entry.Info)
		}
	}
	return out
}

// DiagnoseCallSiteRuntimeSpecializationProto reports structural recognizer results for every
// registered call-site runtime specialization. It is intended for tests and diagnostics, not
// hot dispatch.
func DiagnoseCallSiteRuntimeSpecializationProto(p *FuncProto) []RuntimeSpecializationDiagnostic {
	out := make([]RuntimeSpecializationDiagnostic, 0, len(callSiteValueRuntimeSpecializationRegistry)+len(callSiteNoResultRuntimeSpecializationRegistry))
	runtimeRecognizedBits := recognizedRuntimeSpecializationBits(p)
	for i, entry := range callSiteValueRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		recognized := runtimeRecognizedBits&(uint64(1)<<uint(i)) != 0
		out = append(out, RuntimeSpecializationDiagnostic{
			Specialization: entry.Info,
			Recognized:     recognized,
			Reason:         runtimeSpecializationReason(p, recognized),
		})
	}
	noResultRuntimeRecognizedBits := recognizedCallSiteNoResultRuntimeSpecializationBits(p)
	for i, entry := range callSiteNoResultRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		recognized := noResultRuntimeRecognizedBits&(uint64(1)<<uint(i)) != 0
		out = append(out, RuntimeSpecializationDiagnostic{
			Specialization: entry.Info,
			Recognized:     recognized,
			Reason:         runtimeSpecializationReason(p, recognized),
		})
	}
	return out
}

func runtimeSpecializationFingerprintForProto(proto *FuncProto) runtimeSpecializationFingerprint {
	var fp runtimeSpecializationFingerprint
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

func runtimeSpecializationProtoHasFloatConstant(proto *FuncProto) bool {
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

// RecognizedDriverLoopRuntimeSpecializations returns every registered driver-loop runtime specialization whose
// structural recognizer accepts proto with the supplied global callee map.
func RecognizedDriverLoopRuntimeSpecializations(proto *FuncProto, globals map[string]*FuncProto) []RuntimeSpecializationInfo {
	out := make([]RuntimeSpecializationInfo, 0, len(driverLoopRuntimeSpecializationRegistry))
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

// DiagnoseDriverLoopRuntimeSpecializations reports structural driver-loop recognizer results.
// The globals map should contain compile-time global function protos by name.
func DiagnoseDriverLoopRuntimeSpecializations(proto *FuncProto, globals map[string]*FuncProto) []RuntimeSpecializationDiagnostic {
	out := make([]RuntimeSpecializationDiagnostic, 0, len(driverLoopRuntimeSpecializationRegistry))
	for _, entry := range driverLoopRuntimeSpecializationRegistry {
		if entry.Info.Name == "" {
			continue
		}
		recognized := proto != nil && entry.Recognize != nil && entry.Recognize(proto, globals)
		out = append(out, RuntimeSpecializationDiagnostic{
			Specialization: entry.Info,
			Recognized:     recognized,
			Reason:         driverLoopRuntimeSpecializationReason(proto, globals, recognized),
		})
	}
	return out
}

func runtimeSpecializationReason(proto *FuncProto, recognized bool) string {
	if proto == nil {
		return runtimeSpecializationReasonNilProto
	}
	if recognized {
		return runtimeSpecializationReasonRecognized
	}
	return runtimeSpecializationReasonShapeMismatch
}

func driverLoopRuntimeSpecializationReason(proto *FuncProto, globals map[string]*FuncProto, recognized bool) string {
	if proto == nil {
		return runtimeSpecializationReasonNilProto
	}
	if recognized {
		return runtimeSpecializationReasonDriverRecognized
	}
	if len(globals) == 0 {
		return runtimeSpecializationReasonMissingGlobalProtoMap
	}
	return runtimeSpecializationReasonDriverMismatch
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
