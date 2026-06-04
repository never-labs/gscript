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
			return dialectCSV(body.Str(), options)
		},
	})
	register([]string{"tsv"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectTSV(body.Str(), options)
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
	register([]string{"env"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectKV(body.Str(), options, true)
		},
	})
	register([]string{"ini"}, dialectHandler{
		eval:  dialectINI,
		block: dialectINI,
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

func dialectCSV(src string, opts *Table) ([]Value, error) {
	return dialectDelimited(src, opts, 0)
}

func dialectTSV(src string, opts *Table) ([]Value, error) {
	return dialectDelimited(src, opts, '\t')
}

func dialectDelimited(src string, opts *Table, defaultSep rune) ([]Value, error) {
	csvOpts := csvDialectOptions(opts)
	if csvOpts.Sep == 0 {
		csvOpts.Sep = defaultSep
	}
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

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
