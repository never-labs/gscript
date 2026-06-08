package vm

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestFrameLenPrimitiveReadsNativeFrameRows(t *testing.T) {
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(struct{}{}, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       3,
		Columns:    2,
		SchemaHash: "frame-test",
	})
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameLenPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame)},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_LEN: %v", err)
	}
	if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 3 {
		t.Fatalf("FRAME_LEN result = %#v, want int 3", results)
	}

	fb := proto.Feedback[1]
	if fb.Left != FBTable || fb.Result != FBInt {
		t.Fatalf("FRAME_LEN feedback = left %v result %v, want frame-as-table and int", fb.Left, fb.Result)
	}
}

func TestFrameLenPrimitiveReadsNativeKeyedFrameRows(t *testing.T) {
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(struct{}{}, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadKeyedFrame,
		Rows:       4,
		Columns:    3,
		SchemaHash: "keyed-frame-test",
	})
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameLenPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame)},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_LEN keyed frame: %v", err)
	}
	if len(results) != 1 || !results[0].IsInt() || results[0].Int() != 4 {
		t.Fatalf("FRAME_LEN keyed result = %#v, want int 4", results)
	}
}

func TestFrameLenPrimitiveRejectsPlainTable(t *testing.T) {
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameLenPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(runtime.NewTable())},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "FRAME_LEN operand must be native frame") {
		t.Fatalf("FRAME_LEN plain table error = %v, want native frame error", err)
	}
}

func TestFrameColumnPrimitiveReadsSoAColumn(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5, 3.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-test",
	})
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameColumnPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.StringValue("x")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_COLUMN: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_COLUMN result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().F64()
	if !ok {
		t.Fatalf("FRAME_COLUMN dtype = %s, want f64", results[0].DenseArray().DType())
	}
	want := []float64{1.5, 2.5, 3.5}
	if len(got) != len(want) {
		t.Fatalf("FRAME_COLUMN len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("FRAME_COLUMN[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	fb := proto.Feedback[1]
	if fb.Left != FBTable || fb.Right != FBString || fb.Result != FBAny {
		t.Fatalf("FRAME_COLUMN feedback = left %v right %v result %v, want frame-as-table/string/dense bucket", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameMaskPrimitiveBuildsSoAMask(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10, 12, 8}),
		"id":    runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-test",
	})
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue("price"))
	spec.RawSetString("op", runtime.StringValue(">="))
	spec.RawSetString("value", runtime.FloatValue(10))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameMaskPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_MASK: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_MASK follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("masked id column = %#v, want [10 20]", got)
	}
	if fb := proto.Feedback[1]; fb.Left != FBTable || fb.Right != FBTable || fb.Result != FBAny {
		t.Fatalf("FRAME_MASK feedback = left %v right %v result %v, want table/table/dense bucket", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameColumnPrimitiveRejectsMissingColumn(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x": runtime.NewDenseArrayF64([]float64{1, 2}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    soa.Len(),
		Columns: 1,
	})
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameColumnPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.StringValue("missing")},
	}

	_, err = New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), `FRAME_COLUMN unknown column "missing"`) {
		t.Fatalf("FRAME_COLUMN missing column error = %v, want unknown column error", err)
	}
}

func TestFrameColumnPrimitiveRejectsUnsupportedPayload(t *testing.T) {
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(struct{}{}, runtime.NativePayloadInfo{
		Kind:    runtime.NativePayloadDataFrame,
		Rows:    2,
		Columns: 1,
	})
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameColumnPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.StringValue("x")},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "FRAME_COLUMN unsupported native frame payload") {
		t.Fatalf("FRAME_COLUMN unsupported payload error = %v, want unsupported payload error", err)
	}
}

func TestFrameProjectPrimitiveReadsSoAColumns(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20}),
		"z":  runtime.NewDenseArrayF64([]float64{7, 8}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    3,
		SchemaHash: "soa-frame-test",
	})
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("id"))
	names.RawSetInt(2, runtime.StringValue("x"))
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameProjectPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(names), runtime.StringValue("x")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_PROJECT: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_PROJECT follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 1.5 || got[1] != 2.5 {
		t.Fatalf("projected x column = %#v, want [1.5 2.5]", got)
	}

	fb := proto.Feedback[1]
	if fb.Left != FBTable || fb.Right != FBTable || fb.Result != FBTable {
		t.Fatalf("FRAME_PROJECT feedback = left %v right %v result %v, want table/table/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameFilterPrimitiveFiltersSoARows(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5, 3.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-test",
	})
	mask := runtime.NewDenseArrayBool([]bool{true, false, true})
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameFilterPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.DenseArrayValue(mask), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_FILTER: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_FILTER follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Fatalf("filtered id column = %#v, want [10 30]", got)
	}

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBTable {
		t.Fatalf("FRAME_FILTER feedback = left %v right %v result %v, want table/dense bucket/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameGatherPrimitiveGathersSoARows(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5, 3.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-test",
	})
	indexes := runtime.NewDenseArrayI64([]int64{3, 1})
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameGatherPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.DenseArrayValue(indexes), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_GATHER: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_GATHER follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 30 || got[1] != 10 {
		t.Fatalf("gathered id column = %#v, want [30 10]", got)
	}

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBTable {
		t.Fatalf("FRAME_GATHER feedback = left %v right %v result %v, want table/dense bucket/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameSlicePrimitiveSlicesSoARows(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5, 3.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-test",
	})
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameSlicePrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.IntValue(2), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_SLICE: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_SLICE follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("sliced id column = %#v, want [10 20]", got)
	}

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBInt || fb.Result != FBTable {
		t.Fatalf("FRAME_SLICE feedback = left %v right %v result %v, want table/int/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameOrderPrimitiveBuildsGatherIndexes(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10, 12, 8}),
		"id":    runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-test",
	})
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	order.RawSetString("desc", runtime.BoolValue(true))
	order.RawSetString("limit", runtime.IntValue(2))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameOrderPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(order), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_ORDER: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_ORDER follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 20 || got[1] != 10 {
		t.Fatalf("ordered id column = %#v, want [20 10]", got)
	}

	fb := proto.Feedback[1]
	if fb.Left != FBTable || fb.Right != FBTable || fb.Result != FBAny {
		t.Fatalf("FRAME_ORDER feedback = left %v right %v result %v, want table/table/dense bucket", fb.Left, fb.Right, fb.Result)
	}
}

func TestFramePrimitivePipelineFiltersProjectsAndLoadsColumn(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{99, 100.5, 101.25}),
		"size":  runtime.NewDenseArrayI64([]int64{5, 10, 20}),
		"flag":  runtime.NewDenseArrayBool([]bool{false, true, true}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    3,
		SchemaHash: "pipeline-frame-test",
	})
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("size"))
	names.RawSetInt(2, runtime.StringValue("price"))
	proto := &FuncProto{
		MaxStack:  3,
		Code:      framePipelinePrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.StringValue("price"), runtime.FloatValue(100), runtime.TableValue(names), runtime.StringValue("size")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute frame primitive pipeline: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("frame primitive pipeline result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 20 {
		t.Fatalf("pipeline size column = %#v, want [10 20]", got)
	}
	if fb := proto.Feedback[1]; fb.Left != FBTable || fb.Result != FBAny {
		t.Fatalf("pipeline FRAME_COLUMN feedback = left %v result %v, want table/dense bucket", fb.Left, fb.Result)
	}
	if fb := proto.Feedback[4]; fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBTable {
		t.Fatalf("pipeline FRAME_FILTER feedback = left %v right %v result %v, want table/dense bucket/table", fb.Left, fb.Right, fb.Result)
	}
	if fb := proto.Feedback[5]; fb.Left != FBTable || fb.Right != FBTable || fb.Result != FBTable {
		t.Fatalf("pipeline FRAME_PROJECT feedback = left %v right %v result %v, want table/table/table", fb.Left, fb.Right, fb.Result)
	}
}

func frameLenPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_LEN, 0, 0, 0),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameProjectPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_PROJECT, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameMaskPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_MASK, 1, 0, 1),
		EncodeABC(OP_FRAME_FILTER, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameFilterPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_FRAME_FILTER, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameGatherPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_FRAME_GATHER, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameSlicePrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_FRAME_SLICE, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameOrderPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_ORDER, 1, 0, 1),
		EncodeABC(OP_FRAME_GATHER, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func framePipelinePrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_COLUMN, 1, 0, 1),
		EncodeABx(OP_LOADK, 2, 2),
		EncodeABC(OP_VECTOR_COMPARE, 1, 2, int(runtime.DenseArrayGE)),
		EncodeABC(OP_FRAME_FILTER, 0, 0, 1),
		EncodeABC(OP_FRAME_PROJECT, 0, 0, 3),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 4),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}

func frameColumnPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 1),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}
