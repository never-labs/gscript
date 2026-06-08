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
	maskSpec, err := frameMaskSpecDetails(spec)
	if err != nil {
		return runtime.NilValue(), err
	}
	if maskSpec.Mode == "bool_column" {
		out, handled, err := frameVal.NativeFrameBoolMask(maskSpec.Name)
		if err != nil {
			return runtime.NilValue(), err
		}
		if !handled {
			return runtime.NilValue(), fmt.Errorf("FrameMask operand must be native frame (got %s)", frameVal.TypeName())
		}
		return out, nil
	}
	denseOp, err := runtime.DenseArrayCompareOp(maskSpec.Op)
	if err != nil {
		return runtime.NilValue(), err
	}
	out, handled, err := executeNativeFrameMaskOp(frameVal, maskSpec, denseOp)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameMask operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeNativeFrameMaskOp(frameVal runtime.Value, spec frameMaskSpecData, denseOp runtime.DenseArrayBinaryOp) (runtime.Value, bool, error) {
	if spec.Mode == "bool_column" {
		return frameVal.NativeFrameBoolMask(spec.Name)
	}
	if spec.RHSLiteral {
		return frameVal.NativeFrameMaskLiteralOp(spec.Name, denseOp, spec.RHS)
	}
	return frameVal.NativeFrameMaskOp(spec.Name, denseOp, spec.RHS)
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

func executeFrameFilterProjectColumnValue(frameVal, maskVal runtime.Value, names []string, resultName string) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterProjectColumn mask must be dense array (got %s)", maskVal.TypeName())
	}
	return executeFrameFilterProjectColumnDenseValue(frameVal, maskVal.DenseArray(), names, resultName)
}

func executeFrameFilterProjectColumnDenseValue(frameVal runtime.Value, mask *runtime.DenseArray, names []string, resultName string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameFilterProjectColumn(mask, names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameFilterProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameFilterGatherProjectColumnValue(frameVal, maskVal, indexVal runtime.Value, names []string, resultName string) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterGatherProjectColumn mask must be dense array (got %s)", maskVal.TypeName())
	}
	if !indexVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterGatherProjectColumn indexes must be dense array (got %s)", indexVal.TypeName())
	}
	return executeFrameFilterGatherProjectColumnDenseValue(frameVal, maskVal.DenseArray(), indexVal.DenseArray(), names, resultName)
}

func executeFrameFilterGatherProjectColumnDenseValue(frameVal runtime.Value, mask, indexes *runtime.DenseArray, names []string, resultName string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameFilterGatherProjectColumn(mask, indexes, names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameFilterGatherProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameFilterSliceProjectColumnValue(frameVal, maskVal, endVal runtime.Value, names []string, resultName string) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterSliceProjectColumn mask must be dense array (got %s)", maskVal.TypeName())
	}
	if !endVal.IsInt() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterSliceProjectColumn end must be int (got %s)", endVal.TypeName())
	}
	return executeFrameFilterSliceProjectColumnDenseValue(frameVal, maskVal.DenseArray(), endVal, names, resultName)
}

func executeFrameFilterSliceProjectColumnDenseValue(frameVal runtime.Value, mask *runtime.DenseArray, endVal runtime.Value, names []string, resultName string) (runtime.Value, error) {
	if !endVal.IsInt() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterSliceProjectColumn end must be int (got %s)", endVal.TypeName())
	}
	out, handled, err := frameVal.NativeFrameFilterSliceProjectColumn(mask, 0, int(endVal.Int()), names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameFilterSliceProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameFilterOrderGatherProjectColumnValue(frameVal, maskVal, orderSpec runtime.Value, names []string, resultName string) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterOrderGatherProjectColumn mask must be dense array (got %s)", maskVal.TypeName())
	}
	return executeFrameFilterOrderGatherProjectColumnDenseValue(frameVal, maskVal.DenseArray(), orderSpec, names, resultName)
}

func executeFrameFilterOrderGatherProjectColumnDenseValue(frameVal runtime.Value, mask *runtime.DenseArray, orderSpec runtime.Value, names []string, resultName string) (runtime.Value, error) {
	orderNames, desc, limit, err := frameOrderSpec(orderSpec)
	if err != nil {
		return runtime.NilValue(), err
	}
	out, handled, err := frameVal.NativeFrameFilterOrderGatherProjectColumn(mask, orderNames, desc, limit, names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameFilterOrderGatherProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameBoolMaskFilterProjectColumnValue(frameVal runtime.Value, maskName string, names []string, resultName string) (runtime.Value, error) {
	maskVal, handled, err := frameVal.NativeFrameBoolMask(maskName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameBoolMaskFilterProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return executeFrameFilterProjectColumnValue(frameVal, maskVal, names, resultName)
}

func executeFrameGroupAggregateValue(frameVal, maskVal, specVal runtime.Value) (runtime.Value, error) {
	spec, err := runtime.DecodeFrameGroupAggregateSpec(specVal)
	if err != nil {
		return runtime.NilValue(), err
	}
	var mask *runtime.DenseArray
	if !maskVal.IsNil() {
		if !maskVal.IsDenseArray() {
			return runtime.NilValue(), fmt.Errorf("FrameGroupAggregate mask must be dense array or nil (got %s)", maskVal.TypeName())
		}
		mask = maskVal.DenseArray()
	}
	out, handled, err := frameVal.NativeFrameGroupAggregate(mask, spec)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameGroupAggregate operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func qFrameGroupAggregateRuntimeShapeFromMaskValue(maskVal runtime.Value) string {
	if maskVal.IsNil() {
		return "group/aggregate"
	}
	return "filter/group/aggregate"
}

func qFrameGroupAggregateRuntimeShapeFromInstr(instr *Instr) string {
	if instr == nil || len(instr.Args) < 2 || valueIsConstNil(instr.Args[1]) {
		return "group/aggregate"
	}
	return "filter/group/aggregate"
}

func executeFrameCompareFilterProjectColumnValue(frameVal runtime.Value, sourceName string, op runtime.DenseArrayBinaryOp, rhs runtime.Value, names []string, resultName string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameCompareFilterProjectColumn(sourceName, op, rhs, names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameCompareFilterProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameCompareLiteralFilterProjectColumnValue(frameVal runtime.Value, sourceName string, op runtime.DenseArrayBinaryOp, rhs runtime.Value, names []string, resultName string) (runtime.Value, error) {
	out, handled, err := frameVal.NativeFrameCompareLiteralFilterProjectColumn(sourceName, op, rhs, names, resultName)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameCompareLiteralFilterProjectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeFrameFilterProjectValue(frameVal, maskVal runtime.Value, names []string) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("FrameFilterProject mask must be dense array (got %s)", maskVal.TypeName())
	}
	out, handled, err := frameVal.NativeFrameFilterProject(maskVal.DenseArray(), names)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameFilterProject operand must be native frame (got %s)", frameVal.TypeName())
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

func executeFrameOrderGatherValue(frameVal runtime.Value, spec runtime.Value) (runtime.Value, error) {
	names, desc, limit, err := frameOrderSpec(spec)
	if err != nil {
		return runtime.NilValue(), err
	}
	out, handled, err := frameVal.NativeFrameOrderGather(names, desc, limit)
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameOrderGather operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeQFrameSelectColumnValue(constants []runtime.Value, specs []QFrameSelectColumnSpec, specIdx int, frameVal runtime.Value, argVal runtime.Value, hasArg bool) (runtime.Value, error) {
	if specIdx < 0 || specIdx >= len(specs) {
		return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn spec index %d is out of range", specIdx)
	}
	spec := specs[specIdx]
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
	resultName := constants[spec.ResultColumnConst].Str()
	if spec.RowMode == QFrameSelectColumnRowsNone {
		if out, ok, err := executeQFrameSelectColumnCompareFilterProject(constants, spec, frameVal, argVal, hasArg, names, resultName); ok || err != nil {
			return out, err
		}
	}
	if !qFrameSelectColumnHasPredicate(spec) {
		rows := frameVal
		if spec.RowMode != QFrameSelectColumnRowsNone {
			rowArg, hasRowArg := qFrameSelectColumnRowArg(spec, argVal, hasArg)
			var err error
			rows, err = executeQFrameSelectColumnRows(constants, spec, rows, rowArg, hasRowArg)
			if err != nil {
				return runtime.NilValue(), err
			}
		}
		return executeFrameProjectColumnValue(rows, names, resultName)
	}
	rhs, hasRHS := qFrameSelectColumnCompareRHS(spec, argVal, hasArg)
	mask, err := executeQFrameSelectColumnDenseMask(constants, spec, frameVal, rhs, hasRHS)
	if err != nil {
		return runtime.NilValue(), err
	}
	if spec.RowMode == QFrameSelectColumnRowsNone {
		return executeFrameFilterProjectColumnDenseValue(frameVal, mask, names, resultName)
	}
	rowArg, hasRowArg := qFrameSelectColumnRowArg(spec, argVal, hasArg)
	switch spec.RowMode {
	case QFrameSelectColumnRowsGather:
		if !hasRowArg {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn gather path requires indexes")
		}
		if !rowArg.IsDenseArray() {
			return runtime.NilValue(), fmt.Errorf("FrameFilterGatherProjectColumn indexes must be dense array (got %s)", rowArg.TypeName())
		}
		return executeFrameFilterGatherProjectColumnDenseValue(frameVal, mask, rowArg.DenseArray(), names, resultName)
	case QFrameSelectColumnRowsSlice:
		if !hasRowArg {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn slice path requires end")
		}
		return executeFrameFilterSliceProjectColumnDenseValue(frameVal, mask, rowArg, names, resultName)
	case QFrameSelectColumnRowsOrderGather:
		if spec.RowOrderConst < 0 || spec.RowOrderConst >= len(constants) {
			return runtime.NilValue(), fmt.Errorf("QFrameSelectColumn order spec constant is out of range")
		}
		return executeFrameFilterOrderGatherProjectColumnDenseValue(frameVal, mask, constants[spec.RowOrderConst], names, resultName)
	}
	rows, err := executeFrameFilterValue(frameVal, runtime.DenseArrayValue(mask))
	if err != nil {
		return runtime.NilValue(), err
	}
	rows, err = executeQFrameSelectColumnRows(constants, spec, rows, rowArg, hasRowArg)
	if err != nil {
		return runtime.NilValue(), err
	}
	return executeFrameProjectColumnValue(rows, names, resultName)
}

func executeQFrameSelectColumnCompareFilterProject(constants []runtime.Value, spec QFrameSelectColumnSpec, frameVal runtime.Value, argVal runtime.Value, hasArg bool, names []string, resultName string) (runtime.Value, bool, error) {
	if len(spec.MaskTerms) > 0 {
		return runtime.NilValue(), false, nil
	}
	if spec.MaskSpecConst >= 0 {
		if spec.MaskSpecConst >= len(constants) {
			return runtime.NilValue(), true, fmt.Errorf("QFrameSelectColumn mask spec constant is out of range")
		}
		maskSpec, err := frameMaskSpecDetails(constants[spec.MaskSpecConst])
		if err != nil {
			return runtime.NilValue(), true, err
		}
		if maskSpec.Mode == "bool_column" {
			out, err := executeFrameBoolMaskFilterProjectColumnValue(frameVal, maskSpec.Name, names, resultName)
			return out, true, err
		}
		denseOp, err := runtime.DenseArrayCompareOp(maskSpec.Op)
		if err != nil {
			return runtime.NilValue(), true, err
		}
		if maskSpec.RHSLiteral {
			out, err := executeFrameCompareLiteralFilterProjectColumnValue(frameVal, maskSpec.Name, denseOp, maskSpec.RHS, names, resultName)
			return out, true, err
		}
		out, err := executeFrameCompareFilterProjectColumnValue(frameVal, maskSpec.Name, denseOp, maskSpec.RHS, names, resultName)
		return out, true, err
	}
	if spec.SourceColumnConst < 0 {
		return runtime.NilValue(), false, nil
	}
	if spec.SourceColumnConst >= len(constants) || !constants[spec.SourceColumnConst].IsString() {
		return runtime.NilValue(), true, fmt.Errorf("QFrameSelectColumn source column must be a string constant")
	}
	rhs, hasRHS := qFrameSelectColumnCompareRHS(spec, argVal, hasArg)
	if !hasRHS {
		return runtime.NilValue(), true, fmt.Errorf("QFrameSelectColumn compare path requires rhs")
	}
	out, err := executeFrameCompareFilterProjectColumnValue(frameVal, constants[spec.SourceColumnConst].Str(), spec.CompareOp, rhs, names, resultName)
	return out, true, err
}

func qFrameSelectColumnHasPredicate(spec QFrameSelectColumnSpec) bool {
	return len(spec.MaskTerms) > 0 || spec.MaskSpecConst >= 0 || spec.SourceColumnConst >= 0
}

func executeQFrameSelectColumnMask(constants []runtime.Value, spec QFrameSelectColumnSpec, frameVal runtime.Value, rhsVal runtime.Value, hasRHS bool) (runtime.Value, error) {
	mask, err := executeQFrameSelectColumnDenseMask(constants, spec, frameVal, rhsVal, hasRHS)
	if err != nil {
		return runtime.NilValue(), err
	}
	return runtime.DenseArrayValue(mask), nil
}

func executeQFrameSelectColumnDenseMask(constants []runtime.Value, spec QFrameSelectColumnSpec, frameVal runtime.Value, rhsVal runtime.Value, hasRHS bool) (*runtime.DenseArray, error) {
	if len(spec.MaskTerms) > 0 {
		if spec.MaskRoot < 0 || spec.MaskRoot >= len(spec.MaskTerms) {
			return nil, fmt.Errorf("QFrameSelectColumn mask root is out of range")
		}
		return executeQFrameMaskTermDense(constants, spec, spec.MaskRoot, frameVal, rhsVal, hasRHS)
	}
	if spec.MaskSpecConst >= 0 {
		if spec.MaskSpecConst >= len(constants) {
			return nil, fmt.Errorf("QFrameSelectColumn mask spec constant is out of range")
		}
		maskSpec, err := frameMaskSpecDetails(constants[spec.MaskSpecConst])
		if err != nil {
			return nil, err
		}
		out, handled, err := executeNativeFrameMaskDense(frameVal, maskSpec)
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, fmt.Errorf("QFrameSelectColumn operand must be native frame (got %s)", frameVal.TypeName())
		}
		return out, nil
	}
	if !hasRHS {
		return nil, fmt.Errorf("QFrameSelectColumn compare path requires rhs")
	}
	if spec.SourceColumnConst < 0 || spec.SourceColumnConst >= len(constants) || !constants[spec.SourceColumnConst].IsString() {
		return nil, fmt.Errorf("QFrameSelectColumn source column must be a string constant")
	}
	out, handled, err := frameVal.NativeFrameMaskOp(constants[spec.SourceColumnConst].Str(), spec.CompareOp, rhsVal)
	if err != nil {
		return nil, err
	}
	if !handled {
		return nil, fmt.Errorf("QFrameSelectColumn operand must be native frame (got %s)", frameVal.TypeName())
	}
	return qFrameDenseMaskFromValue(out, "QFrameSelectColumn mask")
}

func executeQFrameMaskTerm(constants []runtime.Value, spec QFrameSelectColumnSpec, termIdx int, frameVal runtime.Value, rhsVal runtime.Value, hasRHS bool) (runtime.Value, error) {
	mask, err := executeQFrameMaskTermDense(constants, spec, termIdx, frameVal, rhsVal, hasRHS)
	if err != nil {
		return runtime.NilValue(), err
	}
	return runtime.DenseArrayValue(mask), nil
}

func executeQFrameMaskTermDense(constants []runtime.Value, spec QFrameSelectColumnSpec, termIdx int, frameVal runtime.Value, rhsVal runtime.Value, hasRHS bool) (*runtime.DenseArray, error) {
	if termIdx < 0 || termIdx >= len(spec.MaskTerms) {
		return nil, fmt.Errorf("QFrameSelectColumn mask term is out of range")
	}
	term := spec.MaskTerms[termIdx]
	switch term.Kind {
	case QFrameMaskTermCompare:
		if term.SourceColumnConst < 0 || term.SourceColumnConst >= len(constants) || !constants[term.SourceColumnConst].IsString() {
			return nil, fmt.Errorf("QFrameSelectColumn mask term source column must be a string constant")
		}
		rhs := term.CompareRHSConst
		if term.DynamicCompareRHS {
			if !hasRHS {
				return nil, fmt.Errorf("QFrameSelectColumn mask term compare path requires rhs")
			}
			rhs = rhsVal
		} else if !term.HasCompareRHSConst {
			return nil, fmt.Errorf("QFrameSelectColumn mask term compare path requires rhs")
		}
		out, handled, err := frameVal.NativeFrameMaskOp(constants[term.SourceColumnConst].Str(), term.CompareOp, rhs)
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, fmt.Errorf("QFrameSelectColumn operand must be native frame (got %s)", frameVal.TypeName())
		}
		return qFrameDenseMaskFromValue(out, "QFrameSelectColumn mask term")
	case QFrameMaskTermFrameMask:
		if term.MaskSpecConst < 0 || term.MaskSpecConst >= len(constants) {
			return nil, fmt.Errorf("QFrameSelectColumn mask term spec constant is out of range")
		}
		maskSpec, err := frameMaskSpecDetails(constants[term.MaskSpecConst])
		if err != nil {
			return nil, err
		}
		out, handled, err := executeNativeFrameMaskDense(frameVal, maskSpec)
		if err != nil {
			return nil, err
		}
		if !handled {
			return nil, fmt.Errorf("QFrameSelectColumn operand must be native frame (got %s)", frameVal.TypeName())
		}
		return out, nil
	case QFrameMaskTermCombine:
		left, err := executeQFrameMaskTermDense(constants, spec, term.LeftTerm, frameVal, rhsVal, hasRHS)
		if err != nil {
			return nil, err
		}
		right, err := executeQFrameMaskTermDense(constants, spec, term.RightTerm, frameVal, rhsVal, hasRHS)
		if err != nil {
			return nil, err
		}
		return runtime.DenseArrayMaskCombineArrays(term.CombineOp, left, right)
	default:
		return nil, fmt.Errorf("QFrameSelectColumn unknown mask term kind %d", term.Kind)
	}
}

func executeNativeFrameMaskDense(frameVal runtime.Value, spec frameMaskSpecData) (*runtime.DenseArray, bool, error) {
	if spec.Mode == "bool_column" {
		out, handled, err := frameVal.NativeFrameBoolMask(spec.Name)
		if err != nil || !handled {
			return nil, handled, err
		}
		mask, err := qFrameDenseMaskFromValue(out, "QFrameSelectColumn bool mask")
		return mask, true, err
	}
	denseOp, err := runtime.DenseArrayCompareOp(spec.Op)
	if err != nil {
		return nil, true, err
	}
	out, handled, err := executeNativeFrameMaskOp(frameVal, spec, denseOp)
	if err != nil || !handled {
		return nil, handled, err
	}
	mask, err := qFrameDenseMaskFromValue(out, "QFrameSelectColumn frame mask")
	return mask, true, err
}

func qFrameDenseMaskFromValue(v runtime.Value, context string) (*runtime.DenseArray, error) {
	if !v.IsDenseArray() {
		return nil, fmt.Errorf("%s must be a dense array (got %s)", context, v.TypeName())
	}
	return v.DenseArray(), nil
}

func qFrameSelectColumnCompareRHS(spec QFrameSelectColumnSpec, argVal runtime.Value, hasArg bool) (runtime.Value, bool) {
	if len(spec.MaskTerms) > 0 {
		for _, term := range spec.MaskTerms {
			if term.DynamicCompareRHS && hasArg {
				return argVal, true
			}
		}
		return runtime.NilValue(), false
	}
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
	if spec.HasRowValueConst {
		return spec.RowValueConst, true
	}
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
		return executeFrameOrderGatherValue(rows, constants[spec.RowOrderConst])
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

func frameProjectColumnSpec(v runtime.Value) ([]string, string, error) {
	if v.IsString() {
		name := v.Str()
		if name == "" {
			return nil, "", fmt.Errorf("FrameProjectColumn result column name must not be empty")
		}
		return []string{name}, name, nil
	}
	if !v.IsTable() {
		return nil, "", fmt.Errorf("FrameProjectColumn spec must be a string or table")
	}
	tbl := v.Table()
	project := tbl.RawGetString("project")
	if project.IsNil() {
		project = tbl.RawGetInt(1)
	}
	names, err := frameProjectColumnNames(project)
	if err != nil {
		return nil, "", err
	}
	result := tbl.RawGetString("column")
	if result.IsNil() {
		result = tbl.RawGetString("result")
	}
	if result.IsNil() {
		result = tbl.RawGetInt(2)
	}
	if !result.IsString() {
		return nil, "", fmt.Errorf("FrameProjectColumn spec must provide result column")
	}
	return names, result.Str(), nil
}

type frameMaskSpecData struct {
	Name       string
	Op         string
	RHS        runtime.Value
	Mode       string
	RHSLiteral bool
}

func frameMaskSpec(v runtime.Value) (string, string, runtime.Value, error) {
	spec, err := frameMaskSpecDetails(v)
	if err != nil {
		return "", "", runtime.NilValue(), err
	}
	return spec.Name, spec.Op, spec.RHS, nil
}

func frameMaskSpecDetails(v runtime.Value) (frameMaskSpecData, error) {
	if !v.IsTable() {
		return frameMaskSpecData{}, fmt.Errorf("FrameMask spec must be a table")
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
		mode := tbl.RawGetString("mode")
		if mode.IsString() && mode.Str() == "bool_column" && column.IsString() {
			return frameMaskSpecData{Name: column.Str(), Mode: "bool_column"}, nil
		}
		return frameMaskSpecData{}, fmt.Errorf("FrameMask spec must provide column and op")
	}
	valueKind := tbl.RawGetString("value_kind")
	mode := tbl.RawGetString("mode")
	return frameMaskSpecData{
		Name:       column.Str(),
		Op:         op.Str(),
		RHS:        rhs,
		Mode:       stringValueOrEmpty(mode),
		RHSLiteral: valueKind.IsString() && valueKind.Str() == "literal",
	}, nil
}

func stringValueOrEmpty(v runtime.Value) string {
	if v.IsString() {
		return v.Str()
	}
	return ""
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
	out, handled, err := frameVal.NativeFrameLen()
	if err != nil {
		return runtime.NilValue(), err
	}
	if !handled {
		return runtime.NilValue(), fmt.Errorf("FrameLen operand must be native frame (got %s)", frameVal.TypeName())
	}
	return out, nil
}

func executeVectorCompareValue(opCode int, leftVal, rightVal runtime.Value) (runtime.Value, error) {
	op := runtime.DenseArrayBinaryOp(opCode)
	if op < runtime.DenseArrayEQ || op > runtime.DenseArrayGE {
		return runtime.NilValue(), fmt.Errorf("VectorCompare op %d is not a comparison", op)
	}
	return runtime.DenseArrayElementwise(op, leftVal, rightVal)
}

func executeVectorMaskValue(opCode int, leftVal, rightVal runtime.Value) (runtime.Value, error) {
	op := runtime.DenseArrayMaskOp(opCode)
	out, err := runtime.DenseArrayMaskCombine(op, leftVal, rightVal)
	if err != nil {
		return runtime.NilValue(), err
	}
	return runtime.DenseArrayValue(out), nil
}

func executeVectorWhereValue(maskVal, trueVal, falseVal runtime.Value) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("VectorWhere mask must be dense array (got %s)", maskVal.TypeName())
	}
	out, err := runtime.DenseArrayWhere(maskVal.DenseArray(), trueVal, falseVal)
	if err != nil {
		return runtime.NilValue(), err
	}
	return runtime.DenseArrayValue(out), nil
}

func executeVectorReduceValue(opCode int, vectorVal runtime.Value) (runtime.Value, error) {
	if !vectorVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("VectorReduce operand must be dense array (got %s)", vectorVal.TypeName())
	}
	return runtime.DenseArrayReduce(runtime.DenseArrayReduceOp(opCode), vectorVal.DenseArray())
}

func executeQVectorWhereReduceValue(opCode int, maskVal, trueVal, falseVal runtime.Value) (runtime.Value, error) {
	if !maskVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("QVectorWhereReduce mask must be dense array (got %s)", maskVal.TypeName())
	}
	return runtime.DenseArrayWhereReduce(runtime.DenseArrayReduceOp(opCode), maskVal.DenseArray(), trueVal, falseVal)
}

func executeQVectorGatherReduceValue(opCode int, vectorVal, indexVal runtime.Value) (runtime.Value, error) {
	gathered, err := executeVectorGatherValue(vectorVal, indexVal)
	if err != nil {
		return runtime.NilValue(), err
	}
	return executeVectorReduceValue(opCode, gathered)
}

func executeVectorScanValue(vectorVal runtime.Value) (runtime.Value, error) {
	if !vectorVal.IsDenseArray() {
		return runtime.NilValue(), fmt.Errorf("VectorScan operand must be dense array (got %s)", vectorVal.TypeName())
	}
	out, err := runtime.DenseArrayScan(vectorVal.DenseArray())
	if err != nil {
		return runtime.NilValue(), err
	}
	return runtime.DenseArrayValue(out), nil
}
