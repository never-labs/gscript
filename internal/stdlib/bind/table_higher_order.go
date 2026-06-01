package bind

import "fmt"

// BuildTableHigherOrderLibWithCaller installs the callback-based table helpers
// into tblLib. The caller is supplied by the active execution engine.
func BuildTableHigherOrderLibWithCaller(call ScriptFunctionCaller, tblLib *Table) *Table {
	if call == nil || tblLib == nil {
		return tblLib
	}
	// table.filter(t, f) -- return new array of values where f(v, k) is truthy
	tblLib.RawSet(StringValue("filter"), FunctionValue(&GoFunction{
		Name: "table.filter",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.filter'")
			}
			tbl := args[0].Table()
			fn := args[1]
			result := NewTable()
			idx := int64(1)
			length := tbl.Length()
			for i := int64(1); i <= int64(length); i++ {
				v := tbl.RawGet(IntValue(i))
				results, err := call(fn, []Value{v, IntValue(i)})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 && results[0].Truthy() {
					result.RawSet(IntValue(idx), v)
					idx++
				}
			}
			return []Value{TableValue(result)}, nil
		},
	}))

	// table.map(t, f) -- return new array/table with f(v, k) applied to each value
	tblLib.RawSet(StringValue("map"), FunctionValue(&GoFunction{
		Name: "table.map",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.map'")
			}
			tbl := args[0].Table()
			fn := args[1]
			result := NewTable()
			length := tbl.Length()
			for i := int64(1); i <= int64(length); i++ {
				v := tbl.RawGet(IntValue(i))
				results, err := call(fn, []Value{v, IntValue(i)})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					result.RawSet(IntValue(i), results[0])
				} else {
					result.RawSet(IntValue(i), NilValue())
				}
			}
			return []Value{TableValue(result)}, nil
		},
	}))

	// table.reduce(t, f, init) -- fold: acc = f(acc, v) for each value, return acc
	tblLib.RawSet(StringValue("reduce"), FunctionValue(&GoFunction{
		Name: "table.reduce",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 3 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.reduce'")
			}
			tbl := args[0].Table()
			fn := args[1]
			acc := args[2]
			length := tbl.Length()
			for i := int64(1); i <= int64(length); i++ {
				v := tbl.RawGet(IntValue(i))
				results, err := call(fn, []Value{acc, v})
				if err != nil {
					return nil, err
				}
				if len(results) > 0 {
					acc = results[0]
				}
			}
			return []Value{acc}, nil
		},
	}))

	// Also add fromArray with interp support for closures.
	tblLib.RawSet(StringValue("fromArray"), FunctionValue(&GoFunction{
		Name: "table.fromArray",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
				return nil, fmt.Errorf("bad argument to 'table.fromArray'")
			}
			arr := args[0].Table()
			keyFn := args[1]
			result := NewTable()
			length := arr.Length()
			for i := int64(1); i <= int64(length); i++ {
				v := arr.RawGet(IntValue(i))
				keys, err := call(keyFn, []Value{v})
				if err != nil {
					return nil, err
				}
				if len(keys) > 0 {
					result.RawSet(keys[0], v)
				}
			}
			return []Value{TableValue(result)}, nil
		},
	}))
	return tblLib
}
