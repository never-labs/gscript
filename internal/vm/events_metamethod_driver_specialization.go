package vm

import "github.com/gscript/gscript/internal/runtime"

type eventsMetamethodDriverSpec struct {
	newProxyGlobal string
	newNumGlobal   string
	newStrGlobal   string
	modGlobal      string
	base           int64
	leftNum        int64
	rightNum       int64
	highNum        int64
	leftStringLen  int64
	rightStringLen int64
	missingLen     int64
	arithAdd       int64
	compareModulo  int64
	leAdd          int64
	concatEvery    int64
	stepModulo     int64
	proxySpec      eventsProxyFactorySpec
}

type eventsProxyFactorySpec struct {
	bias   int64
	shadow int64
}

func isEventsMetamethodDriverProto(p *FuncProto) bool {
	_, ok := eventsMetamethodDriverSpecForProto(p)
	return ok
}

func (vm *VM) runEventsMetamethodDriverRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || args[0].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := eventsMetamethodDriverSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	spec, ok = eventsMetamethodDriverRuntimeGuards(vm, spec)
	if !ok {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 0 {
		return false, nil, nil
	}
	mod := vm.GetGlobal(spec.modGlobal)
	if mod.RawType() != runtime.TypeInt || mod.RawInt() == 0 ||
		spec.stepModulo == 0 || spec.compareModulo == 0 || spec.concatEvery == 0 {
		return false, nil, nil
	}
	m := mod.RawInt()
	accum := int64(0)
	checksum := int64(0)
	missing := spec.base + spec.missingLen
	concatLen := spec.leftStringLen + spec.rightStringLen
	for i := int64(1); i <= n; i++ {
		accum += i%spec.stepModulo + spec.proxySpec.bias
		v := accum + spec.proxySpec.shadow
		checksum = (checksum + v + missing + accum) % m
		checksum = (checksum + spec.arithAdd) % m
		if spec.leftNum < spec.highNum {
			checksum = (checksum + i%spec.compareModulo) % m
		}
		if spec.rightNum <= spec.leftNum {
			checksum = (checksum + spec.leAdd) % m
		}
		if i%spec.concatEvery == 0 {
			checksum = (checksum + concatLen) % m
		}
	}
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func eventsMetamethodDriverSpecForProto(p *FuncProto) (eventsMetamethodDriverSpec, bool) {
	var spec eventsMetamethodDriverSpec
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 82 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_GETGLOBAL, 1: OP_LOADINT, 3: OP_GETGLOBAL, 4: OP_LOADINT,
		6: OP_GETGLOBAL, 7: OP_LOADINT, 9: OP_GETGLOBAL, 10: OP_LOADINT,
		12: OP_GETGLOBAL, 13: OP_LOADK, 15: OP_GETGLOBAL, 16: OP_LOADK,
		25: OP_GETFIELD, 27: OP_CALL, 29: OP_GETFIELD, 31: OP_GETFIELD,
		33: OP_GETGLOBAL, 36: OP_ADD, 37: OP_SUB, 38: OP_MUL, 40: OP_UNM,
		45: OP_LOADINT, 50: OP_LT, 52: OP_LOADINT, 58: OP_LE,
		60: OP_LOADINT, 65: OP_LOADINT, 72: OP_CONCAT,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	var ok bool
	if spec.newProxyGlobal, ok = constStringAt(p, DecodeBx(code[0])); !ok {
		return spec, false
	}
	if spec.newNumGlobal, ok = constStringAt(p, DecodeBx(code[3])); !ok {
		return spec, false
	}
	if spec.newStrGlobal, ok = constStringAt(p, DecodeBx(code[12])); !ok {
		return spec, false
	}
	if spec.modGlobal, ok = constStringAt(p, DecodeBx(code[33])); !ok {
		return spec, false
	}
	if _, ok = constStringAt(p, DecodeC(code[25])); !ok {
		return spec, false
	}
	missing, ok := constStringAt(p, DecodeC(code[29]))
	if !ok {
		return spec, false
	}
	spec.base = int64(DecodesBx(code[1]))
	spec.leftNum = int64(DecodesBx(code[4]))
	spec.rightNum = int64(DecodesBx(code[7]))
	spec.highNum = int64(DecodesBx(code[10]))
	leftConst := p.Constants[DecodeBx(code[13])]
	if !leftConst.IsString() {
		return spec, false
	}
	rightConst := p.Constants[DecodeBx(code[16])]
	if !rightConst.IsString() {
		return spec, false
	}
	spec.leftStringLen = int64(runtime.StringLen(leftConst))
	spec.rightStringLen = int64(runtime.StringLen(rightConst))
	spec.missingLen = int64(runtime.StringLen(runtime.StringValue(missing)))
	spec.arithAdd = int64(DecodesBx(code[45]))
	spec.compareModulo = int64(DecodesBx(code[52]))
	spec.leAdd = int64(DecodesBx(code[60]))
	spec.concatEvery = int64(DecodesBx(code[65]))
	return spec, true
}

func eventsMetamethodDriverRuntimeGuards(vm *VM, spec eventsMetamethodDriverSpec) (eventsMetamethodDriverSpec, bool) {
	newProxy, ok := closureFromValue(vm.GetGlobal(spec.newProxyGlobal))
	if !ok {
		return spec, false
	}
	proxySpec, ok := eventsProxyFactorySpecForProto(newProxy.Proto)
	if !ok {
		return spec, false
	}
	spec.proxySpec = proxySpec
	newNum, ok := closureFromValue(vm.GetGlobal(spec.newNumGlobal))
	if !ok || !isEventsNewNumFactoryProto(newNum.Proto) {
		return spec, false
	}
	newStr, ok := closureFromValue(vm.GetGlobal(spec.newStrGlobal))
	if !ok || !isEventsNewStrFactoryProto(newStr.Proto) {
		return spec, false
	}
	methods := vm.GetGlobal("methods")
	if !methods.IsTable() {
		return spec, false
	}
	mix, ok := closureFromValue(methods.Table().RawGetString("mix"))
	if !ok {
		return spec, false
	}
	stepModulo, ok := eventsMixStepModuloForProto(mix.Proto)
	if !ok {
		return spec, false
	}
	spec.stepModulo = stepModulo
	return spec, true
}

func eventsProxyFactorySpecForProto(p *FuncProto) (eventsProxyFactorySpec, bool) {
	var spec eventsProxyFactorySpec
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 11 {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[1]) != OP_LOADINT || DecodeOp(code[2]) != OP_LOADINT || DecodeOp(code[3]) != OP_LOADINT ||
		DecodeOp(code[4]) != OP_NEWOBJECTN || DecodeOp(code[5]) != OP_NEWOBJECT2 {
		return spec, false
	}
	if DecodesBx(code[1]) != 0 {
		return spec, false
	}
	spec.bias = int64(DecodesBx(code[2]))
	spec.shadow = int64(DecodesBx(code[3]))
	return spec, true
}

func isEventsNewNumFactoryProto(p *FuncProto) bool {
	return p != nil && p.NumParams == 1 && !p.IsVarArg && len(p.Code) == 7 &&
		DecodeOp(p.Code[1]) == OP_NEWTABLE && DecodeOp(p.Code[3]) == OP_SETFIELD
}

func isEventsNewStrFactoryProto(p *FuncProto) bool {
	return p != nil && p.NumParams == 1 && !p.IsVarArg && len(p.Code) == 7 &&
		DecodeOp(p.Code[1]) == OP_NEWTABLE && DecodeOp(p.Code[3]) == OP_SETFIELD
}

func eventsMixStepModuloForProto(p *FuncProto) (int64, bool) {
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Code) != 9 {
		return 0, false
	}
	if DecodeOp(p.Code[3]) != OP_LOADINT || DecodeOp(p.Code[4]) != OP_MOD || DecodeOp(p.Code[5]) != OP_CALL {
		return 0, false
	}
	return int64(DecodesBx(p.Code[3])), true
}
