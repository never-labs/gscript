package runtime

import "fmt"

// buildSoALib creates the "soa" data-oriented structure-of-arrays library.
func buildSoALib() *Table {
	t := NewTable()
	affineManyTerms := &soaAffineManyTermCache{}
	set := func(name string, fn func([]Value) ([]Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn}))
	}
	setFastArg1 := func(name string, fn func([]Value) ([]Value, error), fast func(Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg1: fast}))
	}
	setFastArg2 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg2: fast}))
	}
	setFastArg3 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg3: fast}))
	}
	setFastArg4 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg4: fast}))
	}
	setFastArg5 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value, Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg5: fast}))
	}
	setFastArg6 := func(name string, fn func([]Value) ([]Value, error), fast func(Value, Value, Value, Value, Value, Value) (Value, error)) {
		t.RawSetString(name, FunctionValue(&GoFunction{Name: "soa." + name, Fn: fn, FastArg6: fast}))
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

	setFastArg1("len", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.len", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{IntValue(int64(s.Len()))}, nil
	}, soaLenValue)

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

	setFastArg1("unzip", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.unzip", args, 0)
		if err != nil {
			return nil, err
		}
		cols, err := s.Unzip()
		if err != nil {
			return nil, err
		}
		out := NewTable()
		for _, name := range s.ColumnNames() {
			out.RawSetString(name, DenseArrayValue(cols[name]))
		}
		return []Value{TableValue(out)}, nil
	}, soaUnzipValue)

	set("shape", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.shape", args, 0)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(soaShapeTable(s))}, nil
	})

	setFastArg2("column", func(args []Value) ([]Value, error) {
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
	}, soaColumnValue)

	setFastArg3("withColumn", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.withColumn", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.withColumn: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsDenseArray() {
			return nil, fmt.Errorf("soa.withColumn: argument 3 must be a dense array")
		}
		out, err := s.WithColumn(args[1].Str(), args[2].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(out)}, nil
	}, soaWithColumnValue)

	setFastArg2("dropColumn", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.dropColumn", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.dropColumn: argument 2 must be a string")
		}
		out, err := s.DropColumn(args[1].Str())
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(out)}, nil
	}, soaDropColumnValue)

	setFastArg2("resize", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.resize", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsInt() {
			return nil, fmt.Errorf("soa.resize: argument 2 must be an integer")
		}
		if err := s.Resize(int(args[1].Int())); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaResizeValue)

	setFastArg2("appendRow", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.appendRow", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("soa.appendRow: argument 2 must be a table")
		}
		if err := s.AppendRow(args[1].Table()); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaAppendRowValue)

	setFastArg3("fill", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.fill", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.fill: argument 2 must be a string")
		}
		if len(args) < 3 {
			return nil, fmt.Errorf("soa.fill: argument 3 is required")
		}
		if err := s.Fill(args[1].Str(), args[2]); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaFillValue)

	setFastArg4("fillWhere", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.fillWhere", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.fillWhere: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsDenseArray() {
			return nil, fmt.Errorf("soa.fillWhere: argument 3 must be a bool dense array")
		}
		if len(args) < 4 {
			return nil, fmt.Errorf("soa.fillWhere: argument 4 is required")
		}
		if err := s.FillWhere(args[1].Str(), args[2].DenseArray(), args[3]); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaFillWhereValue)

	setFastArg4("select", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.select", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsDenseArray() {
			return nil, fmt.Errorf("soa.select: argument 2 must be a bool dense array")
		}
		if len(args) < 4 {
			return nil, fmt.Errorf("soa.select: arguments 3 and 4 are required")
		}
		out, err := s.Select(args[1].DenseArray(), args[2], args[3])
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(out)}, nil
	}, soaSelectValue)

	setFastArg5("selectInto", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.selectInto", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.selectInto: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsDenseArray() {
			return nil, fmt.Errorf("soa.selectInto: argument 3 must be a bool dense array")
		}
		if len(args) < 5 {
			return nil, fmt.Errorf("soa.selectInto: arguments 4 and 5 are required")
		}
		if err := s.SelectInto(args[1].Str(), args[2].DenseArray(), args[3], args[4]); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaSelectIntoValue)

	setFastArg4("sumSelect", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.sumSelect", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsDenseArray() {
			return nil, fmt.Errorf("soa.sumSelect: argument 2 must be a bool dense array")
		}
		if len(args) < 4 {
			return nil, fmt.Errorf("soa.sumSelect: arguments 3 and 4 are required")
		}
		out, err := s.SumSelect(args[1].DenseArray(), args[2], args[3])
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	}, soaSumSelectValue)

	setFastArg3("slice", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.slice", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsInt() {
			return nil, fmt.Errorf("soa.slice: argument 2 must be an integer")
		}
		if len(args) < 3 || !args[2].IsInt() {
			return nil, fmt.Errorf("soa.slice: argument 3 must be an integer")
		}
		out, err := s.Slice(int(args[1].Int()-1), int(args[2].Int()))
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(out)}, nil
	}, soaSliceValue)

	setFastArg2("filter", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.filter", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsDenseArray() {
			return nil, fmt.Errorf("soa.filter: argument 2 must be a bool dense array")
		}
		out, err := s.Filter(args[1].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(out)}, nil
	}, soaFilterValue)

	setFastArg2("compact", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.compact", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsDenseArray() {
			return nil, fmt.Errorf("soa.compact: argument 2 must be a bool dense array")
		}
		out, err := s.Compact(args[1].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(out)}, nil
	}, soaCompactValue)

	setFastArg2("gather", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.gather", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsDenseArray() {
			return nil, fmt.Errorf("soa.gather: argument 2 must be an i64 dense array")
		}
		out, err := s.Gather(args[1].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{SoAValue(out)}, nil
	}, soaGatherValue)

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

	setFastArg6("affineWhere", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.affineWhere", args, 0)
		if err != nil {
			return nil, err
		}
		dst, src, scale, err := requireSoAKernelArgs("soa.affineWhere", args)
		if err != nil {
			return nil, err
		}
		if len(args) < 5 || !args[4].IsDenseArray() {
			return nil, fmt.Errorf("soa.affineWhere: argument 5 must be a bool dense array")
		}
		if len(args) < 6 || !args[5].IsNumber() {
			return nil, fmt.Errorf("soa.affineWhere: argument 6 must be numeric")
		}
		if err := s.AffineWhere(dst, src, args[4].DenseArray(), scale, args[5].Number()); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaAffineWhereValue)

	setFastArg2("affineMany", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.affineMany", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsTable() {
			return nil, fmt.Errorf("soa.affineMany: argument 2 must be a table of affine terms")
		}
		if err := affineManyTerms.apply(s, args[1].Table()); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, func(soaValue, termsValue Value) (Value, error) {
		if !soaValue.IsSoA() {
			return NilValue(), fmt.Errorf("soa.affineMany: argument 1 must be soa")
		}
		if !termsValue.IsTable() {
			return NilValue(), fmt.Errorf("soa.affineMany: argument 2 must be a table of affine terms")
		}
		if err := affineManyTerms.apply(soaValue.SoA(), termsValue.Table()); err != nil {
			return NilValue(), err
		}
		return BoolValue(true), nil
	})
	if gf := t.RawGetString("affineMany").GoFunction(); gf != nil {
		gf.NativeKind = NativeKindStdSoAAffineMany
		gf.NativeData = StdSoAAffineManyIdentityPtr()
	}

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

	setFastArg3("sumWhere", func(args []Value) ([]Value, error) {
		v, err := soaMaskedAggregateArgs("soa.sumWhere", args, (*SoA).SumWhere)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaSumWhereValue)

	setFastArg3("minWhere", func(args []Value) ([]Value, error) {
		v, err := soaMaskedAggregateArgs("soa.minWhere", args, (*SoA).MinWhere)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaMinWhereValue)

	setFastArg3("meanWhere", func(args []Value) ([]Value, error) {
		v, err := soaMaskedAggregateArgs("soa.meanWhere", args, (*SoA).MeanWhere)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaMeanWhereValue)

	setFastArg3("maxWhere", func(args []Value) ([]Value, error) {
		v, err := soaMaskedAggregateArgs("soa.maxWhere", args, (*SoA).MaxWhere)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaMaxWhereValue)

	setFastArg3("statsWhere", func(args []Value) ([]Value, error) {
		t, err := soaMaskedStatsArgs("soa.statsWhere", args)
		if err != nil {
			return nil, err
		}
		return []Value{TableValue(t)}, nil
	}, soaStatsWhereValue)

	setFastArg2("countWhere", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.countWhere", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsDenseArray() {
			return nil, fmt.Errorf("soa.countWhere: argument 2 must be a bool dense array")
		}
		v, err := s.CountWhere(args[1].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaCountWhereValue)

	setFastArg4("mask", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.mask", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.mask: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsString() {
			return nil, fmt.Errorf("soa.mask: argument 3 must be a comparison string")
		}
		if len(args) < 4 {
			return nil, fmt.Errorf("soa.mask: argument 4 is required")
		}
		out, err := s.Mask(args[1].Str(), args[2].Str(), args[3])
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(out)}, nil
	}, soaMaskValue)

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

func soaMaskedAggregateArgs(name string, args []Value, fn func(*SoA, string, *DenseArray) (Value, error)) (Value, error) {
	s, err := requireSoAArg(name, args, 0)
	if err != nil {
		return NilValue(), err
	}
	if len(args) < 2 || !args[1].IsString() {
		return NilValue(), fmt.Errorf("%s: argument 2 must be a string", name)
	}
	if len(args) < 3 || !args[2].IsDenseArray() {
		return NilValue(), fmt.Errorf("%s: argument 3 must be a bool dense array", name)
	}
	return fn(s, args[1].Str(), args[2].DenseArray())
}

func soaMaskedStatsArgs(name string, args []Value) (*Table, error) {
	s, err := requireSoAArg(name, args, 0)
	if err != nil {
		return nil, err
	}
	if len(args) < 2 || !args[1].IsString() {
		return nil, fmt.Errorf("%s: argument 2 must be a string", name)
	}
	if len(args) < 3 || !args[2].IsDenseArray() {
		return nil, fmt.Errorf("%s: argument 3 must be a bool dense array", name)
	}
	return s.StatsWhere(args[1].Str(), args[2].DenseArray())
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

func soaAffineWhereValue(soaValue, dstValue, srcValue, scaleValue, maskValue, biasValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.affineWhere: argument 1 must be soa")
	}
	if !dstValue.IsString() {
		return NilValue(), fmt.Errorf("soa.affineWhere: argument 2 must be a string")
	}
	if !srcValue.IsString() {
		return NilValue(), fmt.Errorf("soa.affineWhere: argument 3 must be a string")
	}
	if !scaleValue.IsNumber() {
		return NilValue(), fmt.Errorf("soa.affineWhere: argument 4 must be numeric")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.affineWhere: argument 5 must be a bool dense array")
	}
	if !biasValue.IsNumber() {
		return NilValue(), fmt.Errorf("soa.affineWhere: argument 6 must be numeric")
	}
	if err := soaValue.SoA().AffineWhere(dstValue.Str(), srcValue.Str(), maskValue.DenseArray(), scaleValue.Number(), biasValue.Number()); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaAffineManyValue(soaValue, termsValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.affineMany: argument 1 must be soa")
	}
	if !termsValue.IsTable() {
		return NilValue(), fmt.Errorf("soa.affineMany: argument 2 must be a table of affine terms")
	}
	terms, err := soaAffineTermsFromTable(termsValue.Table())
	if err != nil {
		return NilValue(), err
	}
	if err := soaValue.SoA().AffineMany(terms); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

type soaAffineManyTermCache struct {
	table        *Table
	version      uint64
	termTables   []*Table
	termVersions []uint64
	parsed       []SoAAffineTerm
	planSoA      *SoA
	planGuard    SoAShapeSnapshot
	planWrites   []string
	plans        []soaAffinePlan
}

func (c *soaAffineManyTermCache) terms(tbl *Table) ([]SoAAffineTerm, error) {
	if tbl == nil {
		return nil, fmt.Errorf("soa.affineMany: argument 2 must be a table of affine terms")
	}
	version := tbl.MutationVersion()
	if c.table == tbl && c.version == version && len(c.parsed) > 0 && c.termVersionsMatch() {
		return c.parsed, nil
	}
	terms, termTables, termVersions, err := soaAffineTermsFromTableWithVersions(tbl)
	if err != nil {
		return nil, err
	}
	c.table = tbl
	c.version = version
	c.termTables = termTables
	c.termVersions = termVersions
	c.parsed = terms
	c.clearPlan()
	return terms, nil
}

func (c *soaAffineManyTermCache) termVersionsMatch() bool {
	if len(c.termTables) != len(c.termVersions) {
		return false
	}
	for i, tbl := range c.termTables {
		if tbl == nil || tbl.MutationVersion() != c.termVersions[i] {
			return false
		}
	}
	return true
}

func (c *soaAffineManyTermCache) apply(s *SoA, tbl *Table) error {
	terms, err := c.terms(tbl)
	if err != nil {
		return err
	}
	if c.planSoA == s && len(c.plans) > 0 && s.ValidateSnapshotForWrites(c.planGuard, c.planWrites...) {
		return applySoAAffinePlans(c.plans)
	}
	plans, writes, guard, err := s.affineManyPlan(terms)
	if err != nil {
		return err
	}
	c.planSoA = s
	c.planGuard = guard
	c.planWrites = writes
	c.plans = plans
	return applySoAAffinePlans(plans)
}

func (c *soaAffineManyTermCache) clearPlan() {
	c.planSoA = nil
	c.planGuard = SoAShapeSnapshot{}
	c.planWrites = nil
	c.plans = nil
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

func soaLenValue(soaValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.len: argument 1 must be soa")
	}
	return IntValue(int64(soaValue.SoA().Len())), nil
}

func soaColumnValue(soaValue, nameValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.column: argument 1 must be soa")
	}
	if !nameValue.IsString() {
		return NilValue(), fmt.Errorf("soa.column: argument 2 must be a string")
	}
	col, ok := soaValue.SoA().Column(nameValue.Str())
	if !ok {
		return NilValue(), nil
	}
	return DenseArrayValue(col), nil
}

func soaWithColumnValue(soaValue, nameValue, columnValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.withColumn: argument 1 must be soa")
	}
	if !nameValue.IsString() {
		return NilValue(), fmt.Errorf("soa.withColumn: argument 2 must be a string")
	}
	if !columnValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.withColumn: argument 3 must be a dense array")
	}
	out, err := soaValue.SoA().WithColumn(nameValue.Str(), columnValue.DenseArray())
	if err != nil {
		return NilValue(), err
	}
	return SoAValue(out), nil
}

func soaDropColumnValue(soaValue, nameValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.dropColumn: argument 1 must be soa")
	}
	if !nameValue.IsString() {
		return NilValue(), fmt.Errorf("soa.dropColumn: argument 2 must be a string")
	}
	out, err := soaValue.SoA().DropColumn(nameValue.Str())
	if err != nil {
		return NilValue(), err
	}
	return SoAValue(out), nil
}

func soaResizeValue(soaValue, lengthValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.resize: argument 1 must be soa")
	}
	if !lengthValue.IsInt() {
		return NilValue(), fmt.Errorf("soa.resize: argument 2 must be an integer")
	}
	if err := soaValue.SoA().Resize(int(lengthValue.Int())); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaAppendRowValue(soaValue, rowValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.appendRow: argument 1 must be soa")
	}
	if !rowValue.IsTable() {
		return NilValue(), fmt.Errorf("soa.appendRow: argument 2 must be a table")
	}
	if err := soaValue.SoA().AppendRow(rowValue.Table()); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaFillValue(soaValue, columnValue, fillValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.fill: argument 1 must be soa")
	}
	if !columnValue.IsString() {
		return NilValue(), fmt.Errorf("soa.fill: argument 2 must be a string")
	}
	if err := soaValue.SoA().Fill(columnValue.Str(), fillValue); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaFillWhereValue(soaValue, columnValue, maskValue, fillValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.fillWhere: argument 1 must be soa")
	}
	if !columnValue.IsString() {
		return NilValue(), fmt.Errorf("soa.fillWhere: argument 2 must be a string")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.fillWhere: argument 3 must be a bool dense array")
	}
	if err := soaValue.SoA().FillWhere(columnValue.Str(), maskValue.DenseArray(), fillValue); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaSelectValue(soaValue, maskValue, trueValue, falseValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.select: argument 1 must be soa")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.select: argument 2 must be a bool dense array")
	}
	out, err := soaValue.SoA().Select(maskValue.DenseArray(), trueValue, falseValue)
	if err != nil {
		return NilValue(), err
	}
	return DenseArrayValue(out), nil
}

func soaSelectIntoValue(soaValue, dstValue, maskValue, trueValue, falseValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.selectInto: argument 1 must be soa")
	}
	if !dstValue.IsString() {
		return NilValue(), fmt.Errorf("soa.selectInto: argument 2 must be a string")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.selectInto: argument 3 must be a bool dense array")
	}
	if err := soaValue.SoA().SelectInto(dstValue.Str(), maskValue.DenseArray(), trueValue, falseValue); err != nil {
		return NilValue(), err
	}
	return BoolValue(true), nil
}

func soaSumSelectValue(soaValue, maskValue, trueValue, falseValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.sumSelect: argument 1 must be soa")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.sumSelect: argument 2 must be a bool dense array")
	}
	return soaValue.SoA().SumSelect(maskValue.DenseArray(), trueValue, falseValue)
}

func soaUnzipValue(soaValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.unzip: argument 1 must be soa")
	}
	cols, err := soaValue.SoA().Unzip()
	if err != nil {
		return NilValue(), err
	}
	out := NewTable()
	for _, name := range soaValue.SoA().ColumnNames() {
		out.RawSetString(name, DenseArrayValue(cols[name]))
	}
	return TableValue(out), nil
}

func soaSliceValue(soaValue, firstValue, lastValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.slice: argument 1 must be soa")
	}
	if !firstValue.IsInt() {
		return NilValue(), fmt.Errorf("soa.slice: argument 2 must be an integer")
	}
	if !lastValue.IsInt() {
		return NilValue(), fmt.Errorf("soa.slice: argument 3 must be an integer")
	}
	out, err := soaValue.SoA().Slice(int(firstValue.Int()-1), int(lastValue.Int()))
	if err != nil {
		return NilValue(), err
	}
	return SoAValue(out), nil
}

func soaFilterValue(soaValue, maskValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.filter: argument 1 must be soa")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.filter: argument 2 must be a bool dense array")
	}
	out, err := soaValue.SoA().Filter(maskValue.DenseArray())
	if err != nil {
		return NilValue(), err
	}
	return SoAValue(out), nil
}

func soaCompactValue(soaValue, maskValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.compact: argument 1 must be soa")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.compact: argument 2 must be a bool dense array")
	}
	out, err := soaValue.SoA().Compact(maskValue.DenseArray())
	if err != nil {
		return NilValue(), err
	}
	return SoAValue(out), nil
}

func soaGatherValue(soaValue, indicesValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.gather: argument 1 must be soa")
	}
	if !indicesValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.gather: argument 2 must be an i64 dense array")
	}
	out, err := soaValue.SoA().Gather(indicesValue.DenseArray())
	if err != nil {
		return NilValue(), err
	}
	return SoAValue(out), nil
}

func soaSumWhereValue(soaValue, columnValue, maskValue Value) (Value, error) {
	return soaMaskedAggregateValue("soa.sumWhere", soaValue, columnValue, maskValue, (*SoA).SumWhere)
}

func soaMinWhereValue(soaValue, columnValue, maskValue Value) (Value, error) {
	return soaMaskedAggregateValue("soa.minWhere", soaValue, columnValue, maskValue, (*SoA).MinWhere)
}

func soaMeanWhereValue(soaValue, columnValue, maskValue Value) (Value, error) {
	return soaMaskedAggregateValue("soa.meanWhere", soaValue, columnValue, maskValue, (*SoA).MeanWhere)
}

func soaMaxWhereValue(soaValue, columnValue, maskValue Value) (Value, error) {
	return soaMaskedAggregateValue("soa.maxWhere", soaValue, columnValue, maskValue, (*SoA).MaxWhere)
}

func soaStatsWhereValue(soaValue, columnValue, maskValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.statsWhere: argument 1 must be soa")
	}
	if !columnValue.IsString() {
		return NilValue(), fmt.Errorf("soa.statsWhere: argument 2 must be a string")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.statsWhere: argument 3 must be a bool dense array")
	}
	t, err := soaValue.SoA().StatsWhere(columnValue.Str(), maskValue.DenseArray())
	if err != nil {
		return NilValue(), err
	}
	return TableValue(t), nil
}

func soaMaskedAggregateValue(name string, soaValue, columnValue, maskValue Value, fn func(*SoA, string, *DenseArray) (Value, error)) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("%s: argument 1 must be soa", name)
	}
	if !columnValue.IsString() {
		return NilValue(), fmt.Errorf("%s: argument 2 must be a string", name)
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("%s: argument 3 must be a bool dense array", name)
	}
	return fn(soaValue.SoA(), columnValue.Str(), maskValue.DenseArray())
}

func soaCountWhereValue(soaValue, maskValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.countWhere: argument 1 must be soa")
	}
	if !maskValue.IsDenseArray() {
		return NilValue(), fmt.Errorf("soa.countWhere: argument 2 must be a bool dense array")
	}
	return soaValue.SoA().CountWhere(maskValue.DenseArray())
}

func soaMaskValue(soaValue, columnValue, opValue, rhsValue Value) (Value, error) {
	if !soaValue.IsSoA() {
		return NilValue(), fmt.Errorf("soa.mask: argument 1 must be soa")
	}
	if !columnValue.IsString() {
		return NilValue(), fmt.Errorf("soa.mask: argument 2 must be a string")
	}
	if !opValue.IsString() {
		return NilValue(), fmt.Errorf("soa.mask: argument 3 must be a comparison string")
	}
	out, err := soaValue.SoA().Mask(columnValue.Str(), opValue.Str(), rhsValue)
	if err != nil {
		return NilValue(), err
	}
	return DenseArrayValue(out), nil
}

func soaShapeTable(s *SoA) *Table {
	snapshot, _ := s.Snapshot()
	out := NewTable()
	out.RawSetString("length", IntValue(int64(snapshot.Length)))
	out.RawSetString("version", IntValue(int64(snapshot.ShapeVersion)))
	cols := NewTable()
	for i, desc := range snapshot.Columns {
		col := NewTable()
		col.RawSetString("name", StringValue(desc.Name))
		col.RawSetString("dtype", StringValue(desc.DType.String()))
		col.RawSetString("length", IntValue(int64(desc.Len)))
		col.RawSetString("version", IntValue(int64(desc.Version)))
		cols.RawSetInt(int64(i+1), TableValue(col))
	}
	out.RawSetString("columns", TableValue(cols))
	return out
}

func soaAffineTermsFromTable(tbl *Table) ([]SoAAffineTerm, error) {
	terms, _, _, err := soaAffineTermsFromTableWithVersions(tbl)
	return terms, err
}

func soaAffineTermsFromTableWithVersions(tbl *Table) ([]SoAAffineTerm, []*Table, []uint64, error) {
	n := tbl.Length()
	terms := make([]SoAAffineTerm, 0, n)
	termTables := make([]*Table, 0, n)
	termVersions := make([]uint64, 0, n)
	for i := 1; i <= n; i++ {
		v := tbl.RawGetInt(int64(i))
		if !v.IsTable() {
			return nil, nil, nil, fmt.Errorf("soa.affineMany: term %d must be a table", i)
		}
		termTable := v.Table()
		termVersion := termTable.MutationVersion()
		dst := termTable.RawGetString("dst")
		src := termTable.RawGetString("src")
		scale := termTable.RawGetString("scale")
		bias := termTable.RawGetString("bias")
		if !dst.IsString() || !src.IsString() {
			return nil, nil, nil, fmt.Errorf("soa.affineMany: term %d requires string dst and src", i)
		}
		if !scale.IsNumber() {
			return nil, nil, nil, fmt.Errorf("soa.affineMany: term %d requires numeric scale", i)
		}
		if bias.IsNil() {
			bias = IntValue(0)
		}
		if !bias.IsNumber() {
			return nil, nil, nil, fmt.Errorf("soa.affineMany: term %d requires numeric bias", i)
		}
		terms = append(terms, SoAAffineTerm{
			Dst:   dst.Str(),
			Src:   src.Str(),
			Scale: scale.Number(),
			Bias:  bias.Number(),
		})
		termTables = append(termTables, termTable)
		termVersions = append(termVersions, termVersion)
	}
	return terms, termTables, termVersions, nil
}
