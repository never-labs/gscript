package bind

import (
	"fmt"
	"sort"

	stddata "github.com/never-labs/leia/internal/stdlib/lib/data"
)

const dataFrameMarker = "__data_frame"
const dataColumnMarker = "__data_column"
const dataNullMarker = "__data_null"

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
	set("date", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.date", stddata.KindDate, args)
	})
	set("time", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.time", stddata.KindTime, args)
	})
	set("timestamp", func(args []Value) ([]Value, error) {
		return dataColumnConstructor("data.timestamp", stddata.KindTimestamp, args)
	})
	t.RawSetString("null", dataNullValue())
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
	return t
}

func dataNullValue() Value {
	t := NewTable()
	t.RawSetString(dataNullMarker, BoolValue(true))
	return TableValue(t)
}

func isDataNullValue(v Value) bool {
	return v.IsTable() && v.Table().RawGetString(dataNullMarker).Truthy()
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
		case stddata.KindDate, stddata.KindTime, stddata.KindTimestamp:
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
		libCols = append(libCols, stddata.NewColumn(stddata.Symbol(name), items))
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
	return TableValue(out), nil
}

func dataSchemaTable(names []string, kinds map[string]string) *Table {
	schema := NewTable()
	schema.RawSetString("names", TableValue(dataColumnNamesTable(names)))
	schema.RawSetString("kinds", TableValue(dataColumnKindsTable(names, kinds)))
	return schema
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
	return t != nil && t.RawGetString(dataFrameMarker).Truthy()
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
		return stddata.NullValue
	case v.IsBool():
		return v.Bool()
	case v.IsInt():
		if dataKindIsFloat(kind) {
			if kind == stddata.KindF32 {
				return float32(v.Int())
			}
			return float64(v.Int())
		}
		return dataIntAnyForKind(kind, v.Int())
	case v.IsFloat():
		if kind == stddata.KindF32 {
			return float32(v.Float())
		}
		return v.Float()
	case v.IsString():
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
