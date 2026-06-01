package vm

import "github.com/never-labs/leia/internal/runtime"

func (vm *VM) globalValue(name string) (runtime.Value, bool) {
	if vm.globalOverrides != nil {
		if v, ok := vm.globalOverrides[name]; ok {
			return v, true
		}
	}
	v, ok := vm.globals[name]
	return v, ok
}
