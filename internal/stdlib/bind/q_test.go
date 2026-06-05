package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
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
