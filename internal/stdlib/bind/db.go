package bind

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type dbConnection struct {
	db *sql.DB
}

// BuildDB creates the built-in SQLite-backed "db" standard-library module.
func BuildDB(opts HostOptions) *Table {
	t := markStdlibBoundModule(NewTable())
	var defaultOnce sync.Once
	var defaultConn *dbConnection
	var defaultErr error

	getDefault := func() (*dbConnection, error) {
		defaultOnce.Do(func() {
			defaultConn, defaultErr = openSQLite(":memory:", opts)
		})
		return defaultConn, defaultErr
	}
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "db." + name, Fn: fn}))
	}

	set("open", func(args []Value) ([]Value, error) {
		conn, err := openDBFromArgs(args, opts)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{TableValue(dbConnectionTable(conn))}, nil
	})
	set("memory", func(args []Value) ([]Value, error) {
		conn, err := openSQLite(":memory:", opts)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{TableValue(dbConnectionTable(conn))}, nil
	})
	set("default", func(args []Value) ([]Value, error) {
		conn, err := getDefault()
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{TableValue(dbConnectionTable(conn))}, nil
	})
	for _, name := range []string{"exec", "query", "one", "frame"} {
		method := name
		set(method, func(args []Value) ([]Value, error) {
			conn, err := getDefault()
			if err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return conn.call(method, args)
		})
	}
	return t
}

func dbConnectionTable(conn *dbConnection) *Table {
	t := NewTable()
	t.RawSetString("__db_connection", BoolValue(true))
	for _, name := range []string{"exec", "query", "one", "frame"} {
		method := name
		t.RawSetString(method, FunctionValue(&GoFunction{
			Name: "db.connection." + method,
			Fn: func(args []Value) ([]Value, error) {
				return conn.call(method, args)
			},
		}))
	}
	t.RawSetString("close", FunctionValue(&GoFunction{
		Name: "db.connection.close",
		Fn: func(args []Value) ([]Value, error) {
			if err := conn.db.Close(); err != nil {
				return []Value{NilValue(), StringValue(err.Error())}, nil
			}
			return []Value{BoolValue(true)}, nil
		},
	}))
	return t
}

func (conn *dbConnection) call(name string, args []Value) ([]Value, error) {
	switch name {
	case "exec":
		return conn.exec(args)
	case "query":
		return conn.query(args)
	case "frame":
		return conn.frame(args)
	case "one":
		rows, err := conn.query(args)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 || !rows[0].IsTable() {
			return []Value{NilValue()}, nil
		}
		first := rows[0].Table().RawGetInt(1)
		if first.IsNil() {
			return []Value{NilValue()}, nil
		}
		return []Value{first}, nil
	default:
		return nil, fmt.Errorf("unknown db method %q", name)
	}
}

func (conn *dbConnection) exec(args []Value) ([]Value, error) {
	query, sqlArgs, err := dbSQLInput(args)
	if err != nil {
		return nil, err
	}
	result, err := conn.db.Exec(query, sqlArgs...)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	if affected, err := result.RowsAffected(); err == nil {
		out.RawSetString("rows_affected", IntValue(affected))
	}
	if id, err := result.LastInsertId(); err == nil {
		out.RawSetString("last_insert_id", IntValue(id))
	}
	return []Value{TableValue(out)}, nil
}

func (conn *dbConnection) query(args []Value) ([]Value, error) {
	query, sqlArgs, err := dbSQLInput(args)
	if err != nil {
		return nil, err
	}
	rows, err := conn.db.Query(query, sqlArgs...)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	defer rows.Close()
	table, err := leiaRows(rows)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(table)}, nil
}

func (conn *dbConnection) frame(args []Value) ([]Value, error) {
	query, sqlArgs, err := dbSQLInput(args)
	if err != nil {
		return nil, err
	}
	rows, err := conn.db.Query(query, sqlArgs...)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	defer rows.Close()
	frame, err := leiaFrame(rows)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(frame)}, nil
}

func openDBFromArgs(args []Value, opts HostOptions) (*dbConnection, error) {
	if len(args) == 0 || args[0].IsNil() {
		return openSQLite(":memory:", opts)
	}
	if args[0].IsString() {
		return openSQLite(args[0].Str(), opts)
	}
	if !args[0].IsTable() {
		return nil, fmt.Errorf("db.open: string or options table expected")
	}
	t := args[0].Table()
	driver := "sqlite"
	if v := t.RawGetString("driver"); v.IsString() && v.Str() != "" {
		driver = v.Str()
	}
	if driver != "sqlite" && driver != "sqlite3" {
		return nil, fmt.Errorf("db.open: built-in runtime supports sqlite only")
	}
	dsn := firstSQLString(t, "dsn", "path", "database", "file")
	if dsn == "" {
		dsn = ":memory:"
	}
	return openSQLite(dsn, opts)
}

func openSQLite(dsn string, opts HostOptions) (*dbConnection, error) {
	resolved, err := resolveDBDSN(dsn, opts)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", resolved)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &dbConnection{db: db}, nil
}

func resolveDBDSN(dsn string, opts HostOptions) (string, error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" || dsn == ":memory:" || strings.HasPrefix(dsn, "file:") {
		return dsn, nil
	}
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil || u.Scheme != "" {
			return dsn, nil
		}
	}
	if !HostBool(opts.FilesystemRead, true) || !HostBool(opts.FilesystemWrite, true) {
		return "", fmt.Errorf("database filesystem access disabled")
	}
	root := HostString(opts.FilesystemRoot)
	if root == "" && HostString(opts.ScriptDir) != "" && !filepath.IsAbs(dsn) {
		root = HostString(opts.ScriptDir)
	}
	return resolveSandboxPath(root, dsn)
}

func dbSQLInput(args []Value) (string, []any, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("bad argument #1 to 'db' (SQL string or sql value expected)")
	}
	query := ""
	var argTable *Table
	first := args[0]
	if first.IsString() {
		query = first.Str()
	} else if first.IsTable() {
		t := first.Table()
		query = firstSQLString(t, "query", "text", "sql")
		if v := firstSQLParams(t); !v.IsNil() {
			if !v.IsTable() {
				return "", nil, fmt.Errorf("bad argument #1 to 'db' (args table expected)")
			}
			argTable = v.Table()
		}
	} else {
		return "", nil, fmt.Errorf("bad argument #1 to 'db' (SQL string or sql value expected)")
	}
	if strings.TrimSpace(query) == "" {
		return "", nil, fmt.Errorf("db: SQL query string required")
	}
	if len(args) >= 2 {
		if !args[1].IsTable() {
			return "", nil, fmt.Errorf("bad argument #2 to 'db' (args table expected)")
		}
		argTable = args[1].Table()
	}
	sqlArgs, err := dbArgs(argTable)
	if err != nil {
		return "", nil, err
	}
	return query, sqlArgs, nil
}

func firstSQLParams(tbl *Table) Value {
	for _, key := range []string{"args", "params", "bindings"} {
		value := tbl.RawGetString(key)
		if !value.IsNil() {
			return value
		}
	}
	return NilValue()
}

func dbArgs(args *Table) ([]any, error) {
	if args == nil {
		return nil, nil
	}
	out := make([]any, 0, args.Length())
	for i := int64(1); i <= int64(args.Length()); i++ {
		val := args.RawGetInt(i)
		if val.IsNil() {
			out = append(out, nil)
			continue
		}
		arg, err := leiaToSQLValue(val)
		if err != nil {
			return nil, fmt.Errorf("db args[%d]: %w", i, err)
		}
		out = append(out, arg)
	}
	return out, nil
}

func leiaToSQLValue(v Value) (any, error) {
	switch {
	case v.IsNil():
		return nil, nil
	case v.IsBool():
		return v.Bool(), nil
	case v.IsInt():
		return v.Int(), nil
	case v.IsFloat():
		return v.Float(), nil
	case v.IsString():
		return v.Str(), nil
	default:
		return nil, fmt.Errorf("nil, bool, number, or string expected")
	}
}

func leiaRows(rows *sql.Rows) (*Table, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	columnTypes, _ := rows.ColumnTypes()
	out := NewAppendArrayTable(0)
	rowIndex := int64(1)
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := NewTableSized(len(columns), len(columns))
		for i, col := range columns {
			val := sqlToLeiaFrameValue(values[i], dbColumnTypeName(columnTypes, i))
			row.RawSetString(col, val)
			row.RawSetInt(int64(i+1), val)
		}
		out.RawSetInt(rowIndex, TableValue(row))
		rowIndex++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func leiaFrame(rows *sql.Rows) (*Table, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	columnTypes, _ := rows.ColumnTypes()
	scanned := make([][]Value, 0)
	rowTable := NewAppendArrayTable(0)
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		record := make([]Value, len(columns))
		row := NewTableSized(len(columns), len(columns))
		for i, col := range columns {
			val := sqlToLeiaFrameValue(values[i], dbColumnTypeName(columnTypes, i))
			record[i] = val
			row.RawSetString(col, val)
			row.RawSetInt(int64(i+1), val)
		}
		scanned = append(scanned, record)
		rowTable.RawSetInt(int64(len(scanned)), TableValue(row))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	columnNames := NewAppendArrayTable(len(columns))
	columnsTable := NewTable()
	numericColumns := NewAppendArrayTable(0)
	numericTable := NewTable()
	schemaColumns := NewAppendArrayTable(len(columns))
	kindsTable := NewTable()
	soaColumns := make(map[string]*DenseArray)

	for colIndex, name := range columns {
		columnNames.RawSetInt(int64(colIndex+1), StringValue(name))
		values := make([]Value, len(scanned))
		for rowIndex, record := range scanned {
			values[rowIndex] = record[colIndex]
		}
		columnValue := dbColumnValue(values)
		kind := dbColumnKind(columnValue)
		columnsTable.RawSetString(name, columnValue)
		kindsTable.RawSetString(name, StringValue(kind))
		schemaColumns.RawSetInt(int64(colIndex+1), TableValue(dbSchemaColumn(name, kind, columnValue, values)))
		if dbColumnIsNumericDense(columnValue) {
			numericTable.RawSetString(name, columnValue)
			numericColumns.RawSetInt(int64(numericColumns.Length()+1), StringValue(name))
		}
		if columnValue.IsDenseArray() {
			soaColumns[name] = columnValue.DenseArray()
		}
	}

	schema := NewTable()
	schema.RawSetString("columns", TableValue(schemaColumns))
	schema.RawSetString("names", TableValue(columnNames))
	schema.RawSetString("kinds", TableValue(kindsTable))

	shape := NewTable()
	shape.RawSetString("rows", IntValue(int64(len(scanned))))
	shape.RawSetString("columns", IntValue(int64(len(columns))))

	frame := NewTable()
	frame.RawSetString("kind", StringValue("data_frame"))
	frame.RawSetString("type", StringValue("data_frame"))
	frame.RawSetString("data", TableValue(columnsTable))
	frame.RawSetString("schema", TableValue(schema))
	frame.RawSetString("shape", TableValue(shape))
	frame.RawSetString("nrows", IntValue(int64(len(scanned))))
	frame.RawSetString("ncols", IntValue(int64(len(columns))))
	frame.RawSetString("rows", TableValue(rowTable))
	frame.RawSetString("columns", TableValue(columnsTable))
	frame.RawSetString("column_names", TableValue(columnNames))
	frame.RawSetString("numeric", TableValue(numericTable))
	frame.RawSetString("numeric_columns", TableValue(numericColumns))
	frame.RawSetString("len", IntValue(int64(len(scanned))))
	if len(soaColumns) > 0 {
		soa, err := NewSoA(soaColumns)
		if err != nil {
			return nil, err
		}
		frame.RawSetString("soa", SoAValue(soa))
	}
	return frame, nil
}

func dbColumnTypeName(columnTypes []*sql.ColumnType, index int) string {
	if index < 0 || index >= len(columnTypes) || columnTypes[index] == nil {
		return ""
	}
	return strings.ToLower(columnTypes[index].DatabaseTypeName())
}

func sqlToLeiaFrameValue(v any, columnType string) Value {
	if dbColumnTypeIsBool(columnType) {
		switch x := v.(type) {
		case bool:
			return BoolValue(x)
		case int64:
			return BoolValue(x != 0)
		case float64:
			return BoolValue(x != 0)
		case []byte:
			return BoolValue(string(x) != "" && string(x) != "0")
		case string:
			return BoolValue(x != "" && x != "0")
		}
	}
	return sqlToLeiaValue(v)
}

func dbColumnTypeIsBool(columnType string) bool {
	switch strings.ToLower(columnType) {
	case "bool", "boolean":
		return true
	default:
		return false
	}
}

func dbColumnValue(values []Value) Value {
	if len(values) == 0 {
		return TableValue(NewAppendArrayTable(0))
	}
	allInt := true
	allNumber := true
	allBool := true
	for _, v := range values {
		if !v.IsInt() {
			allInt = false
		}
		if !v.IsNumber() {
			allNumber = false
		}
		if !v.IsBool() {
			allBool = false
		}
	}
	switch {
	case allInt:
		xs := make([]int64, len(values))
		for i, v := range values {
			xs[i] = v.Int()
		}
		return DenseArrayValue(NewDenseArrayI64(xs))
	case allNumber:
		xs := make([]float64, len(values))
		for i, v := range values {
			xs[i] = v.Number()
		}
		return DenseArrayValue(NewDenseArrayF64(xs))
	case allBool:
		out, err := NewDenseArrayOfLen(DenseArrayBool, len(values))
		if err != nil {
			break
		}
		for i, v := range values {
			if err := out.Set(i, v); err != nil {
				return dbPlainColumn(values)
			}
		}
		return DenseArrayValue(out)
	default:
		return dbPlainColumn(values)
	}
	return dbPlainColumn(values)
}

func dbPlainColumn(values []Value) Value {
	out := NewAppendArrayTable(len(values))
	for i, v := range values {
		out.RawSetInt(int64(i+1), v)
	}
	return TableValue(out)
}

func dbColumnKind(columnValue Value) string {
	if columnValue.IsDenseArray() {
		return columnValue.DenseArray().DType().String()
	}
	if !columnValue.IsTable() {
		return "any"
	}
	values := columnValue.Table()
	kind := ""
	for i := int64(1); i <= int64(values.Length()); i++ {
		valueKind := dbScalarKind(values.RawGetInt(i))
		if valueKind == "null" {
			continue
		}
		if kind == "" {
			kind = valueKind
			continue
		}
		if kind != valueKind {
			return "any"
		}
	}
	if kind == "" {
		return "any"
	}
	return kind
}

func dbSchemaColumn(name, kind string, columnValue Value, values []Value) *Table {
	out := NewTable()
	out.RawSetString("name", StringValue(name))
	out.RawSetString("kind", StringValue(kind))
	out.RawSetString("nullable", BoolValue(dbColumnNullable(values)))
	out.RawSetString("dense", BoolValue(columnValue.IsDenseArray()))
	if columnValue.IsDenseArray() {
		out.RawSetString("dtype", StringValue(columnValue.DenseArray().DType().String()))
	} else {
		out.RawSetString("dtype", StringValue(kind))
	}
	return out
}

func dbColumnNullable(values []Value) bool {
	for _, value := range values {
		if value.IsNil() {
			return true
		}
	}
	return false
}

func dbColumnIsNumericDense(value Value) bool {
	if !value.IsDenseArray() {
		return false
	}
	switch value.DenseArray().DType() {
	case DenseArrayI64, DenseArrayF64:
		return true
	default:
		return false
	}
}

func dbScalarKind(value Value) string {
	switch {
	case value.IsNil():
		return "null"
	case value.IsBool():
		return "bool"
	case value.IsInt():
		return "i64"
	case value.IsFloat():
		return "f64"
	case value.IsString():
		return "string"
	default:
		return "any"
	}
}

func sqlToLeiaValue(v any) Value {
	switch x := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(x)
	case int64:
		return IntValue(x)
	case float64:
		return FloatValue(x)
	case string:
		return StringValue(x)
	case []byte:
		return StringValue(string(x))
	case time.Time:
		return StringValue(x.Format(time.RFC3339Nano))
	default:
		return StringValue(fmt.Sprint(x))
	}
}
