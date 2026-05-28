package methodjit

func buildExpandedOpSpecs() [OpMax]OpSpec {
	var out [OpMax]OpSpec
	for op := Op(0); op < OpMax; op++ {
		if spec, ok := buildOpSpec(op); ok {
			out[op] = spec
		}
	}
	return out
}

func (op Op) Spec() (OpSpec, bool) {
	if int(op) < len(expandedOpSpecs) && expandedOpSpecs[op].Name != "" {
		return expandedOpSpecs[op], true
	}
	return OpSpec{}, false
}

func buildOpSpec(op Op) (OpSpec, bool) {
	if int(op) >= len(opSpecs) || opSpecs[op].Name == "" {
		return OpSpec{}, false
	}
	spec := opSpecs[op]
	applyOpSpecBackendPolicies(op, &spec)
	applyOpSpecFieldPolicies(op, &spec)
	applyOpSpecValuePolicies(op, &spec)
	applyOpSpecOraclePolicies(op, &spec)
	applyOpSpecLICMPolicies(op, &spec)
	applyOpSpecNumericPolicies(op, &spec)
	applyOpSpecFieldBarrierPolicies(op, &spec)
	applyOpSpecRangePolicies(op, &spec)
	applyOpSpecStringUnrollPolicies(op, &spec)
	applyOpSpecTableCallPolicies(op, &spec)
	applyOpSpecTypePolicies(op, &spec)
	return spec, true
}

func OpByName(name string) (Op, bool) {
	op, ok := opNameLookup[name]
	return op, ok
}

func (spec OpSpec) MayCallOrRunConcurrently() bool {
	return spec.SideEffect == OpSideEffectCall || spec.SideEffect == OpSideEffectConcurrency
}

func OpsByEmitterFamily(family OpEmitterFamily) []Op {
	var ops []Op
	for op := Op(0); op < OpMax; op++ {
		spec := expandedOpSpecs[op]
		if spec.Name == "" || spec.EmitterFamily != family {
			continue
		}
		ops = append(ops, op)
	}
	return ops
}
