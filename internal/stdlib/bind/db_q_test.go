//go:build leia_q

package bind

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestDBFrameFeedsColumnarQQuery(t *testing.T) {
	interp := runtime.NewCore()
	installTestModules(interp)
	installTestModule(interp, "soa", runtime.TableValue(BuildSOA()))
	installTestModule(interp, "q", runtime.TableValue(BuildQ()))
	installTestModule(interp, "dialect", runtime.TableValue(BuildDialect(HostOptions{}, interp.MaxHostResultBytes)))
	execOnInterp(t, interp, `
		conn := db.memory()
		conn.exec(sql`+"`"+`create table campaigns (
			day integer not null,
			channel_id integer not null,
			spend real not null,
			revenue real not null,
			conversions integer not null,
			label text not null
		)`+"`"+`)
		conn.exec(sql`+"`"+`insert into campaigns values
			(1, 1, 100.0, 250.0, 5, 'search'),
			(1, 2, 80.0, 120.0, 3, 'email'),
			(2, 1, 120.0, 300.0, 6, 'search'),
			(2, 2, 70.0, 210.0, 4, 'email')`+"`"+`)
		frame := conn.frame(sql`+"`"+`select day, channel_id, spend, revenue, conversions, label from campaigns order by day, channel_id`+"`"+`)
		rollup := q.query(frame.soa, {
			by: ["channel_id"],
			select: {
				spend: "spend",
				revenue: "revenue",
				conversions: "conversions",
				days: 1,
			},
			aggregate: {
				spend: "sum",
				revenue: "sum",
				conversions: "sum",
				days: "count",
			},
			order_by: {column: "revenue", desc: true},
		})
	`)

	frame := interp.GetGlobal("frame").Table()
	if got := frame.RawGetString("len"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("frame.len = %v, want 4", got)
	}
	if got := frame.RawGetString("kind"); !got.IsString() || got.Str() != "data_frame" {
		t.Fatalf("frame.kind = %v, want data_frame", got)
	}
	if got := frame.RawGetString("nrows"); !got.IsInt() || got.Int() != 4 {
		t.Fatalf("frame.nrows = %v, want 4", got)
	}
	if got := frame.RawGetString("ncols"); !got.IsInt() || got.Int() != 6 {
		t.Fatalf("frame.ncols = %v, want 6", got)
	}
	rows := frame.RawGetString("rows").Table()
	if got := rows.RawGetInt(1).Table().RawGetString("label").Str(); got != "search" {
		t.Fatalf("frame.rows[1].label = %q, want search", got)
	}
	if got := frame.RawGetString("columns").Table().RawGetString("label").Table().RawGetInt(2).Str(); got != "email" {
		t.Fatalf("frame.columns.label[2] = %q, want email", got)
	}
	if got := frame.RawGetString("data").Table().RawGetString("label").Table().RawGetInt(2).Str(); got != "email" {
		t.Fatalf("frame.data.label[2] = %q, want email", got)
	}
	if got := frame.RawGetString("schema").Table().RawGetString("kinds").Table().RawGetString("label").Str(); got != "string" {
		t.Fatalf("frame.schema.kinds.label = %q, want string", got)
	}
	revenueColumn := frame.RawGetString("numeric").Table().RawGetString("revenue")
	if !revenueColumn.IsDenseArray() {
		t.Fatalf("frame.numeric.revenue = %v, want dense array", revenueColumn)
	}
	if got, _ := revenueColumn.DenseArray().At(3); !got.IsFloat() || got.Float() != 210 {
		t.Fatalf("frame.numeric.revenue[3] = %v, want 210", got)
	}
	if !frame.RawGetString("soa").IsSoA() {
		t.Fatalf("frame.soa missing")
	}

	rollup := interp.GetGlobal("rollup").Table()
	if got := rollup.Length(); got != 2 {
		t.Fatalf("rollup length = %d, want 2", got)
	}
	first := rollup.RawGetInt(1).Table()
	if got := first.RawGetString("channel_id"); !got.IsInt() || got.Int() != 1 {
		t.Fatalf("first.channel_id = %v, want 1", got)
	}
	if got := first.RawGetString("revenue"); !got.IsFloat() || got.Float() != 550 {
		t.Fatalf("first.revenue = %v, want 550", got)
	}
	if got := first.RawGetString("conversions"); !got.IsFloat() || got.Float() != 11 {
		t.Fatalf("first.conversions = %v, want 11", got)
	}
}
