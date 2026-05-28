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
