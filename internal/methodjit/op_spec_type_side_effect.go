package methodjit

func (e OpSideEffect) String() string {
	switch e {
	case OpSideEffectNone:
		return "none"
	case OpSideEffectRead:
		return "read"
	case OpSideEffectWrite:
		return "write"
	case OpSideEffectReadWrite:
		return "read-write"
	case OpSideEffectAllocate:
		return "allocate"
	case OpSideEffectCall:
		return "call"
	case OpSideEffectControl:
		return "control"
	case OpSideEffectConcurrency:
		return "concurrency"
	default:
		return "invalid"
	}
}
