// pass_inline_native_loop.go contains native-effect-loop and pure-numeric
// inline analysis helpers split out of pass_inline.go by pure code movement
// (no behavior change).

package methodjit

import (
	"fmt"
)

func prepareNativeEffectLoopInlineCallee(calleeFn *Function, config InlineConfig) *Function {
	if calleeFn == nil {
		return calleeFn
	}
	out := calleeFn
	out, _ = IntrinsicPass(out)
	if typed, err := TypeSpecializePass(out); err == nil && typed != nil {
		out = typed
	}
	if refreshed, err := SourceFeedbackRefreshPass(out); err == nil && refreshed != nil {
		out = refreshed
	}
	if fixed, err := FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{
		Globals: config.Globals,
	})(out); err == nil && fixed != nil {
		out = fixed
	}
	if lowered, err := TableArrayLowerPass(out); err == nil && lowered != nil {
		out = lowered
	}
	if typedLoads, err := TableArrayLoadTypeSpecializePass(out); err == nil && typedLoads != nil {
		out = typedLoads
	}
	if nested, err := TableArrayNestedLoadPass(out); err == nil && nested != nil {
		out = nested
	}
	if fieldLowered, err := FieldSvalsLowerPass(out); err == nil && fieldLowered != nil {
		out = fieldLowered
	}
	if fixed, err := FixedShapeTableFactsPassWith(FixedShapeTableFactsConfig{
		Globals: config.Globals,
	})(out); err == nil && fixed != nil {
		out = fixed
	}
	if lowered, err := TableArrayLowerPass(out); err == nil && lowered != nil {
		out = lowered
	}
	if typedLoads, err := TableArrayLoadTypeSpecializePass(out); err == nil && typedLoads != nil {
		out = typedLoads
	}
	if stores, err := TableArrayStoreLowerPass(out); err == nil && stores != nil {
		out = stores
	}
	if typed, err := TypeSpecializePass(out); err == nil && typed != nil {
		out = typed
	}
	if loaded, err := LoadEliminationPass(out); err == nil && loaded != nil {
		out = loaded
	}
	if consted, err := ConstPropPass(out); err == nil && consted != nil {
		out = consted
	}
	if dced, err := DCEPass(out); err == nil && dced != nil {
		out = dced
	}
	return out
}

func pureNumericInlineRejectReason(calleeFn *Function) string {
	if calleeFn == nil || calleeFn.Proto == nil {
		return "missing callee IR"
	}
	if calleeFn.Unpromotable {
		return "callee uses unmodeled bytecode"
	}
	if !calleeFn.Proto.MethodJITTier2Callable() {
		return "callee requires vararg frame state"
	}
	if len(calleeFn.Proto.Upvalues) > 0 {
		return "callee captures upvalues"
	}

	returns := 0
	for _, block := range calleeFn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			if instr.Op == OpReturn {
				returns++
				if len(instr.Args) != 1 {
					return "callee does not have exactly one return value"
				}
				if !pureNumericValue(instr.Args[0]) {
					return "return value is not numeric"
				}
				continue
			}
			if !pureNumericInlineOp(instr.Op) {
				return fmt.Sprintf("side-effecting or escaping op %s", instr.Op)
			}
		}
	}
	if returns == 0 {
		return "callee has no return"
	}
	return ""
}

func pureNumericValue(v *Value) bool {
	if v == nil || v.Def == nil {
		return false
	}
	switch v.Def.Type {
	case TypeInt, TypeFloat:
		return true
	case TypeAny, TypeUnknown:
		switch v.Def.Op {
		case OpAdd, OpSub, OpMul, OpDiv, OpMod, OpUnm,
			OpAddInt, OpSubInt, OpMulInt, OpModInt, OpDivIntExact, OpNegInt,
			OpAddFloat, OpSubFloat, OpMulFloat, OpDivFloat, OpNegFloat,
			OpNumToFloat, OpPhi, OpLoadSlot:
			return true
		}
	}
	return false
}

func pureNumericInlineOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.PureNumericInline
}

func nativeEffectLoopInlineRejectReason(calleeFn *Function) string {
	if calleeFn == nil || calleeFn.Proto == nil {
		return "missing callee IR"
	}
	if calleeFn.Unpromotable {
		return "callee uses unmodeled bytecode"
	}
	if !calleeFn.Proto.MethodJITTier2Callable() {
		return "callee requires vararg frame state"
	}
	if len(calleeFn.Proto.Upvalues) > 0 {
		return "callee captures upvalues"
	}
	for _, block := range calleeFn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			if nativeEffectLoopInlineOp(instr.Op) {
				continue
			}
			if instr.Op == OpReturn && len(instr.Args) <= 1 {
				continue
			}
			return fmt.Sprintf("unsupported op %s", instr.Op)
		}
	}
	return ""
}

func calleeHasAllocationIR(calleeFn *Function) bool {
	if calleeFn == nil {
		return false
	}
	for _, block := range calleeFn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpNewTable, OpNewFixedTable, OpSetList:
				return true
			}
		}
	}
	return false
}

func calleeOnlyFixedTableAlloc(calleeFn *Function) bool {
	if calleeFn == nil {
		return false
	}
	for _, block := range calleeFn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpNewFixedTable:
				// ok
			case OpNewTable, OpSetList:
				return false
			}
		}
	}
	return true
}

// calleeIsSimpleConstructor checks that the callee allocates at most one table
// and only sets fields on it (no other side effects). This matches functions
// like `func new_point(x, y) { return {x: x, y: y} }` where BuildGraph
// emits OpNewTable+SetField before FixedTableConstructorLowering runs.
func calleeIsSimpleConstructor(calleeFn *Function) bool {
	if calleeFn == nil {
		return false
	}
	var allocID int
	allocCount := 0
	for _, block := range calleeFn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpNewTable:
				allocCount++
				if allocCount > 1 {
					return false
				}
				allocID = instr.ID
			case OpSetField:
				if len(instr.Args) == 0 || instr.Args[0] == nil || instr.Args[0].ID != allocID {
					return false
				}
			case OpSetList:
				return false
			case OpNewFixedTable:
				// handled by calleeOnlyFixedTableAlloc; not our pattern
				return false
			}
		}
	}
	return allocCount == 1
}

func nativeEffectLoopInlineOp(op Op) bool {
	if pureNumericInlineOp(op) {
		return true
	}
	spec, ok := op.Spec()
	return ok && spec.NativeEffectLoopInline
}
