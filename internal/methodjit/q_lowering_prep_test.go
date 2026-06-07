//go:build darwin && arm64

package methodjit

import (
	"strings"
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

func TestTier2GateBlocksVectorGatherUntilBackendLowering(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_gather",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_GATHER, 0, 1, 0),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if gate.Allowed {
		t.Fatal("OP_VECTOR_GATHER should stay out of Tier 2 until backend lowering exists")
	}
	if !strings.Contains(gate.Reason, "VECTOR_GATHER") {
		t.Fatalf("gate reason = %q, want VECTOR_GATHER", gate.Reason)
	}
}

func TestTier2GateBlocksVectorCompareUntilBackendLowering(t *testing.T) {
	proto := &vm.FuncProto{
		Name:     "vector_compare",
		MaxStack: 2,
		Code: []uint32{
			vm.EncodeABC(vm.OP_VECTOR_COMPARE, 0, 1, int(runtime.DenseArrayGE)),
			vm.EncodeABC(vm.OP_RETURN, 0, 2, 0),
		},
	}

	gate := firstUnsupportedTier2BytecodeGate(proto)
	if gate.Allowed {
		t.Fatal("OP_VECTOR_COMPARE should stay out of Tier 2 until backend lowering exists")
	}
	if !strings.Contains(gate.Reason, "VECTOR_COMPARE") {
		t.Fatalf("gate reason = %q, want VECTOR_COMPARE", gate.Reason)
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
