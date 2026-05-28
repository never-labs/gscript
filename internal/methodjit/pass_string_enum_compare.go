//go:build darwin && arm64

package methodjit

// StringEnumComparePass rewrites comparisons against values loaded from a
// local const-string array into comparisons against the integer array index.
// This is useful for enum-like code such as:
//
//	channel := channels[i]
//	if channel == "web" { ... }
//
// when channels is built by SetList from constant strings and never mutated.
func StringEnumComparePass(fn *Function) (*Function, error) {
	if fn == nil || fn.Proto == nil {
		return fn, nil
	}
	enums := collectConstStringArrayEnums(fn)
	if len(enums) == 0 {
		return fn, nil
	}
	dataOwners := tableArrayDataOwners(fn)
	if len(dataOwners) == 0 {
		return fn, nil
	}
	changed := false
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for i := 0; i < len(block.Instrs); i++ {
			instr := block.Instrs[i]
			if instr == nil {
				continue
			}
			lowered, ok := stringEnumCompareLoweredOp(instr.Op)
			if !ok || lowered != OpEqInt || len(instr.Args) < 2 {
				continue
			}
			load, constArg := stringEnumCompareParts(instr)
			if load == nil || constArg == nil || len(load.Args) < 3 || load.Args[0] == nil || load.Args[2] == nil {
				continue
			}
			tableID, ok := dataOwners[load.Args[0].ID]
			if !ok {
				continue
			}
			enum, ok := enums[tableID]
			if !ok {
				continue
			}
			constIdx := int(constArg.Def.Aux)
			if constIdx < 0 {
				continue
			}
			key := protoConstString(fn.Proto, constIdx)
			if key == "" {
				continue
			}
			oneBasedIndex, ok := enum[key]
			if !ok {
				continue
			}
			idxConst := &Instr{
				ID:    fn.newValueID(),
				Op:    OpConstInt,
				Type:  TypeInt,
				Aux:   int64(oneBasedIndex),
				Block: block,
			}
			block.Instrs = append(block.Instrs[:i], append([]*Instr{idxConst}, block.Instrs[i:]...)...)
			i++
			instr.Op = lowered
			instr.Type = TypeBool
			instr.Args = []*Value{load.Args[2], idxConst.Value()}
			functionRemarks(fn).Add("StringEnumCompare", "changed", block.ID, instr.ID, instr.Op,
				"rewrote const-string array value compare to index compare")
			changed = true
		}
	}
	if changed {
		return DCEPass(fn)
	}
	return fn, nil
}

func collectConstStringArrayEnums(fn *Function) map[int]map[string]int {
	newTables := make(map[int]bool)
	invalid := make(map[int]bool)
	enums := make(map[int]map[string]int)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch instr.Op {
			case OpNewTable:
				newTables[instr.ID] = true
			case OpSetList:
				if len(instr.Args) < 2 || instr.Args[0] == nil || !newTables[instr.Args[0].ID] {
					continue
				}
				tableID := instr.Args[0].ID
				values := make(map[string]int, len(instr.Args)-1)
				ok := true
				for i, arg := range instr.Args[1:] {
					if arg == nil || arg.Def == nil || arg.Def.Op != OpConstString {
						ok = false
						break
					}
					s := protoConstString(fn.Proto, int(arg.Def.Aux))
					if s == "" {
						ok = false
						break
					}
					if _, exists := values[s]; exists {
						ok = false
						break
					}
					values[s] = i + 1
				}
				if ok {
					enums[tableID] = values
				}
			case OpSetTable, OpTableArrayStore, OpAppend:
				if len(instr.Args) > 0 && instr.Args[0] != nil {
					invalid[instr.Args[0].ID] = true
				}
			}
		}
	}
	for id := range invalid {
		delete(enums, id)
	}
	return enums
}

func stringEnumCompareParts(instr *Instr) (*Instr, *Value) {
	if instr == nil || len(instr.Args) < 2 {
		return nil, nil
	}
	left := instr.Args[0]
	right := instr.Args[1]
	if left != nil && left.Def != nil && left.Def.Op == OpTableArrayLoad &&
		right != nil && right.Def != nil && right.Def.Op == OpConstString {
		return left.Def, right
	}
	if right != nil && right.Def != nil && right.Def.Op == OpTableArrayLoad &&
		left != nil && left.Def != nil && left.Def.Op == OpConstString {
		return right.Def, left
	}
	return nil, nil
}
