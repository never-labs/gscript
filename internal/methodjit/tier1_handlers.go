//go:build darwin && arm64

// tier1_handlers.go contains the primary Tier 1 baseline JIT exit handlers.
// These handle the most common operations that the baseline JIT exits to Go for:
// calls, globals, tables, and field access.

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

// handleBaselineOpExit dispatches a baseline op-exit to the appropriate handler.
func (e *BaselineJITEngine) handleBaselineOpExit(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, bf *BaselineFunc) error {
	opCode := vm.Opcode(ctx.BaselineOp)
	switch opCode {
	case vm.OP_CALL:
		return e.handleCall(ctx, regs, base, proto)
	case vm.OP_YIELD:
		return e.handleYield(ctx, regs, base, proto, bf)
	case vm.OP_RESUME:
		return e.handleResume(ctx, regs, base, proto)
	case vm.OP_GETGLOBAL:
		return e.handleGetGlobal(ctx, regs, base, proto, bf)
	case vm.OP_SETGLOBAL:
		return e.handleSetGlobal(ctx, regs, base, proto)
	case vm.OP_NEWTABLE:
		return e.handleNewTable(ctx, regs, base, proto, bf)
	case vm.OP_NEWOBJECT2:
		return e.handleNewObject2(ctx, regs, base, proto, bf)
	case vm.OP_NEWOBJECTN:
		return e.handleNewObjectN(ctx, regs, base, proto, bf)
	case vm.OP_GETTABLE:
		return e.handleGetTable(ctx, regs, base, proto)
	case vm.OP_SETTABLE:
		return e.handleSetTable(ctx, regs, base, proto)
	case vm.OP_GETFIELD:
		return e.handleGetField(ctx, regs, base, proto)
	case vm.OP_SETFIELD:
		return e.handleSetField(ctx, regs, base, proto)
	case vm.OP_SETLIST:
		return e.handleSetList(ctx, regs, base, proto)
	case vm.OP_APPEND:
		return e.handleAppend(ctx, regs, base, proto)
	case vm.OP_CONCAT:
		return e.handleConcat(ctx, regs, base, proto)
	case vm.OP_LEN:
		return e.handleLen(ctx, regs, base, proto)
	case vm.OP_CLOSURE:
		return e.handleClosure(ctx, regs, base, proto)
	case vm.OP_CLOSE:
		return e.handleClose(ctx, regs, base, proto)
	case vm.OP_GETUPVAL:
		return e.handleGetUpval(ctx, regs, base, proto)
	case vm.OP_SETUPVAL:
		return e.handleSetUpval(ctx, regs, base, proto)
	case vm.OP_SELF:
		return e.handleSelf(ctx, regs, base, proto)
	case vm.OP_VARARG:
		return e.handleVararg(ctx, regs, base, proto)
	case vm.OP_TFORCALL:
		return e.handleTForCall(ctx, regs, base, proto)
	case vm.OP_TFORLOOP:
		return e.handleTForLoop(ctx, regs, base, proto)
	case vm.OP_POW:
		return e.handlePow(ctx, regs, base, proto)
	case vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD:
		return e.handleArithmetic(ctx, regs, base, proto)
	case vm.OP_BAND, vm.OP_BOR, vm.OP_BXOR, vm.OP_BANDN, vm.OP_SHL, vm.OP_SHR, vm.OP_BNOT:
		return e.handleBitwise(ctx, regs, base, proto)
	case vm.OP_LT:
		return e.handleLT(ctx, regs, base, proto)
	case vm.OP_LE:
		return e.handleLE(ctx, regs, base, proto)
	default:
		return fmt.Errorf("unhandled baseline op-exit: %s (%d)", vm.OpName(opCode), opCode)
	}
}

func (e *BaselineJITEngine) handleBitwise(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	opCode := vm.Opcode(ctx.BaselineOp)
	a := int(ctx.BaselineA)
	b := int(ctx.BaselineB)
	c := int(ctx.BaselineC)
	absA := base + a

	loadRK := func(idx int) runtime.Value {
		if idx >= vm.RKBit {
			return proto.Constants[idx-vm.RKBit]
		}
		return regs[base+idx]
	}
	bv := loadRK(b)

	if opCode == vm.OP_BNOT {
		n, err := methodJITBitwiseInt(bv)
		if err != nil {
			return err
		}
		if absA < len(regs) {
			regs[absA] = runtime.IntValue(^n)
		}
		return nil
	}

	cv := loadRK(c)
	x, err := methodJITBitwiseInt(bv)
	if err != nil {
		return err
	}
	y, err := methodJITBitwiseInt(cv)
	if err != nil {
		return err
	}

	var out int64
	switch opCode {
	case vm.OP_BAND:
		out = x & y
	case vm.OP_BOR:
		out = x | y
	case vm.OP_BXOR:
		out = x ^ y
	case vm.OP_BANDN:
		out = x &^ y
	case vm.OP_SHL:
		shift, err := methodJITBitwiseShift(cv)
		if err != nil {
			return err
		}
		if shift >= 64 {
			out = 0
		} else {
			out = int64(uint64(x) << shift)
		}
	case vm.OP_SHR:
		shift, err := methodJITBitwiseShift(cv)
		if err != nil {
			return err
		}
		if shift >= 64 {
			out = 0
		} else {
			out = int64(uint64(x) >> shift)
		}
	default:
		return fmt.Errorf("unsupported bitwise opcode for baseline op-exit: %s", vm.OpName(opCode))
	}
	if absA < len(regs) {
		regs[absA] = runtime.IntValue(out)
	}
	return nil
}

func methodJITBitwiseInt(v runtime.Value) (int64, error) {
	n, ok := v.ToNumber()
	if !ok {
		return 0, fmt.Errorf("attempt to perform bitwise operation on %s", v.TypeName())
	}
	if n.IsInt() {
		return n.Int(), nil
	}
	return int64(n.Float()), nil
}

func methodJITBitwiseShift(v runtime.Value) (uint, error) {
	n, err := methodJITBitwiseInt(v)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, fmt.Errorf("negative shift count")
	}
	return uint(n), nil
}

// handleNativeCallExit handles the case where a callee invoked via native BLR
// hits an exit-resume op mid-execution. The callee's exit state is in ctx.
//
// Rather than trying to resume the callee mid-execution (which is fragile with
// nested BLR calls — the exitHandleLabel chain overwrites BaselinePC at each
// level), we take the simpler approach:
//  1. Disable BLR for this callee (prevent future mid-execution exits)
//  2. Re-execute the callee from scratch via e.Execute() which handles all
//     op-exits correctly through its own exit-resume loop
//
// This only happens ONCE per callee (DirectEntryPtr is cleared), so the cost
// of re-execution is amortized across all future calls.
func (e *BaselineJITEngine) handleNativeCallExit(ctx *ExecContext, regs []runtime.Value, base int, callerProto *vm.FuncProto, callerBF *BaselineFunc) (runtime.Value, error) {
	calleeBaseOff := int(ctx.NativeCalleeBaseOff)
	calleeBase := base + calleeBaseOff/8

	// Identify the callee closure. The caller's regs[A] holds the function value
	// that was called. We read it from the register file since ctx.BaselineClosurePtr
	// was already restored to the caller's closure by the ARM64 restore sequence.
	callA := int(ctx.NativeCallA)
	absCallA := base + callA
	if absCallA >= len(regs) {
		return runtime.NilValue(), fmt.Errorf("native-call-exit: call slot %d out of range", absCallA)
	}
	fnVal := regs[absCallA]
	if !fnVal.IsFunction() {
		return runtime.NilValue(), fmt.Errorf("native-call-exit: regs[%d] is not a function", absCallA)
	}
	cl, ok := vmClosureFromValue(fnVal)
	if !ok {
		return runtime.NilValue(), fmt.Errorf("native-call-exit: regs[%d] is not a vm.Closure", absCallA)
	}
	calleeProto := cl.Proto

	// Disable BLR for this callee — it has exit-resume ops that make BLR counterproductive.
	// Future calls will see DirectEntryPtr==0 and go straight to slow path.
	setFuncProtoDirectEntry(calleeProto, 0)
	e.clearBaselineCallCachesForProto(calleeProto)

	calleeBF, ok := e.compiled[calleeProto]
	if !ok {
		return runtime.NilValue(), fmt.Errorf("native-call-exit: callee not compiled")
	}

	// Re-read regs (may have been grown).
	if e.callVM != nil {
		regs = e.callVM.Regs()
	}

	// The BLR caller already copied arguments to regs[calleeBase..]. Re-initialize
	// unused registers to nil (same as Execute does), then re-execute the callee
	// from scratch. This is safe because:
	// - The callee's partial execution only modified registers (no external side effects)
	// - Op-exits like NEWTABLE are at the beginning or are idempotent on retry
	for i := calleeBase + calleeProto.NumParams; i < calleeBase+calleeProto.MaxStack; i++ {
		if i < len(regs) {
			regs[i] = runtime.NilValue()
		}
	}

	// Push a VM frame for the callee (needed for GETUPVAL, CloseUpvalues).
	if e.callVM != nil {
		if !e.callVM.PushFrame(cl, calleeBase) {
			return runtime.NilValue(), fmt.Errorf("native-call-exit: stack overflow")
		}
	}

	// Re-execute the callee from scratch via Execute, which has a proper
	// exit-resume loop that handles all op-exits correctly.
	results, err := e.Execute(calleeBF, regs, calleeBase, calleeProto)
	if err == errOSRRequested && e.osrHandler != nil {
		if e.callVM != nil {
			regs = e.callVM.Regs()
		}
		results, err = e.osrHandler(regs, calleeBase, calleeProto)
	}
	if vm.IsCoroutineYield(err) {
		return runtime.NilValue(), err
	}

	// Close upvalues and pop frame regardless of error.
	if e.callVM != nil {
		e.callVM.CloseUpvalues(calleeBase)
		e.callVM.PopFrame()
	}

	if err != nil {
		return runtime.NilValue(), err
	}

	if len(results) > 0 {
		return results[0], nil
	}
	return runtime.NilValue(), nil
}

// ptrToVMClosure converts a uintptr (from JIT-stored BaselineClosurePtr) to *vm.Closure.
// This is a legitimate conversion: the pointer was obtained from a NaN-boxed value
// that is kept alive by the runtime's GC root system.
//
//go:nosplit
func ptrToVMClosure(ptr uintptr) *vm.Closure {
	// Store the uintptr in a local, then convert via unsafe.Pointer.
	// The pointer is valid because it's kept alive by runtime.keepAliveIface.
	p := *(*unsafe.Pointer)(unsafe.Pointer(&ptr))
	return (*vm.Closure)(p)
}
