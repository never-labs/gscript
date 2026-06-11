//go:build darwin && arm64

// tiering_exit_fast_q_eval.go implements the slim exit lane for the
// OpQEvalSessionEval op-exit in the Tier 2 execute loop.
//
// Background — why this is an exit lane and not a BLR'd Go helper: Tier 2
// native code runs on the goroutine stack (jit.CallJIT is a NOSPLIT asm
// trampoline; tier2_stack.go pre-grows the stack). The Go runtime never
// observes JIT frames today because a goroutine executing JIT code is not at
// an async-preemption-safe point, so suspendG/GC stack scans wait until the
// code returns to Go. A direct BLR from JIT code into a preemptible Go helper
// would break that invariant: while the helper runs, the runtime CAN suspend
// the goroutine, and gentraceback/copystack would then have to walk through
// the JIT frame sitting in the middle of the stack — a frame with no
// funcdata/stack map — which is a fatal "unknown pc" in stack scans and an
// unfixable pointer-adjustment hole in stack growth. The only sanctioned way
// to run Go code is to fully unwind out of native frames first, which is
// exactly what the op-exit does. (cgocallback avoids this only because C code
// runs on a separate non-Go stack; Tier 2 native code does not.)
//
// What this lane removes from the per-iteration round-trip instead:
//
//   - exitStats.record: a mutex + two map operations per iteration. The
//     session-eval op keeps its own lock-free atomic route counters
//     (QEvalSessionEvalStats), so the global exit-stat row is redundant for
//     this op; TestQEvalJITScriptRouting pins per-iteration scaling through
//     the lock-free planned counters.
//   - tryMidRunRefresh: builds a full Tier 2 speculation plan per exit
//     (NewTier2SpeculationPlanWithSuppressedGuardKinds). Session-eval op-exits
//     execute an already-lowered helper and mature no structural feedback, so
//     the loop cannot learn anything mid-run from them; profile refresh still
//     happens on every other exit kind and on function entry.
//   - resyncRegs: the planned/shell session-eval executors are host Go
//     functions over q session state; they never call back into the Leia VM
//     and cannot reallocate the register file, so ctx.Regs/RegsEnd and the
//     field-cache context stay valid.
//   - resume-offset map lookups: memoized per site (lock-free, idempotent).
//
// The lane only handles the success path with a known resume offset. Anything
// else (missing site/resume metadata, exit-resume checking enabled, perf
// stats, trace mode) falls through to the generic ExitOpExit path with
// identical semantics, and helper errors return through the exact same
// "tier2: op-exit: %w" wrap as the generic path.

package methodjit

import (
	"math"

	"github.com/never-labs/leia/internal/runtime"
)

// qEvalSessionEvalResumeOffset resolves (and memoizes per site) the native
// resume offset for an OpQEvalSessionEval exit. The memo is stored in the
// site's atomic cells; concurrent fills write the same value, so plain
// last-write-wins atomics are sufficient.
func (cf *CompiledFunction) qEvalSessionEvalResumeOffset(instrID int, numericPass bool) (int, bool) {
	site := cf.qEvalSessionEvalSite(instrID)
	if site == nil {
		return cf.resumeOffset(instrID, numericPass)
	}
	cell := &site.resumeOff
	if numericPass {
		cell = &site.resumeOffNumeric
	}
	if memo := cell.Load(); memo > 0 {
		return int(memo), true
	} else if memo < 0 {
		return 0, false
	}
	off, ok := cf.resumeOffset(instrID, numericPass)
	if !ok || off <= 0 || off > math.MaxInt32 {
		cell.Store(-1)
		return 0, false
	}
	cell.Store(int32(off))
	return off, true
}

// fastQEvalSessionEvalOpExit executes one OpQEvalSessionEval op-exit on the
// slim lane. It returns done=false without performing ANY side effects when
// the exit cannot be handled here (the caller then runs the generic ExitOpExit
// path). When done=true the helper has executed: err carries the helper error
// with unchanged semantics, and on success regs[result slot] holds the value
// and the returned code pointer resumes native execution.
func (tm *TieringManager) fastQEvalSessionEvalOpExit(cf *CompiledFunction, ctx *ExecContext, regs []runtime.Value, base int) (uintptr, bool, error) {
	if cf == nil {
		return 0, false, nil
	}
	absSlot := base + int(ctx.OpExitSlot)
	absArg1 := base + int(ctx.OpExitArg1)
	if absSlot < 0 || absSlot >= len(regs) || absArg1 < 0 || absArg1 >= len(regs) {
		return 0, false, nil
	}
	instrID := int(ctx.OpExitID)
	resumeOff, ok := cf.qEvalSessionEvalResumeOffset(instrID, ctx.ResumeNumericPass != 0)
	if !ok {
		return 0, false, nil
	}
	out, err := cf.executeQEvalSessionEval(instrID, int(ctx.OpExitAux), regs[absArg1])
	if err != nil {
		return 0, true, err
	}
	regs[absSlot] = out
	return tier2ExitResumeCodePtr(cf, ctx, resumeOff), true, nil
}
