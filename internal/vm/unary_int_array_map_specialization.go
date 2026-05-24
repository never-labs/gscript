package vm

import "github.com/gscript/gscript/internal/runtime"

type unaryIntArrayMapSpec struct{}

type unaryIntAffineClosureSpec struct {
	mul int64
	add int64
}

func isUnaryIntArrayMapProto(p *FuncProto) bool {
	_, ok := unaryIntArrayMapSpecForProto(p)
	return ok
}

func unaryIntArrayMapSpecForProto(p *FuncProto) (unaryIntArrayMapSpec, bool) {
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 16 {
		return unaryIntArrayMapSpec{}, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_NEWTABLE, 1: OP_MOVE, 2: OP_LEN, 3: OP_LOADINT,
		4: OP_MOVE, 5: OP_LOADINT, 6: OP_FORPREP, 7: OP_MOVE,
		8: OP_MOVE, 9: OP_GETTABLE, 10: OP_CALL, 11: OP_MOVE,
		12: OP_SETTABLE, 13: OP_FORLOOP, 14: OP_MOVE, 15: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return unaryIntArrayMapSpec{}, false
		}
	}
	if DecodeA(code[0]) != 2 ||
		DecodeA(code[1]) != 4 || DecodeB(code[1]) != 0 ||
		DecodeA(code[2]) != 3 || DecodeB(code[2]) != 4 ||
		DecodeA(code[3]) != 4 || DecodesBx(code[3]) != 1 ||
		DecodeA(code[4]) != 5 || DecodeB(code[4]) != 3 ||
		DecodeA(code[5]) != 6 || DecodesBx(code[5]) != 1 ||
		DecodeA(code[7]) != 8 || DecodeB(code[7]) != 1 ||
		DecodeA(code[8]) != 10 || DecodeB(code[8]) != 7 ||
		DecodeA(code[9]) != 9 || DecodeB(code[9]) != 0 || DecodeC(code[9]) != 10 ||
		DecodeA(code[10]) != 8 ||
		DecodeA(code[11]) != 9 || DecodeB(code[11]) != 7 ||
		DecodeA(code[12]) != 2 || DecodeB(code[12]) != 9 || DecodeC(code[12]) != 8 ||
		DecodeA(code[14]) != 7 || DecodeB(code[14]) != 2 ||
		DecodeA(code[15]) != 7 || DecodeB(code[15]) != 2 {
		return unaryIntArrayMapSpec{}, false
	}
	return unaryIntArrayMapSpec{}, true
}

func unaryIntAffineClosureSpecForProto(p *FuncProto) (unaryIntAffineClosureSpec, bool) {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 5 {
		return unaryIntAffineClosureSpec{}, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_LOADINT || DecodeOp(code[1]) != OP_MUL ||
		DecodeOp(code[2]) != OP_LOADINT || DecodeOp(code[3]) != OP_ADD ||
		DecodeOp(code[4]) != OP_RETURN {
		return unaryIntAffineClosureSpec{}, false
	}
	if DecodeA(code[0]) != 3 ||
		DecodeA(code[1]) != 2 || DecodeB(code[1]) != 0 || DecodeC(code[1]) != 3 ||
		DecodeA(code[2]) != 3 ||
		DecodeA(code[3]) != 1 || DecodeB(code[3]) != 2 || DecodeC(code[3]) != 3 ||
		DecodeA(code[4]) != 1 || DecodeB(code[4]) != 2 {
		return unaryIntAffineClosureSpec{}, false
	}
	return unaryIntAffineClosureSpec{
		mul: int64(DecodesBx(code[0])),
		add: int64(DecodesBx(code[2])),
	}, true
}

func (vm *VM) runUnaryIntArrayMapRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || !args[0].IsTable() {
		return false, nil, nil
	}
	if _, ok := unaryIntArrayMapSpecForProto(cl.Proto); !ok {
		return false, nil, nil
	}
	fn, ok := closureFromValue(args[1])
	if !ok {
		return false, nil, nil
	}
	affine, ok := unaryIntAffineClosureSpecForProto(fn.Proto)
	if !ok {
		return false, nil, nil
	}
	n := int64(args[0].Table().Length())
	if n < 0 || n > int64(int(n)) {
		return false, nil, nil
	}
	region, ok := args[0].Table().PlainIntArrayRegionForNumericSpecialization(1, int(n))
	if !ok {
		return false, nil, nil
	}
	return true, []runtime.Value{runtime.TableValue(runtime.NewMappedIntArray(region, affine.mul, affine.add))}, nil
}
