//go:build darwin && arm64

package methodjit

type emitterFamilyDelegate struct {
	filename string
	funcName string
	family   OpEmitterFamily
}

func emitterFamilyDelegateRegistry() []emitterFamilyDelegate {
	return []emitterFamilyDelegate{
		{filename: "emit_const.go", funcName: "emitConstInstr", family: OpEmitterConst},
		{filename: "emit_slot.go", funcName: "emitSlotInstr", family: OpEmitterSlot},
		{filename: "emit_arithmetic.go", funcName: "emitArithmeticInstr", family: OpEmitterArithmetic},
		{filename: "emit_compare.go", funcName: "emitCompareInstr", family: OpEmitterCompare},
		{filename: "emit_matrix.go", funcName: "emitMatrixInstr", family: OpEmitterMatrix},
		{filename: "emit_string.go", funcName: "emitStringInstr", family: OpEmitterString},
		{filename: "emit_table_field_instr.go", funcName: "emitTableInstr", family: OpEmitterTable},
		{filename: "emit_table_field_instr.go", funcName: "emitFieldInstr", family: OpEmitterField},
		{filename: "emit_guard_instr.go", funcName: "emitGuardInstr", family: OpEmitterGuard},
		{filename: "emit_call_instr.go", funcName: "emitCallInstr", family: OpEmitterCall},
		{filename: "emit_global_instr.go", funcName: "emitGlobalInstr", family: OpEmitterGlobal},
		{filename: "emit_misc_instr.go", funcName: "emitSpecializationInstr", family: OpEmitterSpecialization},
		{filename: "emit_control_instr.go", funcName: "emitControlInstr", family: OpEmitterControl},
		{filename: "emit_misc_instr.go", funcName: "emitUpvalueInstr", family: OpEmitterUpvalue},
		{filename: "emit_misc_instr.go", funcName: "emitConversionInstr", family: OpEmitterConversion},
		{filename: "emit_misc_instr.go", funcName: "emitLoopInstr", family: OpEmitterLoop},
		{filename: "emit_misc_instr.go", funcName: "emitClosureInstr", family: OpEmitterClosure},
		{filename: "emit_misc_instr.go", funcName: "emitVarargInstr", family: OpEmitterVararg},
		{filename: "emit_misc_instr.go", funcName: "emitConcurrencyInstr", family: OpEmitterConcurrency},
		{filename: "emit_phi_instr.go", funcName: "emitPhiInstr", family: OpEmitterPhi},
		{filename: "emit_misc_instr.go", funcName: "emitSpecialInstr", family: OpEmitterSpecial},
	}
}
