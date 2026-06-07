package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorGatherPrimitiveExecutesDenseArrayGather(t *testing.T) {
	values := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20, 30, 40}))
	indexes := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{4, 2, 1}))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorGatherPrimitiveProgram(),
		Constants: []runtime.Value{values, indexes},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_GATHER: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_GATHER result = %#v, want one dense array", results)
	}
	got, ok := results[0].DenseArray().F64()
	if !ok {
		t.Fatalf("VECTOR_GATHER dtype = %s, want f64", results[0].DenseArray().DType())
	}
	want := []float64{40, 20, 10}
	if len(got) != len(want) {
		t.Fatalf("VECTOR_GATHER len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VECTOR_GATHER[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	fb := proto.Feedback[2]
	if fb.Left != FBAny || fb.Right != FBAny || fb.Result != FBAny {
		t.Fatalf("VECTOR_GATHER feedback = left %v right %v result %v, want observed dense arrays in FBAny bucket", fb.Left, fb.Right, fb.Result)
	}
}

func TestVectorGatherPrimitiveRejectsNonI64Indexes(t *testing.T) {
	values := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20}))
	indexes := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1}))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorGatherPrimitiveProgram(),
		Constants: []runtime.Value{values, indexes},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "indices must be i64") {
		t.Fatalf("VECTOR_GATHER non-i64 indexes error = %v, want i64 error", err)
	}
}

func TestVectorGatherPrimitiveRejectsNonDenseArrayVector(t *testing.T) {
	indexes := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1}))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      vectorGatherPrimitiveProgram(),
		Constants: []runtime.Value{runtime.StringValue("not-vector"), indexes},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_GATHER operand must be dense array") {
		t.Fatalf("VECTOR_GATHER non-vector error = %v, want dense array error", err)
	}
}

func vectorGatherPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_VECTOR_GATHER, 0, 1, 0),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}
