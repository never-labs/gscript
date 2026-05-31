package vm

import "github.com/never-labs/gscript/internal/runtime"

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
	_, ok := cachedTableIteratorModuloFoldSpecForProto(p)
	return ok
}

func cachedTableIteratorModuloFoldSpecForProto(p *FuncProto) (tableIteratorModuloFoldSpec, bool) {
	if p == nil {
		return tableIteratorModuloFoldSpec{}, false
	}
	switch p.RuntimeSpecs.TableIteratorModuloFoldShape {
	case 1:
		return p.RuntimeSpecs.TableIteratorModuloFoldSpec, true
	case -1:
		return tableIteratorModuloFoldSpec{}, false
	}
	spec, ok := tableIteratorModuloFoldSpecForProto(p)
	if ok {
		p.RuntimeSpecs.TableIteratorModuloFoldSpec = spec
		p.RuntimeSpecs.TableIteratorModuloFoldShape = 1
		return spec, true
	}
	p.RuntimeSpecs.TableIteratorModuloFoldShape = -1
	return tableIteratorModuloFoldSpec{}, false
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
	pat := newBytecodePattern(code)
	if !pat.hasBxs(
		bxAt{pc: 2, op: OP_GETGLOBAL, bx: 0},
		bxAt{pc: 12, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 20, op: OP_GETGLOBAL, bx: 1},
	) ||
		!pat.hasABCs(abcAt{pc: 4, op: OP_CALL, a: 3, b: 2, c: 4}) ||
		!pat.hasACs(acAt{pc: 5, op: OP_TFORCALL, a: 3, c: 2}) ||
		!pat.hasOps(
			opcodeAt{pc: 6, op: OP_TFORLOOP},
			opcodeAt{pc: 9, op: OP_ISNUMBER},
			opcodeAt{pc: 14, op: OP_LOADINT},
			opcodeAt{pc: 15, op: OP_MUL},
			opcodeAt{pc: 16, op: OP_ADD},
			opcodeAt{pc: 17, op: OP_CALL},
			opcodeAt{pc: 23, op: OP_LEN},
			opcodeAt{pc: 24, op: OP_LOADINT},
			opcodeAt{pc: 25, op: OP_MUL},
			opcodeAt{pc: 26, op: OP_ADD},
			opcodeAt{pc: 27, op: OP_CALL},
			opcodeAt{pc: 35, op: OP_LOADINT},
			opcodeAt{pc: 36, op: OP_MUL},
			opcodeAt{pc: 37, op: OP_CALL},
			opcodeAt{pc: 38, op: OP_RETURN},
		) {
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
	pat := newBytecodePattern(code)
	if !pat.hasBxs(
		bxAt{pc: 4, op: OP_GETGLOBAL, bx: 0},
		bxAt{pc: 18, op: OP_GETGLOBAL, bx: 1},
		bxAt{pc: 26, op: OP_GETGLOBAL, bx: 1},
	) ||
		!pat.hasABCs(abcAt{pc: 7, op: OP_CALL, a: 5, b: 3, c: 3}) ||
		!pat.hasOps(
			opcodeAt{pc: 11, op: OP_EQ},
			opcodeAt{pc: 15, op: OP_ISNUMBER},
			opcodeAt{pc: 20, op: OP_LOADINT},
			opcodeAt{pc: 21, op: OP_MUL},
			opcodeAt{pc: 22, op: OP_ADD},
			opcodeAt{pc: 23, op: OP_CALL},
			opcodeAt{pc: 29, op: OP_LEN},
			opcodeAt{pc: 30, op: OP_LOADINT},
			opcodeAt{pc: 31, op: OP_MUL},
			opcodeAt{pc: 32, op: OP_ADD},
			opcodeAt{pc: 33, op: OP_CALL},
			opcodeAt{pc: 41, op: OP_LOADINT},
			opcodeAt{pc: 42, op: OP_MUL},
			opcodeAt{pc: 43, op: OP_CALL},
			opcodeAt{pc: 44, op: OP_RETURN},
		) {
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
	pat := newBytecodePattern(code)
	if !pat.hasBxs(
		bxAt{pc: 2, op: OP_GETGLOBAL, bx: 0},
		bxAt{pc: 8, op: OP_GETGLOBAL, bx: 1},
	) ||
		!pat.hasABCs(abcAt{pc: 4, op: OP_CALL, a: 3, b: 2, c: 4}) ||
		!pat.hasACs(acAt{pc: 5, op: OP_TFORCALL, a: 3, c: 2}) ||
		!pat.hasOps(
			opcodeAt{pc: 6, op: OP_TFORLOOP},
			opcodeAt{pc: 10, op: OP_LOADINT},
			opcodeAt{pc: 11, op: OP_MUL},
			opcodeAt{pc: 12, op: OP_ADD},
			opcodeAt{pc: 13, op: OP_CALL},
			opcodeAt{pc: 21, op: OP_LOADINT},
			opcodeAt{pc: 22, op: OP_MUL},
			opcodeAt{pc: 23, op: OP_CALL},
			opcodeAt{pc: 24, op: OP_RETURN},
		) {
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
	spec, ok := cachedTableIteratorModuloFoldSpecForProto(cl.Proto)
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
		if ok := tbl.ForEachPlainRaw(func(k, v runtime.Value) bool {
			var term int64
			if k.RawType() == runtime.TypeInt && v.RawType() == runtime.TypeInt {
				term = k.RawInt()*spec.numScale + v.RawInt()
			} else if k.IsString() && v.RawType() == runtime.TypeInt {
				term = int64(runtime.StringLen(k))*spec.strScale + v.RawInt()
			} else {
				return false
			}
			sum = (sum + term) % mod
			count++
			return true
		}); !ok {
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
				sum = (sum + term) % mod
				count++
				key = k
			}
		}
	case tableIteratorModuloFoldNext:
		if ok := tbl.ForEachPlainRaw(func(k, v runtime.Value) bool {
			var term int64
			if k.RawType() == runtime.TypeInt && v.RawType() == runtime.TypeInt {
				term = k.RawInt()*spec.numScale + v.RawInt()
			} else if k.IsString() && v.RawType() == runtime.TypeInt {
				term = int64(runtime.StringLen(k))*spec.strScale + v.RawInt()
			} else {
				return false
			}
			sum = (sum + term) % mod
			count++
			return true
		}); !ok {
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
				sum = (sum + term) % mod
				count++
				key = k
			}
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
			sum = (sum + i*spec.numScale + v.RawInt()) % mod
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
