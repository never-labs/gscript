package methodjit

import "math"

func rewriteInstr(instr *Instr, op Op, typ Type, args []*Value, aux, aux2 int64) {
	instr.Op = op
	instr.Type = typ
	instr.Args = append([]*Value(nil), args...)
	instr.Aux = aux
	instr.Aux2 = aux2
}

func rewriteInstrToNop(instr *Instr) {
	rewriteInstr(instr, OpNop, TypeUnknown, nil, 0, 0)
}

func rewriteInstrToConstInt(instr *Instr, val int64) {
	rewriteInstr(instr, OpConstInt, TypeInt, nil, val, 0)
}

func rewriteInstrToConstFloat(instr *Instr, val float64) {
	rewriteInstr(instr, OpConstFloat, TypeFloat, nil, int64(math.Float64bits(val)), 0)
}

func rewriteInstrToConstFloatBits(instr *Instr, bits uint64) {
	rewriteInstr(instr, OpConstFloat, TypeFloat, nil, int64(bits), 0)
}

func rewriteInstrToConstNil(instr *Instr) {
	rewriteInstr(instr, OpConstNil, TypeNil, nil, 0, 0)
}

func rewriteInstrToBranch(instr *Instr, cond *Value, trueBlockID, falseBlockID int) {
	rewriteInstr(instr, OpBranch, TypeUnknown, []*Value{cond}, int64(trueBlockID), int64(falseBlockID))
}
