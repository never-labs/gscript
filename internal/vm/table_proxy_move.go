package vm

import rt "github.com/never-labs/leia/internal/runtime"

func (vm *VM) tryForwardingProxyTableMove(src, dst rt.Value, first, last, target int64) (bool, rt.Value, error) {
	if last < first {
		return true, dst, nil
	}
	srcTable, srcCounter, ok := vm.forwardingIndexSource(src)
	if !ok {
		if !src.IsTable() {
			return false, rt.NilValue(), nil
		}
		srcTable = src.Table()
	}
	dstTable, dstCounter, ok := vm.forwardingNewIndexTarget(dst)
	if !ok {
		if !dst.IsTable() {
			return false, rt.NilValue(), nil
		}
		dstTable = dst.Table()
	}
	if srcTable == nil || dstTable == nil {
		return false, rt.NilValue(), nil
	}
	if !dstTable.TryPlainArrayMove(srcTable, first, last, target) {
		return false, rt.NilValue(), nil
	}
	count := last - first + 1
	if srcCounter != nil {
		incrementUpvalueCounter(srcCounter, count)
	}
	if dstCounter != nil {
		incrementUpvalueCounter(dstCounter, count)
	}
	return true, dst, nil
}

func incrementUpvalueCounter(up *Upvalue, delta int64) {
	if up == nil || delta == 0 {
		return
	}
	v := up.Get()
	if v.IsInt() {
		up.Set(rt.IntValue(v.Int() + delta))
	}
}

func (vm *VM) forwardingIndexSource(v rt.Value) (*rt.Table, *Upvalue, bool) {
	proxy, ok := forwardingProxyMetamethod(v, "__index")
	if !ok {
		return nil, nil, false
	}
	tableIdx, counterIdx, ok := matchForwardingIndexProto(proxy.Proto)
	if !ok || tableIdx < 0 || tableIdx >= len(proxy.Upvalues) {
		return nil, nil, false
	}
	tableVal := proxy.Upvalues[tableIdx].Get()
	if !tableVal.IsTable() {
		return nil, nil, false
	}
	var counter *Upvalue
	if counterIdx >= 0 && counterIdx < len(proxy.Upvalues) {
		counter = proxy.Upvalues[counterIdx]
	}
	return tableVal.Table(), counter, true
}

func (vm *VM) forwardingNewIndexTarget(v rt.Value) (*rt.Table, *Upvalue, bool) {
	proxy, ok := forwardingProxyMetamethod(v, "__newindex")
	if !ok {
		return nil, nil, false
	}
	tableIdx, counterIdx, ok := matchForwardingNewIndexProto(proxy.Proto)
	if !ok || tableIdx < 0 || tableIdx >= len(proxy.Upvalues) {
		return nil, nil, false
	}
	tableVal := proxy.Upvalues[tableIdx].Get()
	if !tableVal.IsTable() {
		return nil, nil, false
	}
	var counter *Upvalue
	if counterIdx >= 0 && counterIdx < len(proxy.Upvalues) {
		counter = proxy.Upvalues[counterIdx]
	}
	return tableVal.Table(), counter, true
}

func forwardingProxyMetamethod(v rt.Value, name string) (*Closure, bool) {
	if !v.IsTable() {
		return nil, false
	}
	mt := v.Table().GetMetatable()
	if mt == nil {
		return nil, false
	}
	mm := mt.RawGetString(name)
	if !mm.IsFunction() {
		return nil, false
	}
	return closureFromValue(mm)
}

func matchForwardingIndexProto(proto *FuncProto) (tableUpvalue, counterUpvalue int, ok bool) {
	if proto == nil || proto.NumParams != 2 || proto.IsVarArg {
		return -1, -1, false
	}
	counterUpvalue, offset, ok := matchOptionalCounterIncrementPrefix(proto.Code)
	if !ok || len(proto.Code) != offset+4 {
		return -1, -1, false
	}
	if DecodeOp(proto.Code[offset]) != OP_GETUPVAL || DecodeOp(proto.Code[offset+1]) != OP_MOVE ||
		DecodeOp(proto.Code[offset+2]) != OP_GETTABLE || DecodeOp(proto.Code[offset+3]) != OP_RETURN {
		return -1, -1, false
	}
	tableReg := DecodeA(proto.Code[offset])
	keyReg := DecodeA(proto.Code[offset+1])
	if DecodeB(proto.Code[offset+1]) != 1 {
		return -1, -1, false
	}
	get := proto.Code[offset+2]
	if DecodeB(get) != tableReg || DecodeC(get) != keyReg {
		return -1, -1, false
	}
	ret := proto.Code[offset+3]
	if DecodeA(ret) != DecodeA(get) || DecodeB(ret) != 2 {
		return -1, -1, false
	}
	return DecodeB(proto.Code[offset]), counterUpvalue, true
}

func matchForwardingNewIndexProto(proto *FuncProto) (tableUpvalue, counterUpvalue int, ok bool) {
	if proto == nil || proto.NumParams != 3 || proto.IsVarArg {
		return -1, -1, false
	}
	counterUpvalue, offset, ok := matchOptionalCounterIncrementPrefix(proto.Code)
	if !ok || len(proto.Code) != offset+5 {
		return -1, -1, false
	}
	if DecodeOp(proto.Code[offset]) != OP_MOVE || DecodeOp(proto.Code[offset+1]) != OP_GETUPVAL ||
		DecodeOp(proto.Code[offset+2]) != OP_MOVE || DecodeOp(proto.Code[offset+3]) != OP_SETTABLE ||
		DecodeOp(proto.Code[offset+4]) != OP_RETURN {
		return -1, -1, false
	}
	valReg := DecodeA(proto.Code[offset])
	if DecodeB(proto.Code[offset]) != 2 {
		return -1, -1, false
	}
	tableReg := DecodeA(proto.Code[offset+1])
	keyReg := DecodeA(proto.Code[offset+2])
	if DecodeB(proto.Code[offset+2]) != 1 {
		return -1, -1, false
	}
	set := proto.Code[offset+3]
	if DecodeA(set) != tableReg || DecodeB(set) != keyReg || DecodeC(set) != valReg {
		return -1, -1, false
	}
	return DecodeB(proto.Code[offset+1]), counterUpvalue, true
}

func matchOptionalCounterIncrementPrefix(code []uint32) (upvalue, next int, ok bool) {
	if len(code) >= 4 {
		if c, ok := matchCounterIncrementPrefix(code[:4]); ok {
			return c, 4, true
		}
	}
	return -1, 0, true
}

func matchCounterIncrementPrefix(code []uint32) (int, bool) {
	if len(code) != 4 || DecodeOp(code[0]) != OP_GETUPVAL || DecodeOp(code[1]) != OP_LOADINT ||
		DecodeOp(code[2]) != OP_ADD || DecodeOp(code[3]) != OP_SETUPVAL {
		return -1, false
	}
	counterReg := DecodeA(code[0])
	oneReg := DecodeA(code[1])
	sumReg := DecodeA(code[2])
	upvalue := DecodeB(code[0])
	if DecodesBx(code[1]) != 1 || DecodeB(code[2]) != counterReg || DecodeC(code[2]) != oneReg ||
		DecodeA(code[3]) != sumReg || DecodeB(code[3]) != upvalue {
		return -1, false
	}
	return upvalue, true
}
