package bind

import "testing"

func TestDialectSQLBuildsParameterizedQuery(t *testing.T) {
	interp := runWithLib(t, `
		literal := sql`+"`"+`select * from users where active = 1`+"`"+`
		filtered := sql {
			query: "select * from users where id = :id and team = :team",
			params: {id: 42, team: "platform"}
		}
		opt_filtered := dialect.eval("sql", "select * from events where kind = :kind", {params: {kind: "deploy"}})
		missing, missing_err := sql {query: "select * from users where id = :id", params: {}}
		unused, unused_err := sql {query: "select * from users", params: {id: 1}}
	`, "dialect", BuildDialect(HostOptions{}, nil))

	literal := interp.GetGlobal("literal").Table()
	if got := literal.RawGetString("query").Str(); got != "select * from users where active = 1" {
		t.Fatalf("literal query = %q", got)
	}
	if got := literal.RawGetString("args").Table().Length(); got != 0 {
		t.Fatalf("literal args = %d, want 0", got)
	}
	filtered := interp.GetGlobal("filtered").Table()
	if got := filtered.RawGetString("query").Str(); got != "select * from users where id = ? and team = ?" {
		t.Fatalf("filtered query = %q", got)
	}
	if got := filtered.RawGetString("args").Table().RawGetInt(1).Int(); got != 42 {
		t.Fatalf("arg 1 = %d, want 42", got)
	}
	if got := filtered.RawGetString("args").Table().RawGetInt(2).Str(); got != "platform" {
		t.Fatalf("arg 2 = %q, want platform", got)
	}
	if got := filtered.RawGetString("names").Table().RawGetInt(2).Str(); got != "team" {
		t.Fatalf("name 2 = %q, want team", got)
	}
	if got := interp.GetGlobal("opt_filtered").Table().RawGetString("args").Table().RawGetInt(1).Str(); got != "deploy" {
		t.Fatalf("option params arg = %q, want deploy", got)
	}
	if !interp.GetGlobal("missing").IsNil() || interp.GetGlobal("missing_err").Str() != `sql dialect: missing param "id"` {
		t.Fatalf("missing = %v err %v", interp.GetGlobal("missing"), interp.GetGlobal("missing_err"))
	}
	if !interp.GetGlobal("unused").IsNil() || interp.GetGlobal("unused_err").Str() != "sql dialect: params not referenced: id" {
		t.Fatalf("unused = %v err %v", interp.GetGlobal("unused"), interp.GetGlobal("unused_err"))
	}
}
