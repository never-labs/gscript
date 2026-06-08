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

func TestVectorMaskPrimitiveCombinesDenseBoolMasks(t *testing.T) {
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABx(OP_LOADK, 1, 1),
			EncodeABC(OP_VECTOR_MASK, 0, 1, int(runtime.DenseArrayMaskOr)),
			EncodeABC(OP_RETURN, 0, 2, 0),
		},
		Constants: []runtime.Value{
			runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{true, false, false})),
			runtime.DenseArrayValue(runtime.NewDenseArrayBool([]bool{false, true, false})),
		},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute VECTOR_MASK: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("VECTOR_MASK result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().Bool()
	if !ok || len(got) != 3 || !got[0] || !got[1] || got[2] {
		t.Fatalf("VECTOR_MASK values = %#v, want [true true false]", got)
	}

	fb := proto.Feedback[2]
	if fb.Left != FBAny || fb.Right != FBAny || fb.Result != FBAny {
		t.Fatalf("VECTOR_MASK feedback = left %v right %v result %v, want any/any/any", fb.Left, fb.Right, fb.Result)
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

func TestFrameMaskPrimitiveSupportsStringLiteralSpec(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"sym": runtime.NewDenseArrayString([]string{"AAPL", "MSFT", "AAPL"}),
		"id":  runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-string-mask-test",
	})
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue("sym"))
	spec.RawSetString("op", runtime.StringValue("=="))
	spec.RawSetString("value", runtime.StringValue("AAPL"))
	spec.RawSetString("value_kind", runtime.StringValue("literal"))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameMaskPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec), runtime.StringValue("id")},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_MASK string literal: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_MASK string literal follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Fatalf("string literal masked id column = %#v, want [10 30]", got)
	}
}

func TestFrameMaskPrimitiveSupportsBoolColumnSpec(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"active": runtime.NewDenseArrayBool([]bool{true, false, true}),
		"id":     runtime.NewDenseArrayI64([]int64{10, 20, 30}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    2,
		SchemaHash: "soa-frame-bool-mask-test",
	})
	spec := runtime.NewTable()
	spec.RawSetString("column", runtime.StringValue("active"))
	spec.RawSetString("mode", runtime.StringValue("bool_column"))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameMaskPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec), runtime.StringValue("id")},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_MASK bool column: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_MASK bool column follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Fatalf("bool column masked id column = %#v, want [10 30]", got)
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

func TestFrameProjectColumnPrimitiveReadsProjectedSoAColumn(t *testing.T) {
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
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.TableValue(names))
	spec.RawSetString("column", runtime.StringValue("x"))
	proto := &FuncProto{
		MaxStack:  1,
		Code:      frameProjectColumnPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec)},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_PROJECT_COLUMN: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_PROJECT_COLUMN result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 1.5 || got[1] != 2.5 {
		t.Fatalf("project-column x = %#v, want [1.5 2.5]", got)
	}

	fb := proto.Feedback[1]
	if fb.Left != FBTable || fb.Right != FBTable || fb.Result != FBAny {
		t.Fatalf("FRAME_PROJECT_COLUMN feedback = left %v right %v result %v, want table/table/dense bucket", fb.Left, fb.Right, fb.Result)
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

func TestFrameFilterProjectColumnPrimitiveFiltersProjectedColumn(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5, 3.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20, 30}),
		"z":  runtime.NewDenseArrayF64([]float64{7, 8, 9}),
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
	mask := runtime.NewDenseArrayBool([]bool{true, false, true})
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("id"))
	names.RawSetInt(2, runtime.StringValue("x"))
	spec := runtime.NewTable()
	spec.RawSetString("project", runtime.TableValue(names))
	spec.RawSetString("column", runtime.StringValue("id"))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameFilterProjectColumnPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.DenseArrayValue(mask), runtime.TableValue(spec)},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_FILTER_PROJECT_COLUMN: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_FILTER_PROJECT_COLUMN result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Fatalf("filtered project-column id = %#v, want [10 30]", got)
	}

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBAny {
		t.Fatalf("FRAME_FILTER_PROJECT_COLUMN feedback = left %v right %v result %v, want table/dense bucket/dense bucket", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameFilterProjectPrimitiveFiltersProjectedFrame(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"x":  runtime.NewDenseArrayF64([]float64{1.5, 2.5, 3.5}),
		"id": runtime.NewDenseArrayI64([]int64{10, 20, 30}),
		"z":  runtime.NewDenseArrayF64([]float64{7, 8, 9}),
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
	mask := runtime.NewDenseArrayBool([]bool{true, false, true})
	names := runtime.NewTable()
	names.RawSetInt(1, runtime.StringValue("id"))
	names.RawSetInt(2, runtime.StringValue("x"))
	proto := &FuncProto{
		MaxStack:  2,
		Code:      frameFilterProjectPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.DenseArrayValue(mask), runtime.TableValue(names), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_FILTER_PROJECT: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_FILTER_PROJECT follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 10 || got[1] != 30 {
		t.Fatalf("filtered project id = %#v, want [10 30]", got)
	}

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBTable {
		t.Fatalf("FRAME_FILTER_PROJECT feedback = left %v right %v result %v, want table/dense bucket/table", fb.Left, fb.Right, fb.Result)
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

func TestFrameOrderGatherPrimitiveOrdersFrameRows(t *testing.T) {
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
		MaxStack:  1,
		Code:      frameOrderGatherPrimitiveProgram(),
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(order), runtime.StringValue("id")},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_ORDER_GATHER: %v", err)
	}
	if len(results) != 1 || !results[0].IsDenseArray() {
		t.Fatalf("FRAME_ORDER_GATHER follow-up column result = %#v, want dense array", results)
	}
	got, ok := results[0].DenseArray().I64()
	if !ok || len(got) != 2 || got[0] != 20 || got[1] != 10 {
		t.Fatalf("ordered/gathered id column = %#v, want [20 10]", got)
	}

	fb := proto.Feedback[1]
	if fb.Left != FBTable || fb.Right != FBTable || fb.Result != FBTable {
		t.Fatalf("FRAME_ORDER_GATHER feedback = left %v right %v result %v, want table/table/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameOrderGatherPrimitiveRejectsNonNativeFrame(t *testing.T) {
	order := runtime.NewTable()
	order.RawSetString("column", runtime.StringValue("price"))
	proto := &FuncProto{
		MaxStack: 1,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABC(OP_FRAME_ORDER_GATHER, 0, 0, 1),
			EncodeABC(OP_RETURN, 0, 2, 0),
		},
		Constants: []runtime.Value{runtime.TableValue(runtime.NewTable()), runtime.TableValue(order)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), "FRAME_ORDER_GATHER operand must be native frame") {
		t.Fatalf("FRAME_ORDER_GATHER non-frame error = %v, want native frame error", err)
	}
}

func TestFrameGroupAggregatePrimitiveNoKeyCountAndSum(t *testing.T) {
	frame := frameGroupAggregateTestFrame(t)
	spec := frameGroupAggregateSpec("", []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "qty"},
	})
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABC(OP_LOADNIL, 1, 0, 0),
			EncodeABC(OP_FRAME_GROUP_AGGREGATE, 1, 0, 1),
			EncodeABC(OP_RETURN, 1, 2, 0),
		},
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec)},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_GROUP_AGGREGATE no-key: %v", err)
	}
	out := frameGroupAggregateResultSoA(t, results)
	assertI64Column(t, out, "n", []int64{4})
	assertI64Column(t, out, "total", []int64{25})

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBTable {
		t.Fatalf("FRAME_GROUP_AGGREGATE feedback = left %v right %v result %v, want table/nil-bucket/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameGroupAggregatePrimitiveSingleKeyAndMask(t *testing.T) {
	frame := frameGroupAggregateTestFrame(t)
	spec := frameGroupAggregateSpec("acct", []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "amount"},
	})
	mask := runtime.NewDenseArrayBool([]bool{true, false, true, true})
	proto := &FuncProto{
		MaxStack: 3,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABx(OP_LOADK, 1, 2),
			EncodeABC(OP_FRAME_GROUP_AGGREGATE, 1, 0, 1),
			EncodeABC(OP_RETURN, 1, 2, 0),
		},
		Constants: []runtime.Value{
			runtime.TableValue(frame),
			runtime.TableValue(spec),
			runtime.DenseArrayValue(mask),
		},
	}
	proto.EnsureFeedback()

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_GROUP_AGGREGATE keyed: %v", err)
	}
	out := frameGroupAggregateResultSoA(t, results)
	assertI64Column(t, out, "acct", []int64{1, 2})
	assertI64Column(t, out, "n", []int64{2, 1})
	assertF64Column(t, out, "total", []float64{17.5, 3.5})

	fb := proto.Feedback[2]
	if fb.Left != FBTable || fb.Right != FBAny || fb.Result != FBTable {
		t.Fatalf("FRAME_GROUP_AGGREGATE masked feedback = left %v right %v result %v, want table/dense-bucket/table", fb.Left, fb.Right, fb.Result)
	}
}

func TestFrameGroupAggregatePrimitiveMultiKey(t *testing.T) {
	frame := frameGroupAggregateTestFrame(t)
	spec := frameGroupAggregateSpecMulti([]string{"acct", "flag"}, []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "qty"},
	})
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABC(OP_LOADNIL, 1, 0, 0),
			EncodeABC(OP_FRAME_GROUP_AGGREGATE, 1, 0, 1),
			EncodeABC(OP_RETURN, 1, 2, 0),
		},
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec)},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_GROUP_AGGREGATE multi-key: %v", err)
	}
	out := frameGroupAggregateResultSoA(t, results)
	assertI64Column(t, out, "acct", []int64{1, 1, 2})
	assertBoolColumn(t, out, "flag", []bool{true, false, true})
	assertI64Column(t, out, "n", []int64{2, 1, 1})
	assertI64Column(t, out, "total", []int64{17, 5, 3})
}

func TestFrameGroupAggregatePrimitiveMinMaxAvg(t *testing.T) {
	frame := frameGroupAggregateTestFrame(t)
	spec := frameGroupAggregateSpec("acct", []runtime.FrameAggregateSpec{
		{Name: "min_qty", Op: "min", Column: "qty"},
		{Name: "max_amount", Op: "max", Column: "amount"},
		{Name: "avg_qty", Op: "avg", Column: "qty"},
	})
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABC(OP_LOADNIL, 1, 0, 0),
			EncodeABC(OP_FRAME_GROUP_AGGREGATE, 1, 0, 1),
			EncodeABC(OP_RETURN, 1, 2, 0),
		},
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec)},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_GROUP_AGGREGATE min/max/avg: %v", err)
	}
	out := frameGroupAggregateResultSoA(t, results)
	assertI64Column(t, out, "acct", []int64{1, 2})
	assertI64Column(t, out, "min_qty", []int64{5, 3})
	assertF64Column(t, out, "max_amount", []float64{10, 3.5})
	assertF64Column(t, out, "avg_qty", []float64{22.0 / 3.0, 3})
}

func TestFrameGroupAggregatePrimitiveStringKey(t *testing.T) {
	frame := frameGroupAggregateTestFrame(t)
	spec := frameGroupAggregateSpec("sym", []runtime.FrameAggregateSpec{
		{Name: "n", Op: "count"},
		{Name: "total", Op: "sum", Column: "qty"},
	})
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABC(OP_LOADNIL, 1, 0, 0),
			EncodeABC(OP_FRAME_GROUP_AGGREGATE, 1, 0, 1),
			EncodeABC(OP_RETURN, 1, 2, 0),
		},
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec)},
	}

	results, err := New(map[string]runtime.Value{}).Execute(proto)
	if err != nil {
		t.Fatalf("Execute FRAME_GROUP_AGGREGATE string key: %v", err)
	}
	out := frameGroupAggregateResultSoA(t, results)
	assertStringColumn(t, out, "sym", []string{"AAPL", "MSFT"})
	assertI64Column(t, out, "n", []int64{3, 1})
	assertI64Column(t, out, "total", []int64{20, 5})
}

func TestFrameGroupAggregatePrimitiveRejectsUnsupportedSumColumn(t *testing.T) {
	frame := frameGroupAggregateTestFrame(t)
	spec := frameGroupAggregateSpec("", []runtime.FrameAggregateSpec{
		{Name: "bad", Op: "sum", Column: "flag"},
	})
	proto := &FuncProto{
		MaxStack: 2,
		Code: []uint32{
			EncodeABx(OP_LOADK, 0, 0),
			EncodeABC(OP_LOADNIL, 1, 0, 0),
			EncodeABC(OP_FRAME_GROUP_AGGREGATE, 1, 0, 1),
			EncodeABC(OP_RETURN, 1, 2, 0),
		},
		Constants: []runtime.Value{runtime.TableValue(frame), runtime.TableValue(spec)},
	}

	_, err := New(map[string]runtime.Value{}).Execute(proto)
	if err == nil || !strings.Contains(err.Error(), `FRAME_GROUP_AGGREGATE sum column "flag" must be numeric`) {
		t.Fatalf("FRAME_GROUP_AGGREGATE bool sum error = %v, want numeric error", err)
	}
}

func TestFrameGroupAggregateSchemaHashIncludesSpecShape(t *testing.T) {
	frame := runtime.TableValue(frameGroupAggregateTestFrame(t))
	countSpec := runtime.FrameGroupAggregateSpec{
		By: "acct",
		Aggregates: []runtime.FrameAggregateSpec{
			{Name: "n", Op: "count"},
		},
	}
	sumSpec := runtime.FrameGroupAggregateSpec{
		By: "acct",
		Aggregates: []runtime.FrameAggregateSpec{
			{Name: "total", Op: "sum", Column: "amount"},
		},
	}

	countOut, handled, err := frame.NativeFrameGroupAggregate(nil, countSpec)
	if err != nil || !handled {
		t.Fatalf("count NativeFrameGroupAggregate handled=%v err=%v", handled, err)
	}
	sumOut, handled, err := frame.NativeFrameGroupAggregate(nil, sumSpec)
	if err != nil || !handled {
		t.Fatalf("sum NativeFrameGroupAggregate handled=%v err=%v", handled, err)
	}
	countInfo, ok := countOut.NativeFramePayloadInfo()
	if !ok {
		t.Fatalf("count aggregate missing native payload info")
	}
	sumInfo, ok := sumOut.NativeFramePayloadInfo()
	if !ok {
		t.Fatalf("sum aggregate missing native payload info")
	}
	if countInfo.SchemaHash == "" || sumInfo.SchemaHash == "" || countInfo.SchemaHash == sumInfo.SchemaHash {
		t.Fatalf("group aggregate schema hashes = %q/%q, want non-empty distinct hashes", countInfo.SchemaHash, sumInfo.SchemaHash)
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

func frameGroupAggregateTestFrame(t *testing.T) *runtime.Table {
	t.Helper()
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"acct":   runtime.NewDenseArrayI64([]int64{1, 1, 2, 1}),
		"sym":    runtime.NewDenseArrayString([]string{"AAPL", "MSFT", "AAPL", "AAPL"}),
		"qty":    runtime.NewDenseArrayI64([]int64{10, 5, 3, 7}),
		"amount": runtime.NewDenseArrayF64([]float64{10, 5, 3.5, 7.5}),
		"flag":   runtime.NewDenseArrayBool([]bool{true, false, true, true}),
	})
	if err != nil {
		t.Fatalf("NewSoA: %v", err)
	}
	frame := runtime.NewTable()
	frame.SetNativePayloadWithInfo(soa, runtime.NativePayloadInfo{
		Kind:       runtime.NativePayloadDataFrame,
		Rows:       soa.Len(),
		Columns:    5,
		SchemaHash: "frame-group-aggregate-test",
	})
	return frame
}

func frameGroupAggregateSpec(by string, aggs []runtime.FrameAggregateSpec) *runtime.Table {
	spec := runtime.NewTable()
	if by != "" {
		spec.RawSetString("by", runtime.StringValue(by))
	}
	frameGroupAggregateSetAggs(spec, aggs)
	return spec
}

func frameGroupAggregateSpecMulti(by []string, aggs []runtime.FrameAggregateSpec) *runtime.Table {
	spec := runtime.NewTable()
	if len(by) > 0 {
		byTable := runtime.NewAppendArrayTable(len(by))
		for i, name := range by {
			byTable.RawSetInt(int64(i+1), runtime.StringValue(name))
		}
		spec.RawSetString("by", runtime.TableValue(byTable))
	}
	frameGroupAggregateSetAggs(spec, aggs)
	return spec
}

func frameGroupAggregateSetAggs(spec *runtime.Table, aggs []runtime.FrameAggregateSpec) {
	aggRows := runtime.NewAppendArrayTable(len(aggs))
	for i, agg := range aggs {
		row := runtime.NewTable()
		row.RawSetString("name", runtime.StringValue(agg.Name))
		row.RawSetString("op", runtime.StringValue(agg.Op))
		if agg.Column != "" {
			row.RawSetString("column", runtime.StringValue(agg.Column))
		}
		aggRows.RawSetInt(int64(i+1), runtime.TableValue(row))
	}
	spec.RawSetString("aggregates", runtime.TableValue(aggRows))
}

func frameGroupAggregateResultSoA(t *testing.T, results []runtime.Value) *runtime.SoA {
	t.Helper()
	if len(results) != 1 || !results[0].IsTable() {
		t.Fatalf("FRAME_GROUP_AGGREGATE result = %#v, want table", results)
	}
	payload, _, ok := results[0].Table().NativeFramePayload()
	if !ok {
		t.Fatalf("FRAME_GROUP_AGGREGATE result is not a native frame")
	}
	soa, ok := payload.(*runtime.SoA)
	if !ok {
		t.Fatalf("FRAME_GROUP_AGGREGATE payload = %T, want *runtime.SoA", payload)
	}
	return soa
}

func assertI64Column(t *testing.T, soa *runtime.SoA, name string, want []int64) {
	t.Helper()
	col, ok := soa.Column(name)
	if !ok {
		t.Fatalf("missing i64 column %q", name)
	}
	got, ok := col.I64()
	if !ok {
		t.Fatalf("column %q dtype = %s, want i64", name, col.DType())
	}
	if len(got) != len(want) {
		t.Fatalf("column %q len = %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column %q[%d] = %d, want %d", name, i, got[i], want[i])
		}
	}
}

func assertF64Column(t *testing.T, soa *runtime.SoA, name string, want []float64) {
	t.Helper()
	col, ok := soa.Column(name)
	if !ok {
		t.Fatalf("missing f64 column %q", name)
	}
	got, ok := col.F64()
	if !ok {
		t.Fatalf("column %q dtype = %s, want f64", name, col.DType())
	}
	if len(got) != len(want) {
		t.Fatalf("column %q len = %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column %q[%d] = %v, want %v", name, i, got[i], want[i])
		}
	}
}

func assertBoolColumn(t *testing.T, soa *runtime.SoA, name string, want []bool) {
	t.Helper()
	col, ok := soa.Column(name)
	if !ok {
		t.Fatalf("missing bool column %q", name)
	}
	got, ok := col.Bool()
	if !ok {
		t.Fatalf("column %q dtype = %s, want bool", name, col.DType())
	}
	if len(got) != len(want) {
		t.Fatalf("column %q len = %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column %q[%d] = %v, want %v", name, i, got[i], want[i])
		}
	}
}

func assertStringColumn(t *testing.T, soa *runtime.SoA, name string, want []string) {
	t.Helper()
	col, ok := soa.Column(name)
	if !ok {
		t.Fatalf("missing string column %q", name)
	}
	got, ok := col.StringValues()
	if !ok {
		t.Fatalf("column %q dtype = %s, want string", name, col.DType())
	}
	if len(got) != len(want) {
		t.Fatalf("column %q len = %d, want %d", name, len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("column %q[%d] = %q, want %q", name, i, got[i], want[i])
		}
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

func frameProjectColumnPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_PROJECT_COLUMN, 0, 0, 1),
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

func frameFilterProjectColumnPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_FRAME_FILTER_PROJECT_COLUMN, 1, 0, 2),
		EncodeABC(OP_RETURN, 1, 2, 0),
	}
}

func frameFilterProjectPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABx(OP_LOADK, 1, 1),
		EncodeABC(OP_FRAME_FILTER_PROJECT, 1, 0, 2),
		EncodeABC(OP_FRAME_COLUMN, 1, 1, 3),
		EncodeABC(OP_RETURN, 1, 2, 0),
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

func frameOrderGatherPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_ORDER_GATHER, 0, 0, 1),
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
