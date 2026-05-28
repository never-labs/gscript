package methodjit

func applyOpSpecOraclePolicies(op Op, spec *OpSpec) {
	applyOpSpecOracleSupportPolicies(op, spec)
}

func applyOpSpecOracleSupportPolicies(op Op, spec *OpSpec) {
	if int(op) < len(opOracleSupportPolicies) && opOracleSupportPolicies[op] != 0 {
		spec.OracleSupport = opOracleSupportPolicies[op]
	}
}
