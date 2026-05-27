//go:build darwin && arm64

// emit_deopt.go: deoptimization helpers for the Method JIT. When a guard or
// unsupported operation fires, these emit the bailout sequence that sets
// ExecContext.ExitCode and returns to the Go-side interpreter.
// Pure code movement from emit_call.go; no behavior change.

package methodjit

import (
	"github.com/gscript/gscript/internal/jit"
)

// emitDeopt emits ARM64 code that bails out to the interpreter.
// Sets ExecContext.ExitCode = ExitDeopt (2) and jumps to the deopt epilogue.
// R140: also writes instr.ID to ExecContext.DeoptInstrID so that post-
// deopt diagnostics (e.g., r138_ack_hang_test.go) can identify which
// specific guard fired without re-running the diag disassembler.
func (ec *emitContext) emitDeopt(instr *Instr) {
	asm := ec.asm
	if ec.numericMode {
		ec.emitStoreAllActiveRegs()
	}
	if instr != nil {
		asm.LoadImm64(jit.X0, int64(instr.ID))
		asm.STR(jit.X0, mRegCtx, execCtxOffDeoptInstrID)
	}
	asm.LoadImm64(jit.X0, int64(ExitDeopt))
	asm.STR(jit.X0, mRegCtx, execCtxOffExitCode)
	if ec.numericMode {
		asm.B("num_deopt_epilogue")
		return
	}
	asm.B("deopt_epilogue")
}

// emitPreciseDeopt flushes the current Tier 2 frame and asks the VM to resume
// the interpreter at instr.SourcePC. It is used by guards that can fire after a
// side effect has already executed in the same frame.
func (ec *emitContext) emitPreciseDeopt(instr *Instr) {
	if !ec.canPreciseDeopt(instr) {
		ec.emitDeopt(instr)
		return
	}
	ec.emitStoreAllActiveRegs()
	ec.asm.LoadImm64(jit.X0, int64(instr.SourcePC))
	ec.asm.STR(jit.X0, mRegCtx, execCtxOffExitResumePC)
	ec.emitDeopt(instr)
}

func (ec *emitContext) canPreciseDeopt(instr *Instr) bool {
	if instr == nil || !instr.HasSource || instr.SourcePC < 0 {
		return false
	}
	if instr.SourceProto == nil {
		return true
	}
	if ec == nil || ec.fn == nil || ec.fn.Proto == nil {
		return false
	}
	return instr.SourceProto == ec.fn.Proto
}
