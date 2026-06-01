package bind

import (
	"fmt"

	urllib "github.com/never-labs/leia/internal/stdlib/lib/url"
)

// BuildURL creates the "url" standard library table.
func BuildURL(maxHostResult func() int64) *Table {
	t := NewTable()
	checkURLString := func(s string) (Value, error) {
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), len(s)); err != nil {
			return NilValue(), err
		}
		return StringValue(s), nil
	}

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "url." + name,
			Fn:   fn,
		}))
	}
	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "url." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setFastArg1Ret2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, Value, int, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:         "url." + name,
			Fn:           fn,
			FastArg1Ret2: fast,
		}))
	}
	setFastArg2Ret2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, Value, int, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:         "url." + name,
			Fn:           fn,
			FastArg2Ret2: fast,
		}))
	}

	// url.parse(str) -- parse URL string -> table
	set("parse", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.parse' (string expected)")
		}
		u, err := urllib.Parse(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}

		result := NewTableSized(0, 8)
		result.RawSetString("scheme", StringValue(u.Scheme))
		result.RawSetString("host", StringValue(u.Host))
		result.RawSetString("port", StringValue(u.Port))
		result.RawSetString("path", StringValue(u.Path))
		result.RawSetString("fragment", StringValue(u.Fragment))
		if err := CheckProjectedHostStringBytes(hostResultLimit(maxHostResult), len(u.Raw)); err != nil {
			return nil, err
		}
		result.RawSetString("raw", StringValue(u.Raw))

		// User info
		if u.HasUser {
			result.RawSetString("user", StringValue(u.User))
			if u.Password != nil {
				result.RawSetString("password", StringValue(*u.Password))
			}
		}

		// Query params as table
		queryTbl := NewTableSized(0, len(u.Query))
		for k, v := range u.Query {
			queryTbl.RawSetString(k, StringValue(v))
		}
		result.RawSetString("query", TableValue(queryTbl))

		return []Value{TableValue(result)}, nil
	})

	// url.build(t) -- build URL from table -> string
	set("build", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'url.build' (table expected)")
		}
		tbl := args[0].Table()
		var parts urllib.Parts
		if v := tbl.RawGet(StringValue("scheme")); v.IsString() {
			parts.Scheme = v.Str()
		}
		if v := tbl.RawGet(StringValue("port")); v.IsString() && v.Str() != "" {
			parts.Port = v.Str()
		}
		if v := tbl.RawGet(StringValue("host")); v.IsString() {
			parts.Host = v.Str()
		}
		if v := tbl.RawGet(StringValue("path")); v.IsString() {
			parts.Path = v.Str()
		}
		if v := tbl.RawGet(StringValue("fragment")); v.IsString() {
			parts.Fragment = v.Str()
		}
		if v := tbl.RawGet(StringValue("user")); v.IsString() {
			parts.HasUser = true
			parts.User = v.Str()
			if pwd := tbl.RawGet(StringValue("password")); pwd.IsString() {
				p := pwd.Str()
				parts.Password = &p
			}
		}
		if v := tbl.RawGet(StringValue("query")); v.IsTable() {
			parts.Query = make(map[string]string)
			qTbl := v.Table()
			k, val, ok := qTbl.Next(NilValue())
			for ok {
				parts.Query[k.String()] = val.String()
				k, val, ok = qTbl.Next(k)
			}
		}
		v, err := checkURLString(urllib.Build(parts))
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})

	// url.encode(str) -- percent-encode a string
	urlEncode := func(arg Value) (Value, error) {
		if !arg.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'url.encode' (string expected)")
		}
		return checkURLString(urllib.Encode(arg.Str()))
	}
	setFastArg1("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'url.encode' (string expected)")
		}
		v, err := urlEncode(args[0])
		return []Value{v}, err
	}, urlEncode)

	// url.decode(str) -- percent-decode a string
	urlDecode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'url.decode' (string expected)")
		}
		decoded, err := urllib.Decode(arg.Str())
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		v, err := checkURLString(decoded)
		if err != nil {
			return NilValue(), NilValue(), 0, err
		}
		return v, NilValue(), 1, nil
	}
	setFastArg1Ret2("decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'url.decode' (string expected)")
		}
		r0, r1, n, err := urlDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, urlDecode)

	// url.queryEncode(t) -- encode table as URL query string: "a=1&b=2"
	urlQueryEncode := func(arg Value) (Value, error) {
		if !arg.IsTable() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'url.queryEncode' (table expected)")
		}
		tbl := arg.Table()
		q := make(map[string]string)
		k, v, ok := tbl.Next(NilValue())
		for ok {
			q[k.String()] = v.String()
			k, v, ok = tbl.Next(k)
		}
		return checkURLString(urllib.QueryEncode(q))
	}
	setFastArg1("queryEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'url.queryEncode' (table expected)")
		}
		v, err := urlQueryEncode(args[0])
		return []Value{v}, err
	}, urlQueryEncode)

	// url.queryDecode(str) -- decode query string -> table
	urlQueryDecode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'url.queryDecode' (string expected)")
		}
		vals, err := urllib.QueryDecode(arg.Str())
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		tbl := NewTableSized(0, len(vals))
		for k, v := range vals {
			tbl.RawSetString(k, StringValue(v))
		}
		return TableValue(tbl), NilValue(), 1, nil
	}
	setFastArg1Ret2("queryDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'url.queryDecode' (string expected)")
		}
		r0, r1, n, err := urlQueryDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, urlQueryDecode)

	// url.join(base, ref) -- resolve ref relative to base URL
	urlJoin := func(baseVal, refVal Value) (Value, Value, int, error) {
		if !baseVal.IsString() || !refVal.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument to 'url.join' (string expected)")
		}
		joined, err := urllib.Join(baseVal.Str(), refVal.Str())
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		v, err := checkURLString(joined)
		if err != nil {
			return NilValue(), NilValue(), 0, err
		}
		return v, NilValue(), 1, nil
	}
	setFastArg2Ret2("join", func(args []Value) ([]Value, error) {
		if len(args) < 2 {
			return nil, fmt.Errorf("bad argument to 'url.join' (string expected)")
		}
		r0, r1, n, err := urlJoin(args[0], args[1])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, urlJoin)

	// url.isValid(str) -- bool
	set("isValid", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.isValid' (string expected)")
		}
		return []Value{BoolValue(urllib.IsValid(args[0].Str()))}, nil
	})

	// url.getHost(str) -- extract just the host from URL
	set("getHost", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.getHost' (string expected)")
		}
		host, err := urllib.Host(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		v, err := checkURLString(host)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	})

	// url.getPath(str) -- extract just the path
	urlGetPath := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'url.getPath' (string expected)")
		}
		path, err := urllib.Path(arg.Str())
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		v, err := checkURLString(path)
		if err != nil {
			return NilValue(), NilValue(), 0, err
		}
		return v, NilValue(), 1, nil
	}
	setFastArg1Ret2("getPath", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'url.getPath' (string expected)")
		}
		r0, r1, n, err := urlGetPath(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, urlGetPath)

	return t
}
