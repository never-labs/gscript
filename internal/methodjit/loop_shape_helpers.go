//go:build darwin && arm64

package methodjit

func firstTwoPhis(block *Block) (*Instr, *Instr) {
	if block == nil {
		return nil, nil
	}
	var out []*Instr
	for _, instr := range block.Instrs {
		if instr == nil || instr.Op != OpPhi {
			break
		}
		out = append(out, instr)
	}
	if len(out) < 2 {
		return nil, nil
	}
	return out[0], out[1]
}

func parseUnitBoundedLoopHeader(header *Block, indexPhi *Instr) (*Value, *Value, bool) {
	if header == nil || indexPhi == nil {
		return nil, nil, false
	}
	var next, limit *Value
	for _, instr := range header.Instrs {
		if instr == nil {
			continue
		}
		if instr.Op == OpAddInt && len(instr.Args) == 2 && isAddOneOf(instr.Value(), indexPhi.Value()) {
			next = instr.Value()
		}
		if instr.Op == OpLeInt && len(instr.Args) == 2 {
			limit = instr.Args[1]
		}
	}
	if next == nil || limit == nil {
		return nil, nil, false
	}
	return next, limit, true
}

func parseAnyUnitBoundedLoopHeader(header *Block) (*Value, *Value, bool) {
	if header == nil {
		return nil, nil, false
	}
	for _, instr := range header.Instrs {
		if instr == nil || instr.Op != OpLeInt || len(instr.Args) != 2 {
			continue
		}
		if add := instr.Args[0]; add != nil && add.Def != nil && add.Def.Op == OpAddInt && len(add.Def.Args) == 2 {
			return add, instr.Args[1], true
		}
	}
	return nil, nil, false
}
