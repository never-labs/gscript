package bind

import (
	"bufio"
	"bytes"
	"fmt"
	"mime"
	"net/textproto"
	"sort"
	"strings"

	dialectlib "github.com/never-labs/leia/internal/support/dialect"
)

func registerDialectWeb(register dialectRegisterFunc) {
	register([]string{"url"}, dialectHandler{
		eval: func(body Value, _ *Table) ([]Value, error) {
			return dialectURL(body.Str())
		},
	})
	register([]string{"html_escape"}, dialectHandler{
		eval: func(body Value, options *Table) ([]Value, error) {
			return dialectHTMLEscape(body.Str(), options)
		},
	})
	register([]string{"urlquery"}, dialectHandler{
		eval:  dialectURLQuery,
		block: dialectURLQuery,
	})
	register([]string{"mime"}, dialectHandler{
		eval:  dialectMIME,
		block: dialectMIME,
	})
	register([]string{"headers", "http_headers"}, dialectHandler{
		eval:  dialectHeaders,
		block: dialectHeaders,
	})
}

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

func dialectMIME(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode" || mode == "format" {
		return dialectMIMEEncode(body, opts)
	}

	mediaType, params, err := mime.ParseMediaType(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	result := NewTable()
	result.RawSetString("type", StringValue(mediaType))
	result.RawSetString("raw", StringValue(body.Str()))
	paramTable := NewTable()
	for key, val := range params {
		paramTable.RawSetString(key, StringValue(val))
	}
	result.RawSetString("params", TableValue(paramTable))
	return []Value{TableValue(result)}, nil
}

func dialectMIMEEncode(body Value, opts *Table) ([]Value, error) {
	mediaType := ""
	paramsValue := NilValue()
	if body.IsTable() {
		tbl := body.Table()
		if v := tbl.RawGetString("type"); v.IsString() {
			mediaType = v.Str()
		}
		paramsValue = tbl.RawGetString("params")
	} else {
		mediaType = body.Str()
	}
	if opts != nil {
		if v := opts.RawGetString("type"); v.IsString() {
			mediaType = v.Str()
		}
		if v := opts.RawGetString("params"); v.IsTable() {
			paramsValue = v
		}
	}
	if mediaType == "" {
		return nil, fmt.Errorf("mime dialect: media type required for encode")
	}

	params := make(map[string]string)
	if paramsValue.IsTable() {
		paramsValue.Table().ForEachPlainRaw(func(k, v Value) bool {
			if k.IsString() {
				params[k.Str()] = v.String()
			}
			return true
		})
	}
	formatted := mime.FormatMediaType(mediaType, params)
	if formatted == "" {
		return []Value{NilValue(), StringValue("invalid media type or parameter")}, nil
	}
	return []Value{StringValue(formatted)}, nil
}

func dialectHeaders(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode" || mode == "format" {
		text, err := encodeHeaderFields(body)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(text)}, nil
	}
	fields, err := parseHeaderFields(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{fields}, nil
}

func parseHeaderFields(src string) (Value, error) {
	reader := textproto.NewReader(bufio.NewReader(strings.NewReader(src + "\r\n\r\n")))
	mimeHeader, err := reader.ReadMIMEHeader()
	if err != nil {
		return NilValue(), err
	}
	out := NewTable()
	for key, vals := range mimeHeader {
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
	return TableValue(out), nil
}

func encodeHeaderFields(body Value) (string, error) {
	if !body.IsTable() {
		return "", fmt.Errorf("headers dialect: table required for encode")
	}
	header := textproto.MIMEHeader{}
	var invalidKey string
	var invalidValueKey string
	body.Table().ForEachPlainRaw(func(k, v Value) bool {
		if !k.IsString() {
			return true
		}
		if !isHeaderFieldName(k.Str()) {
			invalidKey = k.Str()
			return false
		}
		name := textproto.CanonicalMIMEHeaderKey(k.Str())
		if v.IsTable() {
			tbl := v.Table()
			for i := 1; i <= tbl.Length(); i++ {
				val := tbl.RawGetInt(int64(i)).String()
				if strings.ContainsAny(val, "\r\n") {
					invalidValueKey = name
					return false
				}
				header.Add(name, val)
			}
			return true
		}
		val := v.String()
		if strings.ContainsAny(val, "\r\n") {
			invalidValueKey = name
			return false
		}
		header.Set(name, val)
		return true
	})
	if invalidKey != "" {
		return "", fmt.Errorf("headers dialect: invalid header name %q", invalidKey)
	}
	if invalidValueKey != "" {
		return "", fmt.Errorf("headers dialect: invalid header value for %q", invalidValueKey)
	}

	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var buf bytes.Buffer
	for _, key := range keys {
		for _, val := range header[key] {
			fmt.Fprintf(&buf, "%s: %s\r\n", key, val)
		}
	}
	return buf.String(), nil
}

func isHeaderFieldName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' {
			continue
		}
		switch c {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
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
