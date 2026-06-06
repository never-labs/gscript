package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestDataFrameFromLeiaTablesRowsRoundTrip(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: {"AAPL", "MSFT"},
    qty: {10, 20},
})
n := data.len(f)
cols := data.columns(f)
rows := data.rows(f)
f2 := data.frame(rows)
rows2 := data.rows(f2)
`)

	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("data.len = %v, want 2", got)
	}
	cols := interp.GetGlobal("cols").Table()
	if got := cols.RawGetInt(1); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("cols[1] = %v, want sym", got)
	}
	if got := cols.RawGetInt(2); !got.IsString() || got.Str() != "qty" {
		t.Fatalf("cols[2] = %v, want qty", got)
	}
	assertDataRows(t, interp.GetGlobal("rows").Table())
	assertDataRows(t, interp.GetGlobal("rows2").Table())
}

func TestDataFrameFromDenseArraysRowsRoundTrip(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    qty: []i64{10, 20},
    price: []f64{101.5, 202.25},
    active: []bool{true, false},
})
n := data.len(f)
cols := data.columns(f)
rows := data.rows(f)
f2 := data.frame(rows)
rows2 := data.rows(f2)
`)

	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("data.len = %v, want 2", got)
	}
	cols := interp.GetGlobal("cols").Table()
	if cols.Length() != 3 {
		t.Fatalf("columns length = %d, want 3", cols.Length())
	}
	rows := interp.GetGlobal("rows").Table()
	row1 := rows.RawGetInt(1).Table()
	if got := row1.RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("rows[1].qty = %v, want 10", got)
	}
	if got := row1.RawGetString("price"); !got.IsFloat() || got.Float() != 101.5 {
		t.Fatalf("rows[1].price = %v, want 101.5", got)
	}
	if got := row1.RawGetString("active"); !got.IsBool() || !got.Bool() {
		t.Fatalf("rows[1].active = %v, want true", got)
	}
	row2 := interp.GetGlobal("rows2").Table().RawGetInt(2).Table()
	if got := row2.RawGetString("qty"); !got.IsInt() || got.Int() != 20 {
		t.Fatalf("rows2[2].qty = %v, want 20", got)
	}
	if got := row2.RawGetString("active"); !got.IsBool() || got.Bool() {
		t.Fatalf("rows2[2].active = %v, want false", got)
	}
}

func TestDataFrameFromSoA(t *testing.T) {
	s, err := NewSoA(map[string]*DenseArray{
		"qty":   NewDenseArrayI64([]int64{1, 2}),
		"price": NewDenseArrayF64([]float64{10.5, 20.5}),
	})
	if err != nil {
		t.Fatal(err)
	}
	frame, err := dataFrameValue(SoAValue(s))
	if err != nil {
		t.Fatal(err)
	}
	lib := BuildData()
	rowsFn := lib.RawGetString("rows").GoFunction()
	got, err := rowsFn.Fn([]Value{frame})
	if err != nil {
		t.Fatal(err)
	}
	rows := got[0].Table()
	if rows.Length() != 2 {
		t.Fatalf("rows length = %d, want 2", rows.Length())
	}
	if v := rows.RawGetInt(2).Table().RawGetString("price"); !v.IsFloat() || v.Float() != 20.5 {
		t.Fatalf("rows[2].price = %v, want 20.5", v)
	}
}

func TestDataTypedColumnConstructorsAndKinds(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL", "MSFT", "AAPL"}),
    venue: data.string({"XNAS", "XNYS", "XNAS"}),
    qty: data.i64({10, 15, 20}),
    price: data.f64({100.5, 90, 101.0}),
    active: data.bool({true, nil, false}),
})
n := data.len(f)
kinds := data.kinds(f)
rows := data.rows(f)
`)

	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("data.len = %v, want 3", got)
	}
	kinds := interp.GetGlobal("kinds").Table()
	for name, want := range map[string]string{
		"active": "bool",
		"price":  "f64",
		"qty":    "i64",
		"sym":    "symbol",
		"venue":  "string",
	} {
		if got := kinds.RawGetString(name); !got.IsString() || got.Str() != want {
			t.Fatalf("kinds.%s = %v, want %s", name, got, want)
		}
	}

	rows := interp.GetGlobal("rows").Table()
	row1 := rows.RawGetInt(1).Table()
	if got := row1.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL string", got)
	}
	if got := row1.RawGetString("price"); !got.IsFloat() || got.Float() != 100.5 {
		t.Fatalf("rows[1].price = %v, want 100.5", got)
	}
	row2 := rows.RawGetInt(2).Table()
	if got := row2.RawGetString("active"); !got.IsNil() {
		t.Fatalf("rows[2].active = %v, want nil", got)
	}
	row3 := rows.RawGetInt(3).Table()
	if got := row3.RawGetString("active"); !got.IsBool() || got.Bool() {
		t.Fatalf("rows[3].active = %v, want false", got)
	}

	frame := interp.GetGlobal("f").Table()
	columns := frame.RawGetString("columns").Table()
	if got := columns.RawGetString("sym"); !got.IsTable() || got.Table().RawGetString(dataColumnMarker).Truthy() {
		t.Fatalf("frame columns.sym = %v, want plain table vector", got)
	}
}

func TestDataExtendedNumericColumnConstructorsKindsAndNulls(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    i8v: data.i8({1, data.null}),
    i16v: data.i16({2, data.null}),
    i32v: data.i32({3, data.null}),
    u8v: data.u8({4, data.null}),
    u16v: data.u16({5, data.null}),
    u32v: data.u32({6, data.null}),
    u64v: data.u64({7, data.null}),
    f32v: data.f32({8, data.null}),
})
kinds := data.kinds(f)
rows := data.rows(f)
g := f.gather({2, 1})
gkinds := data.kinds(g)
grows := data.rows(g)
`)

	kinds := interp.GetGlobal("kinds").Table()
	for name, want := range map[string]string{
		"i8v":  "i8",
		"i16v": "i16",
		"i32v": "i32",
		"u8v":  "u8",
		"u16v": "u16",
		"u32v": "u32",
		"u64v": "u64",
		"f32v": "f32",
	} {
		if got := kinds.RawGetString(name); !got.IsString() || got.Str() != want {
			t.Fatalf("kinds.%s = %v, want %s", name, got, want)
		}
	}

	row1 := interp.GetGlobal("rows").Table().RawGetInt(1).Table()
	if got := row1.RawGetString("i32v"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("rows[1].i32v = %v, want 3", got)
	}
	if got := row1.RawGetString("u64v"); !got.IsInt() || got.Int() != 7 {
		t.Fatalf("rows[1].u64v = %v, want 7", got)
	}
	if got := row1.RawGetString("f32v"); !got.IsNumber() {
		t.Fatalf("rows[1].f32v = %v, want number", got)
	}
	row2 := interp.GetGlobal("rows").Table().RawGetInt(2).Table()
	for _, name := range []string{"i8v", "i16v", "i32v", "u8v", "u16v", "u32v", "u64v", "f32v"} {
		if got := row2.RawGetString(name); !got.IsTable() || !isDataNullValue(got) {
			t.Fatalf("rows[2].%s = %v, want data.null sentinel", name, got)
		}
	}

	gkinds := interp.GetGlobal("gkinds").Table()
	if got := gkinds.RawGetString("u64v"); !got.IsString() || got.Str() != "u64" {
		t.Fatalf("gathered kind u64v = %v, want u64", got)
	}
	grow1 := interp.GetGlobal("grows").Table().RawGetInt(1).Table()
	if got := grow1.RawGetString("i32v"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("gather rows[1].i32v = %v, want data.null sentinel", got)
	}
	grow2 := interp.GetGlobal("grows").Table().RawGetInt(2).Table()
	if got := grow2.RawGetString("f32v"); !got.IsNumber() {
		t.Fatalf("gather rows[2].f32v = %v, want number", got)
	}
}

func TestDataSymbolsAndStringColumnsStayDistinct(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL"}),
    text: data.string({"AAPL"}),
})
kinds := data.kinds(f)
rows := data.rows(f)
`)

	kinds := interp.GetGlobal("kinds").Table()
	if got := kinds.RawGetString("sym"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("kinds.sym = %v, want symbol", got)
	}
	if got := kinds.RawGetString("text"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("kinds.text = %v, want string", got)
	}
	row := interp.GetGlobal("rows").Table().RawGetInt(1).Table()
	if got := row.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL string", got)
	}
	if got := row.RawGetString("text"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].text = %v, want AAPL string", got)
	}
}

func TestDataTemporalColumnConstructors(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    d: data.date({"2026-06-06", data.null}),
    tm: data.time({"09:30:00", "16:00:00"}),
    ts: data.timestamp({"2026-06-06T09:30:00Z", data.null}),
})
kinds := data.kinds(f)
rows := data.rows(f)
g := f.gather({2, 1})
gkinds := data.kinds(g)
grows := data.rows(g)
`)

	kinds := interp.GetGlobal("kinds").Table()
	for name, want := range map[string]string{
		"d":  "date",
		"tm": "time",
		"ts": "timestamp",
	} {
		if got := kinds.RawGetString(name); !got.IsString() || got.Str() != want {
			t.Fatalf("kinds.%s = %v, want %s", name, got, want)
		}
	}
	row2 := interp.GetGlobal("rows").Table().RawGetInt(2).Table()
	if got := row2.RawGetString("d"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].d = %v, want data.null sentinel", got)
	}
	if got := row2.RawGetString("ts"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].ts = %v, want data.null sentinel", got)
	}
	gkinds := interp.GetGlobal("gkinds").Table()
	if got := gkinds.RawGetString("d"); !got.IsString() || got.Str() != "date" {
		t.Fatalf("gathered kind d = %v, want date", got)
	}
	grow1 := interp.GetGlobal("grows").Table().RawGetInt(1).Table()
	if got := grow1.RawGetString("tm"); !got.IsString() || got.Str() != "16:00:00" {
		t.Fatalf("gather rows[1].tm = %v, want 16:00:00", got)
	}
}

func TestDataFrameSchemaColumnsRowGatherStableDeclarationOrder(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    z: data.i64({1, 2, 3}),
    a: data.string({"x", "y", "z"}),
    m: data.bool({true, false, true}),
})
cols := data.columns(f)
schemaNames := f.schema.names
schemaKinds := f.schema.kinds
row := f.row(2)
g := f.gather({3, 1})
gcols := data.columns(g)
grows := data.rows(g)
`)

	assertStringArray := func(name string, got *Table, want []string) {
		t.Helper()
		if got.Length() != len(want) {
			t.Fatalf("%s length = %d, want %d", name, got.Length(), len(want))
		}
		for i, expected := range want {
			v := got.RawGetInt(int64(i + 1))
			if !v.IsString() || v.Str() != expected {
				t.Fatalf("%s[%d] = %v, want %s", name, i+1, v, expected)
			}
		}
	}

	wantNames := []string{"z", "a", "m"}
	assertStringArray("data.columns", interp.GetGlobal("cols").Table(), wantNames)
	assertStringArray("schema.names", interp.GetGlobal("schemaNames").Table(), wantNames)
	assertStringArray("gather columns", interp.GetGlobal("gcols").Table(), wantNames)

	schemaKinds := interp.GetGlobal("schemaKinds").Table()
	for name, want := range map[string]string{"z": "i64", "a": "string", "m": "bool"} {
		if got := schemaKinds.RawGetString(name); !got.IsString() || got.Str() != want {
			t.Fatalf("schema.kinds.%s = %v, want %s", name, got, want)
		}
	}
	row := interp.GetGlobal("row").Table()
	if got := row.RawGetString("z"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("row.z = %v, want 2", got)
	}
	if got := row.RawGetString("a"); !got.IsString() || got.Str() != "y" {
		t.Fatalf("row.a = %v, want y", got)
	}
	grow1 := interp.GetGlobal("grows").Table().RawGetInt(1).Table()
	if got := grow1.RawGetString("z"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("gather rows[1].z = %v, want 3", got)
	}
	if got := grow1.RawGetString("a"); !got.IsString() || got.Str() != "z" {
		t.Fatalf("gather rows[1].a = %v, want z", got)
	}
}

func TestDataNullSentinelScriptBehavior(t *testing.T) {
	interp := runWithDataAndArray(t, `
sentinel := data.null
f := data.frame({
    qty: data.i64({10, data.null, 30}),
    label: data.string({"live", data.null, "done"}),
})
rows := data.rows(f)
f2 := data.frame(rows)
rows2 := data.rows(f2)
`)

	if got := interp.GetGlobal("sentinel"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("data.null = %v, want sentinel table", got)
	}
	rows := interp.GetGlobal("rows").Table()
	row2 := rows.RawGetInt(2).Table()
	if got := row2.RawGetString("qty"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].qty = %v, want data.null sentinel", got)
	}
	if got := row2.RawGetString("label"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].label = %v, want data.null sentinel", got)
	}
	row3 := rows.RawGetInt(3).Table()
	if got := row3.RawGetString("qty"); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("rows[3].qty = %v, want 30", got)
	}
	row2Again := interp.GetGlobal("rows2").Table().RawGetInt(2).Table()
	if got := row2Again.RawGetString("qty"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows2[2].qty = %v, want data.null sentinel", got)
	}
}

func assertDataRows(t testing.TB, rows *Table) {
	t.Helper()
	if rows.Length() != 2 {
		t.Fatalf("rows length = %d, want 2", rows.Length())
	}
	row1 := rows.RawGetInt(1).Table()
	if got := row1.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL", got)
	}
	if got := row1.RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("rows[1].qty = %v, want 10", got)
	}
	row2 := rows.RawGetInt(2).Table()
	if got := row2.RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("rows[2].sym = %v, want MSFT", got)
	}
	if got := row2.RawGetString("qty"); !got.IsInt() || got.Int() != 20 {
		t.Fatalf("rows[2].qty = %v, want 20", got)
	}
}

func runWithDataAndArray(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	for name, lib := range map[string]*Table{
		"array": BuildArray(),
		"data":  BuildData(),
	} {
		interp.SetGlobal(name, runtime.TableValue(lib))
		interp.SetModule(name, runtime.TableValue(lib))
	}
	execOnInterp(t, interp, src)
	return interp
}
