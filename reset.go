package gscript

// Reset clears all script-created VM state and reinitializes the VM with the
// same options used by New.
//
// Reset discards globals, loaded module cache, bytecode VM/JIT state, script
// directory changes made by executed programs, and registered Go bindings added
// after construction. Options such as WithLibs, WithRequirePath, WithPrint,
// WithMaxSteps, WithVM, and WithJIT are preserved.
//
// Reset is explicit: Pool does not call it unless a reset hook is configured.
// Like the rest of VM, Reset is not goroutine-safe.
func (vm *VM) Reset() {
	if vm == nil {
		return
	}
	fresh := newVM(vm.opts)
	vm.interp = fresh.interp
	vm.bvm = nil
}
