package vm

import (
	"testing"

	"github.com/gscript/gscript/internal/runtime"
)

func intGridAggregateTestProto() *FuncProto {
	code := make([]uint32, 169)
	for pc, inst := range map[int]uint32{
		0:   EncodeABC(OP_NEWTABLE, 2, 5, 0),
		8:   EncodeABC(OP_NEWTABLE, 8, 7, 0),
		18:  EncodeABC(OP_NEWTABLE, 16, 4, 0),
		30:  EncodeAsBx(OP_FORPREP, 23, 100),
		34:  EncodeAsBx(OP_FORPREP, 27, 95),
		42:  EncodeABC(OP_CALL, 31, 5, 2),
		65:  EncodeABC(OP_NEWOBJECTN, 34, 0, 35),
		130: EncodeAsBx(OP_FORLOOP, 27, -96),
		131: EncodeAsBx(OP_FORLOOP, 23, -101),
		136: EncodeAsBx(OP_FORPREP, 29, 29),
		144: EncodeAsBx(OP_FORPREP, 34, 20),
		165: EncodeAsBx(OP_FORLOOP, 34, -21),
		166: EncodeAsBx(OP_FORLOOP, 29, -30),
		168: EncodeABC(OP_RETURN, 35, 2, 0),
	} {
		code[pc] = inst
	}
	constants := make([]runtime.Value, 24)
	for i := 0; i < 23; i++ {
		if i == 16 {
			constants[i] = runtime.IntValue(0)
			continue
		}
		constants[i] = runtime.StringValue("k")
	}
	constants[23] = runtime.IntValue(1_000_000_007)
	return &FuncProto{
		Name:      "aggregate",
		NumParams: 2,
		MaxStack:  49,
		Code:      code,
		Constants: constants,
	}
}

func TestIntGridAggregateRuntimeSpecializationDiagnostics(t *testing.T) {
	aggregate := intGridAggregateTestProto()

	requireRuntimeSpecializationInfo(t, CallSiteRuntimeSpecializationCatalog(), "int_grid_aggregate")
	requireRuntimeSpecializationInfo(t, RecognizedCallSiteRuntimeSpecializations(aggregate), "int_grid_aggregate")
	if !cachedRuntimeSpecializationRecognized(aggregate, runtimeSpecializationIntGridAggregate) {
		t.Fatal("int_grid_aggregate rejected by runtime specialization cache")
	}
	if aggregate.IntGridAggregateKernel == nil || aggregate.IntGridAggregateKernel.spec == nil {
		t.Fatal("int_grid_aggregate proto-local spec was not generated")
	}

	diag := requireRuntimeSpecializationDiagnostic(t, DiagnoseCallSiteRuntimeSpecializationProto(aggregate), "int_grid_aggregate")
	if !diag.Recognized || diag.Reason != runtimeSpecializationReasonRecognized {
		t.Fatalf("diagnostic = %+v, want recognized %q", diag, runtimeSpecializationReasonRecognized)
	}
}

func TestIntGridAggregateRuntimeSpecializationRecordsHit(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	defer runtime.DisableRuntimePathStats()

	vm := New(runtime.NewInterpreterGlobals())
	handled, results, err := vm.tryRunCallSiteValueRuntimeSpecialization(
		NewClosure(intGridAggregateTestProto()),
		[]runtime.Value{runtime.IntValue(64), runtime.IntValue(3)},
		true,
	)
	if err != nil {
		t.Fatalf("run int_grid_aggregate: %v", err)
	}
	if !handled || len(results) != 1 || !results[0].IsInt() {
		t.Fatalf("handled=%v results=%v, want one int result", handled, results)
	}
	if got := runtimeRuntimeSpecializationHitCount(stats, RuntimeSpecializationRouteCallSiteValue, "int_grid_aggregate"); got != 1 {
		t.Fatalf("int_grid_aggregate structural hit count = %d, want 1", got)
	}
}
