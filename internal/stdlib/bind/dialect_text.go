package bind

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/never-labs/leia/internal/runtime"
	csvlib "github.com/never-labs/leia/internal/stdlib/lib/csv"
	dialectlib "github.com/never-labs/leia/internal/support/dialect"
)

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
		return []Value{runtime.JSONGoToValue(goVal)}, nil
	}
	data, err := json.Marshal(runtime.JSONValueToGo(body))
	if err != nil {
		return nil, fmt.Errorf("json dialect: %v", err)
	}
	return []Value{StringValue(string(data))}, nil
}

func dialectCSV(src string, opts *Table) ([]Value, error) {
	csvOpts := csvDialectOptions(opts)
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
