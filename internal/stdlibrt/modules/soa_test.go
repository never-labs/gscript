package modules

import (
	"reflect"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestSOAZipShapeDirectAccessAndAggregates(t *testing.T) {
	interp := runWithLib(t, `
points := soa.zip({
    x: []f64{1, 2, 3},
    y: []f64{10, 20, 30},
    id: []i64{101, 102, 103},
})
n := soa.len(points)
cols := soa.columns(points)
shape := soa.shape(points)
xcol := soa.column(points, "x")
sameX := points["x"]
row2 := soa.row(points, 2)
row2.x = 22
beforeWriteback := points.x[2]
ok := soa.setRow(points, 2, row2)
points.y = []f64{100, 200, 300}
points.z = []i64{7, 8, 9}
geMask := soa.mask(points, "x", ">=", 3)
selected := soa.select(points, geMask, "x", 0)
sumSelected := soa.sumSelect(points, geMask, "x", 0)
selectedIdx := soa.indicesWhere(points, geMask)
maskedStats := soa.statsWhere(points, "x", []bool{true, false, true})
`, "soa", BuildSOA())

	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("soa.len = %v, want 3", got)
	}
	if got := interp.GetGlobal("cols").Table().RawGetInt(1); !got.IsString() || got.Str() != "id" {
		t.Fatalf("cols[1] = %v, want id", got)
	}
	if got := interp.GetGlobal("shape").Table().RawGetString("length"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("shape.length = %v, want 3", got)
	}
	assertSOADenseF64(t, interp.GetGlobal("xcol"), []float64{1, 22, 3})
	assertSOADenseF64(t, interp.GetGlobal("sameX"), []float64{1, 22, 3})
	if got := interp.GetGlobal("beforeWriteback"); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("beforeWriteback = %v, want 2", got)
	}
	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("setRow result = %v, want true", got)
	}
	assertSOADenseF64(t, interp.GetGlobal("selected"), []float64{0, 22, 3})
	if got := interp.GetGlobal("sumSelected"); !got.IsFloat() || got.Float() != 25 {
		t.Fatalf("sumSelected = %v, want 25", got)
	}
	assertSOADenseI64(t, interp.GetGlobal("selectedIdx"), []int64{2, 3})
	stats := interp.GetGlobal("maskedStats").Table()
	if got := stats.RawGetString("count"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("maskedStats.count = %v, want 2", got)
	}
	points := interp.GetGlobal("points").SoA()
	assertSOADenseF64(t, DenseArrayValue(mustSOATestColumn(t, points, "y")), []float64{100, 200, 300})
	assertSOADenseI64(t, DenseArrayValue(mustSOATestColumn(t, points, "z")), []int64{7, 8, 9})
}

func TestSOAHotBuiltinsExposeFastArgPaths(t *testing.T) {
	lib := BuildSOA()
	for _, name := range []string{
		"len", "column", "withColumn", "dropColumn", "resize", "appendRow",
		"fill", "fillWhere", "unzip", "slice", "filter", "compact", "gather",
		"indicesWhere", "scatterInto", "addScaled", "affine", "affineWhere",
		"affineMany", "sum", "stats", "mask", "select", "selectInto", "sumSelect",
	} {
		fn := lib.RawGetString(name).GoFunction()
		if fn == nil {
			t.Fatalf("soa.%s is not a GoFunction", name)
		}
		if fn.FastArg1 == nil && fn.FastArg2 == nil && fn.FastArg3 == nil &&
			fn.FastArg4 == nil && fn.FastArg5 == nil && fn.FastArg6 == nil {
			t.Fatalf("soa.%s has no fast-arg path", name)
		}
	}
	affineMany := lib.RawGetString("affineMany").GoFunction()
	if !runtime.IsStdSoAAffineManyFunction(FunctionValue(affineMany)) {
		t.Fatalf("soa.affineMany is missing native identity marker")
	}
}

func TestSOAAffineManyFastPathUsesRuntimePlans(t *testing.T) {
	lib := BuildSOA()
	fn := lib.RawGetString("affineMany").GoFunction()
	if fn == nil || fn.FastArg2 == nil {
		t.Fatal("soa.affineMany fast path missing")
	}
	s, err := NewSoA(map[string]*DenseArray{
		"x": NewDenseArrayF64([]float64{1, 2, 3}),
		"y": NewDenseArrayF64([]float64{0, 0, 0}),
		"v": NewDenseArrayF64([]float64{10, 20, 30}),
	})
	if err != nil {
		t.Fatal(err)
	}
	terms := NewTable()
	term1 := NewTable()
	term1.RawSetString("dst", StringValue("x"))
	term1.RawSetString("src", StringValue("v"))
	term1.RawSetString("scale", FloatValue(0.5))
	term1.RawSetString("bias", FloatValue(1))
	term2 := NewTable()
	term2.RawSetString("dst", StringValue("y"))
	term2.RawSetString("src", StringValue("v"))
	term2.RawSetString("scale", FloatValue(2))
	term2.RawSetString("bias", FloatValue(0))
	terms.RawSetInt(1, TableValue(term1))
	terms.RawSetInt(2, TableValue(term2))
	if got, err := fn.FastArg2(SoAValue(s), TableValue(terms)); err != nil || !got.Bool() {
		t.Fatalf("soa.affineMany fast path = %v, %v; want true, nil", got, err)
	}
	assertSOADenseF64(t, DenseArrayValue(mustSOATestColumn(t, s, "x")), []float64{6, 11, 16})
	assertSOADenseF64(t, DenseArrayValue(mustSOATestColumn(t, s, "y")), []float64{20, 40, 60})
}

func assertSOADenseF64(t *testing.T, v Value, want []float64) {
	t.Helper()
	if !v.IsDenseArray() || v.DenseArray().DType() != DenseArrayF64 {
		t.Fatalf("value = %v, want f64 dense array", v)
	}
	got, _ := v.DenseArray().F64()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dense f64 = %v, want %v", got, want)
	}
}

func assertSOADenseI64(t *testing.T, v Value, want []int64) {
	t.Helper()
	if !v.IsDenseArray() || v.DenseArray().DType() != DenseArrayI64 {
		t.Fatalf("value = %v, want i64 dense array", v)
	}
	got, _ := v.DenseArray().I64()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dense i64 = %v, want %v", got, want)
	}
}

func mustSOATestColumn(t testing.TB, s *SoA, name string) *DenseArray {
	t.Helper()
	col, ok := s.Column(name)
	if !ok {
		t.Fatalf("missing soa column %q", name)
	}
	return col
}
