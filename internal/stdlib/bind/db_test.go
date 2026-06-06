package bind

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestDBMemoryExecQueryOneParameterizedSQL(t *testing.T) {
	interp := runtime.NewCore()
	installTestModules(interp)
	installTestModule(interp, "dialect", runtime.TableValue(BuildDialect(HostOptions{}, interp.MaxHostResultBytes)))
	execOnInterp(t, interp, `
		conn := db.memory()
		create_result := conn.exec(sql`+"`"+`create table users (id integer primary key, name text not null, active bool not null)`+"`"+`)
		conn.exec(sql {query: "insert into users (name, active) values (:name, :active)", params: {name: "Ada", active: true}})
		conn.exec(sql {query: "insert into users (name, active) values (:name, :active)", params: {name: "Grace", active: false}})
		active_users := conn.query(sql {query: "select id, name from users where active = :active order by id", params: {active: true}})
		ada := conn.one(sql {query: "select name from users where id = :id", params: {id: 1}})
	`)

	rows := interp.GetGlobal("active_users").Table()
	if got := rows.Length(); got != 1 {
		t.Fatalf("active_users length = %d, want 1", got)
	}
	row := rows.RawGetInt(1).Table()
	if got := row.RawGetString("name").Str(); got != "Ada" {
		t.Fatalf("row.name = %q, want Ada", got)
	}
	if got := row.RawGetInt(2).Str(); got != "Ada" {
		t.Fatalf("row[2] = %q, want Ada", got)
	}
	if got := interp.GetGlobal("ada").Table().RawGetString("name").Str(); got != "Ada" {
		t.Fatalf("one.name = %q, want Ada", got)
	}
	if got := interp.GetGlobal("create_result").Table().RawGetString("rows_affected"); !got.IsNil() && got.Int() != 0 {
		t.Fatalf("create rows_affected = %v, want absent or 0", got)
	}
}

func TestDBDefaultModuleFunctionsAndInjectionSafety(t *testing.T) {
	interp := runtime.NewCore()
	installTestModules(interp)
	installTestModule(interp, "dialect", runtime.TableValue(BuildDialect(HostOptions{}, interp.MaxHostResultBytes)))
	execOnInterp(t, interp, `
		db.exec(sql`+"`"+`create table audit (id integer primary key, name text not null)`+"`"+`)
		attack := "Ada'); drop table audit; --"
		db.exec(sql {query: "insert into audit (name) values (:name)", params: {name: attack}})
		matches := db.query(sql {query: "select name from audit where name = :name", params: {name: attack}})
		after_attack := db.query(sql`+"`"+`select count(*) as n from audit`+"`"+`)
	`)

	matches := interp.GetGlobal("matches").Table()
	if got := matches.Length(); got != 1 {
		t.Fatalf("matches length = %d, want 1", got)
	}
	if got := matches.RawGetInt(1).Table().RawGetString("name").Str(); !strings.Contains(got, "drop table audit") {
		t.Fatalf("stored attack value = %q, want literal parameter", got)
	}
	count := interp.GetGlobal("after_attack").Table().RawGetInt(1).Table().RawGetString("n")
	if !count.IsInt() || count.Int() != 1 {
		t.Fatalf("count after attack = %v, want 1", count)
	}
}

func TestDBOpenRejectsNonSQLiteDriver(t *testing.T) {
	lib := BuildDB(HostOptions{})
	open := lib.RawGetString("open").GoFunction()
	opts := NewTable()
	opts.RawSetString("driver", StringValue("postgres"))
	got, err := open.Fn([]Value{TableValue(opts)})
	if err != nil {
		t.Fatalf("db.open unexpected Go error: %v", err)
	}
	if len(got) != 2 || !got[0].IsNil() || !strings.Contains(got[1].Str(), "sqlite only") {
		t.Fatalf("db.open postgres = %v, want nil sqlite-only error", got)
	}
}

func TestDBAggregateOneAndConstraintError(t *testing.T) {
	interp := runtime.NewCore()
	installTestModules(interp)
	installTestModule(interp, "dialect", runtime.TableValue(BuildDialect(HostOptions{}, interp.MaxHostResultBytes)))
	execOnInterp(t, interp, `
		conn := db.memory()
		conn.exec(sql`+"`"+`create table ledger (id integer primary key, project text not null unique, cents integer not null)`+"`"+`)
		conn.exec(sql {query: "insert into ledger (project, cents) values (:project, :cents)", params: {project: "alpha", cents: 12500}})
		conn.exec(sql {query: "insert into ledger (project, cents) values (:project, :cents)", params: {project: "beta", cents: -2500}})
		ledger_total := conn.one(sql`+"`"+`select sum(cents) as cents from ledger`+"`"+`)
		ledger_rows := conn.query(sql`+"`"+`select count(*) as n, sum(cents) as cents from ledger`+"`"+`)
		duplicate_project, duplicate_project_err := conn.exec(sql {query: "insert into ledger (project, cents) values (:project, :cents)", params: {project: "alpha", cents: 100}})
	`)

	total := interp.GetGlobal("ledger_total").Table().RawGetString("cents")
	if !total.IsInt() || total.Int() != 10000 {
		t.Fatalf("ledger_total.cents = %v, want 10000", total)
	}
	row := interp.GetGlobal("ledger_rows").Table().RawGetInt(1).Table()
	if got := row.RawGetString("n"); !got.IsInt() || got.Int() != 2 {
		t.Fatalf("ledger_rows[1].n = %v, want 2", got)
	}
	if got := row.RawGetString("cents"); !got.IsInt() || got.Int() != 10000 {
		t.Fatalf("ledger_rows[1].cents = %v, want 10000", got)
	}
	if got := interp.GetGlobal("duplicate_project"); !got.IsNil() {
		t.Fatalf("duplicate_project = %v, want nil", got)
	}
	if got := interp.GetGlobal("duplicate_project_err"); !got.IsString() || !strings.Contains(got.Str(), "constraint") {
		t.Fatalf("duplicate_project_err = %v, want constraint error string", got)
	}
}

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
			by: {"channel_id"},
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
	rows := frame.RawGetString("rows").Table()
	if got := rows.RawGetInt(1).Table().RawGetString("label").Str(); got != "search" {
		t.Fatalf("frame.rows[1].label = %q, want search", got)
	}
	if got := frame.RawGetString("columns").Table().RawGetString("label").Table().RawGetInt(2).Str(); got != "email" {
		t.Fatalf("frame.columns.label[2] = %q, want email", got)
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
