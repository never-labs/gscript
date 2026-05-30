//go:build darwin && arm64

package methodjit

import (
	"strings"

	"github.com/Never-Labs/gscript/internal/vm"
)

// SpecializedABIKind names the entry/return convention a function can use.
// The analysis result is consumed by codegen to decide whether to emit and use
// the raw-int self-recursive ABI; non-eligible functions stay on the boxed VM
// ABI.
type SpecializedABIKind uint8

const (
	SpecializedABINone SpecializedABIKind = iota
	SpecializedABIRawInt
	SpecializedABITyped
)

// SpecializedABIParamRep describes how one fixed parameter is represented at
// a specialized entry point.

type SpecializedABIParamRep uint8

const (
	SpecializedABIParamBoxed SpecializedABIParamRep = iota
	SpecializedABIParamRawInt
	SpecializedABIParamRawFloat
	SpecializedABIParamRawTablePtr
)

// SpecializedABIReturnRep describes the result representation at a specialized
// return point.

type SpecializedABIReturnRep uint8

const (
	SpecializedABIReturnNone SpecializedABIReturnRep = iota
	SpecializedABIReturnBoxed
	SpecializedABIReturnRawInt
	SpecializedABIReturnRawFloat
	SpecializedABIReturnRawTablePtr
)

// SpecializedABI is the analysis result for a candidate specialized entry.

type SpecializedABI struct {
	Kind      SpecializedABIKind
	Params    []SpecializedABIParamRep
	Return    SpecializedABIReturnRep
	Eligible  bool
	RejectWhy string
}

func specializedABIParamName(rep SpecializedABIParamRep) string {
	switch rep {
	case SpecializedABIParamRawInt:
		return "raw-int"
	case SpecializedABIParamRawFloat:
		return "raw-float"
	case SpecializedABIParamRawTablePtr:
		return "raw-table"
	case SpecializedABIParamBoxed:
		return "boxed"
	default:
		return "unknown"
	}
}

func typedABISignature(abi TypedSelfABI) uint64 {
	if !abi.Eligible {
		return 0
	}
	sig := uint64(0x54414249) // "TABI"
	sig = sig*131 + uint64(abi.NumParams)
	sig = sig*131 + uint64(abi.Return)
	for _, rep := range abi.Params {
		sig = sig*131 + uint64(rep+1)
	}
	return sig
}

func specializedABIParamSummary(params []SpecializedABIParamRep) string {
	if len(params) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(params))
	for _, rep := range params {
		parts = append(parts, specializedABIParamName(rep))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func specializedABIReturnName(rep SpecializedABIReturnRep) string {
	switch rep {
	case SpecializedABIReturnNone:
		return "none"
	case SpecializedABIReturnBoxed:
		return "boxed"
	case SpecializedABIReturnRawInt:
		return "raw-int"
	case SpecializedABIReturnRawFloat:
		return "raw-float"
	case SpecializedABIReturnRawTablePtr:
		return "raw-table"
	default:
		return "unknown"
	}
}

// RawIntSelfABI is the compact codegen contract for the private numeric
// recursive entry. It started as the self-recursive entry descriptor and still
// carries that name for compatibility, but it also covers structurally pure
// numeric self+peer recursion accepted by qualifiesForNumericCrossRecursiveCandidate.

type RawIntSelfABI struct {
	Eligible   bool
	NumParams  int
	ParamSlots []int
	Return     SpecializedABIReturnRep
	RejectWhy  string
}

// TypedSelfABI is the private method-JIT ABI for fixed-shape recursive
// specializations whose hot recursive edge can avoid the boxed VM CALL convention.
// Parameters are passed in X0..X3 as raw int64 or *runtime.Table pointers.
// The return value is delivered in X0 with the representation named by Return.

type TypedSelfABI struct {
	Eligible   bool
	NumParams  int
	ParamSlots []int
	Params     []SpecializedABIParamRep
	Return     SpecializedABIReturnRep
	RejectWhy  string
}

type specializedSlotRep uint8

const (
	specializedSlotUnknown specializedSlotRep = iota
	specializedSlotRawInt
	specializedSlotRawFloat
	specializedSlotRawTable
	specializedSlotRawString
	specializedSlotNil
	specializedSlotSelfCallRawInt
	specializedSlotSelfCallRawFloat
	specializedSlotSelfCallRawTable
	specializedSlotSelfFunc
	specializedSlotOtherFunc
	specializedSlotStdMathTable
	specializedSlotMathSqrtFunc
	specializedSlotMathFloorFunc
)

// AnalyzeSpecializedABI recognizes generic raw-int ABI candidates. It is
// intentionally not tied to any one workload: a candidate must have fixed
// integer parameters, a single integer return, and bytecode operations whose
// values can be represented as raw int64/int48 along recursive call edges.

func specializedABIReject(reason string) SpecializedABI {
	return SpecializedABI{
		Kind:      SpecializedABINone,
		Return:    SpecializedABIReturnBoxed,
		RejectWhy: reason,
	}
}

func specializedABIBranchTargets(code []uint32) map[int]bool {
	targets := make(map[int]bool)
	for pc, inst := range code {
		switch vm.DecodeOp(inst) {
		case vm.OP_JMP:
			tgt := pc + 1 + vm.DecodesBx(inst)
			if tgt >= 0 && tgt < len(code) {
				targets[tgt] = true
			}
		case vm.OP_EQ, vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_TESTSET:
			tgt := pc + 2
			if tgt >= 0 && tgt < len(code) {
				targets[tgt] = true
			}
		case vm.OP_FORPREP, vm.OP_FORLOOP:
			tgt := pc + 1 + vm.DecodesBx(inst)
			if tgt >= 0 && tgt < len(code) {
				targets[tgt] = true
			}
		}
	}
	return targets
}

func specializedABIRKIsRawInt(slots []specializedSlotRep, proto *vm.FuncProto, idx int) bool {
	if idx >= vm.RKBit {
		return specializedABIConstIsInt(proto, idx-vm.RKBit)
	}
	return specializedABIRepIsRawInt(getSpecializedSlot(slots, idx))
}

func specializedABIRepIsRawInt(rep specializedSlotRep) bool {
	return rep == specializedSlotRawInt || rep == specializedSlotSelfCallRawInt
}

func specializedABIDynamicRecursiveArgsAreRawInt(slots []specializedSlotRep, callSlot, numParams int) bool {
	if numParams <= 0 {
		return false
	}
	for arg := callSlot + 1; arg < callSlot+numParams; arg++ {
		if !specializedABIRepIsRawInt(getSpecializedSlot(slots, arg)) {
			return false
		}
	}
	return getSpecializedSlot(slots, callSlot+numParams) == specializedSlotSelfCallRawInt
}

func specializedABIConstIsInt(proto *vm.FuncProto, idx int) bool {
	return idx >= 0 && idx < len(proto.Constants) && proto.Constants[idx].IsInt()
}

func specializedABIConstIsFloat(proto *vm.FuncProto, idx int) bool {
	return idx >= 0 && idx < len(proto.Constants) && proto.Constants[idx].IsFloat()
}

func specializedABIConstString(proto *vm.FuncProto, idx int) string {
	if idx < 0 || idx >= len(proto.Constants) {
		return ""
	}
	return proto.Constants[idx].Str()
}

func getSpecializedSlot(slots []specializedSlotRep, idx int) specializedSlotRep {
	if idx < 0 || idx >= len(slots) {
		return specializedSlotUnknown
	}
	return slots[idx]
}

func setSpecializedSlot(slots []specializedSlotRep, idx int, rep specializedSlotRep) {
	if idx >= 0 && idx < len(slots) {
		slots[idx] = rep
	}
}
