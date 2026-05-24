package vm

import "github.com/gscript/gscript/internal/runtime"

type linkedRecordChecksumSpec struct {
	valueMod   int64
	scanStep   int64
	rootModulo int64
	finalScale int64
	addGlobal  string
}

func isLinkedRecordChecksumProto(p *FuncProto) bool {
	_, ok := linkedRecordChecksumSpecForProto(p)
	return ok
}

func linkedRecordChecksumSpecForProto(p *FuncProto) (linkedRecordChecksumSpec, bool) {
	var spec linkedRecordChecksumSpec
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 59 ||
		len(p.Constants) < 7 || !stringConst(p.Constants, 6) {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_NEWTABLE ||
		DecodeOp(code[1]) != OP_LOADINT || DecodesBx(code[1]) != 0 ||
		DecodeOp(code[5]) != OP_FORPREP ||
		DecodeOp(code[6]) != OP_NEWTABLE ||
		DecodeOp(code[7]) != OP_LOADNIL ||
		DecodeOp(code[11]) != OP_FORPREP ||
		DecodeOp(code[12]) != OP_ADD ||
		DecodeOp(code[14]) != OP_MUL ||
		DecodeOp(code[15]) != OP_LOADINT ||
		DecodeOp(code[16]) != OP_MOD ||
		DecodeOp(code[18]) != OP_NEWOBJECTN ||
		DecodeOp(code[23]) != OP_SETFIELD ||
		DecodeOp(code[26]) != OP_SETTABLE ||
		DecodeOp(code[29]) != OP_FORLOOP ||
		DecodeOp(code[32]) != OP_LOADINT ||
		DecodeOp(code[33]) != OP_FORPREP ||
		DecodeOp(code[35]) != OP_GETTABLE ||
		DecodeOp(code[36]) != OP_GETGLOBAL || DecodeBx(code[36]) != 6 ||
		DecodeOp(code[38]) != OP_GETFIELD ||
		DecodeOp(code[39]) != OP_GETFIELD ||
		DecodeOp(code[40]) != OP_ADD ||
		DecodeOp(code[41]) != OP_CALL ||
		DecodeOp(code[43]) != OP_FORLOOP ||
		DecodeOp(code[45]) != OP_LOADINT ||
		DecodeOp(code[46]) != OP_MOD ||
		DecodeOp(code[47]) != OP_LOADINT ||
		DecodeOp(code[48]) != OP_ADD ||
		DecodeOp(code[49]) != OP_SETTABLE ||
		DecodeOp(code[50]) != OP_FORLOOP ||
		DecodeOp(code[51]) != OP_GETGLOBAL || DecodeBx(code[51]) != 6 ||
		DecodeOp(code[54]) != OP_LEN ||
		DecodeOp(code[55]) != OP_LOADINT ||
		DecodeOp(code[56]) != OP_MUL ||
		DecodeOp(code[57]) != OP_CALL ||
		DecodeOp(code[58]) != OP_RETURN {
		return spec, false
	}
	spec = linkedRecordChecksumSpec{
		valueMod:   int64(DecodesBx(code[15])),
		scanStep:   int64(DecodesBx(code[32])),
		rootModulo: int64(DecodesBx(code[45])),
		finalScale: int64(DecodesBx(code[55])),
		addGlobal:  p.Constants[6].Str(),
	}
	if spec.valueMod == 0 || spec.scanStep <= 0 || spec.rootModulo <= 0 {
		return linkedRecordChecksumSpec{}, false
	}
	return spec, true
}

func (vm *VM) runLinkedRecordChecksumRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := linkedRecordChecksumSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	mod, ok := vm.tableIteratorModuloFoldModulus(spec.addGlobal)
	if !ok || mod == 0 {
		return false, nil, nil
	}
	n := args[0].RawInt()
	reps := args[1].RawInt()
	checksum := int64(0)
	roots := runtime.NewTableSized(int(spec.rootModulo), 0)
	for r := int64(1); r <= reps; r++ {
		for i := int64(1); i <= n; i += spec.scanStep {
			value := positiveModInt64(i*r, spec.valueMod)
			checksum = positiveModInt64(checksum+i+r+value, mod)
		}
		roots.RawSetInt(positiveModInt64(r, spec.rootModulo)+1, runtime.BoolValue(true))
	}
	result := positiveModInt64(checksum+int64(roots.Len())*spec.finalScale, mod)
	return true, []runtime.Value{runtime.IntValue(result)}, nil
}
