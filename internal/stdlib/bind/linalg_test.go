package bind

import (
	"fmt"
	"math"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	stddata "github.com/never-labs/leia/internal/stdlib/lib/data"
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
row := linalg.row(1, 2, 3)
col := linalg.col({4, 5})
fromTable := linalg.vector({4, 5, 6})
nested := linalg.matrix({{1, 2}, {3, 4}})
m := linalg.matrix(2, 3, {1, 2, 3, 4, 5, 6})
before := linalg.get(m, 2, 3)
linalg.set(m, 1, 2, 9)
after := linalg.get(m, 1, 2)
zv := linalg.zeros(3)
zm := linalg.zeros(2, 2)
eye := linalg.eye(3)
diag := linalg.diag({7, 8})
row_at := linalg.at(row, 3)
col_at := linalg.at(col, 2)
matrix_at := linalg.at(m, 2, 1)
vec_at := linalg.at({9, 8, 7}, 2)
`)
	assertTableFloat(t, interp.GetGlobal("v"), 3, 3)
	assertMatrixFloat(t, interp.GetGlobal("row"), 1, 3, 3, 3)
	assertMatrixFloat(t, interp.GetGlobal("col"), 2, 1, 2, 5)
	assertMatrixFloat(t, interp.GetGlobal("nested"), 2, 2, 4, 4)
	assertTableFloat(t, interp.GetGlobal("fromTable"), 2, 5)
	assertFloat(t, interp.GetGlobal("before"), 6)
	assertFloat(t, interp.GetGlobal("after"), 9)
	assertTableFloat(t, interp.GetGlobal("zv"), 3, 0)
	assertMatrixFloat(t, interp.GetGlobal("zm"), 2, 2, 4, 0)
	assertMatrixFloat(t, interp.GetGlobal("eye"), 3, 3, 5, 1)
	assertMatrixFloat(t, interp.GetGlobal("diag"), 2, 2, 4, 8)
	assertFloat(t, interp.GetGlobal("row_at"), 3)
	assertFloat(t, interp.GetGlobal("col_at"), 5)
	assertFloat(t, interp.GetGlobal("matrix_at"), 4)
	assertFloat(t, interp.GetGlobal("vec_at"), 8)
}

func TestLinalgOps(t *testing.T) {
	interp := linalgInterp(t, `
a := linalg.vector(1, 2, 3)
b := linalg.vector({4, 5, 6})
sum := linalg.add(a, b)
diff := linalg.sub(b, a)
shifted := linalg.add(a, b, 10)
scalarFirst := linalg.sub(10, a)
product := linalg.mul(a, b, 2)
quot := linalg.div(product, 2)
scaled := linalg.scale(a, 2)
affine := linalg.affine(a, b, 2, -1)
affineShift := linalg.affine(a, 10, 0.5, 1)
axpy := linalg.axpy(a, 0.5, b)
add_scaled := linalg.add_scaled(a, -2, 1)
dot := linalg.dot(a, b)
norm := linalg.norm(linalg.vector(3, 4))
m := linalg.matrix(2, 2, {1, 2, 3, 4})
mv := linalg.matvec(m, {10, 20})
mt := linalg.transpose(m)
mt_short := linalg.T(m)
mt_lower := linalg.t(m)
trace := linalg.trace(m)
mm := linalg.matmul(m, linalg.eye(2))
mm3 := linalg.matmul(m, linalg.eye(2), linalg.matrix({{1, 0}, {0, 2}}))
mmv := linalg.matmul(m, {10, 20})
cm := linalg.chainmul(m, linalg.eye(2), linalg.eye(2))
sw := linalg.sandwich(linalg.matrix({{1, 2}, {0, 1}}), linalg.eye(2))
mshift := linalg.add(m, 10)
maffine := linalg.affine(m, linalg.eye(2), 2, -10)
rowAffine := linalg.affine(linalg.row(1, 2), 10, 2, -1)
colAffine := linalg.affine(100, linalg.col(1, 2), 1, -3)
vec_from_row := linalg.vec(linalg.row(7, 8))
vec_from_col := linalg.vec(linalg.col(9, 10))
scalarAffine := linalg.affine(2, 3, 4, 5)
sol := linalg.solve2(linalg.matrix(2, 2, {2, 1, 1, 3}), {1, 2})
solg := linalg.solve(linalg.matrix({{2, 1}, {1, 3}}), {1, 2})
solm := linalg.solve(linalg.matrix({{2, 1}, {1, 3}}), linalg.matrix({{1, 2}, {2, 1}}))
solr := linalg.solve_right(linalg.matrix({{4, 7}}), linalg.matrix({{2, 1}, {1, 3}}))
single := linalg.scalar(linalg.matrix({{42}}))
`)
	assertTableFloat(t, interp.GetGlobal("sum"), 3, 9)
	assertTableFloat(t, interp.GetGlobal("diff"), 1, 3)
	assertTableFloat(t, interp.GetGlobal("shifted"), 3, 19)
	assertTableFloat(t, interp.GetGlobal("scalarFirst"), 2, 8)
	assertTableFloat(t, interp.GetGlobal("product"), 2, 20)
	assertTableFloat(t, interp.GetGlobal("quot"), 3, 18)
	assertTableFloat(t, interp.GetGlobal("scaled"), 2, 4)
	assertTableFloat(t, interp.GetGlobal("affine"), 1, -2)
	assertTableFloat(t, interp.GetGlobal("affine"), 3, 0)
	assertTableFloat(t, interp.GetGlobal("affineShift"), 2, 11)
	assertTableFloat(t, interp.GetGlobal("axpy"), 3, 6)
	assertTableFloat(t, interp.GetGlobal("add_scaled"), 2, 0)
	assertFloat(t, interp.GetGlobal("dot"), 32)
	assertFloat(t, interp.GetGlobal("norm"), 5)
	assertTableFloat(t, interp.GetGlobal("mv"), 1, 50)
	assertMatrixFloat(t, interp.GetGlobal("mt"), 2, 2, 2, 3)
	assertMatrixFloat(t, interp.GetGlobal("mt_short"), 2, 2, 2, 3)
	assertMatrixFloat(t, interp.GetGlobal("mt_lower"), 2, 2, 3, 2)
	assertFloat(t, interp.GetGlobal("trace"), 5)
	assertMatrixFloat(t, interp.GetGlobal("mm"), 2, 2, 3, 3)
	assertMatrixFloat(t, interp.GetGlobal("mm3"), 2, 2, 4, 8)
	assertTableFloat(t, interp.GetGlobal("mmv"), 1, 50)
	assertTableFloat(t, interp.GetGlobal("mmv"), 2, 110)
	assertMatrixFloat(t, interp.GetGlobal("cm"), 2, 2, 4, 4)
	assertMatrixFloat(t, interp.GetGlobal("sw"), 2, 2, 1, 5)
	assertMatrixFloat(t, interp.GetGlobal("sw"), 2, 2, 2, 2)
	assertMatrixFloat(t, interp.GetGlobal("mshift"), 2, 2, 3, 13)
	assertMatrixFloat(t, interp.GetGlobal("maffine"), 2, 2, 1, -8)
	assertMatrixFloat(t, interp.GetGlobal("maffine"), 2, 2, 4, -2)
	assertMatrixFloat(t, interp.GetGlobal("rowAffine"), 1, 2, 2, -6)
	assertMatrixFloat(t, interp.GetGlobal("colAffine"), 2, 1, 2, 94)
	assertTableFloat(t, interp.GetGlobal("vec_from_row"), 2, 8)
	assertTableFloat(t, interp.GetGlobal("vec_from_col"), 2, 10)
	assertFloat(t, interp.GetGlobal("scalarAffine"), 23)
	assertTableFloat(t, interp.GetGlobal("sol"), 1, 0.2)
	assertTableFloat(t, interp.GetGlobal("sol"), 2, 0.6)
	assertTableFloat(t, interp.GetGlobal("solg"), 1, 0.2)
	assertTableFloat(t, interp.GetGlobal("solg"), 2, 0.6)
	assertMatrixFloat(t, interp.GetGlobal("solm"), 2, 2, 1, 0.2)
	assertMatrixFloat(t, interp.GetGlobal("solm"), 2, 2, 2, 1)
	assertMatrixFloat(t, interp.GetGlobal("solm"), 2, 2, 4, 0)
	assertMatrixFloat(t, interp.GetGlobal("solr"), 1, 2, 1, 1)
	assertMatrixFloat(t, interp.GetGlobal("solr"), 1, 2, 2, 2)
	assertFloat(t, interp.GetGlobal("single"), 42)
}

func TestLinalgMatrixResultsAreDenseMatrixCompatible(t *testing.T) {
	for _, tc := range []struct {
		name   string
		src    string
		row    int
		col    int
		want   float64
		setRow int
		setCol int
	}{
		{
			name:   "row",
			src:    `m := linalg.row({1, 2})`,
			row:    0,
			col:    1,
			want:   2,
			setRow: 0,
			setCol: 1,
		},
		{
			name:   "col",
			src:    `m := linalg.col(1, 2)`,
			row:    1,
			col:    0,
			want:   2,
			setRow: 1,
			setCol: 0,
		},
		{
			name: "nested",
			src:  `m := linalg.matrix({{1, 2}, {3, 4}})`,
			row:  1,
			col:  0,
			want: 3,
		},
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
		{
			name: "affine matrix",
			src:  `m := linalg.affine(linalg.matrix(2, 2, {1, 2, 3, 4}), 10, 2, -1)`,
			row:  1,
			col:  0,
			want: -4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setRow, setCol := tc.setRow, tc.setCol
			if setRow == 0 && setCol == 0 {
				setCol = 1
			}
			interp := linalgMigrationInterp(t, tc.src+fmt.Sprintf(`
before := matrix.getf(m, %d, %d)
matrix.setf(m, %d, %d, 9)
after := matrix.getf(m, %d, %d)
`, tc.row, tc.col, setRow, setCol, setRow, setCol))
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
		{
			name:     "affine vector",
			src:      `v := linalg.affine(linalg.vector(1, 2, 3), 10, 2, -1)`,
			wantLen:  3,
			wantMean: -6,
			wantNorm: math.Sqrt(116),
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

func TestLinalgDenseRuntimeStatsFeedQCacheStats(t *testing.T) {
	qClearCaches()
	stddata.ClearRuntimeKernelExecutionStats()
	t.Cleanup(qClearCaches)
	t.Cleanup(stddata.ClearRuntimeKernelExecutionStats)

	interp := linalgMigrationInterp(t, `
v := linalg.vector(1, 2, 3)
scaled := linalg.scale(v, 2)
m := linalg.matmul(linalg.matrix(2, 2, {1, 2, 3, 4}), linalg.eye(2))
stats := q.cache_stats()
`)
	assertTableFloat(t, interp.GetGlobal("scaled"), 3, 6)
	assertMatrixFloat(t, interp.GetGlobal("m"), 2, 2, 3, 3)

	row := qTestCacheStatsRowTable(t, interp.GetGlobal("stats").Table(), "q_runtime_kernel_execution")
	if got := row.RawGetString("hits"); !got.IsInt() || got.Int() < 5 {
		t.Fatalf("q_runtime_kernel_execution hits = %v, want at least 5", got)
	}
	stats := row.RawGetString("stats").Table()
	if stats == nil {
		t.Fatal("q_runtime_kernel_execution stats table is nil")
	}
	assertQSQLDataRuntimeKernelStat(t, stats, "LinalgVectorConstruct", "linalg/vector/construct/f64/3", "attempt", 1)
	assertQSQLDataRuntimeKernelStat(t, stats, "LinalgVectorConstruct", "linalg/vector/construct/f64/3", "hit", 1)
	assertQSQLDataRuntimeKernelStat(t, stats, "LinalgVectorScale", "linalg/vector/scale/f64/3", "hit", 1)
	assertQSQLDataRuntimeKernelStat(t, stats, "LinalgMatrixConstruct", "linalg/matrix/construct/f64/2x2", "hit", 1)
	assertQSQLDataRuntimeKernelStat(t, stats, "LinalgMatrixEye", "linalg/matrix/eye/f64/2x2", "hit", 1)
	assertQSQLDataRuntimeKernelStat(t, stats, "LinalgMatrixMatmul", "linalg/matrix/matmul/2x2/f64/2x2", "hit", 1)
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
