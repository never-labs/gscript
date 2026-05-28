package methodjit

func rewriteStringNativeInstr(instr *Instr, op Op, typ Type, args []*Value, aux, aux2 int64) {
	instr.Op = op
	instr.Type = typ
	instr.Args = append([]*Value(nil), args...)
	instr.Aux = aux
	instr.Aux2 = aux2
}

func eraseStringNativeInstr(instr *Instr) {
	rewriteStringNativeInstr(instr, OpNop, TypeUnknown, nil, 0, 0)
}
