//go:build darwin && arm64

// tier2_alt_stack.go wires Tier 2 execution onto a dedicated non-Go stack
// (R5-K feasibility prototype, gated by LEIA_JIT_ALT_STACK=1).
//
// Motivation: with JIT frames on the goroutine stack, every JIT->Go
// interaction must fully unwind through the exit protocol because the Go
// runtime cannot scan/copy across funcdata-less JIT frames. On a dedicated
// mmap'd stack (internal/jit.JITStack), JIT frames are invisible to the
// runtime, and JIT code can BLR directly into preemptible Go helpers via
// jit.JITHelperEntryPC: the thunk switches SP back to the goroutine stack
// and opens a real, unwindable frame whose fabricated return PC points at
// the Go caller of the trampoline (the cgocallback technique). See
// internal/jit/trampoline_stack_arm64.s for the precise discipline.
//
// What this enables here: the OpQEvalSessionEval per-iteration helper is
// called directly from native code (emit_q_eval_session_eval.go), skipping
// the JIT epilogue + Go dispatch loop + resume-entry prologue round trip of
// the op-exit path (even its slim lane, tiering_exit_fast_q_eval.go).
//
// Safety invariants:
//   - One in-flight helper per JIT stack. Helpers may run arbitrary Go
//     (preemption, GC, stack growth, panic) and may even recursively start
//     nested Tier 2 executions — those acquire their own stacks from the
//     pool. They must never resume the suspended JIT stack themselves.
//   - ctx.JITStackHdr must be zero whenever native code might run on the
//     goroutine stack with the same ExecContext (legacy jit.CallJIT paths,
//     e.g. the native-callee resume loop); direct-call sites CBZ on it and
//     fall back to the generic op-exit.
//   - A helper panic unwinds the fabricated chain into executeTier2's
//     caller, abandoning the JIT stack mid-frame. The deferred release
//     returns the stack to the pool; the next acquire starts fresh from the
//     header, so no cleanup is needed.

package methodjit

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/never-labs/leia/internal/jit"
	"github.com/never-labs/leia/internal/runtime"
)

// tier2DirectHelperCalls counts direct BLR Go-helper calls dispatched by the
// bridge. Diagnostic: lets tests/benches verify the direct lane actually
// engages (vs. silently falling back to the generic op-exit).
var tier2DirectHelperCalls atomic.Uint64

// Tier2DirectHelperCallCount returns the number of direct BLR helper calls
// dispatched since process start.
func Tier2DirectHelperCallCount() uint64 { return tier2DirectHelperCalls.Load() }

// tier2AltStackEnv gates both the alternate-stack execution mode and the
// emission of direct helper-call sites. Read once: emission and execution
// must agree for the lifetime of the process.
var tier2AltStackEnv = os.Getenv("LEIA_JIT_ALT_STACK") == "1"

func tier2AltStackEnabled() bool { return tier2AltStackEnv }

// tier2AltStackSize matches the existing goroutine-stack reserve budget
// (ensureTier2NativeStack pre-grows 128 KiB) with generous headroom for
// native BLR recursion.
const tier2AltStackSize = 512 << 10

var tier2AltStackPool = sync.Pool{}

// acquireTier2AltStack returns a pooled (or fresh) JIT stack, or nil if
// allocation fails (callers then fall back to the legacy trampoline).
func acquireTier2AltStack() *jit.JITStack {
	if s, ok := tier2AltStackPool.Get().(*jit.JITStack); ok && s != nil {
		return s
	}
	s, err := jit.NewJITStack(tier2AltStackSize)
	if err != nil {
		return nil
	}
	return s
}

func releaseTier2AltStack(s *jit.JITStack) {
	if s != nil {
		tier2AltStackPool.Put(s)
	}
}

func init() {
	jit.SetJITHelperBridge(tier2JITHelperBridge)
}

// tier2JITHelperBridge is the Go-side dispatcher for direct BLR helper calls
// from Tier 2 native code running on a JIT alternate stack. It executes on
// the goroutine stack with full Go semantics. Arguments arrive through the
// same ExecContext op-exit fields the generic exit path uses; results are
// written straight into the register file (heap-allocated; kept alive by the
// executeTier2 frame, which the runtime can see through the fabricated
// unwind chain).
func tier2JITHelperBridge(ctxPtr uintptr) {
	ctx := (*ExecContext)(unsafe.Pointer(ctxPtr))
	tier2DirectHelperCalls.Add(1)
	switch Op(ctx.OpExitOp) {
	case OpQEvalSessionEval:
		cf := (*CompiledFunction)(unsafe.Pointer(ctx.HelperCF))
		if cf == nil {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: nil CompiledFunction")
			ctx.HelperErrFlag = 1
			return
		}
		// Slot offsets are compile-time constants validated against the
		// register budget at entry; ctx.Regs points at regs[base].
		argp := (*runtime.Value)(unsafe.Pointer(ctx.Regs + uintptr(ctx.OpExitArg1)*uintptr(jit.ValueSize)))
		out, err := cf.executeQEvalSessionEval(int(ctx.OpExitID), int(ctx.OpExitAux), *argp)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		*(*runtime.Value)(unsafe.Pointer(ctx.Regs + uintptr(ctx.OpExitSlot)*uintptr(jit.ValueSize))) = out
		ctx.HelperErrFlag = 0
	default:
		ctx.HelperErr = fmt.Errorf("tier2: direct helper: unsupported op %v", Op(ctx.OpExitOp))
		ctx.HelperErrFlag = 1
	}
}
