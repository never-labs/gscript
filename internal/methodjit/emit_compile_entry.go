//go:build darwin && arm64

package methodjit

import (
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
)

// emitTier2EntryMark writes 1 to proto.EnteredTier2 (one byte). It is
// called at the head of each Tier 2 entry point so that a single glance
// at proto.EnteredTier2 (for example through -jit-stats or diagnostics)
// answers "did native Tier 2 code actually run for this proto?". Uses
// X16/X17 — AAPCS scratch registers (IP0/IP1) — which are safe at entry
// before any callee-saved registers are live. Cost: roughly six instructions
// per invocation (LoadImm64 up to 4 + MOVimm16 + STRB), small enough for
// recursive hot paths while keeping diagnostics explicit.
//
// The address of proto.EnteredTier2 is stable because Go's GC is
// non-moving for heap allocations; FuncProto is heap-allocated and is
// kept alive by the owning VM/Closure for the lifetime of the code.
func (ec *emitContext) emitTier2EntryMark() {
	if ec.fn == nil || ec.fn.Proto == nil {
		return
	}
	asm := ec.asm
	addr := int64(uintptr(unsafe.Pointer(&ec.fn.Proto.EnteredTier2)))
	asm.LoadImm64(jit.X16, addr)
	asm.MOVimm16(jit.X17, 1)
	asm.STRB(jit.X17, jit.X16, 0)
}

func (ec *emitContext) typedSelfAfterParamsLabel() string {
	return "t2_typed_self_after_params"
}

func (ec *emitContext) emitTypedSelfEntry() {
	asm := ec.asm
	asm.Label("t2_typed_self_entry")
	ec.emitTier2EntryMark()
	asm.SUBimm(jit.SP, jit.SP, uint16(ec.typedSelfFrameSize()))
	asm.STP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.X29, jit.SP, 0)
	ec.emitSaveTypedSelfFrameRegs()
	asm.B("t2_typed_entry_params")
}

func (ec *emitContext) emitTypedPeerClobberEntry() {
	asm := ec.asm
	asm.Label("t2_typed_peer_clobber_entry")
	ec.emitTier2EntryMark()
	asm.SUBimm(jit.SP, jit.SP, 16)
	asm.STP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.X29, jit.SP, 0)
	asm.B("t2_typed_entry_params")
}

func (ec *emitContext) emitTypedEntryParamsLabel() {
	ec.asm.Label("t2_typed_entry_params")
	ec.emitTypedEntryParams()
}

func (ec *emitContext) typedPeerClobberEntryEnabled() bool {
	if ec == nil {
		return false
	}
	return typedPeerClobberABIEnabled(ec.typedSelfABI)
}

func typedPeerClobberABIEnabled(abi TypedSelfABI) bool {
	return abi.Eligible &&
		len(abi.Params) == 2 &&
		abi.Params[0] == SpecializedABIParamRawTablePtr &&
		abi.Params[1] == SpecializedABIParamRawInt
}

func (ec *emitContext) emitTypedEntryParams() {
	asm := ec.asm
	entryParamLoads := ec.typedSelfEntryParamLoads(ec.fn.Entry)
	for i, rep := range ec.typedSelfABI.Params {
		src := jit.Reg(int(jit.X0) + i)
		load, hasLoad := ec.entryParamLoad(i)
		hasLoad = hasLoad && entryParamLoads != nil && entryParamLoads[i]
		switch rep {
		case SpecializedABIParamRawInt:
			jit.EmitBoxIntFast(asm, jit.X16, src, mRegTagInt)
			asm.STR(jit.X16, mRegRegs, slotOffset(i))
			if hasLoad {
				if pr, ok := ec.alloc.ValueRegs[load.ID]; ok && !pr.IsFloat {
					dst := jit.Reg(pr.Reg)
					if load.Type == TypeInt {
						if src != dst {
							asm.MOVreg(dst, src)
						}
					} else if dst != jit.X16 {
						asm.MOVreg(dst, jit.X16)
					}
				}
			}
		case SpecializedABIParamRawFloat:
			asm.STR(src, mRegRegs, slotOffset(i))
			if hasLoad {
				if pr, ok := ec.alloc.ValueRegs[load.ID]; ok {
					if pr.IsFloat {
						asm.FMOVtoFP(jit.FReg(pr.Reg), src)
					} else {
						dst := jit.Reg(pr.Reg)
						if dst != src {
							asm.MOVreg(dst, src)
						}
					}
				}
			}
		case SpecializedABIParamRawTablePtr:
			emitBoxTablePtr(asm, jit.X16, src, jit.X17)
			asm.STR(jit.X16, mRegRegs, slotOffset(i))
			if hasLoad {
				if pr, ok := ec.alloc.ValueRegs[load.ID]; ok && !pr.IsFloat {
					dst := jit.Reg(pr.Reg)
					if dst != jit.X16 {
						asm.MOVreg(dst, jit.X16)
					}
				}
			}
		}
	}
	if entryParamLoads != nil {
		asm.B(ec.typedSelfAfterParamsLabel())
	} else {
		asm.B(ec.entryBlockLabel())
	}
}

func (ec *emitContext) emitTypedPeerClobberRestoreAndReturn() {
	asm := ec.asm
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, 16)
	asm.RET()
}

func (ec *emitContext) emitTypedSelfRawIntReturnEpilogue() {
	ec.asm.Label("t2_typed_self_raw_int_epilogue")
	ec.asm.MOVimm16(jit.X16, 0)
	ec.asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	ec.emitTypedSelfFrameRestoreAndReturn()
}

func (ec *emitContext) emitTypedPeerClobberRawIntReturnEpilogue() {
	ec.asm.Label("t2_typed_peer_clobber_raw_int_epilogue")
	ec.asm.MOVimm16(jit.X16, 0)
	ec.asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	ec.emitTypedPeerClobberRestoreAndReturn()
}

func (ec *emitContext) emitTypedSelfRawFloatReturnEpilogue() {
	ec.asm.Label("t2_typed_self_raw_float_epilogue")
	ec.asm.MOVimm16(jit.X16, 0)
	ec.asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	ec.emitTypedSelfFrameRestoreAndReturn()
}

func (ec *emitContext) emitTypedPeerClobberRawFloatReturnEpilogue() {
	ec.asm.Label("t2_typed_peer_clobber_raw_float_epilogue")
	ec.asm.MOVimm16(jit.X16, 0)
	ec.asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	ec.emitTypedPeerClobberRestoreAndReturn()
}

func (ec *emitContext) emitTypedSelfReturnEpilogue() {
	asm := ec.asm
	asm.Label("t2_typed_self_epilogue")
	failLabel := ec.uniqueLabel("typed_self_return_fail")
	doneLabel := ec.uniqueLabel("typed_self_return_done")

	switch ec.typedSelfABI.Return {
	case SpecializedABIReturnNone:
		// Zero-result typed self calls return only status; X0 is ignored by
		// the caller and CALL C=1 must not fabricate a result slot.
	case SpecializedABIReturnRawInt:
		emitCheckIsIntPinned(asm, jit.X0, jit.X1)
		asm.BCond(jit.CondNE, failLabel)
		jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	case SpecializedABIReturnRawFloat:
		asm.LSRimm(jit.X1, jit.X0, 48)
		asm.MOVimm16(jit.X2, jit.NB_TagNilShr48)
		asm.CMPreg(jit.X1, jit.X2)
		asm.BCond(jit.CondGE, failLabel)
	case SpecializedABIReturnRawTablePtr:
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, failLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	default:
		asm.B(failLabel)
	}
	asm.MOVimm16(jit.X16, 0)
	asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	asm.B(doneLabel)

	asm.Label(failLabel)
	asm.LoadImm64(jit.X16, int64(ExitDeopt))
	asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)

	asm.Label(doneLabel)
	ec.emitTypedSelfFrameRestoreAndReturn()
}

func (ec *emitContext) emitTypedPeerClobberReturnEpilogue() {
	asm := ec.asm
	asm.Label("t2_typed_peer_clobber_epilogue")
	failLabel := ec.uniqueLabel("typed_peer_clobber_return_fail")
	doneLabel := ec.uniqueLabel("typed_peer_clobber_return_done")

	switch ec.typedSelfABI.Return {
	case SpecializedABIReturnNone:
	case SpecializedABIReturnRawInt:
		emitCheckIsIntPinned(asm, jit.X0, jit.X1)
		asm.BCond(jit.CondNE, failLabel)
		jit.EmitUnboxInt(asm, jit.X0, jit.X0)
	case SpecializedABIReturnRawFloat:
		asm.LSRimm(jit.X1, jit.X0, 48)
		asm.MOVimm16(jit.X2, jit.NB_TagNilShr48)
		asm.CMPreg(jit.X1, jit.X2)
		asm.BCond(jit.CondGE, failLabel)
	case SpecializedABIReturnRawTablePtr:
		jit.EmitCheckIsTableFull(asm, jit.X0, jit.X1, jit.X2, failLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0)
	default:
		asm.B(failLabel)
	}
	asm.MOVimm16(jit.X16, 0)
	asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	asm.B(doneLabel)

	asm.Label(failLabel)
	asm.LoadImm64(jit.X16, int64(ExitDeopt))
	asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)

	asm.Label(doneLabel)
	ec.emitTypedPeerClobberRestoreAndReturn()
}

func (ec *emitContext) emitSaveTypedSelfFrameRegs() {
	gprs, fprs := ec.typedSelfSavedRegs()
	off := 16
	for i := 0; i < len(gprs); {
		r0 := jit.Reg(gprs[i])
		if i+1 < len(gprs) {
			ec.asm.STP(r0, jit.Reg(gprs[i+1]), jit.SP, off)
			off += 16
			i += 2
			continue
		}
		ec.asm.STR(r0, jit.SP, off)
		off += 8
		i++
	}
	off = (off + 15) &^ 15
	for i := 0; i < len(fprs); {
		r0 := jit.FReg(fprs[i])
		if i+1 < len(fprs) {
			ec.asm.FSTP(r0, jit.FReg(fprs[i+1]), jit.SP, off)
			off += 16
			i += 2
			continue
		}
		ec.asm.FSTRd(r0, jit.SP, off)
		off += 8
		i++
	}
}

func (ec *emitContext) emitRestoreTypedSelfFrameRegs() {
	gprs, fprs := ec.typedSelfSavedRegs()
	off := 16
	gprOffs := make([]int, len(gprs))
	for i := 0; i < len(gprs); {
		gprOffs[i] = off
		if i+1 < len(gprs) {
			gprOffs[i+1] = off
			off += 16
			i += 2
			continue
		}
		off += 8
		i++
	}
	off = (off + 15) &^ 15
	fprOffs := make([]int, len(fprs))
	for i := 0; i < len(fprs); {
		fprOffs[i] = off
		if i+1 < len(fprs) {
			fprOffs[i+1] = off
			off += 16
			i += 2
			continue
		}
		off += 8
		i++
	}
	for i := len(fprs) - 1; i >= 0; {
		if i > 0 && fprOffs[i] == fprOffs[i-1] {
			ec.asm.FLDP(jit.FReg(fprs[i-1]), jit.FReg(fprs[i]), jit.SP, fprOffs[i])
			i -= 2
			continue
		}
		ec.asm.FLDRd(jit.FReg(fprs[i]), jit.SP, fprOffs[i])
		i--
	}
	for i := len(gprs) - 1; i >= 0; {
		if i > 0 && gprOffs[i] == gprOffs[i-1] {
			ec.asm.LDP(jit.Reg(gprs[i-1]), jit.Reg(gprs[i]), jit.SP, gprOffs[i])
			i -= 2
			continue
		}
		ec.asm.LDR(jit.Reg(gprs[i]), jit.SP, gprOffs[i])
		i--
	}
}

func (ec *emitContext) emitTypedSelfFrameRestoreAndReturn() {
	asm := ec.asm
	ec.emitRestoreTypedSelfFrameRegs()
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, uint16(ec.typedSelfFrameSize()))
	asm.RET()
}

func (ec *emitContext) emitFullFrameRestoreAndReturn() {
	asm := ec.asm
	ec.emitRestoreCalleeSavedFPRs()
	asm.LDP(jit.X27, jit.X28, jit.SP, 80)
	asm.LDP(jit.X25, jit.X26, jit.SP, 64)
	asm.LDP(jit.X23, jit.X24, jit.SP, 48)
	asm.LDP(jit.X21, jit.X22, jit.SP, 32)
	asm.LDP(jit.X19, jit.X20, jit.SP, 16)
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	asm.RET()
}

func (ec *emitContext) emitSaveCalleeSavedFPRs() {
	if ec == nil || !ec.useFPR {
		return
	}
	if ec.calleeSavedFPRPairUsed(8, 9) {
		ec.asm.FSTP(jit.D8, jit.D9, jit.SP, 96)
	}
	if ec.calleeSavedFPRPairUsed(10, 11) {
		ec.asm.FSTP(jit.D10, jit.D11, jit.SP, 112)
	}
}

func (ec *emitContext) emitRestoreCalleeSavedFPRs() {
	if ec == nil || !ec.useFPR {
		return
	}
	if ec.calleeSavedFPRPairUsed(8, 9) {
		ec.asm.FLDP(jit.D8, jit.D9, jit.SP, 96)
	}
	if ec.calleeSavedFPRPairUsed(10, 11) {
		ec.asm.FLDP(jit.D10, jit.D11, jit.SP, 112)
	}
}

func (ec *emitContext) calleeSavedFPRPairUsed(a, b int) bool {
	if ec == nil {
		return false
	}
	if ec.alloc == nil {
		return ec.useFPR
	}
	for _, pr := range ec.alloc.ValueRegs {
		if pr.IsFloat && (pr.Reg == a || pr.Reg == b) {
			return true
		}
	}
	return false
}

func (ec *emitContext) emitPrologue() {
	asm := ec.asm

	// R146: mark native entry before anything else (AAPCS scratch only).
	ec.emitTier2EntryMark()

	// Allocate stack frame.
	asm.SUBimm(jit.SP, jit.SP, uint16(frameSize))
	// Save FP, LR.
	asm.STP(jit.X29, jit.X30, jit.SP, 0)
	// Set FP = SP.
	asm.ADDimm(jit.X29, jit.SP, 0)
	// Save callee-saved GPRs.
	asm.STP(jit.X19, jit.X20, jit.SP, 16)
	asm.STP(jit.X21, jit.X22, jit.SP, 32)
	asm.STP(jit.X23, jit.X24, jit.SP, 48)
	asm.STP(jit.X25, jit.X26, jit.SP, 64)
	asm.STP(jit.X27, jit.X28, jit.SP, 80)
	// Save callee-saved FPRs only if float values are register-allocated.
	ec.emitSaveCalleeSavedFPRs()

	// Set up pinned registers.
	// X0 holds ExecContext pointer (from callJIT trampoline).
	asm.MOVreg(mRegCtx, jit.X0)                       // X19 = ctx
	asm.LDR(mRegRegs, mRegCtx, execCtxOffRegs)        // X26 = ctx.Regs
	asm.LDR(mRegConsts, mRegCtx, execCtxOffConstants) // X27 = ctx.Constants
	asm.LoadImm64(mRegTagInt, nb64(jit.NB_TagInt))    // X24 = 0xFFFE000000000000
	asm.LoadImm64(mRegTagBool, nb64(jit.NB_TagBool))  // X25 = 0xFFFD000000000000
	ec.emitSetRawSelfRegsEndFromMRegRegs()
	ec.emitBoxedEntryShapeGuards()
	if ec.fn != nil && ec.fn.Entry != nil && len(ec.fn.Blocks) > 0 && ec.fn.Blocks[0] != ec.fn.Entry {
		asm.B(ec.entryBlockLabel())
	}
}

func (ec *emitContext) emitEpilogue() {
	asm := ec.asm

	asm.Label("epilogue")

	// Store exit code 0 (normal return) to ExecContext.
	asm.MOVimm16(jit.X0, 0)
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)

	// Shared register restore and return (used by both normal and deopt paths).
	asm.Label("deopt_epilogue")
	leafDeoptLabel := ec.uniqueLabel("leaf_deopt_epilogue")
	typedDeoptLabel := ec.uniqueLabel("typed_deopt_epilogue")
	typedClobberDeoptLabel := ec.uniqueLabel("typed_clobber_deopt_epilogue")
	leafDeoptContinueLabel := ec.uniqueLabel("leaf_deopt_continue")
	ec.emitLoadCallMode(jit.X16)
	if ec.typedSelfABI.Eligible {
		asm.CMPimm(jit.X16, callModeTypedSelf)
		asm.BCond(jit.CondEQ, typedDeoptLabel)
		if ec.typedPeerClobberEntryEnabled() {
			asm.CMPimm(jit.X16, callModeTypedPeerClobber)
			asm.BCond(jit.CondEQ, typedClobberDeoptLabel)
		}
	}
	asm.CMPimm(jit.X16, callModeLeafX0)
	asm.BCond(jit.CondEQ, leafDeoptLabel)
	asm.B(leafDeoptContinueLabel)
	if ec.typedSelfABI.Eligible {
		asm.Label(typedDeoptLabel)
		ec.emitTypedSelfFrameRestoreAndReturn()
		if ec.typedPeerClobberEntryEnabled() {
			asm.Label(typedClobberDeoptLabel)
			ec.emitTypedPeerClobberRestoreAndReturn()
		}
	}
	asm.Label(leafDeoptLabel)
	ec.emitRestoreCalleeSavedFPRs()
	asm.LDP(jit.X27, jit.X28, jit.SP, 80)
	asm.LDP(jit.X25, jit.X26, jit.SP, 64)
	asm.LDP(jit.X23, jit.X24, jit.SP, 48)
	asm.LDP(jit.X21, jit.X22, jit.SP, 32)
	asm.LDP(jit.X19, jit.X20, jit.SP, 16)
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	asm.RET()
	asm.Label(leafDeoptContinueLabel)

	// Restore callee-saved FPRs only if they were saved.
	ec.emitRestoreCalleeSavedFPRs()
	// Restore callee-saved GPRs.
	asm.LDP(jit.X27, jit.X28, jit.SP, 80)
	asm.LDP(jit.X25, jit.X26, jit.SP, 64)
	asm.LDP(jit.X23, jit.X24, jit.SP, 48)
	asm.LDP(jit.X21, jit.X22, jit.SP, 32)
	asm.LDP(jit.X19, jit.X20, jit.SP, 16)
	// Restore FP, LR.
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	// Deallocate stack frame.
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	// Return.
	asm.RET()

	if !ec.skipStandardDirectEntry {
		// --- Tier 2 leaf entry point for Tier 2 BLR callers ---
		// This entry keeps the boxed-X0 return ABI, but still preserves the
		// callee-saved register set. Tier 2 callers spill known live SSA
		// values around BLR; the full frame keeps the native protocol robust
		// when register pressure or liveness conservatism changes.
		asm.Label("t2_leaf_entry")
		ec.emitTier2EntryMark()
		asm.SUBimm(jit.SP, jit.SP, uint16(frameSize))
		asm.STP(jit.X29, jit.X30, jit.SP, 0)
		asm.ADDimm(jit.X29, jit.SP, 0)
		asm.STP(jit.X19, jit.X20, jit.SP, 16)
		asm.STP(jit.X21, jit.X22, jit.SP, 32)
		asm.STP(jit.X23, jit.X24, jit.SP, 48)
		asm.STP(jit.X25, jit.X26, jit.SP, 64)
		asm.STP(jit.X27, jit.X28, jit.SP, 80)
		ec.emitSaveCalleeSavedFPRs()
		asm.MOVreg(mRegCtx, jit.X0)                       // X19 = ctx
		asm.LDR(mRegRegs, mRegCtx, execCtxOffRegs)        // X26 = ctx.Regs
		asm.LDR(mRegConsts, mRegCtx, execCtxOffConstants) // X27 = ctx.Constants
		asm.LoadImm64(mRegTagInt, nb64(jit.NB_TagInt))    // X24
		asm.LoadImm64(mRegTagBool, nb64(jit.NB_TagBool))  // X25
		ec.emitSetRawSelfRegsEndFromMRegRegs()
		ec.emitBoxedEntryShapeGuards()
		asm.B(ec.entryBlockLabel())

		// --- Direct entry point for BLR callers (Tier 1 native call) ---
		// Uses the FULL frame (same as normal entry) because Tier 2 may use
		// callee-saved GPRs (X20-X23) for register allocation. The Tier 1
		// caller expects callee-saved registers to be preserved across BLR.
		// Caller has set: X0=ctx, ctx.Regs=callee regs base,
		// ctx.Constants=callee constants, CallMode=1.
		asm.Label("t2_direct_entry")
		// R146: mark native entry (BLR-from-Tier-1 path).
		ec.emitTier2EntryMark()
		asm.SUBimm(jit.SP, jit.SP, uint16(frameSize))
		asm.STP(jit.X29, jit.X30, jit.SP, 0)
		asm.ADDimm(jit.X29, jit.SP, 0)
		asm.STP(jit.X19, jit.X20, jit.SP, 16)
		asm.STP(jit.X21, jit.X22, jit.SP, 32)
		asm.STP(jit.X23, jit.X24, jit.SP, 48)
		asm.STP(jit.X25, jit.X26, jit.SP, 64)
		asm.STP(jit.X27, jit.X28, jit.SP, 80)
		ec.emitSaveCalleeSavedFPRs()
		asm.MOVreg(mRegCtx, jit.X0)                       // X19 = ctx
		asm.LDR(mRegRegs, mRegCtx, execCtxOffRegs)        // X26 = ctx.Regs
		asm.LDR(mRegConsts, mRegCtx, execCtxOffConstants) // X27 = ctx.Constants
		asm.LoadImm64(mRegTagInt, nb64(jit.NB_TagInt))    // X24
		asm.LoadImm64(mRegTagBool, nb64(jit.NB_TagBool))  // X25
		ec.emitSetRawSelfRegsEndFromMRegRegs()
		ec.emitBoxedEntryShapeGuards()
		asm.B(ec.entryBlockLabel())
	}

	// --- Self-call entry point (R40) ---
	// Only emitted when the function has self-calls AND the Tier 2 emit
	// will generate BL "t2_self_entry". Gated on ec.fn.Proto.HasSelfCalls.
	// This keeps insn count unchanged for non-self-recursive functions.
	//
	// Lightweight entry for proven-self Tier 2 calls. Caller has already
	// set up: ctx (unchanged), ctx.Regs (advanced), ctx.Constants
	// (unchanged, same proto), tag constants X24/X25 (unchanged).
	// Skip: MOVreg mRegCtx, LDR mRegConsts, LoadImm64 X24/X25.
	// Keep: frame allocation, callee-saved regs save (ARM64 ABI),
	//       LDR mRegRegs from ctx.Regs (caller advanced it).
	//
	// Savings: 4 setup insns per self-call (MOVreg + LDR X27 +
	//          2×LoadImm64). Blast radius: small; correctness argument:
	//          self-call means same proto, same ctx, tags are
	//          invariant globals.
	if ec.fn != nil && ec.fn.Proto != nil && ec.fn.Proto.HasSelfCalls {
		asm.Label("t2_self_entry")
		asm.SUBimm(jit.SP, jit.SP, uint16(frameSize))
		asm.STP(jit.X29, jit.X30, jit.SP, 0)
		asm.ADDimm(jit.X29, jit.SP, 0)
		asm.STP(jit.X19, jit.X20, jit.SP, 16)
		asm.STP(jit.X21, jit.X22, jit.SP, 32)
		asm.STP(jit.X23, jit.X24, jit.SP, 48)
		asm.STP(jit.X25, jit.X26, jit.SP, 64)
		asm.STP(jit.X27, jit.X28, jit.SP, 80)
		ec.emitSaveCalleeSavedFPRs()
		// Skip MOVreg mRegCtx, X0  (mRegCtx unchanged in self-call)
		asm.LDR(mRegRegs, mRegCtx, execCtxOffRegs)
		ec.emitBoxedEntryShapeGuards()
		asm.B(ec.entryBlockLabel())
	}

	// R129: numeric entry + pass-2 body are emitted AFTER epilogue +
	// deferredResumes via emitNumericBody() (called from Compile).

	// --- Direct epilogue for BLR callers ---
	// Return path when CallMode == 1 in emitReturn. Uses the same frame
	// restore as normal epilogue since the direct entry uses a full frame.
	// t2_leaf_epilogue uses the boxed-X0 leaf return ABI; use X16 for ExitCode
	// so leaf callers can preserve the boxed X0 return value.
	asm.Label("t2_leaf_epilogue")
	asm.MOVimm16(jit.X16, 0)
	asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	ec.emitRestoreCalleeSavedFPRs()
	asm.LDP(jit.X27, jit.X28, jit.SP, 80)
	asm.LDP(jit.X25, jit.X26, jit.SP, 64)
	asm.LDP(jit.X23, jit.X24, jit.SP, 48)
	asm.LDP(jit.X21, jit.X22, jit.SP, 32)
	asm.LDP(jit.X19, jit.X20, jit.SP, 16)
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	asm.RET()

	asm.Label("t2_direct_epilogue")
	asm.MOVimm16(jit.X16, 0)
	asm.STR(jit.X16, mRegCtx, execCtxOffExitCode)
	ec.emitRestoreCalleeSavedFPRs()
	asm.LDP(jit.X27, jit.X28, jit.SP, 80)
	asm.LDP(jit.X25, jit.X26, jit.SP, 64)
	asm.LDP(jit.X23, jit.X24, jit.SP, 48)
	asm.LDP(jit.X21, jit.X22, jit.SP, 32)
	asm.LDP(jit.X19, jit.X20, jit.SP, 16)
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))
	asm.RET()

	if ec.typedSelfABI.Eligible {
		if ec.typedSelfABI.Return == SpecializedABIReturnRawInt {
			ec.emitTypedSelfRawIntReturnEpilogue()
			if ec.typedPeerClobberEntryEnabled() {
				ec.emitTypedPeerClobberRawIntReturnEpilogue()
			}
		} else if ec.typedSelfABI.Return == SpecializedABIReturnRawFloat {
			ec.emitTypedSelfRawFloatReturnEpilogue()
			if ec.typedPeerClobberEntryEnabled() {
				ec.emitTypedPeerClobberRawFloatReturnEpilogue()
			}
		}
		ec.emitTypedSelfReturnEpilogue()
		ec.emitTypedSelfEntry()
		if ec.typedPeerClobberEntryEnabled() {
			ec.emitTypedPeerClobberReturnEpilogue()
			ec.emitTypedPeerClobberEntry()
		}
		ec.emitTypedEntryParamsLabel()
	}

	if ec.numericParamCount > 0 && ec.fn != nil && ec.fn.Proto != nil {
		asm.Label("num_epilogue")
		asm.MOVimm16(jit.X16, 0)
		asm.LDP(jit.X29, jit.X30, jit.SP, 0)
		asm.ADDimm(jit.SP, jit.SP, uint16(numericSelfEntryFrameSize))
		asm.RET()

		asm.Label("num_deopt_epilogue")
		asm.LDR(jit.X16, mRegCtx, execCtxOffExitCode)
		asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
		asm.LDP(jit.X29, jit.X30, jit.SP, 0)
		asm.ADDimm(jit.SP, jit.SP, uint16(numericSelfEntryFrameSize))
		asm.RET()
	}
}
