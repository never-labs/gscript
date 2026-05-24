package vm

import "github.com/gscript/gscript/internal/runtime"

type insertRemoveChecksumSpec struct {
	tempOffset int64
	hotModulo  int64
	coldModulo int64
	addGlobal  string
}

func isInsertRemoveChecksumProto(p *FuncProto) bool {
	_, ok := insertRemoveChecksumSpecForProto(p)
	return ok
}

func insertRemoveChecksumSpecForProto(p *FuncProto) (insertRemoveChecksumSpec, bool) {
	var spec insertRemoveChecksumSpec
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 88 ||
		len(p.Constants) < 9 || !stringConst(p.Constants, 2) {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_NEWTABLE ||
		DecodeOp(code[4]) != OP_FORPREP ||
		DecodeOp(code[7]) != OP_SETTABLE ||
		DecodeOp(code[8]) != OP_GETGLOBAL ||
		DecodeOp(code[12]) != OP_CONCAT ||
		DecodeOp(code[15]) != OP_CALL ||
		DecodeOp(code[16]) != OP_FORLOOP ||
		DecodeOp(code[17]) != OP_GETGLOBAL || DecodeBx(code[17]) != 2 ||
		DecodeOp(code[18]) != OP_GETGLOBAL ||
		DecodeOp(code[20]) != OP_CALL ||
		DecodeOp(code[23]) != OP_LEN ||
		DecodeOp(code[24]) != OP_CALL ||
		DecodeOp(code[28]) != OP_FORPREP ||
		DecodeOp(code[29]) != OP_MOD ||
		DecodeOp(code[30]) != OP_LOADINT ||
		DecodeOp(code[31]) != OP_ADD ||
		DecodeOp(code[32]) != OP_GETGLOBAL ||
		DecodeOp(code[33]) != OP_GETFIELD ||
		DecodeOp(code[37]) != OP_CALL ||
		DecodeOp(code[38]) != OP_GETGLOBAL ||
		DecodeOp(code[39]) != OP_GETFIELD ||
		DecodeOp(code[43]) != OP_CALL ||
		DecodeOp(code[45]) != OP_LOADINT ||
		DecodeOp(code[46]) != OP_MOD ||
		DecodeOp(code[47]) != OP_CONCAT ||
		DecodeOp(code[53]) != OP_LOADINT ||
		DecodeOp(code[54]) != OP_MOD ||
		DecodeOp(code[60]) != OP_LOADINT ||
		DecodeOp(code[61]) != OP_ADD ||
		DecodeOp(code[66]) != OP_LOADINT ||
		DecodeOp(code[67]) != OP_ADD ||
		DecodeOp(code[70]) != OP_GETGLOBAL || DecodeBx(code[70]) != 2 ||
		DecodeOp(code[72]) != OP_GETGLOBAL ||
		DecodeOp(code[75]) != OP_CALL ||
		DecodeOp(code[76]) != OP_GETGLOBAL ||
		DecodeOp(code[78]) != OP_CALL ||
		DecodeOp(code[81]) != OP_LEN ||
		DecodeOp(code[83]) != OP_CALL ||
		DecodeOp(code[85]) != OP_FORLOOP ||
		DecodeOp(code[87]) != OP_RETURN {
		return spec, false
	}
	spec = insertRemoveChecksumSpec{
		tempOffset: int64(DecodesBx(code[60])),
		hotModulo:  int64(DecodesBx(code[45])),
		coldModulo: int64(DecodesBx(code[53])),
		addGlobal:  p.Constants[2].Str(),
	}
	if spec.hotModulo <= 0 || spec.coldModulo <= 0 {
		return insertRemoveChecksumSpec{}, false
	}
	return spec, true
}

func (vm *VM) runInsertRemoveChecksumRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := insertRemoveChecksumSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	mod, ok := vm.tableIteratorModuloFoldModulus(spec.addGlobal)
	if !ok || mod == 0 {
		return false, nil, nil
	}
	n := args[0].RawInt()
	reps := args[1].RawInt()
	if n < 1 {
		return false, nil, nil
	}
	values := make([]int64, n+1)
	for i := int64(1); i <= n; i++ {
		values[i] = i
	}
	checksum := positiveModInt64(n+n, mod)
	for r := int64(1); r <= reps; r++ {
		pos := positiveModInt64(r, n) + 1
		removed := values[pos]
		values[pos] = r
		_ = positiveModInt64(r, spec.hotModulo)
		if positiveModInt64(r, spec.coldModulo) == 0 {
			_ = n + spec.tempOffset
		}
		checksum = positiveModInt64(checksum+removed+n+n, mod)
	}
	return true, []runtime.Value{runtime.IntValue(checksum)}, nil
}
