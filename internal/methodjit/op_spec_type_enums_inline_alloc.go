package methodjit

type OpInlineAllocationRole uint8

const (
	OpInlineAllocationNone OpInlineAllocationRole = iota
	OpInlineAllocationDynamic
	OpInlineAllocationFixed
	OpInlineAllocationFieldInit
	OpInlineAllocationArrayInit
)

func (r OpInlineAllocationRole) String() string {
	switch r {
	case OpInlineAllocationDynamic:
		return "dynamic"
	case OpInlineAllocationFixed:
		return "fixed"
	case OpInlineAllocationFieldInit:
		return "field-init"
	case OpInlineAllocationArrayInit:
		return "array-init"
	default:
		return "none"
	}
}
