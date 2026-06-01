package vm

import "github.com/never-labs/leia/internal/runtime"

func (vm *VM) tryExecuteNumericToIntegerWrapperCall(cl *Closure, absSlot, nArgs, rawC int) (bool, error) {
	if cl == nil || cl.Proto == nil || nArgs != 1 || len(cl.Upvalues) != 0 {
		return false, nil
	}
	proto := cl.Proto
	if !isNumericToIntegerWrapperProto(proto) || !vm.standardNumericToIntegerWrapperGlobalsActive() {
		return false, nil
	}
	arg := runtime.NilValue()
	if absSlot+1 >= 0 && absSlot+1 < len(vm.regs) {
		arg = vm.regs[absSlot+1]
	}
	n, ok := arg.ToNumber()
	result := runtime.BoolValue(false)
	if ok {
		if i := runtime.ToIntegerValue(n); !i.IsNil() {
			result = i
		}
	}
	vm.storeSingleCallResult(absSlot, rawC, result)
	runtime.RecordRuntimePathRuntimeSpecializationHit(string(RuntimeSpecializationRouteCallSiteValue), "numeric_to_integer_wrapper")
	return true, nil
}

func (vm *VM) tryFuseToStringNumericToIntegerWrapper(frame *CallFrame, base int, a int, nArgs int, rawC int, gf *runtime.GoFunction) (bool, error) {
	if gf == nil || gf.Name != "tostring" || gf.FastArg1 == nil || nArgs != 1 || rawC != 2 {
		return false, nil
	}
	proto := frame.closure.Proto
	pc := frame.pc - 1
	if proto == nil || pc+2 >= len(proto.Code) {
		return false, nil
	}
	move := proto.Code[pc+1]
	call := proto.Code[pc+2]
	if DecodeOp(move) != OP_MOVE || DecodeB(move) != a || DecodeOp(call) != OP_CALL || DecodeB(call) != 2 || DecodeC(call) != 2 {
		return false, nil
	}
	wrapperA := DecodeA(call)
	if DecodeA(move) != wrapperA+1 {
		return false, nil
	}
	wrapperSlot := base + wrapperA
	if wrapperSlot < 0 || wrapperSlot >= len(vm.regs) {
		return false, nil
	}
	cl, ok := closureFromValue(vm.regs[wrapperSlot])
	if !ok || cl == nil || !isNumericToIntegerWrapperProto(cl.Proto) || !vm.standardNumericToIntegerWrapperGlobalsActive() {
		return false, nil
	}
	argSlot := base + a + 1
	if argSlot < 0 || argSlot >= len(vm.regs) {
		return false, nil
	}
	result, ok := numericToIntegerAfterToString(vm.regs[argSlot])
	if !ok {
		return false, nil
	}
	vm.storeSingleCallResult(wrapperSlot, 2, result)
	frame.pc += 2
	runtime.RecordRuntimePathRuntimeSpecializationHit(string(RuntimeSpecializationRouteCallSiteValue), "tostring_numeric_to_integer_wrapper")
	return true, nil
}

func numericToIntegerAfterToString(v runtime.Value) (runtime.Value, bool) {
	switch {
	case v.IsInt():
		return v, true
	case v.IsFloat():
		i := runtime.ToIntegerValue(v)
		if i.IsNil() {
			return runtime.BoolValue(false), true
		}
		return i, true
	case v.IsString():
		n, ok := v.ToNumber()
		if !ok {
			return runtime.BoolValue(false), true
		}
		i := runtime.ToIntegerValue(n)
		if i.IsNil() {
			return runtime.BoolValue(false), true
		}
		return i, true
	case v.IsNil() || v.IsBool():
		return runtime.BoolValue(false), true
	default:
		return runtime.NilValue(), false
	}
}

func isNumericToIntegerWrapperProto(proto *FuncProto) bool {
	if proto == nil {
		return false
	}
	switch proto.RuntimeSpecs.NumericToIntegerWrapperShape {
	case 1:
		return true
	case -1:
		return false
	}
	if matchNumericToIntegerWrapperProto(proto) {
		proto.RuntimeSpecs.NumericToIntegerWrapperShape = 1
		return true
	}
	proto.RuntimeSpecs.NumericToIntegerWrapperShape = -1
	return false
}

func matchNumericToIntegerWrapperProto(proto *FuncProto) bool {
	if proto.NumParams != 1 || proto.IsVarArg || len(proto.Upvalues) != 0 || len(proto.Protos) != 0 || len(proto.Code) != 30 || len(proto.Constants) < 3 {
		return false
	}
	if !constString(proto, 0, "tonumber") || !constString(proto, 1, "math") || !constString(proto, 2, "tointeger") {
		return false
	}
	expected := []uint32{
		EncodeABx(OP_GETGLOBAL, 1, 0),
		EncodeABC(OP_MOVE, 2, 0, 0),
		EncodeABC(OP_CALL, 1, 2, 2),
		EncodeABC(OP_LOADNIL, 3, 0, 0),
		EncodeABC(OP_EQ, 0, 1, 3),
		EncodeAsBx(OP_JMP, 0, 1),
		EncodeABC(OP_LOADBOOL, 2, 1, 1),
		EncodeABC(OP_LOADBOOL, 2, 0, 0),
		EncodeABC(OP_TESTSET, 2, 2, 1),
		EncodeAsBx(OP_JMP, 0, 5),
		EncodeABC(OP_LOADBOOL, 3, 0, 0),
		EncodeABC(OP_EQ, 0, 1, 3),
		EncodeAsBx(OP_JMP, 0, 1),
		EncodeABC(OP_LOADBOOL, 2, 1, 1),
		EncodeABC(OP_LOADBOOL, 2, 0, 0),
		EncodeABC(OP_TEST, 2, 0, 0),
		EncodeAsBx(OP_JMP, 0, 2),
		EncodeABC(OP_LOADBOOL, 2, 0, 0),
		EncodeABC(OP_RETURN, 2, 2, 0),
		EncodeABx(OP_GETGLOBAL, 3, 1),
		EncodeABC(OP_GETFIELD, 2, 3, 2),
		EncodeABC(OP_MOVE, 3, 1, 0),
		EncodeABC(OP_CALL, 2, 2, 2),
		EncodeABC(OP_LOADNIL, 3, 0, 0),
		EncodeABC(OP_EQ, 0, 2, 3),
		EncodeAsBx(OP_JMP, 0, 2),
		EncodeABC(OP_LOADBOOL, 3, 0, 0),
		EncodeABC(OP_RETURN, 3, 2, 0),
		EncodeABC(OP_MOVE, 3, 2, 0),
		EncodeABC(OP_RETURN, 3, 2, 0),
	}
	for i, want := range expected {
		if proto.Code[i] != want {
			return false
		}
	}
	return true
}

func constString(proto *FuncProto, index int, want string) bool {
	return index >= 0 && index < len(proto.Constants) && proto.Constants[index].IsString() && proto.Constants[index].Str() == want
}

func (vm *VM) standardNumericToIntegerWrapperGlobalsActive() bool {
	tonumber := vm.GetGlobal("tonumber")
	if !runtime.IsStdToNumberFunction(tonumber) {
		return false
	}
	mathValue := vm.GetGlobal("math")
	if !mathValue.IsTable() {
		return false
	}
	tointeger := mathValue.Table().RawGetString("tointeger")
	gf := tointeger.GoFunction()
	return gf != nil && gf.Name == "math.tointeger" && gf.FastArg1 != nil
}

func (vm *VM) storeSingleCallResult(absSlot, rawC int, result runtime.Value) {
	if rawC == 0 {
		if absSlot >= 0 && absSlot < len(vm.regs) {
			vm.regs[absSlot] = result
		}
		vm.top = absSlot + 1
		return
	}
	nr := rawC - 1
	for i := 0; i < nr; i++ {
		idx := absSlot + i
		if idx < 0 || idx >= len(vm.regs) {
			continue
		}
		if i == 0 {
			vm.regs[idx] = result
		} else {
			vm.regs[idx] = runtime.NilValue()
		}
	}
}
