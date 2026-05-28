package methodjit

func opIsBoxedOrFallback(op, boxed Op) bool {
	if op == boxed {
		return true
	}
	spec, ok := op.Spec()
	return ok && spec.BoxedFallbackOp == boxed
}

func exactIntNarrowOp(op Op) (Op, bool) {
	spec, ok := op.Spec()
	return spec.ExactIntNarrowOp, ok && spec.ExactIntNarrowOp < OpMax
}

func opNarrowsExactlyTo(op, narrowed Op) bool {
	out, ok := exactIntNarrowOp(op)
	return ok && out == narrowed
}

func guardProvenByProducer(v *Value, guardType Type) bool {
	if v == nil || v.Def == nil || guardType == TypeUnknown {
		return false
	}
	spec, ok := v.Def.Op.Spec()
	return ok && spec.GuardProvenResultType == guardType
}

func fixedResultType(op Op) (Type, bool) {
	spec, ok := op.Spec()
	return spec.FixedResultType, ok && spec.FixedResultType != TypeUnknown
}

func typeSpecializedOp(op Op, lt, rt Type) (Op, Type, bool) {
	spec, ok := op.Spec()
	if !ok {
		return OpMax, TypeUnknown, false
	}
	if lt == TypeInt && rt == TypeInt && spec.TypeSpecializeIntOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeIntOp)
	}
	if isNumericType(lt) && isNumericType(rt) && (lt == TypeFloat || rt == TypeFloat) && spec.TypeSpecializeFloatOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeFloatOp)
	}
	if lt == TypeString && rt == TypeString && spec.TypeSpecializeStringOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeStringOp)
	}
	return OpMax, TypeUnknown, false
}

func unaryTypeSpecializedOp(op Op, arg Type) (Op, Type, bool) {
	spec, ok := op.Spec()
	if !ok {
		return OpMax, TypeUnknown, false
	}
	if arg == TypeInt && spec.TypeSpecializeIntOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeIntOp)
	}
	if arg == TypeFloat && spec.TypeSpecializeFloatOp < OpMax {
		return opSpecializedTarget(spec.TypeSpecializeFloatOp)
	}
	return OpMax, TypeUnknown, false
}

func opSpecializedTarget(op Op) (Op, Type, bool) {
	typ, ok := fixedResultType(op)
	if !ok {
		return OpMax, TypeUnknown, false
	}
	return op, typ, true
}
