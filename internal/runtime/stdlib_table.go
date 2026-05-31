package runtime

import (
	"fmt"
	"sort"
	"strings"

	tablelib "github.com/never-labs/gscript/internal/stdlib/data/table"
)

type TableSortCaller func(Value, []Value) ([]Value, error)
type TableSortLess func(Value, Value) (bool, error)
type TableSortLen func(Value) (int64, error)
type TableSortGet func(Value, Value) (Value, error)
type TableSortSet func(Value, Value, Value) error
type TableSortTryPlainArraySort func(Value, int64) bool
type TableMoveGet func(Value, Value) (Value, error)
type TableMoveSet func(Value, Value, Value) error
type TableMoveTryPlainArrayMove func(src, dst Value, first, last, target int64) bool
type TableInsertLen func(Value) (int64, error)
type TableInsertGet func(Value, Value) (Value, error)
type TableInsertSet func(Value, Value, Value) error
type TableInsertTryPlainArrayInsert func(Value, int64, Value, int64) bool
type TableRemoveLen func(Value) (int64, error)
type TableRemoveGet func(Value, Value) (Value, error)
type TableRemoveSet func(Value, Value, Value) error
type TableRemoveTryPlainArrayRemove func(Value, int64, int64) (Value, bool)
type TableUnpackLen func(Value) (int64, error)
type TableUnpackGet func(Value, Value) (Value, error)

// BuildTableSortFunction builds table.sort around caller-provided table access
// and comparison hooks. VM/interpreter callers pass metamethod-aware hooks;
// the base runtime library passes raw table hooks.
func BuildTableSortFunction(call TableSortCaller, less TableSortLess, tableLen TableSortLen, tableGet TableSortGet, tableSet TableSortSet, tryPlain TableSortTryPlainArraySort) *GoFunction {
	sortTable := func(t Value, comp Value, hasComp bool) error {
		length, err := tableLen(t)
		if err != nil {
			return err
		}
		if length < 0 {
			length = 0
		}
		if !hasComp && tryPlain != nil && tryPlain(t, length) {
			return nil
		}

		elems := make([]Value, int(length))
		for i := 0; i < len(elems); i++ {
			v, err := tableGet(t, IntValue(int64(i+1)))
			if err != nil {
				return err
			}
			elems[i] = v
		}

		var sortErr error
		if hasComp && comp.IsFunction() {
			sort.SliceStable(elems, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				results, err := call(comp, []Value{elems[a], elems[b]})
				if err != nil {
					sortErr = err
					return false
				}
				if len(results) > 0 && results[0].Truthy() {
					reverse, err := call(comp, []Value{elems[b], elems[a]})
					if err != nil {
						sortErr = err
						return false
					}
					if len(reverse) > 0 && reverse[0].Truthy() {
						sortErr = fmt.Errorf("invalid order function for sorting")
						return false
					}
					return true
				}
				return false
			})
		} else {
			sort.SliceStable(elems, func(a, b int) bool {
				if sortErr != nil {
					return false
				}
				ok, err := less(elems[a], elems[b])
				if err != nil {
					sortErr = err
					return false
				}
				return ok
			})
		}
		if sortErr != nil {
			return sortErr
		}

		for i, v := range elems {
			if err := tableSet(t, IntValue(int64(i+1)), v); err != nil {
				return err
			}
		}
		return nil
	}

	return &GoFunction{
		Name: "table.sort",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			comp := NilValue()
			if len(args) >= 2 {
				comp = args[1]
			}
			if err := sortTable(args[0], comp, len(args) >= 2); err != nil {
				return nil, err
			}
			return nil, nil
		},
		FastArg1: func(t Value) (Value, error) {
			if !t.IsTable() {
				return NilValue(), fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			if err := sortTable(t, NilValue(), false); err != nil {
				return NilValue(), err
			}
			return NilValue(), nil
		},
		FastArg2: func(t, comp Value) (Value, error) {
			if !t.IsTable() {
				return NilValue(), fmt.Errorf("bad argument #1 to 'table.sort' (table expected)")
			}
			if err := sortTable(t, comp, true); err != nil {
				return NilValue(), err
			}
			return NilValue(), nil
		},
	}
}

func rawTableSortFunction() *GoFunction {
	return BuildTableSortFunction(
		func(fn Value, args []Value) ([]Value, error) {
			if gf := fn.GoFunction(); gf != nil {
				return gf.Fn(args)
			}
			return nil, fmt.Errorf("table.sort comparator must be a Go function or use default ordering")
		},
		func(a, b Value) (bool, error) {
			ok, comparable := a.LessThan(b)
			if comparable {
				return ok, nil
			}
			return false, nil
		},
		func(t Value) (int64, error) {
			return int64(t.Table().Length()), nil
		},
		func(t Value, key Value) (Value, error) {
			return t.Table().RawGet(key), nil
		},
		func(t Value, key Value, val Value) error {
			t.Table().RawSet(key, val)
			return nil
		},
		func(t Value, length int64) bool {
			return t.Table().TryPlainArraySort(length)
		},
	)
}

// BuildTableMoveFunction builds table.move around caller-provided table access
// hooks. Raw callers pass RawGet/RawSet; interpreter callers pass
// metamethod-aware hooks.
func BuildTableMoveFunction(tableGet TableMoveGet, tableSet TableMoveSet, tryPlain TableMoveTryPlainArrayMove) *GoFunction {
	return &GoFunction{
		Name: "table.move",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 4 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument to 'table.move'")
			}
			src := args[0]
			f := toInt(args[1])
			e := toInt(args[2])
			tPos := toInt(args[3])
			dst := src
			if len(args) >= 5 {
				if !args[4].IsTable() {
					return nil, fmt.Errorf("bad argument to 'table.move'")
				}
				dst = args[4]
			}

			if e >= f {
				if tryPlain != nil && tryPlain(src, dst, f, e, tPos) {
					return []Value{dst}, nil
				}
				count := e - f + 1
				if tPos <= f || src.Table() != dst.Table() {
					for i := int64(0); i < count; i++ {
						v, err := tableGet(src, IntValue(f+i))
						if err != nil {
							return nil, err
						}
						if err := tableSet(dst, IntValue(tPos+i), v); err != nil {
							return nil, err
						}
					}
				} else {
					for i := count - 1; i >= 0; i-- {
						v, err := tableGet(src, IntValue(f+i))
						if err != nil {
							return nil, err
						}
						if err := tableSet(dst, IntValue(tPos+i), v); err != nil {
							return nil, err
						}
					}
				}
			}
			return []Value{dst}, nil
		},
	}
}

func rawTableMoveFunction() *GoFunction {
	return BuildTableMoveFunction(
		func(t Value, key Value) (Value, error) {
			return t.Table().RawGet(key), nil
		},
		func(t Value, key Value, val Value) error {
			t.Table().RawSet(key, val)
			return nil
		},
		func(src, dst Value, first, last, target int64) bool {
			return dst.Table().TryPlainArrayMove(src.Table(), first, last, target)
		},
	)
}

// BuildTableInsertFunction builds table.insert around caller-provided table
// access hooks. Raw callers pass RawGet/RawSet; interpreter callers pass
// metamethod-aware hooks.
func BuildTableInsertFunction(tableLen TableInsertLen, tableGet TableInsertGet, tableSet TableInsertSet, tryPlain TableInsertTryPlainArrayInsert) *GoFunction {
	return &GoFunction{
		Name: "table.insert",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.insert' (table expected)")
			}
			if len(args) != 2 && len(args) != 3 {
				return nil, fmt.Errorf("wrong number of arguments to 'table.insert'")
			}

			t := args[0]
			length, err := tableLen(t)
			if err != nil {
				return nil, err
			}

			pos := length + 1
			value := args[1]
			if len(args) == 3 {
				pos = toInt(args[1])
				if pos < 1 || pos > length+1 {
					return nil, fmt.Errorf("bad argument #2 to 'table.insert' (position out of bounds)")
				}
				value = args[2]
			}

			if tryPlain != nil && tryPlain(t, pos, value, length) {
				return nil, nil
			}
			for i := length; i >= pos; i-- {
				v, err := tableGet(t, IntValue(i))
				if err != nil {
					return nil, err
				}
				if err := tableSet(t, IntValue(i+1), v); err != nil {
					return nil, err
				}
			}
			return nil, tableSet(t, IntValue(pos), value)
		},
	}
}

func rawTableInsertFunction() *GoFunction {
	return BuildTableInsertFunction(
		func(t Value) (int64, error) {
			return int64(t.Table().Length()), nil
		},
		func(t Value, key Value) (Value, error) {
			return t.Table().RawGet(key), nil
		},
		func(t Value, key Value, val Value) error {
			t.Table().RawSet(key, val)
			return nil
		},
		func(t Value, pos int64, val Value, length int64) bool {
			return t.Table().TryPlainArrayInsertKnownLength(pos, val, length)
		},
	)
}

// BuildTableRemoveFunction builds table.remove around caller-provided table
// access hooks. Raw callers pass RawGet/RawSet; interpreter callers pass
// metamethod-aware hooks.
func BuildTableRemoveFunction(tableLen TableRemoveLen, tableGet TableRemoveGet, tableSet TableRemoveSet, tryPlain TableRemoveTryPlainArrayRemove) *GoFunction {
	return &GoFunction{
		Name: "table.remove",
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.remove' (table expected)")
			}

			t := args[0]
			length, err := tableLen(t)
			if err != nil {
				return nil, err
			}
			pos := length
			if len(args) >= 2 {
				pos = toInt(args[1])
			}
			if pos < 0 || pos > length+1 || (pos == 0 && length > 0) {
				return nil, fmt.Errorf("bad argument #2 to 'table.remove' (position out of bounds)")
			}
			if pos == length+1 {
				return []Value{NilValue()}, nil
			}

			if tryPlain != nil {
				if removed, ok := tryPlain(t, pos, length); ok {
					return []Value{removed}, nil
				}
			}
			removed, err := tableGet(t, IntValue(pos))
			if err != nil {
				return nil, err
			}
			for i := pos; i < length; i++ {
				v, err := tableGet(t, IntValue(i+1))
				if err != nil {
					return nil, err
				}
				if err := tableSet(t, IntValue(i), v); err != nil {
					return nil, err
				}
			}
			if err := tableSet(t, IntValue(length), NilValue()); err != nil {
				return nil, err
			}
			return []Value{removed}, nil
		},
	}
}

func rawTableRemoveFunction() *GoFunction {
	return BuildTableRemoveFunction(
		func(t Value) (int64, error) {
			return int64(t.Table().Length()), nil
		},
		func(t Value, key Value) (Value, error) {
			return t.Table().RawGet(key), nil
		},
		func(t Value, key Value, val Value) error {
			t.Table().RawSet(key, val)
			return nil
		},
		func(t Value, pos int64, length int64) (Value, bool) {
			return t.Table().TryPlainArrayRemoveKnownLength(pos, length)
		},
	)
}

// BuildTableUnpackFunction builds table.unpack/table.spread around
// caller-provided table access hooks. Raw callers pass RawGet/Length;
// interpreter callers pass metamethod-aware hooks.
func BuildTableUnpackFunction(name string, tableLen TableUnpackLen, tableGet TableUnpackGet) *GoFunction {
	return &GoFunction{
		Name: "table." + name,
		Fn: func(args []Value) ([]Value, error) {
			if len(args) < 1 || !args[0].IsTable() {
				return nil, fmt.Errorf("bad argument #1 to 'table.%s' (table expected)", name)
			}
			t := args[0]
			i := int64(1)
			j, err := tableLen(t)
			if err != nil {
				return nil, err
			}
			if len(args) >= 2 {
				i = toInt(args[1])
			}
			if len(args) >= 3 {
				j = toInt(args[2])
			}
			count, err := tablelib.CheckUnpackRange(name, i, j)
			if err != nil {
				return nil, err
			}
			result := make([]Value, 0, count)
			for k := i; k <= j; k++ {
				v, err := tableGet(t, IntValue(k))
				if err != nil {
					return nil, err
				}
				result = append(result, v)
			}
			return result, nil
		},
	}
}

func rawTableUnpackFunction(name string) *GoFunction {
	return BuildTableUnpackFunction(
		name,
		func(t Value) (int64, error) {
			return int64(t.Table().Length()), nil
		},
		func(t Value, key Value) (Value, error) {
			return t.Table().RawGet(key), nil
		},
	)
}

// buildTableLib creates the "table" standard library table.
func buildTableLib() *Table {
	t := NewTable()

	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSet(StringValue(name), FunctionValue(&GoFunction{
			Name: "table." + name,
			Fn:   fn,
		}))
	}

	t.RawSet(StringValue("insert"), FunctionValue(rawTableInsertFunction()))
	t.RawSet(StringValue("remove"), FunctionValue(rawTableRemoveFunction()))

	// table.concat(t [, sep [, i [, j]]]) -> string
	set("concat", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.concat' (table expected)")
		}
		tbl := args[0].Table()
		sep := ""
		if len(args) >= 2 && args[1].IsString() {
			sep = args[1].Str()
		}
		i := int64(1)
		j := int64(tbl.Length())
		if len(args) >= 3 {
			i = toInt(args[2])
		}
		if len(args) >= 4 {
			j = toInt(args[3])
		}

		var b strings.Builder
		for k := i; k <= j; k++ {
			v := tbl.RawGet(IntValue(k))
			s, ok := ConcatOperandString(v)
			if !ok {
				return nil, fmt.Errorf("invalid value at index %d in table for 'concat'", k)
			}
			if k > i {
				b.WriteString(sep)
			}
			b.WriteString(s)
		}
		return []Value{StringValue(b.String())}, nil
	})

	t.RawSet(StringValue("sort"), FunctionValue(rawTableSortFunction()))

	t.RawSet(StringValue("unpack"), FunctionValue(rawTableUnpackFunction("unpack")))
	t.RawSet(StringValue("spread"), FunctionValue(rawTableUnpackFunction("spread")))

	t.RawSet(StringValue("move"), FunctionValue(rawTableMoveFunction()))

	// table.pack(...) -> table
	set("pack", func(args []Value) ([]Value, error) {
		tbl := NewTable()
		for i, v := range args {
			tbl.RawSet(IntValue(int64(i+1)), v)
		}
		tbl.RawSet(StringValue("n"), IntValue(int64(len(args))))
		return []Value{TableValue(tbl)}, nil
	})

	// table.keys(t) -- return array table of all keys (any type)
	set("keys", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.keys' (table expected)")
		}
		tbl := args[0].Table()
		result := NewTable()
		idx := int64(1)
		k, _, ok := tbl.Next(NilValue())
		for ok {
			result.RawSet(IntValue(idx), k)
			idx++
			k, _, ok = tbl.Next(k)
		}
		return []Value{TableValue(result)}, nil
	})

	// table.values(t) -- return array table of all values
	set("values", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.values' (table expected)")
		}
		tbl := args[0].Table()
		result := NewTable()
		idx := int64(1)
		k, v, ok := tbl.Next(NilValue())
		for ok {
			result.RawSet(IntValue(idx), v)
			idx++
			k, v, ok = tbl.Next(k)
		}
		return []Value{TableValue(result)}, nil
	})

	// table.contains(t, v) -- bool: linear search for value v
	set("contains", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.contains' (table expected)")
		}
		tbl := args[0].Table()
		target := args[1]
		k, v, ok := tbl.Next(NilValue())
		for ok {
			if v.Equal(target) {
				return []Value{BoolValue(true)}, nil
			}
			k, v, ok = tbl.Next(k)
		}
		return []Value{BoolValue(false)}, nil
	})

	// table.indexOf(t, v) -- int key of first occurrence of v, or nil
	set("indexOf", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.indexOf' (table expected)")
		}
		tbl := args[0].Table()
		target := args[1]
		length := tbl.Length()
		for i := int64(1); i <= int64(length); i++ {
			if tbl.RawGet(IntValue(i)).Equal(target) {
				return []Value{IntValue(i)}, nil
			}
		}
		return []Value{NilValue()}, nil
	})

	// table.copy(t) -- shallow copy (new table with same key-value pairs)
	set("copy", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.copy' (table expected)")
		}
		src := args[0].Table()
		dst := NewTable()
		k, v, ok := src.Next(NilValue())
		for ok {
			dst.RawSet(k, v)
			k, v, ok = src.Next(k)
		}
		return []Value{TableValue(dst)}, nil
	})

	// table.merge(t1, t2) -- copy all entries from t2 into t1 (in-place), return t1
	set("merge", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'table.merge' (table expected)")
		}
		t1 := args[0].Table()
		t2 := args[1].Table()
		k, v, ok := t2.Next(NilValue())
		for ok {
			t1.RawSet(k, v)
			k, v, ok = t2.Next(k)
		}
		return []Value{TableValue(t1)}, nil
	})

	// table.count(t) -- count ALL entries including non-integer keys (unlike #t)
	set("count", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.count' (table expected)")
		}
		tbl := args[0].Table()
		count := int64(0)
		k, _, ok := tbl.Next(NilValue())
		for ok {
			count++
			k, _, ok = tbl.Next(k)
		}
		return []Value{IntValue(count)}, nil
	})

	// table.toArray(t) -- convert hash-table to array by taking values in pairs order
	set("toArray", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.toArray' (table expected)")
		}
		src := args[0].Table()
		result := NewTable()
		idx := int64(1)
		k, v, ok := src.Next(NilValue())
		for ok {
			result.RawSet(IntValue(idx), v)
			idx++
			k, v, ok = src.Next(k)
		}
		return []Value{TableValue(result)}, nil
	})

	// table.fromArray(arr, keyFn) -- convert array to table using keyFn(v) as key
	// Note: keyFn must be a GoFunction (no interp needed)
	set("fromArray", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsFunction() {
			return nil, fmt.Errorf("bad argument to 'table.fromArray'")
		}
		arr := args[0].Table()
		keyFn := args[1]
		gf := keyFn.GoFunction()
		if gf == nil {
			return nil, fmt.Errorf("table.fromArray: keyFn must be a Go function (use table library with interp for closures)")
		}
		result := NewTable()
		length := arr.Length()
		for i := int64(1); i <= int64(length); i++ {
			v := arr.RawGet(IntValue(i))
			keys, err := gf.Fn([]Value{v})
			if err != nil {
				return nil, err
			}
			if len(keys) > 0 {
				result.RawSet(keys[0], v)
			}
		}
		return []Value{TableValue(result)}, nil
	})

	// table.unique(t) -- remove duplicate values from array, return new array
	set("unique", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.unique' (table expected)")
		}
		tbl := args[0].Table()
		result := NewTable()
		seen := make([]Value, 0)
		length := tbl.Length()
		idx := int64(1)
		for i := int64(1); i <= int64(length); i++ {
			v := tbl.RawGet(IntValue(i))
			duplicate := false
			for _, prior := range seen {
				if prior.Equal(v) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				seen = append(seen, v)
				result.RawSet(IntValue(idx), v)
				idx++
			}
		}
		return []Value{TableValue(result)}, nil
	})

	// table.flatten(t [, depth]) -- flatten nested arrays to depth levels (default: all)
	set("flatten", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.flatten' (table expected)")
		}
		maxDepth := -1 // -1 means unlimited
		if len(args) >= 2 {
			maxDepth = int(toInt(args[1]))
		}
		result := NewTable()
		idx := int64(1)
		var flattenHelper func(tbl *Table, depth int)
		flattenHelper = func(tbl *Table, depth int) {
			length := tbl.Length()
			for i := int64(1); i <= int64(length); i++ {
				v := tbl.RawGet(IntValue(i))
				if v.IsTable() && (maxDepth < 0 || depth < maxDepth) {
					flattenHelper(v.Table(), depth+1)
				} else {
					result.RawSet(IntValue(idx), v)
					idx++
				}
			}
		}
		flattenHelper(args[0].Table(), 0)
		return []Value{TableValue(result)}, nil
	})

	// table.zip(t1, t2) -- zip two arrays: {{t1[1],t2[1]}, {t1[2],t2[2]}, ...}
	set("zip", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() || !args[1].IsTable() {
			return nil, fmt.Errorf("bad argument to 'table.zip' (table expected)")
		}
		t1 := args[0].Table()
		t2 := args[1].Table()
		len1 := t1.Length()
		len2 := t2.Length()
		minLen := len1
		if len2 < minLen {
			minLen = len2
		}
		result := NewTable()
		for i := int64(1); i <= int64(minLen); i++ {
			pair := NewTable()
			pair.RawSet(IntValue(1), t1.RawGet(IntValue(i)))
			pair.RawSet(IntValue(2), t2.RawGet(IntValue(i)))
			result.RawSet(IntValue(i), TableValue(pair))
		}
		return []Value{TableValue(result)}, nil
	})

	// table.reverse(t) -- reverse array in place, return t
	set("reverse", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument #1 to 'table.reverse' (table expected)")
		}
		tbl := args[0].Table()
		length := tbl.Length()
		for i, j := int64(1), int64(length); i < j; i, j = i+1, j-1 {
			vi := tbl.RawGet(IntValue(i))
			vj := tbl.RawGet(IntValue(j))
			tbl.RawSet(IntValue(i), vj)
			tbl.RawSet(IntValue(j), vi)
		}
		return []Value{TableValue(tbl)}, nil
	})

	// table.slice(t, from [, to]) -- return new array (1-based, from..to inclusive)
	set("slice", func(args []Value) ([]Value, error) {
		if len(args) < 2 || !args[0].IsTable() {
			return nil, fmt.Errorf("bad argument to 'table.slice'")
		}
		tbl := args[0].Table()
		from := toInt(args[1])
		to := int64(tbl.Length())
		if len(args) >= 3 {
			to = toInt(args[2])
		}
		result := NewTable()
		idx := int64(1)
		for i := from; i <= to; i++ {
			result.RawSet(IntValue(idx), tbl.RawGet(IntValue(i)))
			idx++
		}
		return []Value{TableValue(result)}, nil
	})

	return t
}

// buildTableSortWithInterp creates a table.sort that can call closure comparators.
// This is registered separately because it needs access to the interpreter.
func buildTableSortWithInterp(interp *Interpreter, tblLib *Table) {
	tblLib.RawSet(StringValue("sort"), FunctionValue(BuildTableSortFunction(
		interp.callFunction,
		interp.valLessThan,
		interp.tableLenInt,
		interp.tableGet,
		interp.tableSet,
		func(t Value, length int64) bool {
			if tbl := t.Table(); tbl != nil && tbl.TryPlainArraySort(length) {
				return true
			}
			return false
		},
	)))
}

func buildTableProxyWithInterp(interp *Interpreter, tblLib *Table) {
	tblLib.RawSet(StringValue("insert"), FunctionValue(BuildTableInsertFunction(
		interp.tableLenInt,
		interp.tableGet,
		interp.tableSet,
		func(t Value, pos int64, val Value, length int64) bool {
			if tbl := t.Table(); tbl != nil && tbl.TryPlainArrayInsertKnownLength(pos, val, length) {
				return true
			}
			return false
		},
	)))

	tblLib.RawSet(StringValue("remove"), FunctionValue(BuildTableRemoveFunction(
		interp.tableLenInt,
		interp.tableGet,
		interp.tableSet,
		func(t Value, pos int64, length int64) (Value, bool) {
			if tbl := t.Table(); tbl != nil {
				return tbl.TryPlainArrayRemoveKnownLength(pos, length)
			}
			return NilValue(), false
		},
	)))

	tblLib.RawSet(StringValue("unpack"), FunctionValue(BuildTableUnpackFunction("unpack", interp.tableLenInt, interp.tableGet)))
	tblLib.RawSet(StringValue("spread"), FunctionValue(BuildTableUnpackFunction("spread", interp.tableLenInt, interp.tableGet)))

	tblLib.RawSet(StringValue("move"), FunctionValue(BuildTableMoveFunction(
		func(t Value, key Value) (Value, error) {
			return interp.tableGet(t, key)
		},
		func(t Value, key Value, val Value) error {
			return interp.tableSet(t, key, val)
		},
		func(src, dst Value, first, last, target int64) bool {
			return dst.Table().TryPlainArrayMove(src.Table(), first, last, target)
		},
	)))
}

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

	// Also add fromArray with interp support for closures
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

// buildTableHigherOrderWithInterp adds filter, map, reduce to the table library.
// These need the interpreter to call GScript closures.
func buildTableHigherOrderWithInterp(interp *Interpreter, tblLib *Table) {
	BuildTableHigherOrderLibWithCaller(interp.callFunction, tblLib)
}
