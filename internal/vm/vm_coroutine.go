package vm

// Coroutine fast-call and resume-payload helpers, split verbatim from vm.go.

import (
	"fmt"
	"github.com/gscript/gscript/internal/runtime"
	"unsafe"
)

func (vm *VM) tryFastCoroutineCall(gf *runtime.GoFunction, base, a, nArgs, c int) (bool, error) {
	switch gf.NativeKind {
	case goFunctionKindCoroutineWrapper:
		co, ok := vmCoroutineFromNativeData(gf.NativeData)
		if !ok {
			return true, fmt.Errorf("invalid wrapped coroutine")
		}
		if handled, err := vm.tryFastWrappedGeneratorCall(co, base, a, nArgs, c); handled {
			return true, err
		}
		if co.status == VMCoroutineDead {
			return true, fmt.Errorf("cannot resume dead coroutine")
		}
		var args []runtime.Value
		if nArgs > 0 {
			start := base + a + 1
			args = vm.regs[start : start+nArgs]
		}
		okResult, values, err := vm.resumeCoroutineRaw(co, args)
		if err != nil {
			return true, err
		}
		if !okResult {
			if len(values) > 0 {
				return true, fmt.Errorf("%s", values[0].String())
			}
			return true, fmt.Errorf("cannot resume dead coroutine")
		}
		if len(values) == 0 {
			vm.writeSingleCallResult(base+a, c, runtime.NilValue())
			return true, nil
		}
		vm.writeCallResults(base+a, c, values)
		return true, nil

	case goFunctionKindCoroutineCreate:
		if nArgs < 1 || !vm.regs[base+a+1].IsFunction() {
			return true, fmt.Errorf("coroutine.create expects a function")
		}
		cl, ok := closureFromValue(vm.regs[base+a+1])
		if !ok {
			gf := vm.regs[base+a+1].GoFunction()
			if gf == nil {
				return true, fmt.Errorf("coroutine.create expects a GScript function")
			}
			co := NewVMGoCoroutine(gf)
			vm.recordCoroutineCreated(false)
			vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
			return true, nil
		}
		co := NewVMCoroutine(cl)
		vm.recordCoroutineCreated(false)
		vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
		return true, nil

	case goFunctionKindCoroutineResume:
		co, args, err := vm.coroutineResumeBoundaryFromSlots(base+a, nArgs)
		if err != nil {
			return true, err
		}
		okResult, values, err := vm.resumeCoroutineRaw(co, args)
		if err != nil {
			return true, err
		}
		vm.finishCoroutineResumeToSlots(base+a, c, okResult, values)
		return true, nil

	case goFunctionKindCoroutineYield:
		results, err, suspended := vm.handleCoroutineYieldFromSlots(base+a, nArgs, c)
		if err != nil {
			return true, err
		}
		if suspended {
			return true, nil
		}
		vm.writeCallResults(base+a, c, results)
		return true, nil

	case goFunctionKindCoroutineIsYield:
		vm.writeSingleCallResult(base+a, c, runtime.BoolValue(vm.activeCoroutine() != nil))
		return true, nil
	}

	if gf == vm.coroutineCreateFn || gf.Name == coroutineCreateName {
		if nArgs < 1 || !vm.regs[base+a+1].IsFunction() {
			return true, fmt.Errorf("coroutine.create expects a function")
		}
		cl, ok := closureFromValue(vm.regs[base+a+1])
		if !ok {
			gf := vm.regs[base+a+1].GoFunction()
			if gf == nil {
				return true, fmt.Errorf("coroutine.create expects a GScript function")
			}
			co := NewVMGoCoroutine(gf)
			vm.recordCoroutineCreated(false)
			vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
			return true, nil
		}
		co := NewVMCoroutine(cl)
		vm.recordCoroutineCreated(false)
		vm.writeSingleCallResult(base+a, c, runtime.VMCoroutineValue(unsafe.Pointer(co), co))
		return true, nil
	}

	if gf == vm.coroutineResumeFn || gf.Name == coroutineResumeName {
		co, args, err := vm.coroutineResumeBoundaryFromSlots(base+a, nArgs)
		if err != nil {
			return true, err
		}
		okResult, values, err := vm.resumeCoroutineRaw(co, args)
		if err != nil {
			return true, err
		}
		vm.finishCoroutineResumeToSlots(base+a, c, okResult, values)
		return true, nil
	}

	if gf == vm.coroutineYieldFn || gf.Name == coroutineYieldName {
		results, err, suspended := vm.handleCoroutineYieldFromSlots(base+a, nArgs, c)
		if err != nil {
			return true, err
		}
		if suspended {
			return true, nil
		}
		vm.writeCallResults(base+a, c, results)
		return true, nil
	}

	if gf.Name == coroutineIsYieldableName {
		vm.writeSingleCallResult(base+a, c, runtime.BoolValue(vm.activeCoroutine() != nil))
		return true, nil
	}

	return false, nil
}

func (vm *VM) writeSingleCallResult(dst, c int, result runtime.Value) {
	if c == 0 {
		vm.regs[dst] = result
		vm.top = dst + 1
		return
	}
	if c == 1 {
		return
	}
	vm.regs[dst] = result
	for i := 1; i < c-1; i++ {
		vm.regs[dst+i] = runtime.NilValue()
	}
}

func (vm *VM) writeCoroutineResumeResults(dst, c int, ok bool, values []runtime.Value) {
	if c == 0 {
		vm.regs[dst] = runtime.BoolValue(ok)
		for i, r := range values {
			vm.regs[dst+1+i] = r
		}
		vm.top = dst + 1 + len(values)
		return
	}
	if c == 3 && len(values) == 1 {
		vm.regs[dst] = runtime.BoolValue(ok)
		vm.regs[dst+1] = values[0]
		return
	}
	if c == 2 && len(values) == 0 {
		vm.regs[dst] = runtime.BoolValue(ok)
		return
	}
	nr := c - 1
	for i := 0; i < nr; i++ {
		switch {
		case i == 0:
			vm.regs[dst] = runtime.BoolValue(ok)
		case i-1 < len(values):
			vm.regs[dst+i] = values[i-1]
		default:
			vm.regs[dst+i] = runtime.NilValue()
		}
	}
}

func (vm *VM) ResumePayloadIsFieldOnly(proto *FuncProto, nextPC, resumeA, c int) bool {
	if proto == nil || c != 3 {
		return false
	}
	if nextPC >= 0 && nextPC < len(proto.Code) {
		if proto.ResumePayloadCache == nil {
			proto.ResumePayloadCache = make([]int8, len(proto.Code))
		}
		switch proto.ResumePayloadCache[nextPC] {
		case 1:
			return false
		case 2:
			return true
		}
		result := vm.resumePayloadIsFieldOnlyUncached(proto, nextPC, resumeA, c)
		if result {
			proto.ResumePayloadCache[nextPC] = 2
		} else {
			proto.ResumePayloadCache[nextPC] = 1
		}
		return result
	}
	return vm.resumePayloadIsFieldOnlyUncached(proto, nextPC, resumeA, c)
}

func (vm *VM) resumePayloadIsFieldOnlyUncached(proto *FuncProto, nextPC, resumeA, c int) bool {
	payloadReg := resumeA + 1
	for pc := nextPC; pc < len(proto.Code); pc++ {
		inst := proto.Code[pc]
		op := DecodeOp(inst)
		a := DecodeA(inst)
		b := DecodeB(inst)
		cc := DecodeC(inst)

		switch op {
		case OP_GETFIELD:
			if b == payloadReg {
				continue
			}
			if a == payloadReg {
				return true
			}
		case OP_RESUME, OP_FORLOOP:
			return true
		case OP_RETURN:
			return !registerRangeMayRead(payloadReg, a, b)
		case OP_JMP:
			if DecodesBx(inst) < 0 {
				return true
			}
		case OP_MOVE, OP_UNM, OP_BNOT, OP_NOT, OP_ISNUMBER, OP_LEN:
			if b == payloadReg {
				return false
			}
			if a == payloadReg {
				return true
			}
		case OP_GETTABLE, OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW, OP_BAND, OP_BOR, OP_BXOR, OP_BANDN, OP_SHL, OP_SHR, OP_EQ, OP_LT, OP_LE:
			if b == payloadReg || (cc < RKBit && cc == payloadReg) {
				return false
			}
			if op != OP_EQ && op != OP_LT && op != OP_LE && a == payloadReg {
				return true
			}
		case OP_SETTABLE:
			if a == payloadReg || b == payloadReg || (cc < RKBit && cc == payloadReg) {
				return false
			}
		case OP_SETFIELD:
			if a == payloadReg || cc == payloadReg {
				return false
			}
		case OP_TEST:
			if a == payloadReg {
				return false
			}
		case OP_TESTSET:
			if b == payloadReg {
				return false
			}
			if a == payloadReg {
				return true
			}
		case OP_CALL, OP_YIELD, OP_GO:
			if callRegisterRangeMayRead(payloadReg, a, b) {
				return false
			}
			if callRegisterRangeMayWrite(payloadReg, a, cc) {
				return true
			}
		case OP_TFORCALL:
			if payloadReg >= a && payloadReg <= a+2 {
				return false
			}
			if payloadReg >= a+3 && payloadReg <= a+2+cc {
				return true
			}
		case OP_TFORLOOP:
			if payloadReg == a || payloadReg == a+1 {
				return false
			}
		case OP_SETGLOBAL, OP_SETGLOBALRO, OP_SETUPVAL, OP_CLOSE, OP_APPEND, OP_SEND, OP_DEFER, OP_CHECKCONST, OP_TRYSEND:
			if a == payloadReg || b == payloadReg {
				return false
			}
			if op == OP_TRYSEND && cc == payloadReg {
				return true
			}
		case OP_SELF:
			if b == payloadReg || (cc < RKBit && cc == payloadReg) {
				return false
			}
			if a == payloadReg || a+1 == payloadReg {
				return true
			}
		case OP_CONCAT:
			if payloadReg >= b && payloadReg <= cc {
				return false
			}
			if a == payloadReg {
				return true
			}
		case OP_SETLIST:
			if a == payloadReg || (payloadReg > a && payloadReg <= a+b) {
				return false
			}
		case OP_FORPREP:
			if payloadReg >= a && payloadReg <= a+3 {
				return false
			}
		case OP_RECV:
			if a == payloadReg {
				return true
			}
			if b == payloadReg {
				return false
			}
		case OP_TRYRECV:
			if b == payloadReg {
				return false
			}
			if a == payloadReg || cc == payloadReg {
				return true
			}
		case OP_SELECT:
			if payloadReg >= b && payloadReg < b+cc*3 {
				return false
			}
			if a == payloadReg || a+1 == payloadReg {
				return true
			}
		default:
			if a == payloadReg {
				return true
			}
		}
	}
	return true
}
