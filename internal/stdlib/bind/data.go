package bind

import (
	"fmt"
	"sort"
	"strings"

	stddata "github.com/never-labs/leia/internal/stdlib/lib/data"
)

const dataFrameMarker = "__data_frame"
const dataColumnMarker = "__data_column"
const dataNullMarker = "__data_null"
const dataNullKindMarker = "__data_null_kind"

// BuildData creates the "data" standard library table.
func BuildData() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "data." + name, Fn: fn}))
	}
	set("frame", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("data.frame: argument 1 is required")
		}
		frame, err := dataFrameValue(args[0])
		if err != nil {
			return nil, err
		}
		return []Value{frame}, nil
	})
	set("bool", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.bool", stddata.KindBool, args)
	})
	set("i64", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.i64", stddata.KindI64, args)
	})
	set("i8", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.i8", stddata.KindI8, args)
	})
	set("i16", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.i16", stddata.KindI16, args)
	})
	set("i32", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.i32", stddata.KindI32, args)
	})
	set("u8", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.u8", stddata.KindU8, args)
	})
	set("u16", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.u16", stddata.KindU16, args)
	})
	set("u32", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.u32", stddata.KindU32, args)
	})
	set("u64", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.u64", stddata.KindU64, args)
	})
	set("f64", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.f64", stddata.KindF64, args)
	})
	set("f32", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.f32", stddata.KindF32, args)
	})
	set("string", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.string", stddata.KindString, args)
	})
	set("strings", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.strings", stddata.KindString, args)
	})
	set("symbols", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.symbols", stddata.KindSymbol, args)
	})
	set("month", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.month", stddata.KindMonth, args)
	})
	set("date", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.date", stddata.KindDate, args)
	})
	set("datetime", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.datetime", stddata.KindDateTime, args)
	})
	set("timespan", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.timespan", stddata.KindTimespan, args)
	})
	set("minute", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.minute", stddata.KindMinute, args)
	})
	set("second", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.second", stddata.KindSecond, args)
	})
	set("time", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.time", stddata.KindTime, args)
	})
	set("timestamp", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.timestamp", stddata.KindTimestamp, args)
	})
	t.RawSetString("null", dataNullValue())
	set("is_null", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("data.is_null: argument 1 is required")
		}
		return []Value{BoolValue(args[0].IsNil() || isDataNullValue(args[0]))}, nil
	})
	set("rows", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.rows", args, 0)
		if err != nil {
			return nil, err
		}
		rows, err := dataFrameRows(frame)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(rows)}, nil
	})
	set("len", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.len", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{frame.RawGetString("len")}, nil
	})
	set("columns", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.columns", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(copyDataColumnNames(frame))}, nil
	})
	set("kinds", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.kinds", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(copyDataColumnKinds(frame))}, nil
	})
	set("schema_hash", func(args []Value) ([]Value, error) {
		frame, err := dataLibFrameArg("data.schema_hash", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{StringValue(frame.SchemaFingerprint())}, nil
	})
	set("take", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.take", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsInt() {
			return nil, fmt.Errorf("data.take: argument 2 must be row count")
		}
		taken, err := dataFrameTake(frame, int(args[1].Int()))
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(taken)}, nil
	})
	set("gather", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.gather", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("data.gather: argument 2 must be an index table")
		}
		gathered, err := dataFrameGather(frame, args[1].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(gathered)}, nil
	})
	set("project", func(args []Value) ([]Value, error) {
		frame, err := requireDataFrame("data.project", args, 0)
		if err != nil {
			return nil, err
		}
		names, err := dataProjectNames(args[1:])
		if err != nil {
			return nil, err
		}
		projected, err := dataFrameProject(frame, names)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(projected)}, nil
	})
	set("same_schema", func(args []Value) ([]Value, error) {
		left, err := requireDataFrame("data.same_schema", args, 0)
		if err != nil {
			return nil, err
		}
		right, err := requireDataFrame("data.same_schema", args, 1)
		if err != nil {
			return nil, err
		}
		return []Value{BoolValue(dataFrameSameSchema(left, right))}, nil
	})
	set("save", func(args []Value) ([]Value, error) {
		frame, err := dataLibFrameArg("data.save", args, 0)
		if err != nil {
			return nil, err
		}
		path, err := dataPathStringArg("data.save", args, 1, "path")
		if err != nil {
			return nil, err
		}
		if err := stddata.SaveFrameDir(path, frame); err != nil {
			return nil, fmt.Errorf("data.save: %w", err)
		}
		return []Value{BoolValue(true)}, nil
	})
	set("load", func(args []Value) ([]Value, error) {
		path, err := dataPathStringArg("data.load", args, 0, "path")
		if err != nil {
			return nil, err
		}
		frame, err := stddata.LoadFrameDir(path)
		if err != nil {
			return nil, fmt.Errorf("data.load: %w", err)
		}
		out, err := dataFrameValueFromLib(frame)
		if err != nil {
			return nil, fmt.Errorf("data.load: %w", err)
		}
		return []Value{out}, nil
	})
	set("info", func(args []Value) ([]Value, error) {
		path, err := dataPathStringArg("data.info", args, 0, "path")
		if err != nil {
			return nil, err
		}
		if info, err := stddata.ReadPartitionedStoreInfo(path); err == nil {
			return []Value{TableValue(dataPartitionedInfoTable(info))}, nil
		}
		info, err := stddata.ReadFrameStoreInfo(path)
		if err != nil {
			return nil, fmt.Errorf("data.info: %w", err)
		}
		return []Value{TableValue(dataFrameInfoTable(info))}, nil
	})
	set("save_partitioned", func(args []Value) ([]Value, error) {
		frame, err := dataLibFrameArg("data.save_partitioned", args, 0)
		if err != nil {
			return nil, err
		}
		path, err := dataPathStringArg("data.save_partitioned", args, 1, "path")
		if err != nil {
			return nil, err
		}
		cols, err := dataPartitionColumnArgs("data.save_partitioned", args[2:])
		if err != nil {
			return nil, err
		}
		if err := stddata.SavePartitionedFrameDir(path, frame, cols...); err != nil {
			return nil, fmt.Errorf("data.save_partitioned: %w", err)
		}
		return []Value{BoolValue(true)}, nil
	})
	set("load_partitioned", func(args []Value) ([]Value, error) {
		path, err := dataPathStringArg("data.load_partitioned", args, 0, "path")
		if err != nil {
			return nil, err
		}
		filters, err := dataPartitionFilters("data.load_partitioned", args[1:])
		if err != nil {
			return nil, err
		}
		frame, err := stddata.LoadPartitionedFrameDir(path, filters)
		if err != nil {
			return nil, fmt.Errorf("data.load_partitioned: %w", err)
		}
		out, err := dataFrameValueFromLib(frame)
		if err != nil {
			return nil, fmt.Errorf("data.load_partitioned: %w", err)
		}
		return []Value{out}, nil
	})
	return t
}

func dataNullValue() Value {
	return dataTypedNullValue("")
}

func dataTypedNullValue(kind stddata.Kind) Value {
	t := NewTable()
	t.RawSetString(dataNullMarker, BoolValue(true))
	if kind != "" && kind != stddata.KindAny && kind != stddata.KindNull {
		t.RawSetString(dataNullKindMarker, StringValue(string(kind)))
	}
	return TableValue(t)
}

func isDataNullValue(v Value) bool {
	return v.IsTable() && v.Table().RawGetString(dataNullMarker).Truthy()
}

func dataNullValueKind(v Value) stddata.Kind {
	if !v.IsTable() {
		return ""
	}
	kind := v.Table().RawGetString(dataNullKindMarker)
	if !kind.IsString() {
		return ""
	}
	return stddata.Kind(kind.Str())
}

func dataColumnConstructor(name string, kind stddata.Kind, args []Value) ([]Value, error) {
	if len(args) < 1 {
		return nil, fmt.Errorf("%s: argument 1 is required", name)
	}
	vector := args[0]
	if !isDataVectorValue(vector) {
		return nil, fmt.Errorf("%s: argument 1 must be an array table or dense array", name)
	}
	if err := dataValidateColumnKind(name, kind, vector); err != nil {
		return nil, err
	}
	return []Value{dataColumnValue(kind, vector)}, nil
}

func dataColumnValue(kind stddata.Kind, vector Value) Value {
	t := NewTable()
	t.RawSetString(dataColumnMarker, BoolValue(true))
	t.RawSetString("kind", StringValue(string(kind)))
	t.RawSetString("values", vector)
	return TableValue(t)
}

func dataValidateColumnKind(name string, kind stddata.Kind, vector Value) error {
	n, err := dataRawVectorLen(vector)
	if err != nil {
		return err
	}
	for i := 0; i < n; i++ {
		v, err := dataRawVectorAt(vector, i)
		if err != nil {
			return err
		}
		if v.IsNil() || isDataNullValue(v) {
			continue
		}
		switch kind {
		case stddata.KindBool:
			if !v.IsBool() {
				return fmt.Errorf("%s: item %d must be bool or nil", name, i+1)
			}
		case stddata.KindI64, stddata.KindI8, stddata.KindI16, stddata.KindI32, stddata.KindU8, stddata.KindU16, stddata.KindU32, stddata.KindU64:
			if !v.IsInt() {
				return fmt.Errorf("%s: item %d must be int or nil", name, i+1)
			}
			if !dataIntInKindRange(kind, v.Int()) {
				return fmt.Errorf("%s: item %d out of range for %s", name, i+1, kind)
			}
		case stddata.KindF64, stddata.KindF32:
			if !v.IsNumber() {
				return fmt.Errorf("%s: item %d must be number or nil", name, i+1)
			}
		case stddata.KindString, stddata.KindSymbol:
			if !v.IsString() {
				return fmt.Errorf("%s: item %d must be string or nil", name, i+1)
			}
		case stddata.KindMonth, stddata.KindDate, stddata.KindDateTime, stddata.KindTimespan,
			stddata.KindMinute, stddata.KindSecond, stddata.KindTime, stddata.KindTimestamp:
			if !dataTemporalScalar(v) {
				return fmt.Errorf("%s: item %d must be scalar or nil", name, i+1)
			}
		default:
			return fmt.Errorf("%s: unsupported column kind %q", name, kind)
		}
	}
	return nil
}

func dataTemporalScalar(v Value) bool {
	return v.IsBool() || v.IsInt() || v.IsFloat() || v.IsString()
}

func dataFrameValue(v Value) (Value, error) {
	if v.IsSoA() {
		return dataFrameFromSoA(v.SoA())
	}
	if !v.IsTable() {
		return NilValue(), fmt.Errorf("data.frame: argument 1 must be a column table, row table, frame, or soa")
	}
	tbl := v.Table()
	if isDataFrameTable(tbl) {
		return v, nil
	}
	if rows, ok := rowsTableToColumns(tbl); ok {
		return dataFrameFromColumns(rows)
	}
	return dataFrameFromColumns(tbl)
}

func dataFrameFromSoA(s *SoA) (Value, error) {
	if s == nil {
		return NilValue(), fmt.Errorf("data.frame: soa is nil")
	}
	cols := NewTable()
	for _, name := range s.ColumnNames() {
		col, ok := s.Column(name)
		if !ok {
			return NilValue(), fmt.Errorf("data.frame: soa column %q not found", name)
		}
		cols.RawSetString(name, DenseArrayValue(col))
	}
	return dataFrameFromColumns(cols)
}

func dataFrameFromColumns(cols *Table) (Value, error) {
	if cols == nil {
		return NilValue(), fmt.Errorf("data.frame: columns must be a table")
	}
	names := dataPlainStringKeys(cols)
	if names == nil {
		return NilValue(), fmt.Errorf("data.frame: columns must be a plain string-keyed table of vectors")
	}
	values := make(map[string]Value)
	kinds := make(map[string]string)
	for _, name := range names {
		val := cols.RawGetString(name)
		if !isDataColumnValue(val) {
			return NilValue(), fmt.Errorf("data.frame: columns must be a plain string-keyed table of vectors")
		}
		values[name] = dataColumnWrappedValues(val)
		kinds[name] = string(dataColumnKind(val))
	}
	libCols := make([]stddata.Column, 0, len(names))
	for _, name := range names {
		items, err := dataColumnAnyValuesForKind(values[name], stddata.Kind(kinds[name]))
		if err != nil {
			return NilValue(), fmt.Errorf("data.frame column %q: %w", name, err)
		}
		col, err := stddata.NewColumnWithKind(stddata.Symbol(name), stddata.Kind(kinds[name]), items)
		if err != nil {
			return NilValue(), fmt.Errorf("data.frame column %q: %w", name, err)
		}
		libCols = append(libCols, col)
	}
	frame, err := stddata.NewFrame(libCols...)
	if err != nil {
		return NilValue(), err
	}
	out := NewTable()
	out.RawSetString(dataFrameMarker, BoolValue(true))
	out.RawSetString("len", IntValue(int64(frame.Len())))
	out.RawSetString("columns", TableValue(copyDataColumns(names, values)))
	out.RawSetString("column_names", TableValue(dataColumnNamesTable(names)))
	out.RawSetString("column_kinds", TableValue(dataColumnKindsTable(names, kinds)))
	out.RawSetString("schema", TableValue(dataSchemaTable(names, kinds)))
	out.RawSetString("row", FunctionValue(&GoFunction{Name: "data.frame.row", Fn: dataFrameRowMethod(out)}))
	out.RawSetString("gather", FunctionValue(&GoFunction{Name: "data.frame.gather", Fn: dataFrameGatherMethod(out)}))
	for _, name := range names {
		out.RawSetString(name, values[name])
	}
	dataDecorateFrameTable(out, nil)
	setDataFrameNativePayload(out, frame)
	return TableValue(out), nil
}

func dataDecorateFrameTable(frame, rows *Table) {
	if frame == nil {
		return
	}
	nrows := int64(0)
	if lenValue := frame.RawGetString("len"); lenValue.IsInt() {
		nrows = lenValue.Int()
	}
	ncols := int64(0)
	if names := frame.RawGetString("column_names"); names.IsTable() {
		ncols = int64(names.Table().Length())
	}
	columns := frame.RawGetString("columns")
	frame.RawSetString("kind", StringValue("data_frame"))
	frame.RawSetString("type", StringValue("data_frame"))
	if columns.IsTable() {
		frame.RawSetString("data", columns)
	}
	if rows != nil {
		frame.RawSetString("rows", TableValue(rows))
		for i := 1; i <= rows.Length(); i++ {
			frame.RawSetInt(int64(i), rows.RawGetInt(int64(i)))
		}
	}
	frame.RawSetString("nrows", IntValue(nrows))
	frame.RawSetString("ncols", IntValue(ncols))
	shape := NewTable()
	shape.RawSetString("rows", IntValue(nrows))
	shape.RawSetString("columns", IntValue(ncols))
	frame.RawSetString("shape", TableValue(shape))
	if rows == nil {
		dataInstallLazyFrameRows(frame, int(nrows))
	}
}

func dataInstallLazyFrameRows(frame *Table, nrows int) {
	if frame == nil || nrows <= 0 {
		return
	}
	cache := make(map[int64]Value)
	getRow := func(key int64) (Value, bool) {
		if key < 1 || key > int64(nrows) {
			return NilValue(), false
		}
		if row, ok := cache[key]; ok {
			return row, true
		}
		row, err := dataFrameRow(frame, int(key))
		if err != nil {
			return NilValue(), false
		}
		value := TableValue(row)
		cache[key] = value
		return value, true
	}
	rows := NewTable()
	rows.RawSetString("len", IntValue(int64(nrows)))
	rows.SetLazyIntGetter(nrows, getRow)
	frame.RawSetString("rows", TableValue(rows))
	frame.SetLazyIntGetter(nrows, getRow)
}

func dataSchemaTable(names []string, kinds map[string]string) *Table {
	schema := NewTable()
	schema.RawSetString("names", TableValue(dataColumnNamesTable(names)))
	schema.RawSetString("kinds", TableValue(dataColumnKindsTable(names, kinds)))
	if frame, err := dataFrameFromSchemaParts(names, kinds, 0); err == nil {
		schema.RawSetString("hash", StringValue(frame.SchemaFingerprint()))
	}
	return schema
}

func dataFrameFromSchemaParts(names []string, kinds map[string]string, rows int) (stddata.Frame, error) {
	cols := make([]stddata.Column, 0, len(names))
	for _, name := range names {
		kind := stddata.Kind(kinds[name])
		values := make([]any, rows)
		for i := range values {
			values[i] = stddata.NullForKind(kind)
		}
		col, err := stddata.NewColumnWithKind(stddata.Symbol(name), kind, values)
		if err != nil {
			return stddata.Frame{}, err
		}
		cols = append(cols, col)
	}
	return stddata.NewFrame(cols...)
}

func dataFrameRowMethod(frame *Table) func([]Value) ([]Value, error) {
	return func(args []Value) ([]Value, error) {
		if len(args) > 0 && args[0].IsTable() && args[0].Table() == frame {
			args = args[1:]
		}
		if len(args) < 1 || !args[0].IsInt() {
			return nil, fmt.Errorf("data.frame.row: argument 1 must be row index")
		}
		row, err := dataFrameRow(frame, int(args[0].Int()))
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(row)}, nil
	}
}

func dataFrameGatherMethod(frame *Table) func([]Value) ([]Value, error) {
	return func(args []Value) ([]Value, error) {
		if len(args) > 0 && args[0].IsTable() && args[0].Table() == frame {
			args = args[1:]
		}
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("data.frame.gather: argument 1 must be an index table")
		}
		gathered, err := dataFrameGather(frame, args[0].Table())
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(gathered)}, nil
	}
}

func dataFrameRow(frame *Table, index int) (*Table, error) {
	if index < 1 || index > int(frame.RawGetString("len").Int()) {
		return nil, fmt.Errorf("data.frame.row: row %d out of range", index)
	}
	cols := frame.RawGetString("columns").Table()
	row := NewTable()
	for _, name := range dataColumnNames(frame) {
		v, err := dataVectorAt(cols.RawGetString(name), index-1)
		if err != nil {
			return nil, err
		}
		row.RawSetString(name, v)
	}
	return row, nil
}

func dataFrameGather(frame *Table, indexes *Table) (*Table, error) {
	cols := frame.RawGetString("columns").Table()
	outCols := NewTable()
	for _, name := range dataColumnNames(frame) {
		src := cols.RawGetString(name)
		out := NewAppendArrayTable(indexes.Length())
		for i := 1; i <= indexes.Length(); i++ {
			idx := indexes.RawGetInt(int64(i))
			if !idx.IsInt() {
				return nil, fmt.Errorf("data.frame.gather: index %d must be int", i)
			}
			v, err := dataVectorAt(src, int(idx.Int())-1)
			if err != nil {
				return nil, err
			}
			out.RawSetInt(int64(i), v)
		}
		kind := stddata.Kind(dataColumnKinds(frame)[name])
		if kind == "" || kind == stddata.KindAny {
			outCols.RawSetString(name, TableValue(out))
		} else {
			outCols.RawSetString(name, dataColumnValue(kind, TableValue(out)))
		}
	}
	frameValue, err := dataFrameFromColumns(outCols)
	if err != nil {
		return nil, err
	}
	return frameValue.Table(), nil
}

func dataFrameTake(frame *Table, n int) (*Table, error) {
	if n < 0 {
		return nil, fmt.Errorf("data.take: row count must be non-negative")
	}
	frameLen := int(frame.RawGetString("len").Int())
	if n > frameLen {
		n = frameLen
	}
	indexes := NewAppendArrayTable(n)
	for i := 1; i <= n; i++ {
		indexes.RawSetInt(int64(i), IntValue(int64(i)))
	}
	return dataFrameGather(frame, indexes)
}

func dataFrameProject(frame *Table, names []string) (*Table, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("data.project: at least one column is required")
	}
	sourceCols := frame.RawGetString("columns")
	if !sourceCols.IsTable() {
		return nil, fmt.Errorf("data.project: frame columns are invalid")
	}
	kinds := dataColumnKinds(frame)
	outCols := NewTable()
	seen := map[string]struct{}{}
	for _, name := range names {
		if name == "" {
			return nil, fmt.Errorf("data.project: column name must not be empty")
		}
		if _, ok := seen[name]; ok {
			return nil, fmt.Errorf("data.project: column %q is duplicated", name)
		}
		value := sourceCols.Table().RawGetString(name)
		if value.IsNil() {
			return nil, fmt.Errorf("data.project: column %q does not exist", name)
		}
		kind := stddata.Kind(kinds[name])
		if kind == "" || kind == stddata.KindAny {
			outCols.RawSetString(name, value)
		} else {
			outCols.RawSetString(name, dataColumnValue(kind, value))
		}
		seen[name] = struct{}{}
	}
	frameValue, err := dataFrameFromColumns(outCols)
	if err != nil {
		return nil, err
	}
	return frameValue.Table(), nil
}

func dataProjectNames(args []Value) ([]string, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("data.project: at least one column name is required")
	}
	if len(args) == 1 && args[0].IsTable() {
		tbl := args[0].Table()
		names := make([]string, 0, tbl.Length())
		for i := 1; i <= tbl.Length(); i++ {
			name := tbl.RawGetInt(int64(i))
			if !name.IsString() {
				return nil, fmt.Errorf("data.project: column name %d must be string", i)
			}
			names = append(names, name.Str())
		}
		return names, nil
	}
	names := make([]string, 0, len(args))
	for i, arg := range args {
		if !arg.IsString() {
			return nil, fmt.Errorf("data.project: argument %d must be column name string", i+2)
		}
		names = append(names, arg.Str())
	}
	return names, nil
}

func dataFrameSameSchema(left, right *Table) bool {
	leftNames := dataColumnNames(left)
	rightNames := dataColumnNames(right)
	if len(leftNames) != len(rightNames) {
		return false
	}
	leftKinds := dataColumnKinds(left)
	rightKinds := dataColumnKinds(right)
	for i, name := range leftNames {
		if rightNames[i] != name || leftKinds[name] != rightKinds[name] {
			return false
		}
	}
	return true
}

func dataLibFrameArg(name string, args []Value, index int) (stddata.Frame, error) {
	if len(args) <= index {
		return stddata.Frame{}, fmt.Errorf("%s: argument %d is required", name, index+1)
	}
	frameValue, err := dataFrameValue(args[index])
	if err != nil {
		return stddata.Frame{}, fmt.Errorf("%s: %w", name, err)
	}
	return dataLibFrameFromTable(frameValue.Table())
}

func dataLibFrameFromTable(frame *Table) (stddata.Frame, error) {
	if frame == nil {
		return stddata.Frame{}, fmt.Errorf("argument 1 must be a data frame")
	}
	if native, ok, err := dataNativeFramePayload(frame); err != nil {
		return stddata.Frame{}, err
	} else if ok {
		return native, nil
	}
	if !isDataFrameMarkerTable(frame) {
		return stddata.Frame{}, fmt.Errorf("argument 1 must be a data frame")
	}
	names := dataColumnNames(frame)
	kinds := dataColumnKinds(frame)
	colsTable := frame.RawGetString("columns")
	if !colsTable.IsTable() {
		return stddata.Frame{}, fmt.Errorf("frame columns are invalid")
	}
	cols := make([]stddata.Column, 0, len(names))
	for _, name := range names {
		kind := stddata.Kind(kinds[name])
		vector := dataColumnWrappedValues(colsTable.Table().RawGetString(name))
		if native, ok := dataNativeArrayFromValue(vector); ok {
			cols = append(cols, stddata.Column{Name: stddata.Symbol(name), Data: native})
			continue
		}
		values, err := dataColumnAnyValuesForKind(vector, kind)
		if err != nil {
			return stddata.Frame{}, fmt.Errorf("column %q: %w", name, err)
		}
		col, err := stddata.NewColumnWithKind(stddata.Symbol(name), kind, values)
		if err != nil {
			return stddata.Frame{}, err
		}
		cols = append(cols, col)
	}
	return stddata.NewFrame(cols...)
}

func dataNativeFramePayload(frame *Table) (stddata.Frame, bool, error) {
	if frame == nil {
		return stddata.Frame{}, false, nil
	}
	if payload, info, ok := frame.NativeFramePayload(); ok {
		if info.Kind != NativePayloadDataFrame {
			return stddata.Frame{}, false, fmt.Errorf("argument 1 must be a data frame")
		}
		native, hasPayload := payload.(stddata.Frame)
		if !hasPayload {
			return stddata.Frame{}, false, fmt.Errorf("native data frame payload is invalid")
		}
		return native, true, nil
	}
	if kind, ok := frame.NativePayloadKind(); ok {
		if kind != NativePayloadDataFrame {
			return stddata.Frame{}, false, fmt.Errorf("argument 1 must be a data frame")
		}
		return stddata.Frame{}, false, fmt.Errorf("native data frame payload is invalid")
	}
	if native, ok := frame.NativePayload().(stddata.Frame); ok {
		return native, true, nil
	}
	return stddata.Frame{}, false, nil
}

func dataFrameValueFromLib(frame stddata.Frame) (Value, error) {
	return dataFrameFacadeValueFromLib(frame)
}

func dataFrameFacadeValueFromLib(frame stddata.Frame) (Value, error) {
	cols := NewTable()
	names := make([]string, 0, len(frame.Columns()))
	kinds := make(map[string]string, len(frame.Columns()))
	for _, col := range frame.Columns() {
		name := string(col.Name)
		names = append(names, name)
		kinds[name] = string(col.Data.Kind())
		cols.RawSetString(name, dataColumnValue(col.Data.Kind(), dataArrayFacadeValue(col.Data, dataValueFromAny)))
	}
	out := NewTable()
	out.RawSetString(dataFrameMarker, BoolValue(true))
	out.RawSetString("len", IntValue(int64(frame.Len())))
	out.RawSetString("columns", TableValue(cols))
	out.RawSetString("column_names", TableValue(dataColumnNamesTable(names)))
	out.RawSetString("column_kinds", TableValue(dataColumnKindsTable(names, kinds)))
	out.RawSetString("schema", TableValue(dataSchemaTable(names, kinds)))
	out.RawSetString("row", FunctionValue(&GoFunction{Name: "data.frame.row", Fn: dataFrameRowMethod(out)}))
	out.RawSetString("gather", FunctionValue(&GoFunction{Name: "data.frame.gather", Fn: dataFrameGatherMethod(out)}))
	for _, name := range names {
		out.RawSetString(name, dataColumnWrappedValues(cols.RawGetString(name)))
	}
	dataDecorateFrameTable(out, nil)
	setDataFrameNativePayload(out, frame)
	return TableValue(out), nil
}

func dataArrayFacadeValue(array stddata.Array, convert func(any) Value) Value {
	out := NewTable()
	if array == nil {
		return TableValue(out)
	}
	out.RawSetString("kind", StringValue(string(array.Kind())))
	out.RawSetString("type", StringValue("data_array"))
	out.RawSetString("len", IntValue(int64(array.Len())))
	cache := make(map[int64]Value)
	out.SetLazyIntGetter(array.Len(), func(key int64) (Value, bool) {
		if key < 1 || key > int64(array.Len()) {
			return NilValue(), false
		}
		if v, ok := cache[key]; ok {
			return v, true
		}
		item, ok := array.At(int(key - 1))
		if !ok {
			return NilValue(), false
		}
		if stddata.IsNull(item) {
			if kind := array.Kind(); kind != "" && kind != stddata.KindAny && kind != stddata.KindNull {
				item = stddata.NullForKind(kind)
			}
		}
		v := convert(item)
		cache[key] = v
		return v, true
	})
	meta := NewTable()
	meta.RawSetString("__tostring", FunctionValue(&GoFunction{Name: "data.array.__tostring", Fn: func(args []Value) ([]Value, error) {
		return []Value{StringValue(fmt.Sprint(array))}, nil
	}}))
	out.SetMetatable(meta)
	setDataArrayNativePayload(out, array)
	return TableValue(out)
}

func setDataFrameNativePayload(table *Table, frame stddata.Frame) {
	if table == nil {
		return
	}
	payload := any(frame)
	if soa, ok := dataFrameRuntimeSoA(frame); ok {
		payload = soa
	}
	table.SetNativePayloadWithInfo(payload, NativePayloadInfo{
		Kind:       NativePayloadDataFrame,
		Rows:       frame.Len(),
		Columns:    len(frame.Schema().Names()),
		SchemaHash: frame.SchemaFingerprint(),
	})
}

func dataFrameRuntimeSoA(frame stddata.Frame) (*SoA, bool) {
	names := frame.Schema().Names()
	if len(names) == 0 {
		return nil, false
	}
	cols := make(map[string]*DenseArray, len(names))
	for _, name := range names {
		array, ok := frame.Column(name)
		if !ok {
			return nil, false
		}
		col, ok := dataArrayRuntimeDense(array)
		if !ok {
			return nil, false
		}
		cols[string(name)] = col
	}
	soa, err := NewSoA(cols)
	if err != nil {
		return nil, false
	}
	return soa, true
}

func dataArrayRuntimeDense(array stddata.Array) (*DenseArray, bool) {
	if array == nil || dataArrayHasNull(array) {
		return nil, false
	}
	switch array.Kind() {
	case stddata.KindI64:
		xs := make([]int64, array.Len())
		for i := range xs {
			v, ok := array.At(i)
			if !ok {
				return nil, false
			}
			switch n := v.(type) {
			case int64:
				xs[i] = n
			default:
				return nil, false
			}
		}
		return NewDenseArrayI64(xs), true
	case stddata.KindF64:
		xs := make([]float64, array.Len())
		for i := range xs {
			v, ok := array.At(i)
			if !ok {
				return nil, false
			}
			switch n := v.(type) {
			case float64:
				xs[i] = n
			default:
				return nil, false
			}
		}
		return NewDenseArrayF64(xs), true
	case stddata.KindBool:
		out, err := NewDenseArrayOfLen(DenseArrayBool, array.Len())
		if err != nil {
			return nil, false
		}
		for i := 0; i < array.Len(); i++ {
			v, ok := array.At(i)
			if !ok {
				return nil, false
			}
			b, ok := v.(bool)
			if !ok {
				return nil, false
			}
			if err := out.Set(i, BoolValue(b)); err != nil {
				return nil, false
			}
		}
		return out, true
	default:
		return nil, false
	}
}

func setDataArrayNativePayload(table *Table, array stddata.Array) {
	if table == nil || array == nil {
		return
	}
	table.SetNativePayloadWithInfo(array, NativePayloadInfo{
		Kind:       NativePayloadDataColumn,
		Rows:       array.Len(),
		ColumnKind: string(array.Kind()),
	})
}

func dataNativeArrayFromValue(v Value) (stddata.Array, bool) {
	if !v.IsTable() {
		return nil, false
	}
	tbl := v.Table()
	if kind, ok := tbl.NativePayloadKind(); ok {
		if kind != NativePayloadDataColumn {
			return nil, false
		}
		array, ok := tbl.NativePayload().(stddata.Array)
		return array, ok
	}
	array, ok := tbl.NativePayload().(stddata.Array)
	return array, ok
}

func dataValueFromAny(v any) Value {
	if stddata.IsNull(v) {
		if kind, ok := stddata.NullKind(v); ok {
			return dataTypedNullValue(kind)
		}
		return dataNullValue()
	}
	switch x := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(x)
	case int:
		return IntValue(int64(x))
	case int8:
		return IntValue(int64(x))
	case int16:
		return IntValue(int64(x))
	case int32:
		return IntValue(int64(x))
	case int64:
		return IntValue(x)
	case uint8:
		return IntValue(int64(x))
	case uint16:
		return IntValue(int64(x))
	case uint32:
		return IntValue(int64(x))
	case uint64:
		return IntValue(int64(x))
	case float32:
		return FloatValue(float64(x))
	case float64:
		return FloatValue(x)
	case string:
		return StringValue(x)
	case stddata.Symbol:
		return StringValue(string(x))
	case stddata.Month:
		return IntValue(int64(x))
	case stddata.Date:
		return IntValue(int64(x))
	case stddata.DateTime:
		return IntValue(int64(x))
	case stddata.Timespan:
		return IntValue(int64(x))
	case stddata.Minute:
		return IntValue(int64(x))
	case stddata.Second:
		return IntValue(int64(x))
	case stddata.Time:
		return IntValue(int64(x))
	case stddata.Timestamp:
		return IntValue(int64(x))
	default:
		return StringValue(fmt.Sprint(x))
	}
}

func dataStringArg(name string, args []Value, index int, label string) (string, error) {
	if len(args) <= index || !args[index].IsString() {
		return "", fmt.Errorf("%s: argument %d must be %s string", name, index+1, label)
	}
	return args[index].Str(), nil
}

func dataPathStringArg(name string, args []Value, index int, label string) (string, error) {
	path, err := dataStringArg(name, args, index, label)
	if err != nil {
		return "", err
	}
	return qNormalizePathString(path), nil
}

func qPathStringArg(name string, args []Value, index int, label string) (string, error) {
	return dataPathStringArg(name, args, index, label)
}

func qNormalizePathString(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "`")
	path = strings.TrimPrefix(path, ":")
	return path
}

func dataPartitionColumnArgs(name string, args []Value) ([]stddata.Symbol, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("%s: at least one partition column is required", name)
	}
	var out []stddata.Symbol
	for i, arg := range args {
		if arg.IsTable() {
			tbl := arg.Table()
			for j := 1; j <= tbl.Length(); j++ {
				item := tbl.RawGetInt(int64(j))
				if !item.IsString() {
					return nil, fmt.Errorf("%s: partition column %d must be string", name, j)
				}
				out = append(out, stddata.Symbol(item.Str()))
			}
			continue
		}
		if !arg.IsString() {
			return nil, fmt.Errorf("%s: argument %d must be partition column string or table", name, i+3)
		}
		out = append(out, stddata.Symbol(arg.Str()))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: at least one partition column is required", name)
	}
	return out, nil
}

func dataPartitionFilters(name string, args []Value) (map[stddata.Symbol]any, error) {
	if len(args) == 0 || args[0].IsNil() {
		return nil, nil
	}
	if len(args) != 1 || !args[0].IsTable() {
		return nil, fmt.Errorf("%s: optional argument 2 must be filter table", name)
	}
	filters := map[stddata.Symbol]any{}
	ok := args[0].Table().ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		filters[stddata.Symbol(key.Str())] = dataValueAny(val)
		return true
	})
	if !ok {
		return nil, fmt.Errorf("%s: filter table must have string keys", name)
	}
	return filters, nil
}

func dataFrameInfoTable(info stddata.FrameStoreInfo) *Table {
	out := NewTable()
	out.RawSetString("format", StringValue(info.Format))
	out.RawSetString("version", IntValue(int64(info.Version)))
	out.RawSetString("rows", IntValue(int64(info.Rows)))
	out.RawSetString("columns", TableValue(dataStoredColumnsTable(info.Columns)))
	return out
}

func dataPartitionedInfoTable(info stddata.PartitionedStoreInfo) *Table {
	out := NewTable()
	out.RawSetString("format", StringValue(info.Format))
	out.RawSetString("version", IntValue(int64(info.Version)))
	out.RawSetString("rows", IntValue(int64(info.Rows)))
	out.RawSetString("partition_columns", TableValue(dataStringArrayTable(info.PartitionColumns)))
	out.RawSetString("columns", TableValue(dataStoredColumnsTable(info.Columns)))
	partitions := NewAppendArrayTable(len(info.Partitions))
	for i, part := range info.Partitions {
		row := NewTable()
		row.RawSetString("path", StringValue(part.Path))
		row.RawSetString("rows", IntValue(int64(part.Rows)))
		values := NewTable()
		for name, value := range part.Values {
			values.RawSetString(name, dataValueFromAny(value))
		}
		row.RawSetString("values", TableValue(values))
		partitions.RawSetInt(int64(i+1), TableValue(row))
	}
	out.RawSetString("partitions", TableValue(partitions))
	return out
}

func dataStoredColumnsTable(cols []stddata.StoredColumn) *Table {
	out := NewAppendArrayTable(len(cols))
	for i, col := range cols {
		row := NewTable()
		row.RawSetString("name", StringValue(col.Name))
		row.RawSetString("kind", StringValue(string(col.Kind)))
		row.RawSetString("file", StringValue(col.File))
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return out
}

func dataStringArrayTable(values []string) *Table {
	out := NewAppendArrayTable(len(values))
	for i, value := range values {
		out.RawSetInt(int64(i+1), StringValue(value))
	}
	return out
}

func rowsTableToColumns(rows *Table) (*Table, bool) {
	if rows == nil || rows.Length() == 0 {
		return nil, false
	}
	first := rows.RawGetInt(1)
	if !first.IsTable() {
		return nil, false
	}
	names := dataPlainStringKeys(first.Table())
	if names == nil {
		return nil, false
	}
	cols := NewTable()
	for _, name := range names {
		col := NewAppendArrayTable(rows.Length())
		for i := 1; i <= rows.Length(); i++ {
			row := rows.RawGetInt(int64(i))
			if !row.IsTable() {
				return nil, false
			}
			v := row.Table().RawGetString(name)
			if v.IsNil() {
				return nil, false
			}
			col.RawSetInt(int64(i), v)
		}
		cols.RawSetString(name, TableValue(col))
	}
	return cols, true
}

func requireDataFrame(name string, args []Value, index int) (*Table, error) {
	if len(args) <= index || !args[index].IsTable() || !isDataFrameTable(args[index].Table()) {
		return nil, fmt.Errorf("%s: argument %d must be a data frame", name, index+1)
	}
	return args[index].Table(), nil
}

func isDataFrameTable(t *Table) bool {
	if t == nil {
		return false
	}
	if t.IsFrameFacade() {
		return true
	}
	if _, ok := t.NativePayloadKind(); ok {
		return false
	}
	return isDataFrameMarkerTable(t)
}

func isDataFrameMarkerTable(t *Table) bool {
	return t != nil && t.RawGetString(dataFrameMarker).Truthy()
}

func isNativeDataFrameFacade(t *Table) bool {
	return t != nil && t.IsFrameFacade()
}

func isDataColumnValue(v Value) bool {
	return isDataVectorValue(v) || isDataColumnWrapper(v)
}

func isDataVectorValue(v Value) bool {
	return v.IsTable() || v.IsDenseArray()
}

func isDataColumnWrapper(v Value) bool {
	return v.IsTable() && v.Table().RawGetString(dataColumnMarker).Truthy()
}

func dataColumnWrappedValues(v Value) Value {
	if isDataColumnWrapper(v) {
		return v.Table().RawGetString("values")
	}
	return v
}

func dataColumnWrappedKind(v Value) (stddata.Kind, bool) {
	if !isDataColumnWrapper(v) {
		return "", false
	}
	kind := v.Table().RawGetString("kind")
	if !kind.IsString() {
		return "", false
	}
	return stddata.Kind(kind.Str()), true
}

func dataColumnKind(v Value) stddata.Kind {
	if kind, ok := dataColumnWrappedKind(v); ok {
		return kind
	}
	if v.IsDenseArray() {
		return stddata.Kind(v.DenseArray().DType().String())
	}
	return stddata.KindAny
}

func dataFrameRows(frame *Table) (*Table, error) {
	nv := frame.RawGetString("len")
	if !nv.IsInt() {
		return nil, fmt.Errorf("data.rows: frame len is invalid")
	}
	n := int(nv.Int())
	cols := frame.RawGetString("columns")
	if !cols.IsTable() {
		return nil, fmt.Errorf("data.rows: frame columns are invalid")
	}
	names := dataColumnNames(frame)
	out := NewAppendArrayTable(n)
	for i := 0; i < n; i++ {
		row := NewTable()
		for _, name := range names {
			v, err := dataVectorAt(cols.Table().RawGetString(name), i)
			if err != nil {
				return nil, fmt.Errorf("data.rows column %q: %w", name, err)
			}
			row.RawSetString(name, v)
		}
		out.RawSetInt(int64(i+1), TableValue(row))
	}
	return out, nil
}

func dataColumnKinds(frame *Table) map[string]string {
	kindsTable := frame.RawGetString("column_kinds")
	kinds := map[string]string{}
	if !kindsTable.IsTable() {
		return kinds
	}
	_ = kindsTable.Table().ForEachPlainRaw(func(key, val Value) bool {
		if key.IsString() && val.IsString() {
			kinds[key.Str()] = val.Str()
		}
		return true
	})
	return kinds
}

func copyDataColumnKinds(frame *Table) *Table {
	out := NewTable()
	kinds := dataColumnKinds(frame)
	for _, name := range dataColumnNames(frame) {
		out.RawSetString(name, StringValue(kinds[name]))
	}
	return out
}

func dataColumnKindsTable(names []string, kinds map[string]string) *Table {
	out := NewTable()
	for _, name := range names {
		out.RawSetString(name, StringValue(kinds[name]))
	}
	return out
}

func dataColumnNames(frame *Table) []string {
	namesTable := frame.RawGetString("column_names")
	if !namesTable.IsTable() {
		return nil
	}
	names := make([]string, 0, namesTable.Table().Length())
	for i := 1; i <= namesTable.Table().Length(); i++ {
		name := namesTable.Table().RawGetInt(int64(i))
		if name.IsString() {
			names = append(names, name.Str())
		}
	}
	return names
}

func copyDataColumnNames(frame *Table) *Table {
	return dataColumnNamesTable(dataColumnNames(frame))
}

func dataColumnNamesTable(names []string) *Table {
	out := NewAppendArrayTable(len(names))
	for i, name := range names {
		out.RawSetInt(int64(i+1), StringValue(name))
	}
	return out
}

func copyDataColumns(names []string, values map[string]Value) *Table {
	out := NewTable()
	for _, name := range names {
		out.RawSetString(name, values[name])
	}
	return out
}

func dataPlainStringKeys(tbl *Table) []string {
	if tbl == nil {
		return nil
	}
	if names := tbl.ShapeFieldNames(); len(names) > 0 {
		count := 0
		ok := tbl.ForEachPlainRaw(func(key, val Value) bool {
			if !key.IsString() {
				return false
			}
			count++
			return true
		})
		if !ok || count != len(names) {
			return nil
		}
		for _, name := range names {
			if tbl.RawGetString(name).IsNil() {
				return nil
			}
		}
		return names
	}
	names := make([]string, 0, tbl.Length())
	ok := tbl.ForEachPlainRaw(func(key, val Value) bool {
		if !key.IsString() {
			return false
		}
		names = append(names, key.Str())
		return true
	})
	if !ok {
		return nil
	}
	sort.Strings(names)
	return names
}

func dataVectorAnyValues(v Value) ([]any, error) {
	return dataColumnAnyValues(v)
}

func dataColumnAnyValues(v Value) ([]any, error) {
	kind, wrapped := dataColumnWrappedKind(v)
	if !wrapped {
		kind = dataColumnKind(v)
	}
	return dataColumnAnyValuesForKind(dataColumnWrappedValues(v), kind)
}

func dataColumnAnyValuesForKind(v Value, kind stddata.Kind) ([]any, error) {
	n, err := dataRawVectorLen(v)
	if err != nil {
		return nil, err
	}
	out := make([]any, n)
	for i := 0; i < n; i++ {
		item, err := dataRawVectorAt(v, i)
		if err != nil {
			return nil, err
		}
		out[i] = dataValueAnyForKind(item, kind)
	}
	return out, nil
}

func dataVectorLen(v Value) (int, error) {
	return dataRawVectorLen(dataColumnWrappedValues(v))
}

func dataRawVectorLen(v Value) (int, error) {
	switch {
	case v.IsDenseArray():
		return v.DenseArray().Len(), nil
	case v.IsTable():
		return v.Table().Length(), nil
	default:
		return 0, fmt.Errorf("vector must be a table or dense array")
	}
}

func dataVectorAt(v Value, i int) (Value, error) {
	return dataRawVectorAt(dataColumnWrappedValues(v), i)
}

func dataRawVectorAt(v Value, i int) (Value, error) {
	switch {
	case v.IsDenseArray():
		return v.DenseArray().At(i)
	case v.IsTable():
		return v.Table().RawGetInt(int64(i + 1)), nil
	default:
		return NilValue(), fmt.Errorf("vector must be a table or dense array")
	}
}

func dataValueAny(v Value) any {
	return dataValueAnyForKind(v, dataColumnKind(v))
}

func dataValueAnyForKind(v Value, kind stddata.Kind) any {
	switch {
	case v.IsNil():
		return nil
	case isDataNullValue(v):
		if kind := dataNullValueKind(v); kind != "" {
			return stddata.NullForKind(kind)
		}
		return stddata.NullValue
	case v.IsBool():
		return v.Bool()
	case v.IsInt():
		switch kind {
		case stddata.KindMonth, stddata.KindDate, stddata.KindDateTime, stddata.KindTimespan,
			stddata.KindMinute, stddata.KindSecond, stddata.KindTime, stddata.KindTimestamp:
			if parsed, ok := qParseTemporalAny(kind, v.Int()); ok {
				return parsed
			}
		}
		if dataKindIsFloat(kind) {
			if kind == stddata.KindF32 {
				return float32(v.Int())
			}
			return float64(v.Int())
		}
		return dataIntAnyForKind(kind, v.Int())
	case v.IsFloat():
		switch kind {
		case stddata.KindMonth, stddata.KindDate, stddata.KindDateTime, stddata.KindTimespan,
			stddata.KindMinute, stddata.KindSecond, stddata.KindTime, stddata.KindTimestamp:
			if parsed, ok := qParseTemporalAny(kind, int64(v.Float())); ok {
				return parsed
			}
		}
		if kind == stddata.KindF32 {
			return float32(v.Float())
		}
		return v.Float()
	case v.IsString():
		switch kind {
		case stddata.KindMonth, stddata.KindDate, stddata.KindDateTime, stddata.KindTimespan,
			stddata.KindMinute, stddata.KindSecond, stddata.KindTime, stddata.KindTimestamp:
			if parsed, ok := qParseTemporalAny(kind, v.Str()); ok {
				return parsed
			}
		}
		if kind == stddata.KindSymbol {
			return stddata.Symbol(v.Str())
		}
		return v.Str()
	default:
		return v.String()
	}
}

func dataIntAnyForKind(kind stddata.Kind, n int64) any {
	switch kind {
	case stddata.KindI8:
		return int8(n)
	case stddata.KindI16:
		return int16(n)
	case stddata.KindI32:
		return int32(n)
	case stddata.KindU8:
		return uint8(n)
	case stddata.KindU16:
		return uint16(n)
	case stddata.KindU32:
		return uint32(n)
	case stddata.KindU64:
		return uint64(n)
	case stddata.KindI64:
		return n
	default:
		return n
	}
}

func dataKindIsFloat(kind stddata.Kind) bool {
	return kind == stddata.KindF64 || kind == stddata.KindF32
}

func dataIntInKindRange(kind stddata.Kind, n int64) bool {
	switch kind {
	case stddata.KindI8:
		return n >= -128 && n <= 127
	case stddata.KindI16:
		return n >= -32768 && n <= 32767
	case stddata.KindI32:
		return n >= -2147483648 && n <= 2147483647
	case stddata.KindU8:
		return n >= 0 && n <= 255
	case stddata.KindU16:
		return n >= 0 && n <= 65535
	case stddata.KindU32:
		return n >= 0 && n <= 4294967295
	case stddata.KindU64:
		return n >= 0
	default:
		return true
	}
}
