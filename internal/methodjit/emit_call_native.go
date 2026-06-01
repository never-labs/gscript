//go:build darwin && arm64

// emit_call_native.go implements native ARM64 BLR calls for the Tier 2 Method JIT.
//
// When Tier 2 encounters OpCall, instead of exiting to Go (emitCallExit ~80ns),
// it emits a native BLR sequence (~10ns) identical to Tier 1's tier1_call.go.
// The key difference: Tier 2 must spill/reload live SSA register allocations
// around the BLR since the callee is free to use the same allocatable registers.
//
// Native call sequence:
//   1. Store function value and arguments to the VM register file
//   2. Spill ALL live SSA registers (GPR + FPR) to their home slots
//   3. Type check: is the function a compiled VMClosure?
//   4. Resolve a direct entry; if no DirectEntryPtr/Tier2DirectEntryPtr is
//      published, fall to slow path
//   5. Bounds check: callee register window fits in register file
//   6. Increment callee's CallCount (for tiering)
//   7. Save caller state on native stack (X26, X27, FP, LR, CallMode, etc.)
//   8. Copy args to callee register window
//   9. Set up callee context, BLR to callee's direct entry
//  10. Restore caller state from stack
//  11. Check callee exit code
//  12. Reload ALL live SSA registers from home slots
//  13. Store result to SSA value's home
//
// Slow path: falls back to emitCallExit (exit-resume via Go).

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
)

const (
	// Tier 2 direct/self entries use a full 128-byte frame. executeTier2
	// reserves native stack budget before entering JIT code, so this can be
	// higher than the no-reserve emergency limit while still avoiding Go stack
	// guard corruption.
	maxNativeCallDepth = 128

	// Raw-int self calls use an args-only caller shim plus a 16-byte numeric
	// callee frame, much smaller than the boxed direct-entry frame. Multi-arg
	// raw self calls bound native recursion with RawSelfRegsEnd, so they do
	// not need per-call NativeCallDepth traffic.
	maxRawSelfCallDepth = 512

	// Raw-int self BL remains behind a kill switch, but the v1 entry,
	// resume, fallback, and return contract is now wired through
	// emitCallNativeRawIntSelf.
	enableNumericSelfBL = true
)

// emitCallNative emits a native BLR call sequence for OpCall in Tier 2.
// Uses selective spill/reload of SSA registers around the BLR: only registers
// that are actually live across the call point are saved/restored. Falls back
// to emitCallExit on the slow path (non-closure, uncompiled, overflow, etc.).
func (ec *emitContext) emitCallNative(instr *Instr) {
	asm := ec.asm

	desc := callExitDescriptorFromInstr(instr)
	ec.traceNativeCallEmit(instr, "generic native", nil, nil)
	funcSlot := desc.slot
	nArgs := desc.nArgs
	nRets := desc.nRets
	noDepthCallee := ec.staticNoDepthCallee(instr)

	// Step 1: Store the function value and arguments to the VM register file.
	// This must happen BEFORE spilling, since resolveValueNB may read from
	// SSA registers that we're about to spill.
	ec.emitStoreCallFrameArgs(instr, funcSlot)

	// Step 2: Selectively spill only registers that are LIVE across this call.
	// A value is live across the call if it's used by any instruction after the
	// call in the same block, or is used by a phi in a successor block.
	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillSelectiveForCall(liveGPRs, liveFPRs)

	// Labels for the native call path.
	slowLabel := ec.uniqueLabel("t2call_slow")
	doneLabel := ec.uniqueLabel("t2call_done")
	exitHandleLabel := ec.uniqueLabel("t2call_callee_exit")

	// Callee base offset: past ALL Tier 2 slots (NumRegs + temp slots).
	// This prevents the callee's register window from clobbering our SSA temp slots.
	calleeBaseOff := ec.nextSlot * jit.ValueSize

	// Step 3: Load function value from regs[funcSlot]. The native-depth guard
	// runs after callee resolution, where LeafNoCall can skip it entirely.
	asm.LDR(jit.X0, mRegRegs, slotOffset(funcSlot))

	// --- Polymorphic call IC fast path ---
	// Allocate this call site's cache slot (4 ways × 4 uint64).
	icIdx := ec.nextCallCacheIndex
	ec.nextCallCacheIndex++
	ec.recordCallCachePC(icIdx, instr.SourcePC)
	cacheOff := icIdx * tier2CallCacheStrideBytes
	icDoneLabel := ec.uniqueLabel("t2call_ic_done")

	// X3 = &CallCache[site][0].
	asm.LDR(jit.X3, mRegCtx, execCtxOffTier2CallCache)
	if cacheOff > 0 {
		if cacheOff <= 4095 {
			asm.ADDimm(jit.X3, jit.X3, uint16(cacheOff))
		} else {
			asm.LoadImm64(jit.X4, int64(cacheOff))
			asm.ADDreg(jit.X3, jit.X3, jit.X4)
		}
	}
	icHitLabels := make([]string, tier2CallCacheWays)
	for way := 0; way < tier2CallCacheWays; way++ {
		icHitLabels[way] = ec.uniqueLabel("t2call_ic_hit")
		wayOff := way * tier2CallCacheWayBytes
		asm.LDR(jit.X4, jit.X3, wayOff+baselineCallCacheBoxedOff)
		asm.CMPreg(jit.X0, jit.X4)
		asm.BCond(jit.CondEQ, icHitLabels[way])
	}

	// --- IC Miss: original type check + proto load path ---
	// Type check: must be ptr (0xFFFF) with sub-type = 8 (VMClosure).
	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X2, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, slowLabel)

	// Check sub-type == 8.
	asm.LSRimm(jit.X1, jit.X0, uint8(nbPtrSubShift))
	asm.LoadImm64(jit.X2, 0xF)
	asm.ANDreg(jit.X1, jit.X1, jit.X2)
	asm.CMPimm(jit.X1, nbPtrSubVMClosure)
	asm.BCond(jit.CondNE, slowLabel)

	// Step 4: Extract raw pointer -> X0 = *vm.Closure.
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)

	// Load Proto, DirectEntryPtr.
	asm.LDR(jit.X1, jit.X0, vmClosureOffProto)          // X1 = *FuncProto
	asm.LDR(jit.X2, jit.X1, funcProtoOffDirectEntryPtr) // X2 = DirectEntryPtr
	missHaveEntryLabel := ec.uniqueLabel("t2call_miss_have_entry")
	asm.CBNZ(jit.X2, missHaveEntryLabel)
	asm.LDR(jit.X2, jit.X1, funcProtoOffTier2DirectEntryPtr)
	asm.Label(missHaveEntryLabel)
	asm.CBZ(jit.X2, slowLabel) // not compiled -> slow

	// Update IC cache with the boxed closure value (reload from
	// memory since X0 now holds the raw ptr), direct entry, proto, and entry
	// publication version.
	icUpdateWayLabels := make([]string, tier2CallCacheWays)
	for way := 0; way < tier2CallCacheWays; way++ {
		icUpdateWayLabels[way] = ec.uniqueLabel("t2call_ic_update")
		wayOff := way * tier2CallCacheWayBytes
		asm.LDR(jit.X4, jit.X3, wayOff+baselineCallCacheBoxedOff)
		asm.CBZ(jit.X4, icUpdateWayLabels[way])
	}
	asm.B(icUpdateWayLabels[0])
	for way := 0; way < tier2CallCacheWays; way++ {
		wayOff := way * tier2CallCacheWayBytes
		asm.Label(icUpdateWayLabels[way])
		asm.LDR(jit.X4, mRegRegs, slotOffset(funcSlot)) // re-load boxed value
		asm.STR(jit.X4, jit.X3, wayOff+baselineCallCacheBoxedOff)
		ec.emitTaggedLeafEntryIfAvailable(jit.X1, jit.X2, jit.X4)
		asm.STR(jit.X2, jit.X3, wayOff+baselineCallCacheEntryOff)
		asm.STR(jit.X1, jit.X3, wayOff+baselineCallCacheProtoOff)
		asm.LDR(jit.X4, jit.X1, funcProtoOffDirectEntryVersion)
		asm.STR(jit.X4, jit.X3, wayOff+baselineCallCacheVersionOff)
		asm.B(icDoneLabel)
	}

	// --- IC Hit: validate direct-entry version, refreshing entry on change. ---
	// X0 still holds the boxed closure value (matched cache).
	for way := 0; way < tier2CallCacheWays; way++ {
		wayOff := way * tier2CallCacheWayBytes
		asm.Label(icHitLabels[way])
		asm.LDR(jit.X2, jit.X3, wayOff+baselineCallCacheEntryOff)   // X2 = cached direct entry
		asm.LDR(jit.X1, jit.X3, wayOff+baselineCallCacheProtoOff)   // X1 = cached *Proto
		asm.LDR(jit.X4, jit.X3, wayOff+baselineCallCacheVersionOff) // X4 = cached entry version
		asm.LDR(jit.X5, jit.X1, funcProtoOffDirectEntryVersion)
		icVersionOKLabel := ec.uniqueLabel("t2call_ic_version_ok")
		asm.CMPreg(jit.X4, jit.X5)
		asm.BCond(jit.CondEQ, icVersionOKLabel)
		asm.LDR(jit.X4, jit.X1, funcProtoOffDirectEntryPtr)
		icHaveEntryLabel := ec.uniqueLabel("t2call_ic_have_entry")
		asm.CBNZ(jit.X4, icHaveEntryLabel)
		asm.LDR(jit.X4, jit.X1, funcProtoOffTier2DirectEntryPtr)
		// DirectEntryPtr can be cleared when a baseline/native caller disables
		// generic BLR after an exit. Tier 2 ICs may still use the separate Tier 2
		// entry while it is published, but must not keep a stale entry after both
		// published entry pointers have been cleared.
		asm.CBZ(jit.X4, slowLabel)
		asm.Label(icHaveEntryLabel)
		asm.MOVreg(jit.X2, jit.X4)
		ec.emitTaggedLeafEntryIfAvailable(jit.X1, jit.X2, jit.X4)
		asm.STR(jit.X2, jit.X3, wayOff+baselineCallCacheEntryOff)
		asm.STR(jit.X5, jit.X3, wayOff+baselineCallCacheVersionOff)
		asm.Label(icVersionOKLabel)
		jit.EmitExtractPtr(asm, jit.X0, jit.X0) // X0 = *Closure
		asm.B(icDoneLabel)
	}

	asm.Label(icDoneLabel)
	ec.emitDecodeTaggedPeerEntry(jit.X2, jit.X5)

	if noDepthCallee != nil {
		asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(noDepthCallee))))
		asm.CMPreg(jit.X1, jit.X3)
		asm.BCond(jit.CondNE, slowLabel)
	}
	flagSpec := ec.callCalleeFlagSpec(instr)
	if noDepthCallee == nil {
		ec.emitGuardCalleeProtoSet(flagSpec.protos, slowLabel)
	}

	// Step 5: Bounds check: callee register window fits in register file.
	asm.LDR(jit.X3, jit.X1, funcProtoOffMaxStack) // X3 = calleeMaxStack (int)
	asm.LSLimm(jit.X3, jit.X3, 3)                 // X3 = calleeMaxStack * 8
	if calleeBaseOff <= 4095 {
		asm.ADDimm(jit.X3, jit.X3, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X4, int64(calleeBaseOff))
		asm.ADDreg(jit.X3, jit.X3, jit.X4)
	}
	asm.ADDreg(jit.X3, jit.X3, mRegRegs) // X3 = mRegRegs + calleeBaseOff + calleeMaxStack*8
	asm.LDR(jit.X4, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondHI, slowLabel) // unsigned greater than -> slow path

	// Step 6: Increment callee's CallCount until Tier 2 is installed. Once a
	// callee has a Tier 2 entry, the hot native peer-call path no longer needs
	// to feed promotion counters on every call.
	// X0 = *vm.Closure, X1 = *FuncProto, X2 = DirectEntryPtr.
	skipCallCountLabel := ec.uniqueLabel("t2call_skip_callcount")
	asm.LDR(jit.X3, jit.X1, funcProtoOffTier2DirectEntryPtr)
	asm.CBNZ(jit.X3, skipCallCountLabel)
	asm.LDR(jit.X3, jit.X1, funcProtoOffCallCount)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, jit.X1, funcProtoOffCallCount)
	// If at Tier 2 threshold, fall to slow path to trigger compilation.
	asm.CMPimm(jit.X3, tmDefaultTier2Threshold)
	asm.BCond(jit.CondEQ, slowLabel)
	asm.Label(skipCallCountLabel)

	staticSelf := ec.fn != nil && ec.fn.Proto != nil && ec.fn.Proto.HasSelfCalls && ec.isStaticSelfCall(instr)
	stackSlowLabel := ec.uniqueLabel("t2call_stack_slow")
	knownLeafCall := noDepthCallee != nil || flagSpec.knownLeaf
	knownNoGlobalCall := staticSelf || flagSpec.knownNoGlobal
	dynamicCalleeFlags := (noDepthCallee == nil && !knownLeafCall) || (!staticSelf && !knownNoGlobalCall)

	// Step 7: Save caller state on stack (128 bytes, 16-byte aligned).
	// R111: for a static self-call, GlobalCache is invariant
	// (same proto → same GlobalCache), so skip saving that field.
	// CallMode cannot be skipped: top-level Tier 2 enters with CallMode=0,
	// while a BL/BLR callee must return through the direct epilogue.
	asm.SUBimm(jit.SP, jit.SP, 128)
	if dynamicCalleeFlags {
		knownFlags := uint16(0)
		if knownLeafCall {
			knownFlags |= 1
		}
		if knownNoGlobalCall {
			knownFlags |= 2
		}
		asm.MOVimm16(jit.X6, knownFlags)
		if noDepthCallee == nil && !knownLeafCall {
			asm.LDRB(jit.X4, jit.X1, funcProtoOffLeafNoCall)
			asm.ORRreg(jit.X6, jit.X6, jit.X4)
		}
		if !staticSelf && !knownNoGlobalCall {
			asm.LDRB(jit.X4, jit.X1, funcProtoOffNoGlobalOps)
			asm.LSLimm(jit.X4, jit.X4, 1)
			asm.ORRreg(jit.X6, jit.X6, jit.X4)
		}
		asm.STR(jit.X6, jit.SP, 120)
	}
	if noDepthCallee == nil && !knownLeafCall {
		depthOKLabel := ec.uniqueLabel("t2call_depth_ok")
		asm.TBNZ(jit.X6, 0, depthOKLabel)
		asm.LDR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
		asm.CMPimm(jit.X3, maxNativeCallDepth)
		asm.BCond(jit.CondGE, stackSlowLabel)
		asm.Label(depthOKLabel)
	}
	asm.STP(jit.X29, jit.X30, jit.SP, 0)
	asm.STP(mRegRegs, mRegConsts, jit.SP, 16)
	ec.emitLoadCallMode(jit.X3)
	asm.STR(jit.X3, jit.SP, 32)
	// Save caller's ClosurePtr (always — closure instance may differ).
	asm.LDR(jit.X3, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X3, jit.SP, 40)
	if !staticSelf {
		if !knownNoGlobalCall {
			skipSaveGlobalsLabel := ec.uniqueLabel("t2call_skip_save_globals")
			asm.TBNZ(jit.X6, 1, skipSaveGlobalsLabel)
			asm.LDR(jit.X3, mRegCtx, execCtxOffBaselineGlobalCache)
			asm.STR(jit.X3, jit.SP, 48)
			asm.LDR(jit.X3, mRegCtx, execCtxOffTier2GlobalCache)
			asm.STR(jit.X3, jit.SP, 56)
			asm.LDR(jit.X3, mRegCtx, execCtxOffTier2GlobalCacheGen)
			asm.STR(jit.X3, jit.SP, 64)
			asm.LDR(jit.X3, mRegCtx, execCtxOffTier2GlobalIndex)
			asm.STR(jit.X3, jit.SP, 72)
			asm.Label(skipSaveGlobalsLabel)
		}
	}
	// Keep the callee closure pointer for ExitNativeCallExit. If the callee
	// returns through an exit-resume path, caller state is restored before Go
	// sees the exit, so the raw closure pointer must survive independently.
	asm.STR(jit.X0, jit.SP, 112)

	// Step 8: Copy args to callee register window.
	for i := 0; i < nArgs; i++ {
		srcOff := slotOffset(funcSlot + 1 + i)
		dstOff := calleeBaseOff + i*jit.ValueSize
		asm.LDR(jit.X3, mRegRegs, srcOff)
		asm.STR(jit.X3, mRegRegs, dstOff)
	}

	// Step 9: Set up callee context and BLR.
	// Advance mRegRegs to callee base.
	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X3, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X3)
	}
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)

	// Load callee's constants.
	asm.LDR(mRegConsts, jit.X1, funcProtoOffConstants)
	asm.STR(mRegConsts, mRegCtx, execCtxOffConstants)

	// Set callee's ClosurePtr.
	asm.STR(jit.X0, mRegCtx, execCtxOffBaselineClosurePtr)

	// Set CallMode. Tagged call-IC entries use the Tier 2-only boxed leaf ABI
	// that returns the normal boxed result in X0.
	ec.emitStoreCallMode(jit.X5)

	// R111: skip GlobalCache setup on static self-call (per-proto invariant).
	if !staticSelf {
		if !knownNoGlobalCall {
			skipSetupGlobalsLabel := ec.uniqueLabel("t2call_skip_setup_globals")
			asm.TBNZ(jit.X6, 1, skipSetupGlobalsLabel)
			// Load callee's GlobalValCache from Proto.
			asm.LDR(jit.X3, jit.X1, funcProtoOffGlobalValCachePtr)
			asm.STR(jit.X3, mRegCtx, execCtxOffBaselineGlobalCache)
			asm.LDR(jit.X3, jit.X1, funcProtoOffTier2GlobalCachePtr)
			asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalCache)
			asm.LDR(jit.X3, jit.X1, funcProtoOffTier2GlobalCacheGenPtr)
			asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalCacheGen)
			asm.LDR(jit.X3, jit.X1, funcProtoOffTier2GlobalIndexPtr)
			asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalIndex)
			asm.Label(skipSetupGlobalsLabel)
		}
		asm.LDR(jit.X3, jit.X1, funcProtoOffFieldCache)
		asm.STR(jit.X3, mRegCtx, execCtxOffBaselineFieldCache)
		asm.LDR(jit.X3, jit.X1, funcProtoOffFieldPolyCache)
		asm.STR(jit.X3, mRegCtx, execCtxOffBaselineFieldPolyCache)
		asm.LDR(jit.X3, jit.X1, funcProtoOffTableStringKeyCache)
		asm.STR(jit.X3, mRegCtx, execCtxOffBaselineTableStringKeyCache)
	}

	// Increment NativeCallDepth unless the guarded callee is a leaf.
	if noDepthCallee == nil && !knownLeafCall {
		skipDepthIncLabel := ec.uniqueLabel("t2call_skip_depth_inc")
		asm.TBNZ(jit.X6, 0, skipDepthIncLabel)
		asm.LDR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
		asm.ADDimm(jit.X3, jit.X3, 1)
		asm.STR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
		asm.Label(skipDepthIncLabel)
	}

	// R40/R110: self-call fast path via HasSelfCalls. Statically proven
	// raw-int self calls are routed before this function to the dedicated
	// emitCallNativeRawIntSelf protocol; the generic path always keeps the
	// boxed VM call/return ABI.
	if ec.fn != nil && ec.fn.Proto != nil && ec.fn.Proto.HasSelfCalls {
		asm.MOVreg(jit.X0, mRegCtx)
		if ec.isStaticSelfCall(instr) {
			// R110: static self-call — 1 insn.
			asm.BL("t2_self_entry")
		} else {
			selfCallLabel := ec.uniqueLabel("t2call_do_self")
			afterBlLabel := ec.uniqueLabel("t2call_after_bl")
			// X1 still holds *FuncProto from step 4 load.
			asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(ec.fn.Proto))))
			asm.CMPreg(jit.X1, jit.X3)
			asm.BCond(jit.CondEQ, selfCallLabel)
			// Non-self: original BLR path.
			asm.BLR(jit.X2)
			asm.B(afterBlLabel)
			// Self-call: PC-relative BL to lightweight entry
			// (t2_self_entry skips 4 redundant setup insns vs t2_direct_entry).
			asm.Label(selfCallLabel)
			asm.BL("t2_self_entry")
			asm.Label(afterBlLabel)
		}
	} else {
		asm.MOVreg(jit.X0, mRegCtx)
		asm.BLR(jit.X2)
	}

	// Decrement NativeCallDepth unless the guarded callee is a leaf.
	if noDepthCallee == nil && !knownLeafCall {
		skipDepthDecLabel := ec.uniqueLabel("t2call_skip_depth_dec")
		asm.LDR(jit.X6, jit.SP, 120)
		asm.TBNZ(jit.X6, 0, skipDepthDecLabel)
		asm.LDR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
		asm.SUBimm(jit.X3, jit.X3, 1)
		asm.STR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
		asm.Label(skipDepthDecLabel)
	}
	ec.emitLoadCallMode(jit.X8)

	// Snapshot callee exit metadata only on the cold exit path. Successful
	// native peer calls are the hot path and do not need resume metadata.
	skipExitSnapshotLabel := ec.uniqueLabel("t2call_skip_exit_snapshot")
	asm.MOVimm16(jit.X7, 0)
	asm.LDR(jit.X3, mRegCtx, execCtxOffExitCode)
	asm.CBZ(jit.X3, skipExitSnapshotLabel)
	asm.MOVimm16(jit.X7, 1)
	ec.emitPushNativeCallExitFrameIfNested(jit.X3, jit.X4, jit.X5, jit.X6)
	asm.STR(jit.X3, mRegCtx, execCtxOffNativeCalleeExitCode)
	asm.LDR(jit.X3, mRegCtx, execCtxOffResumeNumericPass)
	asm.STR(jit.X3, mRegCtx, execCtxOffNativeCalleeResumePass)
	asm.LDR(jit.X3, mRegCtx, execCtxOffExitResumePC)
	asm.STR(jit.X3, mRegCtx, execCtxOffNativeCalleeResumePC)
	asm.LDR(jit.X3, jit.SP, 112)
	asm.STR(jit.X3, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.Label(skipExitSnapshotLabel)

	// Step 10: Restore caller state.
	asm.LDP(mRegRegs, mRegConsts, jit.SP, 16)
	asm.LDR(jit.X3, jit.SP, 32)
	ec.emitStoreCallMode(jit.X3)
	asm.LDR(jit.X3, jit.SP, 40)
	asm.STR(jit.X3, mRegCtx, execCtxOffBaselineClosurePtr)
	if !staticSelf {
		if !knownNoGlobalCall {
			skipRestoreGlobalsLabel := ec.uniqueLabel("t2call_skip_restore_globals")
			asm.LDR(jit.X6, jit.SP, 120)
			asm.TBNZ(jit.X6, 1, skipRestoreGlobalsLabel)
			asm.LDR(jit.X3, jit.SP, 48)
			asm.STR(jit.X3, mRegCtx, execCtxOffBaselineGlobalCache)
			asm.LDR(jit.X3, jit.SP, 56)
			asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalCache)
			asm.LDR(jit.X3, jit.SP, 64)
			asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalCacheGen)
			asm.LDR(jit.X3, jit.SP, 72)
			asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalIndex)
			asm.Label(skipRestoreGlobalsLabel)
		}
		if ec.fn != nil && ec.fn.Proto != nil {
			asm.LoadImm64(jit.X3, int64(uintptr(unsafe.Pointer(ec.fn.Proto))))
			asm.LDR(jit.X4, jit.X3, funcProtoOffFieldCache)
			asm.STR(jit.X4, mRegCtx, execCtxOffBaselineFieldCache)
			asm.LDR(jit.X4, jit.X3, funcProtoOffFieldPolyCache)
			asm.STR(jit.X4, mRegCtx, execCtxOffBaselineFieldPolyCache)
			asm.LDR(jit.X4, jit.X3, funcProtoOffTableStringKeyCache)
			asm.STR(jit.X4, mRegCtx, execCtxOffBaselineTableStringKeyCache)
		}
	}
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, 128)

	// Restore ctx pointers.
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
	asm.STR(mRegConsts, mRegCtx, execCtxOffConstants)

	// Step 11: Check callee exit code.
	asm.CBNZ(jit.X7, exitHandleLabel)

	// R143: save representation state BEFORE the post-BL emit. Reloading
	// selective values normalizes boxed homes, and the mutually-exclusive
	// exit/fallback emit paths must still see the pre-call representation.
	savedReprs := ec.snapshotValueReprs()

	ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)

	resultReadyLabel := ec.uniqueLabel("t2call_result_ready")
	if nRets > 0 {
		asm.CMPimm(jit.X8, callModeLeafX0)
		asm.BCond(jit.CondEQ, resultReadyLabel)
		asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineReturnValue)
		asm.Label(resultReadyLabel)
		ec.storeResultNB(jit.X0, instr.ID)
	}
	postSuccessReprs := ec.snapshotValueReprs()

	asm.B(doneLabel)

	if noDepthCallee == nil && !knownLeafCall {
		asm.Label(stackSlowLabel)
		asm.ADDimm(jit.SP, jit.SP, 128)
		asm.B(slowLabel)
	}

	// --- Callee exited mid-execution (deopt/op-exit within callee) ---
	// Return to Go with enough metadata to resume the callee's own
	// exit-resume loop. This avoids replaying the call from the beginning
	// after the callee may already have mutated visible state.
	asm.Label(exitHandleLabel)
	ec.emitRequireNativeCalleeTier2Only(slowLabel)
	ec.restoreValueReprSnapshot(savedReprs)
	ec.emitNativeCallExit(instr, funcSlot, nArgs, nRets, calleeBaseOff)

	// Slow path: no native entry was taken, so the normal caller-side
	// call-exit fallback executes the call exactly once through the VM.
	asm.Label(slowLabel)
	ec.restoreValueReprSnapshot(savedReprs)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitUnboxRawIntRegs(postSuccessReprs)
	ec.restoreValueReprSnapshot(postSuccessReprs)

	// --- Done: merge point for native and slow paths ---
	asm.Label(doneLabel)
}

func (ec *emitContext) emitRequireNativeCalleeTier2Only(slowLabel string) {
	asm := ec.asm
	// This predicate is only needed after a callee exit. Keeping it out of
	// the successful call path avoids per-call DirectEntryPtr traffic.
	asm.LDR(jit.X0, mRegCtx, execCtxOffNativeCalleeClosurePtr)
	asm.CBZ(jit.X0, slowLabel)
	asm.LDR(jit.X0, jit.X0, vmClosureOffProto)
	asm.LDR(jit.X0, jit.X0, funcProtoOffDirectEntryPtr)
	asm.CBNZ(jit.X0, slowLabel)
	asm.MOVimm16(jit.X0, 1)
	asm.STR(jit.X0, mRegCtx, execCtxOffNativeCalleeTier2Only)
}

func (ec *emitContext) emitNativeCallExit(instr *Instr, funcSlot, nArgs, nRets, calleeBaseOff int) {
	ec.emitStoreNativeCallExitDescriptor(callExitDescriptor{
		slot:    funcSlot,
		nArgs:   nArgs,
		nRets:   nRets,
		instrID: instr.ID,
	}, calleeBaseOff)
	ec.emitCallProtocolExitToGo(ExitNativeCallExit)
}

// emitCallNativeStaticSelfFast emits the boxed-value self-call path for a
// statically proven recursive call. It keeps the same public contract as the
// generic native call path (boxed args/results in the VM register file,
// ExitCode checked after return), but skips closure type checks, the
// monomorphic call IC, proto/direct-entry loads, global-cache switching, and
// the full callee-save frame on the recursive entry.
func (ec *emitContext) emitCallNativeStaticSelfFast(instr *Instr) {
	if ec.fn == nil || ec.fn.Proto == nil || !ec.fn.Proto.HasSelfCalls || !ec.isStaticSelfCall(instr) {
		ec.emitCallNative(instr)
		return
	}
	ec.traceNativeCallEmit(instr, "generic native", ec.fn.Proto, nil)

	asm := ec.asm
	desc := callExitDescriptorFromInstr(instr)
	funcSlot := desc.slot
	nArgs := desc.nArgs
	nRets := desc.nRets

	ec.emitStoreCallFrameArgs(instr, funcSlot)

	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillSelectiveForCall(liveGPRs, liveFPRs)

	slowLabel := ec.uniqueLabel("t2self_slow")
	doneLabel := ec.uniqueLabel("t2self_done")
	exitHandleLabel := ec.uniqueLabel("t2self_callee_exit")

	savedReprs := ec.snapshotValueReprs()

	calleeBaseOff := ec.nextSlot * jit.ValueSize

	asm.LDR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
	asm.CMPimm(jit.X3, maxNativeCallDepth)
	asm.BCond(jit.CondGE, slowLabel)

	calleeFrameBytes := ec.nextSlot * jit.ValueSize
	if calleeBaseOff+calleeFrameBytes <= 4095 {
		asm.ADDimm(jit.X3, mRegRegs, uint16(calleeBaseOff+calleeFrameBytes))
	} else {
		asm.LoadImm64(jit.X3, int64(calleeBaseOff+calleeFrameBytes))
		asm.ADDreg(jit.X3, mRegRegs, jit.X3)
	}
	asm.LDR(jit.X4, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondHI, slowLabel)

	asm.SUBimm(jit.SP, jit.SP, 64)
	asm.STP(jit.X29, jit.X30, jit.SP, 0)
	asm.STP(mRegRegs, mRegConsts, jit.SP, 16)
	ec.emitLoadCallMode(jit.X3)
	asm.STR(jit.X3, jit.SP, 32)
	asm.LDR(jit.X3, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.STR(jit.X3, jit.SP, 40)

	for i := 0; i < nArgs; i++ {
		srcOff := slotOffset(funcSlot + 1 + i)
		dstOff := calleeBaseOff + i*jit.ValueSize
		asm.LDR(jit.X3, mRegRegs, srcOff)
		asm.STR(jit.X3, mRegRegs, dstOff)
	}

	if calleeBaseOff <= 4095 {
		asm.ADDimm(mRegRegs, mRegRegs, uint16(calleeBaseOff))
	} else {
		asm.LoadImm64(jit.X3, int64(calleeBaseOff))
		asm.ADDreg(mRegRegs, mRegRegs, jit.X3)
	}
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
	asm.MOVimm16(jit.X3, 1)
	ec.emitStoreCallMode(jit.X3)

	asm.LDR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)

	asm.MOVreg(jit.X0, mRegCtx)
	asm.BL("t2_self_entry")

	asm.LDR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)
	asm.SUBimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, mRegCtx, execCtxOffNativeCallDepth)

	asm.LDP(mRegRegs, mRegConsts, jit.SP, 16)
	asm.LDR(jit.X3, jit.SP, 32)
	ec.emitStoreCallMode(jit.X3)
	asm.LDR(jit.X3, jit.SP, 40)
	asm.STR(jit.X3, mRegCtx, execCtxOffBaselineClosurePtr)
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, 64)
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)
	asm.STR(mRegConsts, mRegCtx, execCtxOffConstants)

	asm.LDR(jit.X3, mRegCtx, execCtxOffExitCode)
	asm.CBNZ(jit.X3, exitHandleLabel)

	if nRets > 0 {
		asm.LDR(jit.X0, mRegCtx, execCtxOffBaselineReturnValue)
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot))
	}

	ec.emitReloadSelectiveForCall(liveGPRs, liveFPRs)

	if nRets > 0 {
		asm.LDR(jit.X0, mRegRegs, slotOffset(funcSlot))
		ec.storeResultNB(jit.X0, instr.ID)
	}
	postSuccessReprs := ec.snapshotValueReprs()
	asm.B(doneLabel)

	asm.Label(exitHandleLabel)
	asm.Label(slowLabel)
	ec.restoreValueReprSnapshot(savedReprs)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
	ec.emitUnboxRawIntRegs(postSuccessReprs)
	ec.restoreValueReprSnapshot(postSuccessReprs)

	asm.Label(doneLabel)
}

// emitOpCall dispatches OpCall to the regular or tail variant and
// applies the post-call invalidation of cross-block verification caches.
// Extracted from emit_dispatch.go to keep that file under rule 13's
// 1000-line cap.
func (ec *emitContext) emitOpCall(instr *Instr) {
	if ec.numericMode && ec.tailCallInstrs[instr.ID] && ec.isNumericStaticSelfCall(instr) {
		ec.emitCallNativeNumericTail(instr)
	} else if !ec.tailCallInstrs[instr.ID] && ec.isNumericStaticSelfCall(instr) {
		ec.emitCallNativeRawIntSelf(instr)
	} else if ec.emitGuardedConstCallIfEligible(instr) {
	} else if ec.emitCallSiteRuntimeSpecializationOpExitIfEligible(instr) {
	} else if ec.emitCallNativeRawIntPeerIfEligible(instr) {
	} else if ec.emitCallNativeFieldShapeTypedPeerIfEligible(instr) {
	} else if ec.emitCallNativeTypedPeerIfEligible(instr) {
	} else if ec.emitCallNativeTypedSelfIfEligible(instr) {
	} else if ec.isStaticSelfCall(instr) && !ec.tailCallInstrs[instr.ID] && callResultCountFromAux2(instr.Aux2) > 0 && !ec.nativeCallReplaySafe {
		ec.traceNativeCallEmit(instr, "call-exit", ec.fn.Proto, nil)
		ec.emitCallExit(instr)
	} else if ec.tailCallInstrs[instr.ID] && ec.isStaticSelfCall(instr) && !ec.hasEntryShapeGuards() {
		ec.emitStaticSelfTailLoop(instr)
	} else if ec.isStaticSelfCall(instr) {
		ec.emitCallNativeStaticSelfFast(instr)
	} else if ec.tailCallInstrs[instr.ID] {
		// R107: tail call — frame-replacing BR on the fast path. The
		// slow-path fallback (emitCallExitFallback) still produces a
		// normal return value, so we DO emit the following OpReturn:
		// on the fast path it's dead code (BR already transferred
		// control), on the slow path it correctly completes the call.
		ec.emitCallNative(instr)
	} else {
		ec.emitCallNative(instr)
	}
	// Calls can modify any table's shape — invalidate verification caches.
	ec.shapeVerified = make(map[int]uint32)
	ec.tableVerified = make(map[int]bool)
	ec.kindVerified = make(map[int]uint16)
	ec.keysDirtyWritten = make(map[int]bool)
	ec.dmVerified = make(map[int]bool)
}

func (ec *emitContext) invalidateCallClobberedFactsAfterResume() {
	ec.shapeVerified = make(map[int]uint32)
	ec.tableVerified = make(map[int]bool)
	ec.kindVerified = make(map[int]uint16)
	ec.keysDirtyWritten = make(map[int]bool)
	ec.dmVerified = make(map[int]bool)
	for valueID := range ec.activeRegs {
		if ec.valueReprOf(valueID) == valueReprRawDataPtr {
			ec.clearValueRepr(valueID)
		}
	}
}

func (ec *emitContext) emitCallNativeNumericTail(instr *Instr) {
	asm := ec.asm
	slowLabel := ec.uniqueLabel("t2numtail_slow")

	entryLabel, hasEntry := ec.entryBlockLabelOK()
	if len(instr.Args) == 0 || ec.fn == nil || ec.fn.Proto == nil || !hasEntry {
		asm.B(slowLabel)
	} else {
		ec.emitNumericArgsInRegs(instr, ec.fn.Proto.NumParams)
		asm.B(entryLabel)
	}

	asm.Label(slowLabel)
	ec.emitCallNative(instr)
}

// emitStaticSelfTailLoop lowers a proven self tail-call into an in-frame loop.
// This avoids growing the native stack and also avoids the generic BR-to-direct
// tail path, whose context/slot protocol is too broad for recursive raw-int
// shapes. The preceding GetGlobal still runs, so cache misses and global exits
// happen before this point.
func (ec *emitContext) emitStaticSelfTailLoop(instr *Instr) {
	if ec.fn == nil || ec.fn.Proto == nil || ec.fn.Entry == nil {
		ec.emitCallNative(instr)
		return
	}
	nArgs := len(instr.Args) - 1
	if nArgs != ec.fn.Proto.NumParams || nArgs > 4 {
		ec.emitCallNative(instr)
		return
	}

	// Tail-call argument assignment is semantically parallel. Stage into
	// scratch registers that cannot be source homes for allocated SSA values
	// before overwriting parameter slots.
	scratch := []jit.Reg{jit.X4, jit.X5, jit.X6, jit.X7}
	for i := 0; i < nArgs; i++ {
		src := ec.resolveValueNB(instr.Args[1+i].ID, scratch[i])
		if src != scratch[i] {
			ec.asm.MOVreg(scratch[i], src)
		}
	}
	for i := 0; i < nArgs; i++ {
		ec.asm.STR(scratch[i], mRegRegs, slotOffset(i))
	}
	ec.asm.B(ec.blockLabelFor(ec.fn.Entry))
}

// emitCallNativeTail emits a tail-call variant of OpCall: when the Call's
// result is returned immediately (Call→Return pattern in the same block),
// we replace our stack frame with the callee's instead of stacking a new
// one. Eliminates caller frame save/restore + BLR/RET overhead, and stops
// stack growth for tail-recursive chains.
//
// Sequence:
//  1. Store fn + args to regs (same as emitCallNative step 1).
//  2. Closure type-check + resolve callee's DirectEntry + bounds check.
//  3. Copy args from regs[funcSlot+1..] to regs[0..nArgs-1] (tail window).
//  4. Set callee context: Constants, ClosurePtr, CallMode=1, GlobalCache.
//     Do NOT advance ctx.Regs (reuse current frame's register window).
//     Do NOT increment NativeCallDepth (we're replacing, not nesting).
//  5. Set X0 to the current ctx pointer, then inline our epilogue.
//  6. BR X2 (tail jump to callee's direct entry).
//
// Correctness: after step 5, LR is the CALLER-OF-CURRENT's return address
// (saved by our prologue at sp+0). After callee runs and does its own
// RET in its epilogue, it returns directly to caller-of-current, as
// required by TCO semantics.
//
// Slow-path fallback: emits the same emitCallExitFallback as emitCallNative
// for non-closure targets, uncompiled callees, or overflow cases.
func (ec *emitContext) emitCallNativeTail(instr *Instr) {
	asm := ec.asm

	funcSlot := int(instr.Aux)
	nArgs := len(instr.Args) - 1
	nRets := callResultCountFromAux2(instr.Aux2)

	// Step 1: Store fn + args to regs (same as emitCallNative).
	if len(instr.Args) > 0 {
		fnReg := ec.resolveValueNB(instr.Args[0].ID, jit.X0)
		if fnReg != jit.X0 {
			asm.MOVreg(jit.X0, fnReg)
		}
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot))
	}
	for i := 1; i < len(instr.Args); i++ {
		argReg := ec.resolveValueNB(instr.Args[i].ID, jit.X0)
		if argReg != jit.X0 {
			asm.MOVreg(jit.X0, argReg)
		}
		asm.STR(jit.X0, mRegRegs, slotOffset(funcSlot+i))
	}

	// For the slow-path fallback, still need all active regs in memory so
	// the Go-side handler can inspect them.
	liveGPRs, liveFPRs := ec.computeLiveAcrossCall(instr)
	ec.emitSpillSelectiveForCall(liveGPRs, liveFPRs)

	slowLabel := ec.uniqueLabel("t2tail_slow")

	// Step 2: Closure type check (ptr + sub-type 8), with R108 mono-IC
	// fast path.
	asm.LDR(jit.X0, mRegRegs, slotOffset(funcSlot))

	icIdx := ec.nextCallCacheIndex
	ec.nextCallCacheIndex++
	ec.recordCallCachePC(icIdx, instr.SourcePC)
	cacheOff := icIdx * tier2CallCacheStrideBytes
	icHitLabel := ec.uniqueLabel("t2tail_ic_hit")
	icDoneLabel := ec.uniqueLabel("t2tail_ic_done")

	asm.LDR(jit.X3, mRegCtx, execCtxOffTier2CallCache)
	asm.LDR(jit.X4, jit.X3, cacheOff)
	asm.CMPreg(jit.X0, jit.X4)
	asm.BCond(jit.CondEQ, icHitLabel)

	// --- IC Miss: full type check + proto load ---
	asm.LSRimm(jit.X1, jit.X0, 48)
	asm.MOVimm16(jit.X2, jit.NB_TagPtrShr48)
	asm.CMPreg(jit.X1, jit.X2)
	asm.BCond(jit.CondNE, slowLabel)
	asm.LSRimm(jit.X1, jit.X0, uint8(nbPtrSubShift))
	asm.LoadImm64(jit.X2, 0xF)
	asm.ANDreg(jit.X1, jit.X1, jit.X2)
	asm.CMPimm(jit.X1, nbPtrSubVMClosure)
	asm.BCond(jit.CondNE, slowLabel)

	// Extract raw pointer -> X0 = *vm.Closure.
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)

	// Load Proto (X1), DirectEntryPtr (X2).
	asm.LDR(jit.X1, jit.X0, vmClosureOffProto)
	asm.LDR(jit.X2, jit.X1, funcProtoOffDirectEntryPtr)
	tailMissHaveEntryLabel := ec.uniqueLabel("t2tail_miss_have_entry")
	asm.CBNZ(jit.X2, tailMissHaveEntryLabel)
	asm.LDR(jit.X2, jit.X1, funcProtoOffTier2DirectEntryPtr)
	asm.Label(tailMissHaveEntryLabel)
	asm.CBZ(jit.X2, slowLabel)

	// Cache update on successful miss path.
	asm.LDR(jit.X4, mRegRegs, slotOffset(funcSlot))
	asm.STR(jit.X4, jit.X3, cacheOff)
	asm.STR(jit.X2, jit.X3, cacheOff+8)
	asm.STR(jit.X1, jit.X3, cacheOff+16)
	asm.LDR(jit.X4, jit.X1, funcProtoOffDirectEntryVersion)
	asm.STR(jit.X4, jit.X3, cacheOff+24)
	asm.B(icDoneLabel)

	// --- IC Hit: validate direct-entry version, refreshing entry on change. ---
	asm.Label(icHitLabel)
	asm.LDR(jit.X2, jit.X3, cacheOff+8)
	asm.LDR(jit.X1, jit.X3, cacheOff+16)
	asm.LDR(jit.X4, jit.X3, cacheOff+24)
	asm.LDR(jit.X5, jit.X1, funcProtoOffDirectEntryVersion)
	tailICVersionOKLabel := ec.uniqueLabel("t2tail_ic_version_ok")
	asm.CMPreg(jit.X4, jit.X5)
	asm.BCond(jit.CondEQ, tailICVersionOKLabel)
	asm.LDR(jit.X4, jit.X1, funcProtoOffDirectEntryPtr)
	tailICHaveEntryLabel := ec.uniqueLabel("t2tail_ic_have_entry")
	asm.CBNZ(jit.X4, tailICHaveEntryLabel)
	asm.LDR(jit.X4, jit.X1, funcProtoOffTier2DirectEntryPtr)
	asm.CBZ(jit.X4, slowLabel)
	asm.Label(tailICHaveEntryLabel)
	asm.MOVreg(jit.X2, jit.X4)
	asm.STR(jit.X2, jit.X3, cacheOff+8)
	asm.STR(jit.X5, jit.X3, cacheOff+24)
	asm.Label(tailICVersionOKLabel)
	jit.EmitExtractPtr(asm, jit.X0, jit.X0)

	asm.Label(icDoneLabel)

	// Bounds check: callee window (at the TAIL base = 0) fits in register file.
	asm.LDR(jit.X3, jit.X1, funcProtoOffMaxStack)
	asm.LSLimm(jit.X3, jit.X3, 3)
	asm.ADDreg(jit.X3, jit.X3, mRegRegs)
	asm.LDR(jit.X4, mRegCtx, execCtxOffRegsEnd)
	asm.CMPreg(jit.X3, jit.X4)
	asm.BCond(jit.CondHI, slowLabel)

	// CallCount increment for tiering. A published Tier 2 entry means the
	// callee no longer needs hot tail calls to feed promotion counters.
	skipTailCallCountLabel := ec.uniqueLabel("t2tail_skip_callcount")
	asm.LDR(jit.X3, jit.X1, funcProtoOffTier2DirectEntryPtr)
	asm.CBNZ(jit.X3, skipTailCallCountLabel)
	asm.LDR(jit.X3, jit.X1, funcProtoOffCallCount)
	asm.ADDimm(jit.X3, jit.X3, 1)
	asm.STR(jit.X3, jit.X1, funcProtoOffCallCount)
	asm.CMPimm(jit.X3, tmDefaultTier2Threshold)
	asm.BCond(jit.CondEQ, slowLabel)
	asm.Label(skipTailCallCountLabel)

	// Step 3: Copy args to tail window regs[0..nArgs-1]. Forward order is
	// safe because src = funcSlot+1+i > dst = i for all i >= 0.
	for i := 0; i < nArgs; i++ {
		srcOff := slotOffset(funcSlot + 1 + i)
		dstOff := slotOffset(i)
		if srcOff == dstOff {
			continue
		}
		asm.LDR(jit.X3, mRegRegs, srcOff)
		asm.STR(jit.X3, mRegRegs, dstOff)
	}

	// Step 4: Set callee context. ctx.Regs is UNCHANGED (reuse frame).
	asm.LDR(mRegConsts, jit.X1, funcProtoOffConstants)
	asm.STR(mRegConsts, mRegCtx, execCtxOffConstants)
	asm.STR(jit.X0, mRegCtx, execCtxOffBaselineClosurePtr) // X0 = closure ptr
	asm.MOVimm16(jit.X3, 1)
	ec.emitStoreCallMode(jit.X3)
	asm.LDR(jit.X3, jit.X1, funcProtoOffGlobalValCachePtr)
	asm.STR(jit.X3, mRegCtx, execCtxOffBaselineGlobalCache)
	asm.LDR(jit.X3, jit.X1, funcProtoOffTier2GlobalCachePtr)
	asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalCache)
	asm.LDR(jit.X3, jit.X1, funcProtoOffTier2GlobalCacheGenPtr)
	asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalCacheGen)
	asm.LDR(jit.X3, jit.X1, funcProtoOffTier2GlobalIndexPtr)
	asm.STR(jit.X3, mRegCtx, execCtxOffTier2GlobalIndex)
	asm.LDR(jit.X3, jit.X1, funcProtoOffFieldCache)
	asm.STR(jit.X3, mRegCtx, execCtxOffBaselineFieldCache)
	asm.LDR(jit.X3, jit.X1, funcProtoOffFieldPolyCache)
	asm.STR(jit.X3, mRegCtx, execCtxOffBaselineFieldPolyCache)
	asm.LDR(jit.X3, jit.X1, funcProtoOffTableStringKeyCache)
	asm.STR(jit.X3, mRegCtx, execCtxOffBaselineTableStringKeyCache)
	// Persist the (unchanged) mRegRegs back to ctx.Regs so callee's
	// direct-entry reload sees the correct base.
	asm.STR(mRegRegs, mRegCtx, execCtxOffRegs)

	// Step 5: Inline our own epilogue — restore callee-saved regs, FP/LR,
	// deallocate frame. Do NOT emit RET; we'll BR to callee instead.
	// X2 (direct entry addr) must survive; none of the LDP writes touch X2.
	// The callee direct entry expects X0=ctx. Capture it before restoring
	// X19, whose saved value belongs to our caller rather than this frame.
	asm.MOVreg(jit.X0, mRegCtx)
	ec.emitRestoreCalleeSavedFPRs()
	asm.LDP(jit.X27, jit.X28, jit.SP, 80)
	asm.LDP(jit.X25, jit.X26, jit.SP, 64)
	asm.LDP(jit.X23, jit.X24, jit.SP, 48)
	asm.LDP(jit.X21, jit.X22, jit.SP, 32)
	asm.LDP(jit.X19, jit.X20, jit.SP, 16)
	asm.LDP(jit.X29, jit.X30, jit.SP, 0)
	asm.ADDimm(jit.SP, jit.SP, uint16(frameSize))

	// Step 6: Tail jump to callee's direct entry (no link register update).
	asm.BR(jit.X2)

	// Slow-path fallback: falls back to the exit-resume path (which handles
	// return value normally — so the following OpReturn still runs correctly).
	asm.Label(slowLabel)
	ec.emitCallExitFallback(instr, funcSlot, nArgs, nRets)
}

// emitNumericArgsInRegs (R124) materializes raw int64 args into X0..X(N-1)
// ahead of a BL t2_numeric_self_entry_N. Allocated SSA GPRs are X20-X23/X28,
// so using the destination ABI register as the load/unbox scratch cannot
// clobber another live raw argument source.
func (ec *emitContext) emitNumericArgsInRegs(instr *Instr, nParams int) {
	asm := ec.asm
	for i := 0; i < nParams; i++ {
		dst := jit.Reg(int(jit.X0) + i)
		src := ec.resolveRawInt(instr.Args[1+i].ID, dst)
		if src != dst {
			asm.MOVreg(dst, src)
		}
	}
}

// emitCallExitFallback emits the exit-resume sequence for a CALL that couldn't
// take the native BLR path. This is identical to emitCallExit but without the
// arg-store (args were already stored in emitCallNative step 1) and without
// re-spilling (already spilled in step 2).
//
// The fallback path also spills ALL active registers (not just live ones) because
// the Go-side exit handler may inspect any register in the register file.
func (ec *emitContext) emitCallExitFallback(instr *Instr, funcSlot, nArgs, nRets int) {
	asm := ec.asm

	// The selective spill from the native path only saved live-across-call values.
	// The Go-side handler needs all active registers in memory, so spill the rest.
	ec.recordExitResumeCheckSite(instr, ExitCallExit, callExitModifiedSlots(funcSlot, nRets), exitResumeCheckOptions{
		RequireCallFunc:   true,
		RequireRawIntArgs: ec.isNumericStaticSelfCall(instr),
	})
	ec.emitStoreAllActiveRegs()

	// Write call descriptor to ExecContext.
	ec.emitStoreCallExitDescriptor(callExitDescriptor{
		slot:    funcSlot,
		nArgs:   nArgs,
		nRets:   nRets,
		instrID: instr.ID,
	})

	// Set ExitCode = ExitCallExit and return to Go.
	ec.emitCallProtocolExitToGo(ExitCallExit)

	// Continue label: the resume entry jumps here after Go handles the call.
	continueLabel := ec.passLabel(fmt.Sprintf("call_continue_%d", instr.ID))
	asm.Label(continueLabel)

	// Reload all active registers from memory.
	ec.emitReloadAllActiveRegs()

	// Load call result from regs[funcSlot].
	asm.LDR(jit.X0, mRegRegs, slotOffset(funcSlot))
	ec.storeResultNB(jit.X0, instr.ID)

	// Record for deferred resume entry generation.
	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}
