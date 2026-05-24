package vm

import "github.com/gscript/gscript/internal/runtime"

type tableAffineUpdateModuloLeafSpec struct {
	updateField string
	biasField   string
	xScale      int64
	yScale      int64
	countScale  int64
	modGlobal   string
}

type tableAffineUpdateModuloLeafCache struct {
	recognized bool
	spec       tableAffineUpdateModuloLeafSpec
	countGet   runtime.FieldCacheEntry
	countSet   runtime.FieldCacheEntry
	biasGet    runtime.FieldCacheEntry
}

func isTableAffineUpdateModuloLeafProto(p *FuncProto) bool {
	_, ok := tableAffineUpdateModuloLeafSpecForProto(p)
	return ok
}

func tableAffineUpdateModuloLeafSpecForProto(p *FuncProto) (tableAffineUpdateModuloLeafSpec, bool) {
	var spec tableAffineUpdateModuloLeafSpec
	if p != nil && p.TableAffineUpdateModuloSpecialization != nil {
		c := p.TableAffineUpdateModuloSpecialization
		return c.spec, c.recognized
	}
	spec, ok := recognizeTableAffineUpdateModuloLeafSpec(p)
	if p != nil {
		p.TableAffineUpdateModuloSpecialization = &tableAffineUpdateModuloLeafCache{
			recognized: ok,
			spec:       spec,
		}
	}
	return spec, ok
}

func recognizeTableAffineUpdateModuloLeafSpec(p *FuncProto) (tableAffineUpdateModuloLeafSpec, bool) {
	var spec tableAffineUpdateModuloLeafSpec
	if p == nil || p.NumParams != 3 || p.IsVarArg || len(p.Code) != 18 {
		return spec, false
	}
	code := p.Code
	if DecodeOp(code[0]) != OP_GETFIELD || DecodeA(code[0]) != 4 || DecodeB(code[0]) != 0 ||
		DecodeOp(code[1]) != OP_LOADINT || DecodeA(code[1]) != 5 || DecodesBx(code[1]) != 1 ||
		DecodeOp(code[2]) != OP_ADD || DecodeA(code[2]) != 3 || DecodeB(code[2]) != 4 || DecodeC(code[2]) != 5 ||
		DecodeOp(code[3]) != OP_SETFIELD || DecodeA(code[3]) != 0 || DecodeC(code[3]) != 3 ||
		DecodeOp(code[4]) != OP_LOADINT || DecodeA(code[4]) != 8 ||
		DecodeOp(code[5]) != OP_MUL || DecodeA(code[5]) != 7 || DecodeB(code[5]) != 1 || DecodeC(code[5]) != 8 ||
		DecodeOp(code[6]) != OP_LOADINT || DecodeA(code[6]) != 9 ||
		DecodeOp(code[7]) != OP_MUL || DecodeA(code[7]) != 8 || DecodeB(code[7]) != 2 || DecodeC(code[7]) != 9 ||
		DecodeOp(code[8]) != OP_ADD || DecodeA(code[8]) != 6 || DecodeB(code[8]) != 7 || DecodeC(code[8]) != 8 ||
		DecodeOp(code[9]) != OP_GETFIELD || DecodeA(code[9]) != 7 || DecodeB(code[9]) != 0 ||
		DecodeOp(code[10]) != OP_ADD || DecodeA(code[10]) != 5 || DecodeB(code[10]) != 6 || DecodeC(code[10]) != 7 ||
		DecodeOp(code[11]) != OP_GETFIELD || DecodeA(code[11]) != 7 || DecodeB(code[11]) != 0 ||
		DecodeOp(code[12]) != OP_LOADINT || DecodeA(code[12]) != 8 ||
		DecodeOp(code[13]) != OP_MUL || DecodeA(code[13]) != 6 || DecodeB(code[13]) != 7 || DecodeC(code[13]) != 8 ||
		DecodeOp(code[14]) != OP_ADD || DecodeA(code[14]) != 4 || DecodeB(code[14]) != 5 || DecodeC(code[14]) != 6 ||
		DecodeOp(code[15]) != OP_GETGLOBAL || DecodeA(code[15]) != 5 ||
		DecodeOp(code[16]) != OP_MOD || DecodeA(code[16]) != 3 || DecodeB(code[16]) != 4 || DecodeC(code[16]) != 5 ||
		DecodeOp(code[17]) != OP_RETURN || DecodeA(code[17]) != 3 || DecodeB(code[17]) != 2 {
		return spec, false
	}
	updateIdx := DecodeC(code[0])
	if updateIdx != DecodeB(code[3]) || updateIdx != DecodeC(code[11]) ||
		updateIdx < 0 || updateIdx >= len(p.Constants) || !p.Constants[updateIdx].IsString() {
		return spec, false
	}
	biasIdx := DecodeC(code[9])
	if biasIdx < 0 || biasIdx >= len(p.Constants) || !p.Constants[biasIdx].IsString() {
		return spec, false
	}
	modIdx := DecodeBx(code[15])
	if modIdx < 0 || modIdx >= len(p.Constants) || !p.Constants[modIdx].IsString() {
		return spec, false
	}
	spec.updateField = p.Constants[updateIdx].Str()
	spec.biasField = p.Constants[biasIdx].Str()
	spec.xScale = int64(DecodesBx(code[4]))
	spec.yScale = int64(DecodesBx(code[6]))
	spec.countScale = int64(DecodesBx(code[12]))
	spec.modGlobal = p.Constants[modIdx].Str()
	return spec, true
}

func (vm *VM) runTableAffineUpdateModuloLeafRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if len(args) != 3 || !args[0].IsTable() ||
		args[1].RawType() != runtime.TypeInt || args[2].RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	cache := tableAffineUpdateModuloLeafCacheForProto(cl.Proto)
	if cache == nil || !cache.recognized {
		return false, nil, nil
	}
	spec := cache.spec
	tbl := args[0].Table()
	countValue := tbl.RawGetStringCached(spec.updateField, &cache.countGet)
	biasValue := tbl.RawGetStringCached(spec.biasField, &cache.biasGet)
	modValue := vm.GetGlobal(spec.modGlobal)
	if countValue.RawType() != runtime.TypeInt ||
		biasValue.RawType() != runtime.TypeInt ||
		modValue.RawType() != runtime.TypeInt {
		return false, nil, nil
	}
	mod := modValue.RawInt()
	if mod == 0 {
		return false, nil, nil
	}
	count := countValue.RawInt() + 1
	tbl.RawSetStringCached(spec.updateField, runtime.IntValue(count), &cache.countSet)
	result := (args[1].RawInt()*spec.xScale +
		args[2].RawInt()*spec.yScale +
		biasValue.RawInt() +
		count*spec.countScale) % mod
	return true, []runtime.Value{runtime.IntValue(result)}, nil
}

func tableAffineUpdateModuloLeafCacheForProto(p *FuncProto) *tableAffineUpdateModuloLeafCache {
	if p == nil {
		return nil
	}
	if p.TableAffineUpdateModuloSpecialization == nil {
		tableAffineUpdateModuloLeafSpecForProto(p)
	}
	return p.TableAffineUpdateModuloSpecialization
}
