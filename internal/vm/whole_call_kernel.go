package vm

import "github.com/gscript/gscript/internal/runtime"

const maxWholeCallScalarScratch = 1 << 20

func wholeCallKernelArity(n int) bool {
	return n == 1 || n == 2 || n == 3 || n == 4
}

func (vm *VM) tryValueWholeCallKernel(cl *Closure, args []runtime.Value, c int, dst int) (bool, error) {
	handled, results, err := vm.tryRunValueWholeCallKernel(cl, args)
	if !handled || err != nil {
		return handled, err
	}
	vm.writeCallResults(dst, c, results)
	return true, nil
}

func (vm *VM) tryRunValueWholeCallKernel(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if handled, results, err := vm.tryRunWholeCallValueRuntimeSpecialization(cl, args, true); handled || err != nil {
		return handled, results, err
	}
	return false, nil, nil
}

func (vm *VM) tryRunNonRecursiveTableValueWholeCallKernel(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if handled, results, err := vm.tryRunWholeCallValueRuntimeSpecialization(cl, args, false); handled || err != nil {
		return handled, results, err
	}
	return false, nil, nil
}

// tryWholeCallKernel executes a guarded whole-call numeric kernel and writes
// the no-result call convention used by in-place kernels.
func (vm *VM) tryWholeCallKernel(cl *Closure, args []runtime.Value, c int, dst int) (bool, error) {
	handled, err := vm.tryRunWholeCallKernel(cl, args)
	if !handled || err != nil {
		return handled, err
	}
	vm.writeNoResults(dst, c)
	return true, nil
}

func (vm *VM) tryRunWholeCallKernel(cl *Closure, args []runtime.Value) (bool, error) {
	if handled, err := vm.tryRunWholeCallNoResultRuntimeSpecialization(cl, args); handled || err != nil {
		return handled, err
	}
	return false, nil
}

// TryRunNoResultWholeCallKernelForJIT executes a guarded no-result structural
// whole-call kernel for a JIT exit helper. It returns handled=false when the
// callee or arguments do not satisfy the registered kernel guards.
func (vm *VM) TryRunNoResultWholeCallKernelForJIT(fn runtime.Value, args []runtime.Value) (bool, error) {
	cl, ok := closureFromValue(fn)
	if !ok {
		return false, nil
	}
	return vm.tryRunWholeCallKernel(cl, args)
}

func (vm *VM) writeNoResults(dst, c int) {
	if c == 0 {
		vm.top = dst
		return
	}
	for i := 0; i < c-1; i++ {
		vm.regs[dst+i] = runtime.NilValue()
	}
}

func (vm *VM) wholeCallFloatScratch(n int) []float64 {
	if n <= 0 {
		return nil
	}
	if n > maxWholeCallScalarScratch {
		return make([]float64, n)
	}
	if cap(vm.wholeCallFloatBuf) < n {
		vm.wholeCallFloatBuf = make([]float64, n)
	}
	return vm.wholeCallFloatBuf[:n]
}

func (vm *VM) wholeCallIntScratch(n int) []int64 {
	if n <= 0 {
		return nil
	}
	if n > maxWholeCallScalarScratch {
		return make([]int64, n)
	}
	if cap(vm.wholeCallIntBuf) < n {
		vm.wholeCallIntBuf = make([]int64, n)
	}
	return vm.wholeCallIntBuf[:n]
}

func (vm *VM) wholeCallValueScratch(n int) []runtime.Value {
	if n <= 0 {
		return nil
	}
	if cap(vm.wholeCallValueBuf) < n {
		vm.wholeCallValueBuf = make([]runtime.Value, n)
	}
	return vm.wholeCallValueBuf[:n]
}
