// pass_inline_feedback.go contains the feedback-driven callee resolution
// helpers split out of pass_inline.go by pure code movement (no behavior change).
//
// These resolve dynamic call sites to a concrete callee proto via call-site
// feedback and apply argument/upvalue type facts to the cloned callee IR.

package methodjit

import (
	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
	"unsafe"
)

func recordTier2SpecDependency(fn *Function, callee *vm.FuncProto) {
	if fn == nil || fn.Analysis == nil || callee == nil || callee == fn.Proto {
		return
	}
	functionSpeculationFacts(fn).RecordSpecDependencyProto(fn.Proto, callee)
}

func inlineFeedbackCalleeProto(fn *Function, instr *Instr) (*vm.FuncProto, bool) {
	proto, _, ok := inlineFeedbackCallee(fn, instr)
	return proto, ok
}

func inlineFeedbackCallee(fn *Function, instr *Instr) (*vm.FuncProto, uintptr, bool) {
	if proto, ok := callABIFeedbackCalleeProto(fn, instr); ok && proto != nil {
		if closure, closureProto, closureOK := inlineFeedbackVMClosure(fn, instr); closureOK && closureProto == proto {
			return proto, closure, true
		}
		return proto, 0, true
	}
	if c, ok := inlineFeedbackFieldShapeCase(fn, instr); ok && c.VMProto != nil {
		return c.VMProto, 0, true
	}
	return nil, 0, false
}

func inlineFeedbackVMClosure(fn *Function, instr *Instr) (uintptr, *vm.FuncProto, bool) {
	if fn == nil || fn.Proto == nil || instr == nil || !instr.HasSource ||
		instr.SourcePC < 0 || instr.SourcePC >= len(fn.Proto.CallSiteFeedback) {
		return 0, nil, false
	}
	return fn.Proto.CallSiteFeedback[instr.SourcePC].StableCalleeVMClosure()
}

func inlineFeedbackFieldShapeCase(fn *Function, instr *Instr) (FieldPolyShapeCase, bool) {
	cases := fieldShapeCalleeCases(fn, instr)
	if len(cases) != 1 || cases[0].VMProto == nil {
		return FieldPolyShapeCase{}, false
	}
	return cases[0], true
}

func applyInlineArgTypeFacts(calleeFn *Function, callArgs []*Value) *Function {
	if calleeFn == nil || len(callArgs) == 0 {
		return calleeFn
	}
	for _, block := range calleeFn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpLoadSlot {
				continue
			}
			slot := int(instr.Aux)
			if slot < 0 || slot >= len(callArgs) {
				continue
			}
			if typ, ok := inlineArgValueType(callArgs[slot]); ok &&
				(instr.Type == TypeAny || instr.Type == TypeUnknown) {
				instr.Type = typ
			}
		}
	}
	return calleeFn
}

func applyInlineClosureUpvalueFacts(calleeFn *Function, closurePtr uintptr) *Function {
	if calleeFn == nil || closurePtr == 0 {
		return calleeFn
	}
	cl := (*vm.Closure)(unsafe.Pointer(closurePtr))
	if cl == nil {
		return calleeFn
	}
	for _, block := range calleeFn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpGetUpval {
				continue
			}
			idx := int(instr.Aux)
			if idx < 0 || idx >= len(cl.Upvalues) || cl.Upvalues[idx] == nil {
				continue
			}
			if typ, ok := inlineRuntimeValueType(cl.Upvalues[idx].Get()); ok {
				instr.Type = typ
			}
		}
	}
	return calleeFn
}

func inlineRuntimeValueType(v runtime.Value) (Type, bool) {
	switch {
	case v.IsInt():
		return TypeInt, true
	case v.IsFloat():
		return TypeFloat, true
	case v.IsBool():
		return TypeBool, true
	case v.IsString():
		return TypeString, true
	case v.IsTable():
		return TypeTable, true
	case v.IsFunction():
		return TypeFunction, true
	case v.IsNil():
		return TypeNil, true
	default:
		return TypeUnknown, false
	}
}

func inlineArgValueType(v *Value) (Type, bool) {
	if v == nil || v.Def == nil {
		return TypeUnknown, false
	}
	if v.Def.Op == OpGuardType {
		return Type(v.Def.Aux), true
	}
	switch v.Def.Type {
	case TypeInt, TypeFloat, TypeString, TypeTable, TypeBool:
		return v.Def.Type, true
	default:
		return TypeUnknown, false
	}
}

func hasInlineFeedbackCallee(fn *Function) bool {
	if fn == nil {
		return false
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpCall {
				continue
			}
			if _, ok := inlineFeedbackCalleeProto(fn, instr); ok {
				return true
			}
		}
	}
	return false
}

func inlineCalleeHasRuntimeSpecializationEntry(callee *vm.FuncProto, globals map[string]*vm.FuncProto) bool {
	if callee == nil || len(globals) == 0 {
		return false
	}
	if _, ok := analyzeMutualRecursiveIntSCC(callee, globals); ok {
		return true
	}
	return false
}
