package methodjit

func rewriteStringNativeInstr(instr *Instr, op Op, typ Type, args []*Value, aux, aux2 int64) {
	rewriteInstr(instr, op, typ, args, aux, aux2)
}

func eraseStringNativeInstr(instr *Instr) {
	rewriteInstrToNop(instr)
}
