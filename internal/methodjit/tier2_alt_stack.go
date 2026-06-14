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
	case OpQVectorWhereReduce:
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
		slot := base + int(ctx.OpExitSlot)
		tempBase := base + int(ctx.OpExitArg1)
		nArgs := int(ctx.OpExitArg2)
		if slot < 0 || slot >= len(regs) || tempBase < 0 || nArgs != 3 || tempBase+nArgs > len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QVectorWhereReduce register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeQVectorWhereReduce(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			regs[tempBase],
			regs[tempBase+1],
			regs[tempBase+2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpQVectorGatherReduce:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QVectorGatherReduce register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeQVectorGatherReduce(
			int(ctx.OpExitID),
			int(ctx.OpExitAux),
			regs[arg1],
			regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameLen:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameLen register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameLen(regs[arg1], qTypedRuntimeExecutionRouteDirectHelper)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameColumn:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameColumn register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameColumn column name constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameColumn(
			regs[arg1],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameMask:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameMask register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameMask spec constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameMask(
			regs[arg1],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameProject:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameProject register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameProject column list constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameProject(
			regs[arg1],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameFilter:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameFilter register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilter(
			regs[arg1],
			regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameFilterProject:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameFilterProject register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameFilterProject column list constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilterProject(
			regs[arg1],
			regs[arg2],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameGather:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameGather register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameGather(
			regs[arg1],
			regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameSlice:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameSlice register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameSlice(
			regs[arg1],
			regs[arg2],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameOrder:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameOrder register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameOrder spec constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameOrder(
			regs[arg1],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameOrderGather:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameOrderGather register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameOrderGather spec constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameOrderGather(
			regs[arg1],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameProjectColumn:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameProjectColumn register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameProjectColumn spec constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameProjectColumn(
			regs[arg1],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameFilterProjectColumn:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameFilterProjectColumn register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameFilterProjectColumn spec constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameFilterProjectColumn(
			regs[arg1],
			regs[arg2],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpFrameGroupAggregate:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		arg2 := base + int(ctx.OpExitArg2)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) || arg2 < 0 || arg2 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameGroupAggregate register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.Proto.Constants) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: FrameGroupAggregate spec constant is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeFrameGroupAggregate(
			regs[arg1],
			regs[arg2],
			cf.Proto.Constants[aux],
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
	case OpQFrameSelectColumn:
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
		slot := base + int(ctx.OpExitSlot)
		arg1 := base + int(ctx.OpExitArg1)
		aux := int(ctx.OpExitAux)
		if slot < 0 || slot >= len(regs) || arg1 < 0 || arg1 >= len(regs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QFrameSelectColumn register range out of bounds")
			ctx.HelperErrFlag = 1
			return
		}
		if cf.Proto == nil || aux < 0 || aux >= len(cf.QFrameSelectColumnSpecs) {
			ctx.HelperErr = fmt.Errorf("tier2: direct helper: QFrameSelectColumn spec index is out of range")
			ctx.HelperErrFlag = 1
			return
		}
		argVal := runtime.NilValue()
		hasArg := false
		if ctx.OpExitArg2 >= 0 {
			arg2 := base + int(ctx.OpExitArg2)
			if arg2 < 0 || arg2 >= len(regs) {
				ctx.HelperErr = fmt.Errorf("tier2: direct helper: QFrameSelectColumn dynamic arg out of bounds")
				ctx.HelperErrFlag = 1
				return
			}
			argVal = regs[arg2]
			hasArg = true
		}
		out, err := cf.qFrameVectorRuntimeExecutionAdapter().executeQFrameSelectColumn(
			cf.Proto.Constants,
			aux,
			regs[arg1],
			argVal,
			hasArg,
			qTypedRuntimeExecutionRouteDirectHelper,
		)
		if err != nil {
			ctx.HelperErr = err
			ctx.HelperErrFlag = 1
			return
		}
		regs[slot] = out
		ctx.HelperErrFlag = 0
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
