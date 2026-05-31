package methodjit

import (
	"github.com/never-labs/gscript/internal/runtime"
	"github.com/never-labs/gscript/internal/vm"
)

func vmClosureFromValue(v runtime.Value) (*vm.Closure, bool) {
	if p := v.VMClosurePointer(); p != nil {
		return (*vm.Closure)(p), true
	}
	cl, ok := v.Ptr().(*vm.Closure)
	return cl, ok
}
