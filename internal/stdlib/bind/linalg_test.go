package bind

import (
	"math"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func linalgInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	return runWithLib(t, src, "linalg", BuildLinalg())
}

func TestLinalgVectorMatrixAndAccess(t *testing.T) {
	interp := linalgInterp(t, `
v := linalg.vector(1, 2, 3)
fromTable := linalg.vector({4, 5, 6})
m := linalg.matrix(2, 3, {1, 2, 3, 4, 5, 6})
before := linalg.get(m, 2, 3)
linalg.set(m, 1, 2, 9)
after := linalg.get(m, 1, 2)
zv := linalg.zeros(3)
zm := linalg.zeros(2, 2)
eye := linalg.eye(3)
diag := linalg.diag({7, 8})
`)
	assertTableFloat(t, interp.GetGlobal("v"), 3, 3)
	assertTableFloat(t, interp.GetGlobal("fromTable"), 2, 5)
	assertFloat(t, interp.GetGlobal("before"), 6)
	assertFloat(t, interp.GetGlobal("after"), 9)
	assertTableFloat(t, interp.GetGlobal("zv"), 3, 0)
	assertMatrixFloat(t, interp.GetGlobal("zm"), 2, 2, 4, 0)
	assertMatrixFloat(t, interp.GetGlobal("eye"), 3, 3, 5, 1)
	assertMatrixFloat(t, interp.GetGlobal("diag"), 2, 2, 4, 8)
}

func TestLinalgOps(t *testing.T) {
	interp := linalgInterp(t, `
a := linalg.vector(1, 2, 3)
b := linalg.vector({4, 5, 6})
sum := linalg.add(a, b)
diff := linalg.sub(b, a)
scaled := linalg.scale(a, 2)
dot := linalg.dot(a, b)
norm := linalg.norm(linalg.vector(3, 4))
m := linalg.matrix(2, 2, {1, 2, 3, 4})
mv := linalg.matvec(m, {10, 20})
mt := linalg.transpose(m)
mm := linalg.matmul(m, linalg.eye(2))
sol := linalg.solve2(linalg.matrix(2, 2, {2, 1, 1, 3}), {1, 2})
`)
	assertTableFloat(t, interp.GetGlobal("sum"), 3, 9)
	assertTableFloat(t, interp.GetGlobal("diff"), 1, 3)
	assertTableFloat(t, interp.GetGlobal("scaled"), 2, 4)
	assertFloat(t, interp.GetGlobal("dot"), 32)
	assertFloat(t, interp.GetGlobal("norm"), 5)
	assertTableFloat(t, interp.GetGlobal("mv"), 1, 50)
	assertMatrixFloat(t, interp.GetGlobal("mt"), 2, 2, 2, 3)
	assertMatrixFloat(t, interp.GetGlobal("mm"), 2, 2, 3, 3)
	assertTableFloat(t, interp.GetGlobal("sol"), 1, 0.2)
	assertTableFloat(t, interp.GetGlobal("sol"), 2, 0.6)
}

func assertFloat(t *testing.T, got runtime.Value, want float64) {
	t.Helper()
	if !got.IsNumber() || math.Abs(toFloat(got)-want) > 1e-9 {
		t.Fatalf("got %v, want %g", got, want)
	}
}

func assertTableFloat(t *testing.T, got runtime.Value, index int64, want float64) {
	t.Helper()
	if !got.IsTable() {
		t.Fatalf("got %s, want table", got.TypeName())
	}
	assertFloat(t, got.Table().RawGetInt(index), want)
}

func assertMatrixFloat(t *testing.T, got runtime.Value, rows, cols int64, index int64, want float64) {
	t.Helper()
	if !got.IsTable() {
		t.Fatalf("got %s, want matrix table", got.TypeName())
	}
	tbl := got.Table()
	if tbl.RawGetString("rows").Int() != rows || tbl.RawGetString("cols").Int() != cols {
		t.Fatalf("shape = %dx%d, want %dx%d", tbl.RawGetString("rows").Int(), tbl.RawGetString("cols").Int(), rows, cols)
	}
	assertTableFloat(t, tbl.RawGetString("values"), index, want)
}
