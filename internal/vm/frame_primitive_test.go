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

func frameFilterPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_FRAME_FILTER, 0, 0, 1),
		EncodeABC(OP_FRAME_COLUMN, 0, 0, 2),
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
