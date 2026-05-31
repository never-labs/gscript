package runtime

import (
	"fmt"

	stdcsv "github.com/never-labs/gscript/internal/stdlib/data/csv"
)

// buildCSVLib creates the "csv" standard library table.
func buildCSVLib(interps ...*Interpreter) *Table {
	t := NewTable()
	var interp *Interpreter
	if len(interps) > 0 {
		interp = interps[0]
	}
	maxHostResult := func() int64 {
		if interp == nil {
			return 0
		}
		return interp.maxHostResult
	}

	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "csv." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "csv." + name,
			Fn:       fn,
			FastArg2: fast,
		}))
	}

	csvOptions := func(optsVal Value) stdcsv.Options {
		var opts stdcsv.Options
		if optsVal.IsTable() {
			optsTbl := optsVal.Table()
			if v := optsTbl.RawGet(StringValue("sep")); v.IsString() && len(v.Str()) > 0 {
				opts.Sep = rune(v.Str()[0])
			}
			if v := optsTbl.RawGet(StringValue("comment")); v.IsString() && len(v.Str()) > 0 {
				opts.Comment = rune(v.Str()[0])
			}
			if v := optsTbl.RawGet(StringValue("trimSpace")); v.IsBool() {
				opts.TrimSpace = v.Bool()
			}
			if v := optsTbl.RawGet(StringValue("lazyQuotes")); v.IsBool() {
				opts.LazyQuotes = v.Bool()
			}
		}
		return opts
	}

	rowsToValue := func(rows [][]string) Value {
		result := NewAppendArrayTable(len(rows))
		for i, record := range rows {
			row := NewSequentialArrayTable(len(record))
			for j, field := range record {
				row.RawSetInt(int64(j+1), StringValue(field))
			}
			result.RawSetInt(int64(i+1), TableValue(row))
		}
		return TableValue(result)
	}

	headerRowsToValue := func(rows []map[string]string) Value {
		result := NewAppendArrayTable(len(rows))
		for i, record := range rows {
			row := NewTableSized(0, len(record))
			for header, field := range record {
				row.RawSetString(header, StringValue(field))
			}
			result.RawSetInt(int64(i+1), TableValue(row))
		}
		return TableValue(result)
	}

	rowsFromValue := func(rowsVal Value) [][]string {
		rows := rowsVal.Table()
		length := rows.Length()
		out := make([][]string, 0, length)
		for i := int64(1); i <= int64(length); i++ {
			rowVal := rows.RawGet(IntValue(i))
			if !rowVal.IsTable() {
				continue
			}
			row := rowVal.Table()
			rowLen := row.Length()
			record := make([]string, rowLen)
			for j := int64(1); j <= int64(rowLen); j++ {
				record[j-1] = row.RawGet(IntValue(j)).String()
			}
			out = append(out, record)
		}
		return out
	}

	headersFromValue := func(headersVal Value) []string {
		headersTbl := headersVal.Table()
		headersLen := headersTbl.Length()
		headers := make([]string, headersLen)
		for i := int64(1); i <= int64(headersLen); i++ {
			headers[i-1] = headersTbl.RawGet(IntValue(i)).String()
		}
		return headers
	}

	headerRowsFromValue := func(rowsVal Value, headers []string) []map[string]string {
		rows := rowsVal.Table()
		length := rows.Length()
		out := make([]map[string]string, 0, length)
		for i := int64(1); i <= int64(length); i++ {
			rowVal := rows.RawGet(IntValue(i))
			if !rowVal.IsTable() {
				continue
			}
			row := rowVal.Table()
			record := make(map[string]string, len(headers))
			for _, h := range headers {
				record[h] = row.RawGet(StringValue(h)).String()
			}
			out = append(out, record)
		}
		return out
	}

	// csv.parse(str [, opts]) -- parse CSV string -> table of rows
	// opts: {sep=",", comment="#", trimSpace=true, lazyQuotes=false}
	csvParse := func(dataVal, optsVal Value) (Value, error) {
		if !dataVal.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'csv.parse' (string expected)")
		}
		rows, err := stdcsv.Parse(dataVal.Str(), csvOptions(optsVal))
		if err != nil {
			return NilValue(), fmt.Errorf("csv.parse: %v", err)
		}
		return rowsToValue(rows), nil
	}
	setFastArg1("parse", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'csv.parse' (string expected)")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		v, err := csvParse(args[0], opts)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, func(data Value) (Value, error) {
		return csvParse(data, NilValue())
	})

	// csv.parseWithHeaders(str [, opts]) -- parse CSV, first row is headers
	csvParseWithHeaders := func(dataVal, optsVal Value) (Value, error) {
		if !dataVal.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'csv.parseWithHeaders' (string expected)")
		}
		rows, err := stdcsv.ParseWithHeaders(dataVal.Str(), csvOptions(optsVal))
		if err != nil {
			return NilValue(), fmt.Errorf("csv.parseWithHeaders: %v", err)
		}
		return headerRowsToValue(rows), nil
	}
	setFastArg1("parseWithHeaders", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'csv.parseWithHeaders' (string expected)")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		v, err := csvParseWithHeaders(args[0], opts)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, func(data Value) (Value, error) {
		return csvParseWithHeaders(data, NilValue())
	})

	// csv.encode(rows [, opts]) -- encode table of rows to CSV string
	// opts: {sep=","}
	csvEncode := func(rowsVal, optsVal Value) (Value, error) {
		if !rowsVal.IsTable() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'csv.encode' (table expected)")
		}
		buf := newHostResultBuffer(maxHostResult())
		if err := stdcsv.Write(rowsFromValue(rowsVal), csvOptions(optsVal), buf); err != nil {
			return NilValue(), fmt.Errorf("csv.encode: %v", err)
		}
		return StringValue(buf.String()), nil
	}
	setFastArg1("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'csv.encode' (table expected)")
		}
		opts := NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		v, err := csvEncode(args[0], opts)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, func(rows Value) (Value, error) {
		return csvEncode(rows, NilValue())
	})

	// csv.encodeWithHeaders(rows, headers [, opts]) -- encode with header row first
	csvEncodeWithHeaders := func(rowsVal, headersVal, optsVal Value) (Value, error) {
		if !rowsVal.IsTable() || !headersVal.IsTable() {
			return NilValue(), fmt.Errorf("bad argument to 'csv.encodeWithHeaders'")
		}
		buf := newHostResultBuffer(maxHostResult())
		headers := headersFromValue(headersVal)
		rows := headerRowsFromValue(rowsVal, headers)
		if err := stdcsv.WriteWithHeaders(rows, headers, csvOptions(optsVal), buf); err != nil {
			return NilValue(), fmt.Errorf("csv.encodeWithHeaders: %v", err)
		}
		return StringValue(buf.String()), nil
	}
	setFastArg2("encodeWithHeaders", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'csv.encodeWithHeaders'")
		}
		opts := NilValue()
		if len(args) >= 3 {
			opts = args[2]
		}
		v, err := csvEncodeWithHeaders(args[0], args[1], opts)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, func(rows, headers Value) (Value, error) {
		return csvEncodeWithHeaders(rows, headers, NilValue())
	})
	if fn := t.RawGet(StringValue("encodeWithHeaders")).GoFunction(); fn != nil {
		fn.FastArg3 = func(rows, headers, opts Value) (Value, error) {
			return csvEncodeWithHeaders(rows, headers, opts)
		}
	}

	return t
}
