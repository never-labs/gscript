package vm

import "github.com/Never-Labs/gscript/internal/runtime"

const maxCallSiteScalarScratch = 1 << 20

func callSiteRuntimeSpecializationArity(n int) bool {
	return n == 1 || n == 2 || n == 3 || n == 4
}

func (vm *VM) tryValueRuntimeSpecialization(cl *Closure, args []runtime.Value, c int, dst int) (bool, error) {
	handled, results, err := vm.tryRunValueRuntimeSpecialization(cl, args)
	if !handled || err != nil {
		return handled, err
	}
	vm.writeCallResults(dst, c, results)
	return true, nil
}

func (vm *VM) tryRunValueRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if handled, results, err := vm.tryRunCallSiteValueRuntimeSpecialization(cl, args, true); handled || err != nil {
		return handled, results, err
	}
	return false, nil, nil
}

func (vm *VM) tryRunNonRecursiveTableValueRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, []runtime.Value, error) {
	if handled, results, err := vm.tryRunCallSiteValueRuntimeSpecialization(cl, args, false); handled || err != nil {
		return handled, results, err
	}
	return false, nil, nil
}

// tryNoResultRuntimeSpecialization executes a guarded call-site numeric specialization and writes
// the no-result call convention used by in-place specializations.
func (vm *VM) tryNoResultRuntimeSpecialization(cl *Closure, args []runtime.Value, c int, dst int) (bool, error) {
	handled, err := vm.tryRunNoResultRuntimeSpecialization(cl, args)
	if !handled || err != nil {
		return handled, err
	}
	vm.writeNoResults(dst, c)
	return true, nil
}

func (vm *VM) tryRunNoResultRuntimeSpecialization(cl *Closure, args []runtime.Value) (bool, error) {
	if handled, err := vm.tryRunCallSiteNoResultRuntimeSpecialization(cl, args); handled || err != nil {
		return handled, err
	}
	return false, nil
}

// TryRunNoResultCallSiteRuntimeSpecializationForJIT executes a guarded no-result structural
// call-site runtime specialization for a JIT exit helper. It returns handled=false when the
// callee or arguments do not satisfy the registered specialization guards.
func (vm *VM) TryRunNoResultCallSiteRuntimeSpecializationForJIT(fn runtime.Value, args []runtime.Value) (bool, error) {
	cl, ok := closureFromValue(fn)
	if !ok {
		return false, nil
	}
	return vm.tryRunNoResultRuntimeSpecialization(cl, args)
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

func (vm *VM) callSiteFloatScratch(n int) []float64 {
	if n <= 0 {
		return nil
	}
	if n > maxCallSiteScalarScratch {
		return make([]float64, n)
	}
	if cap(vm.callSiteFloatBuf) < n {
		vm.callSiteFloatBuf = make([]float64, n)
	}
	return vm.callSiteFloatBuf[:n]
}

func (vm *VM) callSiteIntScratch(n int) []int64 {
	if n <= 0 {
		return nil
	}
	if n > maxCallSiteScalarScratch {
		return make([]int64, n)
	}
	if cap(vm.callSiteIntBuf) < n {
		vm.callSiteIntBuf = make([]int64, n)
	}
	return vm.callSiteIntBuf[:n]
}

func (vm *VM) callSiteValueScratch(n int) []runtime.Value {
	if n <= 0 {
		return nil
	}
	if cap(vm.callSiteValueBuf) < n {
		vm.callSiteValueBuf = make([]runtime.Value, n)
	}
	return vm.callSiteValueBuf[:n]
}
