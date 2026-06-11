//go:build darwin && arm64

// emit_q_eval_session_eval.go emits the OpQEvalSessionEval op-exit with
// selective spill. The op fires once per loop iteration in session-eval hot
// loops, so the generic emitOpExit full spill/reload of every active register
// is the dominant native-side cost; mirroring the OpQEvalPipelinePlan native
// exit, only values live across the exit are spilled and reloaded. The exit
// keeps the generic ExitOpExit code and descriptor layout, so both Go-side
// handlers (ExecContext op-exit and tiering-manager op-exit) dispatch
// unchanged.

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
)

// emitDirectHelperSelfCF publishes this function's OWN CompiledFunction to
// ctx.HelperCF immediately before a direct helper BLR. The Go side seeds
// ctx.HelperCF with the entry function's cf, which is wrong for sites that
// execute inside a natively-BLR'd callee (the ExecContext is shared across
// the whole native execution); the self-cf cell (emit_compile.go) carries
// the containing function's cf, resolved through one indirection because
// the CompiledFunction does not exist yet at emit time. Clobbers X16.
func (ec *emitContext) emitDirectHelperSelfCF() {
	asm := ec.asm
	asm.LoadImm64(jit.X16, int64(uintptr(unsafe.Pointer(ec.selfCFCell))))
	asm.LDR(jit.X16, jit.X16, 0)
	asm.STR(jit.X16, mRegCtx, execCtxOffHelperCF)
}

func (ec *emitContext) emitQEvalSessionEvalExit(instr *Instr) {
	if instr == nil || len(instr.Args) != 1 || instr.Args[0] == nil {
		ec.emitOpExit(instr)
		return
	}
	asm := ec.asm

	resultSlot, hasSlot := ec.slotMap[instr.ID]
	if !hasSlot {
		resultSlot = ec.nextSlot
		ec.slotMap[instr.ID] = resultSlot
		ec.nextSlot++
	}
	recvID := instr.Args[0].ID
	recvSlot, hasRecvSlot := ec.slotMap[recvID]
	if !hasRecvSlot {
		recvSlot = ec.nextSlot
		ec.slotMap[recvID] = recvSlot
		ec.nextSlot++
	}

	gprLive, fprLive := ec.computeLiveAcrossCall(instr)
	ec.recordExitResumeCheckSiteWithLive(instr, ExitOpExit, ec.exitResumeCheckLiveSlots(gprLive, fprLive), []int{resultSlot}, exitResumeCheckOptions{})

	// Stage the receiver session value into its home slot: the Go handler
	// reads regs[base+recvSlot], and selective spill only covers values live
	// AFTER the exit (the receiver may not be, e.g. on the last use).
	recvReg := ec.resolveValueNB(recvID, jit.X0)
	if recvReg != jit.X0 {
		asm.MOVreg(jit.X0, recvReg)
	}
	asm.STR(jit.X0, mRegRegs, slotOffset(recvSlot))
	ec.emitExitResumeCheckShadowStoreGPR(recvSlot, jit.X0)

	ec.emitSpillSelectiveForCall(gprLive, fprLive)

	asm.LoadImm64(jit.X0, int64(instr.Op))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitOp)

	asm.LoadImm64(jit.X0, int64(resultSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitSlot)

	asm.LoadImm64(jit.X0, int64(recvSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitArg1)

	asm.LoadImm64(jit.X0, 0)
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitArg2)

	asm.LoadImm64(jit.X0, instr.Aux)
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitAux)

	asm.LoadImm64(jit.X0, int64(instr.ID))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitID)

	continueLabel := ec.passLabel(fmt.Sprintf("op_continue_%d", instr.ID))

	// R5-K direct helper call (alternate-stack mode): BLR straight into the
	// Go helper thunk instead of unwinding through the op-exit protocol.
	// The thunk (jit.JITHelperEntryPC) switches SP back to the goroutine
	// stack, runs the bridge (tier2_alt_stack.go) with the op-exit
	// descriptor already staged in ctx above, and resumes here with
	// X19-X28/D8-D11 preserved. Falls back to the generic op-exit when the
	// execution is not on a JIT alternate stack (ctx.JITStackHdr == 0:
	// legacy trampoline paths, Diagnose, native-callee resume loops).
	if tier2AltStackEnabled() && ec.selfCFCell != nil {
		helperErrLabel := ec.uniqueLabel(fmt.Sprintf("q_helper_err_%d", instr.ID))
		genericExitLabel := ec.uniqueLabel(fmt.Sprintf("q_helper_exit_%d", instr.ID))
		asm.LDR(jit.X0, mRegCtx, execCtxOffJITStackHdr)
		asm.CBZ(jit.X0, genericExitLabel)
		ec.emitDirectHelperSelfCF()
		asm.LoadImm64(jit.X16, int64(jit.JITHelperEntryPC()))
		asm.BLR(jit.X16)
		asm.LDR(jit.X16, mRegCtx, execCtxOffHelperErrFlag)
		asm.CBNZ(jit.X16, helperErrLabel)
		// Success: result already in its home slot; rejoin the shared
		// reload/continue tail.
		asm.B(continueLabel)
		asm.Label(helperErrLabel)
		asm.LoadImm64(jit.X0, int64(ExitQEvalHelperErr))
		asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
		if ec.numericMode {
			asm.B("num_deopt_epilogue")
		} else {
			asm.B("deopt_epilogue")
		}
		asm.Label(genericExitLabel)
	}

	ec.emitSetResumeNumericPass()
	asm.LoadImm64(jit.X0, int64(ExitOpExit))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}

	asm.Label(continueLabel)

	ec.emitReloadSelectiveForCall(gprLive, fprLive)
	asm.LDR(jit.X0, mRegRegs, slotOffset(resultSlot))
	ec.storeResultNB(jit.X0, instr.ID)

	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}
