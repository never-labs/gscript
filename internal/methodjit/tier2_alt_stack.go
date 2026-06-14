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
	"errors"
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

type tier2DirectHelperFrame struct {
	cf   *CompiledFunction
	regs []runtime.Value
	base int
}

func tier2DirectHelperFrameFor(ctx *ExecContext) (tier2DirectHelperFrame, bool) {
	cf := (*CompiledFunction)(unsafe.Pointer(ctx.HelperCF))
	if cf == nil {
		ctx.HelperErr = fmt.Errorf("tier2: direct helper: nil CompiledFunction")
		ctx.HelperErrFlag = 1
		return tier2DirectHelperFrame{}, false
	}
	regs, base, ok := helperRegsWindow(ctx)
	if !ok {
		ctx.HelperErr = fmt.Errorf("tier2: direct helper: invalid register window")
		ctx.HelperErrFlag = 1
		return tier2DirectHelperFrame{}, false
	}
	return tier2DirectHelperFrame{cf: cf, regs: regs, base: base}, true
}

func tier2DirectHelperAbs(regs []runtime.Value, base int, rel int64) (int, bool) {
	abs := base + int(rel)
	return abs, abs >= 0 && abs < len(regs)
}

func tier2DirectHelperStore(ctx *ExecContext, regs []runtime.Value, slot int, out runtime.Value, err error) bool {
	if err != nil {
		ctx.HelperErr = err
		ctx.HelperErrFlag = 1
		return false
	}
	regs[slot] = out
	ctx.HelperErrFlag = 0
	return true
}

func tier2DirectHelperUnary(ctx *ExecContext, frame tier2DirectHelperFrame, kernel string) (int, int, bool) {
	slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
	arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
	if !okSlot || !okArg1 {
		ctx.HelperErr = fmt.Errorf("tier2: direct helper: %s register range out of bounds", kernel)
		ctx.HelperErrFlag = 1
		return 0, 0, false
	}
	return slot, arg1, true
}

func tier2DirectHelperBinary(ctx *ExecContext, frame tier2DirectHelperFrame, kernel string) (int, int, int, bool) {
	slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
	arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
	arg2, okArg2 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg2)
	if !okSlot || !okArg1 || !okArg2 {
		ctx.HelperErr = fmt.Errorf("tier2: direct helper: %s register range out of bounds", kernel)
		ctx.HelperErrFlag = 1
		return 0, 0, 0, false
	}
	return slot, arg1, arg2, true
}

func tier2DirectHelperConstant(ctx *ExecContext, frame tier2DirectHelperFrame, aux int, message string) (runtime.Value, bool) {
	if frame.cf.Proto == nil || aux < 0 || aux >= len(frame.cf.Proto.Constants) {
		ctx.HelperErr = errors.New(message)
		ctx.HelperErrFlag = 1
		return runtime.NilValue(), false
	}
	return frame.cf.Proto.Constants[aux], true
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
	case OpQEvalPipelinePlan:
		// R6-S: direct lane for the ExitQEvalPipelinePlan protocol. Runs the
		// exact generic handler (executeQEvalPipelinePlanExit on the
		// typed_runtime_native_exit route) against the live register file.
		// The generic path's post-helper resyncRegs is not needed here: like
		// the session-eval executors, the q pipeline backends are host Go
		// functions over q runtime state that never call back into the Leia
		// VM, so the register file cannot be reallocated while they run.
		cf := (*CompiledFunction)(unsafe.Pointer(ctx.HelperCF))
		if cf == nil {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: nil CompiledFunction")
			ctx.HelperErrFlag = 1
			return
		}
		regs, base, ok := helperRegsWindow(ctx)
		if !ok {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: invalid register window")
			ctx.HelperErrFlag = 1
			return
		}
		// Diagnostic exit-stat parity: the generic executeTier2 path records
		// every ExitQEvalPipelinePlan before handling it (exit_stats.go);
		// keep ExitStats observable behavior identical in direct mode.
		if tm := ctx.HelperTM; tm != nil {
			savedExit := ctx.ExitCode
			ctx.ExitCode = ExitQEvalPipelinePlan
			tm.recordTier2Exit(cf.Proto, cf, ctx)
			ctx.ExitCode = savedExit
		}
		if err := cf.executeQEvalPipelinePlanExit(ctx, regs, base, qEvalPipelineExecutionRouteNativeExit); err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		ctx.HelperErrFlag = 0
	case OpQSQLKernelPlan:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, ok := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		if !ok {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QSQLKernelPlan register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if err := frame.cf.executeQSQLKernelPlanSlotWithRoute(
			int(ctx.OpExitAux),
			slot,
			frame.regs,
			string(qTypedRuntimeExecutionRouteDirectHelper),
		); err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		ctx.HelperErrFlag = 0
	case OpQVectorWhereReduce:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		tempBase := frame.base + int(ctx.OpExitArg1)
		nArgs := int(ctx.OpExitArg2)
		if !okSlot || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(frame.regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QVectorWhereReduce register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeQVectorWhereReduce(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			frame.regs[tempBase],
			frame.regs[tempBase+1],
			frame.regs[tempBase+2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpQVectorGatherReduce:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "QVectorGatherReduce")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeQVectorGatherReduce(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpVectorGather:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
		arg2, okArg2 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg2)
		if !okSlot || !okArg1 || !okArg2 {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: VectorGather register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeVectorGather(
			int(ctx.OpExitID),
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpVectorCompare:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
		arg2, okArg2 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg2)
		if !okSlot || !okArg1 || !okArg2 {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: VectorCompare register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeVectorCompare(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpVectorMask:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
		arg2, okArg2 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg2)
		if !okSlot || !okArg1 || !okArg2 {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: VectorMask register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeVectorMask(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpVectorWhere:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		tempBase := frame.base + int(ctx.OpExitArg1)
		nArgs := int(ctx.OpExitArg2)
		if !okSlot || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(frame.regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: VectorWhere register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeVectorWhere(
			int(ctx.OpExitID),
			frame.regs[tempBase],
			frame.regs[tempBase+1],
			frame.regs[tempBase+2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpVectorReduce:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
		if !okSlot || !okArg1 {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: VectorReduce register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeVectorReduce(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			frame.regs[arg1],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpVectorScan:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, okSlot := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitSlot)
		arg1, okArg1 := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg1)
		if !okSlot || !okArg1 {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: VectorScan register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeVectorScan(
			int(ctx.OpExitID),
			frame.regs[arg1],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameLen:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameLen")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameLen(frame.regs[arg1], qTypedRuntimeExecutionRouteDirectHelper)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameColumn:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameColumn")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		column, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameColumn column name constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameColumn(
			frame.regs[arg1],
			column,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameMask:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameMask")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		spec, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameMask spec constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameMask(
			frame.regs[arg1],
			spec,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameProject:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameProject")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		columns, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameProject column list constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameProject(
			frame.regs[arg1],
			columns,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameFilter:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "FrameFilter")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilter(
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameFilterProject:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "FrameFilterProject")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		columns, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameFilterProject column list constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilterProject(
			frame.regs[arg1],
			frame.regs[arg2],
			columns,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameGather:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "FrameGather")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameGather(
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameSlice:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "FrameSlice")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameSlice(
			frame.regs[arg1],
			frame.regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameOrder:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameOrder")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		spec, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameOrder spec constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameOrder(
			frame.regs[arg1],
			spec,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameOrderGather:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameOrderGather")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		spec, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameOrderGather spec constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameOrderGather(
			frame.regs[arg1],
			spec,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameProjectColumn:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "FrameProjectColumn")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		spec, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameProjectColumn spec constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameProjectColumn(
			frame.regs[arg1],
			spec,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameFilterProjectColumn:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "FrameFilterProjectColumn")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		spec, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameFilterProjectColumn spec constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilterProjectColumn(
			frame.regs[arg1],
			frame.regs[arg2],
			spec,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpFrameGroupAggregate:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, arg2, ok := tier2DirectHelperBinary(ctx, frame, "FrameGroupAggregate")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		spec, ok := tier2DirectHelperConstant(ctx, frame, aux, "tier2: direct helper: FrameGroupAggregate spec constant is out of range")
		if !ok {
			return
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeFrameGroupAggregate(
			frame.regs[arg1],
			frame.regs[arg2],
			spec,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	case OpQFrameSelectColumn:
		frame, ok := tier2DirectHelperFrameFor(ctx)
		if !ok {
			return
		}
		slot, arg1, ok := tier2DirectHelperUnary(ctx, frame, "QFrameSelectColumn")
		if !ok {
			return
		}
		aux := int(ctx.OpExitAux)
		if frame.cf.Proto == nil || aux < 0 || aux >= len(frame.cf.QFrameSelectColumnSpecs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QFrameSelectColumn spec index is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		argVal := runtime.NilValue()
		hasArg := false
		if ctx.OpExitArg2 >= 0 {
			arg2, ok := tier2DirectHelperAbs(frame.regs, frame.base, ctx.OpExitArg2)
			if !ok {
				ctx.HelperErr = fmt.Errorf("tier2: direct helper: QFrameSelectColumn dynamic arg out of bounds")
				ctx.HelperErrFlag = 1
				return
			}
			argVal = frame.regs[arg2]
			hasArg = true
		}
		out, err := frame.cf.qFrameVectorRuntimeExecutionAdapter().executeQFrameSelectColumn(
			frame.cf.Proto.Constants,
			aux,
			frame.regs[arg1],
			argVal,
			hasArg,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		tier2DirectHelperStore(ctx, frame.regs, slot, out, err)
	default:
		ctx.HelperErr = fmt.Errorf("tier2: direct helper: unsupported op %v", Op(ctx.OpExitOp))
		ctx.HelperErrFlag = 1
	}
}

// helperRegsWindow reconstructs the VM register file slice and the frame base
// from the ExecContext pointers (Regs = &regs[base], RegsBase = &regs[0],
// RegsEnd = &regs[len]). The pointers are maintained by the Go-side execute
// loops across every event that can reallocate the file, so they are current
// whenever native code runs.
func helperRegsWindow(ctx *ExecContext) ([]runtime.Value, int, bool) {
	if ctx == nil || ctx.RegsBase == 0 || ctx.RegsEnd <= ctx.RegsBase || ctx.Regs < ctx.RegsBase {
		return nil, 0, false
	}
	n := int((ctx.RegsEnd - ctx.RegsBase) / uintptr(jit.ValueSize))
	base := int((ctx.Regs - ctx.RegsBase) / uintptr(jit.ValueSize))
	if n <= 0 || base < 0 || base > n {
		return nil, 0, false
	}
	return unsafe.Slice((*runtime.Value)(unsafe.Pointer(ctx.RegsBase)), n), base, true
}
