package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/never-labs/gscript/internal/runtime"
)

// BuildJSON creates the "json" standard library table.
func BuildJSON() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "json." + name,
			Fn:   fn,
		}))
	}
	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:     "json." + name,
			Fn:       fn,
			FastArg1: fast,
		}))
	}
	setFastArg1Ret2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, Value, int, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name:         "json." + name,
			Fn:           fn,
			FastArg1Ret2: fast,
		}))
	}

	jsonEncode := func(v Value) (Value, error) {
		data, err := json.Marshal(runtime.JSONValueToGo(v))
		if err != nil {
			return NilValue(), fmt.Errorf("json.encode: %v", err)
		}
		return StringValue(string(data)), nil
	}
	setFastArg1("encode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'json.encode'")
		}
		v, err := jsonEncode(args[0])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, jsonEncode)

	jsonDecode := func(v Value) (Value, Value, int, error) {
		if !v.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'json.decode' (string expected)")
		}
		decoder := json.NewDecoder(strings.NewReader(v.Str()))
		decoder.UseNumber()

		var goVal any
		if err := decoder.Decode(&goVal); err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return NilValue(), StringValue("invalid JSON: trailing data"), 2, nil
			}
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return runtime.JSONGoToValue(goVal), NilValue(), 1, nil
	}
	setFastArg1Ret2("decode", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'json.decode'")
		}
		r0, r1, n, err := jsonDecode(args[0])
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return []Value{r0}, nil
		}
		return []Value{r0, r1}, nil
	}, jsonDecode)

	jsonValid := func(v Value) (Value, error) {
		if !v.IsString() {
			return NilValue(), fmt.Errorf("bad argument #1 to 'json.valid' (string expected)")
		}
		return BoolValue(json.Valid([]byte(v.Str()))), nil
	}
	setFastArg1("valid", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'json.valid'")
		}
		v, err := jsonValid(args[0])
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, jsonValid)

	jsonPretty := func(name string, args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'json.%s'", name)
		}
		indent := "  "
		if len(args) >= 2 && args[1].IsString() {
			indent = args[1].Str()
		}
		data, err := json.MarshalIndent(runtime.JSONValueToGo(args[0]), "", indent)
		if err != nil {
			return nil, fmt.Errorf("json.%s: %v", name, err)
		}
		return []Value{StringValue(string(data))}, nil
	}

	set("pretty", func(args []Value) ([]Value, error) {
		return jsonPretty("pretty", args)
	})
	set("indent", func(args []Value) ([]Value, error) {
		return jsonPretty("indent", args)
	})

	return t
}
