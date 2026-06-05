package bind

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

func registerDialectDatabase(register dialectRegisterFunc) {
	register([]string{"sql"}, dialectHandler{
		eval:  dialectSQL,
		block: dialectSQL,
	})
}

func dialectSQL(body Value, opts *Table) ([]Value, error) {
	mode := dialectMode(opts)
	if !dialectModeAllowed(mode, "", "query", "params", "format") {
		return dialectUnknownMode("sql", mode)
	}
	query, params, err := sqlDialectInput(body, opts)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	built, err := buildSQLQuery(query, params)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(sqlQueryObject(built.Query, built.Args, built.Names))}, nil
}

type sqlBuildResult struct {
	Query string
	Args  []Value
	Names []string
}

func sqlDialectInput(body Value, opts *Table) (string, *Table, error) {
	if body.IsString() {
		return body.Str(), optionalSQLParams(opts), nil
	}
	if !body.IsTable() {
		return "", nil, fmt.Errorf("sql dialect: string or table expected")
	}
	tbl := body.Table()
	query := firstSQLString(tbl, "query", "text", "sql")
	if query == "" {
		return "", nil, fmt.Errorf("sql dialect: query string required")
	}
	params := tbl.RawGetString("params")
	if params.IsNil() {
		params = tbl.RawGetString("args")
	}
	if params.IsNil() {
		params = tbl.RawGetString("bindings")
	}
	if params.IsNil() {
		return query, optionalSQLParams(opts), nil
	}
	if !params.IsTable() {
		return "", nil, fmt.Errorf("sql dialect: params must be a table")
	}
	return query, params.Table(), nil
}

func optionalSQLParams(opts *Table) *Table {
	if opts == nil {
		return nil
	}
	params := opts.RawGetString("params")
	if params.IsNil() {
		params = opts.RawGetString("args")
	}
	if params.IsNil() || !params.IsTable() {
		return nil
	}
	return params.Table()
}

func firstSQLString(tbl *Table, keys ...string) string {
	for _, key := range keys {
		value := tbl.RawGetString(key)
		if value.IsString() {
			return value.Str()
		}
	}
	return ""
}

func buildSQLQuery(query string, params *Table) (sqlBuildResult, error) {
	if params == nil {
		return sqlBuildResult{Query: strings.TrimSpace(query)}, nil
	}
	var out strings.Builder
	args := make([]Value, 0)
	names := make([]string, 0)
	for i := 0; i < len(query); {
		ch := rune(query[i])
		if ch != ':' || i+1 >= len(query) || !isSQLNameStart(rune(query[i+1])) {
			out.WriteByte(query[i])
			i++
			continue
		}
		if i > 0 && query[i-1] == ':' {
			out.WriteByte(query[i])
			i++
			continue
		}
		j := i + 2
		for j < len(query) && isSQLNamePart(rune(query[j])) {
			j++
		}
		name := query[i+1 : j]
		value := params.RawGetString(name)
		if value.IsNil() {
			return sqlBuildResult{}, fmt.Errorf("sql dialect: missing param %q", name)
		}
		out.WriteByte('?')
		args = append(args, value)
		names = append(names, name)
		i = j
	}
	if len(args) == 0 && tableHasAnyKey(params) {
		names := make([]string, 0)
		params.ForEachPlainRaw(func(k, _ Value) bool {
			if k.IsString() {
				names = append(names, k.Str())
			}
			return true
		})
		sort.Strings(names)
		return sqlBuildResult{}, fmt.Errorf("sql dialect: params not referenced: %s", strings.Join(names, ", "))
	}
	return sqlBuildResult{Query: strings.TrimSpace(out.String()), Args: args, Names: names}, nil
}

func isSQLNameStart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch)
}

func isSQLNamePart(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

func sqlQueryObject(query string, args []Value, names []string) *Table {
	out := NewTable()
	out.RawSetString("query", StringValue(query))
	argsTable := NewAppendArrayTable(len(args))
	for i, arg := range args {
		argsTable.RawSetInt(int64(i+1), arg)
	}
	namesTable := NewAppendArrayTable(len(names))
	for i, name := range names {
		namesTable.RawSetInt(int64(i+1), StringValue(name))
	}
	out.RawSetString("args", TableValue(argsTable))
	out.RawSetString("names", TableValue(namesTable))
	return out
}
