//go:build darwin && arm64

package tests_test

import "testing"

// ============================================================================
// Invariant 1: Value correctness (single trace execution)
// ============================================================================

func TestInv1_IntArith(t *testing.T) {
	assertVMEqualsJIT(t, `result := 0; for i:=1;i<=100;i++ { result = result + i }`, "result")
}

func TestInv1_FloatArith(t *testing.T) {
	assertVMEqualsJIT(t, `result := 0.0; for i:=1;i<=100;i++ { result = result + 0.01 * i }`, "result")
}

func TestInv1_IntMul(t *testing.T) {
	assertVMEqualsJIT(t, `result := 1; for i:=1;i<=10;i++ { result = result * i }`, "result")
}

func TestInv1_FloatDiv(t *testing.T) {
	assertVMEqualsJIT(t, `result := 1024.0; for i:=1;i<=10;i++ { result = result / 2.0 }`, "result")
}

func TestInv1_IntMod(t *testing.T) {
	assertVMEqualsJIT(t, `result := 0; for i:=1;i<=100;i++ { result = result + i % 7 }`, "result")
}

func TestInv1_MixedIntFloat(t *testing.T) {
	assertVMEqualsJIT(t, `result := 0.0; for i:=1;i<=50;i++ { result = result + i * 0.1 }`, "result")
}

func TestInv1_Negation(t *testing.T) {
	assertVMEqualsJIT(t, `result := 0; for i:=1;i<=10;i++ { result = result + (-i) }`, "result")
}

func TestInv1_FloatMulAccum(t *testing.T) {
	assertVMEqualsJIT(t, `result := 1.0; for i:=1;i<=20;i++ { result = result * 1.05 }`, "result")
}
