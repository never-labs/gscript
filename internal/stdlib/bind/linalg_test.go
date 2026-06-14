package bind

import (
	"fmt"
	"math"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func linalgInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	return runWithLib(t, src, "linalg", BuildLinalg())
}

func linalgMigrationInterp(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "linalg", runtime.TableValue(BuildLinalg()))
	installTestModule(interp, "matrix", runtime.TableValue(BuildMatrix()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	installTestModule(interp, "stats", runtime.TableValue(BuildStats()))
	execOnInterp(t, interp, src)
	return interp
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

func TestLinalgMatrixResultsAreDenseMatrixCompatible(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		row  int
		col  int
		want float64
	}{
		{
			name: "matrix",
			src:  `m := linalg.matrix(2, 2, {1, 2, 3, 4})`,
			row:  1,
			col:  0,
			want: 3,
		},
		{
			name: "eye",
			src:  `m := linalg.eye(2)`,
			row:  1,
			col:  1,
			want: 1,
		},
		{
			name: "diag",
			src:  `m := linalg.diag({5, 6})`,
			row:  1,
			col:  1,
			want: 6,
		},
		{
			name: "zeros rows cols",
			src:  `m := linalg.zeros(2, 2)`,
			row:  1,
			col:  0,
			want: 0,
		},
		{
			name: "matmul",
			src:  `m := linalg.matmul(linalg.matrix(2, 2, {1, 2, 3, 4}), linalg.eye(2))`,
			row:  1,
			col:  0,
			want: 3,
		},
		{
			name: "transpose",
			src:  `m := linalg.transpose(linalg.matrix(2, 2, {1, 2, 3, 4}))`,
			row:  1,
			col:  0,
			want: 2,
		},
		{
			name: "scale matrix",
			src:  `m := linalg.scale(linalg.matrix(2, 2, {1, 2, 3, 4}), 2)`,
			row:  1,
			col:  0,
			want: 6,
		},
		{
			name: "add matrix",
			src:  `m := linalg.add(linalg.matrix(2, 2, {1, 2, 3, 4}), linalg.eye(2))`,
			row:  1,
			col:  0,
			want: 3,
		},
		{
			name: "sub matrix",
			src:  `m := linalg.sub(linalg.matrix(2, 2, {1, 2, 3, 4}), linalg.eye(2))`,
			row:  1,
			col:  0,
			want: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interp := linalgMigrationInterp(t, tc.src+fmt.Sprintf(`
before := matrix.getf(m, %d, %d)
matrix.setf(m, 0, 1, 9)
after := matrix.getf(m, 0, 1)
`, tc.row, tc.col))
			assertFloat(t, interp.GetGlobal("before"), tc.want)
			assertFloat(t, interp.GetGlobal("after"), 9)
			m := interp.GetGlobal("m")
			if !m.IsTable() || m.Table().DMStride() == 0 {
				t.Fatalf("%s returned %s without DenseMatrix metadata", tc.name, m.TypeName())
			}
		})
	}
}

func TestLinalgVectorResultsAreDenseArrayInteroperable(t *testing.T) {
	for _, tc := range []struct {
		name     string
		src      string
		wantLen  int
		wantMean float64
		wantNorm float64
	}{
		{
			name:     "vector",
			src:      `v := linalg.vector(1, 2, 3)`,
			wantLen:  3,
			wantMean: 2,
			wantNorm: math.Sqrt(14),
		},
		{
			name:     "zeros n",
			src:      `v := linalg.zeros(3)`,
			wantLen:  3,
			wantMean: 0,
			wantNorm: 0,
		},
		{
			name:     "matvec",
			src:      `v := linalg.matvec(linalg.matrix(2, 2, {1, 2, 3, 4}), {10, 20})`,
			wantLen:  2,
			wantMean: 80,
			wantNorm: math.Sqrt(14600),
		},
		{
			name:     "scale vector",
			src:      `v := linalg.scale(linalg.vector(1, 2, 3), 2)`,
			wantLen:  3,
			wantMean: 4,
			wantNorm: math.Sqrt(56),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interp := linalgMigrationInterp(t, tc.src+`
mean := stats.mean(v)
count := q.count(v)
norm := linalg.norm(v)
selfDot := linalg.dot(v, v)
`)
			v := interp.GetGlobal("v")
			if !v.IsDenseArray() {
				t.Fatalf("%s returned %s, want dense array", tc.name, v.TypeName())
			}
			if got := v.DenseArray().Len(); got != tc.wantLen {
				t.Fatalf("%s dense len = %d, want %d", tc.name, got, tc.wantLen)
			}
			assertFloat(t, interp.GetGlobal("mean"), tc.wantMean)
			assertFloat(t, interp.GetGlobal("norm"), tc.wantNorm)
			assertFloat(t, interp.GetGlobal("selfDot"), tc.wantNorm*tc.wantNorm)
			if got := interp.GetGlobal("count"); !got.IsInt() || int(got.Int()) != tc.wantLen {
				t.Fatalf("%s q.count = %v, want %d", tc.name, got, tc.wantLen)
			}
		})
	}
}

func assertFloat(t *testing.T, got runtime.Value, want float64) {
	t.Helper()
	if !got.IsNumber() || math.Abs(toFloat(got)-want) > 1e-9 {
		t.Fatalf("got %v, want %g", got, want)
	}
}

func assertTableFloat(t *testing.T, got runtime.Value, index int64, want float64) {
	t.Helper()
	if got.IsDenseArray() {
		v, err := got.DenseArray().At(int(index - 1))
		if err != nil {
			t.Fatalf("dense array[%d]: %v", index, err)
		}
		assertFloat(t, v, want)
		return
	}
	if got.IsTable() {
		assertFloat(t, got.Table().RawGetInt(index), want)
		return
	}
	t.Fatalf("got %s, want table or dense array", got.TypeName())
}

func assertMatrixFloat(t *testing.T, got runtime.Value, rows, cols int64, index int64, want float64) {
	t.Helper()
	if !got.IsTable() {
		t.Fatalf("got %s, want matrix table", got.TypeName())
	}
	tbl := got.Table()
	if backing, denseRows, stride, ok := tbl.DenseMatrixBacking(); ok {
		if int64(denseRows) != rows || int64(stride) != cols {
			t.Fatalf("dense shape = %dx%d, want %dx%d", denseRows, stride, rows, cols)
		}
		assertFloat(t, runtime.FloatValue(backing[index-1]), want)
		return
	}
	if tbl.RawGetString("rows").Int() != rows || tbl.RawGetString("cols").Int() != cols {
		t.Fatalf("shape = %dx%d, want %dx%d", tbl.RawGetString("rows").Int(), tbl.RawGetString("cols").Int(), rows, cols)
	}
	assertTableFloat(t, tbl.RawGetString("values"), index, want)
}
