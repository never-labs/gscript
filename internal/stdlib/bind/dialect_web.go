package bind

import dialectlib "github.com/never-labs/leia/internal/support/dialect"

func dialectHTMLEscape(src string, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if mode == "unescape" || mode == "decode" {
		return []Value{StringValue(dialectlib.HTMLUnescape(src))}, nil
	}
	return []Value{StringValue(dialectlib.HTMLEscape(src))}, nil
}

func dialectURLQuery(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() && mode != "decode" && mode != "parse" {
		values := make(map[string]string)
		body.Table().ForEachPlainRaw(func(k, v Value) bool {
			if k.IsString() {
				values[k.Str()] = v.String()
			}
			return true
		})
		return []Value{StringValue(dialectlib.URLQueryEncode(values))}, nil
	}
	if mode == "escape" || mode == "encode_component" {
		return []Value{StringValue(dialectlib.URLQueryEscape(body.Str()))}, nil
	}
	if mode == "unescape" || mode == "decode_component" {
		decoded, err := dialectlib.URLQueryUnescape(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	}
	values, err := dialectlib.URLQueryParse(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	for key, vals := range values {
		if len(vals) == 1 {
			out.RawSetString(key, StringValue(vals[0]))
			continue
		}
		arr := NewAppendArrayTable(len(vals))
		for i, val := range vals {
			arr.RawSetInt(int64(i+1), StringValue(val))
		}
		out.RawSetString(key, TableValue(arr))
	}
	return []Value{TableValue(out)}, nil
}

func dialectURL(src string) ([]Value, error) {
	u, err := dialectlib.ParseURL(src)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	result := NewTable()
	result.RawSetString("scheme", StringValue(u.Scheme))
	result.RawSetString("host", StringValue(u.Host))
	result.RawSetString("port", StringValue(u.Port))
	result.RawSetString("path", StringValue(u.Path))
	result.RawSetString("fragment", StringValue(u.Fragment))
	result.RawSetString("raw", StringValue(u.Raw))
	result.RawSetString("user", StringValue(u.User))
	result.RawSetString("hasUser", BoolValue(u.HasUser))
	if u.Password != nil {
		result.RawSetString("password", StringValue(*u.Password))
	}
	query := NewTable()
	for k, v := range u.Query {
		query.RawSetString(k, StringValue(v))
	}
	result.RawSetString("query", TableValue(query))
	return []Value{TableValue(result)}, nil
}
