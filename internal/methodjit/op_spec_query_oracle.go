package methodjit

func opOracleSupport(op Op) OpOracleSupport {
	spec, ok := op.Spec()
	if !ok {
		return OpOracleUnsupported
	}
	return spec.OracleSupport
}
