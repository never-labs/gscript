package runtime

import (
	"encoding/base64"
	"fmt"
)

// buildBase64Lib creates the "base64" standard library table.
func buildBase64Lib() *Table {
	t := NewTable()

	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "base64." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setFastArg1Ret2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, Value, int, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:         "base64." + name,
			Fn:           fn,
			FastArg1Ret2: fast,
		}))
	}

	// base64.encode(str) -> standard base64 encoded string
	base64Encode := func(arg Value) (Value, error) {
		if !arg.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'base64.encode' (string expected)")
		}
		return StringValue(base64.StdEncoding.EncodeToString([]byte(arg.Str()))), nil
	}
	setFastArg1("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'base64.encode'")
		}
		v, err := base64Encode(args[0])
		return []Value{v}, err
	}, base64Encode)

	// base64.decode(str) -> decoded string, or nil, "error message"
	base64Decode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'base64.decode' (string expected)")
		}
		decoded, err := base64.StdEncoding.DecodeString(arg.Str())
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return StringValue(string(decoded)), NilValue(), 1, nil
	}
	setFastArg1Ret2("decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'base64.decode'")
		}
		r0, r1, n, err := base64Decode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, base64Decode)

	// base64.urlEncode(str) -> URL-safe base64 encoded string (no padding)
	base64URLEncode := func(arg Value) (Value, error) {
		if !arg.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'base64.urlEncode' (string expected)")
		}
		return StringValue(base64.RawURLEncoding.EncodeToString([]byte(arg.Str()))), nil
	}
	setFastArg1("urlEncode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'base64.urlEncode'")
		}
		v, err := base64URLEncode(args[0])
		return []Value{v}, err
	}, base64URLEncode)

	// base64.urlDecode(str) -> decoded string, or nil, "error message"
	base64URLDecode := func(arg Value) (Value, Value, int, error) {
		if !arg.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'base64.urlDecode' (string expected)")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(arg.Str())
		if err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return StringValue(string(decoded)), NilValue(), 1, nil
	}
	setFastArg1Ret2("urlDecode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'base64.urlDecode'")
		}
		r0, r1, n, err := base64URLDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n == 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, base64URLDecode)

	return t
}
