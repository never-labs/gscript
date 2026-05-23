package runtime

import (
	"fmt"
	"net/url"
)

// buildURLLib creates the "url" standard library table.
func buildURLLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "url." + name,
			Fn:   fn,
		}))
	}

	// url.parse(str) -- parse URL string -> table
	set("parse", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.parse' (string expected)")
		}
		u, err := url.Parse(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}

		result := NewTableSized(0, 8)
		result.RawSetString("scheme", StringValue(u.Scheme))
		result.RawSetString("host", StringValue(u.Hostname()))
		result.RawSetString("port", StringValue(u.Port()))
		result.RawSetString("path", StringValue(u.Path))
		result.RawSetString("fragment", StringValue(u.Fragment))
		result.RawSetString("raw", StringValue(u.String()))

		// User info
		if u.User != nil {
			result.RawSetString("user", StringValue(u.User.Username()))
			if pwd, ok := u.User.Password(); ok {
				result.RawSetString("password", StringValue(pwd))
			}
		}

		// Query params as table
		query := u.Query()
		queryTbl := NewTableSized(0, len(query))
		for k, vals := range query {
			if len(vals) > 0 {
				queryTbl.RawSetString(k, StringValue(vals[0]))
			}
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
		u := &url.URL{}
		if v := tbl.RawGet(StringValue("scheme")); v.IsString() {
			u.Scheme = v.Str()
		}
		host := ""
		if v := tbl.RawGet(StringValue("host")); v.IsString() {
			host = v.Str()
		}
		if v := tbl.RawGet(StringValue("port")); v.IsString() && v.Str() != "" {
			host = host + ":" + v.Str()
		}
		u.Host = host
		if v := tbl.RawGet(StringValue("path")); v.IsString() {
			u.Path = v.Str()
		}
		if v := tbl.RawGet(StringValue("fragment")); v.IsString() {
			u.Fragment = v.Str()
		}
		if v := tbl.RawGet(StringValue("user")); v.IsString() {
			if pwd := tbl.RawGet(StringValue("password")); pwd.IsString() {
				u.User = url.UserPassword(v.Str(), pwd.Str())
			} else {
				u.User = url.User(v.Str())
			}
		}
		if v := tbl.RawGet(StringValue("query")); v.IsTable() {
			q := url.Values{}
			qTbl := v.Table()
			k, val, ok := qTbl.Next(NilValue())
			for ok {
				q.Set(k.String(), val.String())
				k, val, ok = qTbl.Next(k)
			}
			u.RawQuery = q.Encode()
		}
		return []Value{StringValue(u.String())}, nil
	})

	// url.encode(str) -- percent-encode a string
	set("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.encode' (string expected)")
		}
		return []Value{StringValue(url.QueryEscape(args[0].Str()))}, nil
	})

	// url.decode(str) -- percent-decode a string
	set("decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.decode' (string expected)")
		}
		decoded, err := url.QueryUnescape(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(decoded)}, nil
	})

	// url.queryEncode(t) -- encode table as URL query string: "a=1&b=2"
	set("queryEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'url.queryEncode' (table expected)")
		}
		tbl := args[0].Table()
		q := url.Values{}
		k, v, ok := tbl.Next(NilValue())
		for ok {
			q.Set(k.String(), v.String())
			k, v, ok = tbl.Next(k)
		}
		return []Value{StringValue(q.Encode())}, nil
	})

	// url.queryDecode(str) -- decode query string -> table
	set("queryDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.queryDecode' (string expected)")
		}
		vals, err := url.ParseQuery(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		tbl := NewTableSized(0, len(vals))
		for k, v := range vals {
			if len(v) > 0 {
				tbl.RawSetString(k, StringValue(v[0]))
			}
		}
		return []Value{TableValue(tbl)}, nil
	})

	// url.join(base, ref) -- resolve ref relative to base URL
	set("join", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsString() || !args[1].IsString() {
			return nil, fmt.Errorf("bad argument to 'url.join' (string expected)")
		}
		base, err := url.Parse(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		ref, err := url.Parse(args[1].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(base.ResolveReference(ref).String())}, nil
	})

	// url.isValid(str) -- bool
	set("isValid", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.isValid' (string expected)")
		}
		u, err := url.Parse(args[0].Str())
		if err != nil {
			return []Value{BoolValue(false)}, nil
		}
		// Check that it has a scheme and host
		valid := u.Scheme != "" && u.Host != ""
		return []Value{BoolValue(valid)}, nil
	})

	// url.getHost(str) -- extract just the host from URL
	set("getHost", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.getHost' (string expected)")
		}
		u, err := url.Parse(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(u.Hostname())}, nil
	})

	// url.getPath(str) -- extract just the path
	set("getPath", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsString() {
			return nil, fmt.Errorf("bad argument #1 to 'url.getPath' (string expected)")
		}
		u, err := url.Parse(args[0].Str())
		if err != nil {
			return []Value{NilValue(), StringValue(err.Error())}, nil
		}
		return []Value{StringValue(u.Path)}, nil
	})

	return t
}
