package bind

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	stddata "github.com/never-labs/leia/internal/stdlib/lib/data"
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

func TestDataFrameValueExposesUnifiedRowsAndFields(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL", "MSFT"}),
    qty: data.i64({10, 20}),
})
n := #f
first := f[1]
rows := f.rows
apiRows := data.rows(f)
symColumn := f.sym
dataColumns := f.data
`)

	if got := interp.GetGlobal("n"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("#f = %v, want 2", got)
	}
	first := interp.GetGlobal("first").Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("f[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("f[1].qty = %v, want 10", got)
	}
	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("f.rows len = %v, want 2", rows)
	}
	apiRows := interp.GetGlobal("apiRows").Table()
	if apiRows == nil || apiRows.Length() != 2 {
		t.Fatalf("data.rows(f) len = %v, want 2", apiRows)
	}
	if got := interp.GetGlobal("symColumn").Table().RawGetInt(2); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("f.sym[2] = %v, want MSFT", got)
	}
	dataColumns := interp.GetGlobal("dataColumns").Table()
	if dataColumns == nil || !dataColumns.RawGetString("qty").IsTable() {
		t.Fatalf("f.data.qty = %v, want column table", dataColumns.RawGetString("qty"))
	}
}

func TestDataFrameKeepsNativeFramePayloadForRoundTrip(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL", "MSFT"}),
    qty: data.i64({10, 20}),
})
`)
	frame := interp.GetGlobal("f").Table()
	if _, ok := frame.NativePayload().(stddata.Frame); !ok {
		t.Fatalf("data frame native payload = %T, want stddata.Frame", frame.NativePayload())
	}
	libFrame, err := dataLibFrameFromTable(frame)
	if err != nil {
		t.Fatalf("dataLibFrameFromTable: %v", err)
	}
	if libFrame.Len() != 2 {
		t.Fatalf("lib frame len = %d, want 2", libFrame.Len())
	}
	info, ok := frame.NativeFramePayloadInfo()
	if !ok {
		t.Fatal("data frame native frame payload info missing")
	}
	if info.Kind != NativePayloadDataFrame || info.Rows != 2 || info.Columns != 2 || info.SchemaHash != libFrame.SchemaFingerprint() {
		t.Fatalf("data frame native payload info = %#v, want frame rows=2 columns=2 schema=%s", info, libFrame.SchemaFingerprint())
	}
	if _, ok := libFrame.Column("sym"); !ok {
		t.Fatal("lib frame missing sym column")
	}
	if _, ok := libFrame.Column("qty"); !ok {
		t.Fatal("lib frame missing qty column")
	}
}

func TestDataLibFrameFromTableUsesNativeFramePayloadInfoWithoutMarker(t *testing.T) {
	frame, err := stddata.NewFrame(
		stddata.Column{Name: "sym", Data: stddata.NewSymbols([]string{"AAPL"})},
		stddata.Column{Name: "qty", Data: stddata.NewI64([]int64{10})},
	)
	if err != nil {
		t.Fatal(err)
	}
	table := NewTable()
	table.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})

	roundTrip, err := dataLibFrameFromTable(table)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Len() != 1 {
		t.Fatalf("round-trip len = %d, want 1", roundTrip.Len())
	}
	if _, ok := roundTrip.Column("qty"); !ok {
		t.Fatal("round-trip frame missing qty column")
	}
}

func TestDataFrameTablePrefersNativeFramePayloadKindOverMarker(t *testing.T) {
	frame, err := stddata.NewFrame(
		stddata.Column{Name: "sym", Data: stddata.NewSymbols([]string{"AAPL"})},
		stddata.Column{Name: "qty", Data: stddata.NewI64([]int64{10})},
	)
	if err != nil {
		t.Fatal(err)
	}
	keyed, err := stddata.KeyBy(frame, "sym")
	if err != nil {
		t.Fatal(err)
	}
	table := NewTable()
	table.RawSetString(dataFrameMarker, BoolValue(true))
	table.SetNativePayloadWithInfo(keyed, NativePayloadInfo{
		Kind:       NativePayloadKeyedFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})

	if isDataFrameTable(table) {
		t.Fatal("keyed native frame with data frame marker resolved as data frame")
	}
	if _, err := dataLibFrameFromTable(table); err == nil {
		t.Fatal("dataLibFrameFromTable accepted keyed native frame with data frame marker")
	}
}

func TestDataLibFrameFromTableRejectsInvalidTypedNativePayloadBeforeWrapperFallback(t *testing.T) {
	frame, err := stddata.NewFrame(
		stddata.Column{Name: "sym", Data: stddata.NewSymbols([]string{"AAPL"})},
		stddata.Column{Name: "qty", Data: stddata.NewI64([]int64{10})},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := dataFrameValueFromLib(frame)
	if err != nil {
		t.Fatal(err)
	}
	table := value.Table()
	table.SetNativePayloadWithInfo(struct{}{}, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})

	if _, err := dataLibFrameFromTable(table); err == nil || !strings.Contains(err.Error(), "native data frame payload is invalid") {
		t.Fatalf("dataLibFrameFromTable invalid typed payload err = %v, want invalid native payload", err)
	}
}

func TestDataLibFrameFromTableRejectsNonFrameNativeKindBeforeMarkerFallback(t *testing.T) {
	frame, err := stddata.NewFrame(
		stddata.Column{Name: "qty", Data: stddata.NewI64([]int64{10})},
	)
	if err != nil {
		t.Fatal(err)
	}
	table := NewTable()
	table.RawSetString(dataFrameMarker, BoolValue(true))
	table.SetNativePayloadWithInfo(frame, NativePayloadInfo{
		Kind:       NativePayloadDataColumn,
		Rows:       frame.Len(),
		ColumnKind: string(stddata.KindI64),
	})

	if isDataFrameTable(table) {
		t.Fatal("data frame marker overrode non-frame native payload kind")
	}
	if _, err := dataLibFrameFromTable(table); err == nil {
		t.Fatal("dataLibFrameFromTable accepted non-frame native kind with frame payload")
	}
}

func TestDataFrameValueFromLibBuildsNativeFacade(t *testing.T) {
	frame, err := stddata.NewFrame(
		stddata.Column{Name: "sym", Data: stddata.NewSymbols([]string{"AAPL", "MSFT"})},
		stddata.Column{Name: "qty", Data: stddata.NewI64([]int64{10, 20})},
	)
	if err != nil {
		t.Fatal(err)
	}
	value, err := dataFrameValueFromLib(frame)
	if err != nil {
		t.Fatal(err)
	}
	table := value.Table()
	if _, ok := table.NativePayload().(stddata.Frame); !ok {
		t.Fatalf("native payload = %T, want stddata.Frame", table.NativePayload())
	}
	info, ok := table.NativeFramePayloadInfo()
	if !ok {
		t.Fatal("frame native frame payload info missing")
	}
	if info.Kind != NativePayloadDataFrame || info.Rows != 2 || info.Columns != 2 || info.SchemaHash != frame.SchemaFingerprint() {
		t.Fatalf("frame native payload info = %#v, want frame rows=2 columns=2 schema=%s", info, frame.SchemaFingerprint())
	}
	if got := table.Length(); got != 2 {
		t.Fatalf("facade length = %d, want 2", got)
	}
	if got := table.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("facade row 2 sym = %v, want MSFT", got)
	}
	if got := table.RawGetString("qty").Table().RawGetInt(1); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("facade qty[1] = %v, want 10", got)
	}
	qty := table.RawGetString("qty").Table()
	if !qty.IsNativeColumn() {
		t.Fatal("qty column is not marked as native column")
	}
	if _, ok := qty.NativePayload().(stddata.Array); !ok {
		t.Fatalf("qty column native payload = %T, want stddata.Array", qty.NativePayload())
	}
	qtyInfo, ok := qty.NativePayloadInfo()
	if !ok {
		t.Fatal("qty column native payload info missing")
	}
	if qtyInfo.Kind != NativePayloadDataColumn || qtyInfo.Rows != 2 || qtyInfo.ColumnKind != string(stddata.KindI64) {
		t.Fatalf("qty column native payload info = %#v, want data_column rows=2 kind=i64", qtyInfo)
	}
	roundTrip, err := dataLibFrameFromTable(table)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Len() != 2 {
		t.Fatalf("round-trip len = %d, want 2", roundTrip.Len())
	}
	qty.RawSetInt(2, IntValue(99))
	if got := qty.NativePayload(); got != nil {
		t.Fatalf("qty column native payload after mutation = %T, want nil", got)
	}
	if got, ok := qty.NativePayloadInfo(); ok {
		t.Fatalf("qty column native payload info after mutation = %#v, want none", got)
	}
	if got := qty.RawGetInt(1); !got.IsNil() {
		t.Fatalf("qty lazy getter after mutation = %v, want nil", got)
	}
}

func TestDataNativeArrayFromValueUsesUntypedFallbackOnlyWithoutTypedKind(t *testing.T) {
	table := NewTable()
	table.SetNativePayload(stddata.NewI64([]int64{10, 20}))

	if array, ok := dataNativeArrayFromValue(TableValue(table)); !ok || array.Len() != 2 {
		t.Fatalf("legacy native array fallback = %v, %v, want len 2", array, ok)
	}

	table.SetNativePayloadWithInfo(stddata.NewI64([]int64{10, 20}), NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       2,
		ColumnKind: string(stddata.KindI64),
	})
	if _, ok := dataNativeArrayFromValue(TableValue(table)); ok {
		t.Fatal("native array fallback ignored conflicting typed payload kind")
	}
}

func TestDataFrameNativePayloadInvalidatesOnUserMutation(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL", "MSFT"}),
    qty: data.i64({10, 20}),
})
f.extra = 42
`)
	frame := interp.GetGlobal("f").Table()
	if got := frame.NativePayload(); got != nil {
		t.Fatalf("native payload after user mutation = %T, want nil", got)
	}
	if got, ok := frame.NativePayloadInfo(); ok {
		t.Fatalf("native payload info after user mutation = %#v, want none", got)
	}
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
    m: data.month({"2026.06", data.null}),
    d: data.date({"2026-06-06", data.null}),
    dt: data.datetime({"2026.06.06T09:30:00", data.null}),
    span: data.timespan({"1D09:30:00", data.null}),
    minute: data.minute({"09:30", "16:00"}),
    second: data.second({"09:30:00", "16:00:00"}),
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
		"d":      "date",
		"dt":     "datetime",
		"m":      "month",
		"minute": "minute",
		"second": "second",
		"span":   "timespan",
		"tm":     "time",
		"ts":     "timestamp",
	} {
		if got := kinds.RawGetString(name); !got.IsString() || got.Str() != want {
			t.Fatalf("kinds.%s = %v, want %s", name, got, want)
		}
	}
	row2 := interp.GetGlobal("rows").Table().RawGetInt(2).Table()
	if got := row2.RawGetString("m"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].m = %v, want data.null sentinel", got)
	}
	if got := row2.RawGetString("d"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].d = %v, want data.null sentinel", got)
	}
	if got := row2.RawGetString("dt"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].dt = %v, want data.null sentinel", got)
	}
	if got := row2.RawGetString("span"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].span = %v, want data.null sentinel", got)
	}
	if got := row2.RawGetString("ts"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("rows[2].ts = %v, want data.null sentinel", got)
	}
	gkinds := interp.GetGlobal("gkinds").Table()
	for name, want := range map[string]string{"m": "month", "d": "date", "dt": "datetime", "span": "timespan", "minute": "minute", "second": "second", "tm": "time", "ts": "timestamp"} {
		if got := gkinds.RawGetString(name); !got.IsString() || got.Str() != want {
			t.Fatalf("gathered kind %s = %v, want %s", name, got, want)
		}
	}
	grow1 := interp.GetGlobal("grows").Table().RawGetInt(1).Table()
	if got := grow1.RawGetString("m"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("gather rows[1].m = %v, want data.null", got)
	}
	if got := grow1.RawGetString("minute"); !got.IsString() || got.Str() != "16:00" {
		t.Fatalf("gather rows[1].minute = %v, want 16:00", got)
	}
	if got := grow1.RawGetString("second"); !got.IsString() || got.Str() != "16:00:00" {
		t.Fatalf("gather rows[1].second = %v, want 16:00:00", got)
	}
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

func TestDataFrameTakeGatherProjectSchemaStableHelpers(t *testing.T) {
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL", "MSFT", "NVDA"}),
    qty: data.i32({10, 20, 30}),
    price: data.f64({100.5, 80.25, 120.75}),
})
taken := data.take(f, 2)
gathered := data.gather(f, {3, 1})
projected := data.project(f, {"qty", "sym"})
sameTaken := data.same_schema(f, taken)
sameGathered := data.same_schema(f, gathered)
sameProjected := data.same_schema(f, projected)
hash := data.schema_hash(f)
takenHash := data.schema_hash(taken)
gatheredHash := data.schema_hash(gathered)
projectedHash := data.schema_hash(projected)
schemaHash := f.schema.hash
takenRows := data.rows(taken)
gatherRows := data.rows(gathered)
projectKinds := data.kinds(projected)
projectCols := data.columns(projected)
`)

	if got := interp.GetGlobal("sameTaken"); !got.IsBool() || !got.Bool() {
		t.Fatalf("same_schema(f, taken) = %v, want true", got)
	}
	if got := interp.GetGlobal("sameGathered"); !got.IsBool() || !got.Bool() {
		t.Fatalf("same_schema(f, gathered) = %v, want true", got)
	}
	if got := interp.GetGlobal("sameProjected"); !got.IsBool() || got.Bool() {
		t.Fatalf("same_schema(f, projected) = %v, want false", got)
	}
	hash := interp.GetGlobal("hash")
	if !hash.IsString() || hash.Str() == "" {
		t.Fatalf("schema hash = %v, want non-empty string", hash)
	}
	for _, name := range []string{"takenHash", "gatheredHash", "schemaHash"} {
		got := interp.GetGlobal(name)
		if !got.IsString() || got.Str() != hash.Str() {
			t.Fatalf("%s = %v, want %s", name, got, hash.Str())
		}
	}
	projectedHash := interp.GetGlobal("projectedHash")
	if !projectedHash.IsString() || projectedHash.Str() == hash.Str() {
		t.Fatalf("projectedHash = %v, want different from %s", projectedHash, hash.Str())
	}

	takenRow2 := interp.GetGlobal("takenRows").Table().RawGetInt(2).Table()
	if got := takenRow2.RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("taken rows[2].sym = %v, want MSFT", got)
	}
	gatherRow1 := interp.GetGlobal("gatherRows").Table().RawGetInt(1).Table()
	if got := gatherRow1.RawGetString("qty"); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("gather rows[1].qty = %v, want 30", got)
	}
	projectCols := interp.GetGlobal("projectCols").Table()
	if got := projectCols.RawGetInt(1); !got.IsString() || got.Str() != "qty" {
		t.Fatalf("project columns[1] = %v, want qty", got)
	}
	if got := projectCols.RawGetInt(2); !got.IsString() || got.Str() != "sym" {
		t.Fatalf("project columns[2] = %v, want sym", got)
	}
	projectKinds := interp.GetGlobal("projectKinds").Table()
	if got := projectKinds.RawGetString("qty"); !got.IsString() || got.Str() != "i32" {
		t.Fatalf("project kinds.qty = %v, want i32", got)
	}
	if got := projectKinds.RawGetString("sym"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("project kinds.sym = %v, want symbol", got)
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

func TestDataColumnarStoreScriptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	interp := runWithDataAndArray(t, `
f := data.frame({
    sym: data.symbols({"AAPL", "MSFT", "AAPL"}),
    qty: data.i64({10, data.null, 30}),
    price: data.f64({100.5, 101.25, 102.75}),
})
`)
	interp.SetGlobal("store_path", StringValue(dir))
	execOnInterp(t, interp, `
ok := data.save(f, store_path)
loaded := data.load(store_path)
info := data.info(store_path)
rows := data.rows(loaded)
kinds := data.kinds(loaded)
`)

	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("data.save ok = %v, want true", got)
	}
	if got := interp.GetGlobal("info").Table().RawGetString("rows"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("info.rows = %v, want 3", got)
	}
	if got := interp.GetGlobal("kinds").Table().RawGetString("sym"); !got.IsString() || got.Str() != "symbol" {
		t.Fatalf("kinds.sym = %v, want symbol", got)
	}
	rows := interp.GetGlobal("rows").Table()
	row2 := rows.RawGetInt(2).Table()
	if got := row2.RawGetString("qty"); !got.IsTable() || !isDataNullValue(got) {
		t.Fatalf("loaded rows[2].qty = %v, want data.null", got)
	}
	row3 := rows.RawGetInt(3).Table()
	if got := row3.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("loaded rows[3].sym = %v, want AAPL", got)
	}
}

func TestDataPartitionedStoreScriptFiltersPartitions(t *testing.T) {
	dir := t.TempDir()
	interp := runWithDataAndArray(t, `
f := data.frame({
    day: data.i64({1, 1, 2, 2}),
    sym: data.symbols({"AAPL", "MSFT", "AAPL", "MSFT"}),
    qty: data.i64({10, 20, 30, 40}),
})
`)
	interp.SetGlobal("store_path", StringValue(dir))
	execOnInterp(t, interp, `
ok := data.save_partitioned(f, store_path, "day", "sym")
all := data.load_partitioned(store_path)
only := data.load_partitioned(store_path, {sym: "AAPL"})
none := data.load_partitioned(store_path, {sym: "IBM"})
info := data.info(store_path)
all_rows := data.rows(all)
only_rows := data.rows(only)
none_rows := data.rows(none)
`)

	if got := interp.GetGlobal("ok"); !got.IsBool() || !got.Bool() {
		t.Fatalf("data.save_partitioned ok = %v, want true", got)
	}
	if got := interp.GetGlobal("info").Table().RawGetString("partitions").Table().Length(); got != 4 {
		t.Fatalf("partition count = %d, want 4", got)
	}
	if got := interp.GetGlobal("all_rows").Table().Length(); got != 4 {
		t.Fatalf("all rows = %d, want 4", got)
	}
	only := interp.GetGlobal("only_rows").Table()
	if got := only.Length(); got != 2 {
		t.Fatalf("filtered rows = %d, want 2", got)
	}
	if got := only.RawGetInt(2).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("filtered row2 qty = %v, want 30", got)
	}
	if got := interp.GetGlobal("none_rows").Table().Length(); got != 0 {
		t.Fatalf("empty filtered rows = %d, want 0", got)
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
