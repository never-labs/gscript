package methodjit

var opOracleSupportPolicies = [...]OpOracleSupport{
	OpComplexEscapeInSet:    OpOracleUnsupported,
	OpComplexEscapeRowCount: OpOracleUnsupported,
	OpGuardFieldCalleeProto: OpOracleUnsupported,
	OpCallFloor:             OpOracleUnsupported,
	OpFieldCallFloor:        OpOracleUnsupported,
	OpResume:                OpOracleUnsupported,
	OpYield:                 OpOracleUnsupported,
	OpSelf:                  OpOracleUnsupported,
	OpForPrep:               OpOracleUnsupported,
	OpForLoop:               OpOracleUnsupported,
	OpTForCall:              OpOracleUnsupported,
	OpTForLoop:              OpOracleUnsupported,
	OpVararg:                OpOracleUnsupported,
	OpTestSet:               OpOracleUnsupported,
	OpGo:                    OpOracleUnsupported,
	OpMakeChan:              OpOracleUnsupported,
	OpSend:                  OpOracleUnsupported,
	OpRecv:                  OpOracleUnsupported,
	OpPhi:                   OpOraclePseudo,
}
