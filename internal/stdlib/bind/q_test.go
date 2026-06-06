package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestQQueryFiltersSelectsAndGroupsSOA(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := soa.zip({
    sym: []i64{1, 1, 2, 2},
    price: []f64{10, 12, 7.5, 8},
    size: []f64{100, 50, 200, 150},
})

rows := q.query(trades, {
    where: {column: "size", op: ">=", value: 100},
    by: {"sym"},
    select: {
        notional: {"*", "price", "size"},
        size: "size",
        fills: 1,
    },
    aggregate: {
        notional: "sum",
        size: "sum",
        fills: "count",
    },
})

flat := q.query(trades, {
    where: soa.mask(trades, "sym", "==", 1),
    select: {px: "price", qty: "size"},
})
`)

	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows length = %v, want 2", rows)
	}
	first := rows.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("first.sym = %v, want 1", got)
	}
	if got := first.RawGetString("notional"); !got.IsFloat() || got.Float() != 1000 {
		t.Fatalf("first.notional = %v, want 1000", got)
	}
	if got := first.RawGetString("fills"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("first.fills = %v, want 1", got)
	}
	second := rows.RawGetInt(2).Table()
	if got := second.RawGetString("sym"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("second.sym = %v, want 2", got)
	}
	if got := second.RawGetString("notional"); !got.IsFloat() || got.Float() != 2700 {
		t.Fatalf("second.notional = %v, want 2700", got)
	}
	if got := second.RawGetString("size"); !got.IsFloat() || got.Float() != 350 {
		t.Fatalf("second.size = %v, want 350", got)
	}
	if got := second.RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("second.fills = %v, want 2", got)
	}
	flat := interp.GetGlobal("flat").Table()
	if flat == nil || flat.Length() != 2 {
		t.Fatalf("flat length = %v, want 2", flat)
	}
	if got := flat.RawGetInt(2).Table().RawGetString("qty"); !got.IsFloat() || got.Float() != 50 {
		t.Fatalf("flat[2].qty = %v, want 50", got)
	}
}

func TestQModuleRejectsInvalidPlans(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := soa.zip({x: []f64{1}})
ok, err := pcall(func() {
    return q.query(trades, {where: {column: "x", op: "contains", value: 1}})
})
`)
	if got := interp.GetGlobal("ok"); !got.IsBool() || got.Bool() {
		t.Fatalf("ok = %v, want false", got)
	}
	if got := interp.GetGlobal("err"); !got.IsString() || got.Str() == "" {
		t.Fatalf("err = %v, want non-empty string", got)
	}
}

func TestQQueryOrdersAndLimitsRows(t *testing.T) {
	interp := runWithQAndSOA(t, `
trades := soa.zip({
    sym: []i64{1, 2, 3, 4},
    price: []f64{10, 30, 20, 40},
    size: []f64{3, 1, 4, 2},
})

top_prices := q.query(trades, {
    select: {sym: "sym", price: "price", size: "size"},
    order_by: {column: "price", desc: true},
    limit: 2,
})

by_size := q.query(trades, {
    select: {sym: "sym", price: "price", size: "size"},
    order_by: "size",
    limit: 3,
})

notional_by_sym := q.query(trades, {
    by: {"sym"},
    select: {notional: {"*", "price", "size"}},
    aggregate: {notional: "sum"},
    order_by: {column: "notional", desc: true},
    limit: 2,
})
`)
	top := interp.GetGlobal("top_prices").Table()
	if top == nil || top.Length() != 2 {
		t.Fatalf("top_prices length = %v, want 2", top)
	}
	if got := top.RawGetInt(1).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("top_prices[1].sym = %v, want 4", got)
	}
	if got := top.RawGetInt(2).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("top_prices[2].sym = %v, want 2", got)
	}
	bySize := interp.GetGlobal("by_size").Table()
	if got := bySize.RawGetInt(1).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("by_size[1].sym = %v, want 2", got)
	}
	if got := bySize.RawGetInt(3).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("by_size[3].sym = %v, want 1", got)
	}
	grouped := interp.GetGlobal("notional_by_sym").Table()
	if got := grouped.RawGetInt(1).Table().RawGetString("sym"); !got.IsInt() || got.Int() != 3 {
		t.Fatalf("notional_by_sym[1].sym = %v, want 3", got)
	}
	if got := grouped.RawGetInt(2).Table().RawGetString("notional"); !got.IsFloat() || got.Float() != 80 {
		t.Fatalf("notional_by_sym[2].notional = %v, want 80", got)
	}
}

func TestQSQLSelectWhereByOverFrame(t *testing.T) {
	interp := runWithQAndSOA(t,
		"trades := q.eval(\"flip `sym`price`size!(`AAPL`MSFT`AAPL`MSFT;100 80 120 110;10 20 30 40)\")\n"+
			"rollup := q.sql(trades, \"select notional:sum price*size, fills:count i by sym from trades where price>=100\")\n"+
			"also := q.select(trades, \"select px:price, qty:size from trades where sym=`AAPL\")\n")

	rollup := interp.GetGlobal("rollup").Table()
	if rollup == nil || rollup.Length() != 2 {
		t.Fatalf("rollup length = %v, want 2", rollup)
	}
	first := rollup.RawGetInt(1).Table()
	if got := first.RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rollup[1].sym = %v, want AAPL", got)
	}
	if got := first.RawGetString("notional"); !got.IsFloat() || got.Float() != 4600 {
		t.Fatalf("rollup[1].notional = %v, want 4600", got)
	}
	if got := first.RawGetString("fills"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("rollup[1].fills = %v, want 2", got)
	}
	second := rollup.RawGetInt(2).Table()
	if got := second.RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("rollup[2].sym = %v, want MSFT", got)
	}
	if got := second.RawGetString("notional"); !got.IsFloat() || got.Float() != 4400 {
		t.Fatalf("rollup[2].notional = %v, want 4400", got)
	}

	also := interp.GetGlobal("also").Table()
	if also == nil || also.Length() != 2 {
		t.Fatalf("also length = %v, want 2", also)
	}
	if got := also.RawGetInt(2).Table().RawGetString("qty"); !got.IsInt() || got.Int() != 30 {
		t.Fatalf("also[2].qty = %v, want 30", got)
	}
}

func TestQSQLWhereGreaterThanIsStrict(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp, `
frame := data.frame({
    sym: {"low", "edge", "high"},
    price: array.i64({99, 100, 101}),
})
rows := q.sql(frame, "select sym,price from frame where price>100")
`)
	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 1 {
		t.Fatalf("rows len = %v, want 1", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "high" {
		t.Fatalf("rows[1].sym = %v, want high", got)
	}
}

func TestQSQLAcceptsSQLFirstNamedFrame(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\"},\n"+
			"    price: array.i64({100, 80, 120}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"i64\"}\n"+
			"rows := q.sql(\"select sym,price from trades where sym=`AAPL\", {trades: trades})\n"+
			"also := q.select(\"select price from trades where sym=`MSFT\", {trades: trades})\n")

	rows := interp.GetGlobal("rows").Table()
	if rows == nil || rows.Length() != 2 {
		t.Fatalf("rows len = %v, want 2", rows)
	}
	if got := rows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("rows[1].sym = %v, want AAPL", got)
	}
	if got := rows.RawGetInt(2).Table().RawGetString("price"); !got.IsInt() || got.Int() != 120 {
		t.Fatalf("rows[2].price = %v, want 120", got)
	}
	also := interp.GetGlobal("also").Table()
	if also == nil || also.Length() != 1 {
		t.Fatalf("also len = %v, want 1", also)
	}
	if got := also.RawGetInt(1).Table().RawGetString("price"); !got.IsInt() || got.Int() != 80 {
		t.Fatalf("also[1].price = %v, want 80", got)
	}
}

func TestQSQLTypedFrameWhereAndOutputSemantics(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"frame := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"AAPL\", \"IBM\"},\n"+
			"    venue: {\"XNYS\", \"XNAS\", \"BATS\", \"XNYS\"},\n"+
			"    active: array.bool({true, false, true, true}),\n"+
			"    price: array.f64({100.5, 80.25, 120.0, 90.0}),\n"+
			"})\n"+
			"frame.column_kinds = {\n"+
			"    sym: \"symbol\",\n"+
			"    venue: \"string\",\n"+
			"    active: \"bool\",\n"+
			"    price: \"f64\",\n"+
			"}\n"+
			"symbol_rows := q.sql(frame, \"select sym,price from frame where sym=`AAPL\")\n"+
			"string_rows := q.sql(frame, \"select venue from frame where venue=`XNYS\")\n"+
			"bool_rows := q.sql(frame, \"select sym,active from frame where active=true\")\n")

	symbolRows := interp.GetGlobal("symbol_rows").Table()
	if symbolRows == nil || symbolRows.Length() != 2 {
		t.Fatalf("symbol_rows len = %v, want 2", symbolRows)
	}
	if got := symbolRows.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 120 {
		t.Fatalf("symbol_rows[2].price = %v, want 120", got)
	}
	stringRows := interp.GetGlobal("string_rows").Table()
	if stringRows == nil || stringRows.Length() != 2 {
		t.Fatalf("string_rows len = %v, want 2", stringRows)
	}
	if got := stringRows.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNYS" {
		t.Fatalf("string_rows[1].venue = %v, want XNYS", got)
	}
	boolRows := interp.GetGlobal("bool_rows").Table()
	if boolRows == nil || boolRows.Length() != 3 {
		t.Fatalf("bool_rows len = %v, want 3", boolRows)
	}
	if got := boolRows.RawGetInt(1).Table().RawGetString("active"); !got.IsBool() || !got.Bool() {
		t.Fatalf("bool_rows[1].active = %v, want true", got)
	}
}

func TestQSQLPlanCacheKeepsSchemaLiteralAlignmentSeparate(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"symbol_frame := data.frame({name: {\"AAPL\", \"MSFT\"}})\n"+
			"symbol_frame.column_kinds = {name: \"symbol\"}\n"+
			"string_frame := data.frame({name: {\"AAPL\", \"MSFT\"}})\n"+
			"string_frame.column_kinds = {name: \"string\"}\n"+
			"src := \"select name from frame where name=`AAPL\"\n"+
			"symbol_rows := q.sql(symbol_frame, src)\n"+
			"string_rows := q.sql(string_frame, src)\n")

	symbolRows := interp.GetGlobal("symbol_rows").Table()
	if symbolRows == nil || symbolRows.Length() != 1 {
		t.Fatalf("symbol_rows len = %v, want 1", symbolRows)
	}
	if got := symbolRows.RawGetInt(1).Table().RawGetString("name"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("symbol_rows[1].name = %v, want AAPL", got)
	}
	stringRows := interp.GetGlobal("string_rows").Table()
	if stringRows == nil || stringRows.Length() != 1 {
		t.Fatalf("string_rows len = %v, want 1", stringRows)
	}
	if got := stringRows.RawGetInt(1).Table().RawGetString("name"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("string_rows[1].name = %v, want AAPL", got)
	}
}

func TestQSQLReturnsDataFrameCompatibleRowsAndTemporalStrings(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"events := data.frame({\n"+
			"    day: {\"2026-06-05\", \"2026-06-06\"},\n"+
			"    ts: {\"2026-06-05T09:30:00Z\", \"2026-06-06T09:30:00Z\"},\n"+
			"    active: array.bool({true, false}),\n"+
			"    qty: array.i64({10, 20}),\n"+
			"    px: array.f64({1.5, 2.25}),\n"+
			"    note: {nil, \"close\"},\n"+
			"})\n"+
			"events.column_kinds = {day: \"date\", ts: \"timestamp\", active: \"bool\", qty: \"i64\", px: \"f64\", note: \"string\"}\n"+
			"rows := q.sql(events, \"select day,ts,active,qty,px,note from events where active=true\")\n"+
			"data_rows := data.rows(rows)\n")

	rows := interp.GetGlobal("rows").Table()
	if rows == nil {
		t.Fatalf("rows = nil")
	}
	if got := rows.RawGetString("kind"); !got.IsString() || got.Str() != "data_frame" {
		t.Fatalf("rows.kind = %v, want data_frame", got)
	}
	if got := rows.RawGetString("len"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("rows.len = %v, want 1", got)
	}
	if got := rows.RawGetString("ncols"); !got.IsInt() || got.Int() != 6 {
		t.Fatalf("rows.ncols = %v, want 6", got)
	}
	oldRow := rows.RawGetInt(1).Table()
	if got := oldRow.RawGetString("day"); !got.IsString() || got.Str() != "2026-06-05" {
		t.Fatalf("rows[1].day = %v, want 2026-06-05", got)
	}
	if got := oldRow.RawGetString("ts"); !got.IsString() || got.Str() != "2026-06-05T09:30:00Z" {
		t.Fatalf("rows[1].ts = %v, want timestamp string", got)
	}
	if got := oldRow.RawGetString("active"); !got.IsBool() || !got.Bool() {
		t.Fatalf("rows[1].active = %v, want true", got)
	}
	if got := oldRow.RawGetString("qty"); !got.IsInt() || got.Int() != 10 {
		t.Fatalf("rows[1].qty = %v, want 10", got)
	}
	if got := oldRow.RawGetString("px"); !got.IsFloat() || got.Float() != 1.5 {
		t.Fatalf("rows[1].px = %v, want 1.5", got)
	}
	if got := oldRow.RawGetString("note"); !got.IsNil() {
		t.Fatalf("rows[1].note = %v, want nil", got)
	}
	if got := rows.RawGetString("rows").Table().RawGetInt(1).Table().RawGetString("day"); !got.IsString() || got.Str() != "2026-06-05" {
		t.Fatalf("rows.rows[1].day = %v, want 2026-06-05", got)
	}
	if got := rows.RawGetString("column_kinds").Table().RawGetString("day"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("rows.column_kinds.day = %v, want string", got)
	}
	if got := rows.RawGetString("schema").Table().RawGetString("kinds").Table().RawGetString("ts"); !got.IsString() || got.Str() != "string" {
		t.Fatalf("rows.schema.kinds.ts = %v, want string", got)
	}
	dataRows := interp.GetGlobal("data_rows").Table()
	if got := dataRows.RawGetInt(1).Table().RawGetString("day"); !got.IsString() || got.Str() != "2026-06-05" {
		t.Fatalf("data.rows(rows)[1].day = %v, want 2026-06-05", got)
	}
}

func TestQSQLNullOutputFromTypedFrameWrapper(t *testing.T) {
	frame := NewTable()
	columns := NewTable()
	names := NewAppendArrayTable(2)
	kinds := NewTable()
	sym := NewAppendArrayTable(3)
	note := NewAppendArrayTable(3)
	for i, value := range []Value{StringValue("AAPL"), StringValue("MSFT"), StringValue("IBM")} {
		sym.RawSetInt(int64(i+1), value)
	}
	note.RawSetInt(1, StringValue("open"))
	note.RawSetInt(2, NilValue())
	note.RawSetInt(3, StringValue("close"))
	columns.RawSetString("sym", TableValue(sym))
	columns.RawSetString("note", TableValue(note))
	names.RawSetInt(1, StringValue("sym"))
	names.RawSetInt(2, StringValue("note"))
	kinds.RawSetString("sym", StringValue("symbol"))
	kinds.RawSetString("note", StringValue("string"))
	frame.RawSetString(dataFrameMarker, BoolValue(true))
	frame.RawSetString("len", IntValue(3))
	frame.RawSetString("columns", TableValue(columns))
	frame.RawSetString("column_names", TableValue(names))
	frame.RawSetString("column_kinds", TableValue(kinds))

	fn := BuildQ().RawGetString("sql").GoFunction()
	out, err := fn.Fn([]Value{TableValue(frame), StringValue("select sym,note from frame where note=null")})
	if err != nil {
		t.Fatalf("q.sql returned error: %v", err)
	}
	nullRows := out[0].Table()
	if nullRows == nil || nullRows.Length() != 1 {
		t.Fatalf("null_rows len = %v, want 1", nullRows)
	}
	if got := nullRows.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("null_rows[1].sym = %v, want MSFT", got)
	}
	if got := nullRows.RawGetInt(1).Table().RawGetString("note"); !got.IsNil() {
		t.Fatalf("null_rows[1].note = %v (%s), want nil", got, got.TypeName())
	}
}

func TestQSQLOrderByAndLimit(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"trades := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\", \"IBM\"},\n"+
			"    price: array.f64({100, 80, 120, 90}),\n"+
			"    active: array.bool({true, false, true, true}),\n"+
			"})\n"+
			"trades.column_kinds = {sym: \"symbol\", price: \"f64\", active: \"bool\"}\n"+
			"top := q.sql(trades, \"select sym,price from trades where active=true order by price desc limit 2\")\n"+
			"bottom := q.sql(\"select sym,price from trades order by price asc limit 1\", {trades: trades})\n")

	top := interp.GetGlobal("top").Table()
	if top == nil || top.Length() != 2 {
		t.Fatalf("top len = %v, want 2", top)
	}
	if got := top.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("top[1].sym = %v, want NVDA", got)
	}
	if got := top.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("top[2].sym = %v, want AAPL", got)
	}
	bottom := interp.GetGlobal("bottom").Table()
	if bottom == nil || bottom.Length() != 1 {
		t.Fatalf("bottom len = %v, want 1", bottom)
	}
	if got := bottom.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("bottom[1].sym = %v, want MSFT", got)
	}
}

func TestQSQLExposesLibQExecOrderLimitAndLiterals(t *testing.T) {
	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"events := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\", \"IBM\"},\n"+
			"    venue: {\"XNYS\", \"XNAS\", \"XNYS\", \"XNYS\"},\n"+
			"    active: array.bool({true, true, true, false}),\n"+
			"    price: array.f64({100, 80, 120, 90}),\n"+
			"    note: {nil, \"open\", \"open\", \"halted\"},\n"+
			"})\n"+
			"events.column_kinds = {sym: \"symbol\", venue: \"string\", active: \"bool\", price: \"f64\", note: \"string\"}\n"+
			"prices := q.sql(events, \"exec price,sym from events where venue=\\\"XNYS\\\" order by price desc limit 2\")\n"+
			"nulls := q.select(\"exec sym,note from events where note=null\", {events: events})\n"+
			"live := q.sql(events, \"select sym,active from events where active=true order by sym asc limit 1\")\n"+
			"venues := q.sql(events, \"select distinct venue from events order by venue asc take 1\")\n"+
			"prefix := q.select(\"2#select sym,price from events order by price desc\", {events: events})\n")

	prices := interp.GetGlobal("prices").Table()
	if prices == nil || prices.Length() != 2 {
		t.Fatalf("prices len = %v, want 2", prices)
	}
	if got := prices.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("prices[1].sym = %v, want NVDA", got)
	}
	if got := prices.RawGetInt(2).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 100 {
		t.Fatalf("prices[2].price = %v, want 100", got)
	}
	nulls := interp.GetGlobal("nulls").Table()
	if nulls == nil || nulls.Length() != 1 {
		t.Fatalf("nulls len = %v, want 1", nulls)
	}
	if got := nulls.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("nulls[1].sym = %v, want AAPL", got)
	}
	if got := nulls.RawGetInt(1).Table().RawGetString("note"); !got.IsNil() {
		t.Fatalf("nulls[1].note = %v, want nil", got)
	}
	live := interp.GetGlobal("live").Table()
	if live == nil || live.Length() != 1 {
		t.Fatalf("live len = %v, want 1", live)
	}
	if got := live.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("live[1].sym = %v, want AAPL", got)
	}
	venues := interp.GetGlobal("venues").Table()
	if venues == nil || venues.Length() != 1 {
		t.Fatalf("venues len = %v, want 1", venues)
	}
	if got := venues.RawGetInt(1).Table().RawGetString("venue"); !got.IsString() || got.Str() != "XNAS" {
		t.Fatalf("venues[1].venue = %v, want XNAS", got)
	}
	prefix := interp.GetGlobal("prefix").Table()
	if prefix == nil || prefix.Length() != 2 {
		t.Fatalf("prefix len = %v, want 2", prefix)
	}
	if got := prefix.RawGetInt(1).Table().RawGetString("sym"); !got.IsString() || got.Str() != "NVDA" {
		t.Fatalf("prefix[1].sym = %v, want NVDA", got)
	}
	if got := prefix.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "AAPL" {
		t.Fatalf("prefix[2].sym = %v, want AAPL", got)
	}
}

func TestQSQLPlanCachesDoNotStoreFrameData(t *testing.T) {
	qSQLTemplateCacheMu.Lock()
	qSQLTemplateCache = make(map[string]qSQLPlanTemplate)
	qSQLTemplateOrder = nil
	qSQLTemplateCacheMu.Unlock()

	qSQLAlignedPlanCacheMu.Lock()
	qSQLAlignedPlanCache = make(map[string]data.QueryPlan)
	qSQLAlignedPlanOrder = nil
	qSQLAlignedPlanCacheMu.Unlock()

	interp := runtime.NewCore()
	installTestModule(interp, "array", runtime.TableValue(BuildArray()))
	installTestModule(interp, "data", runtime.TableValue(BuildData()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	execOnInterp(t, interp,
		"events := data.frame({\n"+
			"    sym: {\"AAPL\", \"MSFT\", \"NVDA\"},\n"+
			"    price: array.f64({100, 80, 120}),\n"+
			"})\n"+
			"events.column_kinds = {sym: \"symbol\", price: \"f64\"}\n"+
			"rows := q.sql(events, \"select sym,price from events where price>=100 order by price desc limit 1\")\n")

	qSQLTemplateCacheMu.Lock()
	defer qSQLTemplateCacheMu.Unlock()
	if len(qSQLTemplateCache) != 1 {
		t.Fatalf("template cache entries = %d, want 1", len(qSQLTemplateCache))
	}
	for key, tmpl := range qSQLTemplateCache {
		if got := tmpl.plan.Source.Len(); got != 0 {
			t.Fatalf("template cache %q Source.Len() = %d, want zero frame", key, got)
		}
		if len(tmpl.plan.Source.Schema().Names()) != 0 {
			t.Fatalf("template cache %q stored source schema", key)
		}
	}

	qSQLAlignedPlanCacheMu.Lock()
	defer qSQLAlignedPlanCacheMu.Unlock()
	if len(qSQLAlignedPlanCache) != 1 {
		t.Fatalf("aligned cache entries = %d, want 1", len(qSQLAlignedPlanCache))
	}
	for key, plan := range qSQLAlignedPlanCache {
		if got := plan.Source.Len(); got != 0 {
			t.Fatalf("aligned cache %q Source.Len() = %d, want zero frame", key, got)
		}
		if len(plan.Source.Schema().Names()) != 0 {
			t.Fatalf("aligned cache %q stored source schema", key)
		}
	}
}

func TestQSymbolicCoreDataForms(t *testing.T) {
	interp := runWithQAndSOA(t,
		"syms := q.eval(\"`AAPL`MSFT`NVDA\")\n"+
			"spread := q.eval(\"100 101.5 103 - 99.5 100 101\")\n"+
			"running := q.eval(\"+\\\\100 101.5 103\")\n"+
			"total := q.eval(\"+/10 20 30 40\")\n"+
			"named_total := q.eval(\"sum 10 20 30 40\")\n"+
			"named_running := q.eval(\"sums 100 101.5 103\")\n"+
			"idx := q.eval(\"where 100 101.5 103>100\")\n"+
			"idx_count := q.count(idx)\n"+
			"first_two := q.eval(\"2#10 20 30\")\n"+
			"dict := q.eval(\"`bid`ask`last!(99.5 100;100.5 101;100 101.5)\")\n"+
			"trades := q.eval(\"flip `sym`side`price`size!(`AAPL`MSFT`AAPL;`buy`sell`buy;100.5 200 101;10 15 20)\")\n")

	if got := interp.GetGlobal("syms").Table().RawGetInt(2); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("syms[2] = %v, want MSFT", got)
	}
	if got, _ := interp.GetGlobal("spread").DenseArray().At(2); !got.IsFloat() || got.Float() != 2 {
		t.Fatalf("spread[3] = %v, want 2", got)
	}
	if got, _ := interp.GetGlobal("running").DenseArray().At(2); !got.IsFloat() || got.Float() != 304.5 {
		t.Fatalf("running[3] = %v, want 304.5", got)
	}
	if got := interp.GetGlobal("total"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("total = %v, want 100", got)
	}
	if got := interp.GetGlobal("named_total"); !got.IsInt() || got.Int() != 100 {
		t.Fatalf("named_total = %v, want 100", got)
	}
	if got, _ := interp.GetGlobal("named_running").DenseArray().At(2); !got.IsFloat() || got.Float() != 304.5 {
		t.Fatalf("named_running[3] = %v, want 304.5", got)
	}
	if got, _ := interp.GetGlobal("idx").DenseArray().At(0); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("idx[1] = %v, want 2", got)
	}
	if got := interp.GetGlobal("idx_count"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("idx_count = %v, want 2", got)
	}
	if got, _ := interp.GetGlobal("first_two").DenseArray().At(1); !got.IsInt() || got.Int() != 20 {
		t.Fatalf("first_two[2] = %v, want 20", got)
	}
	dict := interp.GetGlobal("dict").Table()
	if got, _ := dict.RawGetString("ask").DenseArray().At(1); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("dict.ask[2] = %v, want 101", got)
	}
	trades := interp.GetGlobal("trades").Table()
	if trades == nil || trades.Length() != 3 {
		t.Fatalf("trades length = %v, want 3", trades)
	}
	if got := trades.RawGetInt(2).Table().RawGetString("sym"); !got.IsString() || got.Str() != "MSFT" {
		t.Fatalf("trades[2].sym = %v, want MSFT", got)
	}
	if got := trades.RawGetInt(3).Table().RawGetString("price"); !got.IsFloat() || got.Float() != 101 {
		t.Fatalf("trades[3].price = %v, want 101", got)
	}
}

func runWithQAndSOA(t *testing.T, src string) *runtime.Interpreter {
	t.Helper()
	interp := runtime.NewCore()
	for name, lib := range map[string]*Table{
		"soa": BuildSOA(),
		"q":   BuildQ(),
	} {
		interp.SetGlobal(name, runtime.TableValue(lib))
		interp.SetModule(name, runtime.TableValue(lib))
	}
	execOnInterp(t, interp, src)
	return interp
}
