//go:build darwin && arm64

package tests_test

import "testing"

// ============================================================================
// Invariant 3: Control flow correctness
// ============================================================================

// Break exits loop correctly

func TestInv3_Break_Simple(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=1;i<=1000;i++ { if i>10 { break }; c=c+1 }; result:=c`, "result")
}

func TestInv3_Break_FloatCondition(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; s:=0.0; for i:=1;i<=1000;i++ { s=s+0.1; if s>5.0 { break }; c=c+1 }; result:=c`, "result")
}

func TestInv3_Break_InNestedLoop(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=1;i<=10;i++ { for j:=1;j<=100;j++ { if j>i*3 { break } }; c=c+1 }; result:=c`, "result")
}

// Nested loops

func TestInv3_NestedLoop_Simple(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0; for i:=1;i<=10;i++ { for j:=1;j<=10;j++ { s=s+1 } }; result:=s`, "result")
}

func TestInv3_NestedLoop_DependentBounds(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0; for i:=1;i<=10;i++ { for j:=1;j<=i;j++ { s=s+1 } }; result:=s`, "result")
}

func TestInv3_NestedLoop_FloatInner(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0.0; for i:=1;i<=5;i++ { for j:=1;j<=5;j++ { s=s+i*0.1+j*0.01 } }; result:=s`, "result")
}
