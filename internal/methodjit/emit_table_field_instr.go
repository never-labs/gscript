//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitTableInstr(instr *Instr) bool {
	switch instr.Op {
	case OpNewTable:
		ec.emitNewTableExit(instr)
	case OpNewFixedTable:
		ec.emitNewFixedTable(instr)
	case OpGetTable:
		ec.emitGetTableNative(instr)
	case OpSetTable:
		ec.emitSetTableNative(instr)
		ec.clearTableArrayBoundedKeys()
		// Dynamic key writes can add new string keys, changing table shape.
		ec.shapeVerified = make(map[int]uint32)
	case OpTableArrayHeader:
		ec.emitTableArrayHeader(instr)
	case OpTableArrayLen:
		ec.emitTableArrayLen(instr)
	case OpTableArrayData:
		ec.emitTableArrayData(instr)
	case OpTableArrayLoad:
		ec.emitTableArrayLoad(instr)
	case OpTableShapeID:
		ec.emitTableShapeID(instr)
	case OpTableArrayStore:
		ec.emitTableArrayStore(instr)
	case OpTableArraySwap:
		ec.emitTableArraySwap(instr)
	case OpTableArraySwapPairs:
		ec.emitTableArraySwapPairs(instr)
		ec.clearTableArrayBoundedKeys()
	case OpTableBoolArrayFill:
		ec.emitTableBoolArrayFill(instr)
		ec.clearTableArrayBoundedKeys()
	case OpTableBoolArrayCount:
		ec.emitTableBoolArrayCount(instr)
	case OpTableIntArrayReversePrefix:
		ec.emitTableIntArrayReversePrefix(instr)
		ec.clearTableArrayBoundedKeys()
	case OpTableIntArrayCopyPrefix:
		ec.emitTableIntArrayCopyPrefix(instr)
		ec.clearTableArrayBoundedKeys()
	case OpTableArrayNestedLoad:
		ec.emitTableArrayNestedLoad(instr)
	case OpFrameLen, OpFrameColumn, OpFrameProject, OpFrameFilter, OpFrameGather, OpFrameSlice, OpFrameOrder, OpVectorGather, OpVectorCompare:
		ec.emitOpExit(instr)
	case OpSetList:
		ec.emitSetListExit(instr)
	case OpAppend:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitFieldInstr(instr *Instr) bool {
	switch instr.Op {
	case OpGetField:
		ec.emitGetField(instr)
	case OpGetFieldNumToFloat:
		ec.emitGetFieldNumToFloat(instr)
	case OpFieldPolyLen:
		ec.emitFieldPolyLen(instr)
	case OpFieldSvals:
		ec.emitFieldSvals(instr)
	case OpFieldLoad:
		ec.emitFieldLoad(instr)
	case OpFieldLoadNumToFloat:
		ec.emitFieldLoadNumToFloat(instr)
	case OpFieldStore:
		ec.emitFieldStore(instr)
	case OpSetField:
		ec.emitSetField(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}
