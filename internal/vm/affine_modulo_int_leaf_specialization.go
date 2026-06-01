package vm

import "github.com/never-labs/leia/internal/runtime"

type affineModuloIntLeafSpec struct {
	mulConst  int64
	modConst  runtime.Value
	modGlobal string
}

func isAffineModuloIntLeafProto(p *FuncProto) bool {
	_, ok := affineModuloIntLeafSpecForProto(p)
	return ok
}

func affineModuloIntLeafSpecForProto(p *FuncProto) (affineModuloIntLeafSpec, bool) {
	var spec affineModuloIntLeafSpec
	if p == nil || p.NumParams != 2 || p.IsVarArg || len(p.Code) != 6 {
		return spec, false
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasAs(aAt{pc: 0, op: OP_LOADINT, a: 5}) ||
		!pat.hasABCs(
			abcAt{pc: 1, op: OP_MUL, a: 4, b: 0, c: 5},
			abcAt{pc: 2, op: OP_ADD, a: 3, b: 4, c: 1},
			abcAt{pc: 4, op: OP_MOD, a: 2, b: 3, c: 4},
		) ||
		!pat.hasABs(abAt{pc: 5, op: OP_RETURN, a: 2, b: 2}) {
		return spec, false
	}
	spec.mulConst = int64(DecodesBx(code[0]))
	switch DecodeOp(code[3]) {
	case OP_LOADINT:
		if DecodeA(code[3]) != 4 {
			return spec, false
		}
		spec.modConst = runtime.IntValue(int64(DecodesBx(code[3])))
	case OP_LOADK:
		if DecodeA(code[3]) != 4 {
			return spec, false
		}
		bx := DecodeBx(code[3])
		if bx < 0 || bx >= len(p.Constants) {
			return spec, false
		}
		spec.modConst = p.Constants[bx]
	case OP_GETGLOBAL:
		if DecodeA(code[3]) != 4 {
			return spec, false
		}
		bx := DecodeBx(code[3])
		if bx < 0 || bx >= len(p.Constants) || !p.Constants[bx].IsString() {
			return spec, false
		}
		spec.modGlobal = p.Constants[bx].Str()
	default:
		return spec, false
	}
	return spec, true
}

func (vm *VM) runAffineModuloIntLeafRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := affineModuloIntLeafSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	modValue := spec.modConst
	if spec.modGlobal != "" {
		modValue = vm.GetGlobal(spec.modGlobal)
	}
	if modValue.RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	mod := modValue.RawInt()
	if mod == 0 {
		return false, nil, nil
	}
	n := (args[0].RawInt()*spec.mulConst + args[1].RawInt()) % mod
	return true, []runtime.Value{runtime.IntValue(n)}, nil
}
