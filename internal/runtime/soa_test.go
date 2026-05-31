package runtime

import (
	"reflect"
	"strings"
	"testing"
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

func mustSoATestColumn(t testing.TB, s *SoA, name string) *DenseArray {
	t.Helper()
	col, ok := s.Column(name)
	if !ok {
		t.Fatalf("missing soa column %q", name)
	}
	return col
}
