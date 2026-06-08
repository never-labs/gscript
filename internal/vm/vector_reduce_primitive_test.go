package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorReducePrimitiveSumsDenseI64(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorReducePrimitiveProgram(runtime.DenseArrayReduceSum),
		Constants: []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, -2, 7}))},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_REDUCE sum: %v", err)
	}
	if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 15 {
		t.Fatalf("VECTOR_REDUCE sum result = %#v, want int 15", results)
	}

	fb := proto.Feedback[1]
	if fb.Left != FBAny || fb.Result != FBInt {
		t.Fatalf("VECTOR_REDUCE feedback = left %v result %v, want dense bucket/int", fb.Left, fb.Result)
	}
}

func TestVectorReducePrimitiveMeansDenseF64(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorReducePrimitiveProgram(runtime.DenseArrayReduceMean),
		Constants: []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1.5, 2.5, 5.0}))},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_REDUCE mean: %v", err)
	}
	if len(results) != 1 || !results[0].IsFloat() || results[0].Float() != 3 {
		t.Fatalf("VECTOR_REDUCE mean result = %#v, want float 3", results)
	}
}

func TestVectorReducePrimitiveRejectsNonDenseOperand(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorReducePrimitiveProgram(runtime.DenseArrayReduceSum),
		Constants: []runtime.Value{runtime.IntValue(10)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_REDUCE operand must be dense array") {
		t.Fatalf("VECTOR_REDUCE non-dense error = %v, want dense operand error", err)
	}
}

func TestVectorReducePrimitiveRejectsBoolDenseArray(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorReducePrimitiveProgram(runtime.DenseArrayReduceSum),
		Constants: []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false}))},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), runtime.ErrDenseArrayDType.Error()) {
		t.Fatalf("VECTOR_REDUCE bool array error = %v, want dtype error", err)
	}
}

func vectorReducePrimitiveProgram(op runtime.DenseArrayReduceOp) []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_VECTOR_REDUCE, 0, 0, int(op)),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}
