package methodjit

func fixedShapeArrayElementWriteRole(op Op) OpFixedShapeArrayElementWriteRole {
	spec, ok := op.Spec()
	if !ok {
		return OpFixedShapeArrayElementWriteNone
	}
	return spec.FixedShapeArrayElementWriteRole
}

func fixedShapeArrayElementReadRole(op Op) OpFixedShapeArrayElementReadRole {
	spec, ok := op.Spec()
	if !ok {
		return OpFixedShapeArrayElementReadNone
	}
	return spec.FixedShapeArrayElementReadRole
}

func fixedShapeReturnArrayElementRole(op Op) OpFixedShapeReturnArrayElementRole {
	spec, ok := op.Spec()
	if !ok {
		return OpFixedShapeReturnArrayElementNone
	}
	return spec.FixedShapeReturnArrayElementRole
}

func localStringArrayTableUseRole(op Op) OpLocalStringArrayTableUseRole {
	spec, ok := op.Spec()
	if !ok {
		return OpLocalStringArrayTableUseNone
	}
	return spec.LocalStringArrayTableUseRole
}

func localStringArrayTableArgIndex(op Op) (int, bool) {
	spec, ok := op.Spec()
	return spec.LocalStringArrayTableArgIndex, ok && spec.LocalStringArrayTableArgIndex >= 0
}

func readonlyTableParamUseRole(op Op) OpReadonlyTableParamUseRole {
	spec, ok := op.Spec()
	if !ok {
		return OpReadonlyTableParamUseNone
	}
	return spec.ReadonlyTableParamUseRole
}
