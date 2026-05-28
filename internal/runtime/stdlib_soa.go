package runtime

import "fmt"

// buildSoALib creates the "soa" data-oriented structure-of-arrays library.
func buildSoALib() *Table {
	t := NewTable()
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn}))
	}
	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg2: fast}))
	}
	setFastArg4 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg4: fast}))
	}
	setFastArg5 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value, Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg5: fast}))
	}

	set("zip", func(args []Value) ([]Value, error) {
		if len(args) < 1 || !args[0].IsTable() {
			return nil, fmt.Errorf("soa.zip: argument 1 must be a table of dense array columns")
		}
		cols := make(map[string]*DenseArray)
		ok := args[0].Table().ForEachPlainRaw(func(key, val Value) bool {
			if !key.IsString() || !val.IsDenseArray() {
				return false
			}
			cols[key.Str()] = val.DenseArray()
			return true
		})
		if !ok {
			return nil, fmt.Errorf("soa.zip: columns must be plain string-keyed dense arrays")
		}
		s, err := NewSoA(cols)
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(s)}, nil
	})

	set("len", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.len", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(s.Len()))}, nil
	})

	set("columns", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.columns", args, 0)
		if err != nil {
			return nil, err
		}
		out := NewTable()
		for i, name := range s.ColumnNames() {
			out.RawSetInt(int64(i+1), StringValue(name))
		}
		return []Value{TableValue(out)}, nil
	})

	set("column", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.column", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.column: argument 2 must be a string")
		}
		col, ok := s.Column(args[1].Str())
		if !ok {
			return []Value{NilValue()}, nil
		}
		return []Value{DenseArrayValue(col)}, nil
	})

	set("row", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.row", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsInt() {
			return nil, fmt.Errorf("soa.row: argument 2 must be an integer")
		}
		row, err := s.Row(int(args[1].Int() - 1))
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(row)}, nil
	})

	set("setRow", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.setRow", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsInt() {
			return nil, fmt.Errorf("soa.setRow: argument 2 must be an integer")
		}
		if len(args) < 3 || !args[2].IsTable() {
			return nil, fmt.Errorf("soa.setRow: argument 3 must be a table")
		}
		if err := s.SetRow(int(args[1].Int()-1), args[2].Table()); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	})

	setFastArg4("addScaled", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.addScaled", args, 0)
		if err != nil {
			return nil, err
		}
		dst, src, scale, err := requireSoAKernelArgs("soa.addScaled", args)
		if err != nil {
			return nil, err
		}
		if err := s.AddScaled(dst, src, scale); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaAddScaledValue)

	setFastArg5("affine", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.affine", args, 0)
		if err != nil {
			return nil, err
		}
		dst, src, scale, err := requireSoAKernelArgs("soa.affine", args)
		if err != nil {
			return nil, err
		}
		if len(args) < 5 || !args[4].IsNumber() {
			return nil, fmt.Errorf("soa.affine: argument 5 must be numeric")
		}
		if err := s.Affine(dst, src, scale, args[4].Number()); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaAffineValue)

	setFastArg2("sum", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.sum", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.sum: argument 2 must be a string")
		}
		v, err := s.Sum(args[1].Str())
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaSumValue)

	return t
}

func requireSoAArg(name string, args []Value, index int) (*SoA, error) {
	if len(args) <= index || !args[index].IsSoA() {
		return nil, fmt.Errorf("%s: argument %d must be soa", name, index+1)
	}
	return args[index].SoA(), nil
}

func requireSoAKernelArgs(name string, args []Value) (dst, src string, scale float64, err error) {
	if len(args) < 2 || !args[1].IsString() {
		return "", "", 0, fmt.Errorf("%s: argument 2 must be a string", name)
	}
	if len(args) < 3 || !args[2].IsString() {
		return "", "", 0, fmt.Errorf("%s: argument 3 must be a string", name)
	}
	if len(args) < 4 || !args[3].IsNumber() {
		return "", "", 0, fmt.Errorf("%s: argument 4 must be numeric", name)
	}
	return args[1].Str(), args[2].Str(), args[3].Number(), nil
}

func soaAddScaledValue(soaValue, dstValue, srcValue, scaleValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.addScaled: argument 1 must be soa")
	}
	if !dstValue.IsString() {
		return NilValue(), fmt.Errorf("soa.addScaled: argument 2 must be a string")
	}
	if !srcValue.IsString() {
		return NilValue(), fmt.Errorf("soa.addScaled: argument 3 must be a string")
	}
	if !scaleValue.IsNumber() {
		return NilValue(), fmt.Errorf("soa.addScaled: argument 4 must be numeric")
	}
	if err := soaValue.SoA().AddScaled(dstValue.Str(), srcValue.Str(), scaleValue.Number()); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaAffineValue(soaValue, dstValue, srcValue, scaleValue, biasValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.affine: argument 1 must be soa")
	}
	if !dstValue.IsString() {
		return NilValue(), fmt.Errorf("soa.affine: argument 2 must be a string")
	}
	if !srcValue.IsString() {
		return NilValue(), fmt.Errorf("soa.affine: argument 3 must be a string")
	}
	if !scaleValue.IsNumber() {
		return NilValue(), fmt.Errorf("soa.affine: argument 4 must be numeric")
	}
	if !biasValue.IsNumber() {
		return NilValue(), fmt.Errorf("soa.affine: argument 5 must be numeric")
	}
	if err := soaValue.SoA().Affine(dstValue.Str(), srcValue.Str(), scaleValue.Number(), biasValue.Number()); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaSumValue(soaValue, columnValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.sum: argument 1 must be soa")
	}
	if !columnValue.IsString() {
		return NilValue(), fmt.Errorf("soa.sum: argument 2 must be a string")
	}
	return soaValue.SoA().Sum(columnValue.Str())
}
