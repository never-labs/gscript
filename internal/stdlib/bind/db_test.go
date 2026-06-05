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
