package vm

import "github.com/never-labs/gscript/internal/runtime"

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
	pat := newBytecodePattern(code)
	if !pat.hasABs(
		abAt{pc: 0, op: OP_GETFIELD, a: 4, b: 0},
		abAt{pc: 9, op: OP_GETFIELD, a: 7, b: 0},
		abAt{pc: 11, op: OP_GETFIELD, a: 7, b: 0},
		abAt{pc: 17, op: OP_RETURN, a: 3, b: 2},
	) ||
		!pat.hasASBxs(asbxAt{pc: 1, op: OP_LOADINT, a: 5, sbx: 1}) ||
		!pat.hasAs(
			aAt{pc: 4, op: OP_LOADINT, a: 8},
			aAt{pc: 6, op: OP_LOADINT, a: 9},
			aAt{pc: 12, op: OP_LOADINT, a: 8},
			aAt{pc: 15, op: OP_GETGLOBAL, a: 5},
		) ||
		!pat.hasACs(acAt{pc: 3, op: OP_SETFIELD, a: 0, c: 3}) ||
		!pat.hasABCs(
			abcAt{pc: 2, op: OP_ADD, a: 3, b: 4, c: 5},
			abcAt{pc: 5, op: OP_MUL, a: 7, b: 1, c: 8},
			abcAt{pc: 7, op: OP_MUL, a: 8, b: 2, c: 9},
			abcAt{pc: 8, op: OP_ADD, a: 6, b: 7, c: 8},
			abcAt{pc: 10, op: OP_ADD, a: 5, b: 6, c: 7},
			abcAt{pc: 13, op: OP_MUL, a: 6, b: 7, c: 8},
			abcAt{pc: 14, op: OP_ADD, a: 4, b: 5, c: 6},
			abcAt{pc: 16, op: OP_MOD, a: 3, b: 4, c: 5},
		) {
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
