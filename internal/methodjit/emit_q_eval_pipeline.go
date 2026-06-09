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

	ec.recordExitResumeCheckSite(instr, ExitQEvalPipelinePlan, []int{resultSlot}, exitResumeCheckOptions{})
	ec.emitStoreAllActiveRegs()

	asm.LoadImm64(jit.X0, int64(resultSlot))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitSlot)

	asm.LoadImm64(jit.X0, instr.Aux)
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitAux)

	asm.LoadImm64(jit.X0, int64(instr.ID))
	asm.STR(jit.X0, mRegCtx, execCtxOffOpExitID)

	ec.emitSetResumeNumericPass()
	asm.LoadImm64(jit.X0, int64(ExitQEvalPipelinePlan))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
	} else {
		asm.B("deopt_epilogue")
	}

	continueLabel := ec.passLabel(fmt.Sprintf("q_eval_pipeline_continue_%d", instr.ID))
	asm.Label(continueLabel)

	ec.emitReloadAllActiveRegs()
	asm.LDR(jit.X0, mRegRegs, slotOffset(resultSlot))
	ec.storeResultNB(jit.X0, instr.ID)

	ec.callExitIDs = append(ec.callExitIDs, instr.ID)
	ec.deferredResumes = append(ec.deferredResumes, deferredResume{
		instrID:       instr.ID,
		continueLabel: continueLabel,
		numericPass:   ec.numericMode,
	})
}
