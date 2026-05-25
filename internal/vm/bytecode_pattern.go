package vm

type bytecodePattern struct {
	code []uint32
}

type numericForLoopShape struct {
	forPrepPC int
	bodyPC    int
	loopPC    int
}

type opcodeAt struct {
	pc int
	op Opcode
}

type abcAt struct {
	pc      int
	op      Opcode
	a, b, c int
}

type aAt struct {
	pc int
	op Opcode
	a  int
}

type abAt struct {
	pc   int
	op   Opcode
	a, b int
}

type acAt struct {
	pc   int
	op   Opcode
	a, c int
}

type asbxAt struct {
	pc     int
	op     Opcode
	a, sbx int
}

type sbxAt struct {
	pc  int
	op  Opcode
	sbx int
}

type bxAt struct {
	pc int
	op Opcode
	bx int
}

func newBytecodePattern(code []uint32) bytecodePattern {
	return bytecodePattern{code: code}
}

func (p bytecodePattern) inst(pc int) (uint32, bool) {
	if pc < 0 || pc >= len(p.code) {
		return 0, false
	}
	return p.code[pc], true
}

func (p bytecodePattern) op(pc int, op Opcode) (uint32, bool) {
	inst, ok := p.inst(pc)
	return inst, ok && DecodeOp(inst) == op
}

func (p bytecodePattern) hasOps(ops ...opcodeAt) bool {
	for _, want := range ops {
		if _, ok := p.op(want.pc, want.op); !ok {
			return false
		}
	}
	return true
}

func (p bytecodePattern) abc(pc int, op Opcode, a, b, c int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodeA(inst) == a && DecodeB(inst) == b && DecodeC(inst) == c
}

func (p bytecodePattern) hasABCs(ops ...abcAt) bool {
	for _, want := range ops {
		if !p.abc(want.pc, want.op, want.a, want.b, want.c) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) a(pc int, op Opcode, a int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodeA(inst) == a
}

func (p bytecodePattern) hasAs(ops ...aAt) bool {
	for _, want := range ops {
		if !p.a(want.pc, want.op, want.a) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) ab(pc int, op Opcode, a, b int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodeA(inst) == a && DecodeB(inst) == b
}

func (p bytecodePattern) hasABs(ops ...abAt) bool {
	for _, want := range ops {
		if !p.ab(want.pc, want.op, want.a, want.b) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) ac(pc int, op Opcode, a, c int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodeA(inst) == a && DecodeC(inst) == c
}

func (p bytecodePattern) hasACs(ops ...acAt) bool {
	for _, want := range ops {
		if !p.ac(want.pc, want.op, want.a, want.c) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) asbx(pc int, op Opcode, a, sbx int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodeA(inst) == a && DecodesBx(inst) == sbx
}

func (p bytecodePattern) hasASBxs(ops ...asbxAt) bool {
	for _, want := range ops {
		if !p.asbx(want.pc, want.op, want.a, want.sbx) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) sbx(pc int, op Opcode, sbx int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodesBx(inst) == sbx
}

func (p bytecodePattern) hasSBxs(ops ...sbxAt) bool {
	for _, want := range ops {
		if !p.sbx(want.pc, want.op, want.sbx) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) bx(pc int, op Opcode, bx int) bool {
	inst, ok := p.op(pc, op)
	return ok && DecodeBx(inst) == bx
}

func (p bytecodePattern) hasBxs(ops ...bxAt) bool {
	for _, want := range ops {
		if !p.bx(want.pc, want.op, want.bx) {
			return false
		}
	}
	return true
}

func (p bytecodePattern) move(pc int, dst, src int) bool {
	return p.abc(pc, OP_MOVE, dst, src, 0)
}

func (p bytecodePattern) loadInt(pc int, dst int, value int) bool {
	return p.asbx(pc, OP_LOADINT, dst, value)
}

func (p bytecodePattern) loadBool(pc int, dst int, value bool) bool {
	b := 0
	if value {
		b = 1
	}
	return p.abc(pc, OP_LOADBOOL, dst, b, 0)
}

func (p bytecodePattern) returnFixed(pc int, src, count int) bool {
	return p.abc(pc, OP_RETURN, src, count, 0)
}

func (p bytecodePattern) jumpTarget(pc int) (int, bool) {
	inst, ok := p.op(pc, OP_JMP)
	if !ok {
		return 0, false
	}
	return pc + 1 + DecodesBx(inst), true
}

func (p bytecodePattern) jumpTo(pc int, target int) bool {
	got, ok := p.jumpTarget(pc)
	return ok && got == target
}

func (p bytecodePattern) numericForLoop(forprepPC, a int) (bodyPC, loopPC int, ok bool) {
	prep, ok := p.op(forprepPC, OP_FORPREP)
	if !ok || DecodeA(prep) != a {
		return 0, 0, false
	}
	bodyPC = forprepPC + 1
	loopPC = bodyPC + DecodesBx(prep)
	loop, ok := p.op(loopPC, OP_FORLOOP)
	if !ok || DecodeA(loop) != a || loopPC+1+DecodesBx(loop) != bodyPC {
		return 0, 0, false
	}
	return bodyPC, loopPC, true
}

func (p bytecodePattern) findNumericForLoopWithBodyLen(startPC int, bodyLen int) (numericForLoopShape, bool) {
	for pc := startPC; pc < len(p.code); pc++ {
		inst, ok := p.op(pc, OP_FORPREP)
		if !ok {
			continue
		}
		bodyPC, loopPC, ok := p.numericForLoop(pc, DecodeA(inst))
		if ok && loopPC-bodyPC == bodyLen {
			return numericForLoopShape{forPrepPC: pc, bodyPC: bodyPC, loopPC: loopPC}, true
		}
	}
	return numericForLoopShape{}, false
}
