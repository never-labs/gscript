//go:build darwin && arm64

// specialized_abi_typed_flow.go: typed-ABI control-flow fact propagation for the
// Method JIT — branch/for-loop fact derivation and per-PC slot/table-fact
// reconstruction used by the typed-peer ABI analysis.
// Pure code movement from specialized_abi.go; no behavior change.

package methodjit

import (
	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

func typedSelfBranchFacts(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int) map[int]specializedSlotRep {
	return typedSelfBranchFactsWithGlobals(proto, params, pc, nil)
}

func typedSelfBranchFactsWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int, numericGlobals map[string]runtime.Value) map[int]specializedSlotRep {
	return typedSelfBranchFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, nil)
}

func typedSelfBranchFactsWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) map[int]specializedSlotRep {
	if proto == nil || pc < 0 {
		return nil
	}
	if typedSelfHasFallthroughPred(proto, pc) {
		return nil
	}
	var facts map[int]specializedSlotRep
	havePred := false
	mergePredFacts := func(pred map[int]specializedSlotRep) {
		if !havePred {
			havePred = true
			if len(pred) == 0 {
				return
			}
			facts = make(map[int]specializedSlotRep, len(pred))
			for slot, rep := range pred {
				facts[slot] = rep
			}
			return
		}
		for slot, rep := range facts {
			if predRep, ok := pred[slot]; !ok || predRep != rep {
				delete(facts, slot)
			}
		}
	}
	for srcPC, inst := range proto.Code {
		op := vm.DecodeOp(inst)
		switch op {
		case vm.OP_JMP:
			target := srcPC + 1 + vm.DecodesBx(inst)
			if target == pc {
				if slots, ok := typedSelfLoopFactSlotsAtPCWithFactsAndGlobals(proto, params, srcPC, numericGlobals, globalArrayElementFacts); ok {
					mergePredFacts(typedSelfSlotFacts(slots))
				}
			}
			continue
		case vm.OP_EQ, vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_TESTSET:
			target := srcPC + 2
			if target == pc {
				if slots, ok := typedSelfLoopFactSlotsAtPCWithFactsAndGlobals(proto, params, srcPC, numericGlobals, globalArrayElementFacts); ok {
					mergePredFacts(typedSelfSlotFacts(slots))
				}
			}
			continue
		case vm.OP_FORLOOP:
		default:
			continue
		}
		bodyTarget := srcPC + 1 + vm.DecodesBx(inst)
		exitTarget := srcPC + 1
		if bodyTarget != pc && exitTarget != pc {
			continue
		}
		a := vm.DecodeA(inst)
		if typedSelfForLoopControlProvenIntWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts) {
			pred := make(map[int]specializedSlotRep)
			addFact := func(slot int, rep specializedSlotRep) {
				if slot >= 0 && slot < maxTrackedSlots {
					pred[slot] = rep
				}
			}
			bodyTarget := srcPC + 1 + vm.DecodesBx(inst)
			if preSlots, ok := typedSelfForLoopPreSlotsWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts); ok {
				for slot, rep := range preSlots {
					if typedSelfLoopBodyWritesSlot(proto, bodyTarget, srcPC, slot) {
						continue
					}
					switch rep {
					case specializedSlotRawInt, specializedSlotRawTable, specializedSlotNil,
						specializedSlotSelfFunc, specializedSlotOtherFunc, specializedSlotStdMathTable,
						specializedSlotMathSqrtFunc, specializedSlotMathFloorFunc:
						addFact(slot, rep)
					}
				}
			}
			preSlots, postSlots, ok := typedSelfForLoopStableSlotsWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts)
			if ok {
				for slot, pre := range preSlots {
					if pre != postSlots[slot] {
						continue
					}
					switch pre {
					case specializedSlotRawInt, specializedSlotRawTable, specializedSlotNil,
						specializedSlotSelfFunc, specializedSlotOtherFunc, specializedSlotStdMathTable,
						specializedSlotMathSqrtFunc, specializedSlotMathFloorFunc:
						addFact(slot, pre)
					}
				}
			}
			addFact(a, specializedSlotRawInt)
			if bodyTarget == pc {
				addFact(a+3, specializedSlotRawInt)
			}
			mergePredFacts(pred)
		}
	}
	if !havePred {
		return nil
	}
	return facts
}

func typedSelfCollectSlotFacts(slots []specializedSlotRep, addFact func(int, specializedSlotRep)) {
	for slot, rep := range slots {
		switch rep {
		case specializedSlotRawInt, specializedSlotRawTable, specializedSlotNil,
			specializedSlotSelfFunc, specializedSlotOtherFunc, specializedSlotStdMathTable,
			specializedSlotMathSqrtFunc, specializedSlotMathFloorFunc:
			addFact(slot, rep)
		}
	}
}

func typedSelfSlotFacts(slots []specializedSlotRep) map[int]specializedSlotRep {
	facts := make(map[int]specializedSlotRep)
	typedSelfCollectSlotFacts(slots, func(slot int, rep specializedSlotRep) {
		facts[slot] = rep
	})
	return facts
}

func typedSelfHasFallthroughPred(proto *vm.FuncProto, pc int) bool {
	if proto == nil || pc <= 0 || pc > len(proto.Code) {
		return false
	}
	switch vm.DecodeOp(proto.Code[pc-1]) {
	case vm.OP_JMP, vm.OP_RETURN, vm.OP_FORPREP, vm.OP_FORLOOP:
		return false
	default:
		return true
	}
}

func typedSelfApplyBranchFacts(slots []specializedSlotRep, facts map[int]specializedSlotRep) {
	for slot, rep := range facts {
		setSpecializedSlot(slots, slot, rep)
	}
}

func typedSelfCallArgSlotMatches(proto *vm.FuncProto, callPC, argIndex int, param SpecializedABIParamRep) bool {
	if proto == nil || callPC < 0 || callPC >= len(proto.Code) {
		return false
	}
	inst := proto.Code[callPC]
	if vm.DecodeOp(inst) != vm.OP_CALL {
		return false
	}
	params, reason := inferTypedSelfABIParams(proto)
	if reason != "" || argIndex < 0 || argIndex >= len(params) {
		return false
	}
	callSlot := vm.DecodeA(inst)
	argSlot := callSlot + 1 + argIndex
	rep, ok := typedSelfSlotRepAtPC(proto, params, callPC, argSlot)
	return ok && typedSelfSlotMatchesParam(rep, param)
}

func typedSelfSlotRepAtPC(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC, slot int) (specializedSlotRep, bool) {
	slots, ok := typedSelfSlotsAtPC(proto, params, targetPC)
	if !ok || slot < 0 || slot >= len(slots) {
		return specializedSlotUnknown, false
	}
	return getSpecializedSlot(slots, slot), true
}

func typedSelfSlotsAtPC(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int) ([]specializedSlotRep, bool) {
	return typedSelfSlotsAtPCWithGlobals(proto, params, targetPC, nil)
}

func typedSelfSlotsAtPCWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int, numericGlobals map[string]runtime.Value) ([]specializedSlotRep, bool) {
	return typedSelfSlotsAtPCWithFactsAndGlobals(proto, params, targetPC, numericGlobals, nil)
}

func typedSelfSlotsAtPCWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) ([]specializedSlotRep, bool) {
	if proto == nil || targetPC < 0 || targetPC > len(proto.Code) {
		return nil, false
	}
	slots := make([]specializedSlotRep, maxTrackedSlots)
	typedSelfResetSlots(slots, params)
	branchTargets := specializedABIBranchTargets(proto.Code)
	for pc := 0; pc < targetPC; pc++ {
		if pc > 0 && branchTargets[pc] {
			typedSelfResetSlots(slots, params)
			typedSelfApplyBranchFacts(slots, typedSelfBranchFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts))
		}
		if !typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto, slots, pc, numericGlobals, globalArrayElementFacts) {
			return nil, false
		}
	}
	return slots, true
}

func typedSelfLoopFactSlotsAtPC(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int) ([]specializedSlotRep, bool) {
	return typedSelfLoopFactSlotsAtPCWithGlobals(proto, params, targetPC, nil)
}

func typedSelfLoopFactSlotsAtPCWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int, numericGlobals map[string]runtime.Value) ([]specializedSlotRep, bool) {
	return typedSelfLoopFactSlotsAtPCWithFactsAndGlobals(proto, params, targetPC, numericGlobals, nil)
}

func typedSelfLoopFactSlotsAtPCWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) ([]specializedSlotRep, bool) {
	if proto == nil || targetPC < 0 || targetPC > len(proto.Code) {
		return nil, false
	}
	slots := make([]specializedSlotRep, maxTrackedSlots)
	typedSelfResetSlots(slots, params)
	branchTargets := specializedABIBranchTargets(proto.Code)
	for pc := 0; pc < targetPC; pc++ {
		if pc > 0 && branchTargets[pc] {
			typedSelfResetSlots(slots, params)
			typedSelfApplyBranchFacts(slots, typedSelfForLoopBranchFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts))
		}
		if !typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto, slots, pc, numericGlobals, globalArrayElementFacts) {
			return nil, false
		}
	}
	return slots, true
}

func typedSelfForLoopBranchFacts(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int) map[int]specializedSlotRep {
	return typedSelfForLoopBranchFactsWithGlobals(proto, params, pc, nil)
}

func typedSelfForLoopBranchFactsWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int, numericGlobals map[string]runtime.Value) map[int]specializedSlotRep {
	return typedSelfForLoopBranchFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, nil)
}

func typedSelfForLoopBranchFactsWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) map[int]specializedSlotRep {
	if proto == nil || pc < 0 {
		return nil
	}
	var facts map[int]specializedSlotRep
	addFact := func(slot int, rep specializedSlotRep) {
		if slot < 0 || slot >= maxTrackedSlots {
			return
		}
		if facts == nil {
			facts = make(map[int]specializedSlotRep)
		}
		facts[slot] = rep
	}
	for srcPC, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_FORLOOP {
			continue
		}
		bodyTarget := srcPC + 1 + vm.DecodesBx(inst)
		exitTarget := srcPC + 1
		if bodyTarget != pc && exitTarget != pc {
			continue
		}
		a := vm.DecodeA(inst)
		if !typedSelfForLoopControlProvenIntWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts) {
			continue
		}
		if preSlots, ok := typedSelfForLoopPreSlotsWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts); ok {
			for slot, rep := range preSlots {
				if typedSelfLoopBodyWritesSlot(proto, bodyTarget, srcPC, slot) {
					continue
				}
				switch rep {
				case specializedSlotRawInt, specializedSlotRawTable, specializedSlotNil,
					specializedSlotSelfFunc, specializedSlotOtherFunc, specializedSlotStdMathTable,
					specializedSlotMathSqrtFunc, specializedSlotMathFloorFunc:
					addFact(slot, rep)
				}
			}
		}
		preSlots, postSlots, ok := typedSelfForLoopStableSlotsWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts)
		if ok {
			for slot, pre := range preSlots {
				if pre == postSlots[slot] {
					switch pre {
					case specializedSlotRawInt, specializedSlotRawTable, specializedSlotNil,
						specializedSlotSelfFunc, specializedSlotOtherFunc, specializedSlotStdMathTable,
						specializedSlotMathSqrtFunc, specializedSlotMathFloorFunc:
						addFact(slot, pre)
					}
				}
			}
		}
		addFact(a, specializedSlotRawInt)
		if bodyTarget == pc {
			addFact(a+3, specializedSlotRawInt)
		}
	}
	return facts
}

func typedSelfForLoopBranchTableFactsWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, pc int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) map[int]FixedShapeTableFact {
	if proto == nil || pc < 0 {
		return nil
	}
	var facts map[int]FixedShapeTableFact
	addFact := func(slot int, fact FixedShapeTableFact) {
		if slot < 0 || slot >= maxTrackedSlots || !fixedShapeTableFactHasUsableTableFact(fact) {
			return
		}
		if facts == nil {
			facts = make(map[int]FixedShapeTableFact)
		}
		facts[slot] = cloneFixedShapeTableFact(fact)
	}
	for srcPC, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_FORLOOP {
			continue
		}
		bodyTarget := srcPC + 1 + vm.DecodesBx(inst)
		exitTarget := srcPC + 1
		if bodyTarget != pc && exitTarget != pc {
			continue
		}
		a := vm.DecodeA(inst)
		if !typedSelfForLoopControlProvenIntWithFactsAndGlobals(proto, params, srcPC, a, numericGlobals, globalArrayElementFacts) {
			continue
		}
		prepPC := typedSelfFindForPrep(proto, srcPC, a)
		if prepPC < 0 {
			continue
		}
		preFacts, ok := typedSelfTableFactsAtPCWithFactsAndGlobals(proto, params, prepPC, numericGlobals, globalArrayElementFacts)
		if !ok {
			continue
		}
		for slot, fact := range preFacts {
			if typedSelfLoopBodyWritesSlot(proto, bodyTarget, srcPC, slot) {
				continue
			}
			addFact(slot, fact)
		}
	}
	return facts
}

func typedSelfTableFactsAtPCWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, targetPC int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) (map[int]FixedShapeTableFact, bool) {
	if proto == nil || targetPC < 0 || targetPC > len(proto.Code) {
		return nil, false
	}
	slots := make([]specializedSlotRep, maxTrackedSlots)
	typedSelfResetSlots(slots, params)
	tableFacts := make(map[int]FixedShapeTableFact)
	branchTargets := specializedABIBranchTargets(proto.Code)
	for pc := 0; pc < targetPC; pc++ {
		if pc > 0 && branchTargets[pc] {
			typedSelfResetSlots(slots, params)
			tableFacts = make(map[int]FixedShapeTableFact)
			typedSelfApplyBranchFacts(slots, typedSelfBranchFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts))
			for slot, fact := range typedSelfForLoopBranchTableFactsWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts) {
				tableFacts[slot] = fact
			}
		}
		if !typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto, slots, pc, numericGlobals, globalArrayElementFacts) {
			return nil, false
		}
		typedSelfAdvanceSimpleTableFacts(proto, slots, tableFacts, pc, globalArrayElementFacts)
	}
	return tableFacts, true
}

func typedSelfAdvanceSimpleTableFacts(proto *vm.FuncProto, slots []specializedSlotRep, tableFacts map[int]FixedShapeTableFact, pc int, globalArrayElementFacts map[string]FixedShapeTableFact) {
	if proto == nil || pc < 0 || pc >= len(proto.Code) || tableFacts == nil {
		return
	}
	inst := proto.Code[pc]
	op := vm.DecodeOp(inst)
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	c := vm.DecodeC(inst)
	kill := func(slot int) {
		if slot >= 0 && slot < maxTrackedSlots {
			delete(tableFacts, slot)
		}
	}
	switch op {
	case vm.OP_MOVE:
		if fact, ok := tableFacts[b]; ok {
			tableFacts[a] = cloneFixedShapeTableFact(fact)
		} else {
			kill(a)
		}
	case vm.OP_GETGLOBAL:
		if fact, ok := typedSelfGlobalArrayElementFact(proto, vm.DecodeBx(inst), globalArrayElementFacts); ok {
			tableFacts[a] = fact
		} else {
			kill(a)
		}
	case vm.OP_GETTABLE:
		if typedSelfSlotIsTable(getSpecializedSlot(slots, b)) && typedSelfRKIsInt(slots, proto, c) {
			if fact, ok := tableFacts[b]; ok && fixedShapeTableFactHasUsableTableFact(fact) {
				tableFacts[a] = cloneFixedShapeTableFact(fact)
				return
			}
		}
		kill(a)
	case vm.OP_GETFIELD:
		name := typedSelfConstFieldName(proto, c)
		if fact, ok := tableFacts[b]; ok {
			if nested, ok := typedSelfNestedTableFactFromFact(fact, name); ok {
				tableFacts[a] = nested
				return
			}
		}
		kill(a)
	case vm.OP_LOADNIL:
		for slot := a; slot <= a+b && slot < maxTrackedSlots; slot++ {
			kill(slot)
		}
	case vm.OP_CALL:
		callC := vm.DecodeC(inst)
		if callC == 0 {
			for slot := a; slot < maxTrackedSlots; slot++ {
				kill(slot)
			}
		} else {
			for slot := a; slot < a+callC-1 && slot < maxTrackedSlots; slot++ {
				kill(slot)
			}
		}
	case vm.OP_SETFIELD, vm.OP_SETTABLE, vm.OP_SETGLOBAL, vm.OP_SETUPVAL, vm.OP_JMP, vm.OP_EQ,
		vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_RETURN, vm.OP_CLOSE:
		return
	case vm.OP_FORLOOP:
		kill(a)
		kill(a + 3)
	case vm.OP_FORPREP:
		kill(a)
	case vm.OP_SELF:
		kill(a)
		kill(a + 1)
	case vm.OP_TFORCALL:
		callC := vm.DecodeC(inst)
		for slot := a + 3; slot < a+3+callC && slot < maxTrackedSlots; slot++ {
			kill(slot)
		}
	default:
		kill(a)
	}
}

func typedSelfForLoopStableSlots(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int) ([]specializedSlotRep, []specializedSlotRep, bool) {
	return typedSelfForLoopStableSlotsWithGlobals(proto, params, forLoopPC, a, nil)
}

func typedSelfForLoopStableSlotsWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, numericGlobals map[string]runtime.Value) ([]specializedSlotRep, []specializedSlotRep, bool) {
	return typedSelfForLoopStableSlotsWithFactsAndGlobals(proto, params, forLoopPC, a, numericGlobals, nil)
}

func typedSelfForLoopStableSlotsWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) ([]specializedSlotRep, []specializedSlotRep, bool) {
	if proto == nil || forLoopPC <= 0 {
		return nil, nil, false
	}
	prepPC := typedSelfFindForPrep(proto, forLoopPC, a)
	if prepPC < 0 {
		return nil, nil, false
	}
	slots := make([]specializedSlotRep, maxTrackedSlots)
	typedSelfResetSlots(slots, params)
	for pc := 0; pc <= prepPC; pc++ {
		if !typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto, slots, pc, numericGlobals, globalArrayElementFacts) {
			return nil, nil, false
		}
	}
	preSlots := append([]specializedSlotRep(nil), slots...)
	for pc := prepPC + 1; pc < forLoopPC; pc++ {
		if !typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto, slots, pc, numericGlobals, globalArrayElementFacts) {
			return nil, nil, false
		}
	}
	postSlots := append([]specializedSlotRep(nil), slots...)
	return preSlots, postSlots, true
}

func typedSelfForLoopPreSlotsWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, numericGlobals map[string]runtime.Value) ([]specializedSlotRep, bool) {
	return typedSelfForLoopPreSlotsWithFactsAndGlobals(proto, params, forLoopPC, a, numericGlobals, nil)
}

func typedSelfForLoopPreSlotsWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) ([]specializedSlotRep, bool) {
	if proto == nil || forLoopPC <= 0 {
		return nil, false
	}
	prepPC := typedSelfFindForPrep(proto, forLoopPC, a)
	if prepPC < 0 {
		return nil, false
	}
	return typedSelfSlotsAtPCWithFactsAndGlobals(proto, params, prepPC, numericGlobals, globalArrayElementFacts)
}

func typedSelfFindForPrep(proto *vm.FuncProto, forLoopPC, a int) int {
	if proto == nil {
		return -1
	}
	for pc := forLoopPC - 1; pc >= 0; pc-- {
		if vm.DecodeOp(proto.Code[pc]) == vm.OP_FORPREP && vm.DecodeA(proto.Code[pc]) == a {
			return pc
		}
	}
	return -1
}

func typedSelfLoopBodyWritesSlot(proto *vm.FuncProto, bodyStart, forLoopPC, slot int) bool {
	if proto == nil || slot < 0 || bodyStart < 0 || forLoopPC < bodyStart || forLoopPC > len(proto.Code) {
		return true
	}
	for pc := bodyStart; pc < forLoopPC; pc++ {
		if typedSelfInstrWritesSlot(proto.Code[pc], slot) {
			return true
		}
	}
	return false
}

func typedSelfInstrWritesSlot(inst uint32, slot int) bool {
	op := vm.DecodeOp(inst)
	a := vm.DecodeA(inst)
	switch op {
	case vm.OP_SETUPVAL, vm.OP_SETGLOBAL, vm.OP_SETFIELD, vm.OP_SETTABLE, vm.OP_SETLIST,
		vm.OP_JMP, vm.OP_EQ, vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_RETURN,
		vm.OP_CLOSE, vm.OP_TFORLOOP:
		return false
	case vm.OP_LOADNIL:
		b := vm.DecodeB(inst)
		return slot >= a && slot <= a+b
	case vm.OP_CALL:
		c := vm.DecodeC(inst)
		if c == 0 {
			return slot >= a
		}
		return slot >= a && slot < a+c-1
	case vm.OP_FORPREP:
		return slot == a
	case vm.OP_FORLOOP:
		return slot == a || slot == a+3
	case vm.OP_SELF:
		return slot == a || slot == a+1
	case vm.OP_TFORCALL:
		c := vm.DecodeC(inst)
		return slot >= a+3 && slot < a+3+c
	default:
		return slot == a
	}
}

func typedSelfApplyStableForLoopFacts(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, slots []specializedSlotRep) {
	typedSelfApplyStableForLoopFactsWithGlobals(proto, params, forLoopPC, a, slots, nil)
}

func typedSelfApplyStableForLoopFactsWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, slots []specializedSlotRep, numericGlobals map[string]runtime.Value) {
	typedSelfApplyStableForLoopFactsWithFactsAndGlobals(proto, params, forLoopPC, a, slots, numericGlobals, nil)
}

func typedSelfApplyStableForLoopFactsWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, slots []specializedSlotRep, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) {
	preSlots, postSlots, ok := typedSelfForLoopStableSlotsWithFactsAndGlobals(proto, params, forLoopPC, a, numericGlobals, globalArrayElementFacts)
	if ok {
		for slot, pre := range preSlots {
			if pre != postSlots[slot] {
				continue
			}
			switch pre {
			case specializedSlotRawInt, specializedSlotRawTable, specializedSlotNil,
				specializedSlotSelfFunc, specializedSlotOtherFunc, specializedSlotStdMathTable,
				specializedSlotMathSqrtFunc, specializedSlotMathFloorFunc:
				setSpecializedSlot(slots, slot, pre)
			}
		}
	}
	setSpecializedSlot(slots, a, specializedSlotRawInt)
	setSpecializedSlot(slots, a+1, specializedSlotRawInt)
	setSpecializedSlot(slots, a+2, specializedSlotRawInt)
}

func typedSelfForLoopControlProvenInt(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int) bool {
	return typedSelfForLoopControlProvenIntWithGlobals(proto, params, forLoopPC, a, nil)
}

func typedSelfForLoopControlProvenIntWithGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, numericGlobals map[string]runtime.Value) bool {
	return typedSelfForLoopControlProvenIntWithFactsAndGlobals(proto, params, forLoopPC, a, numericGlobals, nil)
}

func typedSelfForLoopControlProvenIntWithFactsAndGlobals(proto *vm.FuncProto, params []SpecializedABIParamRep, forLoopPC, a int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) bool {
	if proto == nil || forLoopPC <= 0 {
		return false
	}
	for pc := forLoopPC - 1; pc >= 0; pc-- {
		inst := proto.Code[pc]
		if vm.DecodeOp(inst) != vm.OP_FORPREP || vm.DecodeA(inst) != a {
			continue
		}
		slots, ok := typedSelfSlotsAtPCWithFactsAndGlobals(proto, params, pc, numericGlobals, globalArrayElementFacts)
		if !ok {
			return false
		}
		return typedSelfSlotIsInt(getSpecializedSlot(slots, a)) &&
			typedSelfSlotIsInt(getSpecializedSlot(slots, a+1)) &&
			typedSelfSlotIsInt(getSpecializedSlot(slots, a+2))
	}
	return false
}

func typedSelfAdvanceSimpleSlotFact(proto *vm.FuncProto, slots []specializedSlotRep, pc int) bool {
	return typedSelfAdvanceSimpleSlotFactWithGlobals(proto, slots, pc, nil)
}

func typedSelfAdvanceSimpleSlotFactWithGlobals(proto *vm.FuncProto, slots []specializedSlotRep, pc int, numericGlobals map[string]runtime.Value) bool {
	return typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto, slots, pc, numericGlobals, nil)
}

func typedSelfAdvanceSimpleSlotFactWithGlobalFacts(proto *vm.FuncProto, slots []specializedSlotRep, pc int, numericGlobals map[string]runtime.Value, globalArrayElementFacts map[string]FixedShapeTableFact) bool {
	inst := proto.Code[pc]
	op := vm.DecodeOp(inst)
	a := vm.DecodeA(inst)
	b := vm.DecodeB(inst)
	c := vm.DecodeC(inst)
	switch op {
	case vm.OP_LOADINT:
		setSpecializedSlot(slots, a, specializedSlotRawInt)
	case vm.OP_LOADK:
		if specializedABIConstIsInt(proto, vm.DecodeBx(inst)) {
			setSpecializedSlot(slots, a, specializedSlotRawInt)
		} else {
			setSpecializedSlot(slots, a, specializedSlotUnknown)
		}
	case vm.OP_MOVE:
		setSpecializedSlot(slots, a, getSpecializedSlot(slots, b))
	case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_MOD:
		if typedSelfRKIsInt(slots, proto, b) && typedSelfRKIsInt(slots, proto, c) {
			setSpecializedSlot(slots, a, specializedSlotRawInt)
		} else {
			setSpecializedSlot(slots, a, specializedSlotUnknown)
		}
	case vm.OP_GETTABLE, vm.OP_GETFIELD:
		if op == vm.OP_GETFIELD && getSpecializedSlot(slots, b) == specializedSlotStdMathTable && typedSelfConstFieldName(proto, c) == "sqrt" {
			setSpecializedSlot(slots, a, specializedSlotMathSqrtFunc)
		} else if op == vm.OP_GETFIELD && getSpecializedSlot(slots, b) == specializedSlotStdMathTable && typedSelfConstFieldName(proto, c) == "floor" {
			setSpecializedSlot(slots, a, specializedSlotMathFloorFunc)
		} else if op == vm.OP_GETTABLE && typedSelfSlotIsTable(getSpecializedSlot(slots, b)) && typedSelfRKIsInt(slots, proto, c) {
			setSpecializedSlot(slots, a, specializedSlotRawTable)
		} else if typedSelfFeedbackResultIsInt(proto, pc) {
			setSpecializedSlot(slots, a, specializedSlotRawInt)
		} else if typedSelfFeedbackResultIsTable(proto, pc) {
			setSpecializedSlot(slots, a, specializedSlotRawTable)
		} else {
			setSpecializedSlot(slots, a, specializedSlotUnknown)
		}
	case vm.OP_NEWTABLE, vm.OP_NEWOBJECT2, vm.OP_NEWOBJECTN:
		setSpecializedSlot(slots, a, specializedSlotRawTable)
	case vm.OP_LOADNIL:
		for slot := a; slot <= a+b && slot < len(slots); slot++ {
			setSpecializedSlot(slots, slot, specializedSlotNil)
		}
	case vm.OP_GETGLOBAL:
		if specializedABIConstString(proto, vm.DecodeBx(inst)) == proto.Name {
			setSpecializedSlot(slots, a, specializedSlotSelfFunc)
		} else if rep, ok := typedSelfNumericGlobalRep(proto, vm.DecodeBx(inst), numericGlobals); ok {
			setSpecializedSlot(slots, a, rep)
		} else if _, ok := typedSelfGlobalArrayElementFact(proto, vm.DecodeBx(inst), globalArrayElementFacts); ok {
			setSpecializedSlot(slots, a, specializedSlotRawTable)
		} else if specializedABIConstString(proto, vm.DecodeBx(inst)) == "math" {
			setSpecializedSlot(slots, a, specializedSlotStdMathTable)
		} else {
			setSpecializedSlot(slots, a, specializedSlotOtherFunc)
		}
	case vm.OP_CALL:
		if typedSelfSlotIsMathUnaryFunc(getSpecializedSlot(slots, a)) && b == 2 && c == 2 &&
			typedSelfSlotIsNumeric(getSpecializedSlot(slots, a+1)) {
			if getSpecializedSlot(slots, a) == specializedSlotMathFloorFunc {
				setSpecializedSlot(slots, a, specializedSlotRawInt)
			} else {
				setSpecializedSlot(slots, a, specializedSlotRawFloat)
			}
		} else {
			setSpecializedSlot(slots, a, specializedSlotUnknown)
		}
	case vm.OP_FORPREP:
		if typedSelfSlotIsInt(getSpecializedSlot(slots, a)) &&
			typedSelfSlotIsInt(getSpecializedSlot(slots, a+1)) &&
			typedSelfSlotIsInt(getSpecializedSlot(slots, a+2)) {
			setSpecializedSlot(slots, a, specializedSlotRawInt)
		} else {
			return false
		}
	case vm.OP_LE, vm.OP_LT, vm.OP_EQ, vm.OP_JMP, vm.OP_SETUPVAL, vm.OP_CLOSE,
		vm.OP_SETTABLE, vm.OP_SETFIELD:
	default:
		setSpecializedSlot(slots, a, specializedSlotUnknown)
	}
	return true
}

func typedSelfNumericGlobalRep(proto *vm.FuncProto, constIdx int, numericGlobals map[string]runtime.Value) (specializedSlotRep, bool) {
	if proto == nil || len(numericGlobals) == 0 || constIdx < 0 || constIdx >= len(proto.Constants) {
		return specializedSlotUnknown, false
	}
	c := proto.Constants[constIdx]
	if !c.IsString() {
		return specializedSlotUnknown, false
	}
	v, ok := numericGlobals[c.Str()]
	if !ok {
		return specializedSlotUnknown, false
	}
	if v.IsInt() {
		return specializedSlotRawInt, true
	}
	if v.IsFloat() {
		return specializedSlotRawFloat, true
	}
	return specializedSlotUnknown, false
}

func typedSelfGlobalArrayElementFact(proto *vm.FuncProto, constIdx int, globalFacts map[string]FixedShapeTableFact) (FixedShapeTableFact, bool) {
	if proto == nil || len(globalFacts) == 0 || constIdx < 0 || constIdx >= len(proto.Constants) {
		return FixedShapeTableFact{}, false
	}
	c := proto.Constants[constIdx]
	if !c.IsString() {
		return FixedShapeTableFact{}, false
	}
	fact, ok := globalFacts[c.Str()]
	if !ok || !fixedShapeTableFactHasUsableTableFact(fact) {
		return FixedShapeTableFact{}, false
	}
	return cloneFixedShapeTableFact(fact), true
}
