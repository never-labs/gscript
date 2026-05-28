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
xcol := soa.column(points, "x")
row := soa.row(points, 2)
row.x = 42
ok := soa.setRow(points, 2, row)
updated := soa.column(points, "x")
`); err != nil {
		t.Fatal(err)
	}
	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("n = %v, want 3", got)
	}
	if got := interp.GetGlobal("cols").Table().RawGetInt(1); !got.IsString() || got.Str() != "x" {
		t.Fatalf("cols[1] = %v, want x", got)
	}
	assertDenseF64(t, interp.GetGlobal("xcol"), []float64{1, 42, 3})
	if got := interp.GetGlobal("row").Table().RawGetString("y"); !got.IsFloat() || got.Float() != 20 {
		t.Fatalf("row.y = %v, want 20", got)
	}
	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("ok = %v, want true", got)
	}
	assertDenseF64(t, interp.GetGlobal("updated"), []float64{1, 42, 3})
}

func TestSoANativeColumnKernels(t *testing.T) {
	interp := New()
	if err := runSource(interp, `
xs := []f64{1, 2, 3}
vx := []f64{10, 20, 30}
points := soa.zip({x: xs, vx: vx})
ok1 := soa.addScaled(points, "x", "vx", 0.5)
sum1 := soa.sum(points, "x")
ok2 := soa.affine(points, "x", "vx", 2, 1)
sum2 := soa.sum(points, "x")
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
}

func TestSoAHotBuiltinsExposeFastArgPaths(t *testing.T) {
	lib := buildSoALib()
	addScaled := lib.RawGetString("addScaled").GoFunction()
	affine := lib.RawGetString("affine").GoFunction()
	sum := lib.RawGetString("sum").GoFunction()
	if addScaled == nil || addScaled.FastArg4 == nil {
		t.Fatal("soa.addScaled FastArg4 is nil")
	}
	if affine == nil || affine.FastArg5 == nil {
		t.Fatal("soa.affine FastArg5 is nil")
	}
	if sum == nil || sum.FastArg2 == nil {
		t.Fatal("soa.sum FastArg2 is nil")
	}

	s, err := NewSoA(map[string]*DenseArray{
		"x": NewDenseArrayF64([]float64{1, 2, 3}),
		"v": NewDenseArrayF64([]float64{10, 20, 30}),
	})
	if err != nil {
		t.Fatal(err)
	}
	soaValue := SoAValue(s)
	if got, err := addScaled.FastArg4(soaValue, StringValue("x"), StringValue("v"), FloatValue(0.5)); err != nil || !got.Bool() {
		t.Fatalf("soa.addScaled FastArg4 got=%s err=%v", got.String(), err)
	}
	if got, err := affine.FastArg5(soaValue, StringValue("v"), StringValue("x"), FloatValue(2), FloatValue(1)); err != nil || !got.Bool() {
		t.Fatalf("soa.affine FastArg5 got=%s err=%v", got.String(), err)
	}
	got, err := sum.FastArg2(soaValue, StringValue("v"))
	if err != nil {
		t.Fatal(err)
	}
	if got.Number() != 75 {
		t.Fatalf("soa.sum FastArg2 = %s, want 75", got.String())
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
