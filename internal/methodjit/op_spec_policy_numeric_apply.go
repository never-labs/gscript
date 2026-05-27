package methodjit

func applyOpSpecNumericPolicies(op Op, spec *OpSpec) {
	applyOpSpecNumericInlinePolicies(op, spec)
	applyOpSpecNumericDeoptPolicies(op, spec)
	applyOpSpecNumericSpecializationPolicies(op, spec)
	applyOpSpecNumericRecurrencePolicies(op, spec)
}

func applyOpSpecNumericInlinePolicies(op Op, spec *OpSpec) {
	if int(op) < len(opPureNumericInlinePolicies) {
		spec.PureNumericInline = opPureNumericInlinePolicies[op]
	}
	if int(op) < len(opNativeEffectLoopInlinePolicies) {
		spec.NativeEffectLoopInline = opNativeEffectLoopInlinePolicies[op]
	}
}

func applyOpSpecNumericDeoptPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opDirectDeoptWithoutFullFlushPolicies) {
		spec.DirectDeoptWithoutFullFlush = opDirectDeoptWithoutFullFlushPolicies[op]
	}
}

func applyOpSpecNumericSpecializationPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opGenericSpecializablePolicies) {
		spec.GenericSpecializable = opGenericSpecializablePolicies[op]
	}
	if int(op) < len(opTypeSpecializationPolicies) && opTypeSpecializationPolicies[op].Set {
		policy := opTypeSpecializationPolicies[op]
		spec.TypeSpecializeIntOp = policy.IntOp
		spec.TypeSpecializeFloatOp = policy.FloatOp
		spec.TypeSpecializeStringOp = policy.StringOp
	}
	if int(op) < len(opNumToFloatInsertCandidatePolicies) {
		spec.NumToFloatInsertCandidate = opNumToFloatInsertCandidatePolicies[op]
	}
}

func applyOpSpecNumericRecurrencePolicies(op Op, spec *OpSpec) {
	if int(op) < len(opIntRecurrencePolicies) {
		spec.IntRecurrence = opIntRecurrencePolicies[op]
	}
	if int(op) < len(opNumericOperandPolicies) {
		spec.NumericOperand = opNumericOperandPolicies[op]
	}
}
