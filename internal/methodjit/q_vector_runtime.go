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
