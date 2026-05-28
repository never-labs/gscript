package methodjit

type OpInlineAllocationRole uint8

const (
	OpInlineAllocationNone OpInlineAllocationRole = iota
	OpInlineAllocationDynamic
	OpInlineAllocationFixed
	OpInlineAllocationFieldInit
	OpInlineAllocationArrayInit
)
