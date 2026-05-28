//go:build darwin && arm64

package methodjit

func opByName(name string) (Op, bool) {
	if name == "OpTableArrayNestedLoad" {
		return OpTableArrayNestedLoad, true
	}
	for op := Op(0); op < OpMax; op++ {
		if "Op"+op.String() == name {
			return op, true
		}
	}
	return 0, false
}

func opIntentionallyNotHandledByEmitInstr(op Op) bool {
	switch op {
	case OpBoxInt, OpBoxFloat, OpUnboxInt, OpUnboxFloat:
		return true
	case OpYield:
		return true
	default:
		return false
	}
}
