package methodjit

// OpSideEffect describes the broad runtime effects an IR op may have.
type OpSideEffect uint8

const (
	OpSideEffectInvalid OpSideEffect = iota
	OpSideEffectNone
	OpSideEffectRead
	OpSideEffectWrite
	OpSideEffectReadWrite
	OpSideEffectAllocate
	OpSideEffectCall
	OpSideEffectControl
	OpSideEffectConcurrency
)

// OpEmitterFamily groups ops by the emitter area that owns their lowering.
type OpEmitterFamily uint8

const (
	OpEmitterInvalid OpEmitterFamily = iota
	OpEmitterConst
	OpEmitterSlot
	OpEmitterArithmetic
	OpEmitterMatrix
	OpEmitterSpecialization
	OpEmitterCompare
	OpEmitterString
	OpEmitterTable
	OpEmitterField
	OpEmitterGlobal
	OpEmitterUpvalue
	OpEmitterConversion
	OpEmitterGuard
	OpEmitterControl
	OpEmitterCall
	OpEmitterLoop
	OpEmitterClosure
	OpEmitterVararg
	OpEmitterConcurrency
	OpEmitterPhi
	OpEmitterSpecial
)

func (f OpEmitterFamily) String() string {
	switch f {
	case OpEmitterConst:
		return "const"
	case OpEmitterSlot:
		return "slot"
	case OpEmitterArithmetic:
		return "arithmetic"
	case OpEmitterMatrix:
		return "matrix"
	case OpEmitterSpecialization:
		return "specialization"
	case OpEmitterCompare:
		return "compare"
	case OpEmitterString:
		return "string"
	case OpEmitterTable:
		return "table"
	case OpEmitterField:
		return "field"
	case OpEmitterGlobal:
		return "global"
	case OpEmitterUpvalue:
		return "upvalue"
	case OpEmitterConversion:
		return "conversion"
	case OpEmitterGuard:
		return "guard"
	case OpEmitterControl:
		return "control"
	case OpEmitterCall:
		return "call"
	case OpEmitterLoop:
		return "loop"
	case OpEmitterClosure:
		return "closure"
	case OpEmitterVararg:
		return "vararg"
	case OpEmitterConcurrency:
		return "concurrency"
	case OpEmitterPhi:
		return "phi"
	case OpEmitterSpecial:
		return "special"
	default:
		return "invalid"
	}
}

// OpArgPolicy summarizes how an op consumes Args/Aux metadata.
type OpArgPolicy uint8

const (
	OpArgInvalid OpArgPolicy = iota
	OpArgNone
	OpArgFixed
	OpArgVariadic
	OpArgAux
	OpArgFixedAux
	OpArgVariadicAux
	OpArgControl
)

func (p OpArgPolicy) String() string {
	switch p {
	case OpArgNone:
		return "none"
	case OpArgFixed:
		return "fixed"
	case OpArgVariadic:
		return "variadic"
	case OpArgAux:
		return "aux"
	case OpArgFixedAux:
		return "fixed+aux"
	case OpArgVariadicAux:
		return "variadic+aux"
	case OpArgControl:
		return "control"
	default:
		return "invalid"
	}
}

// OpBackendPolicy describes backend-local cache and verification effects that
// are easy to miss in emit_dispatch.go.
type OpBackendPolicy uint16

const (
	OpBackendPreservesFieldSvalsCache OpBackendPolicy = 1 << iota
	OpBackendPreservesFieldSvalsCacheForFloatResult
	OpBackendPreservesTableArrayBounds
	OpBackendClearsTableArrayBounds
	OpBackendInvalidatesShape
	OpBackendPreservesScratchFPRCache
	OpBackendPreservesScratchFPRCacheForFloatResult
)

type OpSourceFeedbackPolicy uint8

const (
	OpSourceFeedbackNone     OpSourceFeedbackPolicy = 0
	OpSourceFeedbackGetField OpSourceFeedbackPolicy = 1 << iota
	OpSourceFeedbackSetField
	OpSourceFeedbackGetTable
	OpSourceFeedbackSetTable
	OpSourceFeedbackResultType
)

type OpRangeRefineKind uint8

const (
	OpRangeRefineNone OpRangeRefineKind = iota
	OpRangeRefineLessThan
	OpRangeRefineLessEqual
	OpRangeRefineEqualInt
)

type OpNonNegativeDerivationKind uint8

const (
	OpNonNegativeNone OpNonNegativeDerivationKind = iota
	OpNonNegativeConstIntAux
	OpNonNegativeAlways
	OpNonNegativeGuardRangeMin
	OpNonNegativeAllArgs
	OpNonNegativeBinaryAllArgs
	OpNonNegativeModuloDivisor
	OpNonNegativeExactDivPositiveDivisor
	OpNonNegativeForwardArg
)

type OpOracleSupport uint8

const (
	OpOracleInvalid OpOracleSupport = iota
	OpOracleExecutable
	OpOracleTerminator
	OpOraclePseudo
	OpOracleUnsupported
)

func (s OpOracleSupport) String() string {
	switch s {
	case OpOracleExecutable:
		return "executable"
	case OpOracleTerminator:
		return "terminator"
	case OpOraclePseudo:
		return "pseudo"
	case OpOracleUnsupported:
		return "unsupported"
	default:
		return "invalid"
	}
}

type OpTableArrayFactRole uint8

const (
	OpTableArrayFactNone OpTableArrayFactRole = iota
	OpTableArrayFactHeader
	OpTableArrayFactLen
	OpTableArrayFactData
)
