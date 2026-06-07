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

func executeVectorCompareValue(opCode int, leftVal, rightVal runtime.Value) (runtime.Value, error) {
	op := runtime.DenseArrayBinaryOp(opCode)
	if op < runtime.DenseArrayEQ || op > runtime.DenseArrayGE {
		return runtime.NilValue(), fmt.Errorf("VectorCompare op %d is not a comparison", op)
	}
	return runtime.DenseArrayElementwise(op, leftVal, rightVal)
}
