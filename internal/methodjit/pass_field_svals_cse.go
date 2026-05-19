package methodjit

import "fmt"

// FieldSvalsCSEPass merges duplicate OpFieldSvals values that prove the same
// table value still has the same fixed shape. FieldSvalsLower can create
// multiple svals anchors around stores or lowered table operations; keeping
// one SSA value avoids repeated shape checks and svals pointer loads.
func FieldSvalsCSEPass(fn *Function) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	changed := false
	dom := computeDominators(fn)
	var seen []*Instr
	for _, block := range fn.Blocks {
		if block == nil || len(block.Instrs) == 0 {
			continue
		}
		available := make(map[fieldSvalsLowerKey]*Instr)
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			if fieldSvalsGlobalBarrier(instr) {
				available = make(map[fieldSvalsLowerKey]*Instr)
				continue
			}
			if tableID, ok := fieldSvalsMutationTableID(instr); ok {
				for key := range available {
					if key.tableID == tableID {
						delete(available, key)
					}
				}
			}
			if instr.Op != OpFieldSvals || len(instr.Args) == 0 || instr.Args[0] == nil || instr.Aux == 0 {
				continue
			}
			key := fieldSvalsLowerKey{tableID: instr.Args[0].ID, shapeID: uint32(instr.Aux)}
			if prev := available[key]; prev != nil {
				replaceValueUses(fn, instr.ID, prev.Value(), prev.ID)
				instr.Op = OpNop
				instr.Type = TypeUnknown
				instr.Args = nil
				instr.Aux = 0
				instr.Aux2 = 0
				changed = true
				functionRemarks(fn).Add("FieldSvalsCSE", "changed", block.ID, prev.ID, prev.Op,
					fmt.Sprintf("reused svals v%d for table v%d shape %d", prev.ID, key.tableID, key.shapeID))
				continue
			}
			if prev := findReusableCrossBlockFieldSvals(fn, dom, seen, block, instr, key); prev != nil {
				replaceValueUses(fn, instr.ID, prev.Value(), prev.ID)
				instr.Op = OpNop
				instr.Type = TypeUnknown
				instr.Args = nil
				instr.Aux = 0
				instr.Aux2 = 0
				changed = true
				functionRemarks(fn).Add("FieldSvalsCSE", "changed", block.ID, prev.ID, prev.Op,
					fmt.Sprintf("reused dominating svals v%d for table v%d shape %d", prev.ID, key.tableID, key.shapeID))
				continue
			}
			available[key] = instr
			seen = append(seen, instr)
		}
	}
	if changed {
		relinkValueDefs(fn)
	}
	return fn, nil
}

func findReusableCrossBlockFieldSvals(fn *Function, dom *domInfo, seen []*Instr, block *Block, instr *Instr, key fieldSvalsLowerKey) *Instr {
	if fn == nil || dom == nil || block == nil || instr == nil {
		return nil
	}
	var best *Instr
	bestOrder := fieldSvalsLowerOrder{block: -1, index: -1}
	for _, prev := range seen {
		if prev == nil || prev.Block == nil || len(prev.Args) == 0 || prev.Args[0] == nil {
			continue
		}
		if prev.Block.ID == block.ID || prev.Args[0].ID != key.tableID || uint32(prev.Aux) != key.shapeID {
			continue
		}
		if !dom.dominates(prev.Block.ID, block.ID) || !fieldSvalsPathSafe(fn, prev, instr, key.tableID) {
			continue
		}
		order := fieldSvalsLowerDefOrder(fn, prev.ID)
		if best == nil || order.block > bestOrder.block || (order.block == bestOrder.block && order.index > bestOrder.index) {
			best = prev
			bestOrder = order
		}
	}
	return best
}
