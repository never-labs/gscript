//go:build darwin && arm64

package leia

import (
	"github.com/never-labs/leia/internal/methodjit"
	bytecodevm "github.com/never-labs/leia/internal/vm"
)

func enableJIT(bvm *bytecodevm.VM) {
	// TieringManager: Tier 1 (baseline) + Tier 2 (optimizing) with threshold-based
	// promotion. With default threshold (100), functions must be called 100+ times
	// through the VM path to promote. Tier 1 BLR calls bypass the VM, so most
	// functions stay at Tier 1. Use CompileTier2() for explicit promotion.
	tm := methodjit.NewTieringManager()
	bvm.SetMethodJIT(tm)
}
