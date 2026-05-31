//go:build darwin && arm64

package methodjit

import (
	"fmt"
	"sort"
	"unsafe"

	"github.com/never-labs/gscript/internal/jit"
)

func (ec *emitContext) initTier2BlockCounters() {
	if ec == nil || ec.fn == nil {
		return
	}
	ec.tier2BlockCounterIndex = make(map[int]int, len(ec.fn.Blocks))
	ec.tier2BlockCounterMeta = make([]Tier2BlockCounterMeta, 0, len(ec.fn.Blocks))
	protoName := ""
	if ec.fn.Proto != nil {
		protoName = ec.fn.Proto.Name
	}
	for _, block := range ec.fn.Blocks {
		if block == nil {
			continue
		}
		idx := len(ec.tier2BlockCounterMeta)
		ec.tier2BlockCounterIndex[block.ID] = idx
		meta := Tier2BlockCounterMeta{
			Proto:   protoName,
			BlockID: block.ID,
		}
		for _, instr := range block.Instrs {
			if instr == nil {
				continue
			}
			meta.InstrIDs = append(meta.InstrIDs, instr.ID)
			meta.Ops = append(meta.Ops, instr.Op.String())
		}
		ec.tier2BlockCounterMeta = append(ec.tier2BlockCounterMeta, meta)
	}
	if len(ec.tier2BlockCounterMeta) > 0 {
		ec.tier2BlockCounters = make([]uint64, len(ec.tier2BlockCounterMeta))
	}
}

func (ec *emitContext) emitTier2BlockCounter(block *Block) {
	if ec == nil || block == nil || len(ec.tier2BlockCounterIndex) == 0 {
		return
	}
	idx, ok := ec.tier2BlockCounterIndex[block.ID]
	if !ok {
		return
	}
	if len(ec.tier2BlockCounters) == 0 {
		return
	}
	base := uintptr(unsafe.Pointer(&ec.tier2BlockCounters[0]))
	ec.asm.LoadImm64(jit.X16, int64(base))
	offset := idx * 8
	if offset <= 32760 {
		ec.asm.LDR(jit.X17, jit.X16, offset)
		ec.asm.ADDimm(jit.X17, jit.X17, 1)
		ec.asm.STR(jit.X17, jit.X16, offset)
	} else {
		ec.asm.LoadImm64(jit.X17, int64(offset))
		ec.asm.ADDreg(jit.X16, jit.X16, jit.X17)
		ec.asm.LDR(jit.X17, jit.X16, 0)
		ec.asm.ADDimm(jit.X17, jit.X17, 1)
		ec.asm.STR(jit.X17, jit.X16, 0)
	}
}

func (ec *emitContext) initTier2CallCounters() {
	if ec == nil || ec.fn == nil {
		return
	}
	protoName := ""
	if ec.fn.Proto != nil {
		protoName = ec.fn.Proto.Name
	}
	for _, block := range ec.fn.Blocks {
		if block == nil {
			continue
		}
		for _, instr := range block.Instrs {
			if instr == nil || instr.Op != OpFieldCallFloor {
				continue
			}
			for _, outcome := range []string{"success", "fallback", "exit"} {
				ec.tier2CallCounterMeta = append(ec.tier2CallCounterMeta, Tier2CallCounterMeta{
					Proto:   protoName,
					InstrID: instr.ID,
					Op:      instr.Op.String(),
					Kind:    "field_call_floor",
					Outcome: outcome,
				})
			}
		}
	}
	if len(ec.tier2CallCounterMeta) > 0 {
		ec.tier2CallCounters = make([]uint64, len(ec.tier2CallCounterMeta))
	}
}

func (ec *emitContext) emitTier2CallCounter(instr *Instr, kind, outcome string) {
	if ec == nil || instr == nil || len(ec.tier2CallCounters) == 0 {
		return
	}
	idx := -1
	for i, meta := range ec.tier2CallCounterMeta {
		if meta.InstrID == instr.ID && meta.Kind == kind && meta.Outcome == outcome {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	base := uintptr(unsafe.Pointer(&ec.tier2CallCounters[0]))
	ec.asm.LoadImm64(jit.X16, int64(base))
	offset := idx * 8
	if offset <= 32760 {
		ec.asm.LDR(jit.X17, jit.X16, offset)
		ec.asm.ADDimm(jit.X17, jit.X17, 1)
		ec.asm.STR(jit.X17, jit.X16, offset)
	} else {
		ec.asm.LoadImm64(jit.X17, int64(offset))
		ec.asm.ADDreg(jit.X16, jit.X16, jit.X17)
		ec.asm.LDR(jit.X17, jit.X16, 0)
		ec.asm.ADDimm(jit.X17, jit.X17, 1)
		ec.asm.STR(jit.X17, jit.X16, 0)
	}
}

// emitNumericBody emits a second Tier 2 body under numericMode=true.
// The numeric entry label receives raw int args in X0..X(N-1), builds a thin
// FP/LR frame, and jumps to the pass-2 entry block. Raw callers pass the callee
// VM register base directly in the pinned mRegRegs register and spill/reload
// their own live allocated registers around the BL/BLR, so this entry does not
// save the full callee-saved register set used by the boxed public ABI.
func (ec *emitContext) emitNumericBody() {
	if ec.numericParamCount <= 0 {
		return
	}
	if ec.fn == nil || ec.fn.Proto == nil {
		return
	}

	asm := ec.asm
	prevNumericMode := ec.numericMode
	prevActiveRegs := ec.activeRegs
	prevReprs := ec.snapshotValueReprs()
	prevActiveFPRegs := ec.activeFPRegs
	prevShapeVerified := ec.shapeVerified
	prevTableVerified := ec.tableVerified
	prevKindVerified := ec.kindVerified
	prevKeysDirtyWritten := ec.keysDirtyWritten
	prevStringLookupCleanGuarded := ec.stringLookupCleanGuarded
	prevTableArrayBoundedKeys := ec.tableArrayBoundedKeys
	prevDMVerified := ec.dmVerified
	prevFieldSvalsCacheValid := ec.fieldSvalsCacheValid
	prevFieldSvalsCacheTableID := ec.fieldSvalsCacheTableID
	prevFieldSvalsCacheShapeID := ec.fieldSvalsCacheShapeID
	ec.numericMode = true
	entryLabel, ok := ec.entryBlockLabelOK()
	if !ok {
		ec.numericMode = prevNumericMode
		return
	}

	label := fmt.Sprintf("t2_numeric_self_entry_%d", ec.numericParamCount)
	asm.Label(label)
	asm.SUBimm(jit.SP, jit.SP, uint16(numericSelfEntryFrameSize))
	asm.STP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.X29, jit.SP, 0)
	asm.B(entryLabel)

	ec.activeRegs = make(map[int]bool)
	ec.resetValueReprs()
	ec.activeFPRegs = make(map[int]bool)
	ec.clearScratchFPRCache()
	ec.tableArrayBoundedKeys = make(map[tableArrayBoundKey]bool)
	ec.shapeVerified = make(map[int]uint32)
	ec.tableVerified = make(map[int]bool)
	ec.kindVerified = make(map[int]uint16)
	ec.keysDirtyWritten = make(map[int]bool)
	ec.stringLookupCleanGuarded = make(map[int]bool)
	ec.dmVerified = make(map[int]bool)
	ec.invalidateFieldSvalsCache()
	for _, block := range ec.fn.Blocks {
		ec.emitBlock(block)
	}
	ec.numericMode = prevNumericMode
	ec.activeRegs = prevActiveRegs
	ec.restoreValueReprSnapshot(prevReprs)
	ec.activeFPRegs = prevActiveFPRegs
	ec.shapeVerified = prevShapeVerified
	ec.tableVerified = prevTableVerified
	ec.kindVerified = prevKindVerified
	ec.keysDirtyWritten = prevKeysDirtyWritten
	ec.stringLookupCleanGuarded = prevStringLookupCleanGuarded
	ec.tableArrayBoundedKeys = prevTableArrayBoundedKeys
	ec.dmVerified = prevDMVerified
	ec.fieldSvalsCacheValid = prevFieldSvalsCacheValid
	ec.fieldSvalsCacheTableID = prevFieldSvalsCacheTableID
	ec.fieldSvalsCacheShapeID = prevFieldSvalsCacheShapeID
}

// blockLabelFor returns the label for block b in the given emit pass.
// When ec.numericMode is true, returns the prefixed variant.
func (ec *emitContext) blockLabelFor(b *Block) string {
	if ec.numericMode {
		return fmt.Sprintf("num_B%d", b.ID)
	}
	return blockLabel(b)
}

// passLabel (R128 label refactor) wraps a fixed label name with the
// current pass's suffix. Normal pass → unchanged; numeric pass →
// "_num" suffix. Used to disambiguate pass-1 vs pass-2 labels that
// would otherwise collide (call_continue_N, global_continue_N,
// op_continue_N, table_continue_N, call_resume_N).
func (ec *emitContext) passLabel(base string) string {
	if ec.numericMode {
		return base + "_num"
	}
	return base
}

// callExitResumeLabel returns the resume-label name for an instrID
// in the current pass. Free function version kept for backward compat
// in emitDeferredResumes which needs to re-derive the label per entry.
func callExitResumeLabelForPass(instrID int, numericMode bool) string {
	s := fmt.Sprintf("call_resume_%d", instrID)
	if numericMode {
		s += "_num"
	}
	return s
}

func (ec *emitContext) seedEntryShapeGuardState(block *Block) {
	if !ec.hasEntryShapeGuards() || ec.fn == nil || block == nil || block != ec.fn.Entry {
		return
	}
	if len(block.Preds) != 0 {
		return
	}
	for _, instr := range block.Instrs {
		if instr.Op != OpLoadSlot {
			continue
		}
		fact, ok := ec.entryShapeGuards[int(instr.Aux)]
		if !ok || fact.ShapeID == 0 {
			continue
		}
		ec.shapeVerified[instr.ID] = fact.ShapeID
	}
}

func (ec *emitContext) seedBranchShapeGuardState(block *Block) {
	if ec == nil || block == nil || len(block.Preds) != 1 {
		return
	}
	pred := block.Preds[0]
	if pred == nil || len(pred.Succs) == 0 || pred.Succs[0] != block || len(pred.Instrs) == 0 {
		return
	}
	br := pred.Instrs[len(pred.Instrs)-1]
	if br == nil || br.Op != OpBranch || len(br.Args) == 0 || br.Args[0] == nil || br.Args[0].Def == nil {
		return
	}
	tableID, shapeID, ok := branchTableShapeEqConst(br.Args[0].Def)
	if !ok || shapeID == 0 {
		return
	}
	if ec.shapeVerified == nil {
		ec.shapeVerified = make(map[int]uint32)
	}
	if ec.tableVerified == nil {
		ec.tableVerified = make(map[int]bool)
	}
	ec.shapeVerified[tableID] = shapeID
	ec.tableVerified[tableID] = true
}

// emitBlock emits ARM64 code for one basic block.
func (ec *emitContext) emitBlock(block *Block) {
	ec.asm.Label(ec.blockLabelFor(block))
	ec.currentBlockID = block.ID
	typedParamLoads := ec.typedSelfEntryParamLoads(block)
	typedParamLabelEmitted := false
	blockCounterEmitted := false
	if typedParamLoads != nil && len(typedParamLoads) == 0 {
		ec.asm.Label(ec.typedSelfAfterParamsLabel())
		typedParamLabelEmitted = true
	}
	if typedParamLoads == nil || typedParamLabelEmitted {
		ec.emitTier2BlockCounter(block)
		blockCounterEmitted = true
	}

	isLoopBlock := ec.loop != nil && ec.loop.loopBlocks[block.ID]
	isHeader := ec.loop != nil && ec.loop.loopHeaders[block.ID]

	// Reset active register set for this block.
	ec.activeRegs = make(map[int]bool)
	ec.resetValueReprs()
	ec.activeFPRegs = make(map[int]bool)
	ec.clearScratchFPRCache()
	// Seed shape/table verification from the sole predecessor's outgoing state.
	// Only safe when the block has exactly one predecessor — at merge points
	// (multiple preds), different paths may have different table mutations,
	// so we conservatively reset. Loop headers also reset (back-edge may
	// have mutated tables). Single-pred propagation captures the main win:
	// pre-header → body and sequential blocks within a loop.
	// R100: restrict multi-pred merge (R95) to single-pred only — the
	// multi-pred merge showed no measurable benefit and may have
	// contributed to the sort regression (though that's unconfirmed).
	if !isHeader && len(block.Preds) == 1 {
		predID := block.Preds[0].ID
		// Seed from the single predecessor's out-state.
		if m, ok := ec.blockOutShapes[predID]; ok {
			ec.shapeVerified = make(map[int]uint32, len(m))
			for k, v := range m {
				ec.shapeVerified[k] = v
			}
		} else {
			ec.shapeVerified = make(map[int]uint32)
		}
		if m, ok := ec.blockOutTables[predID]; ok {
			ec.tableVerified = make(map[int]bool, len(m))
			for k, v := range m {
				ec.tableVerified[k] = v
			}
		} else {
			ec.tableVerified = make(map[int]bool)
		}
		if m, ok := ec.blockOutKinds[predID]; ok {
			ec.kindVerified = make(map[int]uint16, len(m))
			for k, v := range m {
				ec.kindVerified[k] = v
			}
		} else {
			ec.kindVerified = make(map[int]uint16)
		}
		if m, ok := ec.blockOutKeysDirty[predID]; ok {
			ec.keysDirtyWritten = make(map[int]bool, len(m))
			for k, v := range m {
				ec.keysDirtyWritten[k] = v
			}
		} else {
			ec.keysDirtyWritten = make(map[int]bool)
		}
		ec.stringLookupCleanGuarded = make(map[int]bool)
	} else {
		ec.shapeVerified = make(map[int]uint32)
		ec.tableVerified = make(map[int]bool)
		ec.kindVerified = make(map[int]uint16)
		ec.keysDirtyWritten = make(map[int]bool)
		ec.stringLookupCleanGuarded = make(map[int]bool)
	}
	ec.seedBranchShapeGuardState(block)
	ec.tableArrayBoundedKeys = make(map[tableArrayBoundKey]bool)
	ec.seedEntryShapeGuardState(block)
	// R44: reset DenseMatrix verification at every block boundary. The hot
	// matrix inner loops this targets keep the relevant body in one block, and
	// cross-block propagation complicates merge semantics; conservatively reset.
	ec.dmVerified = make(map[int]bool)
	ec.invalidateFieldSvalsCache()

	if isLoopBlock && !isHeader && ec.safeHeaderRegs != nil {
		ec.activateDirectLoopHeaderGPRs(block)
		// Non-header loop block: activate SAFE registers from the innermost
		// enclosing loop header. Only registers that are NOT clobbered by
		// any non-header block in the loop body are activated. This prevents
		// stale register assumptions in nested or complex loop bodies.
		if innerHeader, ok := ec.loop.blockInnerHeader[block.ID]; ok {
			if hdrRegs, ok := ec.safeHeaderRegs[innerHeader]; ok {
				for _, entry := range hdrRegs {
					ec.activeRegs[entry.ValueID] = true
					if entry.IsRawInt {
						ec.setValueRepr(entry.ValueID, valueReprRawInt)
					}
					if entry.IsRawTablePtr {
						ec.setValueRepr(entry.ValueID, valueReprRawTablePtr)
					}
					if entry.IsRawDataPtr {
						ec.setValueRepr(entry.ValueID, valueReprRawDataPtr)
					}
					if entry.IsRawSvalsPtr {
						ec.setValueRepr(entry.ValueID, valueReprRawFieldSvalsPtr)
					}
				}
			}
		}
	}
	if isLoopBlock && ec.safeHeaderFPRegs != nil {
		// Activate every safe enclosing loop-header FPR value whose register
		// allocation is region-pinned across this block. This extends the old
		// innermost-only model to nested numeric regions without assuming a
		// global register allocator.
		ec.activateLoopHeaderFPRs(block.ID)
	}
	if !isHeader && len(block.Preds) == 1 {
		ec.seedSinglePredRawIntRegs(block)
		ec.seedSinglePredRawFloatRegs(block)
	}
	if ec.rawIntBlockCarry && !isHeader && len(block.Preds) > 1 {
		ec.seedMultiPredRawIntRegs(block)
		ec.seedMultiPredRawFloatRegs(block)
	}
	if !isHeader && len(block.Preds) == 1 {
		ec.seedSinglePredTableArrayKeyRegs(block)
	}
	if isLoopBlock && ec.loopInvariantGPRs != nil {
		ec.activateLoopInvariantGPRs(block.ID)
	}
	if isLoopBlock && ec.loopInvariantFPRs != nil {
		ec.activateLoopInvariantFPRs(block.ID)
	}

	// Phi values are active at block entry (their registers were loaded
	// by emitPhiMoves from the predecessor). When a phi's register
	// conflicts with a loopHeaderRegs value, invalidate the header value.
	for _, instr := range block.Instrs {
		if instr.Op != OpPhi {
			break
		}
		if pr, ok := ec.alloc.ValueRegs[instr.ID]; ok {
			if pr.IsFloat {
				// FPR phi: activated by emitPhiMoves which delivers raw float.
				ec.invalidateFPReg(pr.Reg, instr.ID)
				ec.activeFPRegs[instr.ID] = true
				ec.setValueRepr(instr.ID, valueReprRawFloat)
			} else {
				// Invalidate any header reg value that shares this register.
				ec.invalidateReg(pr.Reg, instr.ID)
				ec.activeRegs[instr.ID] = true
				// Loop header phis: mark int-typed phis as raw int.
				// emitPhiMoves delivers raw ints to loop header phis from
				// both the initial entry (unboxing) and back-edge (raw transfer).
				if isHeader && instr.Type == TypeInt {
					ec.setValueRepr(instr.ID, valueReprRawInt)
				}
			}
		}
	}

	for _, instr := range block.Instrs {
		ec.emitInstr(instr, block)
		ec.deactivateDeadAfter(instr)
		if typedParamLoads != nil && !typedParamLabelEmitted && instr.Op == OpLoadSlot {
			delete(typedParamLoads, int(instr.Aux))
			if len(typedParamLoads) == 0 {
				ec.asm.Label(ec.typedSelfAfterParamsLabel())
				typedParamLabelEmitted = true
				if !blockCounterEmitted {
					ec.emitTier2BlockCounter(block)
					blockCounterEmitted = true
				}
			}
		}
	}

	// Save outgoing shape/table state for single-predecessor propagation.
	outShapes := make(map[int]uint32, len(ec.shapeVerified))
	for k, v := range ec.shapeVerified {
		outShapes[k] = v
	}
	ec.blockOutShapes[block.ID] = outShapes
	outTables := make(map[int]bool, len(ec.tableVerified))
	for k, v := range ec.tableVerified {
		outTables[k] = v
	}
	ec.blockOutTables[block.ID] = outTables
	outKinds := make(map[int]uint16, len(ec.kindVerified))
	for k, v := range ec.kindVerified {
		outKinds[k] = v
	}
	ec.blockOutKinds[block.ID] = outKinds
	outKD := make(map[int]bool, len(ec.keysDirtyWritten))
	for k, v := range ec.keysDirtyWritten {
		outKD[k] = v
	}
	ec.blockOutKeysDirty[block.ID] = outKD

	outRaw := make(map[int]loopRegEntry)
	for valueID := range ec.activeRegs {
		repr := ec.valueReprOf(valueID)
		if repr != valueReprRawInt && repr != valueReprRawTablePtr && repr != valueReprRawDataPtr && repr != valueReprRawFieldSvalsPtr {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || pr.IsFloat {
			continue
		}
		outRaw[pr.Reg] = loopRegEntry{
			ValueID:       valueID,
			IsRawInt:      repr == valueReprRawInt,
			IsRawTablePtr: repr == valueReprRawTablePtr,
			IsRawDataPtr:  repr == valueReprRawDataPtr,
			IsRawSvalsPtr: repr == valueReprRawFieldSvalsPtr,
		}
	}
	ec.blockOutRawIntRegs[block.ID] = outRaw
	outRawFloat := make(map[int]loopFPRegEntry)
	for valueID := range ec.activeFPRegs {
		if ec.valueReprOf(valueID) != valueReprRawFloat {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || !pr.IsFloat {
			continue
		}
		outRawFloat[pr.Reg] = loopFPRegEntry{ValueID: valueID}
	}
	ec.blockOutRawFloatRegs[block.ID] = outRawFloat
}

func (ec *emitContext) seedSinglePredRawIntRegs(block *Block) {
	if ec == nil || block == nil || len(block.Preds) != 1 {
		return
	}
	predID := block.Preds[0].ID
	predOut := ec.blockOutRawIntRegs[predID]
	if len(predOut) == 0 {
		return
	}
	liveIn := ec.blockLiveIn[block.ID]
	if len(liveIn) == 0 {
		return
	}
	regs := make([]int, 0, len(predOut))
	for reg := range predOut {
		regs = append(regs, reg)
	}
	sort.Ints(regs)
	for _, reg := range regs {
		entry := predOut[reg]
		if (!entry.IsRawInt && !entry.IsRawTablePtr && !entry.IsRawDataPtr && !entry.IsRawSvalsPtr) || !liveIn[entry.ValueID] {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[entry.ValueID]
		if !ok || pr.IsFloat || pr.Reg != reg {
			continue
		}
		ec.invalidateReg(reg, entry.ValueID)
		ec.activeRegs[entry.ValueID] = true
		if entry.IsRawInt {
			ec.setValueRepr(entry.ValueID, valueReprRawInt)
		}
		if entry.IsRawTablePtr {
			ec.setValueRepr(entry.ValueID, valueReprRawTablePtr)
		}
		if entry.IsRawDataPtr {
			ec.setValueRepr(entry.ValueID, valueReprRawDataPtr)
		}
		if entry.IsRawSvalsPtr {
			ec.setValueRepr(entry.ValueID, valueReprRawFieldSvalsPtr)
		}
	}
}

func (ec *emitContext) activateDirectLoopHeaderGPRs(block *Block) {
	if ec == nil || block == nil || len(block.Preds) != 1 || ec.loop == nil || ec.loopHeaderRegs == nil {
		return
	}
	headerID, ok := ec.loop.blockInnerHeader[block.ID]
	if !ok || block.Preds[0] == nil || block.Preds[0].ID != headerID {
		return
	}
	hdrRegs := ec.loopHeaderRegs[headerID]
	if len(hdrRegs) == 0 {
		return
	}
	regs := sortedLoopRegEntryIDs(hdrRegs)
	for _, reg := range regs {
		entry := hdrRegs[reg]
		if entry.ValueID == 0 || !ec.blockLiveIn[block.ID][entry.ValueID] {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[entry.ValueID]
		if !ok || pr.IsFloat || pr.Reg != reg {
			continue
		}
		ec.invalidateReg(reg, entry.ValueID)
		ec.activeRegs[entry.ValueID] = true
		if entry.IsRawInt {
			ec.setValueRepr(entry.ValueID, valueReprRawInt)
		}
		if entry.IsRawTablePtr {
			ec.setValueRepr(entry.ValueID, valueReprRawTablePtr)
		}
		if entry.IsRawDataPtr {
			ec.setValueRepr(entry.ValueID, valueReprRawDataPtr)
		}
		if entry.IsRawSvalsPtr {
			ec.setValueRepr(entry.ValueID, valueReprRawFieldSvalsPtr)
		}
	}
}

func (ec *emitContext) seedMultiPredRawIntRegs(block *Block) {
	if ec == nil || block == nil || len(block.Preds) <= 1 {
		return
	}
	liveIn := ec.blockLiveIn[block.ID]
	if len(liveIn) == 0 {
		return
	}
	firstPred := block.Preds[0]
	if firstPred == nil {
		return
	}
	firstOut := ec.blockOutRawIntRegs[firstPred.ID]
	if len(firstOut) == 0 {
		return
	}
	regs := make([]int, 0, len(firstOut))
	for reg := range firstOut {
		regs = append(regs, reg)
	}
	sort.Ints(regs)
	for _, reg := range regs {
		entry := firstOut[reg]
		if (!entry.IsRawInt && !entry.IsRawTablePtr && !entry.IsRawDataPtr && !entry.IsRawSvalsPtr) || !liveIn[entry.ValueID] {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[entry.ValueID]
		if !ok || pr.IsFloat || pr.Reg != reg {
			continue
		}
		allPreds := true
		for _, pred := range block.Preds[1:] {
			if pred == nil {
				allPreds = false
				break
			}
			predEntry, ok := ec.blockOutRawIntRegs[pred.ID][reg]
			if !ok || predEntry.ValueID != entry.ValueID ||
				predEntry.IsRawInt != entry.IsRawInt ||
				predEntry.IsRawTablePtr != entry.IsRawTablePtr ||
				predEntry.IsRawDataPtr != entry.IsRawDataPtr ||
				predEntry.IsRawSvalsPtr != entry.IsRawSvalsPtr {
				allPreds = false
				break
			}
		}
		if !allPreds {
			continue
		}
		ec.invalidateReg(reg, entry.ValueID)
		ec.activeRegs[entry.ValueID] = true
		if entry.IsRawInt {
			ec.setValueRepr(entry.ValueID, valueReprRawInt)
		}
		if entry.IsRawTablePtr {
			ec.setValueRepr(entry.ValueID, valueReprRawTablePtr)
		}
		if entry.IsRawDataPtr {
			ec.setValueRepr(entry.ValueID, valueReprRawDataPtr)
		}
		if entry.IsRawSvalsPtr {
			ec.setValueRepr(entry.ValueID, valueReprRawFieldSvalsPtr)
		}
	}
}

func (ec *emitContext) seedSinglePredRawFloatRegs(block *Block) {
	if ec == nil || block == nil || len(block.Preds) != 1 {
		return
	}
	liveIn := ec.blockLiveIn[block.ID]
	if len(liveIn) == 0 {
		return
	}
	pred := block.Preds[0]
	if pred == nil {
		return
	}
	ec.seedRawFloatRegsFromPredOut(liveIn, ec.blockOutRawFloatRegs[pred.ID])
}

func (ec *emitContext) seedMultiPredRawFloatRegs(block *Block) {
	if ec == nil || block == nil || len(block.Preds) <= 1 {
		return
	}
	liveIn := ec.blockLiveIn[block.ID]
	if len(liveIn) == 0 || block.Preds[0] == nil {
		return
	}
	firstOut := ec.blockOutRawFloatRegs[block.Preds[0].ID]
	if len(firstOut) == 0 {
		return
	}
	merged := make(map[int]loopFPRegEntry)
	for reg, entry := range firstOut {
		if !liveIn[entry.ValueID] {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[entry.ValueID]
		if !ok || !pr.IsFloat || pr.Reg != reg {
			continue
		}
		allPreds := true
		for _, pred := range block.Preds[1:] {
			if pred == nil {
				allPreds = false
				break
			}
			predEntry, ok := ec.blockOutRawFloatRegs[pred.ID][reg]
			if !ok || predEntry.ValueID != entry.ValueID {
				allPreds = false
				break
			}
		}
		if allPreds {
			merged[reg] = entry
		}
	}
	ec.seedRawFloatRegsFromPredOut(liveIn, merged)
}

func (ec *emitContext) seedRawFloatRegsFromPredOut(liveIn map[int]bool, predOut map[int]loopFPRegEntry) {
	if len(liveIn) == 0 || len(predOut) == 0 {
		return
	}
	regs := make([]int, 0, len(predOut))
	for reg := range predOut {
		regs = append(regs, reg)
	}
	sort.Ints(regs)
	for _, reg := range regs {
		entry := predOut[reg]
		if !liveIn[entry.ValueID] {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[entry.ValueID]
		if !ok || !pr.IsFloat || pr.Reg != reg {
			continue
		}
		ec.invalidateFPReg(reg, entry.ValueID)
		ec.activeFPRegs[entry.ValueID] = true
		ec.setValueRepr(entry.ValueID, valueReprRawFloat)
	}
}

func (ec *emitContext) seedSinglePredTableArrayKeyRegs(block *Block) {
	if ec == nil || block == nil || len(block.Preds) != 1 || ec.alloc == nil {
		return
	}
	liveIn := ec.blockLiveIn[block.ID]
	if len(liveIn) == 0 {
		return
	}
	pred := block.Preds[0]
	if pred == nil {
		return
	}
	keyUses := tableArrayKeyUsesInBlock(block)
	if len(keyUses) == 0 {
		return
	}
	defIndex := make(map[int]int)
	defs := make(map[int]*Instr)
	for i, instr := range pred.Instrs {
		if instr == nil || instr.Op.IsTerminator() {
			continue
		}
		defIndex[instr.ID] = i
		defs[instr.ID] = instr
	}
	ids := make([]int, 0, len(keyUses))
	for id := range keyUses {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, valueID := range ids {
		if !liveIn[valueID] {
			continue
		}
		def := defs[valueID]
		if def == nil || !isSinglePredRawCarryValue(def) {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || pr.IsFloat {
			continue
		}
		idx := defIndex[valueID]
		if singlePredRawValueClobberedAfter(pred, idx, pr.Reg, ec.alloc) {
			continue
		}
		ec.invalidateReg(pr.Reg, valueID)
		ec.activeRegs[valueID] = true
		ec.setValueRepr(valueID, valueReprRawInt)
	}
}

func tableArrayKeyUsesInBlock(block *Block) map[int]bool {
	out := make(map[int]bool)
	if block == nil {
		return out
	}
	for _, instr := range block.Instrs {
		if instr == nil {
			continue
		}
		keyArg, ok := tableArrayKeyArgIndex(instr.Op)
		if !ok {
			continue
		}
		if keyArg >= 0 && keyArg < len(instr.Args) && instr.Args[keyArg] != nil {
			out[instr.Args[keyArg].ID] = true
		}
	}
	return out
}

func singlePredRawValueClobberedAfter(block *Block, defIndex int, reg int, alloc *RegAllocation) bool {
	if block == nil || alloc == nil {
		return true
	}
	for i := defIndex + 1; i < len(block.Instrs); i++ {
		instr := block.Instrs[i]
		if instr == nil {
			continue
		}
		if opIsRawCarryClobber(instr.Op) {
			return true
		}
		if pr, ok := alloc.ValueRegs[instr.ID]; ok && !pr.IsFloat && pr.Reg == reg {
			return true
		}
	}
	return false
}

func (ec *emitContext) deactivateDeadAfter(instr *Instr) {
	if ec == nil || instr == nil {
		return
	}
	liveAfter := ec.instrLiveAfter[instr.ID]
	for valueID := range ec.activeRegs {
		if !liveAfter[valueID] {
			delete(ec.activeRegs, valueID)
			ec.clearValueRepr(valueID)
		}
	}
	for valueID := range ec.activeFPRegs {
		if !liveAfter[valueID] {
			delete(ec.activeFPRegs, valueID)
			ec.clearValueRepr(valueID)
		}
	}
}

// merge helpers moved to emit_merge.go (R96, file-size hygiene).
