package vm

import "github.com/gscript/gscript/internal/runtime"

type callableLenPairsDriverSpec struct {
	callableField      string
	proxyField         string
	makeCallableGlobal string
	makeProxyGlobal    string
	modGlobal          string
	pairEveryGlobal    string
	pairProxyLen       int64
	argModulo          int64
	lenScale           int64
	pairAdd            int64
	biasScale          int64
	biasOffset         int64
	callSpec           tableAffineUpdateModuloLeafSpec
}

func isCallableLenPairsDriverProto(p *FuncProto) bool {
	_, ok := callableLenPairsDriverSpecForProto(p)
	return ok
}

func (vm *VM) runCallableLenPairsDriverRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := callableLenPairsDriverSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	spec, ok = callableLenPairsDriverRuntimeGuards(vm, spec)
	if !ok {
		return false, nil, nil
	}
	groups := args[0].RawInt()
	reps := args[1].RawInt()
	if groups < 0 || reps < 0 {
		return false, nil, nil
	}
	mod := vm.GetGlobal(spec.modGlobal)
	pairEvery := vm.GetGlobal(spec.pairEveryGlobal)
	if mod.RawType() != runtime.TypeInt || pairEvery.RawType() != runtime.TypeInt || mod.RawInt() == 0 || pairEvery.RawInt() == 0 {
		return false, nil, nil
	}
	checksum := int64(0)
	m := mod.RawInt()
	pe := pairEvery.RawInt()
	for b := int64(1); b <= groups; b++ {
		bias := b*spec.biasScale + spec.biasOffset
		count := int64(0)
		for i := int64(1); i <= reps; i++ {
			count++
			callValue := ((i+b)*spec.callSpec.xScale + (i%spec.argModulo)*spec.callSpec.yScale + bias + count*spec.callSpec.countScale) % m
			checksum = (checksum + callValue + spec.pairProxyLen*spec.lenScale) % m
			if i%pe == 0 {
				checksum = (checksum + spec.pairAdd) % m
			}
		}
	}
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}

func callableLenPairsDriverSpecForProto(p *FuncProto) (callableLenPairsDriverSpec, bool) {
	var spec callableLenPairsDriverSpec
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Code) != 67 {
		return spec, false
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasOps(
		opcodeAt{pc: 5, op: OP_GETGLOBAL},
		opcodeAt{pc: 8, op: OP_GETGLOBAL},
		opcodeAt{pc: 10, op: OP_LOADINT},
		opcodeAt{pc: 27, op: OP_GETFIELD},
		opcodeAt{pc: 29, op: OP_LOADINT},
		opcodeAt{pc: 33, op: OP_GETFIELD},
		opcodeAt{pc: 34, op: OP_LEN},
		opcodeAt{pc: 35, op: OP_LOADINT},
		opcodeAt{pc: 38, op: OP_GETGLOBAL},
		opcodeAt{pc: 41, op: OP_GETGLOBAL},
		opcodeAt{pc: 46, op: OP_GETGLOBAL},
		opcodeAt{pc: 49, op: OP_GETFIELD},
		opcodeAt{pc: 58, op: OP_LOADINT},
	) || DecodeB(code[27]) != 11 || DecodeB(code[33]) != 11 {
		return spec, false
	}
	var ok bool
	if spec.callableField, ok = constStringAt(p, DecodeC(code[27])); !ok {
		return spec, false
	}
	if spec.proxyField, ok = constStringAt(p, DecodeC(code[33])); !ok {
		return spec, false
	}
	if spec.makeCallableGlobal, ok = constStringAt(p, DecodeBx(code[5])); !ok {
		return spec, false
	}
	if spec.makeProxyGlobal, ok = constStringAt(p, DecodeBx(code[8])); !ok {
		return spec, false
	}
	if spec.modGlobal, ok = constStringAt(p, DecodeBx(code[38])); !ok {
		return spec, false
	}
	if spec.pairEveryGlobal, ok = constStringAt(p, DecodeBx(code[41])); !ok {
		return spec, false
	}
	spec.pairProxyLen = int64(DecodesBx(code[10]))
	spec.argModulo = int64(DecodesBx(code[29]))
	spec.lenScale = int64(DecodesBx(code[35]))
	spec.pairAdd = int64(DecodesBx(code[58]))
	if spec.pairProxyLen < 0 || spec.argModulo == 0 || spec.lenScale < 0 || spec.pairAdd < 0 {
		return spec, false
	}
	return spec, true
}

func callableLenPairsDriverRuntimeGuards(vm *VM, spec callableLenPairsDriverSpec) (callableLenPairsDriverSpec, bool) {
	makeCallable, ok := closureFromValue(vm.GetGlobal(spec.makeCallableGlobal))
	if !ok {
		return spec, false
	}
	callSpec, biasScale, biasOffset, ok := callableLenPairsMakeCallableSpec(makeCallable.Proto)
	if !ok {
		return spec, false
	}
	spec.callSpec = callSpec
	spec.biasScale = biasScale
	spec.biasOffset = biasOffset
	makePairProxy, ok := closureFromValue(vm.GetGlobal(spec.makeProxyGlobal))
	if !ok || !isCallableLenPairsMakePairProxyProto(makePairProxy.Proto) {
		return spec, false
	}
	return spec, true
}

func callableLenPairsMakeCallableSpec(p *FuncProto) (tableAffineUpdateModuloLeafSpec, int64, int64, bool) {
	var callSpec tableAffineUpdateModuloLeafSpec
	if p == nil || p.NumParams != 1 || p.IsVarArg || len(p.Code) != 13 || len(p.Protos) != 1 {
		return callSpec, 0, 0, false
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasOps(
		opcodeAt{pc: 4, op: OP_LOADINT},
		opcodeAt{pc: 5, op: OP_MUL},
		opcodeAt{pc: 6, op: OP_LOADINT},
		opcodeAt{pc: 7, op: OP_ADD},
	) {
		return callSpec, 0, 0, false
	}
	callSpec, ok := tableAffineUpdateModuloLeafSpecForProto(p.Protos[0])
	if !ok {
		return callSpec, 0, 0, false
	}
	return callSpec, int64(DecodesBx(code[4])), int64(DecodesBx(code[6])), true
}

func isCallableLenPairsMakePairProxyProto(p *FuncProto) bool {
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Code) != 25 || len(p.Protos) != 2 {
		return false
	}
	return isReturnSingleUpvalueProto(p.Protos[0]) &&
		isCallableLenPairsPairsFactoryProto(p.Protos[1])
}

func isReturnSingleUpvalueProto(p *FuncProto) bool {
	return p != nil && p.NumParams == 1 && !p.IsVarArg && len(p.Code) == 2 &&
		DecodeOp(p.Code[0]) == OP_GETUPVAL && DecodeA(p.Code[0]) == 1 &&
		DecodeOp(p.Code[1]) == OP_RETURN && DecodeA(p.Code[1]) == 1 && DecodeB(p.Code[1]) == 2
}

func isCallableLenPairsPairsFactoryProto(p *FuncProto) bool {
	return p != nil && p.NumParams == 1 && !p.IsVarArg && len(p.Code) == 7 && len(p.Protos) == 1 &&
		DecodeOp(p.Code[0]) == OP_LOADINT && DecodeA(p.Code[0]) == 1 && DecodesBx(p.Code[0]) == 0 &&
		DecodeOp(p.Code[1]) == OP_CLOSURE &&
		DecodeOp(p.Code[4]) == OP_RETURN && DecodeA(p.Code[4]) == 2 && DecodeB(p.Code[4]) == 4
}

func constStringAt(p *FuncProto, idx int) (string, bool) {
	if idx >= 0 && idx < len(p.Constants) && p.Constants[idx].IsString() {
		return p.Constants[idx].Str(), true
	}
	return "", false
}
