package methodjit

func instructionHasNoSSAResult(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.NoSSAResult
}

func hasSideEffect(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.KeepUnused
}

func needsFloatReg(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	if ok && spec.FloatRegResultBlocked {
		return false
	}
	if instr.Type == TypeFloat {
		return true
	}
	return ok && spec.FloatRegResult
}

func isRawIntOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawIntResult
}

func isRawTablePtrOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawTablePtrResult
}

func isRawTablePtrValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	if isRawTablePtrOp(instr.Op) {
		return true
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.TableResultRawTablePtr && instr.Type == TypeTable
}

func isRawDataPtrOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawDataPtrResult
}

func isRawFloatOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawFloatResult
}

func isMatrixNativeOp(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.MatrixNative
}

func matrixLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.MatrixLoweredOp, ok && spec.MatrixLoweredOp != OpMax
}

func matrixRowLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.MatrixRowLoweredOp, ok && spec.MatrixRowLoweredOp != OpMax
}

func matrixRowConstLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.MatrixRowConstLoweredOp, ok && spec.MatrixRowConstLoweredOp != OpMax
}

func tableArrayLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.TableArrayLoweredOp, ok && spec.TableArrayLoweredOp != OpMax
}

func opIsRawCarryClobber(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.RawCarryClobber
}

func isRawIntCarryValue(instr *Instr) bool {
	if instr == nil {
		return false
	}
	if instr.Type != TypeInt {
		return false
	}
	if isRawIntOp(instr.Op) {
		return true
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.RawIntCarryValue
}

func opIsGlobalConstUnsafe(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.GlobalConstUnsafe
}

func opMayCallOrRunConcurrently(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.MayCallOrRunConcurrently()
}

func crossBlockFieldSvalsGlobalBarrier(instr *Instr) bool {
	if instr == nil {
		return true
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldSvalsCrossBlockBarrier
}

func opIsFieldSvalsGlobalBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldSvalsGlobalBarrier
}

func fieldSvalsFirstArgMutationBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldSvalsFirstArgMutationBarrier
}

func valueProvenNonNil(v *Value) bool {
	if v == nil || v.Def == nil {
		return false
	}
	spec, ok := v.Def.Op.Spec()
	if ok && spec.ProvesNonNilResult {
		return true
	}
	return v.Def.Type == TypeInt || v.Def.Type == TypeFloat || v.Def.Type == TypeBool || v.Def.Type == TypeString || v.Def.Type == TypeTable
}

func isModuloReducibleCallFloor(instr *Instr) bool {
	if instr == nil || instr.Type != TypeInt {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.ModuloReducibleCallFloor
}

func callFloorProjectionOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.CallFloorProjectionOp, ok && spec.CallFloorProjectionOp != OpMax
}

func fieldCallFloorProjectionOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.FieldCallFloorProjectionOp, ok && spec.FieldCallFloorProjectionOp != OpMax
}

func isCallResultRangeGuardCandidate(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.CallResultRangeGuardCandidate
}

func opIsCallFloorSpecStableCallee(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.CallFloorSpecStableCallee
}

func opIsCallFloorSpecFieldShape(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.CallFloorSpecFieldShape
}

func opIsSpeculativeIntUseCandidate(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.SpeculativeIntUseCandidate
}
