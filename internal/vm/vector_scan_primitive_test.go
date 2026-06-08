package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorScanPrimitiveScansDenseI64(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorScanPrimitiveProgram(),
		Constants: []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{2, -1, 4}))},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_SCAN i64: %v", err)
	}
	assertVectorScanI64Result(t, results, []int64{2, 1, 5})

	fb := proto.Feedback[1]
	if fb.Left != FBAny || fb.Result != FBAny {
		t.Fatalf("VECTOR_SCAN feedback = left %v result %v, want dense buckets", fb.Left, fb.Result)
	}
}

func TestVectorScanPrimitiveScansDenseF64(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorScanPrimitiveProgram(),
		Constants: []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{1.5, 2.25, -0.75}))},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_SCAN f64: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_SCAN result = %#v, want one dense array", results)
	}
	got, ok := results[0].DenseArray().F64()
	if !ok || len(got) != 3 || got[0] != 1.5 || got[1] != 3.75 || got[2] != 3 {
		t.Fatalf("VECTOR_SCAN f64 values = %#v, want [1.5 3.75 3]", got)
	}
}

func TestVectorScanPrimitiveRejectsNonDenseOperand(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorScanPrimitiveProgram(),
		Constants: []runtime.Value{runtime.IntValue(10)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_SCAN operand must be dense array") {
		t.Fatalf("VECTOR_SCAN non-dense error = %v, want dense operand error", err)
	}
}

func TestVectorScanPrimitiveRejectsBoolDenseArray(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      vectorScanPrimitiveProgram(),
		Constants: []runtime.Value{runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false}))},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), runtime.ErrDenseArrayDType.Error()) {
		t.Fatalf("VECTOR_SCAN bool array error = %v, want dtype error", err)
	}
}

func vectorScanPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_VECTOR_SCAN, 0, 0, 0),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func assertVectorScanI64Result(t *testing.T, results []runtime.Value, want []int64) {
	t.Helper()
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_SCAN result = %#v, want one dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok {
		t.Fatalf("VECTOR_SCAN dtype = %s, want i64", results[0].DenseArray().DType())
	}
	if len(got) != len(want) {
		t.Fatalf("VECTOR_SCAN len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VECTOR_SCAN[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
