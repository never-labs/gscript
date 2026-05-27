//go:build darwin && arm64

package methodjit

// tier2NativeCallReplaySafe reports whether a native caller may enter fn
// through a direct entry. Native callers cannot resume a callee at the
// callee's own exit-resume point; if the callee exits, the caller falls back by
// re-executing the call from the start. That is only correct when no visible
// side effect can happen before a later exit/deopt in the callee.
func tier2NativeCallReplaySafe(fn *Function) bool {
	if fn == nil || fn.Entry == nil {
		return true
	}

	type workItem struct {
		block          *Block
		seenSideEffect bool
	}
	seenState := make(map[struct {
		blockID        int
		seenSideEffect bool
	}]bool, len(fn.Blocks)*2)
	work := []workItem{{block: fn.Entry}}
	for len(work) > 0 {
		item := work[len(work)-1]
		work = work[:len(work)-1]
		block := item.block
		if block == nil {
			continue
		}

		seenSideEffect := item.seenSideEffect
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			if seenSideEffect && tier2OpMayExitForNativeReplay(instr) {
				return false
			}
			if tier2InstrHasNativeVisibleSideEffect(instr) {
				seenSideEffect = true
			}
		}

		for _, succ := range block.Succs {
			if succ == nil {
				continue
			}
			key := struct {
				blockID        int
				seenSideEffect bool
			}{blockID: succ.ID, seenSideEffect: seenSideEffect}
			if seenState[key] {
				continue
			}
			seenState[key] = true
			work = append(work, workItem{block: succ, seenSideEffect: seenSideEffect})
		}
	}
	return true
}

// tier2NativeCallCalleeResumeSafe reports whether Tier 2 callers may use the
// native callee-resume protocol for fn. Heap/table mutations are safe because
// the caller resumes the callee instead of replaying it. Mutations to VM
// execution context remain gated: the caller may still hold global/upvalue or
// concurrency context bindings from before the native call.
func tier2NativeCallCalleeResumeSafe(fn *Function) bool {
	if fn == nil {
		return true
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			spec, ok := instr.Op.Spec()
			if ok && spec.NativeCalleeResumeUnsafe {
				return false
			}
		}
	}
	return true
}

func tier2InstrHasNativeVisibleSideEffect(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	if !ok {
		return false
	}
	if spec.NativeReplayVisibleTableMutation {
		if len(instr.Args) == 0 {
			return true
		}
		return !tier2ValueIsLocalTableAllocation(instr.Args[0], make(map[int]bool))
	}
	return spec.NativeReplayVisibleSideEffect
}

func tier2ValueIsLocalTableAllocation(v *Value, seen map[int]bool) bool {
	if v == nil || v.Def == nil {
		return false
	}
	if seen[v.ID] {
		return true
	}
	seen[v.ID] = true
	switch v.Def.Op {
	case OpNewTable, OpNewFixedTable:
		return true
	case OpPhi:
		if len(v.Def.Args) == 0 {
			return false
		}
		for _, arg := range v.Def.Args {
			if !tier2ValueIsLocalTableAllocation(arg, seen) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func tier2OpMayExitForNativeReplay(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.NativeReplayMayExit
}
