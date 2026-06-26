package q

// EvalScalarKind identifies scalar results that can cross hot q session
// planned-eval paths without boxing through any.
type EvalScalarKind uint8

const (
	EvalScalarInvalid EvalScalarKind = iota
	EvalScalarInt
	EvalScalarFloat
)

// EvalScalarResult is the typed scalar result channel used by hot q plans.
// The public Eval APIs still return any; this avoids heap boxing only for
// callers that explicitly opt into the planned scalar fast path.
type EvalScalarResult struct {
	Kind EvalScalarKind
	I64  int64
	F64  float64
}

func evalScalarInt(value int64) EvalScalarResult {
	return EvalScalarResult{Kind: EvalScalarInt, I64: value}
}

func evalScalarFloat(value float64) EvalScalarResult {
	return EvalScalarResult{Kind: EvalScalarFloat, F64: value}
}
