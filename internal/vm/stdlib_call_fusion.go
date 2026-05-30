package vm

import "github.com/Never-Labs/gscript/internal/runtime"

func (vm *VM) tryFuseStringSubToNumber(frame *CallFrame, base int, a int, nArgs int, rawC int, gf *runtime.GoFunction) (bool, error) {
	if gf == nil ||
		gf.NativeKind != runtime.NativeKindStdStringSub ||
		gf.NativeData != runtime.StdStringSubIdentityPtr() ||
		gf.FastArg2 == nil ||
		gf.FastArg3 == nil ||
		(nArgs != 2 && nArgs != 3) ||
		rawC != 2 {
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
	tonumberA := DecodeA(call)
	if DecodeA(move) != tonumberA+1 {
		return false, nil
	}
	tonumberSlot := base + tonumberA
	argSlot := base + a + 1
	if tonumberSlot < 0 || tonumberSlot >= len(vm.regs) || argSlot < 0 || argSlot+nArgs > len(vm.regs) {
		return false, nil
	}
	if !runtime.IsStdToNumberFunction(vm.regs[tonumberSlot]) {
		return false, nil
	}

	substr, ok, err := stringSubSliceNoAlloc(vm.regs[argSlot], vm.regs[argSlot+1], optionalArg(vm.regs, argSlot+2, nArgs == 3))
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	result, parsed := runtime.ParseNumberString(substr)
	if !parsed {
		result = runtime.NilValue()
	}
	vm.storeSingleCallResult(tonumberSlot, 2, result)
	frame.pc += 2
	runtime.RecordRuntimePathRuntimeSpecializationHit(string(RuntimeSpecializationRouteCallSiteValue), "string_sub_tonumber")
	return true, nil
}

func optionalArg(regs []runtime.Value, slot int, present bool) runtime.Value {
	if !present || slot < 0 || slot >= len(regs) {
		return runtime.NilValue()
	}
	return regs[slot]
}

func stringSubSliceNoAlloc(sv, iv, jv runtime.Value) (string, bool, error) {
	if !sv.IsString() {
		return "", false, nil
	}
	s := sv.Str()
	slen := len(s)
	i := int(valueToStdlibInt(iv))
	j := slen
	if !jv.IsNil() {
		j = int(valueToStdlibInt(jv))
	}
	if i < 0 {
		i = slen + i + 1
	}
	if i < 1 {
		i = 1
	}
	if j < 0 {
		j = slen + j + 1
	}
	if j > slen {
		j = slen
	}
	if i > j {
		return "", true, nil
	}
	return s[i-1 : j], true, nil
}

func valueToStdlibInt(v runtime.Value) int64 {
	switch {
	case v.IsInt():
		return v.Int()
	case v.IsFloat():
		return int64(v.Float())
	case v.IsString():
		n, ok := v.ToNumber()
		if !ok {
			return 0
		}
		return valueToStdlibInt(n)
	default:
		return 0
	}
}
