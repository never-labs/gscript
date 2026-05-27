//go:build darwin && arm64

// emit_call_native_rawint.go: raw-int self-call and peer-call ARM64 paths.
//
// Pure code-movement split of emit_call_native.go (zero behavior change).

package methodjit

import (
	"fmt"
	"sort"
	"unsafe"

	"github.com/gscript/gscript/internal/jit"
	"github.com/gscript/gscript/internal/vm"
)

const (
	rawPeerFrameSize   = 80
	rawPeerRegsOff     = 0
	rawPeerConstsOff   = 8
	rawPeerFuncOff     = 16
	rawPeerArgsOff     = 24
	rawPeerClosureOff  = 56
	rawPeerCallModeOff = 64
)

func rawSelfFrameSizeFor(nParams int) int {
	return rawSelfFrameSizeForLive(nParams, 0)
}

func rawSelfFrameSizeForLive(nParams, nLiveRaw int) int {
	size := rawSelfLiveSpillsOff(nParams) + nLiveRaw*jit.ValueSize
	return (size + 15) &^ 15
}

func rawSelfLiveSpillsOff(nParams int) int {
	return 0
}

type rawSelfLiveSpill struct {
	valueID  int
	reg      jit.Reg
	slot     int
	stackOff int
}

// emitCallNativeRawIntSelf emits the v1 raw-int self-recursive ABI. It is a
// dedicated static-self path rather than another branch inside emitCallNative:
// args enter the callee as raw ints in X0..X3, success returns raw int in X0,
// and every fallback materializes a normal boxed VM call frame before
// ExitCallExit.
func (ec *emitContext) emitCallNativeRawIntSelf(instr *Instr) {
	if !enableNumericSelfBL || ec.fn == nil || ec.fn.Proto == nil || !ec.isNumericStaticSelfCall(instr) {
		ec.emitCallNativeStaticSelfFast(instr)
		return
	}
	ec.traceNativeCallEmit(instr, "raw-int self", ec.fn.Proto, nil)

	asm := ec.asm
	funcSlot := int(instr.Aux)
	nArgs := len(instr.Args) - 1
	nRets := callResultCountFromAux2(instr.Aux2)
	nParams := ec.fn.Proto.NumParams
	if nArgs != nParams || nParams < 1 || nParams > 4 {
		ec.emitCallNativeStaticSelfFast(instr)
		return
	}

	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)

	preCallSlowLabel := ec.uniqueLabel("t2rawself_slow")
	exitLabel := ec.uniqueLabel("t2rawself_exit")
	fallbackLabel := ec.uniqueLabel("t2rawself_fallback")
	doneLabel := ec.uniqueLabel("t2rawself_done")

	preReprs := ec.snapshotValueReprs()
	rawLiveSpills := ec.rawSelfLiveSpills(liveGPRs, nParams)
	boxedLiveGPRs := liveGPRs
	if len(rawLiveSpills) > 0 {
		boxedLiveGPRs = cloneBoolMap(liveGPRs)
		for _, spill := range rawLiveSpills {
			delete(boxedLiveGPRs, spill.valueID)
		}
	}
	boxedRawReloads := preReprs.rawIntSubset(boxedLiveGPRs)
	if len(boxedLiveGPRs) > 0 || len(liveFPRs) > 0 {
		ec.emitSpillSelectiveForCall(boxedLiveGPRs, liveFPRs)
	}

	// Raw-call frame:
	//   0..       raw live GPR spills that survive the BL on the success path
	//
	// The caller's own entry frame already owns FP/LR, and raw-int self calls
	// stay within one proto/closure/constant domain. The callee base is always
	// caller base + calleeBaseOff, so the successful and callee-exit paths
	// restore mRegRegs with offset arithmetic instead of saving it in the shim
	// frame. Successful raw calls keep ctx.Regs lazy; numeric exit epilogues and
	// raw-call fallback paths publish the current base and materialize raw live
	// spills into boxed VM homes before Go observes the context. Pre-call
	// fallback rebuilds args directly from X0..X3, while callee exits use the
	// native-call-exit descriptor and no longer need saved raw args to replay
	// the call. The boxed function operand is rebuilt from BaselineClosurePtr;
	// static self recursion cannot change closure identity while this native
	// frame is executing. Numeric entries return a status in X16 (0 = success,
	// non-zero = ctx.ExitCode), so raw self calls leave ctx.CallMode unchanged
	// and avoid the per-call ExitCode load on success.
	rawFrameSize := rawSelfFrameSizeForLive(nParams, len(rawLiveSpills))

	ec.emitNumericArgsInRegs(instr, nParams)
	ec.emitAllocRawSelfFrame(rawFrameSize)
	ec.emitSaveRawSelfLiveSpills(rawLiveSpills)

	calleeBaseOff := ec.nextSlot * jit.ValueSize
	calleeFrameBytes := ec.nextSlot * jit.ValueSize
	useRawSelfRegsBudget := nParams >= 2

	if !useRawSelfRegsBudget {
		asm.LDR(jit.X7, mRegCtx, execCtxOffNativeCallDepth)
		asm.CMPimm(jit.X7, maxRawSelfCallDepth)
		asm.BCond(jit.CondGE, preCallSlowLabel)
	}

	if calleeBaseOff+calleeFrameBytes <= 4095 {
		asm.ADDimm(jit.X8, mRegRegs, uint16(calleeBaseOff+calleeFrameBytes))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff+calleeFrameBytes))
		asm.ADDreg(jit.X8, mRegRegs, jit.X8)
	}
	if useRawSelfRegsBudget {
		asm.LDR(jit.X9, mRegCtx, execCtxOffRawSelfRegsEnd)
	} else {
		asm.LDR(jit.X9, mRegCtx, execCtxOffRegsEnd)
	}
	asm.CMPreg(jit.X8, jit.X9)
	asm.BCond(jit.CondHI, preCallSlowLabel)

	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X8)
	}

	if !useRawSelfRegsBudget {
		asm.ADDimm(jit.X7, jit.X7, 1)
		asm.STR(jit.X7, mRegCtx, execCtxOffNativeCallDepth)
	}

	asm.BL(fmt.Sprintf("t2_numeric_self_entry_%d", nParams))

	if !useRawSelfRegsBudget {
		asm.LDR(jit.X7, mRegCtx, execCtxOffNativeCallDepth)
		asm.SUBimm(jit.X7, jit.X7, 1)
		asm.STR(jit.X7, mRegCtx, execCtxOffNativeCallDepth)
	}

	asm.CBNZ(jit.X16, exitLabel)

	ec.emitRestoreRawSelfCallerRegsFromCalleeBase(calleeBaseOff)
	ec.emitReloadRawSelfLiveSpills(rawLiveSpills)
	ec.emitFreeRawSelfFrame(rawFrameSize)
	ec.emitReloadSelectiveForCall(boxedLiveGPRs, liveFPRs)
	ec.emitUnboxRawIntRegs(boxedRawReloads)
	ec.restoreValueReprSnapshot(preReprs)
	ec.storeRawInt(jit.X0, instr.ID)
	postReprs := ec.snapshotValueReprs()
	asm.B(doneLabel)

	asm.Label(exitLabel)
	ec.emitPushNativeCallExitFrameIfNested(jit.X7, jit.X8, jit.X9, jit.X10)
	asm.LDR(jit.X7, mRegCtx, execCtxOffExitCode)
	asm.STR(jit.X7, mRegCtx, execCtxOffNativeCalleeExitCode)
	asm.LDR(jit.X7, mRegCtx, execCtxOffResumeNumericPass)
	asm.STR(jit.X7, mRegCtx, execCtxOffNativeCalleeResumePass)
	asm.LDR(jit.X7, mRegCtx, execCtxOffExitResumePC)
	asm.STR(jit.X7, mRegCtx, execCtxOffNativeCalleeResumePC)
	asm.LDR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X7, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.MOVimm16(jit.X7, 1)
	asm.STR(jit.X7, mRegCtx, execCtxOffNativeCalleeTier2Only)
	ec.emitRestoreRawSelfCallerRegsFromCalleeBase(calleeBaseOff)
	ec.emitPublishRawSelfCallerState()
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitMaterializeRawSelfLiveSpills(rawLiveSpills)
	ec.emitMaterializeRawIntSelfFunctionFromSelfClosure(funcSlot)
	ec.emitFreeRawSelfFrame(rawFrameSize)
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)

	asm.Label(preCallSlowLabel)
	ec.emitPublishRawSelfCallerState()

	asm.Label(fallbackLabel)
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitMaterializeRawSelfLiveSpills(rawLiveSpills)
	ec.emitMaterializeRawIntSelfCallFrameFromArgRegs(funcSlot, nArgs)
	ec.emitFreeRawSelfFrame(rawFrameSize)
	ec.emitRawIntSelfCallExitResume(instr, funcSlot, nArgs, nRets, preReprs, liveGPRs, liveFPRs)
	ec.restoreValueReprSnapshot(postReprs)

	asm.Label(doneLabel)
}

func (ec *emitContext) rawSelfLiveSpills(gprLive map[int]bool, nParams int) []rawSelfLiveSpill {
	if len(gprLive) == 0 {
		return nil
	}
	ids := make([]int, 0, len(gprLive))
	for valueID := range gprLive {
		if ec.valueReprOf(valueID) != valueReprRawInt {
			continue
		}
		pr, ok := ec.alloc.ValueRegs[valueID]
		if !ok || pr.IsFloat {
			continue
		}
		if _, active := ec.activeRegs[valueID]; !active {
			continue
		}
		if _, ok := ec.slotMap[valueID]; !ok {
			continue
		}
		ids = append(ids, valueID)
	}
	if len(ids) == 0 {
		return nil
	}
	sort.Ints(ids)
	spills := make([]rawSelfLiveSpill, 0, len(ids))
	stackOff := rawSelfLiveSpillsOff(nParams)
	for _, valueID := range ids {
		pr := ec.alloc.ValueRegs[valueID]
		spills = append(spills, rawSelfLiveSpill{
			valueID:  valueID,
			reg:      jit.Reg(pr.Reg),
			slot:     ec.slotMap[valueID],
			stackOff: stackOff,
		})
		stackOff += jit.ValueSize
	}
	return spills
}

func (ec *emitContext) emitRestoreRawSelfCallerRegsFromCalleeBase(calleeBaseOff int) {
	asm := ec.asm
	if calleeBaseOff <= 4095 {
		asm.SUBimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.SUBreg(mRegRegs, mRegRegs, jit.X8)
	}
}

func (ec *emitContext) emitAllocRawSelfFrame(rawFrameSize int) {
	if rawFrameSize > 0 {
		ec.asm.SUBimm(jit.SP, jit.SP, uint16(rawFrameSize))
	}
}

func (ec *emitContext) emitFreeRawSelfFrame(rawFrameSize int) {
	if rawFrameSize > 0 {
		ec.asm.ADDimm(jit.SP, jit.SP, uint16(rawFrameSize))
	}
}

func (ec *emitContext) emitSaveRawSelfLiveSpills(spills []rawSelfLiveSpill) {
	for _, spill := range spills {
		ec.asm.STR(spill.reg, jit.SP, spill.stackOff)
	}
}

func (ec *emitContext) emitReloadRawSelfLiveSpills(spills []rawSelfLiveSpill) {
	for _, spill := range spills {
		ec.asm.LDR(spill.reg, jit.SP, spill.stackOff)
	}
}

func (ec *emitContext) emitMaterializeRawSelfLiveSpills(spills []rawSelfLiveSpill) {
	for _, spill := range spills {
		ec.asm.LDR(jit.X10, jit.SP, spill.stackOff)
		jit.EmitBoxIntFast(ec.asm, jit.X10, jit.X10, mRegTagInt)
		ec.asm.STR(jit.X10, mRegRegs, slotOffset(spill.slot))
		ec.emitExitResumeCheckShadowStoreGPR(spill.slot, jit.X10)
	}
}

func (ec *emitContext) emitRestoreRawSelfCallerStateFromCalleeBase(calleeBaseOff int) {
	ec.emitRestoreRawSelfCallerRegsFromCalleeBase(calleeBaseOff)
	ec.emitPublishRawSelfCallerState()
}

func (ec *emitContext) emitPublishRawSelfCallerState() {
	ec.asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
}

func (ec *emitContext) emitBoxCurrentClosure(dst, scratch jit.Reg) {
	ec.asm.LDR(dst, mRegCtx, execCtxOffBaselineClosurePtr)
	ec.asm.UBFX(dst, dst, 0, 44)
	ec.asm.LoadImm64(scratch, nbClosureTagBits)
	ec.asm.ORRreg(dst, dst, scratch)
}

func (ec *emitContext) emitMaterializeRawIntSelfFunctionFromSelfClosure(funcSlot int) {
	asm := ec.asm
	ec.emitBoxCurrentClosure(jit.X10, jit.X11)
	asm.STR(jit.X10, mRegRegs, slotOffset(funcSlot))
	ec.emitExitResumeCheckShadowStoreGPR(funcSlot, jit.X10)
}

func (ec *emitContext) emitMaterializeRawIntSelfCallFrameFromArgRegs(funcSlot, nArgs int) {
	asm := ec.asm
	ec.emitMaterializeRawIntSelfFunctionFromSelfClosure(funcSlot)
	for i := 0; i < nArgs; i++ {
		argReg := jit.Reg(int(jit.X0) + i)
		jit.EmitBoxIntFast(asm, jit.X10, argReg, mRegTagInt)
		asm.STR(jit.X10, mRegRegs, slotOffset(funcSlot+1+i))
		ec.emitExitResumeCheckShadowStoreGPR(funcSlot+1+i, jit.X10)
	}
}

func (ec *emitContext) emitRawIntSelfCallExitResume(instr *Instr, funcSlot, nArgs, nRets int, preReprs valueReprSnapshot, liveGPRs, liveFPRs map[int]bool) {
	asm := ec.asm

	ec.recordExitResumeCheckSiteWithLive(
		instr,
		ExitCallExit,
		ec.exitResumeCheckLiveSlots(liveGPRs, liveFPRs),
		callExitModifiedSlots(funcSlot, nRets),
		exitResumeCheckOptions{RequireCallFunc: true, RequireRawIntArgs: true},
	)

	ec.emitStoreCallExitDescriptor(callExitDescriptor{
		slot:    funcSlot,
		nArgs:   nArgs,
		nRets:   nRets,
		instrID: instr.ID,
	})
	ec.emitCallProtocolExitToGo(ExitCallExit)

	continueLabel := ec.passLabel(fmt.Sprintf("call_continue_%d", instr.ID))
	asm.Label(continueLabel)

	ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)
	ec.emitUnboxRawIntRegs(preReprs)
	ec.restoreValueReprSnapshot(preReprs)
	asm.LDR(jit.X0, mRegRegs, slotOffset(funcSlot))
	resultIntLabel := ec.uniqueLabel("t2rawself_result_int")
	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X2, jit.NB_TagIntShr48)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondEQ, resultIntLabel)
	asm.LoadImm64(jit.X1, int64(ExitDeopt))
	asm.STR(jit.X1, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}
	asm.Label(resultIntLabel)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	ec.storeRawInt(jit.X0, instr.ID)

	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}

func (ec *emitContext) emitCallNativeRawIntPeerIfEligible(instr *Instr) bool {
	callee := ec.rawIntPeerCallee(instr)
	if callee == nil {
		return false
	}
	nArgs := len(instr.Args) - 1
	nRets := callResultCountFromAux2(instr.Aux2)
	if nRets != 1 || nArgs != callee.NumParams || nArgs < 1 || nArgs > 4 {
		return false
	}
	if ec.fn != nil {
		if desc, ok := ec.fn.Analysis.CallFacts().CallABI(instr.ID); ok {
			ec.traceNativeCallEmit(instr, "raw-int peer", callee, &desc)
		} else {
			ec.traceNativeCallEmit(instr, "raw-int peer", callee, nil)
		}
	} else {
		ec.traceNativeCallEmit(instr, "raw-int peer", callee, nil)
	}

	asm := ec.asm
	funcSlot := int(instr.Aux)
	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillTypedPeerLiveForSuccess(liveFPRs)

	fallbackLabel := ec.uniqueLabel("t2rawpeer_fallback")
	exitLabel := ec.uniqueLabel("t2rawpeer_exit")
	materializeLabel := ec.uniqueLabel("t2rawpeer_materialize")
	doneLabel := ec.uniqueLabel("t2rawpeer_done")
	preReprs := ec.snapshotValueReprs()
	leafCallee := rawIntPeerLeafCallee(callee)

	asm.SUBimm(jit.SP, jit.SP, rawPeerFrameSize)
	if !leafCallee {
		asm.STR(mRegRegs, jit.SP, rawPeerRegsOff)
		asm.STR(mRegConsts, jit.SP, rawPeerConstsOff)
		asm.LDR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
		asm.STR(jit.X8, jit.SP, rawPeerClosureOff)
	}

	fnReg := ec.resolveValueNB(instr.Args[0].ID, jit.X6)
	if fnReg != jit.X6 {
		asm.MOVreg(jit.X6, fnReg)
	}
	asm.STR(jit.X6, jit.SP, rawPeerFuncOff)

	ec.emitNumericArgsInRegs(instr, nArgs)
	for i := 0; i < nArgs; i++ {
		argReg := jit.Reg(int(jit.X0) + i)
		asm.STR(argReg, jit.SP, rawPeerArgsOff+i*jit.ValueSize)
	}

	// Guard the static callee identity. Stable globals make this hot-path
	// predictable, but the guard keeps rebinding and cache invalidation safe.
	asm.LSRimm(jit.X7, jit.X6, uint8(nbPtrSubShift))
	asm.LoadImm64(jit.X8, int64((jit.NB_TagPtrShr48<<4)|nbPtrSubVMClosure))
	asm.CMPreg(jit.X7, jit.X8)
	asm.BCond(jit.CondNE, fallbackLabel)
	jit.EmitExtractPtr(asm, jit.X7, jit.X6)
	if !leafCallee {
		asm.STR(jit.X7, mRegCtx, execCtxOffBaselineClosurePtr)
	}
	asm.LDR(jit.X7, jit.X7, vmClosureOffProto)
	asm.LoadImm64(jit.X8, int64(uintptr(unsafe.Pointer(callee))))
	asm.CMPreg(jit.X7, jit.X8)
	asm.BCond(jit.CondNE, fallbackLabel)
	asm.LDR(jit.X16, jit.X7, funcProtoOffTier2NumericEntryPtr)
	asm.CBZ(jit.X16, fallbackLabel)

	if !leafCallee {
		asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		asm.CMPimm(jit.X8, maxNativeCallDepth)
		asm.BCond(jit.CondGE, fallbackLabel)
	}

	calleeBaseOff := ec.nextSlot * jit.ValueSize
	asm.LoadImm64(jit.X8, int64(callee.MaxStack*jit.ValueSize))
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

	// The raw peer path only exists after the callee has published a Tier 2
	// numeric entry, so this hot call no longer needs to feed tiering counters.
	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X8)
	}
	asm.LDR(mRegConsts, jit.X7, funcProtoOffConstants)

	if !leafCallee {
		asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		asm.ADDimm(jit.X8, jit.X8, 1)
		asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		if callee.NumParams >= 2 {
			ec.emitSetRawSelfRegsEnd(mRegRegs, callee.MaxStack, jit.X8, jit.X9)
		}
	}
	asm.BLR(jit.X16)
	if !leafCallee {
		asm.LDR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
		asm.SUBimm(jit.X8, jit.X8, 1)
		asm.STR(jit.X8, mRegCtx, execCtxOffNativeCallDepth)
	}

	asm.CBNZ(jit.X16, exitLabel)

	// Numeric entries return raw int64 in X0 on ExitNormal. Boxed fallback
	// results are handled by emitRawIntPeerCallExitResume.
	if leafCallee {
		ec.emitRestoreRawPeerLeafCallerRegs(calleeBaseOff)
	} else {
		ec.emitRestoreRawPeerCallerState()
	}
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)
	ec.emitUnboxRawIntRegs(preReprs)
	ec.restoreValueReprSnapshot(preReprs)
	ec.storeRawInt(jit.X0, instr.ID)
	postReprs := ec.snapshotValueReprs()
	asm.B(doneLabel)

	asm.Label(exitLabel)
	if leafCallee {
		ec.emitRestoreRawPeerLeafCallerRegs(calleeBaseOff)
	} else {
		ec.emitRestoreRawPeerCallerState()
	}
	asm.B(materializeLabel)

	asm.Label(fallbackLabel)
	if !leafCallee {
		ec.emitRestoreRawPeerCallerState()
	}
	asm.Label(materializeLabel)
	ec.restoreValueReprSnapshot(preReprs)
	ec.emitMaterializeRawIntPeerCallFrame(funcSlot, nArgs)
	asm.ADDimm(jit.SP, jit.SP, rawPeerFrameSize)
	ec.emitRawIntPeerCallExitResume(instr, funcSlot, nArgs, nRets, preReprs, liveGPRs, liveFPRs)
	ec.restoreValueReprSnapshot(postReprs)

	asm.Label(doneLabel)
	return true
}

func rawIntPeerLeafCallee(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	// Leaf raw-int specializations cannot recursively grow the native BLR chain. They
	// still use one numeric entry frame, but repeated loop calls do not stack,
	// so the per-call NativeCallDepth load/store traffic is unnecessary.
	for _, inst := range proto.Code {
		if vm.DecodeOp(inst) == vm.OP_CALL {
			return false
		}
	}
	return true
}

func (ec *emitContext) rawIntPeerCallee(instr *Instr) *vm.FuncProto {
	if instr == nil || ec.fn == nil || instr.Type != TypeInt {
		return nil
	}
	if !ec.inLoopBlock() && !(ec.numericMode && ec.rawIntSelfABI.Eligible) {
		return nil
	}
	if ec.tailCallInstrs[instr.ID] || ec.isStaticSelfCall(instr) {
		return nil
	}
	if len(instr.Args) < 2 {
		return nil
	}
	desc, ok := ec.fn.Analysis.CallFacts().CallABI(instr.ID)
	if !ok || desc.Callee == nil || !desc.RawIntReturn || desc.NumRets != 1 {
		return nil
	}
	callee := desc.Callee
	if protoHasNativeCallUnsafeTableBytecode(callee) {
		return nil
	}
	ok, numParams := qualifyForNumeric(callee)
	if !ok || desc.NumArgs != numParams || len(desc.RawIntParams) != numParams || len(instr.Args) != 1+numParams {
		return nil
	}
	for i := 0; i < numParams; i++ {
		if !desc.RawIntParams[i] {
			return nil
		}
		argID := instr.Args[1+i].ID
		if ec.hasReg(argID) && ec.valueReprOf(argID) == valueReprRawInt {
			continue
		}
		if ec.irTypes[argID] == TypeInt {
			continue
		}
		return nil
	}
	return callee
}

func (ec *emitContext) emitRestoreRawPeerCallerState() {
	asm := ec.asm
	asm.LDR(mRegRegs, jit.SP, rawPeerRegsOff)
	asm.LDR(mRegConsts, jit.SP, rawPeerConstsOff)
	asm.LDR(jit.X8, jit.SP, rawPeerClosureOff)
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
	asm.STR(mRegConsts, mRegCtx, execCtxOffConstants)
	asm.STR(jit.X8, mRegCtx, execCtxOffBaselineClosurePtr)
}

func (ec *emitContext) emitRestoreRawPeerLeafCallerRegs(calleeBaseOff int) {
	ec.emitRestoreRawPeerLeafCallerRegsWithConsts(calleeBaseOff, true)
}

func (ec *emitContext) emitRestoreRawPeerLeafCallerRegsWithConsts(calleeBaseOff int, restoreConsts bool) {
	asm := ec.asm
	if calleeBaseOff <= 4095 {
		asm.SUBimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X8, int64(calleeBaseOff))
		asm.SUBreg(mRegRegs, mRegRegs, jit.X8)
	}
	if restoreConsts {
		asm.LDR(mRegConsts, mRegCtx, execCtxOffConstants)
	}
}

func (ec *emitContext) emitMaterializeRawIntPeerCallFrame(funcSlot, nArgs int) {
	asm := ec.asm
	asm.LDR(jit.X0, jit.SP, rawPeerFuncOff)
	asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot))
	for i := 0; i < nArgs; i++ {
		asm.LDR(jit.X0, jit.SP, rawPeerArgsOff+i*jit.ValueSize)
		jit.EmitBoxIntFast(asm, jit.X0, jit.X0, mRegTagInt)
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot+1+i))
	}
}

func (ec *emitContext) emitRawIntPeerCallExitResume(instr *Instr, funcSlot, nArgs, nRets int, preReprs valueReprSnapshot, liveGPRs, liveFPRs map[int]bool) {
	asm := ec.asm

	ec.recordExitResumeCheckSiteWithLive(
		instr,
		ExitCallExit,
		ec.exitResumeCheckLiveSlots(liveGPRs, liveFPRs),
		callExitModifiedSlots(funcSlot, nRets),
		exitResumeCheckOptions{RequireCallFunc: true, RequireRawIntArgs: true},
	)

	ec.emitStoreCallExitDescriptor(callExitDescriptor{
		slot:    funcSlot,
		nArgs:   nArgs,
		nRets:   nRets,
		instrID: instr.ID,
	})
	ec.emitCallProtocolExitToGo(ExitCallExit)

	continueLabel := ec.passLabel(fmt.Sprintf("call_continue_%d", instr.ID))
	asm.Label(continueLabel)

	ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)
	ec.emitUnboxRawIntRegs(preReprs)
	ec.restoreValueReprSnapshot(preReprs)
	asm.LDR(jit.X0, mRegRegs, slotOffset(funcSlot))
	resultIntLabel := ec.uniqueLabel("t2rawpeer_result_int")
	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X2, jit.NB_TagIntShr48)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondEQ, resultIntLabel)
	asm.LoadImm64(jit.X1, int64(ExitDeopt))
	asm.STR(jit.X1, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}
	asm.Label(resultIntLabel)
	jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	ec.storeRawInt(jit.X0, instr.ID)

	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}
