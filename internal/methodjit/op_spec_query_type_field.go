package methodjit

func opIsFieldRead(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldRead
}

func opIsFieldSlotLoad(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldSlotLoad
}

func opIsFieldWrite(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.FieldWrite
}

func fieldSvalsLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.FieldSvalsLoweredOp, ok && spec.FieldSvalsLoweredOp != OpMax
}

func fieldLenLoweredOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.FieldLenLoweredOp, ok && spec.FieldLenLoweredOp != OpMax
}

func opIsLiteralConst(op Op) bool {
	spec, ok := op.Spec()
	return ok && spec.LiteralConst
}

func loadElimConstCSE(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimConstCSE
}

func loadElimPureCSE(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimPureCSE
}

func loadElimShapeFactKiller(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimShapeFactKiller
}

func loadElimFieldFactWideKiller(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldFactWideKiller
}

func loadElimDynamicTableCacheMutation(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimDynamicTableCacheMutation
}

func loadElimTypedArrayFactMutation(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimTypedArrayFactMutation
}

func loadElimTableCacheKeyArgIndex(op Op) (int, bool) {
	spec, ok := op.Spec()
	return spec.LoadElimTableCacheKeyArgIndex, ok && spec.LoadElimTableCacheKeyArgIndex >= 0
}

func loadElimTableCacheValueArgIndex(op Op) (int, bool) {
	spec, ok := op.Spec()
	return spec.LoadElimTableCacheValueArgIndex, ok && spec.LoadElimTableCacheValueArgIndex >= 0
}

func loadElimFactBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.LoadElimFactBarrier
}

func fieldLenFoldBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldLenFoldBarrier
}

func fieldCallPolyLenFusionBarrier(instr *Instr) bool {
	if instr == nil {
		return false
	}
	spec, ok := instr.Op.Spec()
	return ok && spec.FieldCallPolyLenFusionBarrier
}
