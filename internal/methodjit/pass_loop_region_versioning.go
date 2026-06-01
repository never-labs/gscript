package methodjit

import "github.com/never-labs/leia/internal/vm"

// LoopRegionVersioningPass recognizes single-entry natural loops whose
// preheader carries typed table-array facts and whose header branch proves a
// key is below the table-array len on every continuing iteration.
//
// This first stage does not clone CFG blocks or introduce new deopt points. It
// versions the loop by reusing already-hoisted preheader guards:
//
//	preheader:
//	  hdr  = TableArrayHeader(t)  // table/metatable/kind guard
//	  len  = TableArrayLen(hdr)
//	  data = TableArrayData(hdr)
//	header:
//	  cond = key < len
//	  Branch cond -> body, exit
//	body:
//	  TableArrayLoad(data, len, key)
//	  TableArrayStore(t, data, len, key, value[, header])
//
// The continuing path of OpTableArrayStore is structural-preserving: it writes
// an existing typed-array slot and does not change table kind, backing data, or
// len. Any miss exits before native execution continues, so the preheader facts
// remain valid inside the region.
func LoopRegionVersioningPass(fn *Function) (*Function, error) {
	if fn != nil {
		fn.ensureAnalysis()
	}
	allowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactIntRanges, AnalysisFactSpecDependencyProtos),
		analysisFacts(AnalysisFactTableArrayBoundsSafe, AnalysisFactLoopTableArrayFacts),
		analysisFacts(AnalysisFactTableArrayBoundsSafe),
		analysisFacts(AnalysisFactGlobals),
	)
	return LoopRegionVersioningPassCtx(newPassContext(fn, nil, allowed, false))
}

func LoopRegionVersioningPassCtx(ctx *PassContext) (*Function, error) {
	if ctx == nil {
		return nil, nil
	}
	var globals map[string]*vm.FuncProto
	if globalFacts := ctx.Global(); globalFacts != nil {
		globals = globalFacts.GlobalsMap()
	}
	return loopRegionVersioningPass(ctx.Func(), globals, ctx.LoopSpecialization(), ctx.Numeric(), ctx.Speculation(), ctx.TableShape())
}

func loopRegionVersioningPass(fn *Function, seededGlobals map[string]*vm.FuncProto, loopSpecializationFacts *LoopSpecializationFacts, numeric *NumericFacts, spec *SpeculationFacts, tableShapes *TableShapeFacts) (*Function, error) {
	if fn == nil || len(fn.Blocks) == 0 {
		return fn, nil
	}
	fn.ensureAnalysis()
	if loopSpecializationFacts == nil {
		return fn, nil
	}
	loopSpecializationFacts.SetLoopTableArrayFacts(nil)

	li := computeLoopInfo(fn)
	if !li.hasLoops() {
		return fn, nil
	}

	dom := computeDominators(fn)
	preheaders := computeLoopPreheaders(fn, li)
	safe := cloneTableArrayBoolMap(loopSpecializationFacts.TableArrayUpperBoundSafeMap())
	lowerSafe := cloneTableArrayBoolMap(loopSpecializationFacts.TableArrayLowerBoundSafeMap())
	accessFacts := make(map[int]LoopTableArrayFact)

	for _, header := range fn.Blocks {
		if !li.loopHeaders[header.ID] {
			continue
		}
		preheaderID, ok := preheaders[header.ID]
		if !ok {
			functionRemarks(fn).Add("LoopRegionVersioning", "missed", header.ID, 0, OpNop,
				"loop has no single-entry preheader")
			continue
		}
		preheader := findBlockByID(fn, preheaderID)
		if preheader == nil {
			continue
		}

		guard, guardedSucc := tableArrayLoopUpperGuard(li, header)
		if guard == nil || guardedSucc == nil {
			functionRemarks(fn).Add("LoopRegionVersioning", "missed", header.ID, 0, OpNop,
				"loop header has no key < len branch guard")
			continue
		}
		if len(guard.Args) < 2 || guard.Args[0] == nil || guard.Args[1] == nil {
			continue
		}

		regionFacts := collectLoopRegionTableArrayFactsDominating(fn, dom, header.ID)
		if len(regionFacts) == 0 {
			functionRemarks(fn).Add("LoopRegionVersioning", "missed", preheader.ID, 0, OpTableArrayHeader,
				"preheader has no complete table-array header/len/data fact")
			continue
		}
		key, guardedLimit := guard.Args[0], guard.Args[1]
		limitGuards := make(map[[2]int]bool)
		for _, block := range fn.Blocks {
			if !li.headerBlocks[header.ID][block.ID] || block == header {
				continue
			}
			if !dom.dominates(guardedSucc.ID, block.ID) {
				continue
			}
			for _, instr := range block.Instrs {
				if loopRegionSkipAccess(instr) {
					continue
				}
				fact, ok := loopRegionAccessFact(header.ID, preheader.ID, instr, regionFacts, key, guardedLimit)
				if !ok && guard.Op == OpLeInt {
					if insertedLoopLimitArrayLenGuard(fn, preheader, li.headerBlocks[header.ID], key, guardedLimit, instr, regionFacts, limitGuards) {
						fact, ok = loopRegionAccessFactWithGuardedArrayLen(header.ID, preheader.ID, instr, regionFacts, key)
					}
				}
				if !ok {
					continue
				}
				if hazard := loopRegionAliasingHazard(fn, li.headerBlocks[header.ID], fact, seededGlobals, spec, tableShapes); hazard != nil {
					hazardBlockID := header.ID
					if hazard.Block != nil {
						hazardBlockID = hazard.Block.ID
					}
					functionRemarks(fn).Add("LoopRegionVersioning", "missed", hazardBlockID, hazard.ID, hazard.Op,
						"loop contains structural table mutation or aliasing call")
					continue
				}
				safe[instr.ID] = true
				if loopRegionKeyNonNegative(numeric, li, header, key) {
					lowerSafe[instr.ID] = true
					functionRemarks(fn).Add("LoopRegionVersioning", "changed", block.ID, instr.ID, instr.Op,
						"loop induction proves table-array access lower bound")
				}
				accessFacts[instr.ID] = fact
				functionRemarks(fn).Add("LoopRegionVersioning", "changed", block.ID, instr.ID, instr.Op,
					"preheader table-array fact and loop header guard prove access upper bound")
			}
		}
	}

	if len(safe) == 0 {
		loopSpecializationFacts.SetTableArrayUpperBoundSafe(nil)
		loopSpecializationFacts.SetTableArrayLowerBoundSafe(nil)
		return fn, nil
	}
	loopSpecializationFacts.SetTableArrayUpperBoundSafe(safe)
	if len(lowerSafe) > 0 {
		loopSpecializationFacts.SetTableArrayLowerBoundSafe(lowerSafe)
	}
	loopSpecializationFacts.SetLoopTableArrayFacts(accessFacts)
	return fn, nil
}

func loopRegionSkipAccess(instr *Instr) bool {
	return instr != nil &&
		instr.Op == OpTableArrayStore &&
		instr.Aux2&tableArrayStoreFlagAllowGrow != 0
}

func loopRegionKeyNonNegative(numeric *NumericFacts, li *loopInfo, header *Block, key *Value) bool {
	if li == nil || header == nil || key == nil {
		return false
	}
	if numeric != nil && numeric.IsIntNonNegative(key.ID) {
		return true
	}
	if c, ok := constIntFromValue(key); ok {
		return c >= 0
	}
	if key.Def == nil {
		return false
	}
	if numeric != nil {
		if r, ok := numeric.IntRange(key.ID); ok && r.nonNegative() {
			return true
		}
	}
	for _, instr := range header.Instrs {
		if instr == nil || instr.Op != OpPhi || !instr.Type.isIntegerLike() {
			break
		}
		ind, ok := analyzeForwardInduction(instr, li)
		if !ok || ind.step <= 0 || ind.init.min < 0 {
			continue
		}
		if key.ID == instr.ID || (ind.update != nil && key.ID == ind.update.ID) {
			return true
		}
		if key.Def != nil {
			if step, ok := forwardStepFromPhi(key.Def, instr.ID); ok && step >= 0 {
				return true
			}
		}
	}
	return false
}

// TableArrayBoundsCheckHoistPass is kept as the compatibility entry point for
// older tests and diagnostics. The implementation is now the first loop-region
// versioning stage.
func TableArrayBoundsCheckHoistPass(fn *Function) (*Function, error) {
	return LoopRegionVersioningPass(fn)
}

func cloneTableArrayBoolMap(src map[int]bool) map[int]bool {
	dst := make(map[int]bool, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

type loopRegionTableArrayFact struct {
	table    *Value
	headerID int
	length   *Value
	data     *Value
	kind     int64
}

func collectLoopRegionTableArrayFacts(preheader *Block) []loopRegionTableArrayFact {
	if preheader == nil {
		return nil
	}
	facts := newTableArrayFactSet()
	for _, instr := range preheader.Instrs {
		if instr == nil {
			continue
		}
		facts.RecordByRole(tableArrayFactRole(instr.Op), instr)
	}
	return loopRegionFactsFromTableArrayFacts(facts.CompleteFacts())
}

func collectLoopRegionTableArrayFactsDominating(fn *Function, dom *domInfo, headerID int) []loopRegionTableArrayFact {
	if fn == nil || dom == nil {
		return nil
	}
	facts := newTableArrayFactSet()
	for _, block := range fn.Blocks {
		if block == nil || !dom.dominates(block.ID, headerID) {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			facts.RecordByRole(tableArrayFactRole(instr.Op), instr)
		}
	}
	return loopRegionFactsFromTableArrayFacts(facts.CompleteFacts())
}

func loopRegionFactsFromTableArrayFacts(src []tableArrayCompleteFact) []loopRegionTableArrayFact {
	if len(src) == 0 {
		return nil
	}
	out := make([]loopRegionTableArrayFact, 0, len(src))
	for _, fact := range src {
		out = append(out, loopRegionTableArrayFact{
			table:    fact.table,
			headerID: fact.headerID,
			length:   fact.len,
			data:     fact.data,
			kind:     fact.kind,
		})
	}
	return out
}

func loopRegionStructuralHazard(fn *Function, body map[int]bool) (*Instr, bool) {
	if fn == nil || body == nil {
		return nil, true
	}
	for _, block := range fn.Blocks {
		if !body[block.ID] {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			if opIsTableArrayRegionGlobalBarrier(instr.Op) {
				return instr, true
			}
		}
	}
	return nil, false
}

func loopRegionAliasingHazard(fn *Function, body map[int]bool, fact LoopTableArrayFact, seededGlobals map[string]*vm.FuncProto, spec *SpeculationFacts, tableShapes *TableShapeFacts) *Instr {
	if fn == nil || body == nil || fact.TableID < 0 {
		return nil
	}
	table := &Value{ID: fact.TableID}
	for _, block := range fn.Blocks {
		if block == nil || !body[block.ID] {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			switch {
			case opIsTableArrayRegionAliasingCall(instr.Op):
				if licmLoopCallMayMutateValueWithGlobals(fn, []*Instr{instr}, table, seededGlobals, spec, tableShapes) {
					return instr
				}
			case opIsTableArrayRegionAliasingAlways(instr.Op):
				return instr
			case opIsTableArrayRegionTableMutation(instr.Op):
				if len(instr.Args) >= 1 && instr.Args[0] != nil && instr.Args[0].ID == fact.TableID {
					return instr
				}
			}
		}
	}
	return nil
}

func loopRegionAccessFact(headerID, preheaderID int, instr *Instr, facts []loopRegionTableArrayFact, key, length *Value) (LoopTableArrayFact, bool) {
	if instr == nil || key == nil || length == nil {
		return LoopTableArrayFact{}, false
	}
	layout, ok := tableArrayAccessLayoutForOp(instr.Op)
	if !ok || !loopRegionInstrHasAccessArgs(instr, layout) {
		return LoopTableArrayFact{}, false
	}
	if instr.Args[layout.LenArg].ID != length.ID || instr.Args[layout.KeyArg].ID != key.ID {
		return LoopTableArrayFact{}, false
	}
	for _, fact := range facts {
		if loopRegionAccessMatchesFact(instr, layout, fact) {
			return makeLoopTableArrayFact(headerID, preheaderID, instr, fact, key), true
		}
	}
	return LoopTableArrayFact{}, false
}

func loopRegionAccessFactWithGuardedArrayLen(headerID, preheaderID int, instr *Instr, facts []loopRegionTableArrayFact, key *Value) (LoopTableArrayFact, bool) {
	if instr == nil || key == nil {
		return LoopTableArrayFact{}, false
	}
	layout, ok := tableArrayAccessLayoutForOp(instr.Op)
	if !ok || !loopRegionInstrHasAccessArgs(instr, layout) || instr.Args[layout.KeyArg].ID != key.ID {
		return LoopTableArrayFact{}, false
	}
	for _, fact := range facts {
		if loopRegionAccessMatchesFact(instr, layout, fact) {
			return makeLoopTableArrayFact(headerID, preheaderID, instr, fact, key), true
		}
	}
	return LoopTableArrayFact{}, false
}

func loopRegionInstrHasAccessArgs(instr *Instr, layout tableArrayAccessLayout) bool {
	if instr == nil || layout.DataArg < 0 || layout.LenArg < 0 || layout.KeyArg < 0 {
		return false
	}
	maxArg := layout.KeyArg
	if layout.DataArg > maxArg {
		maxArg = layout.DataArg
	}
	if layout.LenArg > maxArg {
		maxArg = layout.LenArg
	}
	if layout.TableArg > maxArg {
		maxArg = layout.TableArg
	}
	if len(instr.Args) <= maxArg {
		return false
	}
	for _, arg := range []int{layout.DataArg, layout.LenArg, layout.KeyArg} {
		if instr.Args[arg] == nil {
			return false
		}
	}
	return layout.TableArg < 0 || instr.Args[layout.TableArg] != nil
}

func loopRegionAccessMatchesFact(instr *Instr, layout tableArrayAccessLayout, fact loopRegionTableArrayFact) bool {
	if instr == nil || fact.length == nil || fact.data == nil || fact.kind != instr.Aux {
		return false
	}
	if instr.Args[layout.DataArg].ID != fact.data.ID || instr.Args[layout.LenArg].ID != fact.length.ID {
		return false
	}
	return layout.TableArg < 0 || (fact.table != nil && instr.Args[layout.TableArg].ID == fact.table.ID)
}

func insertedLoopLimitArrayLenGuard(fn *Function, preheader *Block, body map[int]bool, loopKey, limit *Value, instr *Instr, facts []loopRegionTableArrayFact, seen map[[2]int]bool) bool {
	if fn == nil || preheader == nil || loopKey == nil || limit == nil || instr == nil || !loopRegionValueInvariant(body, limit) {
		return false
	}
	if !loopRegionInstrUsesKey(instr, loopKey) {
		return false
	}
	for _, fact := range facts {
		if fact.length == nil || !loopRegionInstrUsesFact(instr, fact) {
			continue
		}
		key := [2]int{limit.ID, fact.length.ID}
		if seen[key] {
			return true
		}
		lt := &Instr{
			ID:    fn.newValueID(),
			Op:    OpLtInt,
			Type:  TypeBool,
			Args:  []*Value{limit, fact.length},
			Block: preheader,
		}
		guard := &Instr{
			ID:    fn.newValueID(),
			Op:    OpGuardTruthy,
			Type:  TypeBool,
			Args:  []*Value{lt.Value()},
			Block: preheader,
		}
		insertBeforeTerminator(preheader, lt)
		insertBeforeTerminator(preheader, guard)
		seen[key] = true
		return true
	}
	return false
}

func loopRegionInstrUsesKey(instr *Instr, key *Value) bool {
	if instr == nil || key == nil {
		return false
	}
	layout, ok := tableArrayAccessLayoutForOp(instr.Op)
	if !ok || !loopRegionInstrHasAccessArgs(instr, layout) {
		return false
	}
	return instr.Args[layout.KeyArg].ID == key.ID
}

func loopRegionValueInvariant(body map[int]bool, v *Value) bool {
	if v == nil || v.Def == nil || v.Def.Block == nil {
		return true
	}
	return !body[v.Def.Block.ID]
}

func loopRegionInstrUsesFact(instr *Instr, fact loopRegionTableArrayFact) bool {
	if instr == nil || fact.length == nil || fact.data == nil {
		return false
	}
	layout, ok := tableArrayAccessLayoutForOp(instr.Op)
	if !ok || !loopRegionInstrHasAccessArgs(instr, layout) {
		return false
	}
	return instr.Args[layout.DataArg].ID == fact.data.ID && instr.Args[layout.LenArg].ID == fact.length.ID
}

func makeLoopTableArrayFact(headerID, preheaderID int, instr *Instr, fact loopRegionTableArrayFact, key *Value) LoopTableArrayFact {
	tableID, tableHeaderID, lenID, dataID, keyID := -1, -1, -1, -1, -1
	if fact.table != nil {
		tableID = fact.table.ID
	}
	tableHeaderID = fact.headerID
	if fact.length != nil {
		lenID = fact.length.ID
	}
	if fact.data != nil {
		dataID = fact.data.ID
	}
	if key != nil {
		keyID = key.ID
	}
	return LoopTableArrayFact{
		HeaderBlockID:    headerID,
		PreheaderBlockID: preheaderID,
		TableID:          tableID,
		TableHeaderID:    tableHeaderID,
		LenID:            lenID,
		DataID:           dataID,
		KeyID:            keyID,
		Kind:             fact.kind,
		AccessOp:         instr.Op,
	}
}

func tableArrayLoopUpperGuard(li *loopInfo, header *Block) (*Instr, *Block) {
	if header == nil || len(header.Instrs) == 0 || len(header.Succs) < 2 {
		return nil, nil
	}
	term := header.Instrs[len(header.Instrs)-1]
	if term.Op != OpBranch || len(term.Args) == 0 || term.Args[0] == nil || term.Args[0].Def == nil {
		return nil, nil
	}
	cond := term.Args[0].Def
	if _, ok := orderedRangeRefineKind(cond.Op); !ok || len(cond.Args) < 2 {
		return nil, nil
	}
	body := li.headerBlocks[header.ID]
	if body == nil {
		return nil, nil
	}
	trueSucc, falseSucc := header.Succs[0], header.Succs[1]
	if body[trueSucc.ID] && !body[falseSucc.ID] {
		return cond, trueSucc
	}
	if !body[trueSucc.ID] && body[falseSucc.ID] {
		return nil, nil
	}
	return nil, nil
}

func loopMayMutateTablesOrCall(fn *Function, body map[int]bool) bool {
	_, ok := loopRegionStructuralHazard(fn, body)
	return ok
}
