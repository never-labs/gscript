package runtime

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// buildJSONLib creates the "json" standard library table.
func buildJSONLib() *Table {
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

	// json.encode(value) -> JSON string
	jsonEncode := func(v Value) (Value, error) {
		goVal := jsonGScriptToGo(v)
		data, err := json.Marshal(goVal)
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

	// json.decode(str) -> GScript value, or nil, "error message"
	jsonDecode := func(v Value) (Value, Value, int, error) {
		if !v.IsString() {
			return NilValue(), NilValue(), 0, fmt.Errorf("bad argument #1 to 'json.decode' (string expected)")
		}
		str := v.Str()
		var goVal interface{}
		decoder := json.NewDecoder(strings.NewReader(str))
		decoder.UseNumber()
		if err := decoder.Decode(&goVal); err != nil {
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		var extra interface{}
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return NilValue(), StringValue("invalid JSON: trailing data"), 2, nil
			}
			return NilValue(), StringValue(err.Error()), 2, nil
		}
		return jsonGoToGScript(goVal), NilValue(), 1, nil
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

	// json.pretty(value [, indent]) -> pretty-printed JSON string
	set("pretty", func(args []Value) ([]Value, error) {
		if len(args) < 1 {
			return nil, fmt.Errorf("bad argument #1 to 'json.pretty'")
		}
		indent := "  " // default 2 spaces
		if len(args) >= 2 && args[1].IsString() {
			indent = args[1].Str()
		}
		goVal := jsonGScriptToGo(args[0])
		data, err := json.MarshalIndent(goVal, "", indent)
		if err != nil {
			return nil, fmt.Errorf("json.pretty: %v", err)
		}
		return []Value{StringValue(string(data))}, nil
	})

	return t
}

// jsonGScriptToGo converts a GScript Value to a Go interface{} suitable for json.Marshal.
// This handles the mixed table case (both int and string keys) by converting to object.
func jsonGScriptToGo(v Value) interface{} {
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
			return nil // JSON doesn't support Inf/NaN
		}
		return f
	case TypeString:
		return v.Str()
	case TypeTable:
		tbl := v.Table()
		return jsonTableToGo(tbl)
	default:
		return v.String()
	}
}

// jsonTableToGo converts a GScript table to either a []interface{} (pure array)
// or a map[string]interface{} (object, including mixed tables).
func jsonTableToGo(tbl *Table) interface{} {
	length := tbl.Length()
	hasHashKeys := false

	// Check if there are any non-integer keys by iterating
	key := NilValue()
	totalKeys := 0
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

	// Pure array: only consecutive integer keys 1..n
	if !hasHashKeys && length > 0 && totalKeys == length {
		arr := make([]interface{}, length)
		for i := 1; i <= length; i++ {
			arr[i-1] = jsonGScriptToGo(tbl.RawGet(IntValue(int64(i))))
		}
		return arr
	}

	// Object: mixed or pure hash keys
	m := make(map[string]interface{})
	key = NilValue()
	for {
		k, val, ok := tbl.Next(key)
		if !ok {
			break
		}
		var keyStr string
		if k.IsString() {
			keyStr = k.Str()
		} else {
			keyStr = k.String()
		}
		m[keyStr] = jsonGScriptToGo(val)
		key = k
	}
	return m
}

// jsonGoToGScript converts a Go value (from json.Decoder with UseNumber) to a GScript Value.
func jsonGoToGScript(v interface{}) Value {
	switch val := v.(type) {
	case nil:
		return NilValue()
	case bool:
		return BoolValue(val)
	case json.Number:
		// Try integer first
		if i, err := val.Int64(); err == nil {
			// Verify it round-trips (no truncation of decimals)
			if strconv.FormatInt(i, 10) == val.String() {
				return IntValue(i)
			}
		}
		// Fall back to float
		if f, err := val.Float64(); err == nil {
			return FloatValue(f)
		}
		return StringValue(val.String())
	case float64:
		// Check if it's a whole number
		if float64(int64(val)) == val && !math.IsInf(val, 0) {
			return IntValue(int64(val))
		}
		return FloatValue(val)
	case string:
		return StringValue(val)
	case []interface{}:
		tbl := NewSequentialArrayTable(len(val))
		for i, item := range val {
			tbl.RawSetInt(int64(i+1), jsonGoToGScript(item))
		}
		return TableValue(tbl)
	case map[string]interface{}:
		tbl := NewTableSized(0, len(val))
		for k, item := range val {
			tbl.RawSetString(k, jsonGoToGScript(item))
		}
		return TableValue(tbl)
	default:
		return StringValue(fmt.Sprintf("%v", val))
	}
}
