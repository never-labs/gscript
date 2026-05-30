package methodjit

import "github.com/Never-Labs/gscript/internal/vm"

const boolFillFlagNoStrideOverflow int64 = 1

// BoolTableFillLoopPass replaces a narrow counted loop that only writes a
// constant bool range into a local table with one bulk fill op. The rewrite is
// intentionally conservative: the loop-carried values must not escape the
// removed loop, the body must contain no side effects except the bool store,
// and any typed store feedback must agree with the constant bool value.
// Dynamic stride loops are admitted only when range analysis proves a positive
// stride; codegen still guards kind and bounds and falls back through RawSetInt.
func BoolTableFillLoopPass(fn *Function) (*Function, error) {
	if fn != nil {
		fn.ensureAnalysis()
	}
	allowed := allowedDomainsForModule(
		analysisFacts(AnalysisFactFixedShapeTables, AnalysisFactIntRanges),
		nil,
		nil,
	)
	return BoolTableFillLoopPassCtx(newPassContext(fn, nil, allowed, false))
}

func BoolTableFillLoopPassCtx(ctx *PassContext) (*Function, error) {
	return boolTableFillLoopPass(ctx.Func(), ctx.Numeric())
}

func boolTableFillLoopPass(fn *Function, numeric *NumericFacts) (*Function, error) {
	if fn == nil {
		return fn, nil
	}
	fn.ensureAnalysis()
	for _, header := range append([]*Block(nil), fn.Blocks...) {
		cand, ok := detectBoolTableFillLoop(fn, header, numeric)
		if !ok || !boolFillLoopDefsAreLocal(fn, cand) {
			continue
		}
		fillID := applyBoolTableFillLoop(fn, cand, numeric)
		functionRemarks(fn).Add("BoolTableFillLoop", "changed", cand.preheader.ID, fillID, OpTableBoolArrayFill,
			"replaced const-bool table store loop with bulk bool-array fill")
	}
	return fn, nil
}

type boolFillLoopCandidate struct {
	preheader *Block
	header    *Block
	body      *Block
	exit      *Block
	table     *Value
	end       *Value
	start     *Value
	step      *Value
	byteVal   int64
}

func detectBoolTableFillLoop(fn *Function, header *Block, numeric *NumericFacts) (boolFillLoopCandidate, bool) {
	var zero boolFillLoopCandidate
	if header == nil || len(header.Preds) != 2 || len(header.Succs) != 2 || len(header.Instrs) == 0 {
		return zero, false
	}
	term := header.Instrs[len(header.Instrs)-1]
	if term == nil || term.Op != OpBranch || len(term.Args) != 1 {
		return zero, false
	}
	body, exit := header.Succs[0], header.Succs[1]
	if body == nil || exit == nil || len(body.Preds) != 1 || body.Preds[0] != header || len(body.Succs) != 1 || body.Succs[0] != header {
		return zero, false
	}
	bodyPredIdx := -1
	for i, pred := range header.Preds {
		if pred == body {
			bodyPredIdx = i
			break
		}
	}
	if bodyPredIdx < 0 {
		return zero, false
	}
	preheaderIdx := 1 - bodyPredIdx
	preheader := header.Preds[preheaderIdx]
	if preheader == nil {
		return zero, false
	}

	store := singleBoolFillBodyStore(body)
	if store == nil {
		return zero, false
	}
	table, key, val, kind, ok := boolFillStoreParts(store)
	if !ok || (kind != 0 && kind != int64(vm.FBKindBool)) {
		return zero, false
	}
	if table == nil || key == nil || val == nil || val.Def == nil || val.Def.Op != OpConstBool {
		return zero, false
	}
	if table.Def == nil || table.Def.Op != OpNewTable {
		return zero, false
	}

	cond := term.Args[0]
	if cond == nil || cond.Def == nil || cond.Def.Op != OpLeInt || len(cond.Def.Args) < 2 {
		return zero, false
	}
	if cond.Def.Args[0] == nil || cond.Def.Args[1] == nil {
		return zero, false
	}
	start, step, ok := boolFillLoopStartAndStep(fn, header, bodyPredIdx, preheaderIdx, key, cond.Def.Args[0], numeric)
	if !ok || start == nil || step == nil {
		return zero, false
	}
	byteVal := int64(1)
	if val.Def.Aux != 0 {
		byteVal = 2
	}
	return boolFillLoopCandidate{
		preheader: preheader,
		header:    header,
		body:      body,
		exit:      exit,
		table:     table,
		end:       cond.Def.Args[1],
		start:     start,
		step:      step,
		byteVal:   byteVal,
	}, true
}

func singleBoolFillBodyStore(body *Block) *Instr {
	var store *Instr
	for _, instr := range body.Instrs {
		if instr == nil {
			continue
		}
		if opIsBoolTableFillBodyBenign(instr.Op) {
			continue
		}
		if opIsBoolTableFillStore(instr.Op) {
			if store != nil {
				return nil
			}
			store = instr
			continue
		}
		return nil
	}
	return store
}

func boolFillStoreParts(store *Instr) (table, key, val *Value, kind int64, ok bool) {
	if store == nil {
		return nil, nil, nil, 0, false
	}
	layout, ok := boolTableFillStoreLayoutForOp(store.Op)
	if !ok || len(store.Args) <= layout.TableArg || len(store.Args) <= layout.KeyArg || len(store.Args) <= layout.ValueArg {
		return nil, nil, nil, 0, false
	}
	switch layout.KindSource {
	case OpBoolTableFillKindAux:
		kind = store.Aux
	case OpBoolTableFillKindAux2:
		kind = store.Aux2
	default:
		return nil, nil, nil, 0, false
	}
	return store.Args[layout.TableArg], store.Args[layout.KeyArg], store.Args[layout.ValueArg], kind, true
}

func boolFillLoopStartAndStep(fn *Function, header *Block, bodyPredIdx, preheaderIdx int, key, condKey *Value, numeric *NumericFacts) (*Value, *Value, bool) {
	if key == nil || condKey == nil || key.Def == nil {
		return nil, nil, false
	}

	// Graph-builder FORLOOP shape:
	//   phi = Phi(init-step, key)
	//   key = phi + step
	//   if key <= end { store[key] ... }
	if condKey.ID == key.ID && key.Def.Op == OpAddInt {
		add := key.Def
		if add == nil || add.Op != OpAddInt || len(add.Args) < 2 {
			return nil, nil, false
		}
		phi, step, ok := boolFillLoopPhiAndStep(add)
		if !ok || phi.Block != header || len(phi.Args) != len(header.Preds) {
			return nil, nil, false
		}
		if step.Def == nil || step.Def.Op != OpConstInt || !boolFillPositiveStep(step, numeric) {
			return nil, nil, false
		}
		if phi.Args[bodyPredIdx] == nil || phi.Args[bodyPredIdx].ID != key.ID {
			return nil, nil, false
		}
		init := phi.Args[preheaderIdx]
		if init == nil || init.Def == nil || init.Def.Op != OpConstInt {
			return nil, nil, false
		}
		start := &Instr{Op: OpConstInt, Type: TypeInt, Aux: init.Def.Aux + step.Def.Aux}
		return start.Value(), step, true
	}

	// While-style shape:
	//   phi = Phi(init, phi + step)
	//   if phi <= end { store[phi]; ... }
	if condKey.ID != key.ID || key.Def.Op != OpPhi || key.Def.Block != header || len(key.Def.Args) != len(header.Preds) {
		return nil, nil, false
	}
	update := key.Def.Args[bodyPredIdx]
	if update == nil || update.Def == nil || update.Def.Block == nil {
		return nil, nil, false
	}
	step, ok := boolFillLoopUpdateStep(update.Def, key.ID)
	if !ok || !boolFillPositiveStep(step, numeric) {
		return nil, nil, false
	}
	init := key.Def.Args[preheaderIdx]
	if init == nil {
		return nil, nil, false
	}
	return init, step, true
}

func boolFillLoopPhiAndStep(add *Instr) (*Instr, *Value, bool) {
	a, b := add.Args[0], add.Args[1]
	if a != nil && a.Def != nil && a.Def.Op == OpPhi {
		return a.Def, b, b != nil
	}
	if b != nil && b.Def != nil && b.Def.Op == OpPhi {
		return b.Def, a, a != nil
	}
	return nil, nil, false
}

func boolFillLoopUpdateStep(instr *Instr, phiID int) (*Value, bool) {
	if instr == nil || instr.Op != OpAddInt || len(instr.Args) < 2 {
		return nil, false
	}
	a, b := instr.Args[0], instr.Args[1]
	if a != nil && a.ID == phiID && b != nil {
		return b, true
	}
	if b != nil && b.ID == phiID && a != nil {
		return a, true
	}
	return nil, false
}

func boolFillPositiveStep(step *Value, numeric *NumericFacts) bool {
	if step == nil || step.Def == nil {
		return false
	}
	if step.Def.Op == OpConstInt {
		return step.Def.Aux > 0
	}
	if step.Def.Type != TypeInt {
		return false
	}
	if numeric == nil {
		return false
	}
	r, ok := numeric.IntRange(step.ID)
	return ok && r.known && r.min > 0
}

func boolFillLoopDefsAreLocal(fn *Function, cand boolFillLoopCandidate) bool {
	removed := map[int]bool{cand.header.ID: true, cand.body.ID: true}
	defs := make(map[int]bool)
	for _, block := range []*Block{cand.header, cand.body} {
		for _, instr := range block.Instrs {
			if instr != nil && !instr.Op.IsTerminator() {
				defs[instr.ID] = true
			}
		}
	}
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			for _, arg := range instr.Args {
				if arg == nil || !defs[arg.ID] || removed[block.ID] {
					continue
				}
				return false
			}
		}
	}
	return true
}

func applyBoolTableFillLoop(fn *Function, cand boolFillLoopCandidate, numeric *NumericFacts) int {
	start := cand.start
	var inserted []*Instr
	if start != nil && start.Def != nil && start.Def.ID == 0 && start.Def.Block == nil && start.Def.Op == OpConstInt {
		startInstr := &Instr{
			ID:    fn.nextID,
			Op:    OpConstInt,
			Type:  TypeInt,
			Aux:   start.Def.Aux,
			Block: cand.preheader,
		}
		fn.nextID++
		start = startInstr.Value()
		inserted = append(inserted, startInstr)
	}
	fill := &Instr{
		ID:    fn.nextID,
		Op:    OpTableBoolArrayFill,
		Type:  TypeUnknown,
		Args:  []*Value{cand.table, start, cand.end},
		Aux:   cand.byteVal,
		Block: cand.preheader,
	}
	fn.nextID++
	if !boolFillStepIsOne(cand.step) {
		fill.Args = append(fill.Args, cand.step)
		if boolFillStrideNoOverflow(cand, numeric) {
			fill.Aux2 |= boolFillFlagNoStrideOverflow
		}
	}

	insertAt := len(cand.preheader.Instrs)
	if insertAt > 0 && cand.preheader.Instrs[insertAt-1].Op.IsTerminator() {
		insertAt--
	}
	inserted = append(inserted, fill)
	cand.preheader.Instrs = append(cand.preheader.Instrs[:insertAt], append(inserted, cand.preheader.Instrs[insertAt:]...)...)
	cand.preheader.Succs = []*Block{cand.exit}

	for i, pred := range cand.exit.Preds {
		if pred == cand.header {
			cand.exit.Preds[i] = cand.preheader
		}
	}

	filtered := fn.Blocks[:0]
	for _, block := range fn.Blocks {
		if block == cand.header || block == cand.body {
			continue
		}
		filtered = append(filtered, block)
	}
	fn.Blocks = filtered
	return fill.ID
}

func boolFillStepIsOne(step *Value) bool {
	return step != nil && step.Def != nil && step.Def.Op == OpConstInt && step.Def.Aux == 1
}

func boolFillStrideNoOverflow(cand boolFillLoopCandidate, numeric *NumericFacts) bool {
	if cand.step == nil || cand.end == nil {
		return false
	}
	stepRange := boolFillValueRange(cand.step, numeric)
	endRange := boolFillValueRange(cand.end, numeric)
	return stepRange.known && stepRange.min > 0 && stepRange.max <= MaxInt48 &&
		endRange.known && endRange.max <= MaxInt48
}

func boolFillValueRange(v *Value, numeric *NumericFacts) intRange {
	if v == nil || v.Def == nil {
		return topRange()
	}
	if v.Def.Op == OpConstInt {
		return pointRange(v.Def.Aux)
	}
	if numeric != nil {
		if r, ok := numeric.IntRange(v.ID); ok {
			return r
		}
	}
	return topRange()
}
