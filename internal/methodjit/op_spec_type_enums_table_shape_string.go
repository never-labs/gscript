package methodjit

func (r OpFixedShapeArrayElementWriteRole) String() string {
	switch r {
	case OpFixedShapeArrayElementWriteSingle:
		return "single"
	case OpFixedShapeArrayElementWriteVariadic:
		return "variadic"
	case OpFixedShapeArrayElementWriteConflict:
		return "conflict"
	default:
		return "none"
	}
}

func (r OpFixedShapeArrayElementReadRole) String() string {
	switch r {
	case OpFixedShapeArrayElementReadDirect:
		return "direct"
	case OpFixedShapeArrayElementReadLoweredArray:
		return "lowered-array"
	default:
		return "none"
	}
}

func (r OpFixedShapeReturnArrayElementRole) String() string {
	switch r {
	case OpFixedShapeReturnArrayElementStore:
		return "store"
	case OpFixedShapeReturnArrayElementInvalidator:
		return "invalidator"
	default:
		return "none"
	}
}

func (r OpLocalStringArrayTableUseRole) String() string {
	switch r {
	case OpLocalStringArrayTableUseStore:
		return "store"
	case OpLocalStringArrayTableUseRead:
		return "read"
	default:
		return "none"
	}
}
