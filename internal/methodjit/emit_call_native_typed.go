//go:build darwin && arm64

// emit_call_native_typed.go: typed-ABI self/peer ARM64 paths.
//
// Pure code-movement split of emit_call_native.go (zero behavior change).

package methodjit

import (
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
	gruntime "github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

const (
	typedSelfSavedCallModeOff = 0
	typedSelfArgsOff          = 8
)

func typedSelfFrameSizeFor(nArgs int) int {
	size := typedSelfArgsOff + nArgs*jit.ValueSize
	return (size + 15) &^ 15
}

func (ec *emitContext) emitCallNativeTypedPeerIfEligible(instr *Instr) bool {
	if ec == nil || ec.fn == nil || instr == nil {
		return false
	}
	desc, ok := functionCallFacts(ec.fn).CallABI(instr.ID)
	if !ok || !desc.TypedPeer || desc.Callee == nil {
		return false
	}
	callee := desc.Callee
	nArgs := len(instr.Args) - 1
	nRets := callResultCountFromAux2(instr.Aux2)
	if nRets != desc.NumRets || nArgs != desc.NumArgs || nArgs != callee.NumParams || nArgs < 1 || nArgs > 4 {
		return false
	}
	if len(desc.ParamReps) != nArgs {
		return false
	}
	switch desc.ReturnRep {
	case SpecializedABIReturnRawInt, SpecializedABIReturnRawFloat, SpecializedABIReturnRawTablePtr:
		if nRets != 1 {
			return false
		}
	case SpecializedABIReturnNone:
		if nRets != 0 {
			return false
		}
	default:
		return false
	}
	ec.traceNativeCallEmit(instr, "typed peer", callee, &desc)

	asm := ec.asm
	funcSlot := int(instr.Aux)
	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillTypedPeerLiveForSuccess(liveFPRs)

	fallbackLabel := ec.uniqueLabel("t2typedpeer_fallback")
	exitLabel := ec.uniqueLabel("t2typedpeer_exit")
	doneLabel := ec.uniqueLabel("t2typedpeer_done")
	preReprs := ec.snapshotValueReprs()

	asm.SUBimm(jit.SP, jit.SP, rawPeerFrameSize)
	asm.STR(mRegRegs, jit.SP, rawPeerRegsOff)
	asm.STR(mRegConsts, jit.SP, rawPeerConstsOff)
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, jit.SP, rawPeerClosureOff)
	ec.emitLoadCallMode(jit.X8)
	asm.STR(jit.X8, jit.SP, rawPeerCallModeOff)

	fnReg := ec.resolveValueNB(instr.Args[0].ID, jit.X6)
	if fnReg != jit.X6 {
		asm.MOVreg(jit.X6, fnReg)
	}
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)
	ec.emitTypedPeerArgsInRegsAndSave(instr, desc, fallbackLabel)

	// Argument guards use X6 as scratch. Restore the callee closure value
	// before validating and extracting the typed entry.
	asm.LDR(jit.X6, jit.SP, rawPeerFuncOff)
	asm.LSRimm(jit.X7, jit.X6, uint8(nbPtrSubShift))
	asm.LoadImm64(jit.X8, int64((jit.NB_TagPtrShr48<<4)|nbPtrSubVMClosure))
	asm.CMPreg(jit.X7, jit.X8)
	asm.BCond(jit.CondNE, fallbackLabel)
	jit.EmitExtractPtr(asm, jit.X7, jit.X6)
	asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.LDR(jit.X7, jit.X7, vmClosureOffProto)
	asm.LoadImm64(jit.X8, int64(uintptr(unsafe.Pointer(callee))))
	asm.CMPreg(jit.X7, jit.X8)
	asm.BCond(jit.CondNE, fallbackLabel)
	asm.LDR(jit.X16, jit.X7, funcProtoOffTier2TypedEntryPtr)
	asm.CBZ(jit.X16, fallbackLabel)
	ec.emitTypedPeerABICheck(jit.X7, desc, fallbackLabel)

	asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.CMPimm(jit.X8, maxNativeCallDepth)
	asm.BCond(jit.CondGE, fallbackLabel)

	calleeBaseOff := ec.nextSlot * jit.ValueSize
	asm.LDR(jit.X8, jit.X7, funcProtoOffMaxStack)
	asm.LSLimm(jit.X8, jit.X8, 3)
	if calleeBaseOff <= 4095 {
		asm.ADDimm(jit.X8, jit.X8, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X9, int64(calleeBaseOff))
		asm.ADDreg(jit.X8, jit.X8, jit.X9)
	}
	asm.ADDreg(jit.X8, jit.X8, mRegRegs)
	asm.LDR(jit.X9, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X8, jit.X9)
	asm.BCond(jit.CondHI, fallbackLabel)

	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X8)
	}
	asm.LDR(mRegConsts, jit.X7, funcProtoOffConstants)
	asm.MOVimm16(jit.X8, callModeTypedSelf)
	ec.emitStoreCallMode(jit.X8)
	asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.ADDimm(jit.X8, jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)

	asm.BLR(jit.X16)

	asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.SUBimm(jit.X8, jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.CBNZ(jit.X16, exitLabel)

	ec.emitRestoreTypedPeerCallerState()
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitReloadTypedPeerLiveForSuccess(liveFPRs)
	ec.restoreValueReprSnapshot(preReprs)
	switch desc.ReturnRep {
	case SpecializedABIReturnRawInt:
		ec.storeRawInt(jit.X0, instr.ID)
	case SpecializedABIReturnRawFloat:
		asm.FMOVtoFP(jit.D0, jit.X0)
		ec.storeRawFloat(jit.D0, instr.ID)
	case SpecializedABIReturnRawTablePtr:
		emitBoxTablePtr(asm, jit.X0, jit.X0, jit.X1)
		ec.storeResultNB(jit.X0, instr.ID)
	case SpecializedABIReturnNone:
		// Fixed-arity side-effecting calls produce no SSA result.
	}
	postReprs := ec.snapshotValueReprs()
	asm.B(doneLabel)

	asm.Label(exitLabel)
	ec.emitPushNativeCallExitFrameIfNested(jit.X8, jit.X9, jit.X10, jit.X11)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitCode)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeExitCode)
	asm.LDR(jit.X8, mRegCtx, execCtxOffResumeNumericPass)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePass)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitResumePC)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePC)
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.MOVimm16(jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeTier2Only)
	ec.emitRestoreTypedPeerCallerState()
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitSpillSelectiveForCall(liveGPRs, nil)
	ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, desc)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)
	ec.emitUnboxRawIntRegs(postReprs)
	ec.restoreValueReprSnapshot(postReprs)
	asm.B(doneLabel)

	asm.Label(fallbackLabel)
	ec.emitRestoreTypedPeerCallerState()
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, desc)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitUnboxRawIntRegs(postReprs)
	ec.restoreValueReprSnapshot(postReprs)

	asm.Label(doneLabel)
	return true
}

type fieldShapeTypedPeerCallCase struct {
	shapeID       int
	fieldIdx      int
	callee        *vm.FuncProto
	exactClosure  uintptr
	shapeEpochPtr uintptr
	shapeEpoch    uint64
	desc          CallABIDescriptor
}

func (ec *emitContext) fieldShapeTypedPeerCallCases(instr *Instr) []fieldShapeTypedPeerCallCase {
	if ec == nil || ec.fn == nil || instr == nil || (instr.Op != OpCall && instr.Op != OpCallFloor) || len(instr.Args) < 2 {
		return nil
	}
	calleeLoad := instr.Args[0].Def
	if calleeLoad == nil || calleeLoad.Op != OpGetField || len(calleeLoad.Args) == 0 || calleeLoad.Args[0] == nil {
		return nil
	}
	receiver := calleeLoad.Args[0]
	if instr.Args[1] == nil || instr.Args[1].ID != receiver.ID {
		return nil
	}
	nArgs := len(instr.Args) - 1
	if callResultCountFromAux2(instr.Aux2) != 1 || nArgs < 1 || nArgs > 4 {
		return nil
	}
	cases, _ := functionTableShapeFacts(ec.fn).FieldPolyShapeCases(calleeLoad.ID)
	if len(cases) < 2 {
		return nil
	}
	out := make([]fieldShapeTypedPeerCallCase, 0, len(cases))
	var paramReps []SpecializedABIParamRep
	for _, c := range cases {
		if c.ShapeID == 0 || c.FieldIdx < 0 || c.VMProto == nil || c.VMProto.NumParams != nArgs {
			return nil
		}
		argFacts := map[int]FixedShapeTableFact{0: c.ReceiverFact}
		abi := AnalyzeTypedPeerABIWithArgFacts(c.VMProto, argFacts)
		if !abi.Eligible || len(abi.Params) != nArgs || abi.Params[0] != SpecializedABIParamRawTablePtr {
			return nil
		}
		switch abi.Return {
		case SpecializedABIReturnRawInt, SpecializedABIReturnRawFloat, SpecializedABIReturnRawTablePtr:
		default:
			return nil
		}
		if len(paramReps) == 0 {
			paramReps = append([]SpecializedABIParamRep(nil), abi.Params...)
		} else {
			for i, rep := range abi.Params {
				if paramReps[i] != rep {
					return nil
				}
			}
		}
		for i, rep := range abi.Params {
			switch rep {
			case SpecializedABIParamRawInt:
				if !callABIValueIsInt(instr.Args[1+i]) {
					return nil
				}
			case SpecializedABIParamRawTablePtr:
				if !callABIValueIsTable(instr.Args[1+i]) && i != 0 {
					return nil
				}
			default:
				return nil
			}
		}
		desc := CallABIDescriptor{
			Callee:    c.VMProto,
			NumArgs:   nArgs,
			NumRets:   1,
			TypedPeer: true,
			ParamReps: append([]SpecializedABIParamRep(nil), abi.Params...),
			ReturnRep: abi.Return,
			ArgFacts:  argFacts,
		}
		out = append(out, fieldShapeTypedPeerCallCase{
			shapeID:       int(c.ShapeID),
			fieldIdx:      c.FieldIdx,
			callee:        c.VMProto,
			exactClosure:  c.VMClosure,
			shapeEpochPtr: uintptr(gruntime.ShapeFieldMutationCountPtr(c.ShapeID, c.FieldIdx)),
			shapeEpoch:    gruntime.ShapeFieldMutationCount(c.ShapeID, c.FieldIdx),
			desc:          desc,
		})
	}
	return out
}

func (ec *emitContext) fieldShapeTypedPeerMethodCallCases(instr *Instr) []fieldShapeTypedPeerCallCase {
	if ec == nil || ec.fn == nil || instr == nil || instr.Op != OpFieldCallFloor || len(instr.Args) < 1 {
		return nil
	}
	nArgs := len(instr.Args)
	if callResultCountFromAux2(instr.Aux2) != 1 || nArgs < 1 || nArgs > 4 {
		return nil
	}
	cases, _ := functionTableShapeFacts(ec.fn).FieldPolyShapeCases(instr.ID)
	if len(cases) == 0 {
		return nil
	}
	out := make([]fieldShapeTypedPeerCallCase, 0, len(cases))
	var paramReps []SpecializedABIParamRep
	for _, c := range cases {
		if c.ShapeID == 0 || c.FieldIdx < 0 || c.VMProto == nil || c.VMProto.NumParams != nArgs {
			return nil
		}
		argFacts := map[int]FixedShapeTableFact{0: c.ReceiverFact}
		abi := AnalyzeTypedPeerABIWithArgFacts(c.VMProto, argFacts)
		if !abi.Eligible || len(abi.Params) != nArgs || abi.Params[0] != SpecializedABIParamRawTablePtr {
			return nil
		}
		switch abi.Return {
		case SpecializedABIReturnRawInt, SpecializedABIReturnRawFloat:
		default:
			return nil
		}
		if len(paramReps) == 0 {
			paramReps = append([]SpecializedABIParamRep(nil), abi.Params...)
		} else {
			for i, rep := range abi.Params {
				if paramReps[i] != rep {
					return nil
				}
			}
		}
		for i, rep := range abi.Params {
			switch rep {
			case SpecializedABIParamRawInt:
				if !callABIValueIsInt(instr.Args[i]) {
					return nil
				}
			case SpecializedABIParamRawTablePtr:
				if i != 0 && !callABIValueIsTable(instr.Args[i]) {
					return nil
				}
			default:
				return nil
			}
		}
		shapeEpochPtr := uintptr(gruntime.ShapeFieldMutationCountPtr(c.ShapeID, c.FieldIdx))
		shapeEpoch := gruntime.ShapeFieldMutationCount(c.ShapeID, c.FieldIdx)
		if fieldShapeMethodFieldStableInCallee(c) {
			shapeEpochPtr = 0
			shapeEpoch = 0
		}
		out = append(out, fieldShapeTypedPeerCallCase{
			shapeID:       int(c.ShapeID),
			fieldIdx:      c.FieldIdx,
			callee:        c.VMProto,
			exactClosure:  c.VMClosure,
			shapeEpochPtr: shapeEpochPtr,
			shapeEpoch:    shapeEpoch,
			desc: CallABIDescriptor{
				Callee:    c.VMProto,
				NumArgs:   nArgs,
				NumRets:   1,
				TypedPeer: true,
				ParamReps: append([]SpecializedABIParamRep(nil), abi.Params...),
				ReturnRep: abi.Return,
				ArgFacts:  argFacts,
			},
		})
	}
	return out
}

func fieldShapeMethodFieldStableInCallee(c FieldPolyShapeCase) bool {
	if c.VMProto == nil || c.FieldIdx < 0 || c.FieldIdx >= len(c.ReceiverFact.FieldNames) {
		return false
	}
	field := c.ReceiverFact.FieldNames[c.FieldIdx]
	if field == "" {
		return false
	}
	effects := SummarizeFieldEffects(c.VMProto)
	return effects.ParamMutationKnown(0) && !effects.WritesParamField(0, field)
}

func fieldShapeTypedPeerCasesAllLeaf(cases []fieldShapeTypedPeerCallCase) bool {
	if len(cases) == 0 {
		return false
	}
	for _, c := range cases {
		if c.callee == nil || !c.callee.LeafNoCall {
			return false
		}
	}
	return true
}

func (ec *emitContext) emitCallNativeFieldShapeTypedPeerIfEligible(instr *Instr) bool {
	cases := ec.fieldShapeTypedPeerCallCases(instr)
	if len(cases) < 2 {
		return false
	}
	nArgs := len(instr.Args) - 1
	nRets := callResultCountFromAux2(instr.Aux2)
	funcSlot := int(instr.Aux)
	asm := ec.asm

	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillTypedPeerLiveForSuccess(liveFPRs)

	fallbackLabel := ec.uniqueLabel("t2fieldpeer_fallback")
	exitLabel := ec.uniqueLabel("t2fieldpeer_exit")
	doneLabel := ec.uniqueLabel("t2fieldpeer_done")
	preReprs := ec.snapshotValueReprs()
	calleeBaseOff := ec.nextSlot * jit.ValueSize
	argDesc := cases[0].desc
	argDesc.ArgFacts = nil
	ec.traceNativeCallEmit(instr, "typed peer", argDesc.Callee, &argDesc)
	allLeafCallees := fieldShapeTypedPeerCasesAllLeaf(cases)
	restoreConstsOnSuccess := ec.fnUsesConstPool()

	asm.SUBimm(jit.SP, jit.SP, rawPeerFrameSize)
	if !allLeafCallees {
		asm.STR(mRegRegs, jit.SP, rawPeerRegsOff)
		asm.STR(mRegConsts, jit.SP, rawPeerConstsOff)
	}
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, jit.SP, rawPeerClosureOff)
	ec.emitLoadCallMode(jit.X8)
	asm.STR(jit.X8, jit.SP, rawPeerCallModeOff)

	fnReg := ec.resolveValueNB(instr.Args[0].ID, jit.X6)
	if fnReg != jit.X6 {
		asm.MOVreg(jit.X6, fnReg)
	}
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)
	ec.emitTypedPeerArgsInRegsAndSave(instr, argDesc, fallbackLabel)

	asm.LDRW(jit.X9, jit.X0, jit.TableOffShapeID)
	for _, c := range cases {
		nextLabel := ec.uniqueLabel("t2fieldpeer_next")
		emitCMPWConst(asm, jit.X9, jit.X12, int64(c.shapeID))
		asm.BCond(jit.CondNE, nextLabel)
		asm.LDR(jit.X6, jit.SP, rawPeerFuncOff)
		if c.exactClosure != 0 {
			asm.LoadImm64(jit.X8, nbClosureTagBits|int64(c.exactClosure))
			asm.CMPreg(jit.X6, jit.X8)
			asm.BCond(jit.CondNE, fallbackLabel)
			asm.LoadImm64(jit.X7, int64(c.exactClosure))
			asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
			asm.LoadImm64(jit.X7, int64(uintptr(unsafe.Pointer(c.callee))))
		} else {
			asm.LSRimm(jit.X7, jit.X6, uint8(nbPtrSubShift))
			asm.LoadImm64(jit.X8, int64((jit.NB_TagPtrShr48<<4)|nbPtrSubVMClosure))
			asm.CMPreg(jit.X7, jit.X8)
			asm.BCond(jit.CondNE, fallbackLabel)
			jit.EmitExtractPtr(asm, jit.X7, jit.X6)
			asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
			asm.LDR(jit.X7, jit.X7, vmClosureOffProto)
			asm.LoadImm64(jit.X8, int64(uintptr(unsafe.Pointer(c.callee))))
			asm.CMPreg(jit.X7, jit.X8)
			asm.BCond(jit.CondNE, fallbackLabel)
		}
		asm.LDR(jit.X16, jit.X7, funcProtoOffTier2TypedEntryPtr)
		asm.CBZ(jit.X16, fallbackLabel)

		if !c.callee.LeafNoCall {
			asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
			asm.CMPimm(jit.X8, maxNativeCallDepth)
			asm.BCond(jit.CondGE, fallbackLabel)
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
		asm.BCond(jit.CondHI, fallbackLabel)

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
			ec.emitRestoreTypedPeerLeafCallerStateWithConsts(calleeBaseOff, restoreConstsOnSuccess)
		} else {
			ec.emitRestoreTypedPeerCallerState()
		}
		ec.emitTier2CallCounter(instr, "field_call_floor", "success")
		asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
		ec.emitReloadTypedPeerLiveForSuccess(liveFPRs)
		ec.restoreValueReprSnapshot(preReprs)
		switch c.desc.ReturnRep {
		case SpecializedABIReturnRawInt:
			jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		case SpecializedABIReturnRawFloat:
		case SpecializedABIReturnRawTablePtr:
			emitBoxTablePtr(asm, jit.X0, jit.X0, jit.X1)
		}
		ec.storeResultNB(jit.X0, instr.ID)
		asm.B(doneLabel)
		asm.Label(nextLabel)
	}
	postReprs := ec.snapshotValueReprs()
	asm.B(fallbackLabel)

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
	asm.MOVimm16(jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeTier2Only)
	if allLeafCallees {
		ec.emitRestoreTypedPeerLeafCallerState(calleeBaseOff)
	} else {
		ec.emitRestoreTypedPeerCallerState()
	}
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitSpillSelectiveForCall(liveGPRs, nil)
	ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, argDesc)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)
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
	ec.emitMaterializeTypedPeerCallFrame(funcSlot, nArgs, argDesc)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitUnboxRawIntRegs(postReprs)
	ec.restoreValueReprSnapshot(postReprs)

	asm.Label(doneLabel)
	return true
}

func (ec *emitContext) emitCallNativeTypedSelfIfEligible(instr *Instr) bool {
	if !ec.isTypedStaticSelfCall(instr) {
		return false
	}

	asm := ec.asm
	funcSlot := int(instr.Aux)
	nArgs := len(instr.Args) - 1
	nRets := callResultCountFromAux2(instr.Aux2)
	abi := ec.typedSelfABI
	if abi.Return == SpecializedABIReturnNone {
		// Zero-result typed self recursion is side-effect-only. The native BL
		// path has no return value to validate before restoring the caller
		// frame, and it is not yet safe for mutating double-recursive specializations
		// over shared tables. Keep those calls on the boxed call-exit path.
		return false
	}
	wantRets := 1
	if nRets != wantRets || nArgs != abi.NumParams || len(abi.Params) != nArgs {
		return false
	}
	desc := CallABIDescriptor{
		Callee:    ec.fn.Proto,
		NumArgs:   nArgs,
		NumRets:   nRets,
		ParamReps: append([]SpecializedABIParamRep(nil), abi.Params...),
		ReturnRep: abi.Return,
	}
	ec.traceNativeCallEmit(instr, "typed self", ec.fn.Proto, &desc)

	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	preReprs := ec.snapshotValueReprs()
	ec.emitSpillSelectiveForCall(liveGPRs, liveFPRs)

	exitHandleLabel := ec.uniqueLabel("t2typedself_exit")
	fallbackLabel := ec.uniqueLabel("t2typedself_fallback")
	doneLabel := ec.uniqueLabel("t2typedself_done")
	frameSize := typedSelfFrameSizeFor(nArgs)

	// Keep only the data needed to reconstruct the public VM call frame on
	// fallback/exit. The success path passes typed args in X0..X3 and avoids
	// writing regs[funcSlot..] before the recursive BL.
	asm.SUBimm(jit.SP, jit.SP, uint16(frameSize))
	ec.emitLoadCallMode(jit.X8)
	asm.STR(jit.X8, jit.SP, typedSelfSavedCallModeOff)
	ec.emitTypedSelfArgsInRegsAndSave(instr, abi, fallbackLabel)

	calleeBaseOff := ec.nextSlot * jit.ValueSize
	calleeFrameBytes := ec.nextSlot * jit.ValueSize
	if calleeBaseOff+calleeFrameBytes <= 4095 {
		asm.ADDimm(jit.X8, mRegRegs, uint16(calleeBaseOff+calleeFrameBytes))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff+calleeFrameBytes))
		asm.ADDreg(jit.X8, mRegRegs, jit.X8)
	}
	asm.LDR(jit.X9, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X8, jit.X9)
	asm.BCond(jit.CondHI, fallbackLabel)

	asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.CMPimm(jit.X8, maxNativeCallDepth)
	asm.BCond(jit.CondGE, fallbackLabel)

	asm.MOVimm16(jit.X8, callModeTypedSelf)
	ec.emitStoreCallMode(jit.X8)

	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X8)
	}

	asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.ADDimm(jit.X8, jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)

	asm.BL("t2_typed_self_entry")

	asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	asm.SUBimm(jit.X8, jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)

	if calleeBaseOff <= 4095 {
		asm.SUBimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.SUBreg(mRegRegs, mRegRegs, jit.X8)
	}
	asm.LDR(jit.X8, jit.SP, typedSelfSavedCallModeOff)
	ec.emitStoreCallMode(jit.X8)

	asm.LDR(jit.X8, mRegCtx, execCtxOffExitCode)
	asm.CBNZ(jit.X8, exitHandleLabel)

	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)
	ec.emitUnboxRawIntRegs(preReprs)
	ec.restoreValueReprSnapshot(preReprs)
	switch abi.Return {
	case SpecializedABIReturnNone:
		// CALL C=1: recursive side effects are complete and no result slot is
		// produced or consumed.
	case SpecializedABIReturnRawInt:
		ec.storeRawInt(jit.X0, instr.ID)
	case SpecializedABIReturnRawFloat:
		asm.FMOVtoFP(jit.D0, jit.X0)
		ec.storeRawFloat(jit.D0, instr.ID)
	case SpecializedABIReturnRawTablePtr:
		emitBoxTablePtr(asm, jit.X0, jit.X0, jit.X1)
		ec.storeResultNB(jit.X0, instr.ID)
	default:
		asm.B(fallbackLabel)
	}
	postSuccessReprs := ec.snapshotValueReprs()
	asm.B(doneLabel)

	asm.Label(exitHandleLabel)
	ec.emitPushNativeCallExitFrameIfNested(jit.X8, jit.X9, jit.X10, jit.X11)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitCode)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeExitCode)
	asm.LDR(jit.X8, mRegCtx, execCtxOffResumeNumericPass)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePass)
	asm.LDR(jit.X8, mRegCtx, execCtxOffExitResumePC)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeResumePC)
	asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.MOVimm16(jit.X8, 1)
	asm.STR(jit.X8, mRegCtx, execCtxOffNativeCalleeTier2Only)
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitMaterializeTypedSelfCallFrameFromStack(funcSlot, nArgs, abi)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)

	asm.Label(fallbackLabel)
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitMaterializeTypedSelfCallFrameFromStack(funcSlot, nArgs, abi)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitUnboxRawIntRegs(postSuccessReprs)
	ec.restoreValueReprSnapshot(postSuccessReprs)

	asm.Label(doneLabel)
	return true
}

func (ec *emitContext) emitPushNativeCallExitFrameIfNested(tmpExit, tmpDepth, tmpFrame, tmpVal jit.Reg) {
	asm := ec.asm
	doneLabel := ec.uniqueLabel("native_exit_stack_done")
	overflowLabel := ec.uniqueLabel("native_exit_stack_overflow")

	asm.LDR(tmpExit, mRegCtx, execCtxOffExitCode)
	asm.CMPimm(tmpExit, uint16(ExitNativeCallExit))
	asm.BCond(jit.CondNE, doneLabel)

	asm.LDR(tmpDepth, mRegCtx, execCtxOffNativeCallExitStackDepth)
	asm.CMPimm(tmpDepth, maxNativeCallExitStackDepth)
	asm.BCond(jit.CondGE, overflowLabel)

	asm.LoadImm64(tmpFrame, int64(nativeCallExitFrameSize))
	asm.MUL(tmpFrame, tmpDepth, tmpFrame)
	if execCtxOffNativeCallExitStack <= 4095 {
		asm.ADDimm(tmpFrame, tmpFrame, uint16(execCtxOffNativeCallExitStack))
	} else {
		asm.LoadImm64(tmpVal, int64(execCtxOffNativeCallExitStack))
		asm.ADDreg(tmpFrame, tmpFrame, tmpVal)
	}
	asm.ADDreg(tmpFrame, mRegCtx, tmpFrame)

	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffCallSlot, nativeCallExitFrameOffCallSlot)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffCallNArgs, nativeCallExitFrameOffCallNArgs)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffCallNRets, nativeCallExitFrameOffCallNRets)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffCallID, nativeCallExitFrameOffCallID)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCallA, nativeCallExitFrameOffNativeCallA)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCallB, nativeCallExitFrameOffNativeCallB)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCallC, nativeCallExitFrameOffNativeCallC)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCalleeExitCode, nativeCallExitFrameOffNativeCalleeExitCode)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCalleeResumePass, nativeCallExitFrameOffNativeCalleeResumePass)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCalleeBaseOff, nativeCallExitFrameOffNativeCalleeBaseOff)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCalleeResumePC, nativeCallExitFrameOffNativeCalleeResumePC)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCalleeClosurePtr, nativeCallExitFrameOffNativeCalleeClosurePtr)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCalleeTier2Only, nativeCallExitFrameOffNativeCalleeTier2Only)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffNativeCallerClosurePtr, nativeCallExitFrameOffNativeCallerClosurePtr)
	ec.emitStoreNativeCallExitFrameField(tmpFrame, tmpVal, execCtxOffResumeNumericPass, nativeCallExitFrameOffResumeNumericPass)

	asm.ADDimm(tmpDepth, tmpDepth, 1)
	asm.STR(tmpDepth, mRegCtx, execCtxOffNativeCallExitStackDepth)
	asm.B(doneLabel)

	asm.Label(overflowLabel)
	asm.MOVimm16(tmpVal, 1)
	asm.STR(tmpVal, mRegCtx, execCtxOffNativeCallExitStackOverflow)

	asm.Label(doneLabel)
}

func (ec *emitContext) emitStoreNativeCallExitFrameField(frameReg, tmpReg jit.Reg, ctxOff, frameOff int) {
	ec.asm.LDR(tmpReg, mRegCtx, ctxOff)
	ec.asm.STR(tmpReg, frameReg, frameOff)
}

func (ec *emitContext) emitTypedSelfArgsInRegsAndSave(instr *Instr, abi TypedSelfABI, fallbackLabel string) {
	asm := ec.asm
	for i, rep := range abi.Params {
		dst := jit.Reg(int(jit.X0) + i)
		arg := instr.Args[1+i]
		switch rep {
		case SpecializedABIParamRawInt:
			src := ec.resolveRawInt(arg.ID, dst)
			if src != dst {
				asm.MOVreg(dst, src)
			}
			asm.STR(dst, jit.SP, typedSelfArgsOff+i*jit.ValueSize)
		case SpecializedABIParamRawTablePtr:
			src := ec.resolveValueNB(arg.ID, dst)
			if src != dst {
				asm.MOVreg(dst, src)
			}
			asm.STR(dst, jit.SP, typedSelfArgsOff+i*jit.ValueSize)
			if ec.irTypes[arg.ID] != TypeTable {
				jit.EmitCheckIsTableFull(asm, dst, jit.X6, jit.X7, fallbackLabel)
			}
			jit.EmitExtractPtr(asm, dst, dst)
			if fact, ok := ec.entryShapeGuards[i]; ok && fact.ShapeID != 0 {
				asm.LDRW(jit.X6, dst, jit.TableOffShapeID)
				asm.LoadImm64(jit.X7, int64(fact.ShapeID))
				asm.CMPreg(jit.X6, jit.X7)
				asm.BCond(jit.CondNE, fallbackLabel)
			}
		}
	}
}

func (ec *emitContext) emitMaterializeTypedSelfCallFrameFromStack(funcSlot, nArgs int, abi TypedSelfABI) {
	asm := ec.asm
	ec.emitBoxCurrentClosure(jit.X0, jit.X1)
	asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot))
	for i := 0; i < nArgs; i++ {
		asm.LDR(jit.X0, jit.SP, typedSelfArgsOff+i*jit.ValueSize)
		if i < len(abi.Params) && abi.Params[i] == SpecializedABIParamRawInt {
			jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		}
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot+1+i))
	}
}

func (ec *emitContext) emitRestoreTypedPeerCallerState() {
	ec.emitRestoreRawPeerCallerState()
	ec.asm.LDR(jit.X8, jit.SP, rawPeerCallModeOff)
	ec.emitStoreCallMode(jit.X8)
}

func (ec *emitContext) emitSpillTypedPeerLiveForSuccess(liveFPRs map[int]bool) {
	// The published typed-peer callee entry uses the full native frame and
	// preserves allocatable GPRs. Success paths only need to protect live FPRs;
	// exit/fallback paths still materialize GPR live values before returning
	// through Go.
	ec.emitSpillSelectiveForCall(nil, liveFPRs)
}

func (ec *emitContext) emitReloadTypedPeerLiveForSuccess(liveFPRs map[int]bool) {
	ec.emitReloadSelectiveForCall(nil, liveFPRs)
}

func (ec *emitContext) shouldUseTypedPeerClobberEntry(liveGPRs, liveFPRs map[int]bool) bool {
	// The clobber entry is profitable only when the caller has fewer live
	// registers to preserve than the callee would save in its compact typed
	// frame. Keep this conservative until per-callee clobber metadata is wired
	// into the call-site decision.
	if len(liveFPRs) != 0 {
		return false
	}
	return len(liveGPRs) <= 2
}

func fieldShapeTypedPeerMaxStack(cases []fieldShapeTypedPeerCallCase) int {
	maxStack := 0
	for _, c := range cases {
		if c.callee != nil && c.callee.MaxStack > maxStack {
			maxStack = c.callee.MaxStack
		}
	}
	return maxStack
}

func (ec *emitContext) emitTypedPeerMaxStackCheck(calleeBaseOff, maxCalleeStack int, fallbackLabel string) {
	asm := ec.asm
	asm.LoadImm64(jit.X8, int64(maxCalleeStack*jit.ValueSize))
	if calleeBaseOff <= 4095 {
		asm.ADDimm(jit.X8, jit.X8, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X12, int64(calleeBaseOff))
		asm.ADDreg(jit.X8, jit.X8, jit.X12)
	}
	asm.ADDreg(jit.X8, jit.X8, mRegRegs)
	asm.LDR(jit.X12, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X8, jit.X12)
	asm.BCond(jit.CondHI, fallbackLabel)
}

func (ec *emitContext) emitRestoreTypedPeerCallerModeClosureOnly() {
	asm := ec.asm
	asm.LDR(jit.X8, jit.SP, rawPeerClosureOff)
	asm.STR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.LDR(jit.X8, jit.SP, rawPeerCallModeOff)
	ec.emitStoreCallMode(jit.X8)
}

func (ec *emitContext) emitRestoreTypedPeerLeafCallerState(calleeBaseOff int) {
	ec.emitRestoreTypedPeerLeafCallerStateWithConsts(calleeBaseOff, true)
}

func (ec *emitContext) emitRestoreTypedPeerLeafCallerStateWithConsts(calleeBaseOff int, restoreConsts bool) {
	asm := ec.asm
	ec.emitRestoreRawPeerLeafCallerRegsWithConsts(calleeBaseOff, restoreConsts)
	asm.LDR(jit.X8, jit.SP, rawPeerClosureOff)
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
	asm.STR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.LDR(jit.X8, jit.SP, rawPeerCallModeOff)
	ec.emitStoreCallMode(jit.X8)
}

func (ec *emitContext) emitTypedPeerArgsInRegsAndSave(instr *Instr, desc CallABIDescriptor, fallbackLabel string) {
	if instr == nil || len(instr.Args) < 1 {
		ec.asm.B(fallbackLabel)
		return
	}
	ec.emitTypedPeerArgsFromValuesInRegsAndSave(instr.Args[1:], desc, fallbackLabel)
}

func (ec *emitContext) emitTypedPeerArgsFromValuesInRegsAndSave(args []*Value, desc CallABIDescriptor, fallbackLabel string) {
	ec.emitTypedPeerArgsFromValuesInRegsWithOptionalSave(args, desc, fallbackLabel, true)
}

func (ec *emitContext) emitTypedPeerArgsFromValuesInRegs(args []*Value, desc CallABIDescriptor, fallbackLabel string) {
	ec.emitTypedPeerArgsFromValuesInRegsWithOptionalSave(args, desc, fallbackLabel, false)
}

func (ec *emitContext) emitTypedPeerArgsFromValuesInRegsWithOptionalSave(args []*Value, desc CallABIDescriptor, fallbackLabel string, saveArgs bool) {
	asm := ec.asm
	for i, rep := range desc.ParamReps {
		if i >= len(args) || args[i] == nil {
			asm.B(fallbackLabel)
			return
		}
		dst := jit.Reg(int(jit.X0) + i)
		arg := args[i]
		switch rep {
		case SpecializedABIParamRawInt:
			if ec.irTypes[arg.ID] == TypeInt {
				src := ec.resolveRawInt(arg.ID, dst)
				if src != dst {
					asm.MOVreg(dst, src)
				}
			} else {
				src := ec.resolveValueNB(arg.ID, dst)
				if src != dst {
					asm.MOVreg(dst, src)
				}
				emitCheckIsIntPinned(asm, dst, jit.X6)
				asm.BCond(jit.CondNE, fallbackLabel)
				jit.EmitUnboxInt(asm, dst, dst)
			}
			if saveArgs {
				asm.STR(dst, jit.SP, rawPeerArgsOff+i*jit.ValueSize)
			}
		case SpecializedABIParamRawFloat:
			if ec.irTypes[arg.ID] == TypeFloat {
				src := ec.resolveRawFloat(arg.ID, jit.FReg(int(jit.D0)+i))
				asm.FMOVtoGP(dst, src)
			} else {
				src := ec.resolveValueNB(arg.ID, dst)
				if src != dst {
					asm.MOVreg(dst, src)
				}
				jit.EmitIsTaggedPinned(asm, dst, jit.X6, mRegTagInt)
				asm.BCond(jit.CondEQ, fallbackLabel)
			}
			if saveArgs {
				asm.STR(dst, jit.SP, rawPeerArgsOff+i*jit.ValueSize)
			}
		case SpecializedABIParamRawTablePtr:
			src := ec.resolveValueNB(arg.ID, dst)
			if src != dst {
				asm.MOVreg(dst, src)
			}
			if saveArgs {
				asm.STR(dst, jit.SP, rawPeerArgsOff+i*jit.ValueSize)
			}
			if ec.irTypes[arg.ID] != TypeTable {
				jit.EmitCheckIsTableFull(asm, dst, jit.X6, jit.X7, fallbackLabel)
			}
			jit.EmitExtractPtr(asm, dst, dst)
			if fact, ok := desc.ArgFacts[i]; ok && fact.ShapeID != 0 {
				asm.LDRW(jit.X6, dst, jit.TableOffShapeID)
				asm.LoadImm64(jit.X7, int64(fact.ShapeID))
				asm.CMPreg(jit.X6, jit.X7)
				asm.BCond(jit.CondNE, fallbackLabel)
			}
		default:
			asm.B(fallbackLabel)
		}
	}
}

func (ec *emitContext) emitTypedPeerABICheck(protoReg jit.Reg, desc CallABIDescriptor, fallbackLabel string) {
	sig := typedABISignature(TypedSelfABI{
		Eligible:  true,
		NumParams: desc.NumArgs,
		Params:    desc.ParamReps,
		Return:    desc.ReturnRep,
	})
	if sig == 0 {
		ec.asm.B(fallbackLabel)
		return
	}
	ec.asm.LDR(jit.X9, protoReg, funcProtoOffTier2TypedEntryABI)
	ec.asm.LoadImm64(jit.X8, int64(sig))
	ec.asm.CMPreg(jit.X9, jit.X8)
	ec.asm.BCond(jit.CondNE, fallbackLabel)
}

func (ec *emitContext) emitMaterializeTypedPeerCallFrame(funcSlot, nArgs int, desc CallABIDescriptor) {
	asm := ec.asm
	asm.LDR(jit.X0, jit.SP, rawPeerFuncOff)
	asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot))
	for i := 0; i < nArgs; i++ {
		asm.LDR(jit.X0, jit.SP, rawPeerArgsOff+i*jit.ValueSize)
		if i < len(desc.ParamReps) && desc.ParamReps[i] == SpecializedABIParamRawInt {
			jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		}
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot+1+i))
	}
}

func (ec *emitContext) emitMaterializeTypedPeerCallFrameFromValues(funcSlot int, args []*Value, funcReg jit.Reg) {
	asm := ec.asm
	asm.STR(funcReg, mRegRegs, slotOffset(funcSlot))
	for i, arg := range args {
		if arg == nil {
			continue
		}
		src := ec.resolveValueNB(arg.ID, jit.X0)
		if src != jit.X0 {
			asm.MOVreg(jit.X0, src)
		}
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot+1+i))
	}
}
