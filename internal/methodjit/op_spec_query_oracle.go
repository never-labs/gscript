package methodjit

func opOracleSupport(op Op) OpOracleSupport {
	spec, ok := op.Spec()
	if !ok {
		return OpOracleUnsupported
	}
	return spec.OracleSupport
}

func opOracleUnsupportedReason(op Op) string {
	spec, ok := op.Spec()
	if !ok {
		return "missing-op-spec"
	}
	return spec.OracleUnsupportedReason
}
