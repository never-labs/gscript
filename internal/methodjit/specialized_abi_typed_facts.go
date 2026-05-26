//go:build darwin && arm64

// specialized_abi_typed_facts.go: typed-ABI slot/table-fact helpers and slot
// representation classifiers used by the typed-peer ABI analysis in the Method JIT.
// Pure code movement from specialized_abi.go; no behavior change.

package methodjit

import (
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func typedSelfResetSlots(slots []specializedSlotRep, params []SpecializedABIParamRep) {
	for i := range slots {
		slots[i] = specializedSlotUnknown
	}
	for i, rep := range params {
		switch rep {
		case SpecializedABIParamRawInt:
			slots[i] = specializedSlotRawInt
		case SpecializedABIParamRawFloat:
			slots[i] = specializedSlotRawFloat
		case SpecializedABIParamRawTablePtr:
			slots[i] = specializedSlotRawTable
		}
	}
}

func typedSelfInitialTableFacts(params []SpecializedABIParamRep, argFacts map[int]FixedShapeTableFact) map[int]FixedShapeTableFact {
	if len(argFacts) == 0 {
		return nil
	}
	out := make(map[int]FixedShapeTableFact)
	for i, rep := range params {
		if rep != SpecializedABIParamRawTablePtr {
			continue
		}
		if fact, ok := argFacts[i]; ok {
			out[i] = fact
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func typedSelfMoveTableFact(tableFacts map[int]FixedShapeTableFact, dst, src int) {
	if tableFacts == nil {
		return
	}
	if fact, ok := tableFacts[src]; ok {
		tableFacts[dst] = fact
	} else {
		delete(tableFacts, dst)
	}
}

func typedSelfSetNestedTableFact(tableFacts map[int]FixedShapeTableFact, dst int, receiver FixedShapeTableFact, name string) {
	if tableFacts == nil || name == "" {
		return
	}
	if nested, ok := typedSelfNestedTableFactFromFact(receiver, name); ok {
		tableFacts[dst] = nested
		return
	}
	delete(tableFacts, dst)
}

func typedSelfNestedTableFactFromFact(fact FixedShapeTableFact, name string) (FixedShapeTableFact, bool) {
	if name == "" || len(fact.FieldTableFacts) == 0 {
		return FixedShapeTableFact{}, false
	}
	nested, ok := fact.FieldTableFacts[name]
	if !ok || !fixedShapeTableFactHasUsableTableFact(nested) {
		return FixedShapeTableFact{}, false
	}
	return cloneFixedShapeTableFact(nested), true
}

func typedSelfFieldTypeFromFact(fact FixedShapeTableFact, name string) (Type, bool) {
	if name == "" {
		return TypeUnknown, false
	}
	if nested, ok := typedSelfNestedTableFactFromFact(fact, name); ok && fixedShapeTableFactHasUsableTableFact(nested) {
		return TypeTable, true
	}
	if fact.FieldTypes != nil {
		if typ, ok := fact.FieldTypes[name]; ok && typ != TypeUnknown && typ != TypeAny {
			return typ, true
		}
	}
	if fact.FieldRanges != nil {
		if _, ok := fact.FieldRanges[name]; ok {
			return TypeInt, true
		}
	}
	if fact.FieldLenRanges != nil {
		if _, ok := fact.FieldLenRanges[name]; ok {
			return TypeString, true
		}
	}
	if fact.ShapeID != 0 {
		if idx, ok := fact.fieldIndex(name); ok {
			if vt, stable := runtime.ShapeFieldStableType(fact.ShapeID, idx); stable {
				if typ, ok := runtimeValueTypeToIRType(vt); ok {
					return typ, true
				}
			}
		}
	}
	return TypeUnknown, false
}

func runtimeValueTypeToIRType(vt runtime.ValueType) (Type, bool) {
	switch vt {
	case runtime.TypeInt:
		return TypeInt, true
	case runtime.TypeFloat:
		return TypeFloat, true
	case runtime.TypeBool:
		return TypeBool, true
	case runtime.TypeString:
		return TypeString, true
	case runtime.TypeTable:
		return TypeTable, true
	default:
		return TypeUnknown, false
	}
}

func typedSelfConstFieldName(proto *vm.FuncProto, constIdx int) string {
	if proto == nil || constIdx < 0 || constIdx >= len(proto.Constants) {
		return ""
	}
	key := proto.Constants[constIdx]
	if !key.IsString() {
		return ""
	}
	return key.Str()
}

func typedSelfRKIsInt(slots []specializedSlotRep, proto *vm.FuncProto, idx int) bool {
	if idx >= vm.RKBit {
		return specializedABIConstIsInt(proto, idx-vm.RKBit)
	}
	return typedSelfSlotIsInt(getSpecializedSlot(slots, idx))
}

func typedSelfRKNumericRep(slots []specializedSlotRep, proto *vm.FuncProto, idx int) (specializedSlotRep, bool) {
	if idx >= vm.RKBit {
		k := idx - vm.RKBit
		if specializedABIConstIsInt(proto, k) {
			return specializedSlotRawInt, true
		}
		if specializedABIConstIsFloat(proto, k) {
			return specializedSlotRawFloat, true
		}
		return specializedSlotUnknown, false
	}
	rep := getSpecializedSlot(slots, idx)
	if typedSelfSlotIsNumeric(rep) {
		return rep, true
	}
	return specializedSlotUnknown, false
}

func typedSelfRKIsNil(slots []specializedSlotRep, proto *vm.FuncProto, idx int) bool {
	if idx >= vm.RKBit {
		k := idx - vm.RKBit
		return k >= 0 && k < len(proto.Constants) && proto.Constants[k].IsNil()
	}
	return getSpecializedSlot(slots, idx) == specializedSlotNil
}

func typedSelfCompareOK(slots []specializedSlotRep, proto *vm.FuncProto, b, c int) bool {
	if typedSelfRKIsInt(slots, proto, b) && typedSelfRKIsInt(slots, proto, c) {
		return true
	}
	if typedSelfRKIsNil(slots, proto, b) || typedSelfRKIsNil(slots, proto, c) {
		return true
	}
	// Comparisons do not create ABI-carried values. Unknown table contents may
	// still be compared by the normal boxed/generic compare path as long as
	// they are not later treated as raw int/table arguments.
	return true
}

func typedSelfFeedbackResultIsTable(proto *vm.FuncProto, pc int) bool {
	if proto == nil || pc < 0 {
		return false
	}
	if proto.Feedback != nil && pc < len(proto.Feedback) && proto.Feedback[pc].Result == vm.FBTable {
		return true
	}
	if proto.FieldAccessFeedback != nil && pc < len(proto.FieldAccessFeedback) && proto.FieldAccessFeedback[pc].ValueType == vm.FBTable {
		return true
	}
	return proto.TableKeyFeedback != nil && pc < len(proto.TableKeyFeedback) && proto.TableKeyFeedback[pc].ValueType == vm.FBTable
}

func typedSelfFeedbackResultIsInt(proto *vm.FuncProto, pc int) bool {
	if proto == nil || pc < 0 {
		return false
	}
	if proto.Feedback != nil && pc < len(proto.Feedback) && proto.Feedback[pc].Result == vm.FBInt {
		return true
	}
	if proto.FieldAccessFeedback != nil && pc < len(proto.FieldAccessFeedback) && proto.FieldAccessFeedback[pc].ValueType == vm.FBInt {
		return true
	}
	return proto.TableKeyFeedback != nil && pc < len(proto.TableKeyFeedback) && proto.TableKeyFeedback[pc].ValueType == vm.FBInt
}

func typedSelfFeedbackResultIsFloat(proto *vm.FuncProto, pc int) bool {
	if proto == nil || pc < 0 {
		return false
	}
	if proto.Feedback != nil && pc < len(proto.Feedback) && proto.Feedback[pc].Result == vm.FBFloat {
		return true
	}
	if proto.FieldAccessFeedback != nil && pc < len(proto.FieldAccessFeedback) && proto.FieldAccessFeedback[pc].ValueType == vm.FBFloat {
		return true
	}
	return proto.TableKeyFeedback != nil && pc < len(proto.TableKeyFeedback) && proto.TableKeyFeedback[pc].ValueType == vm.FBFloat
}

func typedSelfParamFieldType(proto *vm.FuncProto, paramSlot, constIdx int) (Type, bool) {
	return typedSelfParamFieldTypeWithFacts(proto, paramSlot, constIdx, nil)
}

func typedSelfParamFieldTypeWithFacts(proto *vm.FuncProto, paramSlot, constIdx int, argFacts map[int]FixedShapeTableFact) (Type, bool) {
	if proto == nil || paramSlot < 0 || paramSlot >= maxTrackedSlots ||
		constIdx < 0 || constIdx >= len(proto.Constants) ||
		(len(proto.ArgShapeFeedback) <= paramSlot && len(proto.ArgArrayElementShapeFeedback) <= paramSlot && len(argFacts) == 0) {
		return TypeUnknown, false
	}
	key := proto.Constants[constIdx]
	if !key.IsString() {
		return TypeUnknown, false
	}
	if fact, ok := argFacts[paramSlot]; ok {
		if typ, ok := typedSelfFieldTypeFromFact(fact, key.Str()); ok {
			return typ, true
		}
	}
	if paramSlot >= proto.NumParams {
		return TypeUnknown, false
	}
	if len(proto.ArgShapeFeedback) > paramSlot {
		feedback := proto.ArgShapeFeedback[paramSlot]
		if len(feedback.FieldTypes) > 0 {
			if typ, ok := feedbackToIRType(feedback.FieldTypes[key.Str()]); ok {
				return typ, true
			}
		}
		if len(feedback.FieldRanges) > 0 {
			if _, _, ok := feedback.FieldRanges[key.Str()].StableRange(); ok {
				return TypeInt, true
			}
		}
		if len(feedback.FieldLenRanges) > 0 {
			if _, _, ok := feedback.FieldLenRanges[key.Str()].StableRange(); ok {
				return TypeString, true
			}
		}
	}
	if len(proto.ArgArrayElementShapeFeedback) <= paramSlot {
		return TypeUnknown, false
	}
	feedback := proto.ArgArrayElementShapeFeedback[paramSlot]
	if len(feedback.FieldTypes) == 0 {
		return TypeUnknown, false
	}
	if len(feedback.FieldRanges) > 0 {
		if _, _, ok := feedback.FieldRanges[key.Str()].StableRange(); ok {
			return TypeInt, true
		}
	}
	if len(feedback.FieldLenRanges) > 0 {
		if _, _, ok := feedback.FieldLenRanges[key.Str()].StableRange(); ok {
			return TypeString, true
		}
	}
	typ, ok := feedbackToIRType(feedback.FieldTypes[key.Str()])
	return typ, ok
}

func typedSelfSlotIsInt(rep specializedSlotRep) bool {
	return rep == specializedSlotRawInt || rep == specializedSlotSelfCallRawInt
}

func specializedSlotRepName(rep specializedSlotRep) string {
	switch rep {
	case specializedSlotUnknown:
		return "unknown"
	case specializedSlotRawInt:
		return "raw-int"
	case specializedSlotRawFloat:
		return "raw-float"
	case specializedSlotRawTable:
		return "raw-table"
	case specializedSlotRawString:
		return "raw-string"
	case specializedSlotNil:
		return "nil"
	case specializedSlotSelfCallRawInt:
		return "self-call-raw-int"
	case specializedSlotSelfCallRawFloat:
		return "self-call-raw-float"
	case specializedSlotSelfCallRawTable:
		return "self-call-raw-table"
	case specializedSlotSelfFunc:
		return "self-func"
	case specializedSlotOtherFunc:
		return "other-func"
	case specializedSlotStdMathTable:
		return "std-math-table"
	case specializedSlotMathSqrtFunc:
		return "math.sqrt"
	case specializedSlotMathFloorFunc:
		return "math.floor"
	default:
		return "invalid"
	}
}

func typedSelfSlotIsFloat(rep specializedSlotRep) bool {
	return rep == specializedSlotRawFloat || rep == specializedSlotSelfCallRawFloat
}

func typedSelfSlotIsNumeric(rep specializedSlotRep) bool {
	return typedSelfSlotIsInt(rep) || typedSelfSlotIsFloat(rep)
}

func typedSelfNumericBaseRep(rep specializedSlotRep) specializedSlotRep {
	if typedSelfSlotIsFloat(rep) {
		return specializedSlotRawFloat
	}
	if typedSelfSlotIsInt(rep) {
		return specializedSlotRawInt
	}
	return specializedSlotUnknown
}

func typedSelfSlotIsTable(rep specializedSlotRep) bool {
	return rep == specializedSlotRawTable || rep == specializedSlotSelfCallRawTable || rep == specializedSlotStdMathTable
}

func typedSelfSlotIsMathUnaryFunc(rep specializedSlotRep) bool {
	return rep == specializedSlotMathSqrtFunc || rep == specializedSlotMathFloorFunc
}

func typedSelfSlotIsString(rep specializedSlotRep) bool {
	return rep == specializedSlotRawString
}

func typedSelfSlotMatchesParam(rep specializedSlotRep, param SpecializedABIParamRep) bool {
	switch param {
	case SpecializedABIParamRawInt:
		return typedSelfSlotIsInt(rep)
	case SpecializedABIParamRawFloat:
		return typedSelfSlotIsFloat(rep)
	case SpecializedABIParamRawTablePtr:
		return typedSelfSlotIsTable(rep)
	default:
		return false
	}
}

func typedSelfInferRKNumericRep(slots []specializedSlotRep, proto *vm.FuncProto, idx int) (specializedSlotRep, bool) {
	if idx >= vm.RKBit {
		k := idx - vm.RKBit
		if specializedABIConstIsFloat(proto, k) {
			return specializedSlotRawFloat, true
		}
		if specializedABIConstIsInt(proto, k) {
			return specializedSlotRawInt, true
		}
		return specializedSlotUnknown, false
	}
	if idx >= 0 && idx < len(slots) && typedSelfSlotIsNumeric(slots[idx]) {
		return slots[idx], true
	}
	return specializedSlotUnknown, false
}

func typedSelfReturnRep(slot specializedSlotRep, current SpecializedABIReturnRep) SpecializedABIReturnRep {
	switch slot {
	case specializedSlotRawInt:
		return SpecializedABIReturnRawInt
	case specializedSlotRawFloat:
		return SpecializedABIReturnRawFloat
	case specializedSlotRawTable:
		return SpecializedABIReturnRawTablePtr
	case specializedSlotSelfCallRawInt:
		return SpecializedABIReturnRawInt
	case specializedSlotSelfCallRawFloat:
		return SpecializedABIReturnRawFloat
	case specializedSlotSelfCallRawTable:
		return SpecializedABIReturnRawTablePtr
	case specializedSlotUnknown:
		if current != SpecializedABIReturnNone {
			return current
		}
		return SpecializedABIReturnBoxed
	default:
		return SpecializedABIReturnBoxed
	}
}
