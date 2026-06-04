package bind

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/never-labs/leia/internal/runtime"
	csvlib "github.com/never-labs/leia/internal/stdlib/lib/csv"
	encodinglib "github.com/never-labs/leia/internal/stdlib/lib/encoding"
	dialectlib "github.com/never-labs/leia/internal/support/dialect"
)

func registerDialectText(register dialectRegisterFunc, maxHostResult func() int64) {
	register([]string{"re", "regexp"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectRegexp(body.Str(), dialectFailFast(options))
		},
	})
	register([]string{"json"}, dialectHandler{
		eval:  dialectJSON,
		block: dialectJSON,
	})
	register([]string{"jsonptr"}, dialectHandler{
		eval:  dialectJSONPointer,
		block: dialectJSONPointer,
	})
	register([]string{"jsonl"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectJSONL(body, options, maxHostResult)
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return dialectJSONL(body, options, maxHostResult)
		},
	})
	register([]string{"csv"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectCSV(body, options, maxHostResult)
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return dialectCSV(body, options, maxHostResult)
		},
	})
	register([]string{"tsv"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectTSV(body, options, maxHostResult)
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return dialectTSV(body, options, maxHostResult)
		},
	})
	register([]string{"mdtable"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectMarkdownTable(body, options, maxHostResult)
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return dialectMarkdownTable(body, options, maxHostResult)
		},
	})
	register([]string{"lines", "split"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectLines(body.Str(), options)
		},
	})
	register([]string{"words"}, dialectHandler{
		eval: func(body Value, _ *Table) ([]Value, error) {
			return dialectWords(body.Str()), nil
		},
	})
	register([]string{"nums", "numbers"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectNumbers(body.Str(), options)
		},
	})
	register([]string{"kv"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectKV(body.Str(), options, false)
		},
	})
	register([]string{"logfmt"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectLogfmt(body, options, maxHostResult)
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return dialectLogfmt(body, options, maxHostResult)
		},
	})
	register([]string{"env"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectKV(body.Str(), options, true)
		},
	})
	register([]string{"ini"}, dialectHandler{
		eval:  dialectINI,
		block: dialectINI,
	})
	register([]string{"semver"}, dialectHandler{
		eval:  dialectSemVer,
		block: dialectSemVer,
	})
	register([]string{"duration"}, dialectHandler{
		eval:  dialectDuration,
		block: dialectDuration,
	})
	register([]string{"tap"}, dialectHandler{
		eval:  dialectTAP,
		block: dialectTAP,
	})
	register([]string{"junit"}, dialectHandler{
		eval: dialectJUnit,
	})
	register([]string{"xml"}, dialectHandler{
		eval:  dialectXML,
		block: dialectXML,
	})
	register([]string{"template"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectTemplate(body, options, maxHostResult)
		},
		block: func(body Value, options *Table) ([]Value, error) {
			return dialectTemplate(body, options, maxHostResult)
		},
	})
}

func dialectXML(body Value, opts *Table) ([]Value, error) {
	mode := "escape"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	switch mode {
	case "", "escape", "encode":
		return []Value{StringValue(encodinglib.XMLEscape(body.Str()))}, nil
	case "unescape", "decode":
		decoded, err := encodinglib.XMLUnescape(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	default:
		return nil, fmt.Errorf("xml dialect: unknown mode %q", mode)
	}
}

func dialectDuration(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if mode == "encode" || !body.IsString() {
		encoded, err := encodeDialectDuration(body)
		if err != nil {
			return nil, err
		}
		return []Value{StringValue(encoded)}, nil
	}
	parts, err := dialectlib.ParseDuration(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(durationPartsTable(parts))}, nil
}

func durationPartsTable(parts dialectlib.DurationParts) *Table {
	out := NewTable()
	out.RawSetString("text", StringValue(parts.Text))
	out.RawSetString("seconds", FloatValue(parts.Seconds))
	out.RawSetString("milliseconds", FloatValue(parts.Milliseconds))
	out.RawSetString("nanoseconds", IntValue(parts.Nanoseconds))
	return out
}

func encodeDialectDuration(body Value) (string, error) {
	switch {
	case body.IsInt():
		return dialectlib.EncodeDurationSeconds(float64(body.Int()))
	case body.IsFloat():
		return dialectlib.EncodeDurationSeconds(body.Float())
	case body.IsTable():
		return encodeDialectDurationTable(body.Table())
	default:
		return "", fmt.Errorf("duration dialect: encode expects number seconds or table")
	}
}

func encodeDialectDurationTable(tbl *Table) (string, error) {
	if text := tbl.RawGetString("text"); text.IsString() {
		parts, err := dialectlib.ParseDuration(text.Str())
		if err != nil {
			return "", err
		}
		return parts.Text, nil
	}
	if duration := tbl.RawGetString("duration"); duration.IsString() {
		parts, err := dialectlib.ParseDuration(duration.Str())
		if err != nil {
			return "", err
		}
		return parts.Text, nil
	}
	if ns := firstDurationField(tbl, "nanoseconds", "ns"); !ns.IsNil() {
		if !ns.IsNumber() {
			return "", fmt.Errorf("duration dialect: nanoseconds must be numeric")
		}
		return dialectlib.EncodeDurationNanoseconds(int64(math.Round(ns.Number()))), nil
	}
	if ms := firstDurationField(tbl, "milliseconds", "ms"); !ms.IsNil() {
		if !ms.IsNumber() {
			return "", fmt.Errorf("duration dialect: milliseconds must be numeric")
		}
		return dialectlib.EncodeDurationMilliseconds(ms.Number())
	}
	if seconds := firstDurationField(tbl, "seconds", "s"); !seconds.IsNil() {
		if !seconds.IsNumber() {
			return "", fmt.Errorf("duration dialect: seconds must be numeric")
		}
		return dialectlib.EncodeDurationSeconds(seconds.Number())
	}
	return "", fmt.Errorf("duration dialect: table encode expects text, nanoseconds, milliseconds, or seconds")
}

func firstDurationField(tbl *Table, names ...string) Value {
	for _, name := range names {
		value := tbl.RawGetString(name)
		if !value.IsNil() {
			return value
		}
	}
	return NilValue()
}

func dialectRegexp(pattern string, failFast bool) ([]Value, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		if failFast {
			return nil, fmt.Errorf("re dialect: %v", err)
		}
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(makeReObject(re))}, nil
}

func dialectJSON(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsString() && mode != "encode" {
		decoder := json.NewDecoder(strings.NewReader(body.Str()))
		decoder.UseNumber()
		var goVal any
		if err := decoder.Decode(&goVal); err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return []Value{NilValue(), StringValue("invalid JSON: trailing data")}, nil
			}
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{runtime.JSONGoToValue(goVal)}, nil
	}
	data, err := json.Marshal(runtime.JSONValueToGo(body))
	if err != nil {
		return nil, fmt.Errorf("json dialect: %v", err)
	}
	return []Value{StringValue(string(data))}, nil
}

func dialectJSONPointer(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if mode == "encode" || mode == "format" {
		if !body.IsTable() {
			return nil, fmt.Errorf("jsonptr dialect: encode expects token array")
		}
		tokens := make([]string, 0, body.Table().Length())
		for i := 1; i <= body.Table().Length(); i++ {
			tokens = append(tokens, body.Table().RawGetInt(int64(i)).String())
		}
		return []Value{StringValue(dialectlib.EncodeJSONPointer(tokens))}, nil
	}
	path := ""
	if opts != nil {
		if v := opts.RawGetString("path"); v.IsString() {
			path = v.Str()
		} else if v := opts.RawGetString("pointer"); v.IsString() {
			path = v.Str()
		}
	}
	target := body
	if body.IsTable() && path == "" {
		tbl := body.Table()
		if v := tbl.RawGetString("path"); v.IsString() {
			path = v.Str()
		} else if v := tbl.RawGetString("pointer"); v.IsString() {
			path = v.Str()
		}
		if data := tbl.RawGetString("data"); !data.IsNil() {
			target = data
		} else if value := tbl.RawGetString("value"); !value.IsNil() {
			target = value
		}
	}
	tokens, err := dialectlib.ParseJSONPointer(path)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	value, err := jsonPointerLookup(target, tokens)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{value}, nil
}

func jsonPointerLookup(value Value, tokens []string) (Value, error) {
	current := value
	for _, token := range tokens {
		if !current.IsTable() {
			return NilValue(), fmt.Errorf("jsonptr: cannot descend into %s", current.TypeName())
		}
		tbl := current.Table()
		if idx, ok := dialectlib.JSONPointerIndex(token); ok && idx < tbl.Length() {
			current = tbl.RawGetInt(int64(idx + 1))
			continue
		}
		current = tbl.RawGetString(token)
		if current.IsNil() {
			return NilValue(), fmt.Errorf("jsonptr: missing token %q", token)
		}
	}
	return current, nil
}

func dialectJSONL(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if mode == "encode" || !body.IsString() {
		data, err := encodeJSONL(body, hostResultLimit(maxHostResult))
		if err != nil {
			return nil, fmt.Errorf("jsonl dialect: %v", err)
		}
		return []Value{StringValue(data)}, nil
	}
	rows, err := decodeJSONL(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{rows}, nil
}

func decodeJSONL(src string) (Value, error) {
	src = strings.TrimSuffix(src, "\n")
	src = strings.TrimSuffix(src, "\r")
	if src == "" {
		return TableValue(NewAppendArrayTable(0)), nil
	}
	lines := strings.Split(src, "\n")
	out := NewAppendArrayTable(len(lines))
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if strings.TrimSpace(line) == "" {
			return NilValue(), fmt.Errorf("line %d: empty JSONL record", i+1)
		}
		val, err := decodeJSONLine(line)
		if err != nil {
			return NilValue(), fmt.Errorf("line %d: %v", i+1, err)
		}
		out.RawSetInt(int64(i+1), val)
	}
	return TableValue(out), nil
}

func decodeJSONLine(line string) (Value, error) {
	decoder := json.NewDecoder(strings.NewReader(line))
	decoder.UseNumber()
	var goVal any
	if err := decoder.Decode(&goVal); err != nil {
		return NilValue(), err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return NilValue(), fmt.Errorf("invalid JSON: trailing data")
		}
		return NilValue(), err
	}
	return runtime.JSONGoToValue(goVal), nil
}

func encodeJSONL(body Value, limit int64) (string, error) {
	if !body.IsTable() {
		data, err := json.Marshal(runtime.JSONValueToGo(body))
		if err != nil {
			return "", err
		}
		if err := CheckProjectedHostStringBytes(limit, len(data)+1); err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	}
	tbl := body.Table()
	if tbl.Length() == 0 && tableHasAnyKey(tbl) {
		data, err := json.Marshal(runtime.JSONValueToGo(body))
		if err != nil {
			return "", err
		}
		if err := CheckProjectedHostStringBytes(limit, len(data)+1); err != nil {
			return "", err
		}
		return string(data) + "\n", nil
	}
	buf := newHostResultBuffer(limit)
	for i := 1; i <= tbl.Length(); i++ {
		data, err := json.Marshal(runtime.JSONValueToGo(tbl.RawGetInt(int64(i))))
		if err != nil {
			return "", err
		}
		if _, err := buf.Write(data); err != nil {
			return "", err
		}
		if _, err := buf.Write([]byte("\n")); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func tableHasAnyKey(tbl *Table) bool {
	_, _, ok := tbl.Next(NilValue())
	return ok
}

func dialectCSV(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	return dialectDelimited(body, opts, 0, maxHostResult)
}

func dialectTSV(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	return dialectDelimited(body, opts, '\t', maxHostResult)
}

func dialectDelimited(body Value, opts *Table, defaultSep rune, maxHostResult func() int64) ([]Value, error) {
	csvOpts := csvDialectOptions(opts)
	if csvOpts.Sep == 0 {
		csvOpts.Sep = defaultSep
	}
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode" || mode == "format" {
		text, err := encodeDelimitedValue(body, opts, csvOpts, maxHostResult)
		if err != nil {
			if strings.Contains(err.Error(), "host result byte limit exceeded") {
				return nil, err
			}
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), len(text)); err != nil {
			return nil, err
		}
		return []Value{StringValue(text)}, nil
	}
	src := body.Str()
	if opts != nil && opts.RawGetString("headers").Truthy() {
		rows, err := csvlib.ParseWithHeaders(src, csvOpts)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{csvHeaderRowsToValue(rows)}, nil
	}
	rows, err := csvlib.Parse(src, csvOpts)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{csvRowsToValue(rows)}, nil
}

func encodeDelimitedValue(body Value, opts *Table, csvOpts csvlib.Options, maxHostResult func() int64) (string, error) {
	if !body.IsTable() {
		return "", fmt.Errorf("csv dialect: table expected for encode")
	}
	buf := newHostResultBuffer(hostResultLimit(maxHostResult))
	headers := csvHeadersFromOptions(opts)
	if len(headers) > 0 {
		rows, err := csvHeaderRowsFromValue(body.Table(), headers)
		if err != nil {
			return "", err
		}
		if err := csvlib.WriteWithHeaders(rows, headers, csvOpts, buf); err != nil {
			return "", err
		}
		return buf.String(), nil
	}
	rows, err := csvRowsFromValue(body.Table())
	if err != nil {
		return "", err
	}
	if err := csvlib.Write(rows, csvOpts, buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func csvDialectOptions(opts *Table) csvlib.Options {
	var out csvlib.Options
	if opts == nil {
		return out
	}
	if v := opts.RawGetString("sep"); v.IsString() && len(v.Str()) > 0 {
		out.Sep = rune(v.Str()[0])
	}
	if v := opts.RawGetString("comment"); v.IsString() && len(v.Str()) > 0 {
		out.Comment = rune(v.Str()[0])
	}
	if v := opts.RawGetString("trimSpace"); v.IsBool() {
		out.TrimSpace = v.Bool()
	}
	if v := opts.RawGetString("lazyQuotes"); v.IsBool() {
		out.LazyQuotes = v.Bool()
	}
	return out
}

func csvRowsToValue(rows [][]string) Value {
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

func csvHeaderRowsToValue(rows []map[string]string) Value {
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

func csvRowsFromValue(rows *Table) ([][]string, error) {
	out := make([][]string, 0, rows.Length())
	for i := 1; i <= rows.Length(); i++ {
		rowVal := rows.RawGetInt(int64(i))
		if !rowVal.IsTable() {
			return nil, fmt.Errorf("csv dialect: row %d must be table", i)
		}
		rowTbl := rowVal.Table()
		record := make([]string, 0, rowTbl.Length())
		for j := 1; j <= rowTbl.Length(); j++ {
			record = append(record, rowTbl.RawGetInt(int64(j)).String())
		}
		out = append(out, record)
	}
	return out, nil
}

func csvHeadersFromOptions(opts *Table) []string {
	if opts == nil {
		return nil
	}
	headersVal := opts.RawGetString("headers")
	if !headersVal.IsTable() {
		return nil
	}
	headersTbl := headersVal.Table()
	headers := make([]string, 0, headersTbl.Length())
	for i := 1; i <= headersTbl.Length(); i++ {
		headers = append(headers, headersTbl.RawGetInt(int64(i)).String())
	}
	return headers
}

func csvHeaderRowsFromValue(rows *Table, headers []string) ([]map[string]string, error) {
	out := make([]map[string]string, 0, rows.Length())
	for i := 1; i <= rows.Length(); i++ {
		rowVal := rows.RawGetInt(int64(i))
		if !rowVal.IsTable() {
			return nil, fmt.Errorf("csv dialect: row %d must be table", i)
		}
		rowTbl := rowVal.Table()
		record := make(map[string]string, len(headers))
		for _, header := range headers {
			record[header] = rowTbl.RawGetString(header).String()
		}
		out = append(out, record)
	}
	return out, nil
}

func dialectMarkdownTable(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsString() && mode != "encode" {
		table, err := dialectlib.ParseMarkdownTable(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{markdownTableRowsToValue(table)}, nil
	}
	table, err := markdownTableFromValue(body, opts)
	if err != nil {
		return nil, fmt.Errorf("mdtable dialect: %v", err)
	}
	text, err := dialectlib.EncodeMarkdownTable(table)
	if err != nil {
		return nil, err
	}
	if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), len(text)); err != nil {
		return nil, err
	}
	return []Value{StringValue(text)}, nil
}

func markdownTableRowsToValue(table dialectlib.MarkdownTable) Value {
	result := NewAppendArrayTable(len(table.Rows))
	headers := NewAppendArrayTable(len(table.Headers))
	for i, header := range table.Headers {
		headers.RawSetInt(int64(i+1), StringValue(header))
	}
	result.RawSetString("_headers", TableValue(headers))
	for i, record := range table.Rows {
		row := NewTableSized(0, len(record))
		for _, header := range table.Headers {
			row.RawSetString(header, StringValue(record[header]))
		}
		result.RawSetInt(int64(i+1), TableValue(row))
	}
	return TableValue(result)
}

func markdownTableFromValue(v Value, opts *Table) (dialectlib.MarkdownTable, error) {
	if !v.IsTable() {
		return dialectlib.MarkdownTable{}, fmt.Errorf("table expected")
	}
	rows := v.Table()
	headers := markdownTableHeadersFromOptions(opts)
	if len(headers) == 0 {
		headers = markdownTableHeadersFromValue(rows.RawGetString("_headers"))
	}
	if len(headers) == 0 && rows.Length() > 0 {
		first := rows.RawGetInt(1)
		if first.IsTable() {
			headers = markdownTableSortedRowHeaders(first.Table())
		}
	}
	if len(headers) == 0 {
		return dialectlib.MarkdownTable{}, fmt.Errorf("headers are required")
	}
	out := dialectlib.MarkdownTable{
		Headers: headers,
		Rows:    make([]map[string]string, 0, rows.Length()),
	}
	for i := 1; i <= rows.Length(); i++ {
		rowVal := rows.RawGetInt(int64(i))
		if !rowVal.IsTable() {
			return dialectlib.MarkdownTable{}, fmt.Errorf("row %d is not a table", i)
		}
		rowTbl := rowVal.Table()
		row := make(map[string]string, len(headers))
		for _, header := range headers {
			cell := rowTbl.RawGetString(header)
			if cell.IsNil() {
				row[header] = ""
			} else {
				row[header] = cell.String()
			}
		}
		out.Rows = append(out.Rows, row)
	}
	return out, nil
}

func markdownTableHeadersFromOptions(opts *Table) []string {
	if opts == nil {
		return nil
	}
	return markdownTableHeadersFromValue(opts.RawGetString("headers"))
}

func markdownTableHeadersFromValue(v Value) []string {
	if !v.IsTable() {
		return nil
	}
	tbl := v.Table()
	headers := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		header := tbl.RawGetInt(int64(i))
		if header.IsString() && header.Str() != "" {
			headers = append(headers, header.Str())
		}
	}
	return headers
}

func markdownTableSortedRowHeaders(row *Table) []string {
	keys := make(map[string]struct{})
	for key, _, ok := row.Next(NilValue()); ok; key, _, ok = row.Next(key) {
		if key.IsString() && key.Str() != "_headers" {
			keys[key.Str()] = struct{}{}
		}
	}
	return sortedStringKeys(keys)
}

func dialectLines(src string, opts *Table) ([]Value, error) {
	keepEmpty := opts != nil && opts.RawGetString("keep_empty").Truthy()
	keepTrailing := opts != nil && opts.RawGetString("keep_trailing").Truthy()
	parts := dialectlib.Lines(src, keepEmpty, keepTrailing)
	out := NewAppendArrayTable(len(parts))
	for i, line := range parts {
		out.RawSetInt(int64(i+1), StringValue(line))
	}
	return []Value{TableValue(out)}, nil
}

func dialectWords(src string) []Value {
	parts := dialectlib.Words(src)
	out := NewAppendArrayTable(len(parts))
	for i, word := range parts {
		out.RawSetInt(int64(i+1), StringValue(word))
	}
	return []Value{TableValue(out)}
}

func dialectNumbers(src string, opts *Table) ([]Value, error) {
	if opts != nil && opts.RawGetString("matrix").Truthy() {
		matrix, err := parseNumberMatrix(src)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{numberMatrixToValue(matrix)}, nil
	}
	values, err := parseNumberFields(src)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{numberRowToValue(values)}, nil
}

func parseNumberMatrix(src string) ([][]Value, error) {
	lines := dialectlib.Lines(src, false, false)
	rows := make([][]Value, 0, len(lines))
	width := -1
	for lineNo, line := range lines {
		values, err := parseNumberFields(line)
		if err != nil {
			return nil, fmt.Errorf("nums dialect line %d: %v", lineNo+1, err)
		}
		if width < 0 {
			width = len(values)
		} else if len(values) != width {
			return nil, fmt.Errorf("nums dialect matrix row %d has %d values, want %d", lineNo+1, len(values), width)
		}
		rows = append(rows, values)
	}
	return rows, nil
}

func parseNumberFields(src string) ([]Value, error) {
	fields := strings.FieldsFunc(src, func(r rune) bool {
		return r == ',' || r == ';' || unicode.IsSpace(r)
	})
	values := make([]Value, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		v, err := parseNumberField(field)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func parseNumberField(field string) (Value, error) {
	if !strings.ContainsAny(field, ".eE") {
		i, err := strconv.ParseInt(field, 10, 64)
		if err == nil {
			return IntValue(i), nil
		}
	}
	f, err := strconv.ParseFloat(field, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		return NilValue(), fmt.Errorf("invalid number %q", field)
	}
	return FloatValue(f), nil
}

func numberRowToValue(values []Value) Value {
	row := NewAppendArrayTable(len(values))
	for i, v := range values {
		row.RawSetInt(int64(i+1), v)
	}
	return TableValue(row)
}

func numberMatrixToValue(rows [][]Value) Value {
	out := NewAppendArrayTable(len(rows))
	for i, row := range rows {
		out.RawSetInt(int64(i+1), numberRowToValue(row))
	}
	return TableValue(out)
}

func dialectKV(src string, opts *Table, envMode bool) ([]Value, error) {
	kvOpts := dialectlib.KVOptions{Sep: "=", Trim: true, EnvMode: envMode}
	if opts != nil && opts.RawGetString("sep").IsString() && opts.RawGetString("sep").Str() != "" {
		kvOpts.Sep = opts.RawGetString("sep").Str()
	}
	if opts != nil && opts.RawGetString("trim").IsBool() {
		kvOpts.Trim = opts.RawGetString("trim").Bool()
	}
	parsed, err := dialectlib.KV(src, kvOpts)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	for key, val := range parsed {
		out.RawSetString(key, StringValue(val))
	}
	return []Value{TableValue(out)}, nil
}

func dialectLogfmt(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode" || mode == "format" {
		values := map[string]string{}
		if body.IsTable() {
			body.Table().ForEachPlainRaw(func(k, v Value) bool {
				if k.IsString() {
					values[k.Str()] = v.String()
				}
				return true
			})
		}
		encoded := dialectlib.EncodeLogfmt(values)
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), len(encoded)); err != nil {
			return nil, err
		}
		return []Value{StringValue(encoded)}, nil
	}
	pairs, err := dialectlib.ParseLogfmt(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	ordered := NewAppendArrayTable(len(pairs))
	for i, pair := range pairs {
		out.RawSetString(pair.Key, StringValue(pair.Value))
		item := NewTable()
		item.RawSetString("key", StringValue(pair.Key))
		item.RawSetString("value", StringValue(pair.Value))
		ordered.RawSetInt(int64(i+1), TableValue(item))
	}
	out.RawSetString("pairs", TableValue(ordered))
	return []Value{TableValue(out)}, nil
}

func dialectINI(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsString() && mode != "encode" {
		doc, err := dialectlib.ParseINI(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{iniDocumentToValue(doc)}, nil
	}
	doc, err := iniDocumentFromValue(body)
	if err != nil {
		return nil, fmt.Errorf("ini dialect: %v", err)
	}
	text, err := dialectlib.EncodeINI(doc)
	if err != nil {
		return nil, err
	}
	return []Value{StringValue(text)}, nil
}

func iniDocumentToValue(doc dialectlib.INIDocument) Value {
	out := NewTable()
	for _, key := range sortedStringKeys(doc.Root) {
		out.RawSetString(key, StringValue(doc.Root[key]))
	}
	for _, sectionName := range sortedStringKeys(doc.Sections) {
		section := NewTable()
		for _, key := range sortedStringKeys(doc.Sections[sectionName]) {
			section.RawSetString(key, StringValue(doc.Sections[sectionName][key]))
		}
		out.RawSetString(sectionName, TableValue(section))
	}
	return TableValue(out)
}

func iniDocumentFromValue(v Value) (dialectlib.INIDocument, error) {
	if !v.IsTable() {
		return dialectlib.INIDocument{}, fmt.Errorf("table expected")
	}
	doc := dialectlib.INIDocument{
		Root:     make(map[string]string),
		Sections: make(map[string]map[string]string),
	}
	if err := collectINIFields(v.Table(), doc.Root, doc.Sections); err != nil {
		return dialectlib.INIDocument{}, err
	}
	return doc, nil
}

func collectINIFields(tbl *Table, root map[string]string, sections map[string]map[string]string) error {
	for key, val, ok := tbl.Next(NilValue()); ok; key, val, ok = tbl.Next(key) {
		if !key.IsString() {
			return fmt.Errorf("string keys expected, got %v", key.Type())
		}
		name := key.Str()
		if val.IsTable() {
			fields := make(map[string]string)
			for subKey, subVal, subOK := val.Table().Next(NilValue()); subOK; subKey, subVal, subOK = val.Table().Next(subKey) {
				if !subKey.IsString() {
					return fmt.Errorf("section %q contains non-string key %v", name, subKey.Type())
				}
				if subVal.IsTable() {
					return fmt.Errorf("section %q key %q has nested table value", name, subKey.Str())
				}
				fields[subKey.Str()] = iniScalarString(subVal)
			}
			sections[name] = fields
			continue
		}
		root[name] = iniScalarString(val)
	}
	return nil
}

func iniScalarString(v Value) string {
	if v.IsNil() {
		return ""
	}
	return v.String()
}

func dialectSemVer(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsString() && mode != "encode" && mode != "format" {
		parsed, err := dialectlib.ParseSemVer(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{semVerToValue(parsed)}, nil
	}
	parsed, err := semVerFromValue(body)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	text, err := dialectlib.FormatSemVer(parsed)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{StringValue(text)}, nil
}

func semVerToValue(v dialectlib.SemVer) Value {
	out := NewTableSized(0, 8)
	out.RawSetString("major", IntValue(v.Major))
	out.RawSetString("minor", IntValue(v.Minor))
	out.RawSetString("patch", IntValue(v.Patch))
	out.RawSetString("prerelease", stringArrayToValue(v.Prerelease))
	out.RawSetString("build", stringArrayToValue(v.Build))
	out.RawSetString("pre", StringValue(strings.Join(v.Prerelease, ".")))
	out.RawSetString("build_metadata", StringValue(strings.Join(v.Build, ".")))
	text, _ := dialectlib.FormatSemVer(v)
	out.RawSetString("version", StringValue(text))
	return TableValue(out)
}

func semVerFromValue(v Value) (dialectlib.SemVer, error) {
	if v.IsString() {
		return dialectlib.ParseSemVer(v.Str())
	}
	if !v.IsTable() {
		return dialectlib.SemVer{}, fmt.Errorf("semver dialect: table expected")
	}
	tbl := v.Table()
	major, err := semVerCoreField(tbl, "major")
	if err != nil {
		return dialectlib.SemVer{}, err
	}
	minor, err := semVerCoreField(tbl, "minor")
	if err != nil {
		return dialectlib.SemVer{}, err
	}
	patch, err := semVerCoreField(tbl, "patch")
	if err != nil {
		return dialectlib.SemVer{}, err
	}
	prerelease, err := semVerIdentifierField(tbl, "prerelease", "pre")
	if err != nil {
		return dialectlib.SemVer{}, err
	}
	build, err := semVerIdentifierField(tbl, "build", "build_metadata")
	if err != nil {
		return dialectlib.SemVer{}, err
	}
	return dialectlib.SemVer{Major: major, Minor: minor, Patch: patch, Prerelease: prerelease, Build: build}, nil
}

func semVerCoreField(tbl *Table, key string) (int64, error) {
	v := tbl.RawGetString(key)
	switch {
	case v.IsInt():
		if v.Int() < 0 {
			return 0, fmt.Errorf("semver dialect: %s must be non-negative", key)
		}
		return v.Int(), nil
	case v.IsFloat():
		f := v.Float()
		if math.Trunc(f) != f || f < 0 || math.IsInf(f, 0) || math.IsNaN(f) {
			return 0, fmt.Errorf("semver dialect: %s must be a non-negative integer", key)
		}
		return int64(f), nil
	default:
		return 0, fmt.Errorf("semver dialect: %s integer field required", key)
	}
}

func semVerIdentifierField(tbl *Table, primary, alias string) ([]string, error) {
	v := tbl.RawGetString(primary)
	if v.IsNil() && alias != "" {
		v = tbl.RawGetString(alias)
	}
	if v.IsNil() {
		return nil, nil
	}
	if v.IsString() {
		if v.Str() == "" {
			return nil, nil
		}
		return strings.Split(v.Str(), "."), nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("semver dialect: %s must be a string or string array", primary)
	}
	ids := make([]string, 0, v.Table().Length())
	for i := 1; i <= v.Table().Length(); i++ {
		item := v.Table().RawGetInt(int64(i))
		if !item.IsString() {
			return nil, fmt.Errorf("semver dialect: %s[%d] must be a string", primary, i)
		}
		ids = append(ids, item.Str())
	}
	return ids, nil
}

func stringArrayToValue(values []string) Value {
	out := NewAppendArrayTable(len(values))
	for i, value := range values {
		out.RawSetInt(int64(i+1), StringValue(value))
	}
	return TableValue(out)
}

func dialectTAP(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsString() && mode != "encode" {
		doc, err := dialectlib.ParseTAP(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{tapDocumentToValue(doc)}, nil
	}
	doc, err := tapDocumentFromValue(body)
	if err != nil {
		return nil, fmt.Errorf("tap dialect: %v", err)
	}
	text, err := dialectlib.EncodeTAP(doc)
	if err != nil {
		return nil, err
	}
	return []Value{StringValue(text)}, nil
}

func tapDocumentToValue(doc dialectlib.TAPDocument) Value {
	out := NewAppendArrayTable(len(doc.Rows))
	for i, row := range doc.Rows {
		item := NewTable()
		item.RawSetString("kind", StringValue(row.Kind))
		if row.Line > 0 {
			item.RawSetString("line", IntValue(int64(row.Line)))
		}
		switch row.Kind {
		case "version":
			item.RawSetString("version", IntValue(int64(row.Version)))
		case "plan":
			item.RawSetString("first", IntValue(int64(row.First)))
			item.RawSetString("last", IntValue(int64(row.Last)))
			setOptionalTAPDirective(item, row.Directive, row.Reason)
		case "test":
			item.RawSetString("ok", BoolValue(row.OK))
			if row.Number > 0 {
				item.RawSetString("number", IntValue(int64(row.Number)))
			}
			item.RawSetString("name", StringValue(row.Name))
			setOptionalTAPDirective(item, row.Directive, row.Reason)
			diagnostics := NewAppendArrayTable(len(row.Diagnostics))
			for j, diagnostic := range row.Diagnostics {
				diagnostics.RawSetInt(int64(j+1), StringValue(diagnostic))
			}
			item.RawSetString("diagnostics", TableValue(diagnostics))
		case "diagnostic":
			item.RawSetString("text", StringValue(row.Text))
		}
		out.RawSetInt(int64(i+1), TableValue(item))
	}
	return TableValue(out)
}

func setOptionalTAPDirective(item *Table, directive, reason string) {
	if directive != "" {
		item.RawSetString("directive", StringValue(directive))
	}
	if reason != "" {
		item.RawSetString("reason", StringValue(reason))
	}
}

func tapDocumentFromValue(v Value) (dialectlib.TAPDocument, error) {
	if !v.IsTable() {
		return dialectlib.TAPDocument{}, fmt.Errorf("table expected")
	}
	tbl := v.Table()
	doc := dialectlib.TAPDocument{Rows: make([]dialectlib.TAPRow, 0, tbl.Length())}
	for i := 1; i <= tbl.Length(); i++ {
		item := tbl.RawGetInt(int64(i))
		if !item.IsTable() {
			return dialectlib.TAPDocument{}, fmt.Errorf("row %d: table expected", i)
		}
		row, err := tapRowFromTable(i, item.Table())
		if err != nil {
			return dialectlib.TAPDocument{}, err
		}
		doc.Rows = append(doc.Rows, row)
	}
	return doc, nil
}

func tapRowFromTable(index int, tbl *Table) (dialectlib.TAPRow, error) {
	kind := tbl.RawGetString("kind")
	if !kind.IsString() || kind.Str() == "" {
		return dialectlib.TAPRow{}, fmt.Errorf("row %d: string kind expected", index)
	}
	row := dialectlib.TAPRow{
		Kind:      kind.Str(),
		Line:      tapOptionalInt(tbl.RawGetString("line")),
		Version:   tapOptionalInt(tbl.RawGetString("version")),
		OK:        tbl.RawGetString("ok").Truthy(),
		Number:    tapOptionalInt(tbl.RawGetString("number")),
		Name:      tapOptionalString(tbl.RawGetString("name")),
		Directive: tapOptionalString(tbl.RawGetString("directive")),
		Reason:    tapOptionalString(tbl.RawGetString("reason")),
		First:     tapOptionalInt(tbl.RawGetString("first")),
		Last:      tapOptionalInt(tbl.RawGetString("last")),
		Text:      tapOptionalString(tbl.RawGetString("text")),
	}
	if diagnostics := tbl.RawGetString("diagnostics"); diagnostics.IsTable() {
		row.Diagnostics = make([]string, 0, diagnostics.Table().Length())
		for i := 1; i <= diagnostics.Table().Length(); i++ {
			row.Diagnostics = append(row.Diagnostics, diagnostics.Table().RawGetInt(int64(i)).String())
		}
	}
	return row, nil
}

func tapOptionalString(v Value) string {
	if v.IsNil() {
		return ""
	}
	return v.String()
}

func tapOptionalInt(v Value) int {
	switch {
	case v.IsInt():
		return int(v.Int())
	case v.IsFloat():
		return int(v.Float())
	default:
		return 0
	}
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func dialectJUnit(body Value, _ *Table) ([]Value, error) {
	report, err := dialectlib.ParseJUnit(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{TableValue(junitReportToTable(report))}, nil
}

func junitReportToTable(report dialectlib.JUnitReport) *Table {
	out := NewTable()
	out.RawSetString("name", StringValue(report.Name))
	out.RawSetString("tests", IntValue(int64(report.Tests)))
	out.RawSetString("failures", IntValue(int64(report.Failures)))
	out.RawSetString("errors", IntValue(int64(report.Errors)))
	out.RawSetString("skipped", IntValue(int64(report.Skipped)))
	out.RawSetString("passed", IntValue(int64(report.Passed)))
	out.RawSetString("time", FloatValue(report.Time))
	out.RawSetString("suites", TableValue(junitSuitesToTable(report.Suites)))
	out.RawSetString("cases", TableValue(junitCasesToTable(report.Cases)))
	return out
}

func junitSuitesToTable(suites []dialectlib.JUnitSuite) *Table {
	out := NewAppendArrayTable(len(suites))
	for i, suite := range suites {
		item := NewTable()
		item.RawSetString("name", StringValue(suite.Name))
		item.RawSetString("tests", IntValue(int64(suite.Tests)))
		item.RawSetString("failures", IntValue(int64(suite.Failures)))
		item.RawSetString("errors", IntValue(int64(suite.Errors)))
		item.RawSetString("skipped", IntValue(int64(suite.Skipped)))
		item.RawSetString("passed", IntValue(int64(suite.Passed)))
		item.RawSetString("time", FloatValue(suite.Time))
		item.RawSetString("cases", TableValue(junitCasesToTable(suite.Cases)))
		out.RawSetInt(int64(i+1), TableValue(item))
	}
	return out
}

func junitCasesToTable(cases []dialectlib.JUnitCase) *Table {
	out := NewAppendArrayTable(len(cases))
	for i, tc := range cases {
		item := NewTable()
		item.RawSetString("name", StringValue(tc.Name))
		item.RawSetString("classname", StringValue(tc.ClassName))
		item.RawSetString("className", StringValue(tc.ClassName))
		item.RawSetString("time", FloatValue(tc.Time))
		item.RawSetString("status", StringValue(tc.Status))
		if tc.Message != "" {
			item.RawSetString("message", StringValue(tc.Message))
		}
		if tc.Type != "" {
			item.RawSetString("type", StringValue(tc.Type))
		}
		if tc.Text != "" {
			item.RawSetString("text", StringValue(tc.Text))
		}
		out.RawSetInt(int64(i+1), TableValue(item))
	}
	return out
}

func dialectTemplate(body Value, opts *Table, maxHostResult func() int64) ([]Value, error) {
	src := body.Str()
	data := NilValue()
	if body.IsTable() {
		tbl := body.Table()
		if text := tbl.RawGetString("text"); text.IsString() {
			src = text.Str()
		} else if text := tbl.RawGetString("template"); text.IsString() {
			src = text.Str()
		}
		data = tbl.RawGetString("data")
	}
	if opts != nil {
		if text := opts.RawGetString("text"); text.IsString() {
			src = text.Str()
		}
		if optData := opts.RawGetString("data"); !optData.IsNil() {
			data = optData
		}
	}
	tpl, err := template.New("dialect").Option(templateMissingKeyOption(opts)).Parse(src)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	buf := newHostResultBuffer(hostResultLimit(maxHostResult))
	if err := tpl.Execute(buf, runtime.JSONValueToGo(data)); err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{StringValue(buf.String())}, nil
}

func templateMissingKeyOption(opts *Table) string {
	mode := "missingkey=zero"
	if opts != nil && opts.RawGetString("missingkey").IsString() {
		mode = opts.RawGetString("missingkey").Str()
	}
	return dialectlib.TemplateMissingKeyOption(mode)
}
