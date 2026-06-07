package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorComparePrimitiveExecutesDenseArrayCompare(t *testing.T) {
	values := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20, 30, 40}))
	threshold := runtime.FloatValue(25)
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorComparePrimitiveProgram(runtime.DenseArrayGE),
		Constants: []runtime.Value{values, threshold},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_COMPARE: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_COMPARE result = %#v, want one dense array", results)
	}
	got, ok := results[0].DenseArray().Bool()
	if !ok {
		t.Fatalf("VECTOR_COMPARE dtype = %s, want bool", results[0].DenseArray().DType())
	}
	want := []bool{false, false, true, true}
	if len(got) != len(want) {
		t.Fatalf("VECTOR_COMPARE len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VECTOR_COMPARE[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	fb := proto.Feedback[2]
	if fb.Left != FBAny || fb.Right != FBFloat || fb.Result != FBAny {
		t.Fatalf("VECTOR_COMPARE feedback = left %v right %v result %v, want dense/float/dense buckets", fb.Left, fb.Right, fb.Result)
	}
}

func TestVectorComparePrimitiveComparesDenseArrays(t *testing.T) {
	left := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6}))
	right := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, 4, 5}))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorComparePrimitiveProgram(runtime.DenseArrayGE),
		Constants: []runtime.Value{left, right},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_COMPARE arrays: %v", err)
	}
	assertVectorCompareBoolResult(t, results, []bool{false, true, true})
}

func TestVectorComparePrimitiveRejectsArithmeticOp(t *testing.T) {
	values := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20}))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorComparePrimitiveProgram(runtime.DenseArrayAdd),
		Constants: []runtime.Value{values, runtime.FloatValue(1)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_COMPARE op") {
		t.Fatalf("VECTOR_COMPARE arithmetic op error = %v, want comparison-op error", err)
	}
}

func TestVectorComparePrimitiveRejectsNonDenseOperand(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorComparePrimitiveProgram(runtime.DenseArrayGE),
		Constants: []runtime.Value{runtime.IntValue(10), runtime.IntValue(5)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "dense array operation requires at least one dense array operand") {
		t.Fatalf("VECTOR_COMPARE scalar-scalar error = %v, want dense operand error", err)
	}
}

func vectorComparePrimitiveProgram(op runtime.DenseArrayBinaryOp) []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_VECTOR_COMPARE, 0, 1, int(op)),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func assertVectorCompareBoolResult(t *testing.T, results []runtime.Value, want []bool) {
	t.Helper()
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_COMPARE result = %#v, want one dense array", results)
	}
	got, ok := results[0].DenseArray().Bool()
	if !ok {
		t.Fatalf("VECTOR_COMPARE dtype = %s, want bool", results[0].DenseArray().DType())
	}
	if len(got) != len(want) {
		t.Fatalf("VECTOR_COMPARE len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VECTOR_COMPARE[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
