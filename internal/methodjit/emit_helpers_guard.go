package methodjit

// emitGuardDeoptExit emits the canonical guard deopt-exit tail shared by many
// table array/field guard sites:
//
//	asm.B(doneLabel)
//	asm.Label(deoptLabel)
//	ec.emitDeopt(instr)        // or ec.emitPreciseDeopt(instr) when precise
//	asm.Label(doneLabel)
//
// It is behavior-preserving: the emitted instruction sequence is identical to
// the inline form regardless of whether the call site used a local `asm`
// variable or `ec.asm` directly, since both alias the same assembler.
func (ec *emitContext) emitGuardDeoptExit(instr *Instr, deoptLabel, doneLabel string, precise bool) {
	ec.asm.B(doneLabel)
	ec.asm.Label(deoptLabel)
	if precise {
		ec.emitPreciseDeopt(instr)
	} else {
		ec.emitDeopt(instr)
	}
	ec.asm.Label(doneLabel)
}
