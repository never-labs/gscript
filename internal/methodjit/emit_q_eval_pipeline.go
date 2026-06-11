//go:build darwin && arm64

package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/jit"
)

func (ec *emitContext) emitQEvalPipelinePlanNativeExit(instr *Instr) {
	asm := ec.asm

	resultSlot, hasSlot := ec.slotMap[instr.ID]
	if !hasSlot {
		resultSlot = ec.nextSlot
		ec.slotMap[instr.ID] = resultSlot
		ec.nextSlot++
	}

	gprLive, fprLive := ec.computeLiveAcrossCall(instr)
	ec.recordExitResumeCheckSiteWithLive(instr, ExitQEvalPipelinePlan, ec.exitResumeCheckLiveSlots(gprLive, fprLive), []int{resultSlot}, exitResumeCheckOptions{})
	ec.emitSpillSelectiveForCall(gprLive, fprLive)

	asm.LoadImm64(jit.X0, int64(resultSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitSlot)

	asm.LoadImm64(jit.X0, instr.Aux)
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitAux)

	asm.LoadImm64(jit.X0, int64(instr.ID))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitID)

	continueLabel := ec.passLabel(fmt.Sprintf("q_eval_pipeline_continue_%d", instr.ID))

	// R6-S direct helper call (alternate-stack mode): BLR straight into the
	// Go helper thunk instead of unwinding through the ExitQEvalPipelinePlan
	// exit protocol (epilogue + Go dispatch + exit-stat record + resume
	// re-entry). The bridge (tier2_alt_stack.go) runs the same
	// executeQEvalPipelinePlanExit handler on the same op-exit descriptor
	// staged above (route and exit-stat recording preserved), then resumes
	// here with X19-X28/D8-D11 intact. Falls back to the generic exit when
	// the execution is not on a JIT alternate stack (ctx.JITStackHdr == 0:
	// legacy trampoline paths, Diagnose, native-callee resume loops).
	// Terminal-return plans need no special casing: the code after the site
	// simply continues to the function's own return sequence, producing the
	// identical result the generic path returns early.
	if tier2AltStackEnabled() && ec.selfCFCell != nil {
		helperErrLabel := ec.uniqueLabel(fmt.Sprintf("q_pipeline_helper_err_%d", instr.ID))
		genericExitLabel := ec.uniqueLabel(fmt.Sprintf("q_pipeline_helper_exit_%d", instr.ID))
		// The dedicated ExitQEvalPipelinePlan exit does not stage OpExitOp;
		// the bridge dispatches on it, so stage it here (the generic path
		// below never reads it). Staged before the hdr load: the thunk ABI
		// requires X0 = hdr at the BLR.
		asm.LoadImm64(jit.X0, int64(OpQEvalPipelinePlan))
		asm.STR(jit.X0, mRegCtx, execCtxOffOpExitOp)
		ec.emitDirectHelperSelfCF()
		asm.LDR(jit.X0, mRegCtx, execCtxOffJITStackHdr)
		asm.CBZ(jit.X0, genericExitLabel)
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
	asm.LoadImm64(jit.X0, int64(ExitQEvalPipelinePlan))
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
