package vm

import "github.com/gscript/gscript/internal/runtime"

const (
	tableIteratorModuloFoldPairs = iota + 1
	tableIteratorModuloFoldNext
	tableIteratorModuloFoldIPairs
)

type tableIteratorModuloFoldSpec struct {
	mode       int
	addGlobal  string
	numScale   int64
	strScale   int64
	countScale int64
}

func isTableIteratorModuloFoldProto(p *FuncProto) bool {
	_, ok := tableIteratorModuloFoldSpecForProto(p)
	return ok
}

func tableIteratorModuloFoldSpecForProto(p *FuncProto) (tableIteratorModuloFoldSpec, bool) {
	if spec, ok := tableIteratorModuloFoldPairsSpec(p); ok {
		return spec, true
	}
	if spec, ok := tableIteratorModuloFoldNextSpec(p); ok {
		return spec, true
	}
	if spec, ok := tableIteratorModuloFoldIPairsSpec(p); ok {
		return spec, true
	}
	return tableIteratorModuloFoldSpec{}, false
}

func tableIteratorModuloFoldPairsSpec(p *FuncProto) (tableIteratorModuloFoldSpec, bool) {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 39 ||
		!constString(p, 0, "pairs") || !stringConst(p.Constants, 1) {
		return tableIteratorModuloFoldSpec{}, false
	}
	code := p.Code
	if DecodeOp(code[2]) != OP_GETGLOBAL || DecodeBx(code[2]) != 0 ||
		DecodeOp(code[4]) != OP_CALL || DecodeA(code[4]) != 3 || DecodeB(code[4]) != 2 || DecodeC(code[4]) != 4 ||
		DecodeOp(code[5]) != OP_TFORCALL || DecodeA(code[5]) != 3 || DecodeC(code[5]) != 2 ||
		DecodeOp(code[6]) != OP_TFORLOOP ||
		DecodeOp(code[9]) != OP_ISNUMBER ||
		DecodeOp(code[12]) != OP_GETGLOBAL || DecodeBx(code[12]) != 1 ||
		DecodeOp(code[14]) != OP_LOADINT ||
		DecodeOp(code[15]) != OP_MUL ||
		DecodeOp(code[16]) != OP_ADD ||
		DecodeOp(code[17]) != OP_CALL ||
		DecodeOp(code[20]) != OP_GETGLOBAL || DecodeBx(code[20]) != 1 ||
		DecodeOp(code[23]) != OP_LEN ||
		DecodeOp(code[24]) != OP_LOADINT ||
		DecodeOp(code[25]) != OP_MUL ||
		DecodeOp(code[26]) != OP_ADD ||
		DecodeOp(code[27]) != OP_CALL ||
		DecodeOp(code[35]) != OP_LOADINT ||
		DecodeOp(code[36]) != OP_MUL ||
		DecodeOp(code[37]) != OP_CALL ||
		DecodeOp(code[38]) != OP_RETURN {
		return tableIteratorModuloFoldSpec{}, false
	}
	return tableIteratorModuloFoldSpec{
		mode:       tableIteratorModuloFoldPairs,
		addGlobal:  p.Constants[1].Str(),
		numScale:   int64(DecodesBx(code[14])),
		strScale:   int64(DecodesBx(code[24])),
		countScale: int64(DecodesBx(code[35])),
	}, true
}

func tableIteratorModuloFoldNextSpec(p *FuncProto) (tableIteratorModuloFoldSpec, bool) {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 45 ||
		!constString(p, 0, "next") || !stringConst(p.Constants, 1) {
		return tableIteratorModuloFoldSpec{}, false
	}
	code := p.Code
	if DecodeOp(code[4]) != OP_GETGLOBAL || DecodeBx(code[4]) != 0 ||
		DecodeOp(code[7]) != OP_CALL || DecodeA(code[7]) != 5 || DecodeB(code[7]) != 3 || DecodeC(code[7]) != 3 ||
		DecodeOp(code[11]) != OP_EQ ||
		DecodeOp(code[15]) != OP_ISNUMBER ||
		DecodeOp(code[18]) != OP_GETGLOBAL || DecodeBx(code[18]) != 1 ||
		DecodeOp(code[20]) != OP_LOADINT ||
		DecodeOp(code[21]) != OP_MUL ||
		DecodeOp(code[22]) != OP_ADD ||
		DecodeOp(code[23]) != OP_CALL ||
		DecodeOp(code[26]) != OP_GETGLOBAL || DecodeBx(code[26]) != 1 ||
		DecodeOp(code[29]) != OP_LEN ||
		DecodeOp(code[30]) != OP_LOADINT ||
		DecodeOp(code[31]) != OP_MUL ||
		DecodeOp(code[32]) != OP_ADD ||
		DecodeOp(code[33]) != OP_CALL ||
		DecodeOp(code[41]) != OP_LOADINT ||
		DecodeOp(code[42]) != OP_MUL ||
		DecodeOp(code[43]) != OP_CALL ||
		DecodeOp(code[44]) != OP_RETURN {
		return tableIteratorModuloFoldSpec{}, false
	}
	return tableIteratorModuloFoldSpec{
		mode:       tableIteratorModuloFoldNext,
		addGlobal:  p.Constants[1].Str(),
		numScale:   int64(DecodesBx(code[20])),
		strScale:   int64(DecodesBx(code[30])),
		countScale: int64(DecodesBx(code[41])),
	}, true
}

func tableIteratorModuloFoldIPairsSpec(p *FuncProto) (tableIteratorModuloFoldSpec, bool) {
	if p == nil || p.NumParams != 1 || p.UsesVarargBytecode || len(p.Code) != 25 ||
		!constString(p, 0, "ipairs") || !stringConst(p.Constants, 1) {
		return tableIteratorModuloFoldSpec{}, false
	}
	code := p.Code
	if DecodeOp(code[2]) != OP_GETGLOBAL || DecodeBx(code[2]) != 0 ||
		DecodeOp(code[4]) != OP_CALL || DecodeA(code[4]) != 3 || DecodeB(code[4]) != 2 || DecodeC(code[4]) != 4 ||
		DecodeOp(code[5]) != OP_TFORCALL || DecodeA(code[5]) != 3 || DecodeC(code[5]) != 2 ||
		DecodeOp(code[6]) != OP_TFORLOOP ||
		DecodeOp(code[8]) != OP_GETGLOBAL || DecodeBx(code[8]) != 1 ||
		DecodeOp(code[10]) != OP_LOADINT ||
		DecodeOp(code[11]) != OP_MUL ||
		DecodeOp(code[12]) != OP_ADD ||
		DecodeOp(code[13]) != OP_CALL ||
		DecodeOp(code[21]) != OP_LOADINT ||
		DecodeOp(code[22]) != OP_MUL ||
		DecodeOp(code[23]) != OP_CALL ||
		DecodeOp(code[24]) != OP_RETURN {
		return tableIteratorModuloFoldSpec{}, false
	}
	return tableIteratorModuloFoldSpec{
		mode:       tableIteratorModuloFoldIPairs,
		addGlobal:  p.Constants[1].Str(),
		numScale:   int64(DecodesBx(code[10])),
		countScale: int64(DecodesBx(code[21])),
	}, true
}

func (vm *VM) runTableIteratorModuloFoldRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 1 || !args[0].IsTable() {
		return false, nil, nil
	}
	spec, ok := tableIteratorModuloFoldSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	tbl := args[0].Table()
	if tbl == nil || tbl.GetMetatable() != nil {
		return false, nil, nil
	}
	mod, ok := vm.tableIteratorModuloFoldModulus(spec.addGlobal)
	if !ok || mod == 0 {
		return false, nil, nil
	}
	var sum, count int64
	switch spec.mode {
	case tableIteratorModuloFoldPairs:
		key := runtime.NilValue()
		for {
			k, v, ok := tbl.Next(key)
			if !ok {
				break
			}
			var term int64
			if k.RawType() == runtime.TypeInt && v.RawType() == runtime.TypeInt {
				term = k.RawInt()*spec.numScale + v.RawInt()
			} else if k.IsString() && v.RawType() == runtime.TypeInt {
				term = int64(runtime.StringLen(k))*spec.strScale + v.RawInt()
			} else {
				return false, nil, nil
			}
			sum = positiveModInt64(sum+term, mod)
			count++
			key = k
		}
	case tableIteratorModuloFoldNext:
		key := runtime.NilValue()
		for {
			k, v, ok := tbl.Next(key)
			if !ok {
				break
			}
			var term int64
			if k.RawType() == runtime.TypeInt && v.RawType() == runtime.TypeInt {
				term = k.RawInt()*spec.numScale + v.RawInt()
			} else if k.IsString() && v.RawType() == runtime.TypeInt {
				term = int64(runtime.StringLen(k))*spec.strScale + v.RawInt()
			} else {
				return false, nil, nil
			}
			sum = positiveModInt64(sum+term, mod)
			count++
			key = k
		}
	case tableIteratorModuloFoldIPairs:
		for i := int64(1); ; i++ {
			v := tbl.RawGetInt(i)
			if v.IsNil() {
				break
			}
			if v.RawType() != runtime.TypeInt {
				return false, nil, nil
			}
			sum = positiveModInt64(sum+i*spec.numScale+v.RawInt(), mod)
			count++
		}
	default:
		return false, nil, nil
	}
	result := positiveModInt64(sum+count*spec.countScale, mod)
	return true, []runtime.Value{runtime.IntValue(result)}, nil
}

func (vm *VM) tableIteratorModuloFoldModulus(addGlobal string) (int64, bool) {
	fn := vm.GetGlobal(addGlobal)
	cl, ok := closureFromValue(fn)
	if !ok || cl == nil || !isTwoArgModAddGlobalLeafProto(cl.Proto) {
		return 0, false
	}
	modName := cl.Proto.Constants[0]
	if !modName.IsString() {
		return 0, false
	}
	mod := vm.GetGlobal(modName.Str())
	if mod.RawType() != runtime.TypeInt {
		return 0, false
	}
	return mod.RawInt(), true
}

func isTwoArgModAddGlobalLeafProto(proto *FuncProto) bool {
	if proto == nil || proto.NumParams != 2 || proto.UsesVarargBytecode || len(proto.Code) != 4 || len(proto.Constants) != 1 {
		return false
	}
	return DecodeOp(proto.Code[0]) == OP_ADD &&
		DecodeA(proto.Code[0]) == 3 &&
		DecodeB(proto.Code[0]) == 0 &&
		DecodeC(proto.Code[0]) == 1 &&
		DecodeOp(proto.Code[1]) == OP_GETGLOBAL &&
		DecodeA(proto.Code[1]) == 4 &&
		DecodeBx(proto.Code[1]) == 0 &&
		DecodeOp(proto.Code[2]) == OP_MOD &&
		DecodeA(proto.Code[2]) == 2 &&
		DecodeB(proto.Code[2]) == 3 &&
		DecodeC(proto.Code[2]) == 4 &&
		DecodeOp(proto.Code[3]) == OP_RETURN &&
		DecodeA(proto.Code[3]) == 2 &&
		DecodeB(proto.Code[3]) == 2
}

func positiveModInt64(a, b int64) int64 {
	r := a % b
	if r != 0 && (r^b) < 0 {
		r += b
	}
	return r
}
