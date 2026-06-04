package bind

import (
	"bufio"
	"bytes"
	"fmt"
	"mime"
	"net/textproto"
	"net/url"
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
	register([]string{"urlpath"}, dialectHandler{
		eval:  dialectURLPath,
		block: dialectURLPath,
	})
	register([]string{"mime"}, dialectHandler{
		eval:  dialectMIME,
		block: dialectMIME,
	})
	register([]string{"headers", "http_headers"}, dialectHandler{
		eval:  dialectHeaders,
		block: dialectHeaders,
	})
	register([]string{"cookie", "cookies"}, dialectHandler{
		eval:  dialectCookie,
		block: dialectCookie,
	})
	register([]string{"httpmsg"}, dialectHandler{
		eval:  dialectHTTPMessage,
		block: dialectHTTPMessage,
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
		values := url.Values{}
		body.Table().ForEachPlainRaw(func(k, v Value) bool {
			if k.IsString() {
				key := k.Str()
				if v.IsTable() {
					tbl := v.Table()
					for i := 1; i <= tbl.Length(); i++ {
						values.Add(key, tbl.RawGetInt(int64(i)).String())
					}
					return true
				}
				values.Set(key, v.String())
			}
			return true
		})
		return []Value{StringValue(values.Encode())}, nil
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

func dialectURLPath(body Value, opts *Table) ([]Value, error) {
	mode := "escape"
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode_template" || mode == "expand_template" || mode == "format_template" {
		return dialectPathTemplate(body, opts, true)
	}
	switch mode {
	case "escape", "encode", "encode_component", "":
		return []Value{StringValue(dialectlib.URLPathEscape(body.Str()))}, nil
	case "unescape", "decode", "decode_component":
		decoded, err := dialectlib.URLPathUnescape(body.Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	case "match_template", "template_match":
		return dialectPathTemplate(body, opts, false)
	default:
		return nil, fmt.Errorf("urlpath dialect: unknown mode %q", mode)
	}
}

func dialectPathTemplate(body Value, opts *Table, encode bool) ([]Value, error) {
	template := ""
	if opts != nil {
		if v := opts.RawGetString("template"); v.IsString() {
			template = v.Str()
		}
	}
	if template == "" {
		template = body.Str()
	}
	if encode {
		params, err := stringMapFromTable(body, "urlpath")
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		path, err := dialectlib.ExpandPathTemplate(template, params)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(path)}, nil
	}
	path := ""
	if opts != nil && opts.RawGetString("path").IsString() {
		path = opts.RawGetString("path").Str()
	} else {
		path = body.Str()
	}
	match, err := dialectlib.MatchPathTemplate(template, path)
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	out := NewTable()
	out.RawSetString("matched", BoolValue(match.Matched))
	out.RawSetString("ok", BoolValue(match.Matched))
	out.RawSetString("template", StringValue(match.Template))
	out.RawSetString("path", StringValue(match.Path))
	params := NewTable()
	for key, val := range match.Params {
		params.RawSetString(key, StringValue(val))
	}
	out.RawSetString("params", TableValue(params))
	return []Value{TableValue(out)}, nil
}

func stringMapFromTable(body Value, dialectName string) (map[string]string, error) {
	if !body.IsTable() {
		return nil, fmt.Errorf("%s dialect: table required for encode", dialectName)
	}
	out := make(map[string]string)
	body.Table().ForEachPlainRaw(func(k, v Value) bool {
		if k.IsString() {
			out[k.Str()] = v.String()
		}
		return true
	})
	return out, nil
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

func dialectCookie(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode" || mode == "format" {
		text, err := encodeCookiePairs(body)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(text)}, nil
	}
	cookies, err := parseCookiePairs(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{cookies}, nil
}

func parseCookiePairs(src string) (Value, error) {
	out := NewTable()
	if strings.TrimSpace(src) == "" {
		return TableValue(out), nil
	}
	for _, part := range strings.Split(src, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, "=")
		if !ok {
			return NilValue(), fmt.Errorf("cookie dialect: invalid cookie pair %q", part)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if !isHeaderFieldName(name) {
			return NilValue(), fmt.Errorf("cookie dialect: invalid cookie name %q", name)
		}
		if !isCookieValue(value) {
			return NilValue(), fmt.Errorf("cookie dialect: invalid cookie value for %q", name)
		}
		addCookieValue(out, name, value)
	}
	return TableValue(out), nil
}

func addCookieValue(out *Table, name, value string) {
	existing := out.RawGetString(name)
	if existing.IsNil() {
		out.RawSetString(name, StringValue(value))
		return
	}
	if existing.IsTable() {
		arr := existing.Table()
		arr.RawSetInt(int64(arr.Length()+1), StringValue(value))
		return
	}
	arr := NewAppendArrayTable(2)
	arr.RawSetInt(1, existing)
	arr.RawSetInt(2, StringValue(value))
	out.RawSetString(name, TableValue(arr))
}

func encodeCookiePairs(body Value) (string, error) {
	if !body.IsTable() {
		return "", fmt.Errorf("cookie dialect: table required for encode")
	}
	type cookiePair struct {
		name  string
		value string
	}
	var pairs []cookiePair
	var invalidName string
	var invalidValueName string
	body.Table().ForEachPlainRaw(func(k, v Value) bool {
		if !k.IsString() {
			return true
		}
		name := k.Str()
		if !isHeaderFieldName(name) {
			invalidName = name
			return false
		}
		if v.IsTable() {
			tbl := v.Table()
			for i := 1; i <= tbl.Length(); i++ {
				val := tbl.RawGetInt(int64(i)).String()
				if !isCookieValue(val) {
					invalidValueName = name
					return false
				}
				pairs = append(pairs, cookiePair{name: name, value: val})
			}
			return true
		}
		val := v.String()
		if !isCookieValue(val) {
			invalidValueName = name
			return false
		}
		pairs = append(pairs, cookiePair{name: name, value: val})
		return true
	})
	if invalidName != "" {
		return "", fmt.Errorf("cookie dialect: invalid cookie name %q", invalidName)
	}
	if invalidValueName != "" {
		return "", fmt.Errorf("cookie dialect: invalid cookie value for %q", invalidValueName)
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		return pairs[i].name < pairs[j].name
	})
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, pair.name+"="+pair.value)
	}
	return strings.Join(parts, "; "), nil
}

func isCookieValue(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < 0x21 || c == '"' || c == ',' || c == ';' || c == '\\' || c == 0x7f {
			return false
		}
	}
	return true
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
	return dialectlib.IsHTTPHeaderFieldName(name)
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

func dialectHTTPMessage(body Value, opts *Table) ([]Value, error) {
	mode := ""
	if opts != nil && opts.RawGetString("mode").IsString() {
		mode = opts.RawGetString("mode").Str()
	}
	if body.IsTable() || mode == "encode" || mode == "format" {
		msg, err := httpMessageFromTable(body)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		text, err := dialectlib.EncodeHTTPMessage(msg)
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(text)}, nil
	}
	msg, err := dialectlib.ParseHTTPMessage(body.Str())
	if err != nil {
		return []Value{NilValue(), StringValue(err.Error())}, nil
	}
	return []Value{httpMessageToTable(msg)}, nil
}

func httpMessageToTable(msg dialectlib.HTTPMessage) Value {
	out := NewTable()
	out.RawSetString("type", StringValue(msg.Type))
	out.RawSetString("startLine", StringValue(msg.StartLine))
	out.RawSetString("version", StringValue(msg.Version))
	if msg.Method != "" {
		out.RawSetString("method", StringValue(msg.Method))
		out.RawSetString("target", StringValue(msg.Target))
	}
	if msg.StatusCode != 0 {
		out.RawSetString("status", IntValue(int64(msg.StatusCode)))
		out.RawSetString("statusCode", IntValue(int64(msg.StatusCode)))
		out.RawSetString("reason", StringValue(msg.Reason))
	}
	out.RawSetString("headers", httpHeadersToTable(msg.Headers))
	out.RawSetString("body", StringValue(msg.Body))
	return TableValue(out)
}

func httpHeadersToTable(headers map[string][]string) Value {
	out := NewTable()
	for key, vals := range headers {
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
	return TableValue(out)
}

func httpMessageFromTable(body Value) (dialectlib.HTTPMessage, error) {
	if !body.IsTable() {
		return dialectlib.HTTPMessage{}, fmt.Errorf("httpmsg dialect: table required for encode")
	}
	tbl := body.Table()
	msg := dialectlib.HTTPMessage{Headers: make(map[string][]string)}
	if v := tbl.RawGetString("type"); v.IsString() {
		msg.Type = v.Str()
	}
	if v := tbl.RawGetString("startLine"); v.IsString() {
		msg.StartLine = v.Str()
	}
	if v := tbl.RawGetString("method"); v.IsString() {
		msg.Method = v.Str()
		if msg.Type == "" {
			msg.Type = "request"
		}
	}
	if v := tbl.RawGetString("target"); v.IsString() {
		msg.Target = v.Str()
	} else if v := tbl.RawGetString("path"); v.IsString() {
		msg.Target = v.Str()
	}
	if v := tbl.RawGetString("version"); v.IsString() {
		msg.Version = v.Str()
	}
	if v := tbl.RawGetString("status"); v.IsInt() || v.IsFloat() {
		msg.StatusCode = int(toInt(v))
		if msg.Type == "" {
			msg.Type = "response"
		}
	} else if v := tbl.RawGetString("statusCode"); v.IsInt() || v.IsFloat() {
		msg.StatusCode = int(toInt(v))
		if msg.Type == "" {
			msg.Type = "response"
		}
	}
	if v := tbl.RawGetString("reason"); v.IsString() {
		msg.Reason = v.Str()
	}
	if v := tbl.RawGetString("body"); !v.IsNil() {
		msg.Body = v.String()
	}
	if v := tbl.RawGetString("headers"); v.IsTable() {
		headers, err := httpHeadersFromTable(v.Table())
		if err != nil {
			return dialectlib.HTTPMessage{}, err
		}
		msg.Headers = headers
	}
	return msg, nil
}

func httpHeadersFromTable(tbl *Table) (map[string][]string, error) {
	headers := make(map[string][]string)
	var invalidKey string
	var invalidValueKey string
	tbl.ForEachPlainRaw(func(k, v Value) bool {
		if !k.IsString() {
			return true
		}
		key := k.Str()
		if !dialectlib.IsHTTPHeaderFieldName(key) {
			invalidKey = key
			return false
		}
		if v.IsTable() {
			arr := v.Table()
			for i := 1; i <= arr.Length(); i++ {
				val := arr.RawGetInt(int64(i)).String()
				if strings.ContainsAny(val, "\r\n") {
					invalidValueKey = key
					return false
				}
				headers[key] = append(headers[key], val)
			}
			return true
		}
		val := v.String()
		if strings.ContainsAny(val, "\r\n") {
			invalidValueKey = key
			return false
		}
		headers[key] = append(headers[key], val)
		return true
	})
	if invalidKey != "" {
		return nil, fmt.Errorf("httpmsg dialect: invalid header name %q", invalidKey)
	}
	if invalidValueKey != "" {
		return nil, fmt.Errorf("httpmsg dialect: invalid header value for %q", invalidValueKey)
	}
	return headers, nil
}
