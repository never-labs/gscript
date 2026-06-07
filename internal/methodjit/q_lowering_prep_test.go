//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/vm"
)

func TestVectorGatherBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_gather",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var gather *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorGather {
				gather = instr
				break
			}
		}
	}
	if gather == nil {
		t.Fatalf("BuildGraph did not emit OpVectorGather:\n%s", Print(fn))
	}
	if len(gather.Args) != 2 {
		t.Fatalf("OpVectorGather arg count = %d, want 2", len(gather.Args))
	}
	if gather.Type != TypeAny {
		t.Fatalf("OpVectorGather type = %s, want Any", gather.Type)
	}
}

func TestVectorCompareBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_compare",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var compare *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpVectorCompare {
				compare = instr
				break
			}
		}
	}
	if compare == nil {
		t.Fatalf("BuildGraph did not emit OpVectorCompare:\n%s", Print(fn))
	}
	if len(compare.Args) != 2 {
		t.Fatalf("OpVectorCompare arg count = %d, want 2", len(compare.Args))
	}
	if compare.Type != TypeAny {
		t.Fatalf("OpVectorCompare type = %s, want Any", compare.Type)
	}
	if compare.Aux != int64(runtime.DenseArrayGE) {
		t.Fatalf("OpVectorCompare Aux = %d, want %d", compare.Aux, runtime.DenseArrayGE)
	}
}

func TestFrameColumnBytecodeBuildsMethodJITIR(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_column",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	fn := BuildGraph(proto)
	var column *Instr
	for _, block := range fn.Blocks {
		for _, instr := range block.Instrs {
			if instr.Op == OpFrameColumn {
				column = instr
				break
			}
		}
	}
	if column == nil {
		t.Fatalf("BuildGraph did not emit OpFrameColumn:\n%s", Print(fn))
	}
	if len(column.Args) != 1 {
		t.Fatalf("OpFrameColumn arg count = %d, want 1", len(column.Args))
	}
	if column.Type != TypeAny {
		t.Fatalf("OpFrameColumn type = %s, want Any", column.Type)
	}
	if column.Aux != 0 {
		t.Fatalf("OpFrameColumn Aux = %d, want const index 0", column.Aux)
	}
}

func TestTier2GateAllowsFrameColumnThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:      "frame_column",
		NumParams: 1,
		MaxStack:  1,
		Constants: []runtime.Value{
			runtime.StringValue("price"),
		},
		Code: []uint32{
			vm.EncodeABC(vm.OP_FRAME_COLUMN, 0, 0, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_FRAME_COLUMN should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorGatherThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_gather",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_GATHER should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestTier2GateAllowsVectorCompareThroughOpExit(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_compare",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if !gate.Allowed {
		t.Fatalf("OP_VECTOR_COMPARE should be Tier2-eligible via op-exit, got %q", gate.Reason)
	}
}

func TestFrameColumnRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	soa, err := runtime.NewSoA(map[string]*runtime.DenseArray{
		"price": runtime.NewDenseArrayF64([]float64{10.5, 20.25}),
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

	result, err := executeFrameColumnValue(runtime.TableValue(frame), "price")
	if err != nil {
		t.Fatalf("execute frame column: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("frame column result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 10.5 || got[1] != 20.25 {
		t.Fatalf("frame column values = %#v, want [10.5 20.25]", got)
	}
}

func TestVectorGatherRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorGatherValue(
		runtime.DenseArrayValue(runtime.NewDenseArrayF64([]float64{10, 20, 30})),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{3, 1})),
	)
	if err != nil {
		t.Fatalf("execute vector gather: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector gather result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().F64()
	if !ok || len(got) != 2 || got[0] != 30 || got[1] != 10 {
		t.Fatalf("vector gather values = %#v, want [30 10]", got)
	}
}

func TestVectorCompareRuntimeHelperUsesRuntimePrimitive(t *testing.T) {
	result, err := executeVectorCompareValue(
		int(runtime.DenseArrayGE),
		runtime.DenseArrayValue(runtime.NewDenseArrayI64([]int64{1, 4, 6})),
		runtime.IntValue(4),
	)
	if err != nil {
		t.Fatalf("execute vector compare: %v", err)
	}
	if !result.IsDenseArray() {
		t.Fatalf("vector compare result = %#v, want dense array", result)
	}
	got, ok := result.DenseArray().Bool()
	if !ok || len(got) != 3 || got[0] || !got[1] || !got[2] {
		t.Fatalf("vector compare values = %#v, want [false true true]", got)
	}
}

func TestTier2ProfileConsumesQSQLNativeIdentityFeedback(t *testing.T) {
	proto := &vm.FuncProto{Name: "qsql_caller", Code: make([]uint32, 1)}
	proto.EnsureFeedback()
	qsql := runtime.FunctionValue(&runtime.GoFunction{
		Name:       "q.sql",
		NativeKind: runtime.NativeKindStdQSQL,
		NativeData: runtime.StdQSQLIdentityPtr(),
	})
	proto.CallSiteFeedback[0].ObserveCall(qsql, nil, 2, 1)

	profile := BuildTier2SpecializationProfile(proto)
	for _, guard := range profile.Guards {
		if guard.Kind != SpecGuardCallNative {
			continue
		}
		if guard.PC != 0 {
			t.Fatalf("q.sql native guard pc = %d, want 0", guard.PC)
		}
		if guard.CalleeNativeKind != runtime.NativeKindStdQSQL {
			t.Fatalf("q.sql native kind = %d, want %d", guard.CalleeNativeKind, runtime.NativeKindStdQSQL)
		}
		if guard.CalleeNativeData != uintptr(runtime.StdQSQLIdentityPtr()) {
			t.Fatalf("q.sql native data = %#x, want %#x", guard.CalleeNativeData, uintptr(runtime.StdQSQLIdentityPtr()))
		}
		if guard.NArgs != 2 || guard.ResultArity != 1 {
			t.Fatalf("q.sql native call shape = nArgs %d resultArity %d, want 2/1", guard.NArgs, guard.ResultArity)
		}
		return
	}
	t.Fatalf("q.sql native feedback did not produce call_native guard: %+v", profile.Guards)
}
