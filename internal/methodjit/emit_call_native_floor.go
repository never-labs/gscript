//go:build darwin && arm64

// emit_call_native_floor.go: field-call / floor ARM64 paths.
//
// Pure code-movement split of emit_call_native.go (zero behavior change).

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
)

func (ec *emitContext) emitOpCallFloor(instr *Instr) {
	ec.emitOpCall(instr)
	ec.emitFloorProjectionFromCallResult(instr)
}

func (ec *emitContext) emitOpFieldCallFloor(instr *Instr) {
	if !ec.emitFieldShapeMethodCallFloorNative(instr) {
		ec.emitDeopt(instr)
	}
}

func (ec *emitContext) emitFieldShapeMethodCallFloorNative(instr *Instr) bool {
	cases := ec.fieldShapeTypedPeerMethodCallCases(instr)
	if len(cases) == 0 {
		return false
	}
	nArgs := len(instr.Args)
	nRets := callResultCountFromAux2(instr.Aux2)
	funcSlot := int(instr.Aux)
	asm := ec.asm

	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)

	fallbackLabel := ec.uniqueLabel("t2fieldmethod_fallback")
	callFallbackLabel := ec.uniqueLabel("t2fieldmethod_call_fallback")
	exitLabel := ec.uniqueLabel("t2fieldmethod_exit")
	doneLabel := ec.uniqueLabel("t2fieldmethod_done")
	preReprs := ec.snapshotValueReprs()
	calleeBaseOff := ec.nextSlot * jit.ValueSize
	argDesc := cases[0].desc
	argDesc.ArgFacts = nil
	ec.traceNativeCallEmit(instr, "typed peer", argDesc.Callee, &argDesc)
	allLeafCallees := fieldShapeTypedPeerCasesAllLeaf(cases)
	restoreConstsOnSuccess := ec.fnUsesConstPool()
	useClobberEntry := ec.shouldUseTypedPeerClobberEntry(liveGPRs, liveFPRs)
	maxCalleeStack := fieldShapeTypedPeerMaxStack(cases)
	if remarks := functionRemarks(ec.fn); remarks != nil {
		remarks.Add("TypedPeerABI", "changed", instr.Block.ID, instr.ID, instr.Op,
			fmt.Sprintf("field-shape method floor cases=%d leaf=%t max_stack=%d clobber_entry=%t live_gprs=%s live_fprs=%s",
				len(cases), allLeafCallees, maxCalleeStack, useClobberEntry,
				ec.formatLiveCallRegs(liveGPRs), ec.formatLiveCallRegs(liveFPRs)))
	}

	asm.SUBimm(jit.SP, jit.SP, rawPeerFrameSize)
	if !allLeafCallees {
		asm.STR(mRegRegs, jit.SP, rawPeerRegsOff)
		asm.STR(mRegConsts, jit.SP, rawPeerConstsOff)
	}
	if useClobberEntry {
		ec.emitSpillSelectiveForCall(liveGPRs, liveFPRs)
	} else {
		ec.emitSpillTypedPeerLiveForSuccess(liveFPRs)
	}
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, jit.SP, rawPeerClosureOff)
	ec.emitLoadCallMode(jit.X8)
	asm.STR(jit.X8, jit.SP, rawPeerCallModeOff)

	if allLeafCallees {
		ec.emitTypedPeerArgsFromValuesInRegs(instr.Args, argDesc, callFallbackLabel)
	} else {
		ec.emitTypedPeerArgsFromValuesInRegsAndSave(instr.Args, argDesc, callFallbackLabel)
	}
	ec.emitTypedPeerMaxStackCheck(calleeBaseOff, maxCalleeStack, fallbackLabel)
	asm.LDRW(jit.X9, jit.X0, jit.TableOffShapeID)
	for _, c := range cases {
		nextLabel := ec.uniqueLabel("t2fieldmethod_next")
		caseCallFallbackLabel := callFallbackLabel
		exactClosureFallbackLabel := ""
		if c.exactClosure != 0 {
			exactClosureFallbackLabel = ec.uniqueLabel("t2fieldmethod_exact_fallback")
			caseCallFallbackLabel = exactClosureFallbackLabel
		}
		emitCMPWConst(asm, jit.X9, jit.X12, int64(c.shapeID))
		asm.BCond(jit.CondNE, nextLabel)

		validateMethodLabel := ec.uniqueLabel("t2fieldmethod_validate")
		if c.exactClosure != 0 {
			if c.shapeEpochPtr != 0 {
				asm.LoadImm64(jit.X8, int64(c.shapeEpochPtr))
				asm.LDR(jit.X8, jit.X8, 0)
				asm.LoadImm64(jit.X12, int64(c.shapeEpoch))
				asm.CMPreg(jit.X8, jit.X12)
				asm.BCond(jit.CondNE, validateMethodLabel)
			}
			asm.LoadImm64(jit.X7, int64(c.exactClosure))
			asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
			asm.LoadImm64(jit.X7, int64(uintptr(unsafe.Pointer(c.callee))))
			asm.B(validateMethodLabel + "_entry")
		}

		asm.Label(validateMethodLabel)
		asm.LDR(jit.X6, jit.X0, jit.TableOffSvals)
		asm.LDR(jit.X6, jit.X6, c.fieldIdx*jit.ValueSize)
		if c.exactClosure != 0 {
			asm.LoadImm64(jit.X8, nbClosureTagBits|int64(c.exactClosure))
			asm.CMPreg(jit.X6, jit.X8)
			asm.BCond(jit.CondNE, callFallbackLabel)
			asm.LoadImm64(jit.X7, int64(c.exactClosure))
			asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
			asm.LoadImm64(jit.X7, int64(uintptr(unsafe.Pointer(c.callee))))
		} else {
			asm.LSRimm(jit.X7, jit.X6, uint8(nbPtrSubShift))
			asm.LoadImm64(jit.X8, int64((jit.NB_TagPtrShr48<<4)|nbPtrSubVMClosure))
			asm.CMPreg(jit.X7, jit.X8)
			asm.BCond(jit.CondNE, callFallbackLabel)
			jit.EmitExtractPtr(asm, jit.X7, jit.X6)
			asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
			asm.LDR(jit.X7, jit.X7, vmClosureOffProto)
			asm.LoadImm64(jit.X8, int64(uintptr(unsafe.Pointer(c.callee))))
			asm.CMPreg(jit.X7, jit.X8)
			asm.BCond(jit.CondNE, callFallbackLabel)
		}
		asm.Label(validateMethodLabel + "_entry")
		if useClobberEntry {
			asm.LDR(jit.X16, jit.X7, funcProtoOffTier2TypedClobberEntryPtr)
		} else {
			asm.LDR(jit.X16, jit.X7, funcProtoOffTier2TypedEntryPtr)
		}
		asm.CBZ(jit.X16, caseCallFallbackLabel)

		if !c.callee.LeafNoCall {
			asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
			asm.CMPimm(jit.X8, maxNativeCallDepth)
			asm.BCond(jit.CondGE, caseCallFallbackLabel)
		}

		if calleeBaseOff <= 4095 {
			asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
		} else {
			asm.LoadImm64(jit.X8, int64(calleeBaseOff))
			asm.ADDreg(mRegRegs, mRegRegs, jit.X8)
		}
		asm.LDR(mRegConsts, jit.X7, funcProtoOffConstants)
		if useClobberEntry {
			asm.MOVimm16(jit.X8, callModeTypedPeerClobber)
		} else {
			asm.MOVimm16(jit.X8, callModeTypedSelf)
		}
		ec.emitStoreCallMode(jit.X8)
		if !c.callee.LeafNoCall {
			asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
			asm.ADDimm(jit.X8, jit.X8, 1)
			asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		}

		asm.BLR(jit.X16)

		if !c.callee.LeafNoCall {
			asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
			asm.SUBimm(jit.X8, jit.X8, 1)
			asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		}
		asm.CBNZ(jit.X16, exitLabel)

		if c.callee.LeafNoCall {
			ec.emitRestoreTypedPeerLeafCallerStateWithConsts(calleeBaseOff, restoreConstsOnSuccess)
		} else {
			ec.emitRestoreTypedPeerCallerState()
		}
		ec.emitTier2CallCounter(instr, "field_call_floor", "success")
		asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
		if useClobberEntry {
			ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)
		} else {
			ec.emitReloadTypedPeerLiveForSuccess(liveFPRs)
		}
		ec.restoreValueReprSnapshot(preReprs)
		switch c.desc.ReturnRep {
		case SpecializedABIReturnRawInt:
			ec.storeRawInt(jit.X0, instr.ID)
		case SpecializedABIReturnRawFloat:
			asm.FMOVtoFP(jit.D0, jit.X0)
			asm.FRINTMd(jit.D0, jit.D0)
			asm.FCVTZS(jit.X0, jit.D0)
			ec.storeRawInt(jit.X0, instr.ID)
		default:
			asm.B(caseCallFallbackLabel)
		}
		ec.emitFieldCallPolyLenFusionStores(instr.ID, uint32(c.shapeID))
		asm.B(doneLabel)
		if exactClosureFallbackLabel != "" {
			asm.Label(exactClosureFallbackLabel)
			asm.LoadImm64(jit.X6, nbClosureTagBits|int64(c.exactClosure))
			asm.B(callFallbackLabel)
		}
		asm.Label(nextLabel)
	}
	postReprs := ec.snapshotValueReprs()
	asm.B(callFallbackLabel)

	asm.Label(exitLabel)
	ec.emitTier2CallCounter(instr, "field_call_floor", "exit")
	ec.emitPushNativeCallExitFrameIfNested(jit.X8, jit.X9, jit.X10, jit.X11)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitCode)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeExitCode)
	asm.LDR(jit.X8, mRegCtx, execCtxOffResumeNumericPass)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePass)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitResumePC)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePC)
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.UBFX(jit.X6, jit.X8, 0, 44)
	asm.LoadImm64(jit.X12, nbClosureTagBits)
	asm.ORRreg(jit.X6, jit.X6, jit.X12)
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)
	asm.MOVimm16(jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeTier2Only)
	if allLeafCallees {
		ec.emitRestoreTypedPeerLeafCallerState(calleeBaseOff)
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	if !useClobberEntry {
		ec.emitSpillSelectiveForCall(liveGPRs, nil)
	}
	if allLeafCallees {
		ec.emitMaterializeTypedPeerCallFrameFromValues(funcSlot, instr.Args, jit.X6)
	} else {
		ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, argDesc)
	}
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)
	ec.emitFloorProjectionFromCallResult(instr)
	ec.emitUnboxRawIntRegs(postReprs)
	ec.restoreValueReprSnapshot(postReprs)
	asm.B(doneLabel)

	asm.Label(callFallbackLabel)
	ec.emitTier2CallCounter(instr, "field_call_floor", "fallback")
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)
	if allLeafCallees {
		ec.emitRestoreTypedPeerCallerModeClosureOnly()
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	if allLeafCallees {
		ec.emitMaterializeTypedPeerCallFrameFromValues(funcSlot, instr.Args, jit.X6)
	} else {
		ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, argDesc)
	}
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitFloorProjectionFromCallResult(instr)
	ec.emitUnboxRawIntRegs(postReprs)
	ec.restoreValueReprSnapshot(postReprs)
	asm.B(doneLabel)

	asm.Label(fallbackLabel)
	if allLeafCallees {
		ec.emitRestoreTypedPeerCallerModeClosureOnly()
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitDeopt(instr)

	asm.Label(doneLabel)
	return true
}

func (ec *emitContext) emitFieldCallPolyLenFusionStores(callID int, shapeID uint32) {
	if ec == nil || ec.fn == nil {
		return
	}
	fusions, _ := functionTableShapeFacts(ec.fn).FieldCallPolyLenFusionCases(callID)
	if len(fusions) == 0 {
		return
	}
	for _, fusion := range fusions {
		if fusion.ShapeID != shapeID {
			continue
		}
		if !ec.canMaterializeFusionValueNow(fusion.LenValueID) {
			continue
		}
		ec.asm.LoadImm64(jit.X8, fusion.Len)
		ec.storeRawInt(jit.X8, fusion.LenValueID)
	}
}

func (ec *emitContext) canMaterializeFusionValueNow(valueID int) bool {
	if ec == nil || ec.alloc == nil {
		return false
	}
	pr, ok := ec.alloc.ValueRegs[valueID]
	if !ok || pr.IsFloat {
		return false
	}
	for activeID, active := range ec.activeRegs {
		if !active || activeID == valueID {
			continue
		}
		activePR, ok := ec.alloc.ValueRegs[activeID]
		if ok && !activePR.IsFloat && activePR.Reg == pr.Reg {
			return false
		}
	}
	return true
}

func (ec *emitContext) emitFieldShapeMethodCallFloorNativeSingleCase(instr *Instr, c fieldShapeTypedPeerCallCase) bool {
	if ec == nil || ec.asm == nil || instr == nil {
		return false
	}
	nArgs := len(instr.Args)
	nRets := callResultCountFromAux2(instr.Aux2)
	funcSlot := int(instr.Aux)
	asm := ec.asm

	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillTypedPeerLiveForSuccess(liveFPRs)

	fallbackLabel := ec.uniqueLabel("t2fieldmethod_fallback")
	callFallbackLabel := ec.uniqueLabel("t2fieldmethod_call_fallback")
	exitLabel := ec.uniqueLabel("t2fieldmethod_exit")
	doneLabel := ec.uniqueLabel("t2fieldmethod_done")
	preReprs := ec.snapshotValueReprs()
	calleeBaseOff := ec.nextSlot * jit.ValueSize
	argDesc := c.desc
	argDesc.ArgFacts = nil

	asm.SUBimm(jit.SP, jit.SP, rawPeerFrameSize)
	if !c.callee.LeafNoCall {
		asm.STR(mRegRegs, jit.SP, rawPeerRegsOff)
		asm.STR(mRegConsts, jit.SP, rawPeerConstsOff)
	}
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, jit.SP, rawPeerClosureOff)
	ec.emitLoadCallMode(jit.X8)
	asm.STR(jit.X8, jit.SP, rawPeerCallModeOff)

	if c.callee.LeafNoCall {
		ec.emitTypedPeerArgsFromValuesInRegs(instr.Args, argDesc, fallbackLabel)
	} else {
		ec.emitTypedPeerArgsFromValuesInRegsAndSave(instr.Args, argDesc, fallbackLabel)
	}
	asm.LDRW(jit.X9, jit.X0, jit.TableOffShapeID)
	emitCMPWConst(asm, jit.X9, jit.X12, int64(c.shapeID))
	asm.BCond(jit.CondNE, fallbackLabel)

	validateMethodLabel := ec.uniqueLabel("t2fieldmethod_validate")
	if c.exactClosure != 0 {
		if c.shapeEpochPtr != 0 {
			asm.LoadImm64(jit.X8, int64(c.shapeEpochPtr))
			asm.LDR(jit.X8, jit.X8, 0)
			asm.LoadImm64(jit.X12, int64(c.shapeEpoch))
			asm.CMPreg(jit.X8, jit.X12)
			asm.BCond(jit.CondNE, validateMethodLabel)
		}
		asm.LoadImm64(jit.X6, nbClosureTagBits|int64(c.exactClosure))
		asm.LoadImm64(jit.X7, int64(c.exactClosure))
		asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
		asm.LoadImm64(jit.X7, int64(uintptr(unsafe.Pointer(c.callee))))
		asm.B(validateMethodLabel + "_entry")
	}

	asm.Label(validateMethodLabel)
	asm.LDR(jit.X6, jit.X0, jit.TableOffSvals)
	asm.LDR(jit.X6, jit.X6, c.fieldIdx*jit.ValueSize)
	if c.exactClosure != 0 {
		asm.LoadImm64(jit.X8, nbClosureTagBits|int64(c.exactClosure))
		asm.CMPreg(jit.X6, jit.X8)
		asm.BCond(jit.CondNE, callFallbackLabel)
		asm.LoadImm64(jit.X7, int64(c.exactClosure))
		asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
		asm.LoadImm64(jit.X7, int64(uintptr(unsafe.Pointer(c.callee))))
	} else {
		asm.LSRimm(jit.X7, jit.X6, uint8(nbPtrSubShift))
		asm.LoadImm64(jit.X8, int64((jit.NB_TagPtrShr48<<4)|nbPtrSubVMClosure))
		asm.CMPreg(jit.X7, jit.X8)
		asm.BCond(jit.CondNE, callFallbackLabel)
		jit.EmitExtractPtr(asm, jit.X7, jit.X6)
		asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
		asm.LDR(jit.X7, jit.X7, vmClosureOffProto)
		asm.LoadImm64(jit.X8, int64(uintptr(unsafe.Pointer(c.callee))))
		asm.CMPreg(jit.X7, jit.X8)
		asm.BCond(jit.CondNE, callFallbackLabel)
	}
	asm.Label(validateMethodLabel + "_entry")
	caseCallFallbackLabel := callFallbackLabel
	exactClosureFallbackLabel := ""
	if c.exactClosure != 0 {
		exactClosureFallbackLabel = ec.uniqueLabel("t2fieldmethod_exact_fallback")
		caseCallFallbackLabel = exactClosureFallbackLabel
	}
	asm.LDR(jit.X16, jit.X7, funcProtoOffTier2TypedEntryPtr)
	asm.CBZ(jit.X16, caseCallFallbackLabel)
	ec.emitTypedPeerABICheck(jit.X7, c.desc, caseCallFallbackLabel)

	if !c.callee.LeafNoCall {
		asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		asm.CMPimm(jit.X8, maxNativeCallDepth)
		asm.BCond(jit.CondGE, caseCallFallbackLabel)
	}

	asm.LoadImm64(jit.X8, int64(c.callee.MaxStack*jit.ValueSize))
	if calleeBaseOff <= 4095 {
		asm.ADDimm(jit.X8, jit.X8, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X12, int64(calleeBaseOff))
		asm.ADDreg(jit.X8, jit.X8, jit.X12)
	}
	asm.ADDreg(jit.X8, jit.X8, mRegRegs)
	asm.LDR(jit.X12, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X8, jit.X12)
	asm.BCond(jit.CondHI, caseCallFallbackLabel)

	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X8)
	}
	asm.LDR(mRegConsts, jit.X7, funcProtoOffConstants)
	asm.MOVimm16(jit.X8, callModeTypedSelf)
	ec.emitStoreCallMode(jit.X8)
	if !c.callee.LeafNoCall {
		asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		asm.ADDimm(jit.X8, jit.X8, 1)
		asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	}

	asm.BLR(jit.X16)

	if !c.callee.LeafNoCall {
		asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		asm.SUBimm(jit.X8, jit.X8, 1)
		asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	}
	asm.CBNZ(jit.X16, exitLabel)

	if c.callee.LeafNoCall {
		ec.emitRestoreTypedPeerLeafCallerStateWithConsts(calleeBaseOff, ec.fnUsesConstPool())
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.emitTier2CallCounter(instr, "field_call_floor", "success")
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitReloadTypedPeerLiveForSuccess(liveFPRs)
	ec.restoreValueReprSnapshot(preReprs)
	switch c.desc.ReturnRep {
	case SpecializedABIReturnRawInt:
		ec.storeRawInt(jit.X0, instr.ID)
	case SpecializedABIReturnRawFloat:
		asm.FMOVtoFP(jit.D0, jit.X0)
		asm.FRINTMd(jit.D0, jit.D0)
		asm.FCVTZS(jit.X0, jit.D0)
		ec.storeRawInt(jit.X0, instr.ID)
	default:
		asm.B(caseCallFallbackLabel)
	}
	asm.B(doneLabel)
	if exactClosureFallbackLabel != "" {
		asm.Label(exactClosureFallbackLabel)
		asm.LoadImm64(jit.X6, nbClosureTagBits|int64(c.exactClosure))
		asm.B(callFallbackLabel)
	}

	asm.Label(exitLabel)
	ec.emitTier2CallCounter(instr, "field_call_floor", "exit")
	ec.emitPushNativeCallExitFrameIfNested(jit.X8, jit.X9, jit.X10, jit.X11)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitCode)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeExitCode)
	asm.LDR(jit.X8, mRegCtx, execCtxOffResumeNumericPass)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePass)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitResumePC)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePC)
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.UBFX(jit.X6, jit.X8, 0, 44)
	asm.LoadImm64(jit.X12, nbClosureTagBits)
	asm.ORRreg(jit.X6, jit.X6, jit.X12)
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)
	asm.MOVimm16(jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeTier2Only)
	if c.callee.LeafNoCall {
		ec.emitRestoreTypedPeerLeafCallerState(calleeBaseOff)
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitSpillSelectiveForCall(liveGPRs, nil)
	if c.callee.LeafNoCall {
		ec.emitMaterializeTypedPeerCallFrameFromValues(funcSlot, instr.Args, jit.X6)
	} else {
		ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, argDesc)
	}
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)
	ec.emitFloorProjectionFromCallResult(instr)
	ec.emitUnboxRawIntRegs(preReprs)
	ec.restoreValueReprSnapshot(preReprs)
	asm.B(doneLabel)

	asm.Label(callFallbackLabel)
	ec.emitTier2CallCounter(instr, "field_call_floor", "fallback")
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)
	if c.callee.LeafNoCall {
		ec.emitRestoreTypedPeerCallerModeClosureOnly()
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	if c.callee.LeafNoCall {
		ec.emitMaterializeTypedPeerCallFrameFromValues(funcSlot, instr.Args, jit.X6)
	} else {
		ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, argDesc)
	}
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitFloorProjectionFromCallResult(instr)
	ec.emitUnboxRawIntRegs(preReprs)
	ec.restoreValueReprSnapshot(preReprs)

	asm.Label(fallbackLabel)
	if c.callee.LeafNoCall {
		ec.emitRestoreTypedPeerCallerModeClosureOnly()
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitDeopt(instr)

	asm.Label(doneLabel)
	return true
}

func (ec *emitContext) emitFloorProjectionFromCallResult(instr *Instr) {
	if instr == nil {
		return
	}
	asm := ec.asm
	valueID := instr.ID

	// OpCallFloor reuses the call's SSA id for the projected floor result.
	// Snapshot the current post-call representation before storing the raw int
	// back to the same id; routing this through the ordinary OpFloor protocol
	// would make the input and output self-referential.
	if ec.hasReg(valueID) && ec.valueReprOf(valueID) == valueReprRawInt {
		src := ec.physReg(valueID)
		if src != jit.X0 {
			asm.MOVreg(jit.X0, src)
		}
		ec.storeRawInt(jit.X0, valueID)
		return
	}
	if ec.hasFPReg(valueID) {
		asm.FRINTMd(jit.D0, ec.physFPReg(valueID))
		asm.FCVTZS(jit.X0, jit.D0)
		ec.storeRawInt(jit.X0, valueID)
		return
	}

	srcReg := ec.resolveValueNB(valueID, jit.X0)
	if srcReg != jit.X0 {
		asm.MOVreg(jit.X0, srcReg)
	}

	floatLabel := ec.uniqueLabel("call_floor_float")
	deoptLabel := ec.uniqueLabel("call_floor_deopt")
	doneLabel := ec.uniqueLabel("call_floor_done")

	emitCheckIsIntPinned(asm, jit.X0, jit.X2)
	asm.BCond(jit.CondNE, floatLabel)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	ec.storeRawInt(jit.X0, valueID)
	asm.B(doneLabel)

	asm.Label(floatLabel)
	asm.LSRimm(jit.X2, jit.X0, 48)
	asm.MOVimm16(jit.X3, jit.NB_TagNilShr48)
	asm.CMPreg(jit.X2, jit.X3)
	asm.BCond(jit.CondGE, deoptLabel)
	asm.FMOVtoFP(jit.D0, jit.X0)
	asm.FRINTMd(jit.D0, jit.D0)
	asm.FCVTZS(jit.X0, jit.D0)
	ec.storeRawInt(jit.X0, valueID)
	asm.B(doneLabel)

	asm.Label(deoptLabel)
	ec.emitDeopt(instr)
	asm.Label(doneLabel)
}
