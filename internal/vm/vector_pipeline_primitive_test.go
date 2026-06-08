package vm

import (
	"reflect"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestVectorPrimitivePipelineCompareMaskWhereGatherReduce(t *testing.T) {
	price := runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{100.5, 99, 120, 80}))
	size := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, 20, 5, 30}))
	signedQty := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{10, -20, 5, 30}))
	indexes := runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1, 4, 2}))
	proto := &FuncProto{
		MaxStack: 8,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABx(OP_LOADK, 1, 1),
			EncodeABC(OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			EncodeABx(OP_LOADK, 2, 2),
			EncodeABx(OP_LOADK, 3, 3),
			EncodeABC(OP_VECTOR_COMPARE, 2, 3, int(runtime.DenseArrayGE)),
			EncodeABC(OP_VECTOR_MASK, 0, 2, int(runtime.DenseArrayMaskAnd)),
			EncodeABx(OP_LOADK, 4, 4),
			EncodeABx(OP_LOADK, 5, 5),
			EncodeABC(OP_VECTOR_WHERE, 0, 4, 5),
			EncodeABx(OP_LOADK, 6, 6),
			EncodeABC(OP_VECTOR_GATHER, 0, 6, 0),
			EncodeABC(OP_MOVE, 7, 0, 0),
			EncodeABC(OP_VECTOR_REDUCE, 0, 0, int(runtime.DenseArrayReduceSum)),
			EncodeABC(OP_MOVE, 1, 0, 0),
			EncodeABC(OP_MOVE, 0, 7, 0),
			EncodeABC(OP_RETURN, 0, 3, 0),
		},
		Constants: []runtime.Value{
			price,
			runtime.FloatValue(100),
			size,
			runtime.IntValue(10),
			signedQty,
			runtime.IntValue(0),
			indexes,
		},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute vector primitive pipeline: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("pipeline results len = %d, want 2", len(results))
	}
	if !results[0].IsDenseArray() {
		t.Fatalf("pipeline gathered result = %v, want dense array", results[0])
	}
	gotVector, ok := results[0].DenseArray().I64()
	if !ok {
		t.Fatalf("pipeline gathered dtype = %s, want i64", results[0].DenseArray().DType())
	}
	if want := []int64{0, 10, 0, 0}; !reflect.DeepEqual(gotVector, want) {
		t.Fatalf("pipeline gathered values = %#v, want %#v", gotVector, want)
	}
	if !results[1].IsInt() || results[1].Int() != 10 {
		t.Fatalf("pipeline reduce result = %v, want int 10", results[1])
	}
}
