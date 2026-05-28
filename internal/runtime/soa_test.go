package runtime

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gscript/gscript/internal/lexer"
	"github.com/gscript/gscript/internal/parser"
)

func TestSoABasics(t *testing.T) {
	s, err := NewSoA(map[string]*DenseArray{
		"x": NewDenseArrayF64([]float64{1, 2}),
		"y": NewDenseArrayF64([]float64{10, 20}),
	})
	if err != nil {
		t.Fatal(err)
	}
	v := SoAValue(s)
	if v.Type() != TypeSoA || v.TypeName() != "soa" || !v.IsSoA() || v.SoA() != s {
		t.Fatalf("SoA value access failed: type=%v name=%s value=%v", v.Type(), v.TypeName(), v)
	}
	if got := s.ColumnNames(); !reflect.DeepEqual(got, []string{"x", "y"}) {
		t.Fatalf("ColumnNames = %v", got)
	}
	if got := s.String(); got != "soa{x, y}[2]" {
		t.Fatalf("String() = %q", got)
	}
}

func TestSoARejectsLengthMismatch(t *testing.T) {
	_, err := NewSoA(map[string]*DenseArray{
		"x": NewDenseArrayF64([]float64{1, 2}),
		"y": NewDenseArrayF64([]float64{10}),
	})
	if err == nil || !strings.Contains(err.Error(), "length mismatch") {
		t.Fatalf("error = %v, want length mismatch", err)
	}
}

func TestSoARowAndSetRow(t *testing.T) {
	s, err := NewSoA(map[string]*DenseArray{
		"alive": NewDenseArrayBool([]bool{true, false}),
		"x":     NewDenseArrayF64([]float64{1, 2}),
		"id":    NewDenseArrayI64([]int64{7, 8}),
	})
	if err != nil {
		t.Fatal(err)
	}
	row, err := s.Row(1)
	if err != nil {
		t.Fatal(err)
	}
	if got := row.RawGetString("x"); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("row.x = %v, want 2", got)
	}
	if got := row.RawGetString("id"); !got.IsInt() || got.Int() != 8 {
		t.Fatalf("row.id = %v, want 8", got)
	}
	row.RawSetString("x", FloatValue(22))
	row.RawSetString("id", IntValue(88))
	row.RawSetString("alive", BoolValue(true))
	if err := s.SetRow(1, row); err != nil {
		t.Fatal(err)
	}
	x, _ := s.Column("x")
	id, _ := s.Column("id")
	alive, _ := s.Column("alive")
	assertDenseF64(t, DenseArrayValue(x), []float64{1, 22})
	assertDenseI64(t, DenseArrayValue(id), []int64{7, 88})
	assertDenseBool(t, DenseArrayValue(alive), []bool{true, true})
}

func TestSoAStdlibZipColumnRow(t *testing.T) {
	interp := New()
	if err := runSource(interp, `
xs := []f64{1, 2, 3}
ys := xs * 10
points := soa.zip({x: xs, y: ys})
n := soa.len(points)
cols := soa.columns(points)
withZ := soa.withColumn(points, "z", []i64{7, 8, 9})
droppedY := soa.dropColumn(withZ, "y")
unzipped := soa.unzip(points)
xcol := soa.column(points, "x")
row := soa.row(points, 2)
row.x = 42
ok := soa.setRow(points, 2, row)
updated := soa.column(points, "x")
window := soa.slice(points, 2, 3)
filtered := soa.filter(points, []bool{true, false, true})
compacted := soa.compact(points, []bool{false, true, true})
gathered := soa.gather(points, []i64{3, 1})
maskedSum := soa.sumWhere(points, "x", []bool{true, false, true})
maskedMin := soa.minWhere(points, "x", []bool{true, false, true})
maskedMean := soa.meanWhere(points, "x", []bool{true, false, true})
maskedMax := soa.maxWhere(points, "x", []bool{true, false, true})
maskedStats := soa.statsWhere(points, "x", []bool{true, false, true})
maskedCount := soa.countWhere(points, []bool{true, false, true})
`); err != nil {
		t.Fatal(err)
	}
	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("n = %v, want 3", got)
	}
	if got := interp.GetGlobal("cols").Table().RawGetInt(1); !got.IsString() || got.Str() != "x" {
		t.Fatalf("cols[1] = %v, want x", got)
	}
	if got := interp.GetGlobal("withZ"); !got.IsSoA() || got.SoA().Len() != 3 {
		t.Fatalf("withZ = %v, want SoA length 3", got)
	}
	assertDenseI64(t, DenseArrayValue(mustSoATestColumn(t, interp.GetGlobal("withZ").SoA(), "z")), []int64{7, 8, 9})
	if _, ok := interp.GetGlobal("droppedY").SoA().Column("y"); ok {
		t.Fatal("droppedY still has y column")
	}
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, interp.GetGlobal("droppedY").SoA(), "x")), []float64{1, 42, 3})
	assertDenseF64(t, interp.GetGlobal("unzipped").Table().RawGetString("y"), []float64{10, 20, 30})
	assertDenseF64(t, interp.GetGlobal("xcol"), []float64{1, 42, 3})
	if got := interp.GetGlobal("row").Table().RawGetString("y"); !got.IsFloat() || got.Float() != 20 {
		t.Fatalf("row.y = %v, want 20", got)
	}
	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok = %v, want true", got)
	}
	assertDenseF64(t, interp.GetGlobal("updated"), []float64{1, 42, 3})
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, interp.GetGlobal("window").SoA(), "x")), []float64{42, 3})
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, interp.GetGlobal("filtered").SoA(), "x")), []float64{1, 3})
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, interp.GetGlobal("compacted").SoA(), "x")), []float64{42, 3})
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, interp.GetGlobal("gathered").SoA(), "x")), []float64{3, 1})
	if got := interp.GetGlobal("maskedSum"); !got.IsFloat() || got.Float() != 4 {
		t.Fatalf("maskedSum = %v, want 4", got)
	}
	if got := interp.GetGlobal("maskedMin"); !got.IsFloat() || got.Float() != 1 {
		t.Fatalf("maskedMin = %v, want 1", got)
	}
	if got := interp.GetGlobal("maskedMean"); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("maskedMean = %v, want 2", got)
	}
	if got := interp.GetGlobal("maskedMax"); !got.IsFloat() || got.Float() != 3 {
		t.Fatalf("maskedMax = %v, want 3", got)
	}
	stats := interp.GetGlobal("maskedStats").Table()
	if got := stats.RawGetString("count"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("maskedStats.count = %v, want 2", got)
	}
	if got := stats.RawGetString("mean"); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("maskedStats.mean = %v, want 2", got)
	}
	if got := interp.GetGlobal("maskedCount"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("maskedCount = %v, want 2", got)
	}
}

func TestSoANativeColumnKernels(t *testing.T) {
	interp := New()
	if err := runSource(interp, `
xs := []f64{1, 2, 3}
vx := []f64{10, 20, 30}
vy := []f64{1, 2, 3}
points := soa.zip({x: xs, vx: vx, y: []f64{0, 0, 0}, vy: vy})
ok1 := soa.addScaled(points, "x", "vx", 0.5)
sum1 := soa.sum(points, "x")
ok2 := soa.affine(points, "x", "vx", 2, 1)
sum2 := soa.sum(points, "x")
okWhere := soa.affineWhere(points, "x", "vx", 0.1, []bool{true, false, true}, 1)
sumWhereUpdate := soa.sum(points, "x")
ok3 := soa.affineMany(points, {
    {dst: "x", src: "vx", scale: 0.25, bias: 0.5},
    {dst: "y", src: "vy", scale: 10, bias: 1},
})
sum3 := soa.sum(points, "x") + soa.sum(points, "y")
shape := soa.shape(points)
`); err != nil {
		t.Fatal(err)
	}
	if got := interp.GetGlobal("ok1"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok1 = %v, want true", got)
	}
	if got := interp.GetGlobal("sum1"); !got.IsFloat() || got.Float() != 36 {
		t.Fatalf("sum1 = %v, want 36", got)
	}
	if got := interp.GetGlobal("ok2"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok2 = %v, want true", got)
	}
	if got := interp.GetGlobal("sum2"); !got.IsFloat() || got.Float() != 123 {
		t.Fatalf("sum2 = %v, want 123", got)
	}
	if got := interp.GetGlobal("okWhere"); !got.IsBool() || !got.Bool() {
		t.Fatalf("okWhere = %v, want true", got)
	}
	if got := interp.GetGlobal("sumWhereUpdate"); !got.IsFloat() || got.Float() != 47 {
		t.Fatalf("sumWhereUpdate = %v, want 47", got)
	}
	if got := interp.GetGlobal("ok3"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok3 = %v, want true", got)
	}
	if got := interp.GetGlobal("sum3"); !got.IsFloat() || got.Float() != 79.5 {
		t.Fatalf("sum3 = %v, want 79.5", got)
	}
	shape := interp.GetGlobal("shape").Table()
	if got := shape.RawGetString("length"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("shape.length = %v, want 3", got)
	}
}

func TestSoASnapshotAndAffineManyGuards(t *testing.T) {
	s, err := NewSoA(map[string]*DenseArray{
		"x":  NewDenseArrayF64([]float64{1, 2}),
		"y":  NewDenseArrayF64([]float64{3, 4}),
		"vx": NewDenseArrayF64([]float64{10, 20}),
		"vy": NewDenseArrayF64([]float64{30, 40}),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.Snapshot("x", "y", "vx", "vy")
	if err != nil {
		t.Fatal(err)
	}
	if !s.ValidateSnapshot(snapshot) {
		t.Fatal("fresh SoA snapshot did not validate")
	}
	if err := s.AffineMany([]SoAAffineTerm{
		{Dst: "x", Src: "vx", Scale: 0.5, Bias: 1},
		{Dst: "y", Src: "vy", Scale: 0.25, Bias: -1},
	}); err != nil {
		t.Fatal(err)
	}
	if s.ValidateSnapshot(snapshot) {
		t.Fatal("snapshot should be invalid after fused column writes")
	}
	x, _ := s.Column("x")
	y, _ := s.Column("y")
	assertDenseF64(t, DenseArrayValue(x), []float64{6, 11})
	assertDenseF64(t, DenseArrayValue(y), []float64{6.5, 9})

	err = s.AffineMany([]SoAAffineTerm{
		{Dst: "x", Src: "vx", Scale: 1, Bias: 0},
		{Dst: "vx", Src: "y", Scale: 1, Bias: 0},
	})
	if err == nil || !strings.Contains(err.Error(), "also written") {
		t.Fatalf("dependent affineMany error = %v, want dependency rejection", err)
	}
}

func TestSoASliceFilterAndUnzipCopyColumns(t *testing.T) {
	s, err := NewSoA(map[string]*DenseArray{
		"x":     NewDenseArrayF64([]float64{1, 2, 3, 4}),
		"id":    NewDenseArrayI64([]int64{10, 20, 30, 40}),
		"alive": NewDenseArrayBool([]bool{true, false, true, false}),
	})
	if err != nil {
		t.Fatal(err)
	}
	window, err := s.Slice(1, 3)
	if err != nil {
		t.Fatal(err)
	}
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, window, "x")), []float64{2, 3})
	assertDenseI64(t, DenseArrayValue(mustSoATestColumn(t, window, "id")), []int64{20, 30})

	mask, _ := s.Column("alive")
	filtered, err := s.Filter(mask)
	if err != nil {
		t.Fatal(err)
	}
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, filtered, "x")), []float64{1, 3})
	assertDenseI64(t, DenseArrayValue(mustSoATestColumn(t, filtered, "id")), []int64{10, 30})

	compacted, err := s.Compact(mask)
	if err != nil {
		t.Fatal(err)
	}
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, compacted, "x")), []float64{1, 3})

	gathered, err := s.Gather(NewDenseArrayI64([]int64{4, 1, 3}))
	if err != nil {
		t.Fatal(err)
	}
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, gathered, "x")), []float64{4, 1, 3})
	assertDenseI64(t, DenseArrayValue(mustSoATestColumn(t, gathered, "id")), []int64{40, 10, 30})

	sum, err := s.SumWhere("x", mask)
	if err != nil {
		t.Fatal(err)
	}
	if !sum.IsFloat() || sum.Float() != 4 {
		t.Fatalf("SumWhere = %s, want 4", sum.String())
	}
	min, err := s.MinWhere("x", mask)
	if err != nil {
		t.Fatal(err)
	}
	if !min.IsFloat() || min.Float() != 1 {
		t.Fatalf("MinWhere = %s, want 1", min.String())
	}
	mean, err := s.MeanWhere("x", mask)
	if err != nil {
		t.Fatal(err)
	}
	if !mean.IsFloat() || mean.Float() != 2 {
		t.Fatalf("MeanWhere = %s, want 2", mean.String())
	}
	max, err := s.MaxWhere("x", mask)
	if err != nil {
		t.Fatal(err)
	}
	if !max.IsFloat() || max.Float() != 3 {
		t.Fatalf("MaxWhere = %s, want 3", max.String())
	}
	stats, err := s.StatsWhere("x", mask)
	if err != nil {
		t.Fatal(err)
	}
	if got := stats.RawGetString("sum"); !got.IsFloat() || got.Float() != 4 {
		t.Fatalf("StatsWhere.sum = %s, want 4", got.String())
	}
	if got := stats.RawGetString("mean"); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("StatsWhere.mean = %s, want 2", got.String())
	}
	count, err := s.CountWhere(mask)
	if err != nil {
		t.Fatal(err)
	}
	if !count.IsInt() || count.Int() != 2 {
		t.Fatalf("CountWhere = %s, want 2", count.String())
	}

	unzipped, err := s.Unzip()
	if err != nil {
		t.Fatal(err)
	}
	if err := unzipped["x"].Set(0, FloatValue(99)); err != nil {
		t.Fatal(err)
	}
	assertDenseF64(t, DenseArrayValue(mustSoATestColumn(t, s, "x")), []float64{1, 2, 3, 4})
}

func TestSoAHotBuiltinsExposeFastArgPaths(t *testing.T) {
	lib := buildSoALib()
	lenFn := lib.RawGetString("len").GoFunction()
	column := lib.RawGetString("column").GoFunction()
	withColumn := lib.RawGetString("withColumn").GoFunction()
	dropColumn := lib.RawGetString("dropColumn").GoFunction()
	unzip := lib.RawGetString("unzip").GoFunction()
	slice := lib.RawGetString("slice").GoFunction()
	filter := lib.RawGetString("filter").GoFunction()
	compact := lib.RawGetString("compact").GoFunction()
	gather := lib.RawGetString("gather").GoFunction()
	addScaled := lib.RawGetString("addScaled").GoFunction()
	affine := lib.RawGetString("affine").GoFunction()
	affineWhere := lib.RawGetString("affineWhere").GoFunction()
	affineMany := lib.RawGetString("affineMany").GoFunction()
	sum := lib.RawGetString("sum").GoFunction()
	sumWhere := lib.RawGetString("sumWhere").GoFunction()
	minWhere := lib.RawGetString("minWhere").GoFunction()
	meanWhere := lib.RawGetString("meanWhere").GoFunction()
	maxWhere := lib.RawGetString("maxWhere").GoFunction()
	statsWhere := lib.RawGetString("statsWhere").GoFunction()
	countWhere := lib.RawGetString("countWhere").GoFunction()
	if lenFn == nil || lenFn.FastArg1 == nil {
		t.Fatal("soa.len FastArg1 is nil")
	}
	if column == nil || column.FastArg2 == nil {
		t.Fatal("soa.column FastArg2 is nil")
	}
	if withColumn == nil || withColumn.FastArg3 == nil {
		t.Fatal("soa.withColumn FastArg3 is nil")
	}
	if dropColumn == nil || dropColumn.FastArg2 == nil {
		t.Fatal("soa.dropColumn FastArg2 is nil")
	}
	if unzip == nil || unzip.FastArg1 == nil {
		t.Fatal("soa.unzip FastArg1 is nil")
	}
	if slice == nil || slice.FastArg3 == nil {
		t.Fatal("soa.slice FastArg3 is nil")
	}
	if filter == nil || filter.FastArg2 == nil {
		t.Fatal("soa.filter FastArg2 is nil")
	}
	if compact == nil || compact.FastArg2 == nil {
		t.Fatal("soa.compact FastArg2 is nil")
	}
	if gather == nil || gather.FastArg2 == nil {
		t.Fatal("soa.gather FastArg2 is nil")
	}
	if addScaled == nil || addScaled.FastArg4 == nil {
		t.Fatal("soa.addScaled FastArg4 is nil")
	}
	if affine == nil || affine.FastArg5 == nil {
		t.Fatal("soa.affine FastArg5 is nil")
	}
	if affineWhere == nil || affineWhere.FastArg6 == nil {
		t.Fatal("soa.affineWhere FastArg6 is nil")
	}
	if affineMany == nil || affineMany.FastArg2 == nil {
		t.Fatal("soa.affineMany FastArg2 is nil")
	}
	if !IsStdSoAAffineManyFunction(FunctionValue(affineMany)) {
		t.Fatal("soa.affineMany native identity is missing")
	}
	if sum == nil || sum.FastArg2 == nil {
		t.Fatal("soa.sum FastArg2 is nil")
	}
	if sumWhere == nil || sumWhere.FastArg3 == nil {
		t.Fatal("soa.sumWhere FastArg3 is nil")
	}
	if minWhere == nil || minWhere.FastArg3 == nil {
		t.Fatal("soa.minWhere FastArg3 is nil")
	}
	if meanWhere == nil || meanWhere.FastArg3 == nil {
		t.Fatal("soa.meanWhere FastArg3 is nil")
	}
	if maxWhere == nil || maxWhere.FastArg3 == nil {
		t.Fatal("soa.maxWhere FastArg3 is nil")
	}
	if statsWhere == nil || statsWhere.FastArg3 == nil {
		t.Fatal("soa.statsWhere FastArg3 is nil")
	}
	if countWhere == nil || countWhere.FastArg2 == nil {
		t.Fatal("soa.countWhere FastArg2 is nil")
	}

	s, err := NewSoA(map[string]*DenseArray{
		"x": NewDenseArrayF64([]float64{1, 2, 3}),
		"v": NewDenseArrayF64([]float64{10, 20, 30}),
	})
	if err != nil {
		t.Fatal(err)
	}
	soaValue := SoAValue(s)
	if got, err := lenFn.FastArg1(soaValue); err != nil || got.Int() != 3 {
		t.Fatalf("soa.len FastArg1 got=%s err=%v", got.String(), err)
	}
	if got, err := column.FastArg2(soaValue, StringValue("x")); err != nil || !got.IsDenseArray() {
		t.Fatalf("soa.column FastArg2 got=%s err=%v", got.String(), err)
	}
	if got, err := withColumn.FastArg3(soaValue, StringValue("id"), DenseArrayValue(NewDenseArrayI64([]int64{1, 2, 3}))); err != nil || !got.IsSoA() {
		t.Fatalf("soa.withColumn FastArg3 got=%s err=%v", got.String(), err)
	}
	if got, err := dropColumn.FastArg2(soaValue, StringValue("v")); err != nil || !got.IsSoA() {
		t.Fatalf("soa.dropColumn FastArg2 got=%s err=%v", got.String(), err)
	}
	if got, err := unzip.FastArg1(soaValue); err != nil || !got.IsTable() {
		t.Fatalf("soa.unzip FastArg1 got=%s err=%v", got.String(), err)
	}
	if got, err := slice.FastArg3(soaValue, IntValue(2), IntValue(3)); err != nil || !got.IsSoA() || got.SoA().Len() != 2 {
		t.Fatalf("soa.slice FastArg3 got=%s err=%v", got.String(), err)
	}
	if got, err := filter.FastArg2(soaValue, DenseArrayValue(NewDenseArrayBool([]bool{true, false, true}))); err != nil || !got.IsSoA() || got.SoA().Len() != 2 {
		t.Fatalf("soa.filter FastArg2 got=%s err=%v", got.String(), err)
	}
	if got, err := compact.FastArg2(soaValue, DenseArrayValue(NewDenseArrayBool([]bool{false, true, true}))); err != nil || !got.IsSoA() || got.SoA().Len() != 2 {
		t.Fatalf("soa.compact FastArg2 got=%s err=%v", got.String(), err)
	}
	if got, err := gather.FastArg2(soaValue, DenseArrayValue(NewDenseArrayI64([]int64{3, 1}))); err != nil || !got.IsSoA() || got.SoA().Len() != 2 {
		t.Fatalf("soa.gather FastArg2 got=%s err=%v", got.String(), err)
	}
	if got, err := addScaled.FastArg4(soaValue, StringValue("x"), StringValue("v"), FloatValue(0.5)); err != nil || !got.Bool() {
		t.Fatalf("soa.addScaled FastArg4 got=%s err=%v", got.String(), err)
	}
	if got, err := affine.FastArg5(soaValue, StringValue("v"), StringValue("x"), FloatValue(2), FloatValue(1)); err != nil || !got.Bool() {
		t.Fatalf("soa.affine FastArg5 got=%s err=%v", got.String(), err)
	}
	if got, err := affineWhere.FastArg6(soaValue, StringValue("v"), StringValue("x"), FloatValue(0.5), DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})), FloatValue(1)); err != nil || !got.Bool() {
		t.Fatalf("soa.affineWhere FastArg6 got=%s err=%v", got.String(), err)
	}
	terms := NewTable()
	term := NewTable()
	term.RawSetString("dst", StringValue("x"))
	term.RawSetString("src", StringValue("v"))
	term.RawSetString("scale", FloatValue(0.1))
	term.RawSetString("bias", FloatValue(0))
	terms.RawSetInt(1, TableValue(term))
	if got, err := affineMany.FastArg2(soaValue, TableValue(terms)); err != nil || !got.Bool() {
		t.Fatalf("soa.affineMany FastArg2 got=%s err=%v", got.String(), err)
	}
	got, err := sum.FastArg2(soaValue, StringValue("v"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number() != 39 {
		t.Fatalf("soa.sum FastArg2 = %s, want 39", got.String())
	}
	got, err = sumWhere.FastArg3(soaValue, StringValue("v"), DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number() != 14 {
		t.Fatalf("soa.sumWhere FastArg3 = %s, want 14", got.String())
	}
	got, err = minWhere.FastArg3(soaValue, StringValue("v"), DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number() != 4 {
		t.Fatalf("soa.minWhere FastArg3 = %s, want 4", got.String())
	}
	got, err = meanWhere.FastArg3(soaValue, StringValue("v"), DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number() != 7 {
		t.Fatalf("soa.meanWhere FastArg3 = %s, want 7", got.String())
	}
	got, err = maxWhere.FastArg3(soaValue, StringValue("v"), DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number() != 10 {
		t.Fatalf("soa.maxWhere FastArg3 = %s, want 10", got.String())
	}
	got, err = statsWhere.FastArg3(soaValue, StringValue("v"), DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Table().RawGetString("mean").Number() != 7 {
		t.Fatalf("soa.statsWhere FastArg3 mean = %s, want 7", got.Table().RawGetString("mean").String())
	}
	got, err = countWhere.FastArg2(soaValue, DenseArrayValue(NewDenseArrayBool([]bool{true, false, true})))
	if err != nil {
		t.Fatal(err)
	}
	if got.Int() != 2 {
		t.Fatalf("soa.countWhere FastArg2 = %s, want 2", got.String())
	}
}

func BenchmarkDenseArrayFilterF64(b *testing.B) {
	xs := make([]float64, 32768)
	mask := make([]bool, len(xs))
	for i := range xs {
		xs[i] = float64(i)
		mask[i] = i%2 == 0
	}
	col := NewDenseArrayF64(xs)
	maskCol := NewDenseArrayBool(mask)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		out, err := col.Filter(maskCol)
		if err != nil {
			b.Fatal(err)
		}
		if out.Len() != len(xs)/2 {
			b.Fatalf("filter len = %d, want %d", out.Len(), len(xs)/2)
		}
	}
}

func runSource(interp *Interpreter, src string) error {
	tokens, err := lexer.New(src).Tokenize()
	if err != nil {
		return err
	}
	prog, err := parser.New(tokens).Parse()
	if err != nil {
		return err
	}
	return interp.Exec(prog)
}

func mustSoATestColumn(t testing.TB, s *SoA, name string) *DenseArray {
	t.Helper()
	col, ok := s.Column(name)
	if !ok {
		t.Fatalf("missing soa column %q", name)
	}
	return col
}
