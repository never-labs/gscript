package methodjit

// StaticTableLenFoldPass folds #t for tables whose array length is fixed by
// dominating SetList construction inside the same function. It is deliberately
// conservative: any dynamic length write or call that receives the table
// invalidates the fact for the whole function.
func StaticTableLenFoldPass(fn *Function) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	facts := collectStaticTableLenFacts(fn)
	if len(facts) == 0 {
		return fn, nil
	}
	dom := computeDominators(fn)
	order := blockInstructionOrder(fn)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpLen || len(instr.Args) < 1 || instr.Args[0] == nil {
				continue
			}
			tableID := instr.Args[0].ID
			fact, ok := staticTableLenFactForLen(facts[tableID], dom, order, block.ID, instr.ID)
			if !ok {
				continue
			}
			rewriteInstrToConstInt(instr, fact.length)
			functionRemarks(fn).Add("StaticTableLenFold", "changed", block.ID, instr.ID, instr.Op,
				"folded length of SetList-constructed local table")
		}
	}
	return fn, nil
}

type staticTableLenFact struct {
	blockID int
	instrID int
	length  int64
}

func collectStaticTableLenFacts(fn *Function) map[int][]staticTableLenFact {
	newTables := make(map[int]bool)
	invalid := make(map[int]bool)
	facts := make(map[int][]staticTableLenFact)
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch {
			case opIsStaticTableLenBuilder(instr.Op):
				newTables[instr.ID] = true
			case opIsStaticTableLenInitializer(instr.Op):
				recordStaticTableLenInitializer(block, instr, newTables, invalid, facts)
			case opIsStaticTableLenInvalidator(instr.Op):
				invalidateStaticTableLenFirstArg(instr, invalid)
				invalidateStaticTableLenValueArgs(instr, newTables, invalid)
			default:
				if opIsCallLikeFactBarrier(instr.Op) {
					for _, arg := range instr.Args {
						if arg != nil {
							invalid[arg.ID] = true
						}
					}
					continue
				}
				for _, arg := range instr.Args {
					if arg != nil && newTables[arg.ID] && !staticTableLenBenignUse(instr, arg.ID) {
						invalid[arg.ID] = true
					}
				}
			}
		}
	}
	for tableID := range invalid {
		delete(facts, tableID)
	}
	return facts
}

func recordStaticTableLenInitializer(block *Block, instr *Instr, newTables map[int]bool, invalid map[int]bool, facts map[int][]staticTableLenFact) {
	if block == nil || instr == nil || len(instr.Args) < 1 || instr.Args[0] == nil {
		return
	}
	invalidateStaticTableLenValueArgs(instr, newTables, invalid)
	tableID := instr.Args[0].ID
	if !newTables[tableID] || invalid[tableID] {
		return
	}
	end := instr.Aux + int64(len(instr.Args)-1) - 1
	if end < 0 {
		return
	}
	facts[tableID] = append(facts[tableID], staticTableLenFact{
		blockID: block.ID,
		instrID: instr.ID,
		length:  end,
	})
}

func invalidateStaticTableLenFirstArg(instr *Instr, invalid map[int]bool) {
	if instr != nil && len(instr.Args) >= 1 && instr.Args[0] != nil {
		invalid[instr.Args[0].ID] = true
	}
}

func invalidateStaticTableLenValueArgs(instr *Instr, newTables map[int]bool, invalid map[int]bool) {
	if instr == nil {
		return
	}
	for _, arg := range instr.Args[1:] {
		if arg != nil && newTables[arg.ID] {
			invalid[arg.ID] = true
		}
	}
}

func staticTableLenBenignUse(instr *Instr, tableID int) bool {
	if instr == nil {
		return false
	}
	if !opIsStaticTableLenBenignUse(instr.Op) {
		return false
	}
	return len(instr.Args) >= 1 && instr.Args[0] != nil && instr.Args[0].ID == tableID
}

func staticTableLenFactForLen(facts []staticTableLenFact, dom *domInfo, order map[int]map[int]int, blockID, instrID int) (staticTableLenFact, bool) {
	var best staticTableLenFact
	ok := false
	for _, fact := range facts {
		if !staticTableLenFactDominatesUse(fact, dom, order, blockID, instrID) {
			continue
		}
		if !ok || fact.length > best.length {
			best = fact
			ok = true
		}
	}
	return best, ok
}

func staticTableLenFactDominatesUse(fact staticTableLenFact, dom *domInfo, order map[int]map[int]int, blockID, instrID int) bool {
	if fact.blockID == blockID {
		blockOrder := order[blockID]
		if blockOrder == nil {
			return false
		}
		return blockOrder[fact.instrID] < blockOrder[instrID]
	}
	return dom != nil && dom.dominates(fact.blockID, blockID)
}

func blockInstructionOrder(fn *Function) map[int]map[int]int {
	out := make(map[int]map[int]int)
	if fn == nil {
		return out
	}
	for _, block := range fn.Blocks {
		if block == nil {
			continue
		}
		m := make(map[int]int, len(block.Instrs))
		for i, instr := range block.Instrs {
			if instr != nil {
				m[instr.ID] = i
			}
		}
		out[block.ID] = m
	}
	return out
}
