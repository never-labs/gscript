//go:build darwin && arm64

package methodjit

func (ec *emitContext) emitSpecializationInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpComplexEscapeInSet:
		ec.emitComplexEscapeInSet(instr)
	case OpComplexEscapeRowCount:
		ec.emitComplexEscapeRowCount(instr)
	case OpRecordArrayLoopSpecialization:
		ec.emitRecordArrayLoopSpecialization(instr)
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitUpvalueInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpGetUpval:
		if len(instr.Args) > 0 {
			ec.emitInlinedGetUpval(instr)
		} else {
			ec.emitOpExit(instr)
		}
		ec.clearTableArrayBoundedKeys()
	case OpSetUpval:
		if len(instr.Args) > 1 {
			ec.emitInlinedSetUpval(instr)
		} else {
			ec.emitOpExit(instr)
		}
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitConversionInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpNumToFloat:
		ec.emitNumToFloat(instr)
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitLoopInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpForPrep, OpForLoop, OpTForCall, OpTForLoop:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitClosureInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpClosure, OpClose:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitVarargInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpVararg:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitConcurrencyInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpGo, OpMakeChan, OpSend, OpRecv:
		ec.emitOpExit(instr)
		ec.clearTableArrayBoundedKeys()
	default:
		return false
	}
	return true
}

func (ec *emitContext) emitSpecialInstr(instr *Instr, _ *Block) bool {
	switch instr.Op {
	case OpNop:
		// nothing
	default:
		return false
	}
	return true
}
