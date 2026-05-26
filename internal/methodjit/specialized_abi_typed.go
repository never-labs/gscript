//go:build darwin && arm64

// specialized_abi_typed.go: typed-peer specialized ABI analysis entry points and
// eligibility for the Method JIT. Covers the analyzeTypedABI* / AnalyzeTypedSelfABI /
// AnalyzeTypedPeerABI* family, slot-usage bounds, and typed parameter inference.
// Pure code movement from specialized_abi.go; no behavior change.

package methodjit

import (
	"fmt"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func AnalyzeTypedSelfABI(proto *vm.FuncProto) TypedSelfABI {
	abi := analyzeTypedABI(proto, true)
	if abi.Eligible && abi.Return == SpecializedABIReturnNone {
		return typedSelfABIReject("zero-result typed self ABI is disabled")
	}
	return abi
}

func analyzeTypedABI(proto *vm.FuncProto, requireSelfCall bool) TypedSelfABI {
	return analyzeTypedABIWithArgFacts(proto, requireSelfCall, nil)
}

func analyzeTypedABIWithArgFacts(proto *vm.FuncProto, requireSelfCall bool, argFacts map[int]FixedShapeTableFact) TypedSelfABI {
	return analyzeTypedABIWithFacts(proto, requireSelfCall, argFacts, nil)
}

func analyzeTypedABIWithFacts(proto *vm.FuncProto, requireSelfCall bool, argFacts map[int]FixedShapeTableFact, arrayElementArgFacts map[int]FixedShapeTableFact) TypedSelfABI {
	return analyzeTypedABIWithFactsAndGlobals(proto, requireSelfCall, argFacts, arrayElementArgFacts, nil, nil)
}

func analyzeTypedABIWithFactsAndGlobals(proto *vm.FuncProto, requireSelfCall bool, argFacts map[int]FixedShapeTableFact, arrayElementArgFacts map[int]FixedShapeTableFact, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) TypedSelfABI {
	if proto == nil {
		return typedSelfABIReject("nil proto")
	}
	if proto.IsVarArg {
		return typedSelfABIReject("vararg function")
	}
	if proto.NumParams < 1 || proto.NumParams > 4 {
		return typedSelfABIReject("unsupported fixed param count")
	}
	if len(proto.Upvalues) != 0 {
		return typedSelfABIReject("upvalues")
	}
	if len(proto.Protos) != 0 {
		return typedSelfABIReject("nested protos")
	}
	if proto.NumParams > maxTrackedSlots || specializedABIUsesSlotAtOrAbove(proto, maxTrackedSlots) {
		return typedSelfABIReject("too many slots")
	}
	if raw := AnalyzeRawIntSelfABI(proto); raw.Eligible && raw.Return == SpecializedABIReturnRawInt {
		return typedSelfABIReject("covered by raw-int ABI")
	}

	params, reason := inferTypedSelfABIParams(proto)
	if reason != "" {
		return typedSelfABIReject(reason)
	}

	slots := make([]specializedSlotRep, maxTrackedSlots)
	typedSelfResetSlots(slots, params)
	tableFacts := typedSelfInitialTableFacts(params, argFacts)
	branchTargets := specializedABIBranchTargets(proto.Code)
	returnRep := SpecializedABIReturnNone
	sawReturn := false
	sawSelfCall := false
	usesTableABI := false
	for _, p := range params {
		if p == SpecializedABIParamRawTablePtr {
			usesTableABI = true
			break
		}
	}

	for pc, inst := range proto.Code {
		if pc > 0 && branchTargets[pc] {
			typedSelfResetSlots(slots, params)
			tableFacts = typedSelfInitialTableFacts(params, argFacts)
			typedSelfApplyBranchFacts(slots, typedSelfBranchFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts))
			for slot, fact := range typedSelfForLoopBranchTableFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts) {
				if tableFacts == nil {
					tableFacts = make(map[int]FixedShapeTableFact)
				}
				tableFacts[slot] = fact
			}
		}

		op := vm.DecodeOp(inst)
		a := vm.DecodeA(inst)
		b := vm.DecodeB(inst)
		c := vm.DecodeC(inst)

		switch op {
		case vm.OP_LOADINT:
			setSpecializedSlot(slots, a, specializedSlotRawInt)
		case vm.OP_LOADK:
			bx := vm.DecodeBx(inst)
			if specializedABIConstIsInt(proto, bx) {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
			} else if specializedABIConstIsFloat(proto, bx) {
				setSpecializedSlot(slots, a, specializedSlotRawFloat)
			} else if specializedABIConstString(proto, bx) != "" {
				setSpecializedSlot(slots, a, specializedSlotRawString)
			} else {
				setSpecializedSlot(slots, a, specializedSlotUnknown)
			}
		case vm.OP_LOADNIL:
			for slot := a; slot <= a+b && slot < len(slots); slot++ {
				setSpecializedSlot(slots, slot, specializedSlotNil)
			}
		case vm.OP_MOVE:
			setSpecializedSlot(slots, a, getSpecializedSlot(slots, b))
			typedSelfMoveTableFact(tableFacts, a, b)
		case vm.OP_GETGLOBAL:
			if specializedABIConstString(proto, vm.DecodeBx(inst)) == proto.Name {
				setSpecializedSlot(slots, a, specializedSlotSelfFunc)
			} else if rep, ok := typedSelfNumericGlobalRep(proto, vm.DecodeBx(inst), numericGlobals); ok {
				setSpecializedSlot(slots, a, rep)
			} else if fact, ok := typedSelfGlobalArrayElementFact(proto, vm.DecodeBx(inst), globalArrayElementFacts); ok {
				setSpecializedSlot(slots, a, specializedSlotRawTable)
				usesTableABI = true
				if tableFacts == nil {
					tableFacts = make(map[int]FixedShapeTableFact)
				}
				tableFacts[a] = fact
			} else if specializedABIConstString(proto, vm.DecodeBx(inst)) == "math" {
				setSpecializedSlot(slots, a, specializedSlotStdMathTable)
			} else {
				setSpecializedSlot(slots, a, specializedSlotOtherFunc)
			}
		case vm.OP_SETUPVAL, vm.OP_CLOSE, vm.OP_JMP:
		case vm.OP_SETGLOBAL:
			return typedSelfABIReject("global mutation")
		case vm.OP_NEWTABLE, vm.OP_NEWOBJECT2, vm.OP_NEWOBJECTN:
			setSpecializedSlot(slots, a, specializedSlotRawTable)
			delete(tableFacts, a)
			usesTableABI = true
		case vm.OP_GETFIELD:
			if !typedSelfSlotIsTable(getSpecializedSlot(slots, b)) {
				return typedSelfABIReject(fmt.Sprintf("non-table field receiver at pc %d", pc))
			}
			name := typedSelfConstFieldName(proto, c)
			if getSpecializedSlot(slots, b) == specializedSlotStdMathTable && name == "sqrt" {
				setSpecializedSlot(slots, a, specializedSlotMathSqrtFunc)
				delete(tableFacts, a)
			} else if getSpecializedSlot(slots, b) == specializedSlotStdMathTable && name == "floor" {
				setSpecializedSlot(slots, a, specializedSlotMathFloorFunc)
				delete(tableFacts, a)
			} else if fact, ok := tableFacts[b]; ok {
				typ, hasTyp := typedSelfFieldTypeFromFact(fact, name)
				if nested, ok := typedSelfNestedTableFactFromFact(fact, name); ok {
					setSpecializedSlot(slots, a, specializedSlotRawTable)
					if tableFacts == nil {
						tableFacts = make(map[int]FixedShapeTableFact)
					}
					tableFacts[a] = nested
				} else if hasTyp && typ == TypeInt {
					setSpecializedSlot(slots, a, specializedSlotRawInt)
					delete(tableFacts, a)
				} else if hasTyp && typ == TypeFloat {
					setSpecializedSlot(slots, a, specializedSlotRawFloat)
					delete(tableFacts, a)
				} else if hasTyp && typ == TypeString {
					setSpecializedSlot(slots, a, specializedSlotRawString)
					delete(tableFacts, a)
				} else if typedSelfFeedbackResultIsTable(proto, pc) {
					setSpecializedSlot(slots, a, specializedSlotRawTable)
					delete(tableFacts, a)
				} else if typedSelfFeedbackResultIsInt(proto, pc) {
					setSpecializedSlot(slots, a, specializedSlotRawInt)
					delete(tableFacts, a)
				} else if typedSelfFeedbackResultIsFloat(proto, pc) {
					setSpecializedSlot(slots, a, specializedSlotRawFloat)
					delete(tableFacts, a)
				} else {
					setSpecializedSlot(slots, a, specializedSlotUnknown)
					delete(tableFacts, a)
				}
			} else if typ, ok := typedSelfParamFieldTypeWithFacts(proto, b, c, tableFacts); ok && typ == TypeTable {
				setSpecializedSlot(slots, a, specializedSlotRawTable)
				typedSelfSetNestedTableFact(tableFacts, a, tableFacts[b], name)
			} else if ok && typ == TypeInt {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
				delete(tableFacts, a)
			} else if ok && typ == TypeFloat {
				setSpecializedSlot(slots, a, specializedSlotRawFloat)
				delete(tableFacts, a)
			} else if ok && typ == TypeString {
				setSpecializedSlot(slots, a, specializedSlotRawString)
				delete(tableFacts, a)
			} else if typedSelfFeedbackResultIsTable(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawTable)
				delete(tableFacts, a)
			} else if typedSelfFeedbackResultIsInt(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
				delete(tableFacts, a)
			} else if typedSelfFeedbackResultIsFloat(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawFloat)
				delete(tableFacts, a)
			} else {
				setSpecializedSlot(slots, a, specializedSlotUnknown)
				delete(tableFacts, a)
			}
		case vm.OP_GETTABLE:
			if !typedSelfSlotIsTable(getSpecializedSlot(slots, b)) {
				return typedSelfABIReject(fmt.Sprintf("non-table index receiver at pc %d", pc))
			}
			if !typedSelfRKIsInt(slots, proto, c) {
				return typedSelfABIReject(fmt.Sprintf("non-int table index at pc %d", pc))
			}
			if fact, ok := arrayElementArgFacts[b]; ok && typedSelfSlotIsTable(getSpecializedSlot(slots, b)) {
				if fact.ArrayElementType == TypeInt {
					setSpecializedSlot(slots, a, specializedSlotRawInt)
					delete(tableFacts, a)
				} else if fact.ArrayElementType == TypeFloat {
					setSpecializedSlot(slots, a, specializedSlotRawFloat)
					delete(tableFacts, a)
				} else {
					setSpecializedSlot(slots, a, specializedSlotRawTable)
					if tableFacts == nil {
						tableFacts = make(map[int]FixedShapeTableFact)
					}
					tableFacts[a] = fact
				}
			} else if fact, ok := tableFacts[b]; ok {
				if fact.ArrayElementType == TypeInt {
					setSpecializedSlot(slots, a, specializedSlotRawInt)
					delete(tableFacts, a)
				} else if fact.ArrayElementType == TypeFloat {
					setSpecializedSlot(slots, a, specializedSlotRawFloat)
					delete(tableFacts, a)
				} else if fixedShapeTableFactHasUsableTableFact(fact) {
					setSpecializedSlot(slots, a, specializedSlotRawTable)
					if tableFacts == nil {
						tableFacts = make(map[int]FixedShapeTableFact)
					}
					tableFacts[a] = fact
				} else {
					setSpecializedSlot(slots, a, specializedSlotUnknown)
					delete(tableFacts, a)
				}
			} else if typedSelfFeedbackResultIsTable(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawTable)
				delete(tableFacts, a)
			} else if typedSelfFeedbackResultIsInt(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
				delete(tableFacts, a)
			} else if typedSelfFeedbackResultIsFloat(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawFloat)
				delete(tableFacts, a)
			} else {
				setSpecializedSlot(slots, a, specializedSlotUnknown)
				delete(tableFacts, a)
			}
		case vm.OP_SETFIELD:
			if len(slots) <= a || !typedSelfSlotIsTable(getSpecializedSlot(slots, a)) {
				return typedSelfABIReject("non-table field store receiver")
			}
		case vm.OP_SETTABLE:
			if len(slots) <= a || !typedSelfSlotIsTable(getSpecializedSlot(slots, a)) {
				return typedSelfABIReject(fmt.Sprintf("non-table index store receiver at pc %d", pc))
			}
			if !typedSelfRKIsInt(slots, proto, b) {
				return typedSelfABIReject(fmt.Sprintf("non-int table store index at pc %d", pc))
			}
		case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD:
			left, lok := typedSelfRKNumericRep(slots, proto, b)
			right, rok := typedSelfRKNumericRep(slots, proto, c)
			if !lok || !rok {
				return typedSelfABIReject(fmt.Sprintf("non-numeric arithmetic operand at pc %d left=%s right=%s", pc,
					specializedSlotRepName(getSpecializedSlot(slots, b)),
					specializedSlotRepName(getSpecializedSlot(slots, c))))
			}
			if op == vm.OP_DIV ||
				left == specializedSlotRawFloat || left == specializedSlotSelfCallRawFloat ||
				right == specializedSlotRawFloat || right == specializedSlotSelfCallRawFloat {
				setSpecializedSlot(slots, a, specializedSlotRawFloat)
			} else {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
			}
		case vm.OP_UNM:
			rep := getSpecializedSlot(slots, b)
			if !typedSelfSlotIsNumeric(rep) {
				return typedSelfABIReject("non-numeric unary operand")
			}
			setSpecializedSlot(slots, a, typedSelfNumericBaseRep(rep))
		case vm.OP_EQ, vm.OP_LT, vm.OP_LE:
			if !typedSelfCompareOK(slots, proto, b, c) {
				return typedSelfABIReject("unsupported comparison operand")
			}
		case vm.OP_TEST:
		case vm.OP_TESTSET:
			setSpecializedSlot(slots, a, specializedSlotUnknown)
		case vm.OP_LEN:
			if typedSelfSlotIsString(getSpecializedSlot(slots, b)) || typedSelfFeedbackResultIsInt(proto, pc) {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
			} else {
				return typedSelfABIReject("unsupported length result")
			}
		case vm.OP_CALL:
			if typedSelfSlotIsMathUnaryFunc(getSpecializedSlot(slots, a)) {
				if b != 2 || c != 2 {
					return typedSelfABIReject("dynamic intrinsic call arity")
				}
				argRep := getSpecializedSlot(slots, a+1)
				if !typedSelfSlotIsNumeric(argRep) {
					return typedSelfABIReject("non-numeric intrinsic argument")
				}
				if getSpecializedSlot(slots, a) == specializedSlotMathFloorFunc {
					setSpecializedSlot(slots, a, specializedSlotRawInt)
				} else {
					setSpecializedSlot(slots, a, specializedSlotRawFloat)
				}
				delete(tableFacts, a)
				continue
			}
			if getSpecializedSlot(slots, a) != specializedSlotSelfFunc {
				return typedSelfABIReject("non-self call")
			}
			if b == 0 || b-1 != proto.NumParams {
				return typedSelfABIReject("dynamic call arity")
			}
			for i := 0; i < proto.NumParams; i++ {
				argRep := getSpecializedSlot(slots, a+1+i)
				if !typedSelfSlotMatchesParam(argRep, params[i]) {
					return typedSelfABIReject("call argument does not match typed ABI")
				}
			}
			sawSelfCall = true
			switch c {
			case 2:
				switch returnRep {
				case SpecializedABIReturnRawInt:
					setSpecializedSlot(slots, a, specializedSlotSelfCallRawInt)
				case SpecializedABIReturnRawFloat:
					setSpecializedSlot(slots, a, specializedSlotSelfCallRawFloat)
				case SpecializedABIReturnRawTablePtr:
					setSpecializedSlot(slots, a, specializedSlotSelfCallRawTable)
				default:
					setSpecializedSlot(slots, a, specializedSlotUnknown)
				}
			case 1:
				// CALL C=1 has zero results and preserves R(A). Do not
				// fabricate a raw result in the destination slot.
			default:
				return typedSelfABIReject("multiple call returns")
			}
		case vm.OP_RETURN:
			var rep SpecializedABIReturnRep
			switch b {
			case 1:
				rep = SpecializedABIReturnNone
			case 2:
				rep = typedSelfReturnRep(getSpecializedSlot(slots, a), returnRep)
			default:
				return typedSelfABIReject("non-single return")
			}
			if rep == SpecializedABIReturnNone || rep == SpecializedABIReturnBoxed {
				if rep != SpecializedABIReturnNone {
					return typedSelfABIReject("unsupported return representation")
				}
			}
			if sawReturn && returnRep != rep {
				return typedSelfABIReject("inconsistent return representation")
			}
			if rep == SpecializedABIReturnRawTablePtr {
				usesTableABI = true
			}
			returnRep = rep
			sawReturn = true
		case vm.OP_FORPREP:
			if !typedSelfSlotIsInt(getSpecializedSlot(slots, a)) ||
				!typedSelfSlotIsInt(getSpecializedSlot(slots, a+1)) ||
				!typedSelfSlotIsInt(getSpecializedSlot(slots, a+2)) {
				return typedSelfABIReject("non-int for-loop control")
			}
			setSpecializedSlot(slots, a, specializedSlotRawInt)
		case vm.OP_FORLOOP:
			if typedSelfForLoopControlProvenIntWithFactsAndGlobals(proto, params, pc, a, numericGlobals, globalArrayElementFacts) {
				typedSelfApplyStableForLoopFactsWithFactsAndGlobals(proto, params, pc, a, slots, numericGlobals, globalArrayElementFacts)
			}
			if !typedSelfSlotIsInt(getSpecializedSlot(slots, a)) ||
				!typedSelfSlotIsInt(getSpecializedSlot(slots, a+1)) ||
				!typedSelfSlotIsInt(getSpecializedSlot(slots, a+2)) {
				return typedSelfABIReject("non-int for-loop control")
			}
			setSpecializedSlot(slots, a, specializedSlotRawInt)
			setSpecializedSlot(slots, a+3, specializedSlotRawInt)
		case vm.OP_LOADBOOL, vm.OP_GETUPVAL, vm.OP_NOT, vm.OP_CONCAT,
			vm.OP_POW, vm.OP_CLOSURE,
			vm.OP_TFORCALL, vm.OP_TFORLOOP, vm.OP_VARARG, vm.OP_SELF,
			vm.OP_GO, vm.OP_MAKECHAN, vm.OP_SEND, vm.OP_RECV, vm.OP_APPEND, vm.OP_SETLIST:
			return typedSelfABIReject("unsupported opcode")
		default:
			return typedSelfABIReject("unknown opcode")
		}
	}

	if !sawReturn {
		return typedSelfABIReject("no return")
	}
	if requireSelfCall && !sawSelfCall {
		return typedSelfABIReject("no self call")
	}
	if !usesTableABI {
		return typedSelfABIReject("no table parameter or return")
	}
	paramSlots := make([]int, proto.NumParams)
	for i := range paramSlots {
		paramSlots[i] = i
	}
	return TypedSelfABI{
		Eligible:   true,
		NumParams:  proto.NumParams,
		ParamSlots: paramSlots,
		Params:     params,
		Return:     returnRep,
	}
}

func AnalyzeTypedPeerABI(proto *vm.FuncProto) TypedSelfABI {
	abi := analyzeTypedABI(proto, false)
	if !abi.Eligible {
		return abi
	}
	if abi.Return == SpecializedABIReturnNone && typedABIHasStaticSelfCall(proto) {
		return typedSelfABIReject("zero-result self-recursive typed peer ABI is disabled")
	}
	for _, rep := range abi.Params {
		if rep == SpecializedABIParamRawTablePtr {
			return abi
		}
	}
	return typedSelfABIReject("no table parameter")
}

func AnalyzeTypedPeerABIWithArgFacts(proto *vm.FuncProto, argFacts map[int]FixedShapeTableFact) TypedSelfABI {
	abi := analyzeTypedABIWithArgFacts(proto, false, argFacts)
	if !abi.Eligible {
		return abi
	}
	if abi.Return == SpecializedABIReturnNone && typedABIHasStaticSelfCall(proto) {
		return typedSelfABIReject("zero-result self-recursive typed peer ABI is disabled")
	}
	for _, rep := range abi.Params {
		if rep == SpecializedABIParamRawTablePtr {
			return abi
		}
	}
	return typedSelfABIReject("no table parameter")
}

func AnalyzeTypedPeerABIWithFacts(proto *vm.FuncProto, argFacts map[int]FixedShapeTableFact, arrayElementArgFacts map[int]FixedShapeTableFact) TypedSelfABI {
	abi := analyzeTypedABIWithFacts(proto, false, argFacts, arrayElementArgFacts)
	if !abi.Eligible {
		return abi
	}
	if abi.Return == SpecializedABIReturnNone && typedABIHasStaticSelfCall(proto) {
		return typedSelfABIReject("zero-result self-recursive typed peer ABI is disabled")
	}
	for _, rep := range abi.Params {
		if rep == SpecializedABIParamRawTablePtr {
			return abi
		}
	}
	return typedSelfABIReject("no table parameter")
}

func AnalyzeTypedPeerABIWithFactsAndGlobals(proto *vm.FuncProto, argFacts map[int]FixedShapeTableFact, arrayElementArgFacts map[int]FixedShapeTableFact, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) TypedSelfABI {
	abi := analyzeTypedABIWithFactsAndGlobals(proto, false, argFacts, arrayElementArgFacts, numericGlobals, globalArrayElementFacts)
	if !abi.Eligible {
		return abi
	}
	if abi.Return == SpecializedABIReturnNone && typedABIHasStaticSelfCall(proto) {
		return typedSelfABIReject("zero-result self-recursive typed peer ABI is disabled")
	}
	for _, rep := range abi.Params {
		if rep == SpecializedABIParamRawTablePtr {
			return abi
		}
	}
	if len(globalArrayElementFacts) > 0 {
		return abi
	}
	return typedSelfABIReject("no table parameter")
}

func typedABIHasStaticSelfCall(proto *vm.FuncProto) bool {
	if proto == nil || proto.Name == "" {
		return false
	}
	slots := make([]bool, maxTrackedSlots)
	for _, inst := range proto.Code {
		op := vm.DecodeOp(inst)
		a := vm.DecodeA(inst)
		b := vm.DecodeB(inst)
		switch op {
		case vm.OP_GETGLOBAL:
			if a >= 0 && a < len(slots) {
				slots[a] = specializedABIConstString(proto, vm.DecodeBx(inst)) == proto.Name
			}
		case vm.OP_MOVE:
			if a >= 0 && a < len(slots) {
				slots[a] = b >= 0 && b < len(slots) && slots[b]
			}
		case vm.OP_CALL:
			if a >= 0 && a < len(slots) && slots[a] {
				return true
			}
			if a >= 0 && a < len(slots) {
				slots[a] = false
			}
		default:
			if typedABIOpWritesA(op) && a >= 0 && a < len(slots) {
				slots[a] = false
			}
		}
	}
	return false
}

func typedABIOpWritesA(op vm.Opcode) bool {
	switch op {
	case vm.OP_SETUPVAL, vm.OP_SETGLOBAL, vm.OP_SETTABLE, vm.OP_SETFIELD,
		vm.OP_EQ, vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_JMP, vm.OP_RETURN,
		vm.OP_FORLOOP, vm.OP_TFORLOOP, vm.OP_CLOSE:
		return false
	default:
		return true
	}
}

func protoReturnsOnlyNoResults(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	sawReturn := false
	for _, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_RETURN {
			continue
		}
		sawReturn = true
		if vm.DecodeB(inst) != 1 {
			return false
		}
	}
	return sawReturn
}

func protoHasRecursiveTableSurface(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		switch vm.DecodeOp(inst) {
		case vm.OP_GETTABLE, vm.OP_SETTABLE, vm.OP_GETFIELD, vm.OP_SETFIELD,
			vm.OP_NEWTABLE, vm.OP_NEWOBJECT2, vm.OP_NEWOBJECTN, vm.OP_SETLIST, vm.OP_APPEND:
			return true
		}
	}
	return false
}

func typedSelfABIReject(reason string) TypedSelfABI {
	return TypedSelfABI{
		Return:    SpecializedABIReturnBoxed,
		RejectWhy: reason,
	}
}

func specializedABIUsesSlotAtOrAbove(proto *vm.FuncProto, limit int) bool {
	if proto == nil || limit <= 0 {
		return true
	}
	if proto.MaxStack > 0 && proto.MaxStack <= limit {
		return false
	}
	uses := func(slot int) bool {
		return slot >= limit
	}
	usesRange := func(start, count int) bool {
		return count > 0 && start+count-1 >= limit
	}
	for _, inst := range proto.Code {
		op := vm.DecodeOp(inst)
		a := vm.DecodeA(inst)
		b := vm.DecodeB(inst)
		c := vm.DecodeC(inst)
		bReg := b < vm.RKBit
		cReg := c < vm.RKBit
		switch op {
		case vm.OP_LOADNIL:
			if usesRange(a, b+1) {
				return true
			}
		case vm.OP_CALL:
			if uses(a) || (b > 0 && usesRange(a+1, b-1)) || (c > 1 && usesRange(a, c-1)) || (b == 0 || c == 0) {
				return true
			}
		case vm.OP_RETURN:
			if b == 0 || usesRange(a, b-1) {
				return true
			}
		case vm.OP_FORPREP, vm.OP_FORLOOP:
			if usesRange(a, 4) {
				return true
			}
		case vm.OP_MOVE:
			if uses(a) || uses(b) {
				return true
			}
		case vm.OP_GETTABLE:
			if uses(a) || uses(b) || (cReg && uses(c)) {
				return true
			}
		case vm.OP_SETTABLE:
			if uses(a) || (bReg && uses(b)) || (cReg && uses(c)) {
				return true
			}
		case vm.OP_GETFIELD, vm.OP_SELF:
			if uses(a) || uses(b) {
				return true
			}
		case vm.OP_SETFIELD:
			if uses(a) || (cReg && uses(c)) {
				return true
			}
		case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD, vm.OP_POW,
			vm.OP_EQ, vm.OP_LT, vm.OP_LE:
			if uses(a) || (bReg && uses(b)) || (cReg && uses(c)) {
				return true
			}
		case vm.OP_UNM, vm.OP_NOT, vm.OP_LEN, vm.OP_TESTSET:
			if uses(a) || uses(b) {
				return true
			}
		case vm.OP_LOADK, vm.OP_LOADINT, vm.OP_LOADBOOL, vm.OP_GETGLOBAL,
			vm.OP_CLOSURE, vm.OP_GETUPVAL, vm.OP_VARARG, vm.OP_NEWTABLE,
			vm.OP_NEWOBJECT2, vm.OP_NEWOBJECTN, vm.OP_TFORCALL:
			if uses(a) {
				return true
			}
		case vm.OP_SETGLOBAL, vm.OP_SETUPVAL, vm.OP_TEST, vm.OP_SETLIST,
			vm.OP_APPEND, vm.OP_SEND:
			if uses(a) {
				return true
			}
		}
	}
	return false
}

func inferTypedSelfABIParams(proto *vm.FuncProto) ([]SpecializedABIParamRep, string) {
	params := make([]SpecializedABIParamRep, proto.NumParams)
	origins := make([]int, maxTrackedSlots)
	for i := range origins {
		origins[i] = -1
	}
	resetOrigins := func() {
		for i := range origins {
			origins[i] = -1
		}
		for i := 0; i < proto.NumParams && i < len(origins); i++ {
			origins[i] = i
		}
	}
	setParam := func(slot int, rep SpecializedABIParamRep) string {
		if slot < 0 || slot >= len(origins) || origins[slot] < 0 {
			return ""
		}
		idx := origins[slot]
		if params[idx] != SpecializedABIParamBoxed && params[idx] != rep {
			return "conflicting parameter representations"
		}
		params[idx] = rep
		return ""
	}
	setParamFromNumericPeer := func(slot int, peer specializedSlotRep, op vm.Opcode) string {
		if typedSelfSlotIsFloat(peer) {
			return setParam(slot, SpecializedABIParamRawFloat)
		}
		if op == vm.OP_DIV {
			return setParam(slot, SpecializedABIParamRawInt)
		}
		return setParam(slot, SpecializedABIParamRawInt)
	}
	slotReps := make([]specializedSlotRep, maxTrackedSlots)
	resetSlotReps := func() {
		for i := range slotReps {
			slotReps[i] = specializedSlotUnknown
		}
		for i, rep := range params {
			switch rep {
			case SpecializedABIParamRawInt:
				slotReps[i] = specializedSlotRawInt
			case SpecializedABIParamRawFloat:
				slotReps[i] = specializedSlotRawFloat
			case SpecializedABIParamRawTablePtr:
				slotReps[i] = specializedSlotRawTable
			}
		}
	}

	resetOrigins()
	resetSlotReps()
	branchTargets := specializedABIBranchTargets(proto.Code)
	for pc, inst := range proto.Code {
		if pc > 0 && branchTargets[pc] {
			resetOrigins()
			resetSlotReps()
		}
		op := vm.DecodeOp(inst)
		a := vm.DecodeA(inst)
		b := vm.DecodeB(inst)
		c := vm.DecodeC(inst)
		switch op {
		case vm.OP_MOVE:
			if a >= 0 && a < len(origins) {
				origins[a] = -1
				slotReps[a] = specializedSlotUnknown
				if b >= 0 && b < len(origins) {
					origins[a] = origins[b]
					slotReps[a] = slotReps[b]
				}
			}
		case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD:
			leftRep, _ := typedSelfInferRKNumericRep(slotReps, proto, b)
			rightRep, _ := typedSelfInferRKNumericRep(slotReps, proto, c)
			if b < vm.RKBit {
				if reason := setParamFromNumericPeer(b, rightRep, op); reason != "" {
					return nil, reason
				}
			}
			if c < vm.RKBit {
				if reason := setParamFromNumericPeer(c, leftRep, op); reason != "" {
					return nil, reason
				}
			}
			if a >= 0 && a < len(origins) {
				origins[a] = -1
				if op == vm.OP_DIV || typedSelfSlotIsFloat(leftRep) || typedSelfSlotIsFloat(rightRep) {
					slotReps[a] = specializedSlotRawFloat
				} else {
					slotReps[a] = specializedSlotRawInt
				}
			}
		case vm.OP_UNM:
			if reason := setParam(b, SpecializedABIParamRawInt); reason != "" {
				return nil, reason
			}
			if a >= 0 && a < len(origins) {
				origins[a] = -1
				slotReps[a] = specializedSlotRawInt
			}
		case vm.OP_EQ, vm.OP_LT, vm.OP_LE:
			if b < vm.RKBit && typedSelfConstOrOriginSuggestsInt(proto, c) {
				if reason := setParam(b, SpecializedABIParamRawInt); reason != "" {
					return nil, reason
				}
			}
			if c < vm.RKBit && typedSelfConstOrOriginSuggestsInt(proto, b) {
				if reason := setParam(c, SpecializedABIParamRawInt); reason != "" {
					return nil, reason
				}
			}
		case vm.OP_GETFIELD, vm.OP_GETTABLE:
			if reason := setParam(b, SpecializedABIParamRawTablePtr); reason != "" {
				return nil, reason
			}
			if op == vm.OP_GETTABLE && c < vm.RKBit {
				if reason := setParam(c, SpecializedABIParamRawInt); reason != "" {
					return nil, reason
				}
			}
			if a >= 0 && a < len(origins) && (op == vm.OP_GETFIELD || op == vm.OP_GETTABLE) {
				origins[a] = -1
				switch {
				case typedSelfFeedbackResultIsFloat(proto, pc):
					slotReps[a] = specializedSlotRawFloat
				case typedSelfFeedbackResultIsInt(proto, pc):
					slotReps[a] = specializedSlotRawInt
				case typedSelfFeedbackResultIsTable(proto, pc):
					slotReps[a] = specializedSlotRawTable
				default:
					slotReps[a] = specializedSlotUnknown
				}
			}
		case vm.OP_SETFIELD:
			if reason := setParam(a, SpecializedABIParamRawTablePtr); reason != "" {
				return nil, reason
			}
		case vm.OP_SETTABLE:
			if reason := setParam(a, SpecializedABIParamRawTablePtr); reason != "" {
				return nil, reason
			}
			if b < vm.RKBit {
				if reason := setParam(b, SpecializedABIParamRawInt); reason != "" {
					return nil, reason
				}
			}
		case vm.OP_FORPREP, vm.OP_FORLOOP:
			for slot := a; slot <= a+2; slot++ {
				if reason := setParam(slot, SpecializedABIParamRawInt); reason != "" {
					return nil, reason
				}
			}
			if a >= 0 && a < len(origins) {
				origins[a] = -1
			}
			if op == vm.OP_FORLOOP && a+3 >= 0 && a+3 < len(origins) {
				origins[a+3] = -1
			}
		case vm.OP_LOADINT, vm.OP_LOADK, vm.OP_LOADNIL, vm.OP_LOADBOOL,
			vm.OP_GETGLOBAL, vm.OP_GETUPVAL, vm.OP_NEWTABLE, vm.OP_NEWOBJECT2, vm.OP_NEWOBJECTN,
			vm.OP_CALL, vm.OP_TESTSET, vm.OP_NOT, vm.OP_LEN, vm.OP_CONCAT,
			vm.OP_CLOSURE, vm.OP_VARARG, vm.OP_SELF, vm.OP_APPEND:
			if a >= 0 && a < len(origins) {
				origins[a] = -1
				slotReps[a] = specializedSlotUnknown
			}
		}
	}
	for i, rep := range params {
		if rep == SpecializedABIParamBoxed {
			return nil, "untyped parameter"
		}
		params[i] = rep
	}
	return params, ""
}

func typedSelfConstOrOriginSuggestsInt(proto *vm.FuncProto, idx int) bool {
	if idx >= vm.RKBit {
		return specializedABIConstIsInt(proto, idx-vm.RKBit)
	}
	return false
}
