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

func frameLenPrimitiveProgram() []uint32 {
	return []uint32{
		EncodeABx(OP_LOADK, 0, 0),
		EncodeABC(OP_FRAME_LEN, 0, 0, 0),
		EncodeABC(OP_RETURN, 0, 2, 0),
	}
}
