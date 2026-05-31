package runtime

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
)

// JSONValueToGo converts a GScript Value to a Go value suitable for json.Marshal.
// Mixed tables, sparse arrays, and hash tables are encoded as JSON objects.
func JSONValueToGo(v Value) any {
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
		return JSONTableToGo(v.Table())
	default:
		return v.String()
	}
}

// JSONTableToGo converts a GScript table to either a JSON array or object.
func JSONTableToGo(tbl *Table) any {
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
			arr[i-1] = JSONValueToGo(tbl.RawGet(IntValue(int64(i))))
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
			m[k.Str()] = JSONValueToGo(val)
		} else {
			m[k.String()] = JSONValueToGo(val)
		}
		key = k
	}
	return m
}

// JSONGoToValue converts a Go value decoded by json.Decoder into a GScript Value.
func JSONGoToValue(v any) Value {
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
			tbl.RawSetInt(int64(i+1), JSONGoToValue(item))
		}
		return TableValue(tbl)
	case map[string]any:
		tbl := NewTableSized(0, len(val))
		for k, item := range val {
			tbl.RawSetString(k, JSONGoToValue(item))
		}
		return TableValue(tbl)
	default:
		return StringValue(fmt.Sprintf("%v", val))
	}
}
