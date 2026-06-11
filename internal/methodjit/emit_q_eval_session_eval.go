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

	"github.com/never-labs/leia/internal/jit"
)

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

	ec.emitSetResumeNumericPass()
	asm.LoadImm64(jit.X0, int64(ExitOpExit))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}

	continueLabel := ec.passLabel(fmt.Sprintf("op_continue_%d", instr.ID))
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
