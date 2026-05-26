//go:build darwin && arm64

// tier1_handlers_coroutine.go contains the Tier 1 baseline JIT exit handlers and
// helpers for coroutine yield/resume operations, including the fast-continuation
// safety analyses used to decide whether a yield can resume natively.
// Pure code movement from tier1_handlers.go; no behavior change.

package methodjit

import (
	"fmt"
	"unsafe"

	"github.com/gscript/gscript/internal/runtime"
	"github.com/gscript/gscript/internal/vm"
)

func (e *BaselineJITEngine) handleYield(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto, bf *BaselineFunc) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for coroutine yield exit")
	}
	slot := int(ctx.BaselineA)
	rawB := int(ctx.BaselineB)
	rawC := int(ctx.BaselineC)
	absSlot := base + slot
	if absSlot >= len(regs) {
		return fmt.Errorf("yield slot %d out of range", absSlot)
	}
	nArgs := rawB - 1
	if rawB == 0 {
		nArgs = e.callVM.Top() - (absSlot + 1)
		if nArgs < 0 {
			nArgs = 0
		}
	}
	if err := e.callVM.SuspendCoroutineFromSlots(absSlot, nArgs, rawC); err != nil {
		return err
	}
	resumePC := int(ctx.BaselinePC)
	resumeOff, ok := baselineResumeOffset(bf, resumePC)
	if !ok {
		return fmt.Errorf("baseline: no yield continuation label for PC %d", resumePC)
	}
	if err := e.callVM.SaveMethodJITContinuation(vm.MethodJITContinuation{
		Compiled: bf,
		Base:     base,
		Proto:    proto,
		PC:       resumePC,
		Offset:   resumeOff,
	}); err != nil {
		return err
	}
	if ctx.CoroutineNativeSwitch != 0 {
		if baselineCoroutineFastContinuationSafe(proto, resumePC) {
			code := uintptr(bf.Code.Ptr()) + uintptr(resumeOff)
			if err := e.callVM.SaveMethodJITFastContinuation(code, uintptr(unsafe.Pointer(ctx)), resumePC); err != nil {
				return err
			}
			ctx.CoroutinePinnedCtx = 1
		}
	}
	return vm.CoroutineYieldError()
}

func baselineCoroutineFastContinuationUsesPooledRecord(proto *vm.FuncProto, resumePC int) bool {
	if proto == nil || resumePC < 0 || resumePC >= len(proto.Code) {
		return false
	}
	startPC := baselineCoroutineFastContinuationStartPC(proto, resumePC)
	for pc := startPC; pc < len(proto.Code); pc++ {
		inst := proto.Code[pc]
		switch vm.DecodeOp(inst) {
		case vm.OP_YIELD:
			return false
		case vm.OP_NEWOBJECTN:
			return baselineNewObjectNCacheable(proto, inst)
		}
	}
	return false
}

func baselineProtoMayUseNativeCoroutineSwitch(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for pc, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_YIELD {
			continue
		}
		resumePC := pc + 1
		if baselineCoroutineFastContinuationSafe(proto, resumePC) {
			return true
		}
	}
	return false
}

func baselineProtoMayUseNativeCoroutineResume(proto *vm.FuncProto) bool {
	if proto == nil {
		return false
	}
	for _, inst := range proto.Code {
		if vm.DecodeOp(inst) != vm.OP_RESUME {
			continue
		}
		if vm.DecodeB(inst) == 2 && vm.DecodeC(inst) == 3 {
			return true
		}
	}
	return false
}

func baselineCoroutineFastContinuationSafe(proto *vm.FuncProto, resumePC int) bool {
	if proto == nil || resumePC < 0 || resumePC >= len(proto.Code) {
		return false
	}
	startPC := baselineCoroutineFastContinuationStartPC(proto, resumePC)
	for pc := startPC; pc < len(proto.Code); pc++ {
		inst := proto.Code[pc]
		switch vm.DecodeOp(inst) {
		case vm.OP_YIELD:
			return vm.DecodeB(inst) == 2
		case vm.OP_LOADNIL, vm.OP_LOADBOOL, vm.OP_LOADINT, vm.OP_LOADK,
			vm.OP_MOVE,
			vm.OP_ADD, vm.OP_SUB, vm.OP_MUL, vm.OP_DIV, vm.OP_MOD,
			vm.OP_UNM, vm.OP_NOT, vm.OP_ISNUMBER,
			vm.OP_EQ, vm.OP_LT, vm.OP_LE, vm.OP_TEST, vm.OP_TESTSET,
			vm.OP_JMP, vm.OP_FORPREP, vm.OP_FORLOOP:
			continue
		case vm.OP_NEWOBJECTN:
			if !baselineNewObjectNCacheable(proto, inst) {
				return false
			}
		default:
			return false
		}
	}
	return false
}

func baselineCoroutineFastContinuationStartPC(proto *vm.FuncProto, resumePC int) int {
	startPC := resumePC
	if proto == nil || resumePC < 0 || resumePC >= len(proto.Code) {
		return startPC
	}
	if inst := proto.Code[resumePC]; vm.DecodeOp(inst) == vm.OP_FORLOOP {
		target := resumePC + 1 + vm.DecodesBx(inst)
		if target >= 0 && target < len(proto.Code) {
			startPC = target
		}
	}
	return startPC
}

func (e *BaselineJITEngine) handleResume(ctx *ExecContext, regs []runtime.Value, base int, proto *vm.FuncProto) error {
	if e.callVM == nil {
		return fmt.Errorf("no callVM for coroutine resume exit")
	}
	slot := int(ctx.BaselineA)
	rawB := int(ctx.BaselineB)
	rawC := int(ctx.BaselineC)
	absSlot := base + slot
	if absSlot >= len(regs) {
		return fmt.Errorf("resume slot %d out of range", absSlot)
	}
	nArgs := rawB - 1
	if rawB == 0 {
		nArgs = e.callVM.Top() - (absSlot + 1)
		if nArgs < 0 {
			nArgs = 0
		}
	}
	payloadFieldOnly := false
	if proto != nil {
		payloadFieldOnly = e.callVM.ResumePayloadIsFieldOnly(proto, int(ctx.BaselinePC), slot, rawC)
	}
	return e.callVM.ResumeCoroutineFromSlots(absSlot, nArgs, rawC, payloadFieldOnly)
}
