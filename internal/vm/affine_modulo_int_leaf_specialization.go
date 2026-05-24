package vm

import "github.com/gscript/gscript/internal/runtime"

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
	if DecodeOp(code[0]) != OP_LOADINT || DecodeA(code[0]) != 5 ||
		DecodeOp(code[1]) != OP_MUL || DecodeA(code[1]) != 4 || DecodeB(code[1]) != 0 || DecodeC(code[1]) != 5 ||
		DecodeOp(code[2]) != OP_ADD || DecodeA(code[2]) != 3 || DecodeB(code[2]) != 4 || DecodeC(code[2]) != 1 ||
		DecodeOp(code[4]) != OP_MOD || DecodeA(code[4]) != 2 || DecodeB(code[4]) != 3 || DecodeC(code[4]) != 4 ||
		DecodeOp(code[5]) != OP_RETURN || DecodeA(code[5]) != 2 || DecodeB(code[5]) != 2 {
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
