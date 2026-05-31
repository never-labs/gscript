//go:build darwin && arm64

package tests_test

import "testing"

// ============================================================================
// Stress tests (dimension combinations)
// ============================================================================

func TestStress_ManyIterations(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0; for i:=1;i<=100000;i++ { s=s+i }; result:=s`, "result")
}

func TestStress_ManySideExits(t *testing.T) {
	assertVMEqualsJIT(t, `c:=0; for i:=0;i<10000;i++ { if i%3==0 { c=c+1 } }; result:=c`, "result")
}

func TestStress_DeepNesting(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0; for i:=1;i<=10;i++ { for j:=1;j<=10;j++ { for k:=1;k<=10;k++ { s=s+1 } } }; result:=s`, "result")
}

func TestStress_FloatPrecision(t *testing.T) {
	assertVMEqualsJIT(t, `s:=0.0; for i:=1;i<=10000;i++ { s=s+0.0001 }; result:=s`, "result")
}

func TestStress_FunctionCallInLoop(t *testing.T) {
	assertVMEqualsJIT(t, `func double(x) { return x * 2 }; s:=0; for i:=1;i<=100;i++ { s=s+double(i) }; result:=s`, "result")
}

func TestStress_TableReadWrite(t *testing.T) {
	assertVMEqualsJIT(t, `a:={}; for i:=1;i<=100;i++ { a[i]=i*10 }; s:=0; for i:=1;i<=100;i++ { s=s+a[i] }; result:=s`, "result")
}

func TestStress_FieldReadWrite(t *testing.T) {
	assertVMEqualsJIT(t, `p:={x:0.0,y:0.0}; for i:=1;i<=100;i++ { p.x=p.x+0.1; p.y=p.y+0.2 }; rx:=p.x; ry:=p.y`, "rx", "ry")
}

func TestStress_MatmulMini(t *testing.T) {
	src := `
n := 5
a := {}; b := {}
for i := 0; i < n; i++ {
    ar := {}; br := {}
    for j := 0; j < n; j++ {
        ar[j] = (i*n+j+1.0)/(n*n)
        br[j] = (j*n+i+1.0)/(n*n)
    }
    a[i] = ar; b[i] = br
}
s := 0.0
for i := 0; i < n; i++ {
    for j := 0; j < n; j++ {
        v := 0.0
        for k := 0; k < n; k++ {
            v = v + a[i][k] * b[k][j]
        }
        s = s + v
    }
}
result := s`
	assertVMEqualsJIT(t, src, "result")
}
