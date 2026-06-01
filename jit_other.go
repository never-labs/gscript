//go:build !(darwin && arm64)

package leia

import bytecodevm "github.com/never-labs/leia/internal/vm"

func enableJIT(_ *bytecodevm.VM) {
	// JIT not available on this platform.
}
