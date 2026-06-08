package methodjit

import (
	"fmt"

	"github.com/never-labs/leia/internal/runtime"
)

func executeVectorGatherValue(vectorVal, indexVal runtime.Value) (runtime.Value, error) {
	if !vectorVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("VectorGather operand must be dense array (got %s)", vectorVal.TypeName())
	}
	if !indexVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("VectorGather indexes must be dense array (got %s)", indexVal.TypeName())
	}
	out, err := vectorVal.DenseArray().Gather(indexVal.DenseArray())
	if err != nil {
		return runtime.NilValue(), err
	}
	return runtime.DenseArrayValue(out), nil
}

func executeFrameColumnValue(frameVal runtime.Value, name string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameColumn(name)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameMaskValue(frameVal runtime.Value, spec runtime.Value) (runtime.Value, error) {
	name, op, rhs, err := frameMaskSpec(spec)
	if err != nil {
		return runtime.NilValue(), err
	}
	denseOp, err := runtime.DenseArrayCompareOp(op)
	if err != nil {
		return runtime.NilValue(), err
	}
	out, handled, err := frameVal.NativeFrameMaskOp(name, denseOp, rhs)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameMask operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameProjectValue(frameVal runtime.Value, names []string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameProject(names)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameProject operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameProjectColumnValue(frameVal runtime.Value, names []string, resultName string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameProjectColumn(names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameFilterValue(frameVal, maskVal runtime.Value) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilter mask must be dense array (got %s)", maskVal.TypeName())
	}
	out, handled, err := frameVal.NativeFrameFilter(maskVal.DenseArray())
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameFilter operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameGatherValue(frameVal, indexVal runtime.Value) (runtime.Value, error) {
	if !indexVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameGather indexes must be dense array (got %s)", indexVal.TypeName())
	}
	out, handled, err := frameVal.NativeFrameGather(indexVal.DenseArray())
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameGather operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameSliceValue(frameVal, endVal runtime.Value) (runtime.Value, error) {
	if !endVal.IsInt() {
		return runtime.NilValue(), fmt.Errorf("FrameSlice end must be int (got %s)", endVal.TypeName())
	}
	out, handled, err := frameVal.NativeFrameSlice(0, int(endVal.Int()))
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameSlice operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameOrderValue(frameVal runtime.Value, spec runtime.Value) (runtime.Value, error) {
	names, desc, limit, err := frameOrderSpec(spec)
	if err != nil {
		return runtime.NilValue(), err
	}
	out, handled, err := frameVal.NativeFrameOrderIndexes(names, desc, limit)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameOrder operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeQFrameSelectColumnValue(constants []runtime.Value, specs []QFrameSelectColumnSpec, specIdx int, frameVal runtime.Value, argVal runtime.Value, hasArg bool) (runtime.Value, error) {
	if specIdx < 0 || specIdx >= len(specs) {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn spec index %d is out of range", specIdx)
	}
	spec := specs[specIdx]
	rhs, hasRHS := qFrameSelectColumnCompareRHS(spec, argVal, hasArg)
	mask, err := executeQFrameSelectColumnMask(constants, spec, frameVal, rhs, hasRHS)
	if err != nil {
		return runtime.NilValue(), err
	}
	rows, err := executeFrameFilterValue(frameVal, mask)
	if err != nil {
		return runtime.NilValue(), err
	}
	rowArg, hasRowArg := qFrameSelectColumnRowArg(spec, argVal, hasArg)
	rows, err = executeQFrameSelectColumnRows(constants, spec, rows, rowArg, hasRowArg)
	if err != nil {
		return runtime.NilValue(), err
	}
	if spec.ProjectConst < 0 || spec.ProjectConst >= len(constants) {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn project constant is out of range")
	}
	names, err := frameProjectColumnNames(constants[spec.ProjectConst])
	if err != nil {
		return runtime.NilValue(), err
	}
	if spec.ResultColumnConst < 0 || spec.ResultColumnConst >= len(constants) || !constants[spec.ResultColumnConst].IsString() {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn result column must be a string constant")
	}
	return executeFrameProjectColumnValue(rows, names, constants[spec.ResultColumnConst].Str())
}

func executeQFrameSelectColumnMask(constants []runtime.Value, spec QFrameSelectColumnSpec, frameVal runtime.Value, rhsVal runtime.Value, hasRHS bool) (runtime.Value, error) {
	if spec.MaskSpecConst >= 0 {
		if spec.MaskSpecConst >= len(constants) {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn mask spec constant is out of range")
		}
		name, op, rhs, err := frameMaskSpec(constants[spec.MaskSpecConst])
		if err != nil {
			return runtime.NilValue(), err
		}
		denseOp, err := runtime.DenseArrayCompareOp(op)
		if err != nil {
			return runtime.NilValue(), err
		}
		out, handled, err := frameVal.NativeFrameMaskOp(name, denseOp, rhs)
		if err != nil {
			return runtime.NilValue(), err
		}
		if !handled {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn operand must be native frame (got %s)", frameVal.TypeName())
		}
		return out, nil
	}
	if !hasRHS {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn compare path requires rhs")
	}
	if spec.SourceColumnConst < 0 || spec.SourceColumnConst >= len(constants) || !constants[spec.SourceColumnConst].IsString() {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn source column must be a string constant")
	}
	out, handled, err := frameVal.NativeFrameMaskOp(constants[spec.SourceColumnConst].Str(), spec.CompareOp, rhsVal)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func qFrameSelectColumnCompareRHS(spec QFrameSelectColumnSpec, argVal runtime.Value, hasArg bool) (runtime.Value, bool) {
	if spec.MaskSpecConst >= 0 {
		return runtime.NilValue(), false
	}
	if spec.HasCompareRHSConst {
		return spec.CompareRHSConst, true
	}
	if spec.DynamicArgRole == QFrameSelectColumnArgCompareRHS && hasArg {
		return argVal, true
	}
	return runtime.NilValue(), false
}

func qFrameSelectColumnRowArg(spec QFrameSelectColumnSpec, argVal runtime.Value, hasArg bool) (runtime.Value, bool) {
	if spec.DynamicArgRole == QFrameSelectColumnArgRowValue && hasArg {
		return argVal, true
	}
	return runtime.NilValue(), false
}

func executeQFrameSelectColumnRows(constants []runtime.Value, spec QFrameSelectColumnSpec, rows runtime.Value, rowArg runtime.Value, hasRowArg bool) (runtime.Value, error) {
	switch spec.RowMode {
	case QFrameSelectColumnRowsNone:
		return rows, nil
	case QFrameSelectColumnRowsGather:
		if !hasRowArg {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn gather path requires indexes")
		}
		return executeFrameGatherValue(rows, rowArg)
	case QFrameSelectColumnRowsSlice:
		if !hasRowArg {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn slice path requires end")
		}
		return executeFrameSliceValue(rows, rowArg)
	case QFrameSelectColumnRowsOrderGather:
		if spec.RowOrderConst < 0 || spec.RowOrderConst >= len(constants) {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn order spec constant is out of range")
		}
		indexes, err := executeFrameOrderValue(rows, constants[spec.RowOrderConst])
		if err != nil {
			return runtime.NilValue(), err
		}
		return executeFrameGatherValue(rows, indexes)
	default:
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn unknown row mode %d", spec.RowMode)
	}
}

func frameProjectColumnNames(v runtime.Value) ([]string, error) {
	if v.IsString() {
		return []string{v.Str()}, nil
	}
	if !v.IsTable() {
		return nil, fmt.Errorf("FrameProject column list must be a string or string array")
	}
	tbl := v.Table()
	names := make([]string, 0, tbl.Length())
	for i := 1; i <= tbl.Length(); i++ {
		item := tbl.RawGetInt(int64(i))
		if !item.IsString() {
			return nil, fmt.Errorf("FrameProject column list item %d must be a string", i)
		}
		names = append(names, item.Str())
	}
	return names, nil
}

func frameMaskSpec(v runtime.Value) (string, string, runtime.Value, error) {
	if !v.IsTable() {
		return "", "", runtime.NilValue(), fmt.Errorf("FrameMask spec must be a table")
	}
	tbl := v.Table()
	column := tbl.RawGetString("column")
	if column.IsNil() {
		column = tbl.RawGetInt(1)
	}
	op := tbl.RawGetString("op")
	if op.IsNil() {
		op = tbl.RawGetInt(2)
	}
	rhs := tbl.RawGetString("value")
	if rhs.IsNil() {
		rhs = tbl.RawGetInt(3)
	}
	if !column.IsString() || !op.IsString() {
		return "", "", runtime.NilValue(), fmt.Errorf("FrameMask spec must provide column and op")
	}
	return column.Str(), op.Str(), rhs, nil
}

func frameOrderSpec(v runtime.Value) ([]string, []bool, int, error) {
	if v.IsString() {
		return []string{v.Str()}, []bool{false}, -1, nil
	}
	if !v.IsTable() {
		return nil, nil, -1, fmt.Errorf("FrameOrder spec must be a string or table")
	}
	tbl := v.Table()
	limit := -1
	if limitValue := tbl.RawGetString("limit"); !limitValue.IsNil() {
		if !limitValue.IsInt() {
			return nil, nil, -1, fmt.Errorf("FrameOrder limit must be an integer")
		}
		if limitValue.Int() < 0 {
			return nil, nil, -1, fmt.Errorf("FrameOrder limit must be non-negative")
		}
		limit = int(limitValue.Int())
	}
	if col := tbl.RawGetString("column"); col.IsString() {
		return []string{col.Str()}, []bool{frameOrderTruthy(tbl.RawGetString("desc"))}, limit, nil
	}
	n := tbl.Length()
	names := make([]string, 0, n)
	desc := make([]bool, 0, n)
	for i := 1; i <= n; i++ {
		item := tbl.RawGetInt(int64(i))
		switch {
		case item.IsString():
			names = append(names, item.Str())
			desc = append(desc, false)
		case item.IsTable():
			itemTable := item.Table()
			col := itemTable.RawGetString("column")
			if !col.IsString() {
				return nil, nil, -1, fmt.Errorf("FrameOrder item %d must provide column", i)
			}
			names = append(names, col.Str())
			desc = append(desc, frameOrderTruthy(itemTable.RawGetString("desc")))
		default:
			return nil, nil, -1, fmt.Errorf("FrameOrder item %d must be a string or table", i)
		}
	}
	return names, desc, limit, nil
}

func frameOrderTruthy(v runtime.Value) bool {
	return !(v.IsNil() || (v.IsBool() && !v.Bool()))
}

func executeFrameLenValue(frameVal runtime.Value) (runtime.Value, error) {
	info, ok := frameVal.NativeFramePayloadInfo()
	if !ok {
		return runtime.NilValue(), fmt.Errorf("FrameLen operand must be native frame (got %s)", frameVal.TypeName())
	}
	return runtime.IntValue(int64(info.Rows)), nil
}

func executeVectorCompareValue(opCode int, leftVal, rightVal runtime.Value) (runtime.Value, error) {
	op := runtime.DenseArrayBinaryOp(opCode)
	if op < runtime.DenseArrayEQ || op > runtime.DenseArrayGE {
		return runtime.NilValue(), fmt.Errorf("VectorCompare op %d is not a comparison", op)
	}
	return runtime.DenseArrayElementwise(op, leftVal, rightVal)
}
