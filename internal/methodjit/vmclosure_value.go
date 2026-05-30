package methodjit

import (
	"github.com/Never-Labs/gscript/internal/runtime"
	"github.com/Never-Labs/gscript/internal/vm"
)

func vmClosureFromValue(v runtime.Value) (*vm.Closure, bool) {
	if p := v.VMClosurePointer(); p != nil {
		return (*vm.Closure)(p), true
	}
	cl, ok := v.Ptr().(*vm.Closure)
	return cl, ok
}
