package methodjit

// FloorPhiSplitPass pushes floor(phi(...)) to predecessor edges. This keeps
// integer inputs as integers and only floors the floating inputs, avoiding a
// mixed numeric phi that widens all paths to float and then immediately floors
// them back to int.
func FloorPhiSplitPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	for iter := 0; iter < 8; iter++ {
		changed := false
		for _, block := range fn.Blocks {
			if block == nil {
				continue
			}
			for _, instr := range block.Instrs {
				if instr == nil || instr.Op != OpFloor || len(instr.Args) != 1 || instr.Args[0] == nil {
					continue
				}
				phi := instr.Args[0].Def
				if phi == nil || phi.Op != OpPhi || phi.Block == nil || len(phi.Args) == 0 || len(phi.Args) != len(phi.Block.Preds) {
					continue
				}
				repl, ok := splitFloorPhi(fn, instr, phi)
				if !ok {
					continue
				}
				replaceValueUses(fn, instr.ID, repl.Value(), repl.ID)
				instr.Op = OpNop
				instr.Type = TypeUnknown
				instr.Args = nil
				functionRemarks(fn).Add("FloorPhiSplit", "changed", block.ID, instr.ID, OpFloor,
					"pushed floor through numeric phi")
				changed = true
				break
			}
			if changed {
				break
			}
		}
		if !changed {
			break
		}
	}
	return fn, nil
}

func splitFloorPhi(fn *Function, floor, phi *Instr) (*Instr, bool) {
	args := make([]*Value, len(phi.Args))
	changed := false
	for i, arg := range phi.Args {
		if arg == nil {
			return nil, false
		}
		switch floorPhiValueType(arg) {
		case TypeInt:
			args[i] = arg
			changed = true
			continue
		}
		if arg.Def != nil && arg.Def.Op == OpFloor {
			args[i] = arg
			changed = true
			continue
		}
		pred := phi.Block.Preds[i]
		if pred == nil {
			return nil, false
		}
		edgeFloor := &Instr{
			ID:    fn.newValueID(),
			Op:    OpFloor,
			Type:  TypeInt,
			Args:  []*Value{arg},
			Block: pred,
		}
		edgeFloor.copySourceFrom(floor)
		insertBeforeTerminator(pred, edgeFloor)
		args[i] = edgeFloor.Value()
		changed = true
	}
	if !changed {
		return nil, false
	}
	out := &Instr{
		ID:    fn.newValueID(),
		Op:    OpPhi,
		Type:  TypeInt,
		Args:  args,
		Block: phi.Block,
	}
	out.copySourceFrom(floor)
	insertHeaderPhi(phi.Block, out)
	return out, true
}

func floorPhiValueType(v *Value) Type {
	if v == nil || v.Def == nil {
		return TypeUnknown
	}
	if spec, ok := v.Def.Op.Spec(); ok && spec.FixedResultType != TypeUnknown {
		return spec.FixedResultType
	}
	if v.Def.Type != TypeUnknown && v.Def.Type != TypeAny {
		return v.Def.Type
	}
	return TypeUnknown
}
