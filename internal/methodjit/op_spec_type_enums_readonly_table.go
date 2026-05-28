package methodjit

type OpReadonlyTableParamUseRole uint8

const (
	OpReadonlyTableParamUseNone OpReadonlyTableParamUseRole = iota
	OpReadonlyTableParamUseBenign
	OpReadonlyTableParamUseFirstArgMutation
	OpReadonlyTableParamUseCallEscape
)

func (r OpReadonlyTableParamUseRole) String() string {
	switch r {
	case OpReadonlyTableParamUseBenign:
		return "benign"
	case OpReadonlyTableParamUseFirstArgMutation:
		return "first-arg-mutation"
	case OpReadonlyTableParamUseCallEscape:
		return "call-escape"
	default:
		return "none"
	}
}
