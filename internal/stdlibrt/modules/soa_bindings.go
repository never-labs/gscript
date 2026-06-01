package modules

import (
	"fmt"

	stdsoa "github.com/never-labs/gscript/internal/stdlib/soa"
)

// BuildSOA creates the "soa" data-oriented structure-of-arrays library.
func BuildSOA() *Table {
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
		start, end := stdsoa.SliceRange(args[1].Int(), args[2].Int())
		out, err := s.Slice(start, end)
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

	indicesCache := &soaIndicesWhereCache{}
	setFastArg2("indicesWhere", func(args []Value) ([]Value, error) {
		out, err := indicesCache.args("soa.indicesWhere", args)
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	}, indicesCache.value)

	setFastArg4("scatterInto", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.scatterInto", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.scatterInto: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsDenseArray() {
			return nil, fmt.Errorf("soa.scatterInto: argument 3 must be an i64 dense array")
		}
		if len(args) < 4 {
			return nil, fmt.Errorf("soa.scatterInto: argument 4 is required")
		}
		if err := s.ScatterInto(args[1].Str(), args[2].DenseArray(), args[3]); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaScatterIntoValue)

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
		kernel, err := requireSoAKernelArgs("soa.addScaled", args)
		if err != nil {
			return nil, err
		}
		if err := s.AddScaled(kernel.Dst, kernel.Src, kernel.Scale); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaAddScaledValue)

	setFastArg5("affine", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.affine", args, 0)
		if err != nil {
			return nil, err
		}
		kernel, err := requireSoAKernelArgs("soa.affine", args)
		if err != nil {
			return nil, err
		}
		if len(args) < 5 || !args[4].IsNumber() {
			return nil, fmt.Errorf("soa.affine: argument 5 must be numeric")
		}
		affine := stdsoa.NewAffineArgs(kernel.Dst, kernel.Src, kernel.Scale, args[4].Number())
		if err := s.Affine(affine.Dst, affine.Src, affine.Scale, affine.Bias); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaAffineValue)

	setFastArg6("affineWhere", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.affineWhere", args, 0)
		if err != nil {
			return nil, err
		}
		kernel, err := requireSoAKernelArgs("soa.affineWhere", args)
		if err != nil {
			return nil, err
		}
		if len(args) < 5 || !args[4].IsDenseArray() {
			return nil, fmt.Errorf("soa.affineWhere: argument 5 must be a bool dense array")
		}
		if len(args) < 6 || !args[5].IsNumber() {
			return nil, fmt.Errorf("soa.affineWhere: argument 6 must be numeric")
		}
		affine := stdsoa.NewAffineArgs(kernel.Dst, kernel.Src, kernel.Scale, args[5].Number())
		if err := s.AffineWhere(affine.Dst, affine.Src, args[4].DenseArray(), affine.Scale, affine.Bias); err != nil {
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

	setFastArg2("min", func(args []Value) ([]Value, error) {
		v, err := soaColumnReduceArgs("soa.min", args, (*SoA).Min)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaMinValue)

	setFastArg2("max", func(args []Value) ([]Value, error) {
		v, err := soaColumnReduceArgs("soa.max", args, (*SoA).Max)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaMaxValue)

	setFastArg2("mean", func(args []Value) ([]Value, error) {
		v, err := soaColumnReduceArgs("soa.mean", args, (*SoA).Mean)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaMeanValue)

	statsCache := &soaStatsCache{}
	setFastArg2("stats", func(args []Value) ([]Value, error) {
		v, err := statsCache.args("soa.stats", args)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, statsCache.value)

	setFastArg3("sumWhere", func(args []Value) ([]Value, error) {
		v, err := soaMaskedAggregateArgs("soa.sumWhere", args, (*SoA).SumWhere)
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaSumWhereValue)

	setFastArg2("scan", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.scan", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.scan: argument 2 must be a string")
		}
		out, err := s.Scan(args[1].Str())
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(out)}, nil
	}, soaScanValue)

	setFastArg3("scanInto", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.scanInto", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.scanInto: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsString() {
			return nil, fmt.Errorf("soa.scanInto: argument 3 must be a string")
		}
		if err := s.ScanInto(args[1].Str(), args[2].Str()); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaScanIntoValue)

	setFastArg4("clamp", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.clamp", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.clamp: argument 2 must be a string")
		}
		if len(args) < 4 {
			return nil, fmt.Errorf("soa.clamp: arguments 3 and 4 are required")
		}
		out, err := s.Clamp(args[1].Str(), args[2], args[3])
		if err != nil {
			return nil, err
		}
		return []Value{DenseArrayValue(out)}, nil
	}, soaClampValue)

	setFastArg5("clampInto", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.clampInto", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.clampInto: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsString() {
			return nil, fmt.Errorf("soa.clampInto: argument 3 must be a string")
		}
		if len(args) < 5 {
			return nil, fmt.Errorf("soa.clampInto: arguments 4 and 5 are required")
		}
		if err := s.ClampInto(args[1].Str(), args[2].Str(), args[3], args[4]); err != nil {
			return nil, err
		}
		return []Value{BoolValue(true)}, nil
	}, soaClampIntoValue)

	setFastArg3("dot", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.dot", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.dot: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsString() {
			return nil, fmt.Errorf("soa.dot: argument 3 must be a string")
		}
		v, err := s.Dot(args[1].Str(), args[2].Str())
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaDotValue)

	setFastArg4("dotWhere", func(args []Value) ([]Value, error) {
		s, err := requireSoAArg("soa.dotWhere", args, 0)
		if err != nil {
			return nil, err
		}
		if len(args) < 2 || !args[1].IsString() {
			return nil, fmt.Errorf("soa.dotWhere: argument 2 must be a string")
		}
		if len(args) < 3 || !args[2].IsString() {
			return nil, fmt.Errorf("soa.dotWhere: argument 3 must be a string")
		}
		if len(args) < 4 || !args[3].IsDenseArray() {
			return nil, fmt.Errorf("soa.dotWhere: argument 4 must be a bool dense array")
		}
		v, err := s.DotWhere(args[1].Str(), args[2].Str(), args[3].DenseArray())
		if err != nil {
			return nil, err
		}
		return []Value{v}, nil
	}, soaDotWhereValue)

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

	maskCache := &soaMaskCache{}
	setFastArg4("mask", func(args []Value) ([]Value, error) {
		out, err := maskCache.args("soa.mask", args)
		if err != nil {
			return nil, err
		}
		return []Value{out}, nil
	}, maskCache.value)

	return t
}
