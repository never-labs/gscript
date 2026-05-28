package methodjit

type OpBoolTableFillKindSource uint8

const (
	OpBoolTableFillKindNone OpBoolTableFillKindSource = iota
	OpBoolTableFillKindAux
	OpBoolTableFillKindAux2
)
