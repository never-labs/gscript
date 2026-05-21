package methodjit

// ObservedParamTypeGuardPass inserts speculative type guards on function
// parameters whose runtime type is stable. Currently limited to numeric
// types (int/float) since table/string parameters can be nil at callsites
// even when the observed feedback only saw non-nil values.
func ObservedParamTypeGuardPass(fn *Function) (*Function, error) {
	if fn == nil || fn.Proto == nil || fn.Entry == nil || len(fn.Proto.ParamTypeFeedback) == 0 {
		return fn, nil
	}

	block := fn.Entry
	for i := 0; i < len(block.Instrs); i++ {
		instr := block.Instrs[i]
		slot := paramLoadSlot(fn, instr)
		if slot < 0 || slot >= len(fn.Proto.ParamTypeFeedback) {
			continue
		}

		// Check if already guarded by a type guard right after the load.
		if i+1 < len(block.Instrs) {
			next := block.Instrs[i+1]
			if next != nil && next.Op == OpGuardType && len(next.Args) == 1 && next.Args[0] != nil && next.Args[0].ID == instr.ID {
				continue
			}
		}

		rf := fn.Proto.ParamTypeFeedback[slot]
		if rf.Count < observedParamTypeGuardMinCount {
			continue
		}
		irType, ok := feedbackToIRType(rf.Type)
		if !ok || irType == TypeAny || irType == TypeUnknown {
			continue
		}
		// Only guard numeric types — table/string parameters can be nil
		// at callsites even when feedback only observed non-nil values.
		if irType != TypeInt && irType != TypeFloat {
			continue
		}

		guard := &Instr{
			ID:          fn.newValueID(),
			Op:          OpGuardType,
			Type:        irType,
			Args:        []*Value{instr.Value()},
			Aux:         int64(irType),
			Block:       block,
			HasSource:   instr.HasSource,
			SourceProto: instr.SourceProto,
			SourcePC:    instr.SourcePC,
			SourceLine:  instr.SourceLine,
		}
		block.Instrs = append(block.Instrs, nil)
		copy(block.Instrs[i+2:], block.Instrs[i+1:])
		block.Instrs[i+1] = guard
		replaceUsesAfterGuard(fn, instr.ID, guard, guard.ID)
		i++
	}
	return fn, nil
}

// paramLoadSlot returns the parameter slot index for a LoadSlot instruction, or -1.
func paramLoadSlot(fn *Function, instr *Instr) int {
	if instr == nil || instr.Op != OpLoadSlot || len(instr.Args) > 0 {
		return -1
	}
	slot := int(instr.Aux)
	if fn == nil || fn.Proto == nil || slot < 0 || slot >= fn.Proto.NumParams {
		return -1
	}
	return slot
}

const observedParamTypeGuardMinCount = 2
