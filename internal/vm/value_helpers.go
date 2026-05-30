package vm

import "github.com/Never-Labs/gscript/internal/runtime"

func closureFromValue(v runtime.Value) (*Closure, bool) {
	if p := v.VMClosurePointer(); p != nil {
		return (*Closure)(p), true
	}
	cl, ok := v.Ptr().(*Closure)
	return cl, ok
}
