package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorWherePrimitiveSelectsDenseArrayValues(t *testing.T) {
	mask := runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, true}))
	trueValues := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30}))
	falseValues := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 2, 3}))
	proto := &FuncProto{
		MaxStack:  3,
		Code:      vectorWherePrimitiveProgram(),
		Constants: []runtime.Value{mask, trueValues, falseValues},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_WHERE: %v", err)
	}
	assertVectorWhereI64Result(t, results, []int64{10, 2, 30})

	fb := proto.Feedback[3]
	if fb.Left != FBAny || fb.Right != FBAny || fb.Result != FBAny {
		t.Fatalf("VECTOR_WHERE feedback = left %v right %v result %v, want dense/dense/dense in FBAny bucket", fb.Left, fb.Right, fb.Result)
	}
}

func TestVectorWherePrimitiveSelectsScalarFallbackValues(t *testing.T) {
	mask := runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, false}))
	trueValues := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 30}))
	proto := &FuncProto{
		MaxStack:  3,
		Code:      vectorWherePrimitiveProgram(),
		Constants: []runtime.Value{mask, trueValues, runtime.IntValue(7)},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_WHERE scalar: %v", err)
	}
	assertVectorWhereI64Result(t, results, []int64{10, 7, 7})
}

func TestVectorWherePrimitiveRejectsNonDenseMask(t *testing.T) {
	proto := &FuncProto{
		MaxStack: 3,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABx(OP_LOADK, 1, 1),
			EncodeABx(OP_LOADK, 2, 2),
			EncodeABC(OP_VECTOR_WHERE, 0, 1, 2),
			EncodeABC(OP_RETURN, 0, 2, 0),
		},
		Constants: []runtime.Value{
			runtime.BoolValue(true),
			runtime.IntValue(10),
			runtime.IntValue(1),
		},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "VECTOR_WHERE mask must be dense array") {
		t.Fatalf("VECTOR_WHERE non-dense mask error = %v, want dense-array mask error", err)
	}
}

func TestVectorWherePrimitiveRejectsNonBoolMask(t *testing.T) {
	mask := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 0}))
	proto := &FuncProto{
		MaxStack:  3,
		Code:      vectorWherePrimitiveProgram(),
		Constants: []runtime.Value{mask, runtime.IntValue(10), runtime.IntValue(1)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "dense array where mask must be bool") {
		t.Fatalf("VECTOR_WHERE non-bool mask error = %v, want bool-mask error", err)
	}
}

func vectorWherePrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABx(OP_LOADK, 2, 2),
		EncodeABC(OP_VECTOR_WHERE, 0, 1, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func assertVectorWhereI64Result(t *testing.T, results []runtime.Value, want []int64) {
	t.Helper()
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_WHERE result = %#v, want one dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok {
		t.Fatalf("VECTOR_WHERE dtype = %s, want i64", results[0].DenseArray().DType())
	}
	if len(got) != len(want) {
		t.Fatalf("VECTOR_WHERE len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("VECTOR_WHERE[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}
