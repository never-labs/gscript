package methodjit

var opOracleSupportPolicies = [...]OpOracleSupport{
	OpComplexEscapeInSet:    OpOracleUnsupported,
	OpComplexEscapeRowCount: OpOracleUnsupported,
	OpGuardFieldCalleeProto: OpOracleUnsupported,
	OpCallFloor:             OpOracleUnsupported,
	OpFieldCallFloor:        OpOracleUnsupported,
	OpResume:                OpOracleUnsupported,
	OpYield:                 OpOracleUnsupported,
	OpGo:                    OpOracleUnsupported,
	OpMakeChan:              OpOracleUnsupported,
	OpSend:                  OpOracleUnsupported,
	OpRecv:                  OpOracleUnsupported,
	OpVectorGather:          OpOracleUnsupported,
	OpPhi:                   OpOraclePseudo,
}

var opOracleUnsupportedReasonPolicies = [...]string{
	OpComplexEscapeInSet:    "runtime-specialization",
	OpComplexEscapeRowCount: "runtime-specialization",
	OpGuardFieldCalleeProto: "callee-shape-guard",
	OpCallFloor:             "call-fold-specialization",
	OpFieldCallFloor:        "call-fold-specialization",
	OpResume:                "coroutine",
	OpYield:                 "coroutine",
	OpGo:                    "concurrency",
	OpMakeChan:              "concurrency",
	OpSend:                  "concurrency",
	OpRecv:                  "concurrency",
	OpVectorGather:          "q-vector-backend-pending",
}
