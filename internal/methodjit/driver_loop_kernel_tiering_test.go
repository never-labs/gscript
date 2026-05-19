//go:build darwin && arm64

package methodjit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDriverLoopKernelTieringKeepsDensePairwiseLoopOnVMRoute(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "benchmarks", "suite", "nbody_dense.gs"))
	if err != nil {
		t.Fatalf("read nbody_dense benchmark: %v", err)
	}
	top := compileProto(t, string(src))
	tm := NewTieringManager()
	info, ok := tm.driverLoopKernelForTiering(top)
	if !ok {
		t.Fatal("nbody_dense main should expose a driver-loop structural kernel")
	}
	if info.Name != "record_pairwise_numeric_loop" {
		t.Fatalf("driver loop kernel=%q, want record_pairwise_numeric_loop", info.Name)
	}
	decision := tm.policy.Decide(top, analyzeFuncProfile(top), PromotionPolicyState{Manager: tm})
	if decision.Action != TieringActionStructuralKernel {
		t.Fatalf("tiering action=%s, want %s", decision.Action, TieringActionStructuralKernel)
	}
	if decision.Kernel.kernel != "record_pairwise_numeric_loop" {
		t.Fatalf("decision kernel=%q, want record_pairwise_numeric_loop", decision.Kernel.kernel)
	}
}
