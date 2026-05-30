package runtime

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
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

	configureCSVReader := func(r *csv.Reader, optsVal Value) {
		if optsVal.IsTable() {
			opts := optsVal.Table()
			if v := opts.RawGet(StringValue("sep")); v.IsString() && len(v.Str()) > 0 {
				r.Comma = rune(v.Str()[0])
			}
			if v := opts.RawGet(StringValue("comment")); v.IsString() && len(v.Str()) > 0 {
				r.Comment = rune(v.Str()[0])
			}
			if v := opts.RawGet(StringValue("trimSpace")); v.IsBool() {
				r.TrimLeadingSpace = v.Bool()
			}
			if v := opts.RawGet(StringValue("lazyQuotes")); v.IsBool() {
				r.LazyQuotes = v.Bool()
			}
		}
	}
	configureCSVWriter := func(w *csv.Writer, optsVal Value) {
		if optsVal.IsTable() {
			opts := optsVal.Table()
			if v := opts.RawGet(StringValue("sep")); v.IsString() && len(v.Str()) > 0 {
				w.Comma = rune(v.Str()[0])
			}
		}
	}

	// csv.parse(str [, opts]) -- parse CSV string -> table of rows
	// opts: {sep=",", comment="#", trimSpace=true, lazyQuotes=false}
	csvParse := func(dataVal, optsVal Value) (Value, error) {
		if !dataVal.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'csv.parse' (string expected)")
		}
		r := csv.NewReader(strings.NewReader(dataVal.Str()))
		r.FieldsPerRecord = -1 // variable number of fields
		configureCSVReader(r, optsVal)

		result := NewAppendArrayTable(8)
		rowIdx := int64(1)
		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return NilValue(), fmt.Errorf("csv.parse: %v", err)
			}
			row := NewSequentialArrayTable(len(record))
			for i, field := range record {
				row.RawSetInt(int64(i+1), StringValue(field))
			}
			result.RawSetInt(rowIdx, TableValue(row))
			rowIdx++
		}
		return TableValue(result), nil
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
		r := csv.NewReader(strings.NewReader(dataVal.Str()))
		r.FieldsPerRecord = -1
		configureCSVReader(r, optsVal)

		// Read header row
		headers, err := r.Read()
		if err != nil {
			if err == io.EOF {
				return TableValue(NewEmptyTable()), nil
			}
			return NilValue(), fmt.Errorf("csv.parseWithHeaders: %v", err)
		}

		result := NewAppendArrayTable(8)
		rowIdx := int64(1)
		for {
			record, err := r.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				return NilValue(), fmt.Errorf("csv.parseWithHeaders: %v", err)
			}
			row := NewTableSized(0, len(headers))
			for i, field := range record {
				if i < len(headers) {
					row.RawSetString(headers[i], StringValue(field))
				}
			}
			result.RawSetInt(rowIdx, TableValue(row))
			rowIdx++
		}
		return TableValue(result), nil
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
		rows := rowsVal.Table()
		buf := newHostResultBuffer(maxHostResult())
		w := csv.NewWriter(buf)
		configureCSVWriter(w, optsVal)

		length := rows.Length()
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
			if err := w.Write(record); err != nil {
				return NilValue(), fmt.Errorf("csv.encode: %v", err)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
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
		rows := rowsVal.Table()
		headersTbl := headersVal.Table()
		buf := newHostResultBuffer(maxHostResult())
		w := csv.NewWriter(buf)
		configureCSVWriter(w, optsVal)

		// Write header row
		headersLen := headersTbl.Length()
		headerNames := make([]string, headersLen)
		for i := int64(1); i <= int64(headersLen); i++ {
			headerNames[i-1] = headersTbl.RawGet(IntValue(i)).String()
		}
		if err := w.Write(headerNames); err != nil {
			return NilValue(), fmt.Errorf("csv.encodeWithHeaders: %v", err)
		}

		// Write data rows
		length := rows.Length()
		for i := int64(1); i <= int64(length); i++ {
			rowVal := rows.RawGet(IntValue(i))
			if !rowVal.IsTable() {
				continue
			}
			row := rowVal.Table()
			record := make([]string, headersLen)
			for j, h := range headerNames {
				record[j] = row.RawGet(StringValue(h)).String()
			}
			if err := w.Write(record); err != nil {
				return NilValue(), fmt.Errorf("csv.encodeWithHeaders: %v", err)
			}
		}
		w.Flush()
		if err := w.Error(); err != nil {
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
