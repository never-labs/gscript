package vm

import "github.com/never-labs/leia/internal/runtime"

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
	pat := newBytecodePattern(code)
	if !pat.hasAs(
		aAt{pc: 0, op: OP_NEWTABLE, a: 2},
		aAt{pc: 10, op: OP_CALL, a: 8},
	) ||
		!pat.hasASBxs(
			asbxAt{pc: 3, op: OP_LOADINT, a: 4, sbx: 1},
			asbxAt{pc: 5, op: OP_LOADINT, a: 6, sbx: 1},
		) ||
		!pat.hasABs(
			abAt{pc: 1, op: OP_MOVE, a: 4, b: 0},
			abAt{pc: 2, op: OP_LEN, a: 3, b: 4},
			abAt{pc: 4, op: OP_MOVE, a: 5, b: 3},
			abAt{pc: 7, op: OP_MOVE, a: 8, b: 1},
			abAt{pc: 8, op: OP_MOVE, a: 10, b: 7},
			abAt{pc: 11, op: OP_MOVE, a: 9, b: 7},
			abAt{pc: 14, op: OP_MOVE, a: 7, b: 2},
			abAt{pc: 15, op: OP_RETURN, a: 7, b: 2},
		) ||
		!pat.hasABCs(
			abcAt{pc: 9, op: OP_GETTABLE, a: 9, b: 0, c: 10},
			abcAt{pc: 12, op: OP_SETTABLE, a: 2, b: 9, c: 8},
		) ||
		!pat.hasOps(
			opcodeAt{pc: 6, op: OP_FORPREP},
			opcodeAt{pc: 13, op: OP_FORLOOP},
		) {
		return unaryIntArrayMapSpec{}, false
	}
	return unaryIntArrayMapSpec{}, true
}

func unaryIntAffineClosureSpecForProto(p *FuncProto) (unaryIntAffineClosureSpec, bool) {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 5 {
		return unaryIntAffineClosureSpec{}, false
	}
	code := p.Code
	pat := newBytecodePattern(code)
	if !pat.hasAs(
		aAt{pc: 0, op: OP_LOADINT, a: 3},
		aAt{pc: 2, op: OP_LOADINT, a: 3},
	) ||
		!pat.hasABCs(
			abcAt{pc: 1, op: OP_MUL, a: 2, b: 0, c: 3},
			abcAt{pc: 3, op: OP_ADD, a: 1, b: 2, c: 3},
		) ||
		!pat.hasABs(abAt{pc: 4, op: OP_RETURN, a: 1, b: 2}) {
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
