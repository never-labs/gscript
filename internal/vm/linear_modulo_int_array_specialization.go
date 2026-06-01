package vm

import "github.com/never-labs/leia/internal/runtime"

type linearModuloIntArrayBuilderSpec struct {
	indexMul  int64
	saltMul   int64
	lengthMul int64
	modulus   int64
}

func isLinearModuloIntArrayBuilderProto(p *FuncProto) bool {
	_, ok := linearModuloIntArrayBuilderSpecForProto(p)
	return ok
}

func linearModuloIntArrayBuilderSpecForProto(p *FuncProto) (linearModuloIntArrayBuilderSpec, bool) {
	var spec linearModuloIntArrayBuilderSpec
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 20 || len(p.Constants) < 1 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_NEWTABLE, 1: OP_LOADINT, 2: OP_MOVE, 3: OP_LOADINT, 4: OP_FORPREP,
		5: OP_LOADINT, 6: OP_MUL, 7: OP_LOADINT, 8: OP_MUL, 9: OP_ADD,
		10: OP_LOADINT, 11: OP_MUL, 12: OP_ADD, 13: OP_LOADK, 14: OP_MOD,
		15: OP_MOVE, 16: OP_SETTABLE, 17: OP_FORLOOP, 18: OP_MOVE, 19: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	if DecodeA(code[0]) != 2 ||
		DecodeA(code[1]) != 3 || DecodesBx(code[1]) != 1 ||
		DecodeA(code[2]) != 4 || DecodeB(code[2]) != 0 ||
		DecodeA(code[3]) != 5 || DecodesBx(code[3]) != 1 ||
		DecodeA(code[6]) != 10 || DecodeB(code[6]) != 6 || DecodeC(code[6]) != 11 ||
		DecodeA(code[8]) != 11 || DecodeB(code[8]) != 1 || DecodeC(code[8]) != 12 ||
		DecodeA(code[9]) != 9 || DecodeB(code[9]) != 10 || DecodeC(code[9]) != 11 ||
		DecodeA(code[11]) != 10 || DecodeB(code[11]) != 0 || DecodeC(code[11]) != 11 ||
		DecodeA(code[12]) != 8 || DecodeB(code[12]) != 9 || DecodeC(code[12]) != 10 ||
		DecodeA(code[14]) != 7 || DecodeB(code[14]) != 8 || DecodeC(code[14]) != 9 ||
		DecodeA(code[15]) != 8 || DecodeB(code[15]) != 6 ||
		DecodeA(code[16]) != 2 || DecodeB(code[16]) != 8 || DecodeC(code[16]) != 7 ||
		DecodeA(code[18]) != 6 || DecodeB(code[18]) != 2 ||
		DecodeA(code[19]) != 6 || DecodeB(code[19]) != 2 {
		return spec, false
	}
	idx := DecodeBx(code[13])
	if idx < 0 || idx >= len(p.Constants) || p.Constants[idx].RawType() != runtime.TypeInt {
		return spec, false
	}
	spec = linearModuloIntArrayBuilderSpec{
		indexMul:  int64(DecodesBx(code[5])),
		saltMul:   int64(DecodesBx(code[7])),
		lengthMul: int64(DecodesBx(code[10])),
		modulus:   p.Constants[idx].RawInt(),
	}
	if spec.modulus == 0 {
		return linearModuloIntArrayBuilderSpec{}, false
	}
	return spec, true
}

func (vm *VM) runLinearModuloIntArrayBuilderRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || args[0].RawType() != runtime.TypeInt || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := linearModuloIntArrayBuilderSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	n := args[0].RawInt()
	if n < 0 || n > int64(int(n)) {
		return false, nil, nil
	}
	t := runtime.NewLinearModuloIntArray(n, args[1].RawInt(), spec.indexMul, spec.saltMul, spec.lengthMul, spec.modulus)
	return true, []runtime.Value{runtime.TableValue(t)}, nil
}

type indexedModuloIntArrayFoldSpec struct {
	init      int64
	hashMul   int64
	indexMod  int64
	indexBias int64
	modGlobal string
}

func isIndexedModuloIntArrayFoldProto(p *FuncProto) bool {
	_, ok := indexedModuloIntArrayFoldSpecForProto(p)
	return ok
}

func indexedModuloIntArrayFoldSpecForProto(p *FuncProto) (indexedModuloIntArrayFoldSpec, bool) {
	var spec indexedModuloIntArrayFoldSpec
	if p == nil || p.NumParams != 2 || p.UsesVarargBytecode || len(p.Code) != 21 || len(p.Constants) < 1 {
		return spec, false
	}
	code := p.Code
	required := map[int]Opcode{
		0: OP_LOADINT, 1: OP_LOADINT, 2: OP_MOVE, 3: OP_LOADINT, 4: OP_FORPREP,
		5: OP_LOADINT, 6: OP_MUL, 7: OP_MOVE, 8: OP_GETTABLE, 9: OP_LOADINT,
		10: OP_MOD, 11: OP_LOADINT, 12: OP_ADD, 13: OP_MUL, 14: OP_ADD,
		15: OP_GETGLOBAL, 16: OP_MOD, 17: OP_MOVE, 18: OP_FORLOOP, 19: OP_MOVE,
		20: OP_RETURN,
	}
	for pc, op := range required {
		if DecodeOp(code[pc]) != op {
			return spec, false
		}
	}
	if DecodeA(code[0]) != 2 ||
		DecodeA(code[1]) != 3 || DecodesBx(code[1]) != 1 ||
		DecodeA(code[2]) != 4 || DecodeB(code[2]) != 1 ||
		DecodeA(code[3]) != 5 || DecodesBx(code[3]) != 1 ||
		DecodeA(code[6]) != 9 || DecodeB(code[6]) != 2 || DecodeC(code[6]) != 10 ||
		DecodeA(code[7]) != 12 || DecodeB(code[7]) != 6 ||
		DecodeA(code[8]) != 11 || DecodeB(code[8]) != 0 || DecodeC(code[8]) != 12 ||
		DecodeA(code[10]) != 13 || DecodeB(code[10]) != 6 || DecodeC(code[10]) != 14 ||
		DecodeA(code[12]) != 12 || DecodeB(code[12]) != 13 || DecodeC(code[12]) != 14 ||
		DecodeA(code[13]) != 10 || DecodeB(code[13]) != 11 || DecodeC(code[13]) != 12 ||
		DecodeA(code[14]) != 8 || DecodeB(code[14]) != 9 || DecodeC(code[14]) != 10 ||
		DecodeA(code[16]) != 7 || DecodeB(code[16]) != 8 || DecodeC(code[16]) != 9 ||
		DecodeA(code[17]) != 2 || DecodeB(code[17]) != 7 ||
		DecodeA(code[19]) != 6 || DecodeB(code[19]) != 2 ||
		DecodeA(code[20]) != 6 || DecodeB(code[20]) != 2 {
		return spec, false
	}
	modGlobal, ok := constStringAt(p, DecodeBx(code[15]))
	if !ok {
		return spec, false
	}
	spec = indexedModuloIntArrayFoldSpec{
		init:      int64(DecodesBx(code[0])),
		hashMul:   int64(DecodesBx(code[5])),
		indexMod:  int64(DecodesBx(code[9])),
		indexBias: int64(DecodesBx(code[11])),
		modGlobal: modGlobal,
	}
	if spec.indexMod == 0 || spec.modGlobal == "" {
		return indexedModuloIntArrayFoldSpec{}, false
	}
	return spec, true
}

func (vm *VM) runIndexedModuloIntArrayFoldRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 2 || !args[0].IsTable() || args[1].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	spec, ok := indexedModuloIntArrayFoldSpecForProto(cl.Proto)
	if !ok {
		return false, nil, nil
	}
	modValue := vm.GetGlobal(spec.modGlobal)
	if modValue.RawType() != runtime.TypeInt || modValue.RawInt() == 0 {
		return false, nil, nil
	}
	n := args[1].RawInt()
	if n < 0 || n > int64(int(n)) {
		return false, nil, nil
	}
	region, ok := args[0].Table().PlainIntArrayRegionForNumericSpecialization(1, int(n))
	if !ok {
		return false, nil, nil
	}
	h := spec.init
	mod := modValue.RawInt()
	for i := int64(1); i <= n; i++ {
		h = (h*spec.hashMul + region[i-1]*(i%spec.indexMod+spec.indexBias)) % mod
	}
	return true, []runtime.Value{runtime.IntValue(h)}, nil
}
