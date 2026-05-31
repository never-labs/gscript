package modules

import "fmt"

// BuildArray creates the "array" dense data library.
func BuildArray() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "array." + name, Fn: fn}))
	}
	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "array." + name, Fn: fn, FastArg1: fast}))
	}
	set("f64", func(args []Value) ([]Value, error) {
		a, err := denseArrayFromArgs(DenseArrayF64, args)
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(a)}, nil
	})
	set("i64", func(args []Value) ([]Value, error) {
		a, err := denseArrayFromArgs(DenseArrayI64, args)
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(a)}, nil
	})
	set("bool", func(args []Value) ([]Value, error) {
		a, err := denseArrayFromArgs(DenseArrayBool, args)
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(a)}, nil
	})
	setFastArg1("sum", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsDenseArray() {
			return nil, fmt.Errorf("array.sum: argument 1 must be a dense array")
		}
		v, err := DenseArrayReduce(DenseArrayReduceSum, args[0].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, arraySumValue)
	return t
}

func arraySumValue(arrayValue Value) (Value, error) {
	if !arrayValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("array.sum: argument 1 must be a dense array")
	}
	return DenseArrayReduce(DenseArrayReduceSum, arrayValue.DenseArray())
}

func denseArrayFromArgs(dtype DenseArrayDType, args []Value) (*DenseArray, error) {
	if len(args) == 1 && args[0].IsTable() {
		tbl := args[0].Table()
		n := tbl.Length()
		out, err := NewDenseArrayOfLen(dtype, n)
		if err != nil {
			return nil, err
		}
		for i := 0; i < n; i++ {
			if err := out.Set(i, tbl.RawGetInt(int64(i+1))); err != nil {
				return nil, err
			}
		}
		return out, nil
	}
	out, err := NewDenseArrayOfLen(dtype, len(args))
	if err != nil {
		return nil, err
	}
	for i, arg := range args {
		if err := out.Set(i, arg); err != nil {
			return nil, fmt.Errorf("array.%s argument %d: %w", dtype, i+1, err)
		}
	}
	return out, nil
}
