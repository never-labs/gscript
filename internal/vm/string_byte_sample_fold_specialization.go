package vm

import "github.com/gscript/gscript/internal/runtime"

type stringByteSampleFoldSpec struct {
	mixGlobal string
	divisor   int64
	stepBias  int64
}

type binaryIntModuloFoldSpec struct {
	mul       int64
	modGlobal string
}

func isStringByteSampleFoldProto(p *FuncProto) bool {
	_, ok := stringByteSampleFoldSpecForProto(p)
	return ok
}

func stringByteSampleFoldSpecForProto(p *FuncProto) (stringByteSampleFoldSpec, bool) {
	var spec stringByteSampleFoldSpec
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 49 || len(p.Constants) != 5 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_GETGLOBAL, 3: OP_LEN, 4: OP_CALL, 6: OP_GETGLOBAL, 7: OP_GETFIELD,
		10: OP_LOADINT, 11: OP_DIV, 12: OP_CALL, 13: OP_LOADINT, 14: OP_ADD,
		19: OP_FORPREP, 20: OP_GETGLOBAL, 23: OP_GETFIELD, 26: OP_CALL, 28: OP_CALL,
		30: OP_FORLOOP, 34: OP_LT, 36: OP_GETGLOBAL, 39: OP_GETFIELD, 42: OP_LEN,
		43: OP_CALL, 45: OP_CALL, 48: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	var ok bool
	if spec.mixGlobal, ok = constStringAt(p, 0); !ok {
		return spec, false
	}
	spec.divisor = int64(DecodesBx(code[10]))
	spec.stepBias = int64(DecodesBx(code[13]))
	if spec.divisor <= 0 || spec.stepBias <= 0 {
		return stringByteSampleFoldSpec{}, false
	}
	return spec, true
}

func (vm *VM) runStringByteSampleFoldRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || !args[1].IsString() {
		return false, nil, nil
	}
	spec, ok := stringByteSampleFoldSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	fold, ok := vm.stringByteSampleFoldRuntimeGuards(spec)
	if !ok {
		return false, nil, nil
	}
	modValue := vm.GetGlobal(fold.modGlobal)
	if modValue.RawType() != runtime.TypeInt || modValue.RawInt() == 0 {
		return false, nil, nil
	}
	mod := modValue.RawInt()
	s := args[1].Str()
	h := binaryIntModuloFold(args[0].RawInt(), int64(len(s)), fold.mul, mod)
	step := int64(len(s))/spec.divisor + spec.stepBias
	for i := int64(1); i <= int64(len(s)); i += step {
		h = binaryIntModuloFold(h, int64(s[i-1]), fold.mul, mod)
	}
	if len(s) > 0 {
		h = binaryIntModuloFold(h, int64(s[len(s)-1]), fold.mul, mod)
	}
	return true, []runtime.Value{runtime.IntValue(h)}, nil
}

func (vm *VM) stringByteSampleFoldRuntimeGuards(spec stringByteSampleFoldSpec) (binaryIntModuloFoldSpec, bool) {
	mix, ok := closureFromValue(vm.GetGlobal(spec.mixGlobal))
	if !ok {
		return binaryIntModuloFoldSpec{}, false
	}
	fold, ok := binaryIntModuloFoldSpecForProto(mix.Proto)
	if !ok ||
		!stdlibHostTableFunction(vm.GetGlobal("math"), "floor", "math.floor") ||
		!stdlibHostTableFunction(vm.GetGlobal("string"), "byte", "string.byte") {
		return binaryIntModuloFoldSpec{}, false
	}
	return fold, true
}

func binaryIntModuloFoldSpecForProto(p *FuncProto) (binaryIntModuloFoldSpec, bool) {
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 6 {
		return binaryIntModuloFoldSpec{}, false
	}
	pat := newBytecodePattern(p.Code)
	if !pat.hasOps(
		opcodeAt{pc: 0, op: OP_LOADINT},
		opcodeAt{pc: 1, op: OP_MUL},
		opcodeAt{pc: 2, op: OP_ADD},
		opcodeAt{pc: 3, op: OP_GETGLOBAL},
		opcodeAt{pc: 4, op: OP_MOD},
		opcodeAt{pc: 5, op: OP_RETURN},
	) {
		return binaryIntModuloFoldSpec{}, false
	}
	modGlobal, ok := constStringAt(p, DecodeBx(p.Code[3]))
	if !ok {
		return binaryIntModuloFoldSpec{}, false
	}
	mul := int64(DecodesBx(p.Code[0]))
	if mul == 0 || modGlobal == "" {
		return binaryIntModuloFoldSpec{}, false
	}
	return binaryIntModuloFoldSpec{mul: mul, modGlobal: modGlobal}, true
}

func binaryIntModuloFold(h, v, mul, mod int64) int64 {
	return positiveModInt64(h*mul+v, mod)
}
