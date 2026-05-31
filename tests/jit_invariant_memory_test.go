//go:build darwin && arm64

package tests_test

import "testing"

// ============================================================================
// Invariant 2: Memory consistency at exit points
// ============================================================================

// Loop-done: all modified slots written back correctly

func TestInv2_LoopDone_SingleVar(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0; for i:=1;i<=100;i++ { s=s+i }; result:=s`, "result")
}

func TestInv2_LoopDone_MultiVar(t *testing.T) {
	assertVMEqualsJIT(t, `a:=0; b:=0; for i:=1;i<=50;i++ { a=a+i; b=b+i*i }; ra:=a; rb:=b`, "ra", "rb")
}

func TestInv2_LoopDone_FloatVar(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0.0; for i:=1;i<=100;i++ { s=s+0.5 }; result:=s`, "result")
}

// Side-exit: memory correct at the guard's bytecode PC

func TestInv2_SideExit_SimpleIf(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=0;i<100;i++ { if i==50 { c=c+1 } }; result:=c`, "result")
}

func TestInv2_SideExit_MultipleExits(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=0;i<100;i++ { if i==20 || i==40 || i==60 || i==80 { c=c+1 } }; result:=c`, "result")
}

func TestInv2_SideExit_ModCondition(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=0;i<100;i++ { if i%7==0 { c=c+1 } }; result:=c`, "result")
}

func TestInv2_SideExit_LTCondition(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=0;i<100;i++ { if i>75 { c=c+1 } }; result:=c`, "result")
}
