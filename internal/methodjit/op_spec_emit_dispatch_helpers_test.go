//go:build darwin && arm64

package methodjit

func emitInstrHandledOps(t testFataler) map[Op]OpEmitterFamily {
	ops := make(map[Op]OpEmitterFamily)
	for op := range emitInstrSwitchHandledOps(t) {
		ops[op] = OpEmitterInvalid
	}
	for _, delegate := range emitterFamilyDelegateRegistry() {
		for op := range emitterSwitchHandledOps(t, delegate.filename, delegate.funcName) {
			if existingFamily, exists := ops[op]; exists {
				t.Fatalf("%s handled by both emitInstr/direct family %v and %s", op, existingFamily, delegate.funcName)
			}
			ops[op] = delegate.family
		}
	}
	return ops
}

func mergeOps(dst, src map[Op]bool) {
	for op := range src {
		dst[op] = true
	}
}
