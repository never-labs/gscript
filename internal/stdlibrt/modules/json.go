package modules

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
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
		data, err := json.Marshal(jsonValueToGo(v))
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
		return jsonGoToValue(goVal), NilValue(), 1, nil
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
		data, err := json.MarshalIndent(jsonValueToGo(args[0]), "", indent)
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

func jsonValueToGo(v Value) any {
	switch v.Type() {
	case TypeNil:
		return nil
	case TypeBool:
		return v.Bool()
	case TypeInt:
		return v.Int()
	case TypeFloat:
		f := v.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil
		}
		return f
	case TypeString:
		return v.Str()
	case TypeTable:
		return jsonTableToGo(v.Table())
	default:
		return v.String()
	}
}

func jsonTableToGo(tbl *Table) any {
	length := tbl.Length()
	hasHashKeys := false
	totalKeys := 0

	key := NilValue()
	for {
		k, _, ok := tbl.Next(key)
		if !ok {
			break
		}
		totalKeys++
		if !k.IsInt() {
			hasHashKeys = true
		}
		key = k
	}

	if !hasHashKeys && length > 0 && totalKeys == length {
		arr := make([]any, length)
		for i := 1; i <= length; i++ {
			arr[i-1] = jsonValueToGo(tbl.RawGet(IntValue(int64(i))))
		}
		return arr
	}

	m := make(map[string]any)
	key = NilValue()
	for {
		k, val, ok := tbl.Next(key)
		if !ok {
			break
		}
		if k.IsString() {
			m[k.Str()] = jsonValueToGo(val)
		} else {
			m[k.String()] = jsonValueToGo(val)
		}
		key = k
	}
	return m
}

func jsonGoToValue(v any) Value {
	switch val := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(val)
	case json.Number:
		if i, err := val.Int64(); err == nil && strconv.FormatInt(i, 10) == val.String() {
			return IntValue(i)
		}
		if f, err := val.Float64(); err == nil {
			return FloatValue(f)
		}
		return StringValue(val.String())
	case float64:
		if float64(int64(val)) == val && !math.IsInf(val, 0) {
			return IntValue(int64(val))
		}
		return FloatValue(val)
	case string:
		return StringValue(val)
	case []any:
		tbl := NewSequentialArrayTable(len(val))
		for i, item := range val {
			tbl.RawSetInt(int64(i+1), jsonGoToValue(item))
		}
		return TableValue(tbl)
	case map[string]any:
		tbl := NewTableSized(0, len(val))
		for k, item := range val {
			tbl.RawSetString(k, jsonGoToValue(item))
		}
		return TableValue(tbl)
	default:
		return StringValue(fmt.Sprintf("%v", val))
	}
}
