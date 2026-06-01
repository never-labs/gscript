package modules

import (
	"bytes"
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
	stdcsv "github.com/never-labs/leia/internal/stdlib/csv"
)

// BuildCSV creates the "csv" standard library table.
func BuildCSV(maxHostResult func() int64) *runtime.Table {
	t := runtime.NewTable()

	setFastArg1 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func(runtime.Value) (runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{Name: "csv." + name, Fn: fn, FastArg1: fast}))
	}
	setFastArg2 := func(name string, fn func([]runtime.Value) ([]runtime.Value, error), fast func(runtime.Value, runtime.Value) (runtime.Value, error)) {
		t.RawSet(runtime.StringValue(name), runtime.FunctionValue(&runtime.GoFunction{Name: "csv." + name, Fn: fn, FastArg2: fast}))
	}

	csvOptions := func(optsVal runtime.Value) stdcsv.Options {
		var opts stdcsv.Options
		if optsVal.IsTable() {
			optsTbl := optsVal.Table()
			if v := optsTbl.RawGet(runtime.StringValue("sep")); v.IsString() && len(v.Str()) > 0 {
				opts.Sep = rune(v.Str()[0])
			}
			if v := optsTbl.RawGet(runtime.StringValue("comment")); v.IsString() && len(v.Str()) > 0 {
				opts.Comment = rune(v.Str()[0])
			}
			if v := optsTbl.RawGet(runtime.StringValue("trimSpace")); v.IsBool() {
				opts.TrimSpace = v.Bool()
			}
			if v := optsTbl.RawGet(runtime.StringValue("lazyQuotes")); v.IsBool() {
				opts.LazyQuotes = v.Bool()
			}
		}
		return opts
	}

	rowsToValue := func(rows [][]string) runtime.Value {
		result := runtime.NewAppendArrayTable(len(rows))
		for i, record := range rows {
			row := runtime.NewSequentialArrayTable(len(record))
			for j, field := range record {
				row.RawSetInt(int64(j+1), runtime.StringValue(field))
			}
			result.RawSetInt(int64(i+1), runtime.TableValue(row))
		}
		return runtime.TableValue(result)
	}

	headerRowsToValue := func(rows []map[string]string) runtime.Value {
		result := runtime.NewAppendArrayTable(len(rows))
		for i, record := range rows {
			row := runtime.NewTableSized(0, len(record))
			for header, field := range record {
				row.RawSetString(header, runtime.StringValue(field))
			}
			result.RawSetInt(int64(i+1), runtime.TableValue(row))
		}
		return runtime.TableValue(result)
	}

	rowsFromValue := func(rowsVal runtime.Value) [][]string {
		rows := rowsVal.Table()
		length := rows.Length()
		out := make([][]string, 0, length)
		for i := int64(1); i <= int64(length); i++ {
			rowVal := rows.RawGet(runtime.IntValue(i))
			if !rowVal.IsTable() {
				continue
			}
			row := rowVal.Table()
			rowLen := row.Length()
			record := make([]string, rowLen)
			for j := int64(1); j <= int64(rowLen); j++ {
				record[j-1] = row.RawGet(runtime.IntValue(j)).String()
			}
			out = append(out, record)
		}
		return out
	}

	headersFromValue := func(headersVal runtime.Value) []string {
		headersTbl := headersVal.Table()
		headersLen := headersTbl.Length()
		headers := make([]string, headersLen)
		for i := int64(1); i <= int64(headersLen); i++ {
			headers[i-1] = headersTbl.RawGet(runtime.IntValue(i)).String()
		}
		return headers
	}

	headerRowsFromValue := func(rowsVal runtime.Value, headers []string) []map[string]string {
		rows := rowsVal.Table()
		length := rows.Length()
		out := make([]map[string]string, 0, length)
		for i := int64(1); i <= int64(length); i++ {
			rowVal := rows.RawGet(runtime.IntValue(i))
			if !rowVal.IsTable() {
				continue
			}
			row := rowVal.Table()
			record := make(map[string]string, len(headers))
			for _, h := range headers {
				record[h] = row.RawGet(runtime.StringValue(h)).String()
			}
			out = append(out, record)
		}
		return out
	}

	csvParse := func(dataVal, optsVal runtime.Value) (runtime.Value, error) {
		if !dataVal.IsString() {
			return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'csv.parse' (string expected)")
		}
		rows, err := stdcsv.Parse(dataVal.Str(), csvOptions(optsVal))
		if err != nil {
			return runtime.NilValue(), fmt.Errorf("csv.parse: %v", err)
		}
		return rowsToValue(rows), nil
	}
	setFastArg1("parse", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'csv.parse' (string expected)")
		}
		opts := runtime.NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		v, err := csvParse(args[0], opts)
		if err != nil {
			return nil, err
		}
		return []runtime.Value{v}, nil
	}, func(data runtime.Value) (runtime.Value, error) { return csvParse(data, runtime.NilValue()) })

	csvParseWithHeaders := func(dataVal, optsVal runtime.Value) (runtime.Value, error) {
		if !dataVal.IsString() {
			return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'csv.parseWithHeaders' (string expected)")
		}
		rows, err := stdcsv.ParseWithHeaders(dataVal.Str(), csvOptions(optsVal))
		if err != nil {
			return runtime.NilValue(), fmt.Errorf("csv.parseWithHeaders: %v", err)
		}
		return headerRowsToValue(rows), nil
	}
	setFastArg1("parseWithHeaders", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'csv.parseWithHeaders' (string expected)")
		}
		opts := runtime.NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		v, err := csvParseWithHeaders(args[0], opts)
		if err != nil {
			return nil, err
		}
		return []runtime.Value{v}, nil
	}, func(data runtime.Value) (runtime.Value, error) { return csvParseWithHeaders(data, runtime.NilValue()) })

	csvEncode := func(rowsVal, optsVal runtime.Value) (runtime.Value, error) {
		if !rowsVal.IsTable() {
			return runtime.NilValue(), fmt.Errorf("bad argument #1 to 'csv.encode' (table expected)")
		}
		buf := newCSVHostResultBuffer(csvHostResultLimit(maxHostResult))
		if err := stdcsv.Write(rowsFromValue(rowsVal), csvOptions(optsVal), buf); err != nil {
			return runtime.NilValue(), fmt.Errorf("csv.encode: %v", err)
		}
		return runtime.StringValue(buf.String()), nil
	}
	setFastArg1("encode", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'csv.encode' (table expected)")
		}
		opts := runtime.NilValue()
		if len(args) >= 2 {
			opts = args[1]
		}
		v, err := csvEncode(args[0], opts)
		if err != nil {
			return nil, err
		}
		return []runtime.Value{v}, nil
	}, func(rows runtime.Value) (runtime.Value, error) { return csvEncode(rows, runtime.NilValue()) })

	csvEncodeWithHeaders := func(rowsVal, headersVal, optsVal runtime.Value) (runtime.Value, error) {
		if !rowsVal.IsTable() || !headersVal.IsTable() {
			return runtime.NilValue(), fmt.Errorf("bad argument to 'csv.encodeWithHeaders'")
		}
		buf := newCSVHostResultBuffer(csvHostResultLimit(maxHostResult))
		headers := headersFromValue(headersVal)
		rows := headerRowsFromValue(rowsVal, headers)
		if err := stdcsv.WriteWithHeaders(rows, headers, csvOptions(optsVal), buf); err != nil {
			return runtime.NilValue(), fmt.Errorf("csv.encodeWithHeaders: %v", err)
		}
		return runtime.StringValue(buf.String()), nil
	}
	setFastArg2("encodeWithHeaders", func(args []runtime.Value) ([]runtime.Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'csv.encodeWithHeaders'")
		}
		opts := runtime.NilValue()
		if len(args) >= 3 {
			opts = args[2]
		}
		v, err := csvEncodeWithHeaders(args[0], args[1], opts)
		if err != nil {
			return nil, err
		}
		return []runtime.Value{v}, nil
	}, func(rows, headers runtime.Value) (runtime.Value, error) {
		return csvEncodeWithHeaders(rows, headers, runtime.NilValue())
	})
	if fn := t.RawGet(runtime.StringValue("encodeWithHeaders")).GoFunction(); fn != nil {
		fn.FastArg3 = func(rows, headers, opts runtime.Value) (runtime.Value, error) {
			return csvEncodeWithHeaders(rows, headers, opts)
		}
	}

	return t
}

func csvHostResultLimit(maxHostResult func() int64) int64 {
	if maxHostResult == nil {
		return 0
	}
	return maxHostResult()
}

type csvHostResultBuffer struct {
	buf bytes.Buffer
	max int64
}

func newCSVHostResultBuffer(max int64) *csvHostResultBuffer { return &csvHostResultBuffer{max: max} }

func (b *csvHostResultBuffer) Write(p []byte) (int, error) {
	if b.max <= 0 {
		return b.buf.Write(p)
	}
	remaining := b.max - int64(b.buf.Len())
	if remaining <= 0 {
		return 0, fmt.Errorf("host result byte limit exceeded (%d)", b.max)
	}
	if int64(len(p)) > remaining {
		if remaining > 0 {
			_, _ = b.buf.Write(p[:remaining])
		}
		return int(remaining), fmt.Errorf("host result byte limit exceeded (%d)", b.max)
	}
	return b.buf.Write(p)
}

func (b *csvHostResultBuffer) String() string { return b.buf.String() }
