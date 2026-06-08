package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorWhereReducePrimitiveSumsWhereSelection(t *testing.T) {
	proto := &FuncProto{
		MaxStack: 3,
		Code:     vectorWhereReducePrimitiveProgram(runtime.DenseArrayReduceSum),
		Constants: vectorWhereReducePrimitiveConstants(
			runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true})),
			runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30})),
			runtime.IntValue(7),
		),
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_WHERE_REDUCE sum: %v", err)
	}
	if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 47 {
		t.Fatalf("VECTOR_WHERE_REDUCE sum result = %#v, want int 47", results)
	}

	fb := proto.Feedback[3]
	if fb.Left != FBAny || fb.Right != FBAny || fb.Result != FBInt {
		t.Fatalf("VECTOR_WHERE_REDUCE feedback = left %v right %v result %v, want dense/dense/int buckets", fb.Left, fb.Right, fb.Result)
	}
}

func TestVectorWhereReducePrimitiveMatchesWhereThenReduceMean(t *testing.T) {
	mask := runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true, false}))
	trueValue := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1, 100, 5, 100}))
	falseValue := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{20, 2, 20, 6}))
	proto := &FuncProto{
		MaxStack:  3,
		Code:      vectorWhereReducePrimitiveProgram(runtime.DenseArrayReduceMean),
		Constants: vectorWhereReducePrimitiveConstants(mask, trueValue, falseValue),
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_WHERE_REDUCE mean: %v", err)
	}
	selected, err := runtime.DenseArrayWhere(mask.DenseArray(), trueValue, falseValue)
	if err != nil {
		t.Fatalf("DenseArrayWhere oracle error: %v", err)
	}
	want, err := runtime.DenseArrayReduce(runtime.DenseArrayReduceMean, selected)
	if err != nil {
		t.Fatalf("DenseArrayReduce oracle error: %v", err)
	}
	if len(results) != 1 || !results[0].Equal(want) {
		t.Fatalf("VECTOR_WHERE_REDUCE mean result = %#v, want %v", results, want)
	}
}

func TestVectorWhereReducePrimitiveRejectsNonDenseMask(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  3,
		Code:      vectorWhereReducePrimitiveProgram(runtime.DenseArrayReduceSum),
		Constants: vectorWhereReducePrimitiveConstants(runtime.BoolValue(true), runtime.IntValue(10), runtime.IntValue(1)),
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_WHERE_REDUCE mask must be dense array") {
		t.Fatalf("VECTOR_WHERE_REDUCE non-dense mask error = %v, want dense-array mask error", err)
	}
}

func TestVectorWhereReducePrimitiveRejectsBoolSelection(t *testing.T) {
	proto := &FuncProto{
		MaxStack: 3,
		Code:     vectorWhereReducePrimitiveProgram(runtime.DenseArrayReduceSum),
		Constants: vectorWhereReducePrimitiveConstants(
			runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false})),
			runtime.BoolValue(true),
			runtime.BoolValue(false),
		),
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), runtime.ErrDenseArrayDType.Error()) {
		t.Fatalf("VECTOR_WHERE_REDUCE bool selection error = %v, want dtype error", err)
	}
}

func TestVectorWhereReducePrimitiveRejectsMissingFalseOperand(t *testing.T) {
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABx(OP_LOADK, 1, 1),
			EncodeABC(OP_VECTOR_WHERE_REDUCE, 0, 1, int(runtime.DenseArrayReduceSum)),
			EncodeABC(OP_RETURN, 0, 2, 0),
		},
		Constants: []runtime.Value{
			runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true})),
			runtime.IntValue(10),
		},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_WHERE_REDUCE true/false operand pair out of range") {
		t.Fatalf("VECTOR_WHERE_REDUCE missing false operand error = %v, want operand-pair error", err)
	}
}

func vectorWhereReducePrimitiveProgram(op runtime.DenseArrayReduceOp) []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABx(OP_LOADK, 2, 2),
		EncodeABC(OP_VECTOR_WHERE_REDUCE, 0, 1, int(op)),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func vectorWhereReducePrimitiveConstants(mask, trueValue, falseValue runtime.Value) []runtime.Value {
	return []runtime.Value{mask, trueValue, falseValue}
}
