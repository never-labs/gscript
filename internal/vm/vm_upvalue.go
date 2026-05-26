package vm

// Open-upvalue management, split verbatim from vm.go.

func (vm *VM) RegisterOpenUpvalue(uv *Upvalue) {
	// Don't add duplicates.
	for _, existing := range vm.openUpvals {
		if existing == uv {
			return
		}
	}
	vm.openUpvals = append(vm.openUpvals, uv)
}

// FindOrCreateUpvalue returns the VM-tracked open upvalue for regIdx.
// JIT op-exit closure creation uses this to mirror interpreter OP_CLOSURE
// semantics and avoid accumulating duplicate open upvalues for loop locals.

func (vm *VM) FindOrCreateUpvalue(regIdx int) *Upvalue {
	return vm.findOrCreateUpvalue(regIdx)
}

// CloseUpvalues closes all open upvalues at or above fromReg.
// Used by the baseline JIT for OP_CLOSE handling.

func (vm *VM) CloseUpvalues(fromReg int) {
	vm.closeUpvalues(fromReg)
}

func (vm *VM) findOrCreateUpvalue(regIdx int) *Upvalue {
	for _, uv := range vm.openUpvals {
		if uv.regIdx == regIdx {
			return uv
		}
	}
	uv := NewOpenUpvalue(&vm.regs[regIdx], regIdx)
	vm.openUpvals = append(vm.openUpvals, uv)
	return uv
}

func (vm *VM) closeUpvalues(fromReg int) {
	if len(vm.openUpvals) == 0 {
		return
	}
	kept := vm.openUpvals[:0]
	for _, uv := range vm.openUpvals {
		if uv.regIdx >= fromReg {
			uv.Close()
		} else {
			kept = append(kept, uv)
		}
	}
	vm.openUpvals = kept
}

// ---- Helpers ----
